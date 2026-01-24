package tracing

import (
	"context"
	"encoding/hex"
	"net/http"
	"strings"
)

const (
	// TraceparentHeader is the W3C Trace Context traceparent header name.
	TraceparentHeader = "traceparent"

	// TracestateHeader is the W3C Trace Context tracestate header name.
	TracestateHeader = "tracestate"

	// W3C Trace Context version.
	traceparentVersion = "00"

	// Trace flags.
	flagSampled = 0x01
)

// Extract extracts the trace context from HTTP headers (W3C traceparent format).
// If no valid traceparent header is found, returns the original context unchanged.
//
// The traceparent format is: {version}-{trace-id}-{parent-id}-{flags}
// Example: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
func Extract(ctx context.Context, headers http.Header) context.Context {
	traceparent := headers.Get(TraceparentHeader)
	if traceparent == "" {
		return ctx
	}

	sc, ok := parseTraceparent(traceparent)
	if !ok {
		return ctx
	}

	return contextWithSpanContext(ctx, sc)
}

// Inject injects the trace context into HTTP headers.
// If there is no span in the context, this is a no-op.
func Inject(ctx context.Context, headers http.Header) {
	// First try to get span from context
	span := SpanFromContext(ctx)
	if span != nil {
		tp := formatTraceparent(span.TraceID, span.SpanID, span.tracer != nil)
		headers.Set(TraceparentHeader, tp)
		return
	}

	// Fall back to span context (from Extract)
	sc := SpanContextFromContext(ctx)
	if sc.IsValid() {
		tp := formatTraceparent(sc.TraceID, sc.SpanID, sc.Sampled)
		headers.Set(TraceparentHeader, tp)
	}
}

// parseTraceparent parses a W3C traceparent header.
// Returns the span context and whether parsing was successful.
func parseTraceparent(traceparent string) (SpanContext, bool) {
	// Format: {version}-{trace-id}-{parent-id}-{flags}
	// Example: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		return SpanContext{}, false
	}

	version := parts[0]
	traceID := parts[1]
	spanID := parts[2]
	flags := parts[3]

	// Validate version (currently only "00" is supported)
	if version != "00" {
		// Per spec, unknown versions with valid format should still be parsed
		if len(version) != 2 {
			return SpanContext{}, false
		}
	}

	// Validate trace ID (32 hex chars = 16 bytes)
	if len(traceID) != 32 {
		return SpanContext{}, false
	}
	if !isValidHex(traceID) {
		return SpanContext{}, false
	}
	// All zeros is invalid
	if traceID == "00000000000000000000000000000000" {
		return SpanContext{}, false
	}

	// Validate span ID (16 hex chars = 8 bytes)
	if len(spanID) != 16 {
		return SpanContext{}, false
	}
	if !isValidHex(spanID) {
		return SpanContext{}, false
	}
	// All zeros is invalid
	if spanID == "0000000000000000" {
		return SpanContext{}, false
	}

	// Parse flags
	if len(flags) != 2 {
		return SpanContext{}, false
	}
	flagBytes, err := hex.DecodeString(flags)
	if err != nil || len(flagBytes) != 1 {
		return SpanContext{}, false
	}
	sampled := (flagBytes[0] & flagSampled) != 0

	return SpanContext{
		TraceID: traceID,
		SpanID:  spanID,
		Sampled: sampled,
	}, true
}

// formatTraceparent formats a traceparent header value.
func formatTraceparent(traceID, spanID string, sampled bool) string {
	flags := "00"
	if sampled {
		flags = "01"
	}
	return traceparentVersion + "-" + traceID + "-" + spanID + "-" + flags
}

// isValidHex returns true if the string contains only valid hex characters.
func isValidHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// TraceIDFromContext returns the trace ID from the current context, if any.
func TraceIDFromContext(ctx context.Context) string {
	if span := SpanFromContext(ctx); span != nil {
		return span.TraceID
	}
	if sc := SpanContextFromContext(ctx); sc.IsValid() {
		return sc.TraceID
	}
	return ""
}

// SpanIDFromContext returns the span ID from the current context, if any.
func SpanIDFromContext(ctx context.Context) string {
	if span := SpanFromContext(ctx); span != nil {
		return span.SpanID
	}
	if sc := SpanContextFromContext(ctx); sc.IsValid() {
		return sc.SpanID
	}
	return ""
}

// Propagator defines how trace context is propagated across boundaries.
type Propagator interface {
	// Extract extracts span context from a carrier.
	Extract(ctx context.Context, carrier Carrier) context.Context
	// Inject injects span context into a carrier.
	Inject(ctx context.Context, carrier Carrier)
}

// Carrier is an interface for reading and writing propagation data.
type Carrier interface {
	Get(key string) string
	Set(key, value string)
}

// HeaderCarrier adapts http.Header to the Carrier interface.
type HeaderCarrier http.Header

// Get returns the value for a key.
func (hc HeaderCarrier) Get(key string) string {
	return http.Header(hc).Get(key)
}

// Set sets a key-value pair.
func (hc HeaderCarrier) Set(key, value string) {
	http.Header(hc).Set(key, value)
}

// W3CTraceContextPropagator implements the W3C Trace Context specification.
type W3CTraceContextPropagator struct{}

// NewW3CTraceContextPropagator creates a new W3C propagator.
func NewW3CTraceContextPropagator() *W3CTraceContextPropagator {
	return &W3CTraceContextPropagator{}
}

// Extract extracts span context from a carrier.
func (p *W3CTraceContextPropagator) Extract(ctx context.Context, carrier Carrier) context.Context {
	traceparent := carrier.Get(TraceparentHeader)
	if traceparent == "" {
		return ctx
	}

	sc, ok := parseTraceparent(traceparent)
	if !ok {
		return ctx
	}

	return contextWithSpanContext(ctx, sc)
}

// Inject injects span context into a carrier.
func (p *W3CTraceContextPropagator) Inject(ctx context.Context, carrier Carrier) {
	span := SpanFromContext(ctx)
	if span != nil {
		tp := formatTraceparent(span.TraceID, span.SpanID, span.tracer != nil)
		carrier.Set(TraceparentHeader, tp)
		return
	}

	sc := SpanContextFromContext(ctx)
	if sc.IsValid() {
		tp := formatTraceparent(sc.TraceID, sc.SpanID, sc.Sampled)
		carrier.Set(TraceparentHeader, tp)
	}
}
