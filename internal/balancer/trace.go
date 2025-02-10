package balancer

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// traceBackend 记录负载均衡选择事件
func traceBackend(ctx context.Context, address string, index int) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	span.AddEvent("Selected backend", trace.WithAttributes(
		attribute.String("backend.address", address),
		attribute.Int("backend.index", index),
	))
}
