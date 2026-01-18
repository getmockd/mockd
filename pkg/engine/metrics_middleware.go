package engine

import (
	"net/http"
	"strconv"
	"time"

	"github.com/getmockd/mockd/pkg/metrics"
)

// metricsResponseWriter wraps http.ResponseWriter to capture the status code.
type metricsResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// newMetricsResponseWriter creates a new metricsResponseWriter.
func newMetricsResponseWriter(w http.ResponseWriter) *metricsResponseWriter {
	return &metricsResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // default
	}
}

// WriteHeader captures the status code and writes it to the underlying ResponseWriter.
func (w *metricsResponseWriter) WriteHeader(code int) {
	if !w.written {
		w.statusCode = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

// Write writes data to the underlying ResponseWriter.
func (w *metricsResponseWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

// Flush implements http.Flusher if the underlying ResponseWriter supports it.
func (w *metricsResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// MetricsMiddleware wraps an http.Handler to record Prometheus metrics.
// It records request counts and durations.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code
		mrw := newMetricsResponseWriter(w)

		// Call the next handler
		next.ServeHTTP(mrw, r)

		// Record metrics
		duration := time.Since(start)
		status := strconv.Itoa(mrw.statusCode)
		path := normalizePathForMetrics(r.URL.Path)

		// Record request count
		if metrics.RequestsTotal != nil {
			if vec, err := metrics.RequestsTotal.WithLabels(r.Method, path, status); err == nil {
				_ = vec.Inc()
			}
		}

		// Record request duration
		if metrics.RequestDuration != nil {
			if vec, err := metrics.RequestDuration.WithLabels(r.Method, path); err == nil {
				vec.Observe(duration.Seconds())
			}
		}

		// Record match hits/misses
		if mrw.statusCode == http.StatusNotFound {
			if metrics.MatchMissesTotal != nil {
				metrics.MatchMissesTotal.Inc()
			}
		}
	})
}

// normalizePathForMetrics normalizes a path for use as a metric label.
// This prevents high cardinality by replacing dynamic path segments with placeholders.
func normalizePathForMetrics(path string) string {
	// For now, return the path as-is
	// In production, you might want to:
	// - Replace UUIDs with {id}
	// - Replace numeric IDs with {id}
	// - Use the matched mock's path pattern instead
	//
	// Example improvements:
	// - /users/123/posts -> /users/{id}/posts
	// - /api/v1/items/abc-def-ghi -> /api/v1/items/{id}
	return path
}

// RecordMatchHit records a hit for a specific mock.
func RecordMatchHit(mockID string) {
	if metrics.MatchHitsTotal != nil {
		if vec, err := metrics.MatchHitsTotal.WithLabels(mockID); err == nil {
			_ = vec.Inc()
		}
	}
}

// RecordSSEConnection records an SSE connection opening or closing.
func RecordSSEConnection(path string, delta int) {
	if metrics.ActiveConnections != nil {
		if vec, err := metrics.ActiveConnections.WithLabels("sse"); err == nil {
			vec.Add(float64(delta))
		}
	}
}

// RecordWebSocketConnection records a WebSocket connection opening or closing.
func RecordWebSocketConnection(path string, delta int) {
	if metrics.ActiveConnections != nil {
		if vec, err := metrics.ActiveConnections.WithLabels("websocket"); err == nil {
			vec.Add(float64(delta))
		}
	}
}

// UpdateMockCounts updates the mock count gauges.
func UpdateMockCounts(mockType string, total, enabled int) {
	if metrics.MocksTotal != nil {
		if vec, err := metrics.MocksTotal.WithLabels(mockType); err == nil {
			vec.Set(float64(total))
		}
	}
	if metrics.MocksEnabled != nil {
		if vec, err := metrics.MocksEnabled.WithLabels(mockType); err == nil {
			vec.Set(float64(enabled))
		}
	}
}
