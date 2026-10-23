package commongrpc_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	grpchealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"
)

const (
	// throttle is short so that reloads happen quickly in tests.
	throttle = 20 * time.Millisecond
	// shortConnectionAge makes a test server close connections often, so
	// that clients reconnect and present their rotated certificates.
	shortConnectionAge = 100 * time.Millisecond
	waitFor            = 10 * time.Second
	tick               = 20 * time.Millisecond

	certFile = "tls.crt"
	keyFile  = "tls.key"
	caFile   = "ca.crt"

	// clientCNHeader carries the common name of the client certificate the
	// test server saw.
	clientCNHeader = "client-cn"
)

// --- Test PKI ---

type testCA struct {
	cert *x509.Certificate
	key  *ecdsa.PrivateKey
}

// newCA creates a certificate authority, self-signed when parent is nil.
func newCA(t *testing.T, name string, parent *testCA) *testCA {
	t.Helper()

	tmpl := &x509.Certificate{
		Subject:               pkix.Name{CommonName: name},
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}

	key := newKey(t)

	issuer, issuerKey := tmpl, key
	if parent != nil {
		issuer, issuerKey = parent.cert, parent.key
	}

	return &testCA{cert: createCertificate(t, tmpl, issuer, key, issuerKey), key: key}
}

// issue creates a certificate from tmpl, signed by ca, with a new key.
func (ca *testCA) issue(t *testing.T, tmpl *x509.Certificate) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()

	key := newKey(t)

	return createCertificate(t, tmpl, ca.cert, key, ca.key), key
}

func createCertificate(t *testing.T, tmpl, issuer *x509.Certificate, key, issuerKey *ecdsa.PrivateKey) *x509.Certificate {
	t.Helper()

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	require.NoError(t, err)

	tmpl.SerialNumber = serial
	tmpl.NotBefore = time.Now().Add(-time.Hour)
	tmpl.NotAfter = time.Now().Add(time.Hour)

	der, err := x509.CreateCertificate(rand.Reader, tmpl, issuer, key.Public(), issuerKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)

	return cert
}

func newKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	return key
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

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()

	require.NoError(t, os.WriteFile(path, data, 0o600))
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
		clientCA: newCA(t, "client-ca", nil),
		serverCA: newCA(t, "server-ca", nil),
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

	f.writeClientChain(t, commonName)
}

