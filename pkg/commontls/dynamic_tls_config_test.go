package commontls_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/fips140"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commontls"
)

const (
	// throttle is short so that reloads happen quickly in tests.
	throttle = 20 * time.Millisecond
	// waitFor bounds how long a test waits for a reload to take effect.
	waitFor = 5 * time.Second
	tick    = 10 * time.Millisecond

	certFile = "tls.crt"
	keyFile  = "tls.key"
	caFile   = "ca.crt"

	serverName = "server.test"

	fipsChildEnv = "DYNAMIC_TLS_CONFIG_FIPS_CHILD"
)

// --- Test PKI ---

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

// newCA creates a certificate authority, self-signed when parent is nil.
func newCA(t *testing.T, name string, parent *testCA, curve elliptic.Curve) *testCA {
	t.Helper()

	key := newKey(t, curve)
	tmpl := &x509.Certificate{
		SerialNumber:          newSerial(t),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	issuer, issuerKey := tmpl, key
	if parent != nil {
		issuer, issuerKey = parent.cert, parent.key
	}

	return &testCA{cert: createCertificate(t, tmpl, issuer, key, issuerKey), key: key}
}

// issue creates a leaf certificate from tmpl, signed by ca, with a new key on curve.
func (ca *testCA) issue(t *testing.T, tmpl *x509.Certificate, curve elliptic.Curve) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()

	key := newKey(t, curve)
	tmpl.SerialNumber = newSerial(t)
	tmpl.NotBefore = time.Now().Add(-time.Hour)
	tmpl.NotAfter = time.Now().Add(time.Hour)

	return createCertificate(t, tmpl, ca.cert, key, ca.key), key
}

func createCertificate(t *testing.T, tmpl, issuer *x509.Certificate, key, issuerKey *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()

	der, err := x509.CreateCertificate(rand.Reader, tmpl, issuer, key.Public(), issuerKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return cert
}

func newKey(t *testing.T, curve elliptic.Curve) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	require.NoError(t, err)

	return key
}

func newSerial(t *testing.T) *big.Int {
	t.Helper()

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	require.NoError(t, err)

	return serial
}

