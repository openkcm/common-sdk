package commonhttp_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"testing"
	"time"

	"github.com/openkcm/common-sdk/internal/testutils"
	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commonhttp"
)

func TestNewClient(t *testing.T) {
	// Arrange
	mutator := testutils.NewMutator(func() commoncfg.HTTPClient {
		return commoncfg.HTTPClient{
			Timeout: 10 * time.Second,
		}
	})

	rsaPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatalf("could not generate RSA key: %s", err)
	}

	rsaPublicKey := &rsaPrivateKey.PublicKey
	x509Cert := x509.Certificate{}

	x509CertDER, err := x509.CreateCertificate(rand.Reader, &x509Cert, &x509Cert, rsaPublicKey, rsaPrivateKey)
	if err != nil {
		t.Fatalf("Error creating x509 certificate: %s", err)
	}

	x509CertPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: x509CertDER,
	}))

	// test nil config
	t.Run("nil config", func(t *testing.T) {
		client, err := commonhttp.NewClient(nil)
		if err == nil {
			t.Errorf("expected error for nil config, got client: %v", client)
		}
	},
	)

	// create the test cases
	tests := []struct {
		name string
		cfg  commoncfg.HTTPClient
	}{
		{
			name: "default values",
			cfg:  mutator(),
		}, {
			name: "custom timeout",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.Timeout = 5 * time.Second
			}),
		}, {
			name: "custom root CAs",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.MTLS = &commoncfg.MTLS{
					Cert: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  x509CertPEM,
					},
					CertKey: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value: string(pem.EncodeToMemory(&pem.Block{
							Type:  "RSA PRIVATE KEY",
							Bytes: x509.MarshalPKCS1PrivateKey(rsaPrivateKey),
						}))},
					ServerCA: &commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  x509CertPEM},
				}
			}),
		}, {
			name: "custom mTLS certificate no Root CA",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.MTLS = &commoncfg.MTLS{
					Cert: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  x509CertPEM,
					},
					CertKey: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value: string(pem.EncodeToMemory(&pem.Block{
							Type:  "RSA PRIVATE KEY",
							Bytes: x509.MarshalPKCS1PrivateKey(rsaPrivateKey),
						}))},
				}
			}),
		},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			client, err := commonhttp.NewClient(&tc.cfg)

			// Assert
			if err != nil {
				t.Errorf("unexpected error: %s", err)
			} else {
				// Check if the client matches the config
				checkClient(t, client, tc.cfg)
			}
		})
	}
}

