package commonhttp_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
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
			Timeout:            10 * time.Second,
			RootCAs:            nil,
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
			Cert:               nil,
			CertKey:            nil,
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
			name: "custom TLS min version",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.MinVersion = tls.VersionTLS13
			}),
		}, {
			name: "insecure skip verify",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.InsecureSkipVerify = true
			}),
		}, {
			name: "custom root CAs",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.RootCAs = &commoncfg.SourceRef{
					Source: commoncfg.EmbeddedSourceValue,
					Value:  x509CertPEM}
			}),
		}, {
			name: "custom mTLS certificate",
			cfg: mutator(func(c *commoncfg.HTTPClient) {
				c.Cert = &commoncfg.SourceRef{
					Source: commoncfg.EmbeddedSourceValue,
					Value:  x509CertPEM}
				c.CertKey = &commoncfg.SourceRef{
					Source: commoncfg.EmbeddedSourceValue,
					Value: string(pem.EncodeToMemory(&pem.Block{
						Type:  "RSA PRIVATE KEY",
						Bytes: x509.MarshalPKCS1PrivateKey(rsaPrivateKey),
					}))}
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

func checkClient(t *testing.T, client *http.Client, cfg commoncfg.HTTPClient) {
	t.Helper()

	if client.Timeout != cfg.Timeout {
		t.Errorf("expected Timeout to be %v, got %v", cfg.Timeout, client.Timeout)
	}

	if client.Transport == nil {
		t.Fatal("expected Transport to be non-nil")
	}

	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("expected Transport to be of type *http.Transport, got %T", client.Transport)
	}

	if transport.TLSClientConfig == nil {
		t.Fatal("expected TLSClientConfig to be non-nil")
	}

	if transport.TLSClientConfig.InsecureSkipVerify != cfg.InsecureSkipVerify {
		t.Errorf("expected InsecureSkipVerify to be %v, got %v", cfg.InsecureSkipVerify, transport.TLSClientConfig.InsecureSkipVerify)
	}

	if transport.TLSClientConfig.MinVersion != cfg.MinVersion {
		t.Errorf("expected MinVersion to be %v, got %v", cfg.MinVersion, transport.TLSClientConfig.MinVersion)
	}
}
