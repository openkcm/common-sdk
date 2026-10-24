package commontls_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commontls"
)

// --- Helpers ---

// testClock is a clock a test can move forward.
type testClock struct {
	now atomic.Pointer[time.Time]
}

func newTestClock() *testClock {
	c := &testClock{}
	now := time.Now()
	c.now.Store(&now)

	return c
}

func (c *testClock) Now() time.Time {
	return *c.now.Load()
}

func (c *testClock) Advance(d time.Duration) {
	now := c.Now().Add(d)
	c.now.Store(&now)
}

// newControlled creates a DynamicTLSConfig that reloads only when the test
// calls Refresh: file events are throttled for an hour and there is no
// periodic check.
func newControlled(t *testing.T, f *fixture, clock *testClock) *commontls.DynamicTLSConfig {
	t.Helper()

	dyn, err := commontls.NewDynamicTLSConfigForTest(time.Hour, 0, f.mtls(withServerName()), clock.Now)
	require.NoError(t, err)
	t.Cleanup(func() { _ = dyn.Close() })

	return dyn
}

// activeClientCN returns the common name of the client certificate dyn
// presents.
func activeClientCN(t *testing.T, dyn *commontls.DynamicTLSConfig) string {
	t.Helper()

	cert, err := dyn.GetClientCertificate(&tls.CertificateRequestInfo{
		Version: tls.VersionTLS13,
		SignatureSchemes: []tls.SignatureScheme{
			tls.ECDSAWithP256AndSHA256, tls.ECDSAWithP384AndSHA384, tls.ECDSAWithP521AndSHA512,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, cert.Leaf, "no client certificate presented")

	return cert.Leaf.Subject.CommonName
}

// trusts reports whether dyn accepts a server certificate issued by ca.
func trusts(t *testing.T, dyn *commontls.DynamicTLSConfig, ca *testCA) bool {
	t.Helper()

	leaf, _ := ca.issue(t, serverTemplate(), elliptic.P256())

	return dyn.VerifyConnection(tls.ConnectionState{ServerName: serverName, PeerCertificates: []*x509.Certificate{leaf}}) == nil
}

// clientCert issues a client certificate for commonName from issuer; change
// adjusts the template first.
func clientCert(t *testing.T, issuer *testCA, commonName string, curve elliptic.Curve,
	change func(*x509.Certificate),
) (*x509.Certificate, *ecdsa.PrivateKey) {
	t.Helper()

	tmpl := clientTemplate(commonName)
	if change != nil {
		change(tmpl)
	}

	return issuer.issue(t, tmpl, curve)
}

// rotateClient issues a client certificate for commonName from issuer (see
// clientCert) and writes it with its key.
func (f *fixture) rotateClient(t *testing.T, issuer *testCA, commonName string, curve elliptic.Curve,
	change func(*x509.Certificate),
) {
	t.Helper()

	cert, key := clientCert(t, issuer, commonName, curve, change)
	f.writeIdentity(t, cert, key)
}

// writeIdentity writes leaf followed by chain to tls.crt, and key to tls.key.
func (f *fixture) writeIdentity(t *testing.T, leaf *x509.Certificate, key *ecdsa.PrivateKey, chain ...*x509.Certificate) {
	t.Helper()

	data := certPEM(leaf)
	for _, cert := range chain {
		data = append(data, certPEM(cert)...)
	}

	writeFile(t, f.path(certFile), data)
	writeFile(t, f.path(keyFile), keyPEM(t, key))
}

// --- Client identity ---

// TestIdentityRotationIsValidated rotates the client identity from a valid
// one to each kind of new material: valid material is activated, material
// with a defect the active identity does not have is rejected and the active
// one stays in use.
func TestIdentityRotationIsValidated(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name string
		// write writes the new identity, for client-v2.
		write      func(t *testing.T, f *fixture)
		wantActive string
	}{
		{
			name: "valid certificate",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				f.rotateClient(t, f.clientCA, "client-v2", elliptic.P256(), nil)
			},
			wantActive: "client-v2",
		},
		{
			name: "certificate without extended key usage",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				f.rotateClient(t, f.clientCA, "client-v2", elliptic.P256(),
					func(c *x509.Certificate) { c.ExtKeyUsage = nil })
			},
			wantActive: "client-v2",
		},
		{
			name: "certificate valid within the clock skew",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				f.rotateClient(t, f.clientCA, "client-v2", elliptic.P256(),
					func(c *x509.Certificate) { c.NotBefore = now.Add(2 * time.Minute) })
			},
			wantActive: "client-v2",
		},
		{
			name: "expired certificate",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				f.rotateClient(t, f.clientCA, "client-v2", elliptic.P256(),
					func(c *x509.Certificate) { c.NotBefore, c.NotAfter = now.Add(-2*time.Hour), now.Add(-time.Hour) })
			},
			wantActive: "client-v1",
		},
		{
			name: "certificate not valid yet",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				f.rotateClient(t, f.clientCA, "client-v2", elliptic.P256(),
					func(c *x509.Certificate) { c.NotBefore, c.NotAfter = now.Add(time.Hour), now.Add(2*time.Hour) })
			},
			wantActive: "client-v1",
		},
		{
			name: "certificate for server authentication only",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				f.rotateClient(t, f.clientCA, "client-v2", elliptic.P256(),
					func(c *x509.Certificate) { c.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth} })
			},
			wantActive: "client-v1",
		},
		{
			name: "weak key",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				f.rotateClient(t, f.clientCA, "client-v2", elliptic.P224(), nil)
			},
			wantActive: "client-v1",
		},
		{
			name: "key that does not match the certificate",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				cert, _ := clientCert(t, f.clientCA, "client-v2", elliptic.P256(), nil)
				_, otherKey := clientCert(t, f.clientCA, "other", elliptic.P256(), nil)
				f.writeIdentity(t, cert, otherKey)
			},
			wantActive: "client-v1",
		},
		{
			name: "certificate file that is not PEM",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				writeFile(t, f.path(certFile), []byte("not a certificate"))
			},
			wantActive: "client-v1",
		},
		{
			name: "chain with an intermediate that cannot be parsed",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				cert, key := clientCert(t, f.clientCA, "client-v2", elliptic.P256(), nil)
				f.writeIdentity(t, cert, key)

				broken := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not DER")})
				writeFile(t, f.path(certFile), append(certPEM(cert), broken...))
			},
			wantActive: "client-v1",
		},
		{
			name: "chain with intermediates in reverse order",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				top := newCA(t, "top", f.clientCA, elliptic.P256())
				issuing := newCA(t, "issuing", top, elliptic.P256())
				cert, key := clientCert(t, issuing, "client-v2", elliptic.P256(), nil)
				f.writeIdentity(t, cert, key, top.cert, issuing.cert)
			},
			wantActive: "client-v2",
		},
		{
			name: "chain including its root",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				issuing := newCA(t, "issuing", f.clientCA, elliptic.P256())
				cert, key := clientCert(t, issuing, "client-v2", elliptic.P256(), nil)
				f.writeIdentity(t, cert, key, issuing.cert, f.clientCA.cert)
			},
			wantActive: "client-v2",
		},
		{
			name: "chain with an expired intermediate",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				issuing := newCAValid(t, "issuing", f.clientCA, elliptic.P256(), now.Add(-2*time.Hour), now.Add(-time.Hour))
				cert, key := clientCert(t, issuing, "client-v2", elliptic.P256(), nil)
				f.writeIdentity(t, cert, key, issuing.cert)
			},
			wantActive: "client-v1",
		},
		{
			name: "chain with an intermediate that did not issue the certificate",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				issuing := newCA(t, "issuing", f.clientCA, elliptic.P256())
				unrelated := newCA(t, "unrelated", f.clientCA, elliptic.P256())
				cert, key := clientCert(t, issuing, "client-v2", elliptic.P256(), nil)
				f.writeIdentity(t, cert, key, unrelated.cert)
			},
			wantActive: "client-v1",
		},
		{
			name: "chain with a weak intermediate",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				issuing := newCA(t, "issuing", f.clientCA, elliptic.P224())
				cert, key := clientCert(t, issuing, "client-v2", elliptic.P256(), nil)
				f.writeIdentity(t, cert, key, issuing.cert)
			},
			wantActive: "client-v1",
		},
		{
			name: "chain whose intermediate is not a certificate authority",
			write: func(t *testing.T, f *fixture) {
				t.Helper()

				notCA, notCAKey := clientCert(t, f.clientCA, "not-a-ca", elliptic.P256(), nil)
				issuer := &testCA{cert: notCA, key: notCAKey}
				cert, key := clientCert(t, issuer, "client-v2", elliptic.P256(), nil)
				f.writeIdentity(t, cert, key, notCA)
			},
			wantActive: "client-v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			dyn := newControlled(t, f, newTestClock())
			require.Equal(t, "client-v1", activeClientCN(t, dyn))

			tt.write(t, f)
			dyn.Refresh()

			assert.Equal(t, tt.wantActive, activeClientCN(t, dyn))
		})
	}
}

