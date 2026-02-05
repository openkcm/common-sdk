package otlp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/exemplar"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/credentials"

	dtsdk "github.com/Dynatrace/OneAgent-SDK-for-Go/sdk"
	slogmulti "github.com/samber/slog-multi"
	slogctx "github.com/veqryn/slog-context"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commonhttp"
	"github.com/openkcm/common-sdk/pkg/logger"
	"github.com/openkcm/common-sdk/pkg/utils"
)

const (
	DefPeriodicReaderInterval = 2 * time.Second
	DefBatchTimeout           = 2 * time.Second
	DefShutdownTimeout        = 5 * time.Second
)

type registry struct {
	res            *resource.Resource
	traceProvider  *trace.TracerProvider
	meterProvider  *metric.MeterProvider
	loggerProvider *log.LoggerProvider

	appCfg *commoncfg.Application
	telCfg *commoncfg.Telemetry
	logCfg *commoncfg.Logger

	logger           *slog.Logger
	shutdownComplete chan struct{}
}

type Option func(*registry)

// WithLogger accepts a custom logger to be used for logging telemetry events.
func WithLogger(log *slog.Logger) Option {
	return func(reg *registry) {
		if log == nil {
			return
		}

		reg.logger = log
	}
}

// WithShutdownComplete accepts a channel to signal when the shutdown is complete.
func WithShutdownComplete(shutdownComplete chan struct{}) Option {
	return func(reg *registry) {
		if shutdownComplete == nil {
			return
		}

		reg.shutdownComplete = shutdownComplete
	}
}

// Init creates a registry, applies all options and startss the initialization.
func Init(ctx context.Context,
	appCfg *commoncfg.Application,
	telCfg *commoncfg.Telemetry,
	logCfg *commoncfg.Logger,
	options ...Option,
) error {
	reg := &registry{
		logger: slog.Default(),
		appCfg: appCfg,
		telCfg: telCfg,
		logCfg: logCfg,
	}

	for _, opt := range options {
		if opt != nil {
			opt(reg)
		}
	}

	return reg.init(ctx)
}

// init initializes loader, trace, metrics and logger based on the given configs.
func (reg *registry) init(ctx context.Context) error {
	err := reg.initResource(ctx)
	if err != nil {
		return reg.abortInit(err)
	}

	// Tracing configuration
	err = reg.initTrace(ctx)
	if err != nil {
		return reg.abortInit(err)
	}

	// Metrics configuration
	err = reg.initMetric(ctx)
	if err != nil {
		return reg.abortInit(err)
	}

	// Logs Configuration
	err = reg.initLogger(ctx)
	if err != nil {
		return reg.abortInit(err)
	}

	if reg.telCfg.Traces.Enabled || reg.telCfg.Logs.Enabled ||
		(reg.telCfg.Metrics.Enabled && !reg.telCfg.Metrics.Prometheus.Enabled) {
		otel.SetTextMapPropagator(
			propagation.NewCompositeTextMapPropagator(
				propagation.TraceContext{},
				propagation.Baggage{},
			),
		)

		go func() {
			<-ctx.Done()

			// revert the default logger from the fan out multi logger to a standard logger
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, nil)))

			// flush and shutdown all telemetry providers within a timeout
			shutdownCtx, shutdownRelease := context.WithTimeout(ctx, DefShutdownTimeout)
			defer shutdownRelease()

			reg.forceFlush(shutdownCtx)

			slogctx.Info(ctx, "Completed graceful shutdown of telemetries")

			// signal that the shutdown is complete
			if reg.shutdownComplete != nil {
				close(reg.shutdownComplete)
			}
		}()
	} else {
		if reg.shutdownComplete != nil {
			close(reg.shutdownComplete)
		}
	}

	return nil
}

// abortInit is called when an error occurs during initialization.
// It closes the shutdownComplete channel if it was set, and returns the error.
func (reg *registry) abortInit(err error) error {
	if reg.shutdownComplete != nil {
		close(reg.shutdownComplete)
	}

	return err
}

