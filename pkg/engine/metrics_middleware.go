package engine

import (
	"bufio"
	"net"
	"net/http"
	"strconv"
	"strings"
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

// Hijack implements http.Hijacker if the underlying ResponseWriter supports it.
// This is required for WebSocket upgrades to work properly through the middleware chain.
func (w *metricsResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, http.ErrNotSupported
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
	if path == "" {
		return path
	}

	// Split the path into segments
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if segment == "" {
			continue
		}

		// Check for UUID (8-4-4-4-12 hex format)
		if isUUID(segment) {
			segments[i] = "{uuid}"
			continue
		}

		// Check for MongoDB ObjectID (24 hex chars)
		if isMongoObjectID(segment) {
			segments[i] = "{id}"
			continue
		}

		// Check for numeric ID (all digits, reasonably long to avoid false positives like "v1", "v2")
		if isNumericID(segment) {
			segments[i] = "{id}"
			continue
		}
	}

	return strings.Join(segments, "/")
}

// isUUID checks if a string matches UUID format (with or without hyphens)
func isUUID(s string) bool {
	// UUID format: 8-4-4-4-12 (with hyphens) = 36 chars
	if len(s) != 36 {
		return false
	}

	// Check format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else {
			if !isHexDigit(byte(c)) {
				return false
			}
		}
	}
	return true
}

// isMongoObjectID checks if a string is a MongoDB ObjectID (24 hex chars)
func isMongoObjectID(s string) bool {
	if len(s) != 24 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isHexDigit(s[i]) {
			return false
		}
	}
	return true
}

// isNumericID checks if a string is a numeric ID (5+ digits to avoid matching "v1", "v2", etc.)
func isNumericID(s string) bool {
	if len(s) < 3 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isHexDigit checks if a byte is a valid hex digit
func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
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
