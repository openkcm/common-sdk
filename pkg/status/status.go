package status

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/samber/oops"

	slogctx "github.com/veqryn/slog-context"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/prof"
)

const (
	DefReadHeaderTimeout = 2 * time.Second
	DefShutdownTimeout   = 5 * time.Second
)

type probesConfig struct {
	handlers map[string]func(http.ResponseWriter, *http.Request)
}

// ProbeOption configures a [Handler].
type ProbeOption func(*probesConfig)

func WithHealthZ(handler func(http.ResponseWriter, *http.Request)) ProbeOption {
	return WithCustom("healthz", handler)
}

func WithReadiness(handler func(http.ResponseWriter, *http.Request)) ProbeOption {
	return WithCustom("readiness", handler)
}

func WithLiveness(handler func(http.ResponseWriter, *http.Request)) ProbeOption {
	return WithCustom("liveness", handler)
}

func WithStartup(handler func(http.ResponseWriter, *http.Request)) ProbeOption {
	return WithCustom("startup", handler)
}

func WithCustom(name string, handler func(http.ResponseWriter, *http.Request)) ProbeOption {
	return func(c *probesConfig) {
		if handler == nil {
			return
		}

		c.handlers[name] = handler
	}
}

// versionHandlerFunc returns a handler function that writes the version information
// to the response. This is used in the status server.
func versionHandlerFunc(cfg *commoncfg.Application) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		info, err := json.Marshal(cfg.BuildInfo)
		if err != nil {
			slogctx.Error(req.Context(), "Failed to marshal application build info: %v", err)

			info = []byte(`{"version":"unknown"}`)
		}

		_, err = w.Write(info)
		if err != nil {
			return
		}
	}
}

// registerDefaultHandlers registers the default http handlers for the status server
func registerDefaultHandlers(_ context.Context, cfg *commoncfg.BaseConfig,
	mux *http.ServeMux,
	probeHandlers map[string]func(http.ResponseWriter, *http.Request),
) {
	if cfg.Telemetry.Metrics.Prometheus.Enabled {
		mux.Handle("/metrics", promhttp.Handler())
	}

	if cfg.Status.Profiling {
		prof.RegisterPProfHandlers(mux)
	}

	mux.HandleFunc("/version", versionHandlerFunc(&cfg.Application))

	for name, fn := range probeHandlers {
		mux.HandleFunc("/probe/"+name, fn)
	}
}

// createStatusServer creates a status http server using the given probesConfig
func createStatusServer(ctx context.Context,
	cfg *commoncfg.BaseConfig,
	mux *http.ServeMux,
	probeHandlers map[string]func(http.ResponseWriter, *http.Request),
) *http.Server {
	registerDefaultHandlers(ctx, cfg, mux, probeHandlers)

	slogctx.Info(ctx, "Creating status server", "address", cfg.Status.Address)

	return &http.Server{
		Addr:              cfg.Status.Address,
		Handler:           mux,
		ReadHeaderTimeout: DefReadHeaderTimeout,
	}
}

// StartStatusServer starts the status server using the given probesConfig.
func Start(ctx context.Context, cfg *commoncfg.BaseConfig, probes ...ProbeOption) error {
	if !cfg.Status.Enabled {
		return nil
	}

	mux := http.NewServeMux()

	prCfg := &probesConfig{
		handlers: make(map[string]func(http.ResponseWriter, *http.Request)),
	}

	for _, pr := range probes {
		if pr != nil {
			pr(prCfg)
		}
	}

	server := createStatusServer(ctx, cfg, mux, prCfg.handlers)

	slogctx.Info(ctx, "Starting status listener", "address", server.Addr)

	var lc net.ListenConfig

	listener, err := lc.Listen(ctx, "tcp", server.Addr)
	if err != nil {
		return oops.In("Status Server").
			WithContext(ctx).
			Wrapf(err, "Failed creating status listener")
	}

	go func() {
		slogctx.Info(ctx, "Starting status server", "address", server.Addr)

		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slogctx.Error(ctx, "ErrorField serving status http endpoint", "error", err)
		}

		slogctx.Info(ctx, "Stopped status server")
	}()

	<-ctx.Done()

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), DefShutdownTimeout)
	defer shutdownRelease()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		return oops.In("Status Server").
			WithContext(ctx).
			Wrapf(err, "Failed shutting down status server")
	}

	slogctx.Info(ctx, "Completed graceful shutdown of status server")

	return nil
}