// forceFlush forces to immediately export all OpenTelemetry pending data and shutdowns.
func (reg *registry) forceFlush(ctx context.Context) {
	wg := sync.WaitGroup{}
	if reg.traceProvider != nil {
		wg.Add(1)

		go func() {
			defer wg.Done()

			slogctx.Info(ctx, "Stopping trace telemetry ...")
			_ = reg.traceProvider.ForceFlush(ctx)
			_ = reg.traceProvider.Shutdown(ctx)
			slogctx.Info(ctx, "Stopped and flushed the trace telemetry")
		}()
	}

	if reg.meterProvider != nil {
		wg.Add(1)

		go func() {
			defer wg.Done()

			slogctx.Info(ctx, "Stopping meter telemetry ...")
			_ = reg.meterProvider.ForceFlush(ctx)
			_ = reg.meterProvider.Shutdown(ctx)
			slogctx.Info(ctx, "Stopped and flushed the meter telemetry")
		}()
	}

	if reg.loggerProvider != nil {
		slogctx.Info(ctx, "Stopping logs telemetry ...")
		_ = reg.loggerProvider.ForceFlush(ctx)
		_ = reg.loggerProvider.Shutdown(ctx)
		slogctx.Info(ctx, "Stopped and flushed the logs telemetry")
	}

	wg.Wait()
}

// initResource creates and sets a merged OpenTelemetry loader.
func (reg *registry) initResource(ctx context.Context) error {
	attrs := make([]attribute.KeyValue, 0, 2+len(CreateAttributesFrom(*reg.appCfg)))
	attrs = append(attrs,
		semconv.ServiceVersion(reg.appCfg.BuildInfo.Version),
		semconv.ServiceName(reg.appCfg.Name),
	)
	attrs = append(attrs, CreateAttributesFrom(*reg.appCfg)...)

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL, attrs...),
	)
	if err != nil {
		return err
	}

	if reg.telCfg.DynatraceOneAgent {
		oneagentsdk := dtsdk.CreateInstance()
		dtMetadata := oneagentsdk.GetEnrichmentMetadata()

		var attributes []attribute.KeyValue
		for k, v := range dtMetadata {
			attributes = append(attributes, attribute.KeyValue{Key: attribute.Key(k), Value: attribute.StringValue(v)})
		}

		second, err := resource.New(ctx, resource.WithAttributes(attributes...))
		if err != nil {
			return err
		}

		res, err = resource.Merge(
			res,
			second,
		)
		if err != nil {
			return err
		}
	}

	reg.res = res

	return nil
}

// initTrace initializes the OpenTelemetry trace provider for the application.
func (reg *registry) initTrace(ctx context.Context) error {
	if !reg.telCfg.Traces.Enabled {
		return nil
	}

	slogctx.Info(ctx, "Starting traces telemetry ...")

	var option trace.TracerProviderOption

	switch reg.telCfg.Traces.Protocol {
	case commoncfg.GRPCProtocol:
		exporter, err := initTraceGrpcExporter(ctx, reg.telCfg)
		if err != nil {
			return err
		}

		option = trace.WithBatcher(exporter, trace.WithBatchTimeout(DefBatchTimeout))
	case commoncfg.HTTPProtocol:
		exporter, err := initTraceHTTPExporter(ctx, reg.telCfg)
		if err != nil {
			return err
		}

		option = trace.WithBatcher(exporter, trace.WithBatchTimeout(DefBatchTimeout))
	}

	reg.traceProvider = trace.NewTracerProvider(
		option,
		trace.WithResource(reg.res),
		trace.WithSampler(trace.AlwaysSample()),
	)
	otel.SetTracerProvider(reg.traceProvider)

	slogctx.Info(ctx, "Started successfully traces telemetry")

	return nil
}

// initTraceHttpExporter initializes an OTLP trace exporter over HTTP based on the provided telemetry configuration.
// It supports different authentication methods depending on the secret type.
func initTraceGrpcExporter(ctx context.Context, cfg *commoncfg.Telemetry) (*otlptrace.Exporter, error) {
	var sec otlptracegrpc.Option

	switch cfg.Traces.SecretRef.Type {
	case commoncfg.ApiTokenSecretType:
		token, err := computeAPITokenAuthorizationHeader(&cfg.Traces.SecretRef.APIToken)
		if err != nil {
			return nil, err
		}

		sec = otlptracegrpc.WithHeaders(map[string]string{"Authorization": token})
	case commoncfg.BasicSecretType:
		value, err := computeBasicAuthorizationHeader(&cfg.Traces.SecretRef.Basic)
		if err != nil {
			return nil, err
		}

		sec = otlptracegrpc.WithHeaders(map[string]string{"Authorization": value})
	case commoncfg.MTLSSecretType:
		tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.Traces.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		sec = otlptracegrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig))
	case commoncfg.InsecureSecretType:
		sec = otlptracegrpc.WithInsecure()
	default:
		return nil, fmt.Errorf("trace grpc doesn't unsupport secret type: %s", cfg.Traces.SecretRef.Type)
	}

	host, err := commoncfg.ExtractValueFromSourceRef(&cfg.Traces.Host)
	if err != nil {
		return nil, err
	}

	options := make([]otlptracegrpc.Option, 0, 2)
	options = append(options, sec)
	options = append(options, otlptracegrpc.WithEndpoint(string(host)))

	return otlptracegrpc.New(ctx, options...)
}