// serverTemplate is valid for serverName and the loopback addresses.
func serverTemplate() *x509.Certificate {
	return &x509.Certificate{
		Subject:     pkix.Name{CommonName: serverName},
		DNSNames:    []string{serverName},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
}

func clientTemplate(commonName string) *x509.Certificate {
	return &x509.Certificate{
		Subject:     pkix.Name{CommonName: commonName},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
}

func certPEM(cert *x509.Certificate) []byte {
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
}

func keyPEM(t *testing.T, key *ecdsa.PrivateKey) []byte {
	t.Helper()

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
}

// --- Credentials on disk ---

// fixture holds a client certificate, its key and the server CA in files, as
// a mounted Kubernetes secret would.
type fixture struct {
	dir      string
	clientCA *testCA
	serverCA *testCA
}

func newFixture(t *testing.T) *fixture {
	t.Helper()

	f := &fixture{
		dir:      t.TempDir(),
		clientCA: newCA(t, "client-ca", nil, elliptic.P256()),
		serverCA: newCA(t, "server-ca", nil, elliptic.P256()),
	}
	f.writeClientCert(t, "client-v1")
	f.writeServerCA(t, f.serverCA)

	return f
}

func (f *fixture) path(name string) string {
	return filepath.Join(f.dir, name)
}

func (f *fixture) writeClientCert(t *testing.T, commonName string) {
	t.Helper()

	cert, key := f.clientCA.issue(t, clientTemplate(commonName), elliptic.P256())
	writeFile(t, f.path(certFile), certPEM(cert))
	writeFile(t, f.path(keyFile), keyPEM(t, key))
}

// writeClientChain writes a client certificate issued by intermediates[0],
// followed by the intermediates, the way cert-manager writes tls.crt. Each
// intermediate is issued by the next one, the last by the root.
func (f *fixture) writeClientChain(t *testing.T, commonName string, intermediates ...*testCA) {
	t.Helper()

	cert, key := intermediates[0].issue(t, clientTemplate(commonName), elliptic.P256())

	chain := certPEM(cert)
	for _, ca := range intermediates {
		chain = append(chain, certPEM(ca.cert)...)
	}

	writeFile(t, f.path(certFile), chain)
	writeFile(t, f.path(keyFile), keyPEM(t, key))
}

func (f *fixture) writeServerCA(t *testing.T, ca *testCA) {
	t.Helper()

	writeFile(t, f.path(caFile), certPEM(ca.cert))
}

func (f *fixture) mtls(attrs *commoncfg.TLSAttributes) *commoncfg.MTLS {
	ca := fileRef(f.path(caFile))

	return &commoncfg.MTLS{
		Cert:       fileRef(f.path(certFile)),
		CertKey:    fileRef(f.path(keyFile)),
		ServerCA:   &ca,
		Attributes: attrs,
	}
}

func (f *fixture) newDynamic(t *testing.T, attrs *commoncfg.TLSAttributes) *commontls.DynamicTLSConfig {
	t.Helper()

	dyn, err := commontls.NewDynamicTLSConfig(throttle, f.mtls(attrs))
	require.NoError(t, err)
	t.Cleanup(func() { _ = dyn.Close() })

	return dyn
}

// startServer starts an HTTPS server presenting a certificate from serverCA
// and requiring client certificates from clientCA.
func (f *fixture) startServer(t *testing.T) *testServer {
	t.Helper()

	return startServer(t, f.serverCertificate(t, f.serverCA), f.clientCA.cert)
}

func (f *fixture) serverCertificate(t *testing.T, ca *testCA) tls.Certificate {
	t.Helper()

	leaf, key := ca.issue(t, serverTemplate(), elliptic.P256())

	return tls.Certificate{Certificate: [][]byte{leaf.Raw}, PrivateKey: key, Leaf: leaf}
}

// serverChain returns a server certificate issued by intermediates[0], sent
// together with the intermediates. Each intermediate is issued by the next
// one, the last by the root.
func (f *fixture) serverChain(t *testing.T, intermediates ...*testCA) tls.Certificate {
	t.Helper()

	leaf, key := intermediates[0].issue(t, serverTemplate(), elliptic.P256())

	cert := tls.Certificate{Certificate: [][]byte{leaf.Raw}, PrivateKey: key, Leaf: leaf}
	for _, ca := range intermediates {
		cert.Certificate = append(cert.Certificate, ca.cert.Raw)
	}

	return cert
}

func withServerName() *commoncfg.TLSAttributes {
	return &commoncfg.TLSAttributes{ServerName: serverName}
}

func fileRef(path string) commoncfg.SourceRef {
	return commoncfg.SourceRef{
		Source: commoncfg.FileSourceValue,
		File:   commoncfg.CredentialFile{Path: path, Format: commoncfg.BinaryFileFormat},
	}
}

func embeddedRef(value []byte) commoncfg.SourceRef {
	return commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(value)}
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, data, 0o600))
}

// --- Test server and client ---

// testServer answers every request with the common name of the client
// certificate. Its own certificate can be replaced while it runs.
type testServer struct {
	*httptest.Server

	cert atomic.Pointer[tls.Certificate]
}

func startServer(t *testing.T, cert tls.Certificate, clientCA *x509.Certificate) *testServer {
	t.Helper()

	ts := &testServer{}
	ts.cert.Store(&cert)

	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(clientCA)

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.TLS.PeerCertificates[0].Subject.CommonName)
	}))
	srv.Config.ErrorLog = log.New(io.Discard, "", 0) // failed handshakes are expected in some tests
	srv.TLS = &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetConfigForClient: func(*tls.ClientHelloInfo) (*tls.Config, error) {
			return &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{*ts.cert.Load()},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				ClientCAs:    clientCAs,
			}, nil
		},
	}
	srv.StartTLS()
	t.Cleanup(srv.Close)

	ts.Server = srv

	return ts
}