// writeClientChain writes a client certificate followed by intermediates, the
// way cert-manager writes tls.crt. The certificate is issued by
// intermediates[0], each intermediate by the next one and the last by the
// client CA; without intermediates the client CA issues the certificate.
func (f *fixture) writeClientChain(t *testing.T, commonName string, intermediates ...*testCA) {
	t.Helper()

	issuer := f.clientCA
	if len(intermediates) > 0 {
		issuer = intermediates[0]
	}

	cert, key := issuer.issue(t, &x509.Certificate{
		Subject:     pkix.Name{CommonName: commonName},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	})

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

// serverCertificate is valid for localhost and the loopback addresses.
func serverCertificate(t *testing.T, ca *testCA) tls.Certificate {
	t.Helper()

	leaf, key := ca.issue(t, &x509.Certificate{
		Subject:     pkix.Name{CommonName: "localhost"},
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	})

	return tls.Certificate{Certificate: [][]byte{leaf.Raw}, PrivateKey: key, Leaf: leaf}
}

func (f *fixture) clientConfig(address string, attrs *commoncfg.TLSAttributes) *commoncfg.GRPCClient {
	ca := fileRef(f.path(caFile))

	return &commoncfg.GRPCClient{
		Address: address,
		SecretRef: &commoncfg.SecretRef{
			Type: commoncfg.MTLSSecretType,
			MTLS: commoncfg.MTLS{
				Cert:       fileRef(f.path(certFile)),
				CertKey:    fileRef(f.path(keyFile)),
				ServerCA:   &ca,
				Attributes: attrs,
			},
		},
	}
}

func fileRef(path string) commoncfg.SourceRef {
	return commoncfg.SourceRef{
		Source: commoncfg.FileSourceValue,
		File:   commoncfg.CredentialFile{Path: path, Format: commoncfg.BinaryFileFormat},
	}
}

// --- Test server ---

type serverOptions struct {
	// listenAddr defaults to an ephemeral port on 127.0.0.1.
	listenAddr string
	// maxConnectionAge is how long the server keeps a connection before
	// closing it; zero keeps connections open.
	maxConnectionAge time.Duration
	// insecure serves plaintext instead of mutual TLS.
	insecure bool
}

// testServer is a gRPC server, built with commongrpc.NewServer, serving the
// gRPC health service. Unless insecure, it requires client certificates from
// the fixture's client CA and reports their common name in clientCNHeader.
// Its certificate can be replaced while it runs, and it can be restarted on
// the same address.
type testServer struct {
	f    *fixture
	opts serverOptions
	addr string

	cert       atomic.Pointer[tls.Certificate]
	health     *grpchealth.Server
	handshakes atomic.Int64 // completed mutual TLS handshakes

	mu  sync.Mutex
	srv *grpc.Server
}

func (f *fixture) startServer(t *testing.T, opts serverOptions) *testServer {
	t.Helper()

	ts := &testServer{f: f, opts: opts, health: grpchealth.NewServer()}

	cert := serverCertificate(t, f.serverCA)
	ts.cert.Store(&cert)

	listenAddr := opts.listenAddr
	if listenAddr == "" {
		listenAddr = "127.0.0.1:0"
	}

	ts.serve(t, listenAddr)

	t.Cleanup(func() {
		ts.mu.Lock()
		defer ts.mu.Unlock()

		ts.srv.Stop()
	})

	return ts
}

func (ts *testServer) serve(t *testing.T, address string) {
	t.Helper()

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", address)
	require.NoError(t, err)

	options := []grpc.ServerOption{grpc.ChainUnaryInterceptor(reportClientCN)}
	if !ts.opts.insecure {
		options = append(options, grpc.Creds(credentials.NewTLS(ts.tlsConfig())))
	}

	srv := commongrpc.NewServer(t.Context(), &commoncfg.GRPCServer{
		MaxRecvMsgSize: 4 << 20,
		Attributes: commoncfg.GRPCServerAttributes{
			MaxConnectionAge:      ts.opts.maxConnectionAge,
			MaxConnectionAgeGrace: time.Second,
		},
	}, options...)
	healthpb.RegisterHealthServer(srv, ts.health)

	go func() { _ = srv.Serve(lis) }()

	ts.mu.Lock()
	ts.srv = srv
	ts.mu.Unlock()

	ts.addr = lis.Addr().String()
}

func (ts *testServer) tlsConfig() *tls.Config {
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(ts.f.clientCA.cert)

	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientCAs,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return ts.cert.Load(), nil
		},
		VerifyConnection: func(tls.ConnectionState) error {
			ts.handshakes.Add(1)
			return nil
		},
	}
}

// restart stops the server, which closes all its connections, and serves
// again on the same address.
func (ts *testServer) restart(t *testing.T) {
	t.Helper()

	ts.mu.Lock()
	ts.srv.Stop()
	ts.mu.Unlock()

	ts.serve(t, ts.addr)
}

func (ts *testServer) port(t *testing.T) string {
	t.Helper()

	_, port, err := net.SplitHostPort(ts.addr)
	require.NoError(t, err)

	return port
}

// reportClientCN returns the common name of the client certificate in the
// clientCNHeader response header.
func reportClientCN(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	if p, ok := peer.FromContext(ctx); ok {
		if info, ok := p.AuthInfo.(credentials.TLSInfo); ok && len(info.State.PeerCertificates) > 0 {
			_ = grpc.SetHeader(ctx, metadata.Pairs(clientCNHeader, info.State.PeerCertificates[0].Subject.CommonName))
		}
	}

	return handler(ctx, req)
}

// --- Client helpers ---