// initTraceHttpExporter initializes an OTLP trace exporter over HTTP based on the provided telemetry configuration.
// It supports different authentication methods depending on the secret type.
func initTraceHTTPExporter(ctx context.Context, cfg *commoncfg.Telemetry) (*otlptrace.Exporter, error) {
	var sec otlptracehttp.Option

	switch cfg.Traces.SecretRef.Type {
	case commoncfg.ApiTokenSecretType:
		client, err := commonhttp.NewClientFromAPIToken(&cfg.Traces.SecretRef.APIToken)
		if err != nil {
			return nil, err
		}

		sec = otlptracehttp.WithHTTPClient(client)
	case commoncfg.MTLSSecretType:
		tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.Traces.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		sec = otlptracehttp.WithTLSClientConfig(tlsConfig)
	case commoncfg.BasicSecretType:
		httpClient, err := commonhttp.NewClientFromBasic(&cfg.Traces.SecretRef.Basic)
		if err != nil {
			return nil, err
		}

		otlptracehttp.WithHTTPClient(httpClient)
	case commoncfg.OAuth2SecretType:
		httpClient, err := commonhttp.NewClientFromOAuth2(&cfg.Traces.SecretRef.OAuth2)
		if err != nil {
			return nil, err
		}

		otlptracehttp.WithHTTPClient(httpClient)
	case commoncfg.InsecureSecretType:
		sec = otlptracehttp.WithInsecure()
	}

	host, err := commoncfg.ExtractValueFromSourceRef(&cfg.Traces.Host)
	if err != nil {
		return nil, err
	}

	return otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(string(host)),
		otlptracehttp.WithURLPath(cfg.Traces.URL),
		sec,
	)
}

// initMetric initializes Prometheus metrics using chosen protocol.
func (reg *registry) initMetric(ctx context.Context) error {
	if !reg.telCfg.Metrics.Enabled {
		return nil
	}

	if reg.telCfg.Metrics.Prometheus.Enabled {
		prometheusExporter, err := prometheus.New()
		if err != nil {
			return err
		}

		reg.meterProvider = metric.NewMeterProvider(
			metric.WithResource(reg.res),
			metric.WithReader(prometheusExporter),
		)
		otel.SetMeterProvider(reg.meterProvider)

		return nil
	}

	slogctx.Info(ctx, "Starting meters telemetry ...")

	var periodicReader *metric.PeriodicReader

	switch reg.telCfg.Metrics.Protocol {
	case commoncfg.GRPCProtocol:
		exporter, err := initMetricGrpcExporter(ctx, reg.telCfg)
		if err != nil {
			return err
		}

		periodicReader = metric.NewPeriodicReader(exporter, metric.WithInterval(DefPeriodicReaderInterval))
	case commoncfg.HTTPProtocol:
		exporter, err := initMetricHTTPExporter(ctx, reg.telCfg)
		if err != nil {
			return err
		}

		periodicReader = metric.NewPeriodicReader(exporter, metric.WithInterval(DefPeriodicReaderInterval))
	}

	opts := make([]metric.Option, 0, 3)
	opts = append(opts,
		metric.WithResource(reg.res),
		metric.WithReader(periodicReader),
		metric.WithExemplarFilter(exemplar.AlwaysOnFilter),
	)

	reg.meterProvider = metric.NewMeterProvider(opts...)
	otel.SetMeterProvider(reg.meterProvider)

	// Start collecting Go runtime metrics (GC, heap, CPU)
	err := runtime.Start(runtime.WithMeterProvider(reg.meterProvider))
	if err != nil {
		return err
	}

	slogctx.Info(ctx, "Started successfully meters telemetry")

	return nil
}

