package metrics

import (
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestCounter(t *testing.T) {
	t.Run("without labels", func(t *testing.T) {
		r := NewRegistry()
		c := r.NewCounter("test_counter", "A test counter")

		c.Inc()
		c.Inc()
		c.Add(3)

		samples := c.Collect()
		if len(samples) != 1 {
			t.Fatalf("expected 1 sample, got %d", len(samples))
		}
		if samples[0].Value != 5 {
			t.Errorf("expected value 5, got %f", samples[0].Value)
		}
	})

	t.Run("with labels", func(t *testing.T) {
		r := NewRegistry()
		c := r.NewCounter("http_requests", "Total HTTP requests", "method", "status")

		vec, err := c.WithLabels("GET", "200")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_ = vec.Inc()
		vec, _ = c.WithLabels("GET", "200")
		_ = vec.Inc()
		vec, _ = c.WithLabels("POST", "201")
		_ = vec.Add(5)

		samples := c.Collect()
		if len(samples) != 2 {
			t.Fatalf("expected 2 samples, got %d", len(samples))
		}

		// Check that we have both label combinations
		found := make(map[string]float64)
		for _, s := range samples {
			key := s.Labels["method"] + "_" + s.Labels["status"]
			found[key] = s.Value
		}

		if found["GET_200"] != 2 {
			t.Errorf("expected GET_200=2, got %f", found["GET_200"])
		}
		if found["POST_201"] != 5 {
			t.Errorf("expected POST_201=5, got %f", found["POST_201"])
		}
	})

	t.Run("wrong label count returns error", func(t *testing.T) {
		r := NewRegistry()
		c := r.NewCounter("test", "test", "label1", "label2")
		_, err := c.WithLabels("only_one") // Should return error
		if err == nil {
			t.Error("expected error for wrong label count")
		}
		if !errors.Is(err, ErrLabelCountMismatch) {
			t.Errorf("expected ErrLabelCountMismatch, got %v", err)
		}
	})

	t.Run("negative add returns error", func(t *testing.T) {
		r := NewRegistry()
		c := r.NewCounter("test", "test")
		err := c.Add(-1) // Should return error
		if err == nil {
			t.Error("expected error for negative add")
		}
		if !errors.Is(err, ErrNegativeCounterValue) {
			t.Errorf("expected ErrNegativeCounterValue, got %v", err)
		}
	})
}

func TestGauge(t *testing.T) {
	t.Run("without labels", func(t *testing.T) {
		r := NewRegistry()
		g := r.NewGauge("test_gauge", "A test gauge")

		g.Set(10)
		samples := g.Collect()
		if len(samples) != 1 || samples[0].Value != 10 {
			t.Errorf("expected value 10")
		}

		g.Inc()
		samples = g.Collect()
		if samples[0].Value != 11 {
			t.Errorf("expected value 11, got %f", samples[0].Value)
		}

		g.Dec()
		g.Dec()
		samples = g.Collect()
		if samples[0].Value != 9 {
			t.Errorf("expected value 9, got %f", samples[0].Value)
		}

		g.Add(-5)
		samples = g.Collect()
		if samples[0].Value != 4 {
			t.Errorf("expected value 4, got %f", samples[0].Value)
		}
	})

	t.Run("with labels", func(t *testing.T) {
		r := NewRegistry()
		g := r.NewGauge("active_connections", "Active connections", "protocol")

		vec, err := g.WithLabels("http")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		vec.Set(100)
		vec, _ = g.WithLabels("websocket")
		vec.Set(50)
		vec, _ = g.WithLabels("http")
		vec.Inc()

		samples := g.Collect()
		if len(samples) != 2 {
			t.Fatalf("expected 2 samples, got %d", len(samples))
		}

		found := make(map[string]float64)
		for _, s := range samples {
			found[s.Labels["protocol"]] = s.Value
		}

		if found["http"] != 101 {
			t.Errorf("expected http=101, got %f", found["http"])
		}
		if found["websocket"] != 50 {
			t.Errorf("expected websocket=50, got %f", found["websocket"])
		}
	})
}

func TestHistogram(t *testing.T) {
	t.Run("basic histogram", func(t *testing.T) {
		r := NewRegistry()
		h := r.NewHistogram("request_duration", "Request duration", []float64{0.1, 0.5, 1.0})

		h.Observe(0.05) // Should go in 0.1 bucket
		h.Observe(0.3)  // Should go in 0.5 bucket
		h.Observe(0.8)  // Should go in 1.0 bucket
		h.Observe(2.0)  // Should go in +Inf bucket

		samples := h.Collect()

		// Should have 4 buckets (0.1, 0.5, 1.0, +Inf) + _sum + _count = 6 samples
		if len(samples) != 6 {
			t.Fatalf("expected 6 samples, got %d", len(samples))
		}

		// Check buckets are cumulative
		bucketValues := make(map[string]float64)
		var sum, count float64
		for _, s := range samples {
			if strings.HasSuffix(s.Name, "_bucket") {
				bucketValues[s.Labels["le"]] = s.Value
			} else if strings.HasSuffix(s.Name, "_sum") {
				sum = s.Value
			} else if strings.HasSuffix(s.Name, "_count") {
				count = s.Value
			}
		}

		// Check cumulative counts
		if bucketValues["0.1"] != 1 {
			t.Errorf("expected le=0.1 count=1, got %f", bucketValues["0.1"])
		}
		if bucketValues["0.5"] != 2 {
			t.Errorf("expected le=0.5 count=2, got %f", bucketValues["0.5"])
		}
		if bucketValues["1"] != 3 {
			t.Errorf("expected le=1 count=3, got %f", bucketValues["1"])
		}
		if bucketValues["+Inf"] != 4 {
			t.Errorf("expected le=+Inf count=4, got %f", bucketValues["+Inf"])
		}

		// Check sum and count
		expectedSum := 0.05 + 0.3 + 0.8 + 2.0
		if sum != expectedSum {
			t.Errorf("expected sum=%f, got %f", expectedSum, sum)
		}
		if count != 4 {
			t.Errorf("expected count=4, got %f", count)
		}
	})

	t.Run("with labels", func(t *testing.T) {
		r := NewRegistry()
		h := r.NewHistogram("http_duration", "HTTP duration", []float64{0.1, 1.0}, "method")

		vec, err := h.WithLabels("GET")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		vec.Observe(0.05)
		vec, _ = h.WithLabels("POST")
		vec.Observe(0.5)

		samples := h.Collect()
		// 2 label combinations * (2 buckets + 1 inf + sum + count) = 2 * 5 = 10
		if len(samples) != 10 {
			t.Fatalf("expected 10 samples, got %d", len(samples))
		}
	})
}

func TestRegistry_Handler(t *testing.T) {
	r := NewRegistry()

	c := r.NewCounter("test_requests_total", "Total requests", "method")
	g := r.NewGauge("test_active", "Active items")
	h := r.NewHistogram("test_duration_seconds", "Duration", []float64{0.1, 1.0})

	vec, _ := c.WithLabels("GET")
	_ = vec.Inc()
	vec, _ = c.WithLabels("POST")
	_ = vec.Add(5)
	_ = g.Set(42)
	_ = h.Observe(0.5)

	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()

	r.Handler().ServeHTTP(rec, req)

	resp := rec.Result()
	body, _ := io.ReadAll(resp.Body)
	output := string(body)

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/plain; version=0.0.4; charset=utf-8" {
		t.Errorf("unexpected Content-Type: %s", contentType)
	}

	// Check for expected lines
	expectedLines := []string{
		"# HELP test_requests_total Total requests",
		"# TYPE test_requests_total counter",
		`test_requests_total{method="GET"} 1`,
		`test_requests_total{method="POST"} 5`,
		"# HELP test_active Active items",
		"# TYPE test_active gauge",
		"test_active 42",
		"# HELP test_duration_seconds Duration",
		"# TYPE test_duration_seconds histogram",
		`test_duration_seconds_bucket{le="0.1"} 0`,
		`test_duration_seconds_bucket{le="1"} 1`,
		`test_duration_seconds_bucket{le="+Inf"} 1`,
		"test_duration_seconds_sum 0.5",
		"test_duration_seconds_count 1",
	}

	for _, expected := range expectedLines {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing expected line: %s", expected)
		}
	}
}