// urlFor returns the server URL with host in place of the loopback address.
func (ts *testServer) urlFor(t *testing.T, host string) string {
	t.Helper()

	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	return "https://" + net.JoinHostPort(host, u.Port())
}

// newClient returns an HTTP client that opens a new connection, and so does a
// new handshake, for every request. If dialAddr is set, every connection goes
// to dialAddr whatever the host in the URL.
func newClient(cfg *tls.Config, dialAddr string) *http.Client {
	transport := &http.Transport{TLSClientConfig: cfg, DisableKeepAlives: true}
	if dialAddr != "" {
		transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, dialAddr)
		}
	}

	return &http.Client{Transport: transport, Timeout: waitFor}
}

// get returns the response body, which is the common name of the client
// certificate the server saw.
func get(ctx context.Context, client *http.Client, target string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	return string(body), err
}

// requireClientCertEventually waits until the server sees the client
// certificate with the given common name.
func requireClientCertEventually(t *testing.T, client *http.Client, target, commonName string) {
	t.Helper()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		body, err := get(t.Context(), client, target)
		assert.NoError(c, err)
		assert.Equal(c, commonName, body)
	}, waitFor, tick)
}

// --- Construction ---

func TestNewDynamicTLSConfigErrors(t *testing.T) {
	t.Parallel()

	t.Run("nil MTLS", func(t *testing.T) {
		t.Parallel()

		_, err := commontls.NewDynamicTLSConfig(throttle, nil)
		require.ErrorIs(t, err, commoncfg.ErrMTLSIsNil)
	})

	tests := []struct {
		name   string
		modify func(t *testing.T, f *fixture, mtls *commoncfg.MTLS)
	}{
		{
			name: "directory of the certificate does not exist",
			modify: func(_ *testing.T, f *fixture, mtls *commoncfg.MTLS) {
				mtls.Cert = fileRef(filepath.Join(f.dir, "missing", certFile))
			},
		},
		{
			name: "key file does not exist",
			modify: func(_ *testing.T, f *fixture, mtls *commoncfg.MTLS) {
				mtls.CertKey = fileRef(f.path("missing.key"))
			},
		},
		{
			name: "CA file does not exist",
			modify: func(_ *testing.T, f *fixture, mtls *commoncfg.MTLS) {
				ca := fileRef(f.path("missing-ca.crt"))
				mtls.ServerCA = &ca
			},
		},
		{
			name: "key does not match the certificate",
			modify: func(t *testing.T, f *fixture, _ *commoncfg.MTLS) {
				t.Helper()

				_, otherKey := f.clientCA.issue(t, clientTemplate("other"), elliptic.P256())
				writeFile(t, f.path(keyFile), keyPEM(t, otherKey))
			},
		},
		{
			name: "certificate is not PEM",
			modify: func(t *testing.T, f *fixture, _ *commoncfg.MTLS) {
				t.Helper()

				writeFile(t, f.path(certFile), []byte("not a certificate"))
			},
		},
		{
			name: "source type is not set",
			modify: func(_ *testing.T, _ *fixture, mtls *commoncfg.MTLS) {
				mtls.Cert.Source = ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			mtls := f.mtls(withServerName())
			tt.modify(t, f, mtls)

			dyn, err := commontls.NewDynamicTLSConfig(throttle, mtls)
			require.Error(t, err)
			assert.Nil(t, dyn)
		})
	}
}

