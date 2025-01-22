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
// # Default Metrics
//
// The package provides pre-defined metrics for tracking mock server activity:
//
//   - mockd_requests_total: Counter for all mock requests (labels: method, path, status)
//   - mockd_request_duration_seconds: Histogram for request latency (labels: method, path)
//   - mockd_active_connections: Gauge for active stateful connections (labels: protocol)
//   - mockd_mocks_total: Gauge for configured mocks (labels: type)
//   - mockd_mocks_enabled: Gauge for enabled mocks (labels: type)
//
// # Label Conventions
//
// All labels use consistent lowercase values:
//
//   - method: GET, POST, graphql, grpc, soap, mqtt, sse, websocket
//   - status: numeric (200, 400) for HTTP-like, lowercase codes for gRPC (ok, cancelled)
//   - protocol: websocket, sse, grpc
//
// # Usage
//
//	// Initialize the default metrics registry
//	registry := metrics.Init()
//
//	// HTTP request
//	metrics.RequestsTotal.WithLabels("GET", "/api/users", "200").Inc()
//	metrics.RequestDuration.WithLabels("GET", "/api/users").Observe(0.123)
//
//	// GraphQL request
//	metrics.RequestsTotal.WithLabels("graphql", "/graphql", "200").Inc()
//
//	// gRPC request
//	metrics.RequestsTotal.WithLabels("grpc", "/myservice/MyMethod", "ok").Inc()
//
//	// WebSocket message
//	metrics.RequestsTotal.WithLabels("websocket", "/ws/chat", "inbound").Inc()
//	metrics.ActiveConnections.WithLabels("websocket").Inc()
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