func TestConcurrency(t *testing.T) {
	r := NewRegistry()
	c := r.NewCounter("concurrent_counter", "Test counter", "worker")
	g := r.NewGauge("concurrent_gauge", "Test gauge")
	h := r.NewHistogram("concurrent_histogram", "Test histogram", []float64{1, 10, 100})

	var wg sync.WaitGroup
	workers := 100
	iterations := 1000

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			workerID := "worker"
			for j := 0; j < iterations; j++ {
				vec, _ := c.WithLabels(workerID)
				_ = vec.Inc()
				_ = g.Inc()
				_ = h.Observe(float64(j % 50))
			}
		}(i)
	}

	wg.Wait()

	// Verify counter
	samples := c.Collect()
	total := float64(0)
	for _, s := range samples {
		total += s.Value
	}
	expected := float64(workers * iterations)
	if total != expected {
		t.Errorf("expected counter total %f, got %f", expected, total)
	}

	// Verify gauge
	samples = g.Collect()
	if len(samples) != 1 || samples[0].Value != expected {
		t.Errorf("expected gauge value %f, got %f", expected, samples[0].Value)
	}

	// Verify histogram count
	samples = h.Collect()
	for _, s := range samples {
		if strings.HasSuffix(s.Name, "_count") {
			if s.Value != expected {
				t.Errorf("expected histogram count %f, got %f", expected, s.Value)
			}
		}
	}
}

