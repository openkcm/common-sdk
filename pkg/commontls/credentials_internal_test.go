package commontls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestCert creates a certificate signed by parent (self-signed when nil),
// returning it with its key.
func newTestCert(tb testing.TB, name string, isCA bool, parent *x509.Certificate, parentKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey) {
	tb.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(tb, err)

	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(time.Now().UnixNano()),
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  isCA,
		BasicConstraintsValid: true,
	}
	if isCA {
		tmpl.KeyUsage = x509.KeyUsageCertSign
	}

	if parent == nil {
		parent, parentKey = tmpl, key
	}

	der, err := x509.CreateCertificate(rand.Reader, tmpl, parent, key.Public(), parentKey)
	require.NoError(tb, err)

	cert, err := x509.ParseCertificate(der)
	require.NoError(tb, err)

	return cert, key
}

func encodeCert(cert *x509.Certificate) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: pemCertificateType, Bytes: cert.Raw}))
}

func TestParsePEMCertificates(t *testing.T) {
	t.Parallel()

	first, _ := newTestCert(t, "first", true, nil, nil)
	second, _ := newTestCert(t, "second", true, nil, nil)
	one, two := encodeCert(first), encodeCert(second)

	tests := []struct {
		name         string
		data         string
		wantCerts    int
		wantProblems []problem
	}{
		{name: "one certificate", data: one, wantCerts: 1},
		{name: "two certificates", data: one + two, wantCerts: 2},
		{name: "blank and comment lines", data: "# bundle\n\n" + one + "\n# second\n" + two + "\n", wantCerts: 2},
		{name: "Windows line endings", data: strings.ReplaceAll(one+two, "\n", "\r\n"), wantCerts: 2},
		{name: "empty"},
		{name: "blank lines only", data: "\n \n\t\n"},
		{
			name:         "text before a block",
			data:         "Certificate:\n" + one,
			wantCerts:    1,
			wantProblems: []problem{problemTextOutsidePEM},
		},
		{
			name:         "text after the last block",
			data:         one + "trailing\n",
			wantCerts:    1,
			wantProblems: []problem{problemTextOutsidePEM},
		},
		{
			name:         "last block cut short",
			data:         one + two[:len(two)/2],
			wantCerts:    1,
			wantProblems: []problem{problemMalformedPEM},
		},
		{
			name:         "block cut short before another block",
			data:         one[:len(one)/2] + "\n" + two,
			wantProblems: []problem{problemMalformedPEM},
		},
		{
			name:         "private key block",
			data:         "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n" + one,
			wantCerts:    1,
			wantProblems: []problem{problemMalformedPEM},
		},
		{
			name: "block with headers",
			data: string(pem.EncodeToMemory(&pem.Block{
				Type: pemCertificateType, Headers: map[string]string{"Proc-Type": "4,ENCRYPTED"}, Bytes: first.Raw,
			})),
			wantProblems: []problem{problemMalformedPEM},
		},
		{
			name:         "invalid base64",
			data:         "-----BEGIN CERTIFICATE-----\n!!!!\n-----END CERTIFICATE-----\n" + one,
			wantCerts:    1,
			wantProblems: []problem{problemMalformedPEM},
		},
		{
			name: "valid base64 that is not a certificate",
			data: "-----BEGIN CERTIFICATE-----\n" + base64.StdEncoding.EncodeToString([]byte("not DER")) +
				"\n-----END CERTIFICATE-----\n",
			wantProblems: []problem{problemMalformedPEM},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			certs, problems := parsePEMCertificates([]byte(tt.data))

			assert.Len(t, certs, tt.wantCerts)
			assert.Equal(t, tt.wantProblems, normalize(problems))
		})
	}
}

// FuzzParsePEMCertificates checks that no input makes the parser panic, and
// that it only ever returns certificates it parsed, never more than there are
// PEM blocks.
func FuzzParsePEMCertificates(f *testing.F) {
	cert, _ := newTestCert(f, "fuzz", true, nil, nil)
	valid := encodeCert(cert)

	for _, seed := range []string{
		"", valid, valid + valid, "# comment\n" + valid, valid[:len(valid)/2],
		"-----BEGIN CERTIFICATE-----\n-----BEGIN CERTIFICATE-----\n", "-----END CERTIFICATE-----\n" + valid,
		"text\n-----BEGIN X-----\nAA==\n-----END Y-----\n",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		certs, _ := parsePEMCertificates(data)

		assert.LessOrEqual(t, len(certs), strings.Count(string(data), pemBeginMarker))

		for _, c := range certs {
			require.NotNil(t, c)
		}
	})
}

func TestChainPath(t *testing.T) {
	t.Parallel()

	root, rootKey := newTestCert(t, "root", true, nil, nil)
	top, topKey := newTestCert(t, "top", true, root, rootKey)
	issuing, issuingKey := newTestCert(t, "issuing", true, top, topKey)
	leaf, _ := newTestCert(t, "leaf", false, issuing, issuingKey)
	unrelated, _ := newTestCert(t, "unrelated", true, nil, nil)

	tests := []struct {
		name          string
		chain         []*x509.Certificate
		wantPath      []*x509.Certificate
		wantUnrelated bool
	}{
		{name: "leaf only", chain: []*x509.Certificate{leaf}, wantPath: []*x509.Certificate{leaf}},
		{
			name:     "in order",
			chain:    []*x509.Certificate{leaf, issuing, top},
			wantPath: []*x509.Certificate{leaf, issuing, top},
		},
		{
			name:     "in reverse order, with the root",
			chain:    []*x509.Certificate{leaf, root, top, issuing},
			wantPath: []*x509.Certificate{leaf, issuing, top, root},
		},
		{
			name:          "missing link",
			chain:         []*x509.Certificate{leaf, top},
			wantPath:      []*x509.Certificate{leaf},
			wantUnrelated: true,
		},
		{
			name:          "unrelated certificate",
			chain:         []*x509.Certificate{leaf, issuing, unrelated},
			wantPath:      []*x509.Certificate{leaf, issuing},
			wantUnrelated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path, unrelated := chainPath(tt.chain)

			assert.Equal(t, tt.wantPath, path)
			assert.Equal(t, tt.wantUnrelated, unrelated)
		})
	}
}

func TestAccept(t *testing.T) {
	t.Parallel()

	expired := []problem{problemLeafExpired}
	extra := []problem{problemUnrelatedCert}
	weak := []problem{problemWeakKey}

	tests := []struct {
		name         string
		new, active  []problem
		newUsable    bool
		activeUsable bool
		want         bool
	}{
		{name: "no problems", newUsable: true, activeUsable: true, want: true},
		{name: "same problems", new: extra, active: extra, newUsable: true, activeUsable: true, want: true},
		{name: "fewer problems", active: extra, newUsable: true, activeUsable: true, want: true},
		{name: "new problem while the active work", new: weak, active: extra, newUsable: true, activeUsable: true},
		{name: "new problem but repairs the active", new: extra, active: expired, newUsable: true, want: true},
		{name: "new problem and does not work either", new: []problem{problemLeafExpired, problemWeakKey}, active: expired},
		{name: "same unusable problem", new: expired, active: expired, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, accept(normalize(tt.new), normalize(tt.active), tt.newUsable, tt.activeUsable))
		})
	}
}