// TestIdentityRotationNeverRegresses covers active identities that already
// have defects, which creation accepts: a reload may keep a defect, may not
// add one, and may repair an identity that no longer works.
func TestIdentityRotationNeverRegresses(t *testing.T) {
	t.Parallel()

	now := time.Now()
	expired := func(c *x509.Certificate) { c.NotBefore, c.NotAfter = now.Add(-3*time.Hour), now.Add(-time.Hour) }

	t.Run("an expired identity is replaced by a valid one", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		f.rotateClient(t, f.clientCA, "client-v1", elliptic.P256(), expired)

		dyn := newControlled(t, f, newTestClock())
		require.Equal(t, "client-v1", activeClientCN(t, dyn), "creation must not fail on an expired certificate")

		f.rotateClient(t, f.clientCA, "client-v2", elliptic.P256(), nil)
		dyn.Refresh()

		assert.Equal(t, "client-v2", activeClientCN(t, dyn))
	})

	t.Run("an identity with an extra certificate is replaced by one with the same defect but not a new one", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		extra := newCA(t, "extra", nil, elliptic.P256())

		cert, key := clientCert(t, f.clientCA, "client-v1", elliptic.P256(), nil)
		f.writeIdentity(t, cert, key, extra.cert)

		dyn := newControlled(t, f, newTestClock())

		cert, key = clientCert(t, f.clientCA, "client-v2", elliptic.P256(), nil)
		f.writeIdentity(t, cert, key, extra.cert)
		dyn.Refresh()
		require.Equal(t, "client-v2", activeClientCN(t, dyn))

		cert, key = clientCert(t, f.clientCA, "client-v3", elliptic.P224(), nil)
		f.writeIdentity(t, cert, key, extra.cert)
		dyn.Refresh()
		assert.Equal(t, "client-v2", activeClientCN(t, dyn))
	})

	t.Run("an identity that expired meanwhile is replaced by one with a minor defect", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		clock := newTestClock()
		dyn := newControlled(t, f, clock)

		clock.Advance(2 * time.Hour) // client-v1 is valid for an hour only

		extra := newCA(t, "extra", nil, elliptic.P256())
		cert, key := clientCert(t, f.clientCA, "client-v2", elliptic.P256(), func(c *x509.Certificate) {
			c.NotBefore, c.NotAfter = now.Add(time.Hour), now.Add(4*time.Hour)
		})
		f.writeIdentity(t, cert, key, extra.cert)
		dyn.Refresh()

		assert.Equal(t, "client-v2", activeClientCN(t, dyn))
	})

	t.Run("a certificate rejected as not valid yet is activated once it is", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		clock := newTestClock()
		dyn := newControlled(t, f, clock)

		f.rotateClient(t, f.clientCA, "client-v2", elliptic.P256(), func(c *x509.Certificate) {
			c.NotBefore, c.NotAfter = now.Add(30*time.Minute), now.Add(3*time.Hour)
		})
		dyn.Refresh()
		require.Equal(t, "client-v1", activeClientCN(t, dyn))

		clock.Advance(40 * time.Minute)
		dyn.Refresh()

		assert.Equal(t, "client-v2", activeClientCN(t, dyn))
	})
}

