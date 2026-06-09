package otel

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// endUserProcessor stamps end-user identity (user.id, user.email, user.full_name,
// session.id) onto every span at start, read from the request context. This
// propagates user.id to all spans in a trace for per-user aggregation in
// trace-native backends like Langfuse, without instrumenting each span site.
type endUserProcessor struct{}

func (endUserProcessor) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	if attrs := EndUserAttrs(parent); len(attrs) > 0 {
		s.SetAttributes(attrs...)
	}
}

func (endUserProcessor) OnEnd(sdktrace.ReadOnlySpan) {}

func (endUserProcessor) Shutdown(context.Context) error { return nil }

func (endUserProcessor) ForceFlush(context.Context) error { return nil }

// SetEndUserSpan stamps end-user identity onto the active span in ctx. Needed
// for the root HTTP server span, which otelhttp starts before auth runs — so
// the start-time processor can't yet see the user.
func SetEndUserSpan(ctx context.Context) {
	if attrs := EndUserAttrs(ctx); len(attrs) > 0 {
		trace.SpanFromContext(ctx).SetAttributes(attrs...)
	}
}
