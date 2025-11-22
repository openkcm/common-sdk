package otlp_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log/slog"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	config "github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/otlp"
)

func Test_OTLP_Init_Log(t *testing.T) {
	minimalLogger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	appCfg := &config.Application{Name: "test-service", BuildInfo: config.BuildInfo{Component: config.Component{Version: "1.0.0"}}}
	logCfg := &config.Logger{}

	tests := []struct {
		name     string
		secRef   config.SecretRef
		host     config.SourceRef
		protocol config.Protocol
		err      bool
	}{
		{
			name:     "Insecure over HTTP",
			protocol: config.HTTPProtocol,
			secRef: config.SecretRef{
				Type: config.InsecureSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4318",
			},
			err: false,
		},
		{
			name:     "Insecure over GRPC",
			protocol: config.GRPCProtocol,
			secRef: config.SecretRef{
				Type: config.InsecureSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4317",
			},
			err: false,
		},
		{
			name:     "APIToken over HTTP",
			protocol: config.HTTPProtocol,
			secRef: config.SecretRef{
				Type: config.ApiTokenSecretType,
				APIToken: config.SourceRef{
					Source: config.EmbeddedSourceValue,
					Value:  "test-api-token",
				},
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4318",
			},
			err: false,
		},
		{
			name:     "APIToken over GRPC",
			protocol: config.GRPCProtocol,
			secRef: config.SecretRef{
				Type: config.ApiTokenSecretType,
				APIToken: config.SourceRef{
					Source: config.EmbeddedSourceValue,
					Value:  "test-api-token",
				},
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4317",
			},
			err: false,
		},
		{
			name:     "MTLS over HTTP",
			protocol: config.HTTPProtocol,
			secRef: config.SecretRef{
				Type: config.MTLSSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4318",
			},
			err: false,
		},
		{
			name:     "MTLS over GRPC",
			protocol: config.GRPCProtocol,
			secRef: config.SecretRef{
				Type: config.MTLSSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4317",
			},
			err: false,
		},
		{
			name:     "Fail No Host",
			protocol: config.GRPCProtocol,
			secRef: config.SecretRef{
				Type: config.MTLSSecretType,
			},
			err: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.secRef.Type == config.MTLSSecretType {
				certPEM, keyPEM, caPEM, err := generatePEMs()
				require.NoError(t, err)

				tt.secRef.MTLS = config.MTLS{
					Cert: config.SourceRef{
						Source: config.EmbeddedSourceValue,
						Value:  string(certPEM),
					},
					CertKey: config.SourceRef{
						Source: config.EmbeddedSourceValue,
						Value:  string(keyPEM),
					},
					ServerCA: config.SourceRef{
						Source: config.EmbeddedSourceValue,
						Value:  string(caPEM),
					},
				}
			}

			telCfg := &config.Telemetry{
				Traces:  config.Trace{Enabled: false},
				Metrics: config.Metric{Enabled: false},
				Logs: config.Log{
					Enabled:   true,
					SecretRef: tt.secRef,
					Host:      tt.host,
					Protocol:  tt.protocol,
				},
			}

			// Create a channel to wait for completion of the shutdown
			shutdownComplete := make(chan struct{})
			ctx, cancel := context.WithCancel(t.Context())
			// Act
			err := otlp.Init(ctx, appCfg, telCfg, logCfg,
				otlp.WithLogger(minimalLogger),
				otlp.WithShutdownComplete(shutdownComplete),
			)
			// Initiate the shutdown
			cancel()
			// Wait for the shutdown to complete
			<-shutdownComplete

			// Assert
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			slog.Info("Test log") // To see if it isn't looping on logging
		})
	}
}

