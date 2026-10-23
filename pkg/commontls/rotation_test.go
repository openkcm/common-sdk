package commontls_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	roots := dyn.ActiveRootsForTest()
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