func TestConfigAppliesAttributes(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	dyn := f.newDynamic(t, &commoncfg.TLSAttributes{
		ServerName:                serverName,
		SessionTicketsDisabled:    true,
		AllowTLSRenegotiationOnce: true,
	})

	// The fields of the embedded *tls.Config are used directly.
	assert.Equal(t, uint16(tls.VersionTLS12), dyn.MinVersion)
	assert.Equal(t, serverName, dyn.ServerName)
	assert.True(t, dyn.SessionTicketsDisabled)
	assert.Equal(t, tls.RenegotiateOnceAsClient, dyn.Renegotiation)
	assert.Empty(t, dyn.Certificates, "the client certificate is served by GetClientCertificate")
	assert.NotNil(t, dyn.GetClientCertificate)
}

func TestConfigVerificationMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// modify adjusts the CA sources and attributes of the fixture's MTLS.
		modify func(f *fixture, mtls *commoncfg.MTLS)
		// wantVerifyConnection is true when the server is verified against
		// reloadable CAs in VerifyConnection.
		wantVerifyConnection   bool
		wantInsecureSkipVerify bool
		wantRootCAs            bool
	}{
		{
			name:                   "CA from a file is verified in VerifyConnection",
			modify:                 func(*fixture, *commoncfg.MTLS) {},
			wantVerifyConnection:   true,
			wantInsecureSkipVerify: true,
		},
		{
			name: "CA from a file among root CAs is verified in VerifyConnection",
			modify: func(f *fixture, mtls *commoncfg.MTLS) {
				mtls.ServerCA = nil
				mtls.RootCAs = []commoncfg.SourceRef{embeddedRef(certPEM(f.clientCA.cert)), fileRef(f.path(caFile))}
			},
			wantVerifyConnection:   true,
			wantInsecureSkipVerify: true,
		},
		{
			name: "embedded CA uses standard verification",
			modify: func(f *fixture, mtls *commoncfg.MTLS) {
				ca := embeddedRef(certPEM(f.serverCA.cert))
				mtls.ServerCA = &ca
			},
			wantRootCAs: true,
		},
		{
			name: "no CA uses standard verification with the system pool",
			modify: func(_ *fixture, mtls *commoncfg.MTLS) {
				mtls.ServerCA = nil
			},
		},
		{
			name: "InsecureSkipVerify attribute turns verification off",
			modify: func(_ *fixture, mtls *commoncfg.MTLS) {
				mtls.Attributes = &commoncfg.TLSAttributes{InsecureSkipVerify: true}
			},
			wantInsecureSkipVerify: true,
			wantRootCAs:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			mtls := f.mtls(withServerName())
			tt.modify(f, mtls)

			dyn, err := commontls.NewDynamicTLSConfig(throttle, mtls)
			require.NoError(t, err)
			t.Cleanup(func() { _ = dyn.Close() })

			assert.Equal(t, tt.wantVerifyConnection, dyn.VerifyConnection != nil, "VerifyConnection set")
			assert.Equal(t, tt.wantInsecureSkipVerify, dyn.InsecureSkipVerify, "InsecureSkipVerify")
			assert.Equal(t, tt.wantRootCAs, dyn.RootCAs != nil, "RootCAs set")
		})
	}
}

func TestCloneIsIndependentAndKeepsCallbacks(t *testing.T) {
	t.Parallel()

	dyn := newFixture(t).newDynamic(t, withServerName())

	clone := dyn.Clone()
	clone.NextProtos = []string{"h2"}
	clone.ServerName = "changed.test"

	assert.NotSame(t, dyn.Config, clone)
	assert.Empty(t, dyn.NextProtos)
	assert.Equal(t, serverName, dyn.ServerName)
	assert.NotNil(t, clone.GetClientCertificate, "a clone must keep serving the reloaded certificate")
	assert.NotNil(t, clone.VerifyConnection, "a clone must keep verifying against the reloaded CAs")
}

// --- Handshakes ---

func TestMutualTLSHandshake(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t)
	dyn := f.newDynamic(t, withServerName())

	body, err := get(t.Context(), newClient(dyn.Config, ""), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "client-v1", body)
}

