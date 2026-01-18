// Package tracing provides OpenTelemetry-compatible distributed tracing for the mock server.
//
// This package implements W3C Trace Context propagation and span creation without any external
// dependencies, using only the standard library. It is designed to be lightweight and compatible
// with OTLP (OpenTelemetry Protocol) exporters.
//
// Key features:
//   - W3C Trace Context format (traceparent header) for distributed tracing
//   - Context propagation via context.Context
//   - Multiple exporters: stdout (for debugging) and OTLP HTTP
//   - Span attributes and events for detailed tracing
//   - Thread-safe span operations
//
// Usage:
//
//	// Create a tracer with service name
//	tracer := tracing.NewTracer("my-service",
//	    tracing.WithExporter(tracing.NewStdoutExporter()),
//	)
//
//	// Start a span
//	ctx, span := tracer.Start(ctx, "operation-name")
//	defer span.End()
//
//	// Add attributes and events
//	span.SetAttribute("http.method", "GET")
//	span.SetAttribute("http.url", "/api/users")
//	span.AddEvent("processing started")
//
//	// Set status on error
//	if err != nil {
//	    span.SetStatus(tracing.StatusError, err.Error())
//	}
//
// Context Propagation:
//
//	// Extract trace context from incoming HTTP request
//	ctx := tracing.Extract(ctx, req.Header)
//
//	// Inject trace context into outgoing HTTP request
//	tracing.Inject(ctx, outReq.Header)
//
// The package follows the W3C Trace Context specification:
// https://www.w3.org/TR/trace-context/
//
// Trace ID format: 32 hex characters (16 bytes)
// Span ID format: 16 hex characters (8 bytes)
// Traceparent format: {version}-{trace-id}-{parent-id}-{flags}
// Example: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
package tracing
