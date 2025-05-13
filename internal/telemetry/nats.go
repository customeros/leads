package telemetry

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

// InjectTraceContextIntoNatsMsg injects the current trace context into NATS message headers
func InjectTraceContextIntoNatsMsg(ctx context.Context, msg *nats.Msg) {
	if msg == nil {
		return
	}

	// Initialize headers if nil
	if msg.Header == nil {
		msg.Header = nats.Header{}
	}

	// Get the propagator and inject the context into headers
	propagator := otel.GetTextMapPropagator()
	carrier := propagation.HeaderCarrier(msg.Header)
	propagator.Inject(ctx, carrier)
}

// ExtractTraceContextFromNatsMsg extracts the trace context from NATS message headers
func ExtractTraceContextFromNatsMsg(ctx context.Context, msg *nats.Msg) context.Context {
	if msg == nil || msg.Header == nil {
		return ctx
	}

	// Get the propagator and extract the context from headers
	propagator := otel.GetTextMapPropagator()
	carrier := propagation.HeaderCarrier(msg.Header)
	return propagator.Extract(ctx, carrier)
}
