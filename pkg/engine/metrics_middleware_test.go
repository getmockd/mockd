package engine

import (
	"bufio"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePathForMetrics(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Numeric IDs
		{
			name:     "numeric ID in path",
			input:    "/users/123",
			expected: "/users/{id}",
		},
		{
			name:     "multiple numeric IDs",
			input:    "/users/123/posts/456",
			expected: "/users/{id}/posts/{id}",
		},
		{
			name:     "long numeric ID",
			input:    "/orders/9876543210",
			expected: "/orders/{id}",
		},

		// UUIDs
		{
			name:     "UUID in path",
			input:    "/items/a1b2c3d4-e5f6-7890-abcd-ef1234567890",
			expected: "/items/{uuid}",
		},
		{
			name:     "uppercase UUID",
			input:    "/items/A1B2C3D4-E5F6-7890-ABCD-EF1234567890",
			expected: "/items/{uuid}",
		},
		{
			name:     "UUID with prefix path",
			input:    "/api/v1/users/a1b2c3d4-e5f6-7890-abcd-ef1234567890/profile",
			expected: "/api/v1/users/{uuid}/profile",
		},

		// MongoDB ObjectIDs (24 hex chars)
		{
			name:     "MongoDB ObjectID",
			input:    "/documents/507f1f77bcf86cd799439011",
			expected: "/documents/{id}",
		},

		// Short hex strings are NOT normalized (too risky - could match legitimate paths)
		// Only UUIDs, ObjectIDs, and numeric IDs are normalized
		{
			name:     "short hex NOT normalized (could be legitimate)",
			input:    "/sessions/abc123def",
			expected: "/sessions/abc123def",
		},
		{
			name:     "medium hex NOT normalized (could be legitimate)",
			input:    "/tokens/abc123def456789a",
			expected: "/tokens/abc123def456789a",
		},

		// No normalization needed
		{
			name:     "static path",
			input:    "/api/v1/users",
			expected: "/api/v1/users",
		},
		{
			name:     "root path",
			input:    "/",
			expected: "/",
		},
		{
			name:     "named resource",
			input:    "/users/admin",
			expected: "/users/admin",
		},
		{
			name:     "query params not affected",
			input:    "/search?q=test",
			expected: "/search?q=test",
		},

		// Mixed scenarios
		{
			name:     "mixed IDs",
			input:    "/orgs/123/projects/a1b2c3d4-e5f6-7890-abcd-ef1234567890/tasks/456",
			expected: "/orgs/{id}/projects/{uuid}/tasks/{id}",
		},
		{
			name:     "API versioning preserved",
			input:    "/api/v2/users/123",
			expected: "/api/v2/users/{id}",
		},

		// Edge cases
		{
			name:     "empty path",
			input:    "",
			expected: "",
		},
		{
			name:     "path with trailing slash",
			input:    "/users/123/",
			expected: "/users/{id}/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizePathForMetrics(tt.input)
			if result != tt.expected {
				t.Errorf("normalizePathForMetrics(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func BenchmarkNormalizePathForMetrics(b *testing.B) {
	paths := []string{
		"/api/v1/users",
		"/users/123/posts/456",
		"/items/a1b2c3d4-e5f6-7890-abcd-ef1234567890",
		"/documents/507f1f77bcf86cd799439011/comments/789",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range paths {
			normalizePathForMetrics(p)
		}
	}
}

// ---------------------------------------------------------------------------
// Helper: collect a counter value for given label set
// ---------------------------------------------------------------------------

func collectCounterValue(t *testing.T, c *metrics.Counter, labels ...string) float64 {
	t.Helper()
	if len(labels) == 0 {
		// No-label counter — look for the single sample
		samples := c.Collect()
		if len(samples) == 0 {
			return 0
		}
		return samples[0].Value
	}
	// Labeled counter — match by label values
	for _, s := range c.Collect() {
		match := true
		for _, lv := range labels {
			found := false
			for _, v := range s.Labels {
				if v == lv {
					found = true
					break
				}
			}
			if !found {
				match = false
				break
			}
		}
		if match {
			return s.Value
		}
	}
	return 0
}

func collectGaugeValue(t *testing.T, g *metrics.Gauge, labels ...string) float64 {
	t.Helper()
	if len(labels) == 0 {
		samples := g.Collect()
		if len(samples) == 0 {
			return 0
		}
		return samples[0].Value
	}
	for _, s := range g.Collect() {
		match := true
		for _, lv := range labels {
			found := false
			for _, v := range s.Labels {
				if v == lv {
					found = true
					break
				}
			}
			if !found {
				match = false
				break
			}
		}
		if match {
			return s.Value
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// initMetrics is a test helper that resets then re-inits the global metrics.
// ---------------------------------------------------------------------------

func initMetrics(t *testing.T) {
	t.Helper()
	metrics.Reset()
	metrics.Init()
	t.Cleanup(func() { metrics.Reset() })
}

// ---------------------------------------------------------------------------
// MetricsMiddleware tests
// ---------------------------------------------------------------------------

// TestMetricsMiddleware groups all tests that depend on the global metrics
// singleton. They run sequentially within this parent test to avoid races
// on metrics.Reset() / metrics.Init().
func TestMetricsMiddleware(t *testing.T) {
	t.Run("ServesRequest", func(t *testing.T) {
		initMetrics(t)

		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("hello"))
		})

		handler := MetricsMiddleware(inner)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "hello", rec.Body.String())
	})

	t.Run("RecordsRequestMetrics", func(t *testing.T) {
		initMetrics(t)

		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		handler := MetricsMiddleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		// RequestsTotal should have a sample for GET /api/v1/health 200
		require.NotNil(t, metrics.RequestsTotal)
		val := collectCounterValue(t, metrics.RequestsTotal, "GET", "/api/v1/health", "200")
		assert.Equal(t, float64(1), val)

		// RequestDuration should have at least one observation
		require.NotNil(t, metrics.RequestDuration)
		samples := metrics.RequestDuration.Collect()
		assert.NotEmpty(t, samples, "expected RequestDuration samples after a request")
	})

	t.Run("RecordsMissOn404", func(t *testing.T) {
		initMetrics(t)

		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})
		handler := MetricsMiddleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/not/found", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)

		// MatchMissesTotal should be incremented
		require.NotNil(t, metrics.MatchMissesTotal)
		val := collectCounterValue(t, metrics.MatchMissesTotal)
		assert.Equal(t, float64(1), val)
	})

	t.Run("NoMissOnSuccess", func(t *testing.T) {
		initMetrics(t)

		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		handler := MetricsMiddleware(inner)

		req := httptest.NewRequest(http.MethodGet, "/ok", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		require.NotNil(t, metrics.MatchMissesTotal)
		val := collectCounterValue(t, metrics.MatchMissesTotal)
		assert.Equal(t, float64(0), val)
	})
}

// ---------------------------------------------------------------------------
// metricsResponseWriter tests
// ---------------------------------------------------------------------------

func TestMetricsResponseWriter_WriteHeaderCapturesStatus(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	mrw := newMetricsResponseWriter(rec)

	mrw.WriteHeader(http.StatusCreated)

	assert.Equal(t, http.StatusCreated, mrw.statusCode)
	assert.True(t, mrw.written)
	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestMetricsResponseWriter_WriteHeaderOnlyFirstCall(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	mrw := newMetricsResponseWriter(rec)

	mrw.WriteHeader(http.StatusCreated)
	mrw.WriteHeader(http.StatusBadRequest) // second call — statusCode should stay 201

	assert.Equal(t, http.StatusCreated, mrw.statusCode)
}

func TestMetricsResponseWriter_WriteAutoSets200(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	mrw := newMetricsResponseWriter(rec)

	n, err := mrw.Write([]byte("data"))
	require.NoError(t, err)
	assert.Equal(t, 4, n)

	// statusCode should remain 200 (the default) and written should be true
	assert.Equal(t, http.StatusOK, mrw.statusCode)
	assert.True(t, mrw.written)
}

func TestMetricsResponseWriter_DefaultStatus200(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	mrw := newMetricsResponseWriter(rec)

	// Before any write or WriteHeader, statusCode is 200
	assert.Equal(t, http.StatusOK, mrw.statusCode)
	assert.False(t, mrw.written)
}

// ---------------------------------------------------------------------------
// Flush / Hijack delegation tests
// ---------------------------------------------------------------------------

// flusherRecorder is an http.ResponseWriter that also implements http.Flusher.
type flusherRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func (f *flusherRecorder) Flush() { f.flushed = true }

func TestMetricsResponseWriter_FlushDelegatesToFlusher(t *testing.T) {
	t.Parallel()

	fr := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}
	mrw := newMetricsResponseWriter(fr)

	mrw.Flush()

	assert.True(t, fr.flushed, "expected Flush to be delegated to the underlying Flusher")
}

func TestMetricsResponseWriter_FlushNoPanicOnNonFlusher(t *testing.T) {
	t.Parallel()

	// httptest.ResponseRecorder does NOT implement http.Flusher in older Go,
	// but to be safe we use a minimal writer that definitely doesn't.
	mrw := newMetricsResponseWriter(httptest.NewRecorder())

	// This should simply be a no-op, no panic.
	assert.NotPanics(t, func() { mrw.Flush() })
}

// hijackRecorder is an http.ResponseWriter that also implements http.Hijacker.
type hijackRecorder struct {
	http.ResponseWriter
	hijacked bool
}

func (h *hijackRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.hijacked = true
	return nil, nil, nil
}

func TestMetricsResponseWriter_HijackDelegatesToHijacker(t *testing.T) {
	t.Parallel()

	hr := &hijackRecorder{ResponseWriter: httptest.NewRecorder()}
	mrw := newMetricsResponseWriter(hr)

	conn, rw, err := mrw.Hijack()
	require.NoError(t, err)
	assert.Nil(t, conn)
	assert.Nil(t, rw)
	assert.True(t, hr.hijacked)
}

func TestMetricsResponseWriter_HijackReturnsErrOnNonHijacker(t *testing.T) {
	t.Parallel()

	mrw := newMetricsResponseWriter(httptest.NewRecorder())

	_, _, err := mrw.Hijack()
	assert.ErrorIs(t, err, http.ErrNotSupported)
}

// ---------------------------------------------------------------------------
// RecordMatchHit tests
// ---------------------------------------------------------------------------

// TestMetricsRecordMatchHit groups RecordMatchHit tests (global metrics state).
func TestMetricsRecordMatchHit(t *testing.T) {
	t.Run("NilMetricsNoPanic", func(t *testing.T) {
		metrics.Reset()
		t.Cleanup(func() { metrics.Reset() })

		assert.NotPanics(t, func() {
			RecordMatchHit("some-mock-id")
		})
	})

	t.Run("AfterInit", func(t *testing.T) {
		initMetrics(t)

		RecordMatchHit("mock-abc")
		RecordMatchHit("mock-abc")
		RecordMatchHit("mock-xyz")

		val := collectCounterValue(t, metrics.MatchHitsTotal, "mock-abc")
		assert.Equal(t, float64(2), val)

		val2 := collectCounterValue(t, metrics.MatchHitsTotal, "mock-xyz")
		assert.Equal(t, float64(1), val2)
	})
}

// ---------------------------------------------------------------------------
// RecordSSEConnection / RecordWebSocketConnection tests
// ---------------------------------------------------------------------------

// TestMetricsRecordConnections groups SSE and WebSocket connection recording tests.
func TestMetricsRecordConnections(t *testing.T) {
	t.Run("SSE_Smoke", func(t *testing.T) {
		initMetrics(t)

		RecordSSEConnection("/events", 1)
		RecordSSEConnection("/events", 1)
		RecordSSEConnection("/events", -1)

		val := collectGaugeValue(t, metrics.ActiveConnections, "sse")
		assert.Equal(t, float64(1), val)
	})

	t.Run("WebSocket_Smoke", func(t *testing.T) {
		initMetrics(t)

		RecordWebSocketConnection("/ws", 1)

		val := collectGaugeValue(t, metrics.ActiveConnections, "websocket")
		assert.Equal(t, float64(1), val)
	})

	t.Run("SSE_NilMetricsNoPanic", func(t *testing.T) {
		metrics.Reset()
		t.Cleanup(func() { metrics.Reset() })

		assert.NotPanics(t, func() {
			RecordSSEConnection("/events", 1)
		})
	})

	t.Run("WebSocket_NilMetricsNoPanic", func(t *testing.T) {
		metrics.Reset()
		t.Cleanup(func() { metrics.Reset() })

		assert.NotPanics(t, func() {
			RecordWebSocketConnection("/ws", 1)
		})
	})
}

// ---------------------------------------------------------------------------
// UpdateMockCounts tests
// ---------------------------------------------------------------------------

// TestMetricsUpdateMockCounts groups UpdateMockCounts tests.
func TestMetricsUpdateMockCounts(t *testing.T) {
	t.Run("Smoke", func(t *testing.T) {
		initMetrics(t)

		UpdateMockCounts("http", 10, 8)
		UpdateMockCounts("graphql", 3, 2)

		totalHTTP := collectGaugeValue(t, metrics.MocksTotal, "http")
		assert.Equal(t, float64(10), totalHTTP)

		enabledHTTP := collectGaugeValue(t, metrics.MocksEnabled, "http")
		assert.Equal(t, float64(8), enabledHTTP)

		totalGQL := collectGaugeValue(t, metrics.MocksTotal, "graphql")
		assert.Equal(t, float64(3), totalGQL)

		enabledGQL := collectGaugeValue(t, metrics.MocksEnabled, "graphql")
		assert.Equal(t, float64(2), enabledGQL)
	})

	t.Run("NilMetricsNoPanic", func(t *testing.T) {
		metrics.Reset()
		t.Cleanup(func() { metrics.Reset() })

		assert.NotPanics(t, func() {
			UpdateMockCounts("http", 5, 3)
		})
	})
}
