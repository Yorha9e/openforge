package adapter

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitOTelTracer configures the global OpenTelemetry tracer provider to
// export spans via OTLP/gRPC to the given endpoint (e.g. "localhost:4317").
// It also installs the W3C TraceContext + Baggage propagator on the global
// propagator so that incoming and outgoing traceparent headers are
// preserved across Go↔Node hops.
//
// On success the returned shutdown function should be deferred to flush
// any pending spans before process exit.  Errors are returned for
// unrecoverable initialisation failures (e.g. malformed endpoint) but
// transient network errors during dial are tolerated — the batcher will
// retry in the background.
func InitOTelTracer(ctx context.Context, serviceName, otlpEndpoint string) (func(context.Context) error, error) {
	if otlpEndpoint == "" {
		return nil, fmt.Errorf("otlp endpoint is required")
	}
	if serviceName == "" {
		serviceName = "openforge"
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("otlptracegrpc.New: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		// Resource creation can fail in OTel when the environment is
		// unusual; fall back to a minimal resource so the provider
		// still initialises rather than blocking startup.
		res = resource.NewSchemaless(
			semconv.ServiceName(serviceName),
		)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	// Install the global tracer provider so that any code calling
	// otel.Tracer("...") automatically uses the OTLP-backed one.
	otel.SetTracerProvider(tp)

	// Install W3C TraceContext + Baggage propagator globally so incoming
	// traceparent headers are honoured and outgoing requests are
	// instrumented with the W3C spec format.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
