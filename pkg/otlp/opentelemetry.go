package otlp

import (
	"context"
	"crypto/tls"
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
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/logger"
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

// init initializes resource, trace, metrics and logger based on the given configs.
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

// initResource creates and sets a merged OpenTelemetry resource.
func (reg *registry) initResource(ctx context.Context) error {
	attrs := make([]attribute.KeyValue, 0)
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
		credential, err := commoncfg.LoadValueFromSourceRef(cfg.Traces.SecretRef.APIToken)
		if err != nil {
			return nil, err
		}

		sec = otlptracegrpc.WithHeaders(map[string]string{"Authorization": "Api-Token " + string(credential)})
	case commoncfg.MTLSSecretType:
		cert, err := commoncfg.LoadMTLSClientCertificate(cfg.Traces.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		caCertPool, err := commoncfg.LoadMTLSCACertPool(cfg.Traces.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		sec = otlptracegrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{*cert},
			RootCAs:      caCertPool,
			MinVersion:   tls.VersionTLS12,
			MaxVersion:   tls.VersionTLS13,
		}))
	case commoncfg.InsecureSecretType:
		sec = otlptracegrpc.WithInsecure()
	}

	host, err := commoncfg.LoadValueFromSourceRef(cfg.Traces.Host)
	if err != nil {
		return nil, err
	}

	options := []otlptracegrpc.Option{}
	options = append(options, sec)
	options = append(options, otlptracegrpc.WithEndpoint(string(host)))

	if cfg.Logs.URL != "" {
		options = append(options, otlptracegrpc.WithEndpointURL(cfg.Logs.URL))
	}

	return otlptracegrpc.New(ctx, options...)
}

// initTraceHttpExporter initializes an OTLP trace exporter over HTTP based on the provided telemetry configuration.
// It supports different authentication methods depending on the secret type.
func initTraceHTTPExporter(ctx context.Context, cfg *commoncfg.Telemetry) (*otlptrace.Exporter, error) {
	var sec otlptracehttp.Option

	switch cfg.Traces.SecretRef.Type {
	case commoncfg.ApiTokenSecretType:
		credential, err := commoncfg.LoadValueFromSourceRef(cfg.Traces.SecretRef.APIToken)
		if err != nil {
			return nil, err
		}

		sec = otlptracehttp.WithHeaders(map[string]string{"Authorization": "Api-Token " + string(credential)})
	case commoncfg.MTLSSecretType:
		cert, err := commoncfg.LoadMTLSClientCertificate(cfg.Traces.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		caCertPool, err := commoncfg.LoadMTLSCACertPool(cfg.Traces.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		sec = otlptracehttp.WithTLSClientConfig(&tls.Config{
			Certificates: []tls.Certificate{*cert},
			RootCAs:      caCertPool,
			MinVersion:   tls.VersionTLS12,
			MaxVersion:   tls.VersionTLS13,
		})
	case commoncfg.InsecureSecretType:
		sec = otlptracehttp.WithInsecure()
	}

	host, err := commoncfg.LoadValueFromSourceRef(cfg.Traces.Host)
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

	var opts []metric.Option

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
		credential, err := commoncfg.LoadValueFromSourceRef(cfg.Metrics.SecretRef.APIToken)
		if err != nil {
			return nil, err
		}

		sec = otlpmetricgrpc.WithHeaders(map[string]string{"Authorization": "Api-Token " + string(credential)})
	case commoncfg.MTLSSecretType:
		cert, err := commoncfg.LoadMTLSClientCertificate(cfg.Metrics.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		caCertPool, err := commoncfg.LoadMTLSCACertPool(cfg.Metrics.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		sec = otlpmetricgrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{*cert},
			RootCAs:      caCertPool,
			MinVersion:   tls.VersionTLS12,
			MaxVersion:   tls.VersionTLS13,
		}))
	case commoncfg.InsecureSecretType:
		sec = otlpmetricgrpc.WithInsecure()
	}

	host, err := commoncfg.LoadValueFromSourceRef(cfg.Metrics.Host)
	if err != nil {
		return nil, err
	}

	var options []otlpmetricgrpc.Option

	options = append(options, sec)
	options = append(options, otlpmetricgrpc.WithEndpoint(string(host)))

	if cfg.Logs.URL != "" {
		options = append(options, otlpmetricgrpc.WithEndpointURL(cfg.Logs.URL))
	}

	return otlpmetricgrpc.New(ctx, options...)
}

