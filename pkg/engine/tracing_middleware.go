// Package engine provides the core mock server engine.
package engine

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/getmockd/mockd/pkg/tracing"
)

// TracingMiddleware wraps an HTTP handler with distributed tracing support.
// It extracts trace context from incoming requests, creates a span for each request,
// and records relevant HTTP attributes.
//
// If tracer is nil, the handler is returned unchanged (opt-in behavior).
func TracingMiddleware(tracer *tracing.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if tracer == nil {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract trace context from incoming request headers
			ctx := tracing.Extract(r.Context(), r.Header)

			// Create span name like "HTTP GET /path"
			spanName := fmt.Sprintf("HTTP %s %s", r.Method, r.URL.Path)

			// Start a new span
			ctx, span := tracer.Start(ctx, spanName)
			defer span.End()

			// Set HTTP request attributes
			span.SetAttribute("http.method", r.Method)
			span.SetAttribute("http.url", r.URL.String())
			span.SetAttribute("http.target", r.URL.Path)
			span.SetAttribute("http.host", r.Host)
			span.SetAttribute("http.scheme", r.URL.Scheme)
			if r.URL.Scheme == "" {
				if r.TLS != nil {
					span.SetAttribute("http.scheme", "https")
				} else {
					span.SetAttribute("http.scheme", "http")
				}
			}

			// Set user agent if present
			if ua := r.UserAgent(); ua != "" {
				span.SetAttribute("http.user_agent", ua)
			}

			// Set content length if present
			if r.ContentLength > 0 {
				span.SetAttribute("http.request_content_length", strconv.FormatInt(r.ContentLength, 10))
			}

			// Wrap the response writer to capture status code
			wrapped := &statusCapturingResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK, // Default status code
			}

			// Update request context with span
			r = r.WithContext(ctx)

			// Call the next handler
			next.ServeHTTP(wrapped, r)

			// Set response attributes
			span.SetAttribute("http.status_code", strconv.Itoa(wrapped.statusCode))

			// Set span status based on HTTP status code
			if wrapped.statusCode >= 400 && wrapped.statusCode < 500 {
				span.SetStatus(tracing.StatusError, fmt.Sprintf("HTTP client error: %d", wrapped.statusCode))
			} else if wrapped.statusCode >= 500 {
				span.SetStatus(tracing.StatusError, fmt.Sprintf("HTTP server error: %d", wrapped.statusCode))
			} else {
				span.SetStatus(tracing.StatusOK, "")
			}
		})
	}
}

// statusCapturingResponseWriter wraps http.ResponseWriter to capture the status code.
type statusCapturingResponseWriter struct {
	http.ResponseWriter
	statusCode    int
	headerWritten bool
}

// WriteHeader captures the status code before writing the header.
func (w *statusCapturingResponseWriter) WriteHeader(code int) {
	if !w.headerWritten {
		w.statusCode = code
		w.headerWritten = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write captures status code if not already written (implicit 200 OK).
func (w *statusCapturingResponseWriter) Write(b []byte) (int, error) {
	if !w.headerWritten {
		w.statusCode = http.StatusOK
		w.headerWritten = true
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter for http.ResponseController support.
func (w *statusCapturingResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