// TestUnreadableFilesKeepTheActiveCredentials removes the key and the CA file,
// as a broken volume would: the active credentials stay in use, and the next
// readable files are activated.
func TestUnreadableFilesKeepTheActiveCredentials(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	dyn := newControlled(t, f, newTestClock())

	require.NoError(t, os.Remove(f.path(keyFile)))
	require.NoError(t, os.Remove(f.path(caFile)))
	dyn.Refresh()

	assert.Equal(t, "client-v1", activeClientCN(t, dyn))
	assert.True(t, trusts(t, dyn, f.serverCA))

	newServerCA := newCA(t, "server-ca-v2", nil, elliptic.P256())
	f.writeClientCert(t, "client-v2")
	f.writeServerCA(t, newServerCA)
	dyn.Refresh()

	assert.Equal(t, "client-v2", activeClientCN(t, dyn))
	assert.True(t, trusts(t, dyn, newServerCA))
}

func TestUnchangedFilesKeepTheActiveIdentity(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	dyn := newControlled(t, f, newTestClock())

	request := &tls.CertificateRequestInfo{Version: tls.VersionTLS13, SignatureSchemes: []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256}}

	before, err := dyn.GetClientCertificate(request)
	require.NoError(t, err)

	dyn.Refresh()

	after, err := dyn.GetClientCertificate(request)
	require.NoError(t, err)
	assert.Same(t, before, after)
}

// --- Certificate authorities ---