// initMetricGrpcExporter initializes metrics gRPC exporter.
// It supports different authentication methods depending on the secret type.
func initMetricGrpcExporter(ctx context.Context, cfg *commoncfg.Telemetry) (*otlpmetricgrpc.Exporter, error) {
	var sec otlpmetricgrpc.Option

	switch cfg.Metrics.SecretRef.Type {
	case commoncfg.ApiTokenSecretType:
		token, err := computeAPITokenAuthorizationHeader(&cfg.Metrics.SecretRef.APIToken)
		if err != nil {
			return nil, err
		}

		sec = otlpmetricgrpc.WithHeaders(map[string]string{"Authorization": token})
	case commoncfg.BasicSecretType:
		value, err := computeBasicAuthorizationHeader(&cfg.Metrics.SecretRef.Basic)
		if err != nil {
			return nil, err
		}

		sec = otlpmetricgrpc.WithHeaders(map[string]string{"Authorization": value})
	case commoncfg.MTLSSecretType:
		tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.Metrics.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		sec = otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig))
	case commoncfg.InsecureSecretType:
		sec = otlpmetricgrpc.WithInsecure()
	default:
		return nil, fmt.Errorf("metric grpc doesn't unsupport secret type: %s", cfg.Metrics.SecretRef.Type)
	}

	host, err := commoncfg.ExtractValueFromSourceRef(&cfg.Metrics.Host)
	if err != nil {
		return nil, err
	}

	options := make([]otlpmetricgrpc.Option, 0, 2)
	options = append(options, sec)
	options = append(options, otlpmetricgrpc.WithEndpoint(string(host)))

	return otlpmetricgrpc.New(ctx, options...)
}

// initMetricHttpExporter initializes metrics http exporter.
// It supports different authentication methods depending on the secret type.
func initMetricHTTPExporter(ctx context.Context, cfg *commoncfg.Telemetry) (*otlpmetrichttp.Exporter, error) {
	var sec otlpmetrichttp.Option

	switch cfg.Metrics.SecretRef.Type {
	case commoncfg.ApiTokenSecretType:
		client, err := commonhttp.NewClientFromAPIToken(&cfg.Metrics.SecretRef.APIToken)
		if err != nil {
			return nil, err
		}

		sec = otlpmetrichttp.WithHTTPClient(client)
	case commoncfg.MTLSSecretType:
		tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.Metrics.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		sec = otlpmetrichttp.WithTLSClientConfig(tlsConfig)
	case commoncfg.BasicSecretType:
		httpClient, err := commonhttp.NewClientFromBasic(&cfg.Metrics.SecretRef.Basic)
		if err != nil {
			return nil, err
		}

		otlpmetrichttp.WithHTTPClient(httpClient)
	case commoncfg.OAuth2SecretType:
		httpClient, err := commonhttp.NewClientFromOAuth2(&cfg.Metrics.SecretRef.OAuth2)
		if err != nil {
			return nil, err
		}

		otlpmetrichttp.WithHTTPClient(httpClient)
	case commoncfg.InsecureSecretType:
		sec = otlpmetrichttp.WithInsecure()
	}

	host, err := commoncfg.ExtractValueFromSourceRef(&cfg.Metrics.Host)
	if err != nil {
		return nil, err
	}

	return otlpmetrichttp.New(
		ctx,
		otlpmetrichttp.WithEndpoint(string(host)),
		otlpmetrichttp.WithURLPath(cfg.Metrics.URL),
		otlpmetrichttp.WithTemporalitySelector(
			func(metric.InstrumentKind) metricdata.Temporality { return metricdata.DeltaTemporality },
		),
		sec,
	)
}

// initLogger initializes logger with GDPR Middleware and OpenTelemetry functionality and appends it to logger given in optionConfig.
func (reg *registry) initLogger(ctx context.Context) error {
	if !reg.telCfg.Logs.Enabled {
		return nil
	}

	slogctx.Info(ctx, "Starting logs telemetry ...")

	var processor *log.BatchProcessor

	switch reg.telCfg.Logs.Protocol {
	case commoncfg.GRPCProtocol:
		exporter, err := initLoggerGrpcExporter(ctx, reg.telCfg)
		if err != nil {
			return err
		}

		processor = log.NewBatchProcessor(exporter)
	case commoncfg.HTTPProtocol:
		exporter, err := initLoggerHTTPExporter(ctx, reg.telCfg)
		if err != nil {
			return err
		}

		processor = log.NewBatchProcessor(exporter)
	}

	reg.loggerProvider = log.NewLoggerProvider(
		log.WithProcessor(processor),
		log.WithResource(reg.res),
	)
	global.SetLoggerProvider(reg.loggerProvider)

	otelLogger := otelslog.NewLogger(reg.appCfg.Name, otelslog.WithLoggerProvider(reg.loggerProvider)).
		With(
			slog.String(commoncfg.AttrEnvironment, reg.appCfg.Environment),
			slog.String(commoncfg.AttrServiceName, reg.appCfg.Name),
		)

	labels := logger.CreateAttributes(reg.appCfg.Labels)
	if len(labels) > 0 {
		otelLogger = otelLogger.WithGroup(commoncfg.AttrLabels).With(labels...)
	}

	slog.SetDefault(slog.New(
		slogmulti.Fanout(
			reg.logger.Handler(),
			slogmulti.Pipe(slogmulti.Middleware(logger.NewGDPRMiddleware(reg.logCfg))).Handler(
				otelLogger.Handler(),
			),
		),
	))

	slogctx.Info(ctx, "Started successfully logs telemetry")

	return nil
}

