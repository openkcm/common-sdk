package otlp

import (
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc/stats"
)

// NewServerHandler returns a gRPC stats.Handler configured for server-side telemetry.
func NewServerHandler() stats.Handler {
	return otelgrpc.NewServerHandler(
		otelgrpc.WithTracerProvider(otel.GetTracerProvider()),
		otelgrpc.WithMeterProvider(otel.GetMeterProvider()),
		otelgrpc.WithPropagators(otel.GetTextMapPropagator()),
	)
}

// NewClientHandler returns a gRPC stats.Handler configured for client-side telemetry.
func NewClientHandler() stats.Handler {
	return otelgrpc.NewClientHandler(
		otelgrpc.WithTracerProvider(otel.GetTracerProvider()),
		otelgrpc.WithMeterProvider(otel.GetMeterProvider()),
		otelgrpc.WithPropagators(otel.GetTextMapPropagator()),
	)
}