// TestAuthoritiesRotationIsValidated rotates the certificate authorities from
// the fixture's server CA to each kind of new file, and checks which CAs are
// trusted afterwards.
func TestAuthoritiesRotationIsValidated(t *testing.T) {
	t.Parallel()

	now := time.Now()

	tests := []struct {
		name string
		// content builds the new CA file from the old and the new CA.
		content       func(t *testing.T, oldCA, newCA *testCA) []byte
		wantTrustsOld bool
		wantTrustsNew bool
	}{
		{
			name:          "new authority",
			content:       func(_ *testing.T, _, newCA *testCA) []byte { return certPEM(newCA.cert) },
			wantTrustsNew: true,
		},
		{
			name: "old and new authorities, for an overlap",
			content: func(_ *testing.T, oldCA, newCA *testCA) []byte {
				return append(certPEM(oldCA.cert), certPEM(newCA.cert)...)
			},
			wantTrustsOld: true,
			wantTrustsNew: true,
		},
		{
			name: "comment lines between blocks",
			content: func(_ *testing.T, _, newCA *testCA) []byte {
				return append([]byte("# Issuer: new CA\n\n"), certPEM(newCA.cert)...)
			},
			wantTrustsNew: true,
		},
		{
			name: "Windows line endings",
			content: func(_ *testing.T, _, newCA *testCA) []byte {
				return []byte(strings.ReplaceAll(string(certPEM(newCA.cert)), "\n", "\r\n"))
			},
			wantTrustsNew: true,
		},
		{
			name: "file cut short while being written",
			content: func(_ *testing.T, oldCA, newCA *testCA) []byte {
				whole := append(certPEM(oldCA.cert), certPEM(newCA.cert)...)
				return whole[:len(whole)-40]
			},
			wantTrustsOld: true,
		},
		{
			name: "text between blocks",
			content: func(_ *testing.T, _, newCA *testCA) []byte {
				return append([]byte("Certificate:\n    Data: ...\n"), certPEM(newCA.cert)...)
			},
			wantTrustsOld: true,
		},
		{
			name:          "empty file",
			content:       func(*testing.T, *testCA, *testCA) []byte { return nil },
			wantTrustsOld: true,
		},
		{
			name: "private key instead of a certificate",
			content: func(t *testing.T, _, newCA *testCA) []byte {
				t.Helper()

				return keyPEM(t, newCA.key)
			},
			wantTrustsOld: true,
		},
		{
			name: "block with headers",
			content: func(_ *testing.T, _, newCA *testCA) []byte {
				return pem.EncodeToMemory(&pem.Block{
					Type: "CERTIFICATE", Headers: map[string]string{"Proc-Type": "4,ENCRYPTED"}, Bytes: newCA.cert.Raw,
				})
			},
			wantTrustsOld: true,
		},
		{
			name: "only expired authorities",
			content: func(t *testing.T, _, _ *testCA) []byte {
				t.Helper()

				return certPEM(newCAValid(t, "expired-ca", nil, elliptic.P256(), now.Add(-2*time.Hour), now.Add(-time.Hour)).cert)
			},
			wantTrustsOld: true,
		},
		{
			name: "weak authority key",
			content: func(t *testing.T, _, _ *testCA) []byte {
				t.Helper()

				return certPEM(newCA(t, "weak-ca", nil, elliptic.P224()).cert)
			},
			wantTrustsOld: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t)
			dyn := newControlled(t, f, newTestClock())
			newServerCA := newCA(t, "server-ca-v2", nil, elliptic.P256())

			writeFile(t, f.path(caFile), tt.content(t, f.serverCA, newServerCA))
			dyn.Refresh()

			assert.Equal(t, tt.wantTrustsOld, trusts(t, dyn, f.serverCA), "trusts the old CA")
			assert.Equal(t, tt.wantTrustsNew, trusts(t, dyn, newServerCA), "trusts the new CA")
		})
	}
}

// TestAuthoritiesNeverFallBackToSystemPool starts with a CA file that holds
// no certificate. Creation accepts it, but the client must then trust no
// server at all, rather than the system authorities a nil pool would mean.
func TestAuthoritiesNeverFallBackToSystemPool(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	writeFile(t, f.path(caFile), []byte("# no certificate yet\n"))

	dyn := newControlled(t, f, newTestClock())

	roots := dyn.X509CertPool()
	require.NotNil(t, roots, "a nil pool would trust the system authorities")
	assert.True(t, roots.Equal(x509.NewCertPool()), "the pool must be empty")
	assert.False(t, trusts(t, dyn, f.serverCA))

	writeFile(t, f.path(caFile), certPEM(f.serverCA.cert))
	dyn.Refresh()

	assert.True(t, trusts(t, dyn, f.serverCA), "a usable CA file must replace the empty one")
}

