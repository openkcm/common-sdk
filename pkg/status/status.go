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
	"github.com/openkcm/common-sdk/pkg/health"
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
			slogctx.Error(req.Context(), "Failed to marshal application build info", "error", err)

			info = []byte(`{"version":"unknown"}`)
		}

		_, err = w.Write(info)
		if err != nil {
			return
		}
	}
}

// registerDefaultHandlers registers the default http handlers for the status server
func registerDefaultHandlers(cfg *commoncfg.BaseConfig,
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
	registerDefaultHandlers(cfg, mux, probeHandlers)

	slogctx.Info(ctx, "Creating status server", "address", cfg.Status.Address)

	return &http.Server{
		Addr:              cfg.Status.Address,
		Handler:           mux,
		ReadHeaderTimeout: DefReadHeaderTimeout,
	}
}

// Start starts the status server using the given probesConfig.
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

	var lc net.ListenConfig

	listener, err := lc.Listen(ctx, "tcp", server.Addr)
	if err != nil {
		return oops.In(cfg.Application.Name).
			WithContext(ctx).
			Wrapf(err, "Failed creating status listener")
	}

	go func() {
		slogctx.Info(ctx, "Starting status server", "address", server.Addr)

		err := server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slogctx.Error(ctx, "ErrorField serving status http endpoint", "error", err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), DefShutdownTimeout)
	defer shutdownRelease()

	err = server.Shutdown(shutdownCtx)
	if err != nil {
		return oops.In(cfg.Application.Name).
			WithContext(ctx).
			Wrapf(err, "Failed shutting down status server")
	}

	slogctx.Info(ctx, "Stopped status server")

	return nil
}

// ServeStatus starts the application's liveness and readiness health check server.
//
// This function configures and launches the health endpoints (typically `/live` and
// `/ready`) using the provided base configuration and optional health options.
// It sets up two independent health handlers:
//
//   - Liveness — indicates whether the application is alive.
//
//   - Implemented using a checker with autostart disabled.
//
//   - Exposed through an HTTP endpoint by the health server.
//
//   - Intended to signal if the process should be restarted.
//
//   - Readiness — indicates whether the application is ready to serve traffic.
//
//   - Configured with user-defined health options plus:
//
//   - Disabled autostart (must be triggered manually).
//
//   - Timeout based on BaseConfig.Status.Timeout.
//
//   - A status listener that logs detailed state transitions.
//
//   - Tracks the health of internal dependencies such as databases,
//     cache systems, external services, etc.
//
// A listener callback is registered for readiness state transitions.
// It logs the aggregated health state, including individual component
// statuses and results, making it easier to observe readiness changes
// during startup or degradation events.
//
// Parameters:
//   - ctx: The parent context for controlling the lifecycle of the health server.
//     Cancelling the context will stop the server gracefully.
//   - baseConfig: Common application configuration containing status server settings,
//     including timeout values and standard metadata.
//   - ops: Optional variadic list of additional health.Option values, allowing callers
//     to extend or override readiness configuration (e.g., register checkers).
//
// Behavior:
//  1. Constructs a liveness handler using disabled autostart semantics.
//  2. Builds a readiness handler composed of the provided health options, timeout,
//     and a status-logging listener.
//  3. Delegates to Start(...) to launch the health server with both handlers.
//  4. Returns an error if the server startup fails.
//
// Returns:
//   - error: Non-nil if the health server fails to start; wrapped with contextual
//     application name for improved observability.
//
// ServeStatus is intended for applications that follow Kubernetes-style probing
// conventions (liveness/readiness) or require structured health state management
// for orchestration and monitoring tools.
func ServeStatus(ctx context.Context, baseConfig *commoncfg.BaseConfig, ops ...health.Option) error {
	liveness := WithLiveness(
		health.NewHandler(
			health.NewChecker(health.WithDisabledAutostart()),
		),
	)

	healthOptions := make([]health.Option, 0)
	healthOptions = append(healthOptions,
		health.WithDisabledAutostart(),
		health.WithTimeout(baseConfig.Status.Timeout),
		health.WithStatusListener(func(ctx context.Context, state health.State) {
			subctx := slogctx.With(ctx, "status", state.Status)
			//nolint:fatcontext
			for name, substate := range state.CheckState {
				subctx = slogctx.WithGroup(subctx, name)
				subctx = slogctx.With(subctx,
					"status", substate.Status,
					"result", substate.Result,
				)
			}

			slogctx.Info(subctx, "readiness status changed")
		}),
	)
	healthOptions = append(healthOptions, ops...)

	readiness := WithReadiness(
		health.NewHandler(
			health.NewChecker(healthOptions...),
		),
	)

	err := Start(ctx, baseConfig, liveness, readiness)
	if err != nil {
		return oops.In(baseConfig.Application.Name).Wrapf(err, "Failed starting status server")
	}

	return err
}