// initLoggerGrpcExporter initializes logger gRPC exporter.
// It supports different authentication methods depending on the secret type.
func initLoggerGrpcExporter(ctx context.Context, cfg *commoncfg.Telemetry) (*otlploggrpc.Exporter, error) {
	var sec otlploggrpc.Option

	switch cfg.Logs.SecretRef.Type {
	case commoncfg.ApiTokenSecretType:
		token, err := computeAPITokenAuthorizationHeader(&cfg.Logs.SecretRef.APIToken)
		if err != nil {
			return nil, err
		}

		sec = otlploggrpc.WithHeaders(map[string]string{"Authorization": token})
	case commoncfg.BasicSecretType:
		value, err := computeBasicAuthorizationHeader(&cfg.Logs.SecretRef.Basic)
		if err != nil {
			return nil, err
		}

		sec = otlploggrpc.WithHeaders(map[string]string{"Authorization": value})
	case commoncfg.MTLSSecretType:
		tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.Logs.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		sec = otlploggrpc.WithTLSCredentials(credentials.NewTLS(tlsConfig))
	case commoncfg.InsecureSecretType:
		sec = otlploggrpc.WithInsecure()
	default:
		return nil, fmt.Errorf("logger grpc doesn't unsupport secret type: %s", cfg.Logs.SecretRef.Type)
	}

	host, err := commoncfg.ExtractValueFromSourceRef(&cfg.Logs.Host)
	if err != nil {
		return nil, err
	}

	options := make([]otlploggrpc.Option, 0, 2)
	options = append(options, sec)
	options = append(options, otlploggrpc.WithEndpoint(string(host)))

	return otlploggrpc.New(ctx, options...)
}

func initLoggerHTTPExporter(ctx context.Context, cfg *commoncfg.Telemetry) (*otlploghttp.Exporter, error) {
	var sec otlploghttp.Option

	switch cfg.Logs.SecretRef.Type {
	case commoncfg.ApiTokenSecretType:
		client, err := commonhttp.NewClientFromAPIToken(&cfg.Logs.SecretRef.APIToken)
		if err != nil {
			return nil, err
		}

		sec = otlploghttp.WithHTTPClient(client)
	case commoncfg.MTLSSecretType:
		tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.Logs.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		sec = otlploghttp.WithTLSClientConfig(tlsConfig)
	case commoncfg.BasicSecretType:
		httpClient, err := commonhttp.NewClientFromBasic(&cfg.Logs.SecretRef.Basic)
		if err != nil {
			return nil, err
		}

		otlploghttp.WithHTTPClient(httpClient)
	case commoncfg.OAuth2SecretType:
		httpClient, err := commonhttp.NewClientFromOAuth2(&cfg.Logs.SecretRef.OAuth2)
		if err != nil {
			return nil, err
		}

		otlploghttp.WithHTTPClient(httpClient)
	case commoncfg.InsecureSecretType:
		sec = otlploghttp.WithInsecure()
	}

	host, err := commoncfg.ExtractValueFromSourceRef(&cfg.Logs.Host)
	if err != nil {
		return nil, err
	}

	return otlploghttp.New(
		ctx,
		otlploghttp.WithEndpoint(string(host)),
		otlploghttp.WithURLPath(cfg.Logs.URL),
		sec,
	)
}

func computeBasicAuthorizationHeader(basicAuth *commoncfg.BasicAuth) (string, error) {
	username, err := commoncfg.ExtractValueFromSourceRef(&basicAuth.Username)
	if err != nil {
		return "", fmt.Errorf("failed to extract basic auth username: %w", err)
	}

	password, err := commoncfg.ExtractValueFromSourceRef(&basicAuth.Password)
	if err != nil {
		return "", fmt.Errorf("failed to extract basic auth password: %w", err)
	}

	return "Basic " + utils.BasicAuth(string(username), string(password)), nil
}

func computeAPITokenAuthorizationHeader(token *commoncfg.SourceRef) (string, error) {
	value, err := commoncfg.ExtractValueFromSourceRef(token)
	if err != nil {
		return "", fmt.Errorf("failed to extract api token value: %w", err)
	}

	return "Api-Token " + string(value), nil
}