func TestDefaultMetrics(t *testing.T) {
	// Reset to ensure clean state
	Reset()

	// Init should create all default metrics
	registry := Init()
	if registry == nil {
		t.Fatal("Init() returned nil")
	}

	// Verify default metrics are set
	if RequestsTotal == nil {
		t.Error("RequestsTotal is nil")
	}
	if RequestDuration == nil {
		t.Error("RequestDuration is nil")
	}
	if MocksTotal == nil {
		t.Error("MocksTotal is nil")
	}
	if MocksEnabled == nil {
		t.Error("MocksEnabled is nil")
	}
	if ActiveConnections == nil {
		t.Error("ActiveConnections is nil")
	}
	if AdminRequestsTotal == nil {
		t.Error("AdminRequestsTotal is nil")
	}

	// Test using default metrics
	if vec, err := RequestsTotal.WithLabels("GET", "/api/users", "200"); err == nil {
		_ = vec.Inc()
	}
	if vec, err := RequestDuration.WithLabels("GET", "/api/users"); err == nil {
		vec.Observe(0.123)
	}
	if vec, err := MocksTotal.WithLabels("http"); err == nil {
		vec.Set(10)
	}
	if vec, err := ActiveConnections.WithLabels("websocket"); err == nil {
		vec.Inc()
	}

	// Verify through handler
	req := httptest.NewRequest("GET", "/metrics", nil)
	rec := httptest.NewRecorder()
	registry.Handler().ServeHTTP(rec, req)

	body, _ := io.ReadAll(rec.Result().Body)
	output := string(body)

	if !strings.Contains(output, "mockd_requests_total") {
		t.Error("output missing mockd_requests_total")
	}
	if !strings.Contains(output, "mockd_request_duration_seconds") {
		t.Error("output missing mockd_request_duration_seconds")
	}
	if !strings.Contains(output, "mockd_mocks_total") {
		t.Error("output missing mockd_mocks_total")
	}

	// Calling Init() again should return the same registry
	registry2 := Init()
	if registry2 != registry {
		t.Error("Init() should return the same registry on subsequent calls")
	}
}

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		value    float64
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{0.5, "0.5"},
		{0.123456789, "0.123456789"},
		{1e10, "1e+10"},
	}

	for _, tt := range tests {
		got := formatFloat(tt.value)
		if got != tt.expected {
			t.Errorf("formatFloat(%v) = %q, want %q", tt.value, got, tt.expected)
		}
	}
}

func TestEscapeLabelValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{`with "quotes"`, `with \"quotes\"`},
		{"with\nnewline", `with\nnewline`},
		{`back\\slash`, `back\\\\slash`},
	}

	for _, tt := range tests {
		got := escapeLabelValue(tt.input)
		if got != tt.expected {
			t.Errorf("escapeLabelValue(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestDefaultRegistry(t *testing.T) {
	Reset()

	// Before Init, DefaultRegistry should return nil
	if DefaultRegistry() != nil {
		t.Error("DefaultRegistry() should return nil before Init()")
	}

	Init()

	// After Init, DefaultRegistry should return the registry
	if DefaultRegistry() == nil {
		t.Error("DefaultRegistry() should return the registry after Init()")
	}
}

func BenchmarkCounterInc(b *testing.B) {
	r := NewRegistry()
	c := r.NewCounter("bench_counter", "Benchmark counter")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Inc()
		}
	})
}

func BenchmarkCounterWithLabels(b *testing.B) {
	r := NewRegistry()
	c := r.NewCounter("bench_counter", "Benchmark counter", "method", "status")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			vec, _ := c.WithLabels("GET", "200")
			_ = vec.Inc()
		}
	})
}

func BenchmarkHistogramObserve(b *testing.B) {
	r := NewRegistry()
	h := r.NewHistogram("bench_histogram", "Benchmark histogram", DefaultBuckets)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			h.Observe(float64(i%1000) / 1000.0)
			i++
		}
	})
}

func BenchmarkHandler(b *testing.B) {
	r := NewRegistry()

	// Create some metrics with data
	c := r.NewCounter("test_counter", "Test", "method", "status")
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		for _, status := range []string{"200", "201", "400", "404", "500"} {
			vec, _ := c.WithLabels(method, status)
			_ = vec.Add(100)
		}
	}

	g := r.NewGauge("test_gauge", "Test", "type")
	for _, tp := range []string{"http", "websocket", "grpc"} {
		vec, _ := g.WithLabels(tp)
		vec.Set(50)
	}

	h := r.NewHistogram("test_histogram", "Test", DefaultBuckets, "method")
	for _, method := range []string{"GET", "POST"} {
		for i := 0; i < 100; i++ {
			vec, _ := h.WithLabels(method)
			vec.Observe(float64(i) / 1000.0)
		}
	}

	handler := r.Handler()
	req := httptest.NewRequest("GET", "/metrics", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}
}