// fastReconnect lets the client retry quickly after a failed connection
// attempt, instead of gRPC's default backoff of a second and more.
func fastReconnect() grpc.DialOption {
	return grpc.WithConnectParams(grpc.ConnectParams{
		Backoff:           backoff.Config{BaseDelay: 20 * time.Millisecond, Multiplier: 1.2, MaxDelay: 100 * time.Millisecond},
		MinConnectTimeout: time.Second,
	})
}

func newDynamicClientConn(t *testing.T, cfg *commoncfg.GRPCClient, dialOptions ...grpc.DialOption) *commongrpc.DynamicClientConn {
	t.Helper()

	conn, err := commongrpc.NewDynamicClientConn(cfg, throttle, append(dialOptions, fastReconnect())...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	return conn
}

// clientCN calls the health service through client and returns the common
// name of the client certificate the server saw.
func clientCN(ctx context.Context, client healthpb.HealthClient) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, waitFor)
	defer cancel()

	var header metadata.MD

	_, err := client.Check(ctx, &healthpb.HealthCheckRequest{}, grpc.Header(&header))
	if err != nil {
		return "", err
	}

	values := header.Get(clientCNHeader)
	if len(values) == 0 {
		return "", fmt.Errorf("no %s header in the response", clientCNHeader)
	}

	return values[0], nil
}

func requireClientCNEventually(t *testing.T, client healthpb.HealthClient, commonName string) {
	t.Helper()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		cn, err := clientCN(t.Context(), client)
		assert.NoError(c, err)
		assert.Equal(c, commonName, cn)
	}, waitFor, tick)
}