func Test_OTLP_Init_Trace(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	appCfg := &config.Application{Name: "test-service", BuildInfo: config.BuildInfo{Component: config.Component{Version: "1.0.0"}}}
	logCfg := &config.Logger{}

	tests := []struct {
		name     string
		secRef   config.SecretRef
		host     config.SourceRef
		protocol config.Protocol
		err      bool
	}{
		{
			name:     "Insecure over HTTP",
			protocol: config.HTTPProtocol,
			secRef: config.SecretRef{
				Type: config.InsecureSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4318",
			},
			err: false,
		},
		{
			name:     "Insecure over GRPC",
			protocol: config.GRPCProtocol,
			secRef: config.SecretRef{
				Type: config.InsecureSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4317",
			},
			err: false,
		},
		{
			name:     "APIToken over HTTP",
			protocol: config.HTTPProtocol,
			secRef: config.SecretRef{
				Type: config.ApiTokenSecretType,
				APIToken: config.SourceRef{
					Source: config.EmbeddedSourceValue,
					Value:  "test-api-token",
				},
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4318",
			},
			err: false,
		},
		{
			name:     "APIToken over GRPC",
			protocol: config.GRPCProtocol,
			secRef: config.SecretRef{
				Type: config.ApiTokenSecretType,
				APIToken: config.SourceRef{
					Source: config.EmbeddedSourceValue,
					Value:  "test-api-token",
				},
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4317",
			},
			err: false,
		},
		{
			name:     "MTLS over HTTP",
			protocol: config.HTTPProtocol,
			secRef: config.SecretRef{
				Type: config.MTLSSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4318",
			},
			err: false,
		},
		{
			name:     "MTLS over GRPC",
			protocol: config.GRPCProtocol,
			secRef: config.SecretRef{
				Type: config.MTLSSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4317",
			},
			err: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.secRef.Type == config.MTLSSecretType {
				certPEM, keyPEM, caPEM, err := generatePEMs()
				require.NoError(t, err)

				tt.secRef.MTLS = config.MTLS{
					Cert: config.SourceRef{
						Source: config.EmbeddedSourceValue,
						Value:  string(certPEM),
					},
					CertKey: config.SourceRef{
						Source: config.EmbeddedSourceValue,
						Value:  string(keyPEM),
					},
					ServerCA: config.SourceRef{
						Source: config.EmbeddedSourceValue,
						Value:  string(caPEM),
					},
				}
			}

			telCfg := &config.Telemetry{
				Traces: config.Trace{
					Enabled:   true,
					SecretRef: tt.secRef,
					Protocol:  tt.protocol,
					Host:      tt.host,
				},
				Metrics: config.Metric{Enabled: false},
				Logs:    config.Log{Enabled: false},
			}

			err := otlp.Init(ctx, appCfg, telCfg, logCfg)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			slog.Info("Test log") // To see if it isn't looping on logging
		})
	}
}

func Test_OTLP_Init_Metric(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	appCfg := &config.Application{Name: "test-service", BuildInfo: config.BuildInfo{Component: config.Component{Version: "1.0.0"}}}
	logCfg := &config.Logger{}

	tests := []struct {
		name     string
		secRef   config.SecretRef
		host     config.SourceRef
		protocol config.Protocol
		err      bool
	}{
		{
			name:     "Insecure over HTTP",
			protocol: config.HTTPProtocol,
			secRef: config.SecretRef{
				Type: config.InsecureSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4318",
			},
			err: false,
		},
		{
			name:     "Insecure over GRPC",
			protocol: config.GRPCProtocol,
			secRef: config.SecretRef{
				Type: config.InsecureSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4317",
			},
			err: false,
		},
		{
			name:     "APIToken over HTTP",
			protocol: config.HTTPProtocol,
			secRef: config.SecretRef{
				Type: config.ApiTokenSecretType,
				APIToken: config.SourceRef{
					Source: config.EmbeddedSourceValue,
					Value:  "test-api-token",
				},
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4318",
			},
			err: false,
		},
		{
			name:     "APIToken over GRPC",
			protocol: config.GRPCProtocol,
			secRef: config.SecretRef{
				Type: config.ApiTokenSecretType,
				APIToken: config.SourceRef{
					Source: config.EmbeddedSourceValue,
					Value:  "test-api-token",
				},
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4317",
			},
			err: false,
		},
		{
			name:     "MTLS over HTTP",
			protocol: config.HTTPProtocol,
			secRef: config.SecretRef{
				Type: config.MTLSSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4318",
			},
			err: false,
		},
		{
			name:     "MTLS over GRPC",
			protocol: config.GRPCProtocol,
			secRef: config.SecretRef{
				Type: config.MTLSSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4317",
			},
			err: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.secRef.Type == config.MTLSSecretType {
				certPEM, keyPEM, caPEM, err := generatePEMs()
				require.NoError(t, err)

				tt.secRef.MTLS = config.MTLS{
					Cert: config.SourceRef{
						Source: config.EmbeddedSourceValue,
						Value:  string(certPEM),
					},
					CertKey: config.SourceRef{
						Source: config.EmbeddedSourceValue,
						Value:  string(keyPEM),
					},
					ServerCA: config.SourceRef{
						Source: config.EmbeddedSourceValue,
						Value:  string(caPEM),
					},
				}
			}

			telCfg := &config.Telemetry{
				Traces: config.Trace{Enabled: false},
				Metrics: config.Metric{
					Enabled:   true,
					SecretRef: tt.secRef,
					Protocol:  tt.protocol,
					Host:      tt.host,
				},
				Logs: config.Log{Enabled: false},
			}

			err := otlp.Init(ctx, appCfg, telCfg, logCfg)
			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			slog.Info("Test log") // To see if it isn't looping on logging
		})
	}
}

// generatePEMs generates cert, key, and CA in PEM format.
func generatePEMs() ([]byte, []byte, []byte, error) {
	var certPEM, keyPEM, caPEM []byte

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, err
	}

	serial, _ := rand.Int(rand.Reader, big.NewInt(1<<62))

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{Organization: []string{"Test"}},
		NotBefore:    time.Now().Add(-time.Minute),
		NotAfter:     time.Now().Add(time.Hour),

		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, nil, err
	}

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	caPEM = certPEM // in self-signed cert, cert is CA

	return certPEM, keyPEM, caPEM, nil
}