func TestHandshakeServerName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		attrs *commoncfg.TLSAttributes
		// host is put in the URL; connections always go to the test server.
		host    string
		wantErr string
	}{
		{
			name: "host name in the URL is verified through SNI",
			host: serverName,
		},
		{
			name:  "IP address in the URL uses the configured server name",
			attrs: withServerName(),
			host:  "127.0.0.1",
		},
		{
			name:  "configured IP address is checked against the IP SANs",
			attrs: &commoncfg.TLSAttributes{ServerName: "127.0.0.1"},
			host:  "127.0.0.1",
		},
		{
			name:    "IP address in the URL without a configured server name fails",
			host:    "127.0.0.1",
			wantErr: "no server name to verify the server certificate against",
		},
		{
			name:    "server name the certificate is not valid for fails",
			attrs:   &commoncfg.TLSAttributes{ServerName: "other.test"},
			host:    "127.0.0.1",
			wantErr: "certificate is valid for server.test, not other.test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			srv := f.startServer(t)
			dyn := f.newDynamic(t, tt.attrs)
			client := newClient(dyn.Config, srv.Listener.Addr().String())

			body, err := get(t.Context(), client, srv.urlFor(t, tt.host))
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "client-v1", body)
		})
	}
}

func TestHandshakeRejectsServerFromUnknownCA(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	otherCA := newCA(t, "other-ca", nil, elliptic.P256())
	srv := startServer(t, f.serverCertificate(t, otherCA), f.clientCA.cert)
	dyn := f.newDynamic(t, withServerName())

	_, err := get(t.Context(), newClient(dyn.Config, ""), srv.URL)

	var unknownAuthority x509.UnknownAuthorityError
	require.ErrorAs(t, err, &unknownAuthority)
}

func TestEmbeddedCredentialsUseStandardVerification(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t)

	cert, key := f.clientCA.issue(t, clientTemplate("embedded"), elliptic.P256())
	ca := embeddedRef(certPEM(f.serverCA.cert))

	dyn, err := commontls.NewDynamicTLSConfig(throttle, &commoncfg.MTLS{
		Cert:     embeddedRef(certPEM(cert)),
		CertKey:  embeddedRef(keyPEM(t, key)),
		ServerCA: &ca,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = dyn.Close() })

	// crypto/tls verifies the IP address in the URL itself, so no server
	// name has to be configured.
	body, err := get(t.Context(), newClient(dyn.Config, ""), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "embedded", body)
}

// --- VerifyConnection ---

func TestVerifyConnection(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	intermediate := newCA(t, "intermediate", f.serverCA, elliptic.P256())
	leafFromIntermediate, _ := intermediate.issue(t, serverTemplate(), elliptic.P256())
	leaf, _ := f.serverCA.issue(t, serverTemplate(), elliptic.P256())
	clientOnly := serverTemplate()
	clientOnly.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	clientOnlyLeaf, _ := f.serverCA.issue(t, clientOnly, elliptic.P256())

	tests := []struct {
		name    string
		attrs   *commoncfg.TLSAttributes
		state   tls.ConnectionState
		wantErr string
	}{
		{
			name:  "valid certificate for the SNI name",
			state: tls.ConnectionState{ServerName: serverName, PeerCertificates: []*x509.Certificate{leaf}},
		},
		{
			name:  "no SNI falls back to the configured server name",
			attrs: withServerName(),
			state: tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}},
		},
		{
			name:  "SNI name takes precedence over the configured server name",
			attrs: &commoncfg.TLSAttributes{ServerName: "other.test"},
			state: tls.ConnectionState{ServerName: serverName, PeerCertificates: []*x509.Certificate{leaf}},
		},
		{
			name:    "no SNI and no configured server name",
			state:   tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}},
			wantErr: "no server name to verify the server certificate against",
		},
		{
			name:    "no peer certificate",
			state:   tls.ConnectionState{ServerName: serverName},
			wantErr: "server sent no certificate",
		},
		{
			name:    "wrong host name",
			state:   tls.ConnectionState{ServerName: "other.test", PeerCertificates: []*x509.Certificate{leaf}},
			wantErr: "not other.test",
		},
		{
			name: "intermediate sent by the server",
			state: tls.ConnectionState{
				ServerName:       serverName,
				PeerCertificates: []*x509.Certificate{leafFromIntermediate, intermediate.cert},
			},
		},
		{
			name:    "intermediate missing",
			state:   tls.ConnectionState{ServerName: serverName, PeerCertificates: []*x509.Certificate{leafFromIntermediate}},
			wantErr: "certificate signed by unknown authority",
		},
		{
			name:    "certificate not valid for server authentication",
			state:   tls.ConnectionState{ServerName: serverName, PeerCertificates: []*x509.Certificate{clientOnlyLeaf}},
			wantErr: "incompatible key usage",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dyn, err := commontls.NewDynamicTLSConfig(throttle, f.mtls(tt.attrs))
			require.NoError(t, err)
			t.Cleanup(func() { _ = dyn.Close() })

			err = dyn.VerifyConnection(tt.state)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}

			require.NoError(t, err)
		})
	}
}