// openWatch opens a health Watch stream and reads the first status.
func openWatch(t *testing.T, ctx context.Context, cc grpc.ClientConnInterface) healthpb.Health_WatchClient {
	t.Helper()

	stream, err := healthpb.NewHealthClient(cc).Watch(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	require.Equal(t, healthpb.HealthCheckResponse_SERVING, resp.GetStatus())

	return stream
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

// captureLogs sends the output of the default logger to a buffer for the rest
// of the test. The default logger is global, so tests using it must not run
// in parallel.
func captureLogs(t *testing.T) *syncBuffer {
	t.Helper()

	logs := &syncBuffer{}
	previous := slog.Default()

	slog.SetDefault(slog.New(slog.NewTextHandler(logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	return logs
}

// requireLogEventually waits for a log line with message about the watched
// directory dir, which tells that a reload of dir has run.
func requireLogEventually(t *testing.T, logs *syncBuffer, message, dir string) {
	t.Helper()

	require.Eventually(t, func() bool {
		for line := range strings.Lines(logs.String()) {
			if strings.Contains(line, message) && strings.Contains(line, dir) {
				return true
			}
		}

		return false
	}, waitFor, tick, "no log %q for %s", message, dir)
}

// --- Construction ---

func TestNewDynamicClientConn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  func(f *fixture) *commoncfg.GRPCClient
	}{
		{
			name: "without SecretRef",
			cfg: func(*fixture) *commoncfg.GRPCClient {
				return &commoncfg.GRPCClient{Address: "localhost:12345"}
			},
		},
		{
			name: "with insecure SecretRef",
			cfg: func(*fixture) *commoncfg.GRPCClient {
				return &commoncfg.GRPCClient{
					Address:   "localhost:12345",
					SecretRef: &commoncfg.SecretRef{Type: commoncfg.InsecureSecretType},
				}
			},
		},
		{
			name: "with mTLS SecretRef",
			cfg: func(f *fixture) *commoncfg.GRPCClient {
				return f.clientConfig("localhost:12345", nil)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conn := newDynamicClientConn(t, tt.cfg(newFixture(t)))

			assert.True(t, conn.HasClientConn())
			assert.False(t, conn.IsClientConnNil())
		})
	}
}

func TestNewDynamicClientConnErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     func(f *fixture) *commoncfg.GRPCClient
		wantErr error
	}{
		{
			name: "empty address without SecretRef",
			cfg: func(*fixture) *commoncfg.GRPCClient {
				return &commoncfg.GRPCClient{}
			},
			wantErr: commongrpc.ErrEmptyAddress,
		},
		{
			name: "empty address with mTLS SecretRef",
			cfg: func(f *fixture) *commoncfg.GRPCClient {
				return f.clientConfig("", nil)
			},
			wantErr: commongrpc.ErrEmptyAddress,
		},
		{
			name: "unsupported secret type",
			cfg: func(*fixture) *commoncfg.GRPCClient {
				return &commoncfg.GRPCClient{
					Address:   "localhost:12345",
					SecretRef: &commoncfg.SecretRef{Type: "unknown"},
				}
			},
			wantErr: commongrpc.ErrUnsupportedSecretType,
		},
		{
			name: "missing key file",
			cfg: func(f *fixture) *commoncfg.GRPCClient {
				cfg := f.clientConfig("localhost:12345", nil)
				cfg.SecretRef.MTLS.CertKey = fileRef(f.path("missing.key"))

				return cfg
			},
			wantErr: os.ErrNotExist,
		},
		{
			name: "missing directory of the certificate",
			cfg: func(f *fixture) *commoncfg.GRPCClient {
				cfg := f.clientConfig("localhost:12345", nil)
				cfg.SecretRef.MTLS.Cert = fileRef(filepath.Join(f.dir, "missing", certFile))

				return cfg
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			conn, err := commongrpc.NewDynamicClientConn(tt.cfg(newFixture(t)), throttle)
			require.Error(t, err)
			assert.Nil(t, conn)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

// --- Without mutual TLS ---

func TestDynamicClientConnWithoutMTLS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		secretRef *commoncfg.SecretRef
	}{
		{name: "without SecretRef"},
		{name: "with insecure SecretRef", secretRef: &commoncfg.SecretRef{Type: commoncfg.InsecureSecretType}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := newFixture(t).startServer(t, serverOptions{insecure: true})

			// The dial options must be applied as with NewClient.
			var calls atomic.Int64

			countCalls := grpc.WithChainUnaryInterceptor(func(ctx context.Context, method string, req, reply any,
				cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
			) error {
				calls.Add(1)
				return invoker(ctx, method, req, reply, cc, opts...)
			})

			conn := newDynamicClientConn(t, &commoncfg.GRPCClient{Address: srv.addr, SecretRef: tt.secretRef}, countCalls)

			_, err := healthpb.NewHealthClient(conn).Check(t.Context(), &healthpb.HealthCheckRequest{})
			require.NoError(t, err)
			assert.Equal(t, int64(1), calls.Load())
		})
	}
}

// --- Server verification ---

// TestDynamicClientConnVerifiesServerLikeNewClient runs every case through
// NewClient, whose credentials are static, and NewDynamicClientConn, whose
// certificate authorities are reloaded and verified in VerifyConnection. Both
// must accept and reject the same servers.
func TestDynamicClientConnVerifiesServerLikeNewClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// target builds the dial target from the address and port of the
		// test server.
		target      func(addr, port string) string
		ipv6        bool
		attrs       *commoncfg.TLSAttributes
		dialOptions []grpc.DialOption
		// untrusted makes the server present a certificate from a CA the
		// client does not trust.
		untrusted bool
		// wantDialErr is the error expected from the constructor, wantErr
		// the one expected from the RPC.
		wantDialErr string
		wantErr     string
	}{
		{
			name:   "IP address target",
			target: func(addr, _ string) string { return addr },
		},
		{
			name:   "host name target",
			target: func(_, port string) string { return "localhost:" + port },
		},
		{
			name:   "dns scheme with an IP address",
			target: func(addr, _ string) string { return "dns:///" + addr },
		},
		{
			name:   "passthrough scheme with an IP address",
			target: func(addr, _ string) string { return "passthrough:///" + addr },
		},
		{
			name:   "dns scheme with a host name",
			target: func(_, port string) string { return "dns:///localhost:" + port },
		},
		{
			name:   "IPv6 address target",
			target: func(addr, _ string) string { return addr },
			ipv6:   true,
		},
		{
			name:        "authority set to a name the certificate is valid for",
			target:      func(addr, _ string) string { return addr },
			dialOptions: []grpc.DialOption{grpc.WithAuthority("localhost")},
		},
		{
			name:        "authority set to a name the certificate is not valid for",
			target:      func(addr, _ string) string { return addr },
			dialOptions: []grpc.DialOption{grpc.WithAuthority("other.test")},
			wantErr:     "not other.test",
		},
		{
			name:   "server name attribute set to a name the certificate is valid for",
			target: func(addr, _ string) string { return addr },
			attrs:  &commoncfg.TLSAttributes{ServerName: "localhost"},
		},
		{
			name:    "server name attribute set to a name the certificate is not valid for",
			target:  func(addr, _ string) string { return addr },
			attrs:   &commoncfg.TLSAttributes{ServerName: "other.test"},
			wantErr: "not other.test",
		},
		{
			name:        "authority that differs from the server name attribute",
			target:      func(addr, _ string) string { return addr },
			attrs:       &commoncfg.TLSAttributes{ServerName: "localhost"},
			dialOptions: []grpc.DialOption{grpc.WithAuthority("other.test")},
			wantDialErr: "don't match",
		},
		{
			name:      "server certificate from an untrusted CA",
			target:    func(addr, _ string) string { return addr },
			untrusted: true,
			wantErr:   "certificate signed by unknown authority",
		},
	}

	constructors := []struct {
		name string
		dial func(cfg *commoncfg.GRPCClient, opts ...grpc.DialOption) (grpc.ClientConnInterface, func() error, error)
	}{
		{
			name: "NewClient",
			dial: func(cfg *commoncfg.GRPCClient, opts ...grpc.DialOption) (grpc.ClientConnInterface, func() error, error) {
				conn, err := commongrpc.NewClient(cfg, append(opts, fastReconnect())...)
				if err != nil {
					return nil, nil, err
				}

				return conn, conn.Close, nil
			},
		},
		{
			name: "NewDynamicClientConn",
			dial: func(cfg *commoncfg.GRPCClient, opts ...grpc.DialOption) (grpc.ClientConnInterface, func() error, error) {
				conn, err := commongrpc.NewDynamicClientConn(cfg, throttle, append(opts, fastReconnect())...)
				if err != nil {
					return nil, nil, err
				}

				return conn, conn.Close, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := serverOptions{}

			if tt.ipv6 {
				skipWithoutIPv6(t)

				opts.listenAddr = "[::1]:0"
			}

			f := newFixture(t)
			srv := f.startServer(t, opts)

			if tt.untrusted {
				cert := serverCertificate(t, newCA(t, "untrusted-ca", nil))
				srv.cert.Store(&cert)
			}

			for _, c := range constructors {
				t.Run(c.name, func(t *testing.T) {
					cc, closeConn, err := c.dial(f.clientConfig(tt.target(srv.addr, srv.port(t)), tt.attrs), tt.dialOptions...)
					if tt.wantDialErr != "" {
						require.ErrorContains(t, err, tt.wantDialErr)
						return
					}

					require.NoError(t, err)
					t.Cleanup(func() { _ = closeConn() })

					cn, err := clientCN(t.Context(), healthpb.NewHealthClient(cc))
					if tt.wantErr != "" {
						require.ErrorContains(t, err, tt.wantErr)
						return
					}

					require.NoError(t, err)
					assert.Equal(t, "client-v1", cn)
				})
			}
		})
	}
}

