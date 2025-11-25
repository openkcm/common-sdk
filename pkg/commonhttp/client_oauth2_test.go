package commonhttp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// helper to create SourceRef from a literal value for testing
func strRef(value string) *commoncfg.SourceRef {
	return &commoncfg.SourceRef{
		Source: commoncfg.EmbeddedSourceValue, // treat as literal value
		Value:  value,
	}
}

func TestNewClientFromOAuth2(t *testing.T) {
	certPEM, keyPEM, err := generateSelfSignedCert()
	assert.NoError(t, err)

	tests := []struct {
		name       string
		config     *commoncfg.OAuth2
		wantErr    bool
		errMessage string
		check      func(client *http.Client)
	}{
		{
			name:       "nil config",
			config:     nil,
			wantErr:    true,
			errMessage: "oauth2 config is nil",
		},
		{
			name: "missing clientID",
			config: &commoncfg.OAuth2{
				Credentials: commoncfg.OAuth2Credentials{},
			},
			wantErr:    true,
			errMessage: "oauth2.clientID is missing",
		},
		{
			name: "only clientSecret",
			config: &commoncfg.OAuth2{
				Credentials: commoncfg.OAuth2Credentials{
					ClientID:     *strRef("id"),
					ClientSecret: strRef("secret"),
				},
			},
			wantErr: false,
			check: func(client *http.Client) {
				rt, ok := client.Transport.(*clientOAuth2RoundTripper)
				assert.True(t, ok)
				assert.NotNil(t, rt.ClientSecretPost)
				assert.Nil(t, rt.ClientAssertion)
			},
		},
		{
			name: "only clientAssertion",
			config: &commoncfg.OAuth2{
				Credentials: commoncfg.OAuth2Credentials{
					ClientID:            *strRef("id"),
					ClientAssertion:     strRef("jwt"),
					ClientAssertionType: strRef("urn:ietf:params:oauth:client-assertion-type:jwt-bearer"),
				},
			},
			wantErr: false,
			check: func(client *http.Client) {
				rt, ok := client.Transport.(*clientOAuth2RoundTripper)
				assert.True(t, ok)
				assert.Nil(t, rt.ClientSecretPost)
				assert.NotNil(t, rt.ClientAssertion)
				assert.NotNil(t, rt.ClientAssertionType)
			},
		},
		{
			name: "both clientSecret and clientAssertion",
			config: &commoncfg.OAuth2{
				Credentials: commoncfg.OAuth2Credentials{
					ClientID:            *strRef("id"),
					ClientSecret:        strRef("secret"),
					ClientAssertion:     strRef("jwt"),
					ClientAssertionType: strRef("urn:ietf:params:oauth:client-assertion-type:jwt-bearer"),
				},
			},
			wantErr:    true,
			errMessage: "invalid OAuth2 config",
		},
		{
			name: "mTLS config sets transport",
			config: &commoncfg.OAuth2{
				Credentials: commoncfg.OAuth2Credentials{
					ClientID: *strRef("id"),
				},
				MTLS: &commoncfg.MTLS{
					Cert:    *strRef(certPEM),
					CertKey: *strRef(keyPEM),
				},
			},
			wantErr: false,
			check: func(client *http.Client) {
				rt, ok := client.Transport.(*clientOAuth2RoundTripper)
				assert.True(t, ok)
				tlsTransport, ok := rt.Next.(*http.Transport)
				assert.True(t, ok)
				assert.IsType(t, &tls.Config{}, tlsTransport.TLSClientConfig)
				assert.Len(t, tlsTransport.TLSClientConfig.Certificates, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClientFromOAuth2(tt.config)
			if tt.wantErr {
				assert.Error(t, err)

				if tt.errMessage != "" {
					assert.Contains(t, err.Error(), tt.errMessage)
				}

				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, client)

			if tt.check != nil {
				tt.check(client)
			}
		})
	}
}

// generateSelfSignedCert generates a self-signed certificate and key in PEM format
func generateSelfSignedCert() (string, string, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			CommonName:   "localhost",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour), // valid for 1 hour
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", "", err
	}

	certPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return "", "", err
	}

	keyPEMBytes := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})

	return string(certPEMBytes), string(keyPEMBytes), nil
}