// TestVerifyConnectionWarnsAboutNonFIPSChainsInFIPSMode checks that, in FIPS
// 140-3 mode, a server chain crypto/tls would reject for its keys is accepted
// and a warning is logged. FIPS mode can only be enabled at process start, so
// unless the tests already run in it, the test runs itself again in a child
// process with GODEBUG=fips140=on.
func TestVerifyConnectionWarnsAboutNonFIPSChainsInFIPSMode(t *testing.T) {
	if !fips140.Enabled() {
		if os.Getenv(fipsChildEnv) != "" {
			t.Fatal("GODEBUG=fips140=on did not enable FIPS 140-3 mode")
		}

		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+t.Name()+"$", "-test.count=1")

		cmd.Env = append(os.Environ(), "GODEBUG=fips140=on", fipsChildEnv+"=1")

		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "test in FIPS 140-3 mode failed:\n%s", out)

		return
	}

	tests := []struct {
		name     string
		curve    elliptic.Curve
		wantWarn bool
	}{
		{name: "P-256 chain is accepted silently", curve: elliptic.P256()},
		{name: "P-224 chain is accepted with a warning", curve: elliptic.P224(), wantWarn: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := &syncBuffer{}
			previous := slog.Default()

			slog.SetDefault(slog.New(slog.NewTextHandler(logs, nil)))
			t.Cleanup(func() { slog.SetDefault(previous) })

			f := newFixture(t)
			ca := newCA(t, "fips-ca", nil, tt.curve)
			f.writeServerCA(t, ca)
			leaf, _ := ca.issue(t, serverTemplate(), tt.curve)

			dyn := f.newDynamic(t, nil)

			err := dyn.VerifyConnection(tls.ConnectionState{
				ServerName:       serverName,
				PeerCertificates: []*x509.Certificate{leaf},
			})
			require.NoError(t, err)

			if tt.wantWarn {
				assert.Contains(t, logs.String(), "not FIPS-approved")
			} else {
				assert.NotContains(t, logs.String(), "not FIPS-approved")
			}
		})
	}
}

// syncBuffer is a bytes.Buffer safe for the concurrent writes of a logger.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

// --- Rotation ---

func TestClientCertificateRotation(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t)
	dyn := f.newDynamic(t, withServerName())

	// The clients are built once; the rotation has to reach them without
	// rebuilding the transports, including through a clone taken before it.
	client := newClient(dyn.Config, "")
	clonedClient := newClient(dyn.Clone(), "")

	requireClientCertEventually(t, client, srv.URL, "client-v1")
	requireClientCertEventually(t, clonedClient, srv.URL, "client-v1")

	f.writeClientCert(t, "client-v2")

	requireClientCertEventually(t, client, srv.URL, "client-v2")
	requireClientCertEventually(t, clonedClient, srv.URL, "client-v2")
}