// --- Both parts ---

// TestPartsAreActivatedIndependently writes a rejected file for one part and
// a valid one for the other in the same reload.
func TestPartsAreActivatedIndependently(t *testing.T) {
	t.Parallel()

	now := time.Now()

	t.Run("rejected identity, activated authorities", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		dyn := newControlled(t, f, newTestClock())
		newServerCA := newCA(t, "server-ca-v2", nil, elliptic.P256())

		f.rotateClient(t, f.clientCA, "client-v2", elliptic.P256(), func(c *x509.Certificate) {
			c.NotBefore, c.NotAfter = now.Add(-2*time.Hour), now.Add(-time.Hour)
		})
		f.writeServerCA(t, newServerCA)
		dyn.Refresh()

		assert.Equal(t, "client-v1", activeClientCN(t, dyn))
		assert.True(t, trusts(t, dyn, newServerCA))
	})

	t.Run("activated identity, rejected authorities", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		dyn := newControlled(t, f, newTestClock())

		f.writeClientCert(t, "client-v2")
		writeFile(t, f.path(caFile), certPEM(newCA(t, "server-ca-v2", nil, elliptic.P256()).cert)[:100])
		dyn.Refresh()

		assert.Equal(t, "client-v2", activeClientCN(t, dyn))
		assert.True(t, trusts(t, dyn, f.serverCA))
	})
}

// TestPeriodicCheckPicksUpMissedChanges throttles file events for an hour, so
// that only the periodic check can see the rotation.
func TestPeriodicCheckPicksUpMissedChanges(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	dyn, err := commontls.NewDynamicTLSConfigForTest(time.Hour, 20*time.Millisecond, f.mtls(withServerName()), time.Now)
	require.NoError(t, err)
	t.Cleanup(func() { _ = dyn.Close() })

	newServerCA := newCA(t, "server-ca-v2", nil, elliptic.P256())
	f.writeClientCert(t, "client-v2")
	f.writeServerCA(t, newServerCA)

	require.Eventually(t, func() bool {
		return activeClientCN(t, dyn) == "client-v2" && trusts(t, dyn, newServerCA)
	}, waitFor, tick)

	// After Close, the periodic check stops.
	require.NoError(t, dyn.Close())
	f.writeClientCert(t, "client-v3")
	time.Sleep(10 * 20 * time.Millisecond)

	assert.Equal(t, "client-v2", activeClientCN(t, dyn))
}

// TestRejectionsAreLoggedOnce reloads a rejected file several times, as the
// periodic check does every minute: it is logged once, and logged again only
// for a different file. It also checks that an active certificate expiring is
// logged once. It does not run in parallel because it captures the global
// logger.
func TestRejectionsAreLoggedOnce(t *testing.T) {
	logs := captureLogs(t)

	f := newFixture(t)
	clock := newTestClock()
	dyn := newControlled(t, f, clock)

	weak := func(cn string) {
		f.rotateClient(t, f.clientCA, cn, elliptic.P224(), nil)
	}

	weak("weak-1")

	for range 3 {
		dyn.Refresh()
	}

	assert.Equal(t, 1, strings.Count(logs.String(), "rejected reloaded TLS credentials"))

	weak("weak-2")
	dyn.Refresh()
	dyn.Refresh()

	assert.Equal(t, 2, strings.Count(logs.String(), "rejected reloaded TLS credentials"))

	// The fixture's client certificate and server CA are both valid for an
	// hour only: each part is reported once.
	clock.Advance(2 * time.Hour)

	for range 3 {
		dyn.Refresh()
	}

	for _, part := range []string{`part="client certificate"`, `part="certificate authorities"`} {
		lines := 0

		for line := range strings.Lines(logs.String()) {
			if strings.Contains(line, "the active TLS credentials no longer work") && strings.Contains(line, part) {
				lines++
			}
		}

		assert.Equal(t, 1, lines, "expiry of %s logged once", part)
	}
}

// --- End to end ---