func skipWithoutIPv6(t *testing.T) {
	t.Helper()

	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "[::1]:0")
	if err != nil {
		t.Skipf("IPv6 loopback not available: %v", err)
	}

	_ = lis.Close()
}

// --- Rotation ---

func TestDynamicClientConnClientCertificateRotation(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t, serverOptions{maxConnectionAge: shortConnectionAge})
	conn := newDynamicClientConn(t, f.clientConfig(srv.addr, nil))

	// The client is built once, the way generated clients are used, and must
	// keep working across the rotation.
	client := healthpb.NewHealthClient(conn)
	initialConn := conn.ClientConn

	requireClientCNEventually(t, client, "client-v1")

	f.writeClientCert(t, "client-v2")

	requireClientCNEventually(t, client, "client-v2")
	assert.Same(t, initialConn, conn.ClientConn, "the ClientConn must not be replaced")
}

// TestDynamicClientConnClientChainRotation rotates the intermediates of a
// client chain that has two of them, under a root that stays the same: first
// the intermediate that issues the certificate, then the one under the root,
// which means re-issuing everything below it. The server trusts only the
// root, so it accepts the client only if the new intermediates are sent.
func TestDynamicClientConnClientChainRotation(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	top := newCA(t, "client-top-1", f.clientCA)
	issuing := newCA(t, "client-issuing-1", top)
	f.writeClientChain(t, "client-v1", issuing, top)

	srv := f.startServer(t, serverOptions{maxConnectionAge: shortConnectionAge})
	conn := newDynamicClientConn(t, f.clientConfig(srv.addr, nil))
	client := healthpb.NewHealthClient(conn)

	requireClientCNEventually(t, client, "client-v1")

	issuing = newCA(t, "client-issuing-2", top)
	f.writeClientChain(t, "client-v2", issuing, top)
	requireClientCNEventually(t, client, "client-v2")

	top = newCA(t, "client-top-2", f.clientCA)
	issuing = newCA(t, "client-issuing-3", top)
	f.writeClientChain(t, "client-v3", issuing, top)
	requireClientCNEventually(t, client, "client-v3")
}