func TestNewHTTPClient(t *testing.T) {
	// Arrange
	mutator := testutils.NewMutator(func() commoncfg.HTTPClient {
		return commoncfg.HTTPClient{
			Timeout: 10 * time.Second,
		}
	})

	rsaPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatalf("could not generate RSA key: %s", err)
	}

	rsaPublicKey := &rsaPrivateKey.PublicKey
	x509Cert := x509.Certificate{}

	x509CertDER, err := x509.CreateCertificate(rand.Reader, &x509Cert, &x509Cert, rsaPublicKey, rsaPrivateKey)
	if err != nil {
		t.Fatalf("Error creating x509 certificate: %s", err)
	}

	x509CertPEM := string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: x509CertDER,
	}))

	// test nil config
	t.Run("nil config", func(t *testing.T) {
		client, err := commonhttp.NewHTTPClient(nil)
		if err == nil {
			t.Errorf("expected error for nil config, got client: %v", client)
		}
	},
	)

	// create the test cases
	tests := []struct {
		name    string
		cfg     commoncfg.HTTPClient
		wantErr bool
	}{
		{
			name: "default values",
			cfg:  mutator(),
		}, {
			name: "custom timeout",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.Timeout = 5 * time.Second
			}),
		}, {
			name: "custom root CAs",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.MTLS = &commoncfg.MTLS{
					Cert: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  x509CertPEM,
					},
					CertKey: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value: string(pem.EncodeToMemory(&pem.Block{
							Type:  "RSA PRIVATE KEY",
							Bytes: x509.MarshalPKCS1PrivateKey(rsaPrivateKey),
						}))},
					ServerCA: &commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  x509CertPEM},
				}
			}),
		}, {
			name: "custom mTLS certificate no Root CA",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.MTLS = &commoncfg.MTLS{
					Cert: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  x509CertPEM,
					},
					CertKey: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value: string(pem.EncodeToMemory(&pem.Block{
							Type:  "RSA PRIVATE KEY",
							Bytes: x509.MarshalPKCS1PrivateKey(rsaPrivateKey),
						}))},
				}
			}),
		},
		{
			name: "basic auth",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.BasicAuth = &commoncfg.BasicAuth{
					Username: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  "user",
					},
					Password: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  "pass",
					},
				}
			}),
		},
		{
			name: "basic auth missing username",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.BasicAuth = &commoncfg.BasicAuth{
					Password: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  "pass",
					},
				}
			}),
			wantErr: true,
		},
		{
			name: "basic auth with transport attributes",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.BasicAuth = &commoncfg.BasicAuth{
					Username: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  "user",
					},
					Password: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  "pass",
					},
				}
				c.TransportAttributes = &commoncfg.HTTPTransportAttributes{
					TLSHandshakeTimeout: 5 * time.Second,
				}
			}),
		},
		{
			name: "api token",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.APIToken = &commoncfg.SourceRef{
					Source: commoncfg.EmbeddedSourceValue,
					Value:  "token",
				}
			}),
		},
		{
			name: "api token missing value",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.APIToken = &commoncfg.SourceRef{
					Source: commoncfg.EmbeddedSourceValue,
					Value:  "",
				}
			}),
			wantErr: true,
		},
		{
			name: "oauth2 auth",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.OAuth2Auth = &commoncfg.OAuth2{
					Credentials: commoncfg.OAuth2Credentials{
						ClientID: commoncfg.SourceRef{
							Source: commoncfg.EmbeddedSourceValue,
							Value:  "client-id",
						},
						ClientSecret: &commoncfg.SourceRef{
							Source: commoncfg.EmbeddedSourceValue,
							Value:  "client-secret",
						},
						AuthMethod: commoncfg.OAuth2ClientSecretPost,
					},
				}
			}),
		},
		{
			name: "oauth2 auth missing client id",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.OAuth2Auth = &commoncfg.OAuth2{
					Credentials: commoncfg.OAuth2Credentials{
						AuthMethod: commoncfg.OAuth2ClientSecretPost,
					},
				}
			}),
			wantErr: true,
		},
		{
			name: "oauth2 with mTLS",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.OAuth2Auth = &commoncfg.OAuth2{
					Credentials: commoncfg.OAuth2Credentials{
						ClientID: commoncfg.SourceRef{
							Source: commoncfg.EmbeddedSourceValue,
							Value:  "client-id",
						},
						ClientSecret: &commoncfg.SourceRef{
							Source: commoncfg.EmbeddedSourceValue,
							Value:  "client-secret",
						},
						AuthMethod: commoncfg.OAuth2ClientSecretPost,
					},
				}
				c.MTLS = &commoncfg.MTLS{
					Cert: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value:  x509CertPEM,
					},
					CertKey: commoncfg.SourceRef{
						Source: commoncfg.EmbeddedSourceValue,
						Value: string(pem.EncodeToMemory(&pem.Block{
							Type:  "RSA PRIVATE KEY",
							Bytes: x509.MarshalPKCS1PrivateKey(rsaPrivateKey),
						}))},
				}
			}),
		},
		{
			name: "transport attributes",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.TransportAttributes = &commoncfg.HTTPTransportAttributes{
					TLSHandshakeTimeout: 5 * time.Second,
				}
			}),
		},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			client, err := commonhttp.NewHTTPClient(&tc.cfg)

			// Assert
			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %s", err)
				return
			}

			// Check if the client matches the config
			checkClient(t, client, tc.cfg)
		})
	}
}

func checkClient(t *testing.T, client *http.Client, cfg commoncfg.HTTPClient) {
	t.Helper()

	if client.Timeout != cfg.Timeout {
		t.Errorf("expected Timeout to be %v, got %v", cfg.Timeout, client.Timeout)
	}

	if client.Transport == nil {
		t.Fatal("expected Transport to be non-nil")
	}

	// Check if the transport is wrapped correctly based on auth type
	if cfg.BasicAuth != nil {
		// We can't easily check the private type, but we can check if it's NOT *http.Transport directly
		if _, ok := client.Transport.(*http.Transport); ok {
			t.Error("expected Transport to be wrapped for Basic Auth, got *http.Transport")
		}
	} else if cfg.OAuth2Auth != nil {
		if _, ok := client.Transport.(*http.Transport); ok {
			t.Error("expected Transport to be wrapped for OAuth2, got *http.Transport")
		}
	} else if cfg.APIToken != nil {
		if _, ok := client.Transport.(*http.Transport); ok {
			t.Error("expected Transport to be wrapped for API Token, got *http.Transport")
		}
	} else {
		// Default case should be *http.Transport
		transport, ok := client.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("expected Transport to be of type *http.Transport, got %T", client.Transport)
		}

		if transport.TLSClientConfig == nil {
			t.Fatal("expected TLSClientConfig to be non-nil")
		}

		if cfg.TransportAttributes != nil {
			if transport.TLSHandshakeTimeout != cfg.TransportAttributes.TLSHandshakeTimeout {
				t.Errorf("expected TLSHandshakeTimeout to be %v, got %v", cfg.TransportAttributes.TLSHandshakeTimeout, transport.TLSHandshakeTimeout)
			}
		}
	}
}