// TestHandshakesSurviveInvalidRotations rotates through valid and invalid
// identities while handshakes run: invalid ones are rejected, so every
// handshake succeeds, and the last valid identity ends up active.
func TestHandshakesSurviveInvalidRotations(t *testing.T) {
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

	stop := make(chan struct{})

	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}

				_, err := get(t.Context(), client, srv.URL)
				if err != nil {
					failures.Add(1)
					firstErr.CompareAndSwap(nil, err)
				}
			}
		})
	}

	t.Cleanup(func() {
		select {
		case <-stop:
		default:
			close(stop)
		}

		wg.Wait()
	})

	now := time.Now()

	for version := 2; version <= 5; version++ {
		// An invalid identity first, then a valid one.
		f.rotateClient(t, f.clientCA, fmt.Sprintf("expired-%d", version), elliptic.P256(),
			func(c *x509.Certificate) { c.NotBefore, c.NotAfter = now.Add(-2*time.Hour), now.Add(-time.Hour) })
		time.Sleep(5 * throttle)

		commonName := fmt.Sprintf("client-v%d", version)
		f.writeClientCert(t, commonName)
		requireClientCertEventually(t, client, srv.URL, commonName)
	}

	close(stop)
	wg.Wait()

	if failures.Load() > 0 {
		t.Fatalf("%d handshakes failed, first error: %v", failures.Load(), firstErr.Load())
	}
}

// --- Logs ---

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

// --- Every file, in any layout ---

// role is one of the credential files.
type role string

const (
	roleCert     role = "tls.crt"
	roleKey      role = "tls.key"
	roleServerCA role = "ca.crt"
	roleRoot1    role = "root1.crt"
	roleRoot2    role = "root2.crt"
)

// credentialFiles places the credential files in a layout and changes them
// the way that layout is updated in practice.
type credentialFiles interface {
	path(r role) string
	// write changes the given files. Files that are updated together in
	// practice, such as all the files of a Kubernetes volume, change at once.
	write(t *testing.T, contents map[role][]byte)
}

// plainFiles are regular files, at paths chosen by the layout.
type plainFiles map[role]string

func (p plainFiles) path(r role) string {
	return p[r]
}

func (p plainFiles) write(t *testing.T, contents map[role][]byte) {
	t.Helper()

	for r, data := range contents {
		writeFile(t, p[r], data)
	}
}

// kubernetesFiles keep the client chain and key in one Kubernetes volume, a
// secret, and the certificate authorities in another, a config map. The
// kubelet updates a volume by writing its files into a new timestamped
// directory and swapping the ..data link to it.
type kubernetesFiles struct {
	identity, authorities *kubernetesVolume
}

func (k kubernetesFiles) volume(r role) *kubernetesVolume {
	if r == roleCert || r == roleKey {
		return k.identity
	}

	return k.authorities
}

func (k kubernetesFiles) path(r role) string {
	return filepath.Join(k.volume(r).dir, string(r))
}

func (k kubernetesFiles) write(t *testing.T, contents map[role][]byte) {
	t.Helper()

	for _, v := range []*kubernetesVolume{k.identity, k.authorities} {
		changes := map[role][]byte{}

		for r, data := range contents {
			if k.volume(r) == v {
				changes[r] = data
			}
		}

		if len(changes) > 0 {
			v.update(t, changes)
		}
	}
}

// kubernetesVolume is a directory laid out the way the kubelet lays out a
// secret or config map volume.
type kubernetesVolume struct {
	dir     string
	version int
	current string
	files   map[role][]byte
}

func newKubernetesVolume(t *testing.T, files map[role][]byte) *kubernetesVolume {
	t.Helper()

	v := &kubernetesVolume{dir: t.TempDir(), files: map[role][]byte{}}
	v.update(t, files)

	for r := range files {
		require.NoError(t, os.Symlink(filepath.Join("..data", string(r)), filepath.Join(v.dir, string(r))))
	}

	return v
}

// update writes a new version of the volume with changes applied, swaps
// ..data to it and removes the previous version, in the kubelet's order.
func (v *kubernetesVolume) update(t *testing.T, changes map[role][]byte) {
	t.Helper()

	maps.Copy(v.files, changes)

	v.version++
	versionDir := filepath.Join(v.dir, fmt.Sprintf("..2026_%02d", v.version))
	require.NoError(t, os.Mkdir(versionDir, 0o700))

	for r, data := range v.files {
		writeFile(t, filepath.Join(versionDir, string(r)), data)
	}

	tmpLink := filepath.Join(v.dir, "..data_tmp")
	require.NoError(t, os.Symlink(filepath.Base(versionDir), tmpLink))
	require.NoError(t, os.Rename(tmpLink, filepath.Join(v.dir, "..data")))

	if v.current != "" {
		require.NoError(t, os.RemoveAll(v.current))
	}

	v.current = versionDir
}

