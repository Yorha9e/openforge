package server

import (
	"context"

	"google.golang.org/grpc/metadata"

	"go.opentelemetry.io/otel"
)

// gRPCMetadataCarrier adapts a gRPC metadata.MD to the OTel TextMapCarrier
// interface, allowing the W3C TraceContext propagator to inject/extract
// traceparent and tracestate headers across gRPC hops.
type gRPCMetadataCarrier metadata.MD

// Get retrieves a single value for the given key.
func (c gRPCMetadataCarrier) Get(key string) string {
	vals := metadata.MD(c).Get(key)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}

// Set stores a single key-value pair.
func (c gRPCMetadataCarrier) Set(key, value string) {
	metadata.MD(c).Set(key, value)
}

// Keys lists all keys present in the carrier.
func (c gRPCMetadataCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// InjectGRPCTraceContext attaches the current span's W3C traceparent and
// tracestate headers to the outgoing gRPC metadata so the receiving
// service (e.g. the Node.js IO layer) can continue the trace.
//
// Callers should pass a context that already carries the desired parent
// span (typically the stream context from wsConn.traceCtx or a child of it)
// and a fresh metadata.MD.  The returned metadata has the trace headers
// merged in and is safe to attach via metadata.NewOutgoingContext.
func InjectGRPCTraceContext(ctx context.Context, md metadata.MD) metadata.MD {
	if md == nil {
		md = metadata.New(nil)
	}
	otel.GetTextMapPropagator().Inject(ctx, gRPCMetadataCarrier(md))
	return md
}

// ExtractGRPCTraceContext pulls W3C traceparent and tracestate from
// incoming gRPC metadata and returns a child context with the upstream
// span as the parent.  Use this in the Node-side gRPC handlers to bridge
// the trace across the Go↔Node boundary (Go injects, Node extracts).
func ExtractGRPCTraceContext(ctx context.Context, md metadata.MD) context.Context {
	if md == nil {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, gRPCMetadataCarrier(md))
}
