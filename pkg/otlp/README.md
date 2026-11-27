# Distributed tracing and log correlation

Initialize **logger.InitAsDefault** and then **otlp.Init** once at service startup,
before constructing instrumented clients and servers or processing requests.
otlp.Init always installs the W3C Trace Context and Baggage propagators and a
tracer provider. **telemetry.traces.enabled** controls export, so services still
create and propagate valid IDs when export is disabled. No KMS-specific baggage
is added; incoming baggage and vendor tracestate are preserved by the standard
propagators.

The provider uses OpenTelemetry's default ParentBased(AlwaysSample) sampler:
new roots are sampled, and children respect the parent's sampled flag. This
replaces the SDK's previous unconditional AlwaysSample setting. Unsampled
requests continue to propagate context and correlate logs but do not export
spans. Export uses the existing telemetry protocol, endpoint and authentication
configuration; select the approved collector and retain deployment TLS/mesh
controls.

## Correlated logs

The SDK logger adds **traceId** (32 lowercase hex characters) and **spanId**
(16 lowercase hex characters) whenever the log context has a valid span context,
including remote and non-recording/unsampled contexts. Use context-aware logging
such as slogctx.Info(ctx, ...) or slog.InfoContext(ctx, ...). No trace fields
are invented for startup or background logs without an active context; services
can attach their own request/workflow identifiers where applicable.

Field names are captured per logger handler:

    logger:
      formatter:
        fields:
          otel:
            traceId: trace_id
            spanId: span_id

As before, Error-level logs also set error status on a recording span. Log
messages and attributes remain the caller's responsibility: correlation does
not sanitize diagnostic data. See the [logging guide](../logger/logging.md).

## Synchronous boundaries

commongrpc.NewClient and commongrpc.NewServer use the SDK telemetry handlers.
For manually constructed gRPC clients/servers, install otlp.NewClientHandler()
or otlp.NewServerHandler() with grpc.WithStatsHandler/grpc.StatsHandler.
Both factories retain gRPC status codes but omit free-form span status
descriptions, which may contain peer/application data. Returned application
errors are unchanged. This applies to both client and server spans.

Application clients created with **commonhttp.NewHTTPClient** automatically
install standard otelhttp instrumentation around each HTTP attempt:

    client, err := commonhttp.NewHTTPClient(&httpClientConfig)

There is no separate NewTracingClient helper. Migrate application callers of the
lower-level authentication factories to NewHTTPClient to get tracing, metrics
and optional request logging/retries together. HTTPClient.MTLS supplies the TLS
configuration for the resulting application transport. When neither TLS nor
transport overrides are configured, the factory retains the authentication
factory's transport (normally http.DefaultTransport), preserving default proxy
behavior and sharing its connection pool across clients.

With retry enabled, the transport chain is:

    INTERNAL operation span -> Retry -> Telemetry -> Logging -> Auth -> HTTP

Each attempt creates a CLIENT span, injects that span's W3C context, and records
standard HTTP metrics. The retry operation creates one INTERNAL span named
**http.retry**, with the custom **retry.attempts** attribute and the final
**http.response.status_code** when a response is returned. It records error
status on transport/cancellation failures or a final HTTP status of 400 or higher.
Its status description is fixed text. It does not inject headers or record HTTP
metrics. A single telemetry transport delegates tracing and standard metric recording
to otelhttp, with the global tracer and meter providers taken when the client is
built. A client built before telemetry starts still reports to it, through the
global providers' delegation to the first provider set.

HTTP client metrics carry no URL path. Paths often hold identifiers, and every
distinct path would add a series to **http.client.request.duration** and
**http.client.request.body.size** until the SDK's limit of 2000 series per
instrument sends further measurements to a single overflow series. With the
Prometheus reader, that state lasts until the process restarts. To break metrics
down further, add bounded labels, such as **url.template** or an operation name,
to an **otelhttp.Labeler** attached with **otelhttp.ContextWithClientLabeler**.
The client reads that Labeler and never modifies it. A server-side Labeler,
attached with **otelhttp.ContextWithLabeler** (for example by an otelhttp
handler), does not reach client metrics, so an incoming request's
**http.route** is not copied onto outgoing requests.

The client records no error counter of its own. Failed attempts are counted from
**http.client.request.duration**, which otelhttp records for every attempt: a
4xx/5xx response carries **http.response.status_code**, and a transport error
carries **error.type** instead. For example, the failure rate per server in
Prometheus:

    sum by (server_address) (
        rate(http_client_request_duration_seconds_count{http_response_status_code=~"[45].."}[5m])
      or
        rate(http_client_request_duration_seconds_count{error_type!=""}[5m])
    )

**http.client.request.duration** covers each attempt from the telemetry transport
down: writing the log line (when logging is enabled) and adding credentials are
included, not only the network exchange. The attempt span's network events
(**http.dns.start/done**, **http.connect.start/done**, **http.tls.start/done**,
**http.got_conn**, **http.got_first_response_byte**) separate the two. A failed DNS,
connect or TLS phase carries **error.type** on its end event, never the error text.

The operation span ends when the retry loop returns the final response headers
or an error, including time spent in backoff. Attempt spans end at response-body
EOF or Close. Discarded retry responses close before backoff; the final response,
including an exhausted retryable response, stays readable for the caller.
With retries disabled, only the attempt span is created.

The lower-level NewClientFromBasic, NewClientFromOAuth2 and NewClientFromAPIToken
factories remain uninstrumented. OTLP signal exporters use these to avoid tracing
their own exports. Keep exporter clients on that path.

HTTP spans use standard attributes, including URL and transport error text.
Instrumentation does not capture body contents or Authorization header values.
Callers must keep sensitive values out of URLs and error text, and consume or
close response bodies so spans finish. Trace propagation requires the request
context, for example through http.NewRequestWithContext.

## Shutdown

Cancel the context passed to otlp.Init **after** application requests and jobs
have drained. Wait for the channel supplied to otlp.WithShutdownComplete before
exiting. The channel closes after shutdown even when export is disabled; it is
also closed if initialization fails. It is no longer closed immediately on
successful initialization with all export signals disabled.

    telemetryCtx, stopTelemetry := context.WithCancel(context.WithoutCancel(appCtx))
    defer stopTelemetry()
    telemetryStopped := make(chan struct{})
    if err := otlp.Init(telemetryCtx, appCfg, telemetryCfg, loggerCfg,
        otlp.WithShutdownComplete(telemetryStopped)); err != nil {
        return err
    }
    defer func() {
        stopTelemetry()
        <-telemetryStopped
    }()
    return runAndDrainApplication(appCtx)

Shutdown uses a fresh bounded context so cancellation does not discard the final
batch. When OTLP logging was enabled, shutdown restores the configured original
logger before stopping the exporter. Services must keep their shutdown grace
period long enough for application draining plus otlp.DefShutdownTimeout.

## Service responsibilities

The SDK supplies propagation, boundary instrumentation, correlation and export.
Services still choose where to start spans, how to model asynchronous processing,
and which context represents the logical request. Envoy authorization envelope
selection, response header mutations, authorization attributes, and local worker
producer links belong to extauthz. RabbitMQ/Orbital metadata and delayed-work span
links belong to their respective messaging integrations. These are not inferred
by the SDK from arbitrary requests or job functions.