func TestDynamicClientConnServerCARotation(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t, serverOptions{maxConnectionAge: shortConnectionAge})
	conn := newDynamicClientConn(t, f.clientConfig(srv.addr, nil))
	client := healthpb.NewHealthClient(conn)

	requireClientCNEventually(t, client, "client-v1")

	// The server moves to a certificate from a new CA. Once its current
	// connection has aged out, the client cannot connect until it trusts the
	// new CA.
	newServerCA := newCA(t, "server-ca-v2", nil)
	newServerCert := serverCertificate(t, newServerCA)
	srv.cert.Store(&newServerCert)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		_, err := clientCN(t.Context(), client)
		assert.ErrorContains(c, err, "certificate signed by unknown authority")
	}, waitFor, tick)

	f.writeServerCA(t, newServerCA)

	requireClientCNEventually(t, client, "client-v1")
}

func TestDynamicClientConnRPCsDuringRotationSucceed(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t, serverOptions{maxConnectionAge: shortConnectionAge})
	conn := newDynamicClientConn(t, f.clientConfig(srv.addr, nil))
	client := healthpb.NewHealthClient(conn)

	var (
		wg       sync.WaitGroup
		failures atomic.Int64
		firstErr atomic.Value
	)

	ctx, stop := context.WithCancel(t.Context())

	for range 4 {
		wg.Go(func() {
			for ctx.Err() == nil {
				_, err := clientCN(ctx, client)
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

	for version := 2; version <= 4; version++ {
		commonName := fmt.Sprintf("client-v%d", version)
		f.writeClientCert(t, commonName)
		requireClientCNEventually(t, client, commonName)
	}

	stop()
	wg.Wait()

	if failures.Load() > 0 {
		t.Fatalf("%d RPCs failed during rotation, first error: %v", failures.Load(), firstErr.Load())
	}
}

// TestDynamicClientConnKeepsEstablishedConnections checks that a rotation
// leaves established connections alone: an open stream keeps running, and
// RPCs keep using the certificate the connection was opened with until gRPC
// opens a new connection, here after a server restart. It does not run in
// parallel because it captures the global logger.
func TestDynamicClientConnKeepsEstablishedConnections(t *testing.T) {
	logs := captureLogs(t)

	f := newFixture(t)
	srv := f.startServer(t, serverOptions{})
	conn := newDynamicClientConn(t, f.clientConfig(srv.addr, nil))
	client := healthpb.NewHealthClient(conn)

	stream := openWatch(t, t.Context(), conn)
	requireClientCNEventually(t, client, "client-v1")

	f.writeClientCert(t, "client-v2")
	requireLogEventually(t, logs, "reloaded TLS credentials", f.dir)

	srv.health.SetServingStatus("", healthpb.HealthCheckResponse_NOT_SERVING)

	resp, err := stream.Recv()
	require.NoError(t, err, "the stream must survive the rotation")
	assert.Equal(t, healthpb.HealthCheckResponse_NOT_SERVING, resp.GetStatus())

	cn, err := clientCN(t.Context(), client)
	require.NoError(t, err)
	assert.Equal(t, "client-v1", cn, "the established connection keeps its certificate")

	srv.restart(t)

	requireClientCNEventually(t, client, "client-v2")
}

// TestDynamicClientConnFailedReloadKeepsWorking writes an invalid certificate
// and checks that the new connections opened meanwhile, as the server ages out
// the old ones, still use the previous certificate. It does not run in
// parallel because it captures the global logger.
func TestDynamicClientConnFailedReloadKeepsWorking(t *testing.T) {
	logs := captureLogs(t)

	f := newFixture(t)
	srv := f.startServer(t, serverOptions{maxConnectionAge: shortConnectionAge})
	conn := newDynamicClientConn(t, f.clientConfig(srv.addr, nil))
	client := healthpb.NewHealthClient(conn)

	requireClientCNEventually(t, client, "client-v1")

	writeFile(t, f.path(certFile), []byte("not a certificate"))
	requireLogEventually(t, logs, "reloading TLS credentials failed", f.dir)

	handshakesBefore := srv.handshakes.Load()
	deadline := time.Now().Add(6 * shortConnectionAge)

	for time.Now().Before(deadline) {
		cn, err := clientCN(t.Context(), client)
		require.NoError(t, err)
		require.Equal(t, "client-v1", cn)

		time.Sleep(tick)
	}

	assert.GreaterOrEqual(t, srv.handshakes.Load()-handshakesBefore, int64(2),
		"the client must have opened new connections while the certificate file was invalid")

	f.writeClientCert(t, "client-v2")

	requireClientCNEventually(t, client, "client-v2")
}

// --- Close ---

func TestDynamicClientConnClose(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cfg  func(f *fixture, srv *testServer) *commoncfg.GRPCClient
	}{
		{
			name: "with mTLS SecretRef",
			cfg: func(f *fixture, srv *testServer) *commoncfg.GRPCClient {
				return f.clientConfig(srv.addr, nil)
			},
		},
		{
			name: "without SecretRef",
			cfg: func(*fixture, *testServer) *commoncfg.GRPCClient {
				return &commoncfg.GRPCClient{Address: "localhost:12345"}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			srv := f.startServer(t, serverOptions{})

			conn, err := commongrpc.NewDynamicClientConn(tt.cfg(f, srv), throttle)
			require.NoError(t, err)

			require.NoError(t, conn.Close())
			require.NoError(t, conn.Close(), "Close must be safe to call twice")

			assert.Equal(t, connectivity.Shutdown, conn.GetState())

			_, err = clientCN(t.Context(), healthpb.NewHealthClient(conn))
			assert.Equal(t, codes.Canceled, status.Code(err))
		})
	}
}

func TestDynamicClientConnCloseEndsStreams(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t, serverOptions{})

	conn, err := commongrpc.NewDynamicClientConn(f.clientConfig(srv.addr, nil), throttle)
	require.NoError(t, err)

	stream := openWatch(t, t.Context(), conn)

	require.NoError(t, conn.Close())

	_, err = stream.Recv()
	assert.Equal(t, codes.Canceled, status.Code(err))
}

func TestDynamicClientConnConcurrentClose(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t, serverOptions{})

	conn, err := commongrpc.NewDynamicClientConn(f.clientConfig(srv.addr, nil), throttle)
	require.NoError(t, err)

	var wg sync.WaitGroup

	errs := make([]error, 8)
	for i := range errs {
		wg.Go(func() { errs[i] = conn.Close() })
	}

	wg.Wait()

	for _, err := range errs {
		assert.NoError(t, err)
	}

	assert.Equal(t, connectivity.Shutdown, conn.GetState())
}