func TestServerCARotation(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t)
	dyn := f.newDynamic(t, withServerName())
	client := newClient(dyn.Config, "")

	requireClientCertEventually(t, client, srv.URL, "client-v1")

	// The server moves to a certificate from a new CA, which the client does
	// not trust yet.
	newServerCA := newCA(t, "server-ca-v2", nil, elliptic.P256())
	newServerCert := f.serverCertificate(t, newServerCA)
	srv.cert.Store(&newServerCert)

	_, err := get(t.Context(), client, srv.URL)

	var unknownAuthority x509.UnknownAuthorityError
	require.ErrorAs(t, err, &unknownAuthority)

	f.writeServerCA(t, newServerCA)

	requireClientCertEventually(t, client, srv.URL, "client-v1")
}

// TestIntermediateRotation rotates intermediate CAs in chains of several
// intermediates, under roots that stay the same: first the intermediate that
// issues the leaf, then the one under the root, which means re-issuing
// everything below it. The server sends its intermediates in the handshake, so
// their rotation needs no change on the client. The client sends the
// intermediates stored in tls.crt, so their rotation is a reload like any
// other, however long the chain.
func TestIntermediateRotation(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	p256 := elliptic.P256()

	clientTop := newCA(t, "client-top-1", f.clientCA, p256)
	clientIssuing := newCA(t, "client-issuing-1", clientTop, p256)
	f.writeClientChain(t, "client-v1", clientIssuing, clientTop)

	serverTop := newCA(t, "server-top-1", f.serverCA, p256)
	serverIssuing := newCA(t, "server-issuing-1", serverTop, p256)
	srv := startServer(t, f.serverChain(t, serverIssuing, serverTop), f.clientCA.cert)

	dyn := f.newDynamic(t, withServerName())
	client := newClient(dyn.Config, "")

	requireClientCertEventually(t, client, srv.URL, "client-v1")

	rotateServer := func(intermediates ...*testCA) {
		chain := f.serverChain(t, intermediates...)
		srv.cert.Store(&chain)
	}

	serverIssuing = newCA(t, "server-issuing-2", serverTop, p256)
	rotateServer(serverIssuing, serverTop)
	requireClientCertEventually(t, client, srv.URL, "client-v1")

	serverTop = newCA(t, "server-top-2", f.serverCA, p256)
	serverIssuing = newCA(t, "server-issuing-3", serverTop, p256)
	rotateServer(serverIssuing, serverTop)
	requireClientCertEventually(t, client, srv.URL, "client-v1")

	clientIssuing = newCA(t, "client-issuing-2", clientTop, p256)
	f.writeClientChain(t, "client-v2", clientIssuing, clientTop)
	requireClientCertEventually(t, client, srv.URL, "client-v2")

	clientTop = newCA(t, "client-top-2", f.clientCA, p256)
	clientIssuing = newCA(t, "client-issuing-3", clientTop, p256)
	f.writeClientChain(t, "client-v3", clientIssuing, clientTop)
	requireClientCertEventually(t, client, srv.URL, "client-v3")
}