// layouts are the ways the credential files can be spread over directories.
func layouts() []struct {
	name  string
	place func(t *testing.T, initial map[role][]byte) credentialFiles
} {
	plain := func(dirOf func(t *testing.T, r role) string) func(*testing.T, map[role][]byte) credentialFiles {
		return func(t *testing.T, initial map[role][]byte) credentialFiles {
			t.Helper()

			files := plainFiles{}
			for r := range initial {
				files[r] = filepath.Join(dirOf(t, r), string(r))
			}

			files.write(t, initial)

			return files
		}
	}

	return []struct {
		name  string
		place func(t *testing.T, initial map[role][]byte) credentialFiles
	}{
		{
			name: "every file in its own directory",
			place: plain(func(t *testing.T, _ role) string {
				t.Helper()
				return t.TempDir()
			}),
		},
		{
			name: "client files in one directory, all authorities in another",
			place: func() func(*testing.T, map[role][]byte) credentialFiles {
				return func(t *testing.T, initial map[role][]byte) credentialFiles {
					t.Helper()

					identityDir, authoritiesDir := t.TempDir(), t.TempDir()

					return plain(func(_ *testing.T, r role) string {
						if r == roleCert || r == roleKey {
							return identityDir
						}

						return authoritiesDir
					})(t, initial)
				}
			}(),
		},
		{
			name: "every file in the same directory",
			place: func(t *testing.T, initial map[role][]byte) credentialFiles {
				t.Helper()

				dir := t.TempDir()

				return plain(func(*testing.T, role) string { return dir })(t, initial)
			},
		},
		{
			name: "Kubernetes secret for the client files, config map for the authorities",
			place: func(t *testing.T, initial map[role][]byte) credentialFiles {
				t.Helper()

				identity, authorities := map[role][]byte{}, map[role][]byte{}

				for r, data := range initial {
					if r == roleCert || r == roleKey {
						identity[r] = data
					} else {
						authorities[r] = data
					}
				}

				return kubernetesFiles{
					identity:    newKubernetesVolume(t, identity),
					authorities: newKubernetesVolume(t, authorities),
				}
			},
		},
	}
}

// clientChainFiles returns a client certificate for commonName issued by a
// new intermediate under clientCA, followed by that intermediate, and its
// key.
func clientChainFiles(t *testing.T, clientCA *testCA, commonName string) ([]byte, []byte) {
	t.Helper()

	intermediate := newCA(t, commonName+"-intermediate", clientCA, elliptic.P256())
	cert, key := clientCert(t, intermediate, commonName, elliptic.P256(), nil)

	return append(certPEM(cert), certPEM(intermediate.cert)...), keyPEM(t, key)
}

// TestEveryCredentialFileIsFollowedInAnyLayout gives each role its own file:
// the client chain (with an intermediate), its key, the server CA and two
// root CA files. In every layout, each file is rotated on its own, then all of
// them at once, with the real watcher, and every rotation must reach a client
// built once at the start.
func TestEveryCredentialFileIsFollowedInAnyLayout(t *testing.T) {
	t.Parallel()

	for _, layout := range layouts() {
		t.Run(layout.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t) // for its client CA
			caA, caB, caC := newCA(t, "ca-a", nil, elliptic.P256()), newCA(t, "ca-b", nil, elliptic.P256()),
				newCA(t, "ca-c", nil, elliptic.P256())
			chain, key := clientChainFiles(t, f.clientCA, "client-v1")

			files := layout.place(t, map[role][]byte{
				roleCert:     chain,
				roleKey:      key,
				roleServerCA: certPEM(caA.cert),
				roleRoot1:    certPEM(caB.cert),
				roleRoot2:    certPEM(caC.cert),
			})

			serverCA := fileRef(files.path(roleServerCA))
			mtls := &commoncfg.MTLS{
				Cert:       fileRef(files.path(roleCert)),
				CertKey:    fileRef(files.path(roleKey)),
				ServerCA:   &serverCA,
				RootCAs:    []commoncfg.SourceRef{fileRef(files.path(roleRoot1)), fileRef(files.path(roleRoot2))},
				Attributes: withServerName(),
			}

			dyn, err := commontls.NewDynamicTLSConfig(throttle, mtls)
			require.NoError(t, err)
			t.Cleanup(func() { _ = dyn.Close() })

			requireTrust := func(trusted []*testCA, untrusted ...*testCA) {
				t.Helper()

				require.EventuallyWithT(t, func(c *assert.CollectT) {
					for _, ca := range trusted {
						assert.True(c, trusts(t, dyn, ca), "must trust %s", ca.cert.Subject.CommonName)
					}

					for _, ca := range untrusted {
						assert.False(c, trusts(t, dyn, ca), "must not trust %s", ca.cert.Subject.CommonName)
					}
				}, waitFor, tick)
			}

			requireTrust([]*testCA{caA, caB, caC})
			require.Equal(t, "client-v1", activeClientCN(t, dyn))

			// Each file on its own.
			caD := newCA(t, "ca-d", nil, elliptic.P256())
			files.write(t, map[role][]byte{roleRoot2: certPEM(caD.cert)})
			requireTrust([]*testCA{caA, caB, caD}, caC)

			caE := newCA(t, "ca-e", nil, elliptic.P256())
			files.write(t, map[role][]byte{roleServerCA: certPEM(caE.cert)})
			requireTrust([]*testCA{caE, caB, caD}, caA)

			caF := newCA(t, "ca-f", nil, elliptic.P256())
			files.write(t, map[role][]byte{roleRoot1: certPEM(caF.cert)})
			requireTrust([]*testCA{caE, caF, caD}, caB)

			chain, key = clientChainFiles(t, f.clientCA, "client-v2")
			files.write(t, map[role][]byte{roleCert: chain, roleKey: key})
			require.Eventually(t, func() bool { return activeClientCN(t, dyn) == "client-v2" }, waitFor, tick)

			// Everything at once, checked end to end through a client built
			// here: the server presents a certificate from the current server
			// CA and requires a client certificate from the client CA.
			srv := startServer(t, f.serverCertificate(t, caE), f.clientCA.cert)
			client := newClient(dyn.Config, "")
			requireClientCertEventually(t, client, srv.URL, "client-v2")

			caG, caH, caI := newCA(t, "ca-g", nil, elliptic.P256()), newCA(t, "ca-h", nil, elliptic.P256()),
				newCA(t, "ca-i", nil, elliptic.P256())
			chain, key = clientChainFiles(t, f.clientCA, "client-v3")

			files.write(t, map[role][]byte{
				roleCert:     chain,
				roleKey:      key,
				roleServerCA: certPEM(caG.cert),
				roleRoot1:    certPEM(caH.cert),
				roleRoot2:    certPEM(caI.cert),
			})

			newServerCert := f.serverCertificate(t, caG)
			srv.cert.Store(&newServerCert)

			requireTrust([]*testCA{caG, caH, caI}, caE, caF, caD)
			requireClientCertEventually(t, client, srv.URL, "client-v3")
		})
	}
}