// initMetricHttpExporter initializes metrics http exporter.
// It supports different authentication methods depending on the secret type.
func initMetricHTTPExporter(ctx context.Context, cfg *commoncfg.Telemetry) (*otlpmetrichttp.Exporter, error) {
	var sec otlpmetrichttp.Option

	switch cfg.Metrics.SecretRef.Type {
	case commoncfg.ApiTokenSecretType:
		credential, err := commoncfg.LoadValueFromSourceRef(cfg.Metrics.SecretRef.APIToken)
		if err != nil {
			return nil, err
		}

		sec = otlpmetrichttp.WithHeaders(map[string]string{"Authorization": "Api-Token " + string(credential)})
	case commoncfg.MTLSSecretType:
		cert, err := commoncfg.LoadMTLSClientCertificate(cfg.Metrics.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		caCertPool, err := commoncfg.LoadMTLSCACertPool(cfg.Metrics.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		sec = otlpmetrichttp.WithTLSClientConfig(&tls.Config{
			Certificates: []tls.Certificate{*cert},
			RootCAs:      caCertPool,
			MinVersion:   tls.VersionTLS12,
			MaxVersion:   tls.VersionTLS13,
		})
	case commoncfg.InsecureSecretType:
		sec = otlpmetrichttp.WithInsecure()
	}

	host, err := commoncfg.LoadValueFromSourceRef(cfg.Metrics.Host)
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
		WithGroup(commoncfg.AttrService).With(
		slog.String(commoncfg.AttrEnvironment, reg.appCfg.Environment),
		slog.String(commoncfg.AttrName, reg.appCfg.Name),
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
		credential, err := commoncfg.LoadValueFromSourceRef(cfg.Logs.SecretRef.APIToken)
		if err != nil {
			return nil, err
		}

		sec = otlploggrpc.WithHeaders(map[string]string{"Authorization": "Api-Token " + string(credential)})
	case commoncfg.MTLSSecretType:
		cert, err := commoncfg.LoadMTLSClientCertificate(cfg.Logs.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		caCertPool, err := commoncfg.LoadMTLSCACertPool(cfg.Logs.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		sec = otlploggrpc.WithTLSCredentials(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{*cert},
			RootCAs:      caCertPool,
			MinVersion:   tls.VersionTLS12,
			MaxVersion:   tls.VersionTLS13,
		}))
	case commoncfg.InsecureSecretType:
		sec = otlploggrpc.WithInsecure()
	}

	host, err := commoncfg.LoadValueFromSourceRef(cfg.Logs.Host)
	if err != nil {
		return nil, err
	}

	options := []otlploggrpc.Option{}
	options = append(options, sec)
	options = append(options, otlploggrpc.WithEndpoint(string(host)))

	if cfg.Logs.URL != "" {
		options = append(options, otlploggrpc.WithEndpointURL(cfg.Logs.URL))
	}

	return otlploggrpc.New(ctx, options...)
}

func initLoggerHTTPExporter(ctx context.Context, cfg *commoncfg.Telemetry) (*otlploghttp.Exporter, error) {
	var sec otlploghttp.Option

	switch cfg.Logs.SecretRef.Type {
	case commoncfg.ApiTokenSecretType:
		credential, err := commoncfg.LoadValueFromSourceRef(cfg.Logs.SecretRef.APIToken)
		if err != nil {
			return nil, err
		}

		sec = otlploghttp.WithHeaders(map[string]string{"Authorization": "Api-Token " + string(credential)})
	case commoncfg.MTLSSecretType:
		cert, err := commoncfg.LoadMTLSClientCertificate(cfg.Logs.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		caCertPool, err := commoncfg.LoadMTLSCACertPool(cfg.Logs.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}

		sec = otlploghttp.WithTLSClientConfig(&tls.Config{
			Certificates: []tls.Certificate{*cert},
			RootCAs:      caCertPool,
			MinVersion:   tls.VersionTLS12,
			MaxVersion:   tls.VersionTLS13,
		})
	case commoncfg.InsecureSecretType:
		sec = otlploghttp.WithInsecure()
	}

	host, err := commoncfg.LoadValueFromSourceRef(cfg.Logs.Host)
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
