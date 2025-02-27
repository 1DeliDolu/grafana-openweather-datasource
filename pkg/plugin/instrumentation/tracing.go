package instrumentation

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type TracingHelper struct {
	tracer trace.Tracer
}

func NewTracingHelper(tracer trace.Tracer) *TracingHelper {
	if tracer == nil {
		tracer = noop.NewTracerProvider().Tracer("")
	}
	return &TracingHelper{
		tracer: tracer,
	}
}

// StartSpan starts a new span with optional attributes
func (t *TracingHelper) StartSpan(ctx context.Context, name string, attrs ...attribute.KeyValue) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name, trace.WithAttributes(attrs...))
}

// GetSpanContext retrieves the span context from the given context
func (t *TracingHelper) GetSpanContext(ctx context.Context) trace.SpanContext {
	spanCtx := trace.SpanContextFromContext(ctx)
	return spanCtx
}

// GetTraceID returns the trace ID from the given context
func (t *TracingHelper) GetTraceID(ctx context.Context) trace.TraceID {
	spanCtx := t.GetSpanContext(ctx)
	return spanCtx.TraceID()
}