// --- x509CertPool ---

func TestX509CertPool(t *testing.T) {
	t.Parallel()

	poolOf := func(cas ...*testCA) *x509.CertPool {
		pool := x509.NewCertPool()
		for _, ca := range cas {
			pool.AddCert(ca.cert)
		}

		return pool
	}

	t.Run("authorities from files follow rotations", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		dyn := newControlled(t, f, newTestClock())

		before := dyn.X509CertPool()
		require.True(t, before.Equal(poolOf(f.serverCA)))

		newServerCA := newCA(t, "server-ca-v2", nil, elliptic.P256())
		f.writeServerCA(t, newServerCA)
		dyn.Refresh()

		assert.True(t, dyn.X509CertPool().Equal(poolOf(newServerCA)), "a new call returns the rotated authorities")
		assert.True(t, before.Equal(poolOf(f.serverCA)), "an earlier copy is a snapshot")
	})

	t.Run("changing the copy does not change what is trusted", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		dyn := newControlled(t, f, newTestClock())

		attacker := newCA(t, "attacker-ca", nil, elliptic.P256())
		dyn.X509CertPool().AddCert(attacker.cert)

		assert.False(t, trusts(t, dyn, attacker))
		assert.True(t, dyn.X509CertPool().Equal(poolOf(f.serverCA)))
	})

	t.Run("embedded authorities are returned as read at creation", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		mtls := f.mtls(withServerName())
		embedded := commoncfg.SourceRef{Source: commoncfg.EmbeddedSourceValue, Value: string(certPEM(f.serverCA.cert))}
		mtls.ServerCA = &embedded

		dyn, err := commontls.NewDynamicTLSConfig(throttle, mtls)
		require.NoError(t, err)
		t.Cleanup(func() { _ = dyn.Close() })

		roots := dyn.X509CertPool()
		require.NotNil(t, roots)
		assert.True(t, roots.Equal(poolOf(f.serverCA)))
		assert.NotSame(t, dyn.RootCAs, roots, "the field itself is not handed out")
	})

	t.Run("no authorities configured means the system authorities", func(t *testing.T) {
		t.Parallel()

		f := newFixture(t)
		mtls := f.mtls(withServerName())
		mtls.ServerCA = nil

		dyn, err := commontls.NewDynamicTLSConfig(throttle, mtls)
		require.NoError(t, err)
		t.Cleanup(func() { _ = dyn.Close() })

		assert.Nil(t, dyn.X509CertPool())
	})
}