// TestKubernetesSecretRotation rotates the files the way the kubelet updates
// a mounted secret: the files are symlinks through ..data, which is swapped
// atomically to a new directory.
//
// It runs on Linux only. On macOS and BSD, fsnotify uses kqueue, which reports
// a new directory entry only if it sees it while scanning the directory, so
// the short-lived ..data_tmp symlink, and with it the swap, can go unnoticed.
func TestKubernetesSecretRotation(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the atomic ..data swap is only reliably reported by inotify (Linux)")
	}

	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t)

	mount := t.TempDir()
	writeVersion := func(version string) {
		// Generate first, so that the files appear and the swap happens in
		// quick succession, as with the kubelet.
		cert, key := f.clientCA.issue(t, clientTemplate("client-"+version), elliptic.P256())

		dir := filepath.Join(mount, "..2026_"+version)
		require.NoError(t, os.Mkdir(dir, 0o700))
		writeFile(t, filepath.Join(dir, certFile), certPEM(cert))
		writeFile(t, filepath.Join(dir, keyFile), keyPEM(t, key))
		writeFile(t, filepath.Join(dir, caFile), certPEM(f.serverCA.cert))

		tmpLink := filepath.Join(mount, "..data_tmp")
		require.NoError(t, os.Symlink(filepath.Base(dir), tmpLink))
		require.NoError(t, os.Rename(tmpLink, filepath.Join(mount, "..data")))
	}

	writeVersion("v1")

	for _, name := range []string{certFile, keyFile, caFile} {
		require.NoError(t, os.Symlink(filepath.Join("..data", name), filepath.Join(mount, name)))
	}

	ca := fileRef(filepath.Join(mount, caFile))
	dyn, err := commontls.NewDynamicTLSConfig(throttle, &commoncfg.MTLS{
		Cert:       fileRef(filepath.Join(mount, certFile)),
		CertKey:    fileRef(filepath.Join(mount, keyFile)),
		ServerCA:   &ca,
		Attributes: withServerName(),
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = dyn.Close() })

	client := newClient(dyn.Config, "")
	requireClientCertEventually(t, client, srv.URL, "client-v1")

	writeVersion("v2")

	requireClientCertEventually(t, client, srv.URL, "client-v2")
}

func TestFailedReloadKeepsPreviousCredentials(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t)
	dyn := f.newDynamic(t, withServerName())
	client := newClient(dyn.Config, "")

	writeFile(t, f.path(certFile), []byte("not a certificate"))
	time.Sleep(10 * throttle) // let the failing reload run

	body, err := get(t.Context(), client, srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "client-v1", body)

	f.writeClientCert(t, "client-v2")

	requireClientCertEventually(t, client, srv.URL, "client-v2")
}

func TestCloseStopsReloading(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t)

	dyn, err := commontls.NewDynamicTLSConfig(throttle, f.mtls(withServerName()))
	require.NoError(t, err)

	require.NoError(t, dyn.Close())
	require.NoError(t, dyn.Close(), "Close must be safe to call twice")

	f.writeClientCert(t, "client-v2")
	time.Sleep(10 * throttle) // a reload, if any, would have run by now

	// The configuration keeps working with the credentials loaded last.
	body, err := get(t.Context(), newClient(dyn.Config, ""), srv.URL)
	require.NoError(t, err)
	assert.Equal(t, "client-v1", body)
}

func TestHandshakesDuringRotationSucceed(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t)
	dyn := f.newDynamic(t, withServerName())
	client := newClient(dyn.Config, "")

	var (
		wg       sync.WaitGroup
		failures atomic.Int64
		firstErr atomic.Value
	)

	ctx, stop := context.WithCancel(t.Context())

	for range 4 {
		wg.Go(func() {
			for ctx.Err() == nil {
				_, err := get(ctx, client, srv.URL)
				if err != nil && ctx.Err() == nil {
					failures.Add(1)
					firstErr.CompareAndSwap(nil, err)
				}
			}
		})
	}

	t.Cleanup(func() {
		stop()
		wg.Wait()
	})

	for version := 2; version <= 5; version++ {
		commonName := fmt.Sprintf("client-v%d", version)
		f.writeClientCert(t, commonName)
		requireClientCertEventually(t, client, srv.URL, commonName)
	}

	stop()
	wg.Wait()

	if failures.Load() > 0 {
		t.Fatalf("%d handshakes failed during rotation, first error: %v", failures.Load(), firstErr.Load())
	}
}
