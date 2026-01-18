// Package metrics provides Prometheus-compatible metrics collection for the mock server.
//
// This package implements the Prometheus text exposition format (text/plain; version=0.0.4)
// without any external dependencies, using only the standard library.
//
// Supported metric types:
//   - Counter: monotonically increasing value (e.g., request counts)
//   - Gauge: value that can go up or down (e.g., active connections)
//   - Histogram: distribution of values with configurable buckets (e.g., latencies)
//
// All metrics are thread-safe and can be updated from multiple goroutines.
//
// Usage:
//
//	// Initialize the default metrics registry
//	registry := metrics.Init()
//
//	// Access default metrics
//	metrics.RequestsTotal.WithLabels("GET", "/api/users", "200").Inc()
//	metrics.RequestDuration.WithLabels("GET", "/api/users").Observe(0.123)
//
//	// Register the /metrics endpoint
//	http.Handle("/metrics", registry.Handler())
//
// Custom metrics can also be created:
//
//	registry := metrics.NewRegistry()
//	counter := registry.NewCounter("my_counter", "Description of counter", "label1", "label2")
//	counter.WithLabels("value1", "value2").Inc()
package metrics
