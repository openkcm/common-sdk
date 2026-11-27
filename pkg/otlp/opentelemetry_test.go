package otlp_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

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
					ServerCA: &config.SourceRef{
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
					ServerCA: &config.SourceRef{
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
		{
			name:     "Fail Unsupported Protocol",
			protocol: config.Protocol("udp"),
			secRef: config.SecretRef{
				Type: config.InsecureSecretType,
			},
			host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  "localhost:4317",
			},
			err: true,
		},
		{
			// Neither Prometheus nor an OTLP protocol.
			name: "Fail No Destination",
			err:  true,
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
					ServerCA: &config.SourceRef{
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

// Test_OTLP_Init_Metric_PrometheusAndOTLP checks that, with both destinations
// enabled, the same measurement is exposed on /metrics and pushed to the OTLP
// collector.
func Test_OTLP_Init_Metric_PrometheusAndOTLP(t *testing.T) {
	var (
		mu       sync.Mutex
		exported [][]byte
	)

	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/metrics" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		body := io.Reader(r.Body)
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer gz.Close()

			body = gz
		}

		payload, err := io.ReadAll(body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		mu.Lock()

		exported = append(exported, payload)

		mu.Unlock()
	}))
	defer collector.Close()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	appCfg := &config.Application{Name: "test-service", BuildInfo: config.BuildInfo{Component: config.Component{Version: "1.0.0"}}}
	telCfg := &config.Telemetry{
		Traces: config.Trace{Enabled: false},
		Metrics: config.Metric{
			Enabled:  true,
			Protocol: config.HTTPProtocol,
			Host: config.SourceRef{
				Source: config.EmbeddedSourceValue,
				Value:  strings.TrimPrefix(collector.URL, "http://"),
			},
			URL:        "/v1/metrics",
			SecretRef:  config.SecretRef{Type: config.InsecureSecretType},
			Prometheus: config.Prometheus{Enabled: true},
		},
		Logs: config.Log{Enabled: false},
	}

	stopped := make(chan struct{})
	require.NoError(t, otlp.Init(ctx, appCfg, telCfg, &config.Logger{}, otlp.WithShutdownComplete(stopped)))

	// The OTLP reader must be shut down with the telemetry, not left running.
	select {
	case <-stopped:
		t.Fatal("telemetry shutdown completed before the context was cancelled")
	default:
	}

	counter, err := otel.Meter("otlp-test").Int64Counter("test.dual.requests")
	require.NoError(t, err)
	counter.Add(t.Context(), 3)

	// Exposed on /metrics, which the status server serves from the default
	// Prometheus registry.
	rec := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Regexp(t, `(?m)^test_dual_requests_total\{[^}]*\} 3$`, rec.Body.String())

	// Pushed to the collector.
	provider, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider)
	require.True(t, ok, "otlp.Init did not install an SDK meter provider")
	require.NoError(t, provider.ForceFlush(t.Context()))

	mu.Lock()
	defer mu.Unlock()

	require.True(t, slices.ContainsFunc(exported, func(payload []byte) bool {
		return bytes.Contains(payload, []byte("test.dual.requests"))
	}), "the collector did not receive test.dual.requests")
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
