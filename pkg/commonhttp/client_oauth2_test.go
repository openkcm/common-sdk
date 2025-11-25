package commonhttp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
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
		Source: commoncfg.EmbeddedSourceValue,
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
			name: "empty clientID",
			config: &commoncfg.OAuth2{
				Credentials: commoncfg.OAuth2Credentials{AuthMethod: "post"},
			},
			wantErr:    true,
			errMessage: "oauth2.clientID is missing",
		},
		{
			name: "invalid Empty AuthMethod",
			config: &commoncfg.OAuth2{
				Credentials: commoncfg.OAuth2Credentials{
					ClientID: *strRef("id"),
				},
			},
			wantErr: true, // AuthMethod optional; default handled by round-tripper
		},
		{
			name: "client_secret_jwt caching",
			config: &commoncfg.OAuth2{
				Credentials: commoncfg.OAuth2Credentials{
					ClientID:     *strRef("id"),
					AuthMethod:   "jwt",
					ClientSecret: strRef("secret"),
				},
				URL: strRef("https://example.com/token"),
			},
			wantErr: false,
			check: func(client *http.Client) {
				rt, ok := client.Transport.(*clientOAuth2RoundTripper)
				assert.True(t, ok)
				assert.Equal(t, "secret", *rt.ClientSecretJWT)

				// Test JWT generation and caching directly
				jwt1, err1 := rt.requestJWT("client_secret_jwt", *rt.ClientSecretJWT)
				jwt2, err2 := rt.requestJWT("client_secret_jwt", *rt.ClientSecretJWT)

				assert.NoError(t, err1)
				assert.NoError(t, err2)
				assert.Equal(t, jwt1, jwt2) // ensure cached JWT is reused
			},
		},
		{
			name: "private_key_jwt missing assertionType",
			config: &commoncfg.OAuth2{
				Credentials: commoncfg.OAuth2Credentials{
					ClientID:        *strRef("id"),
					AuthMethod:      "private",
					ClientAssertion: strRef("jwt"),
				},
				URL: strRef("https://example.com/token"),
			},
			wantErr:    true,
			errMessage: "clientAssertionType is required",
		},
		{
			name: "private_key_jwt missing assertion",
			config: &commoncfg.OAuth2{
				Credentials: commoncfg.OAuth2Credentials{
					ClientID:            *strRef("id"),
					AuthMethod:          "private",
					ClientAssertionType: strRef("urn:ietf:params:oauth:client-assertion-type:jwt-bearer"),
				},
				URL: strRef("https://example.com/token"),
			},
			wantErr:    true,
			errMessage: "invalid OAuth2 config: clientAssertionType cannot be provided without clientAssertion",
		},
		{
			name: "valid mTLS only",
			config: &commoncfg.OAuth2{
				Credentials: commoncfg.OAuth2Credentials{
					ClientID:   *strRef("id"),
					AuthMethod: "pkce",
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

				_, ok2 := rt.Next.(*http.Transport)
				assert.True(t, ok2)
			},
		},
		{
			name: "secret + irrelevant assertion field ignored",
			config: &commoncfg.OAuth2{
				Credentials: commoncfg.OAuth2Credentials{
					ClientID:            *strRef("id"),
					ClientSecret:        strRef("secret"),
					AuthMethod:          "post",
					ClientAssertion:     strRef("jwt"),                                                    // ignored because AuthMethod is "post"
					ClientAssertionType: strRef("urn:ietf:params:oauth:client-assertion-type:jwt-bearer"), // ignored
				},
			},
			wantErr: false, // no error
			check: func(client *http.Client) {
				rt, ok := client.Transport.(*clientOAuth2RoundTripper)
				assert.True(t, ok)
				assert.Equal(t, "secret", *rt.ClientSecretPost)
				assert.Nil(t, rt.ClientAssertion)
				assert.Nil(t, rt.ClientAssertionType)
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
