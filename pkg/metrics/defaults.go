package metrics

import (
	"sync"
	"time"
)

// Default metrics for the mock server.
// These are initialized by calling Init().
//
// # Label Conventions
//
// All metric labels use lowercase values for consistency:
//
// ## method label values
//   - HTTP: GET, POST, PUT, DELETE, PATCH, etc. (uppercase HTTP methods)
//   - graphql: GraphQL operations
//   - grpc: gRPC calls
//   - soap: SOAP operations
//   - mqtt: MQTT messages
//   - sse: Server-Sent Events
//   - websocket: WebSocket messages
//
// ## status label values
//   - HTTP/GraphQL/SOAP: numeric codes (200, 400, 500, etc.)
//   - gRPC: lowercase gRPC codes (ok, cancelled, unknown, invalid_argument, etc.)
//   - MQTT: ok
//   - SSE: event
//   - WebSocket: inbound, outbound
//
// ## protocol label values (for ActiveConnections)
//   - websocket, sse, grpc (all lowercase)
var (
	// RequestsTotal counts the total number of mock requests.
	// Labels: method, path, status
	// See label conventions above for allowed values.
	RequestsTotal *Counter

	// RequestDuration tracks the duration of mock requests in seconds.
	// Labels: method, path
	// See label conventions above for allowed method values.
	RequestDuration *Histogram

	// MocksTotal is a gauge of the total number of mocks configured.
	// Labels: type (http, websocket, graphql, grpc, soap, mqtt)
	MocksTotal *Gauge

	// MocksEnabled is a gauge of the number of enabled mocks.
	// Labels: type (same values as MocksTotal)
	MocksEnabled *Gauge

	// ActiveConnections tracks the number of active connections.
	// Labels: protocol (websocket, sse, grpc - all lowercase)
	// Note: Only tracks stateful connections. HTTP is stateless.
	ActiveConnections *Gauge

	// AdminRequestsTotal counts the total number of admin API requests.
	// Labels: method, path, status
	AdminRequestsTotal *Counter

	// AdminRequestDuration tracks the duration of admin API requests in seconds.
	// Labels: method, path
	AdminRequestDuration *Histogram

	// RecordingsTotal is a gauge of the total number of recordings.
	// Labels: type (http, websocket, sse, grpc, mqtt)
	RecordingsTotal *Gauge

	// ProxyRequestsTotal counts the total number of proxied requests.
	// Labels: method, status
	ProxyRequestsTotal *Counter

	// MatchHitsTotal counts the number of times each mock was matched.
	// Labels: mock_id
	MatchHitsTotal *Counter

	// MatchMissesTotal counts requests that didn't match any mock.
	MatchMissesTotal *Counter

	// ErrorsTotal counts errors by type.
	// Labels: type (timeout, connection, validation, internal)
	ErrorsTotal *Counter

	// UptimeSeconds is a gauge of the server uptime in seconds.
	UptimeSeconds *Gauge

	// PortInfo is a gauge that exposes information about ports in use.
	// Labels: port, protocol, component
	// Value is 1 if the port is running, 0 otherwise.
	PortInfo *Gauge

	// RuntimeCollectorInstance is the Go runtime metrics collector.
	RuntimeCollectorInstance *RuntimeCollector

	// runtimeCollectorStop stops the runtime collector goroutine.
	runtimeCollectorStop func()

	// defaultRegistry is the global metrics registry.
	defaultRegistry *Registry

	// initOnce ensures Init() is only called once.
	initOnce sync.Once
)

// Init initializes the default metrics and returns the registry.
// This function is idempotent and safe to call multiple times.
func Init() *Registry {
	initOnce.Do(func() {
		defaultRegistry = NewRegistry()

		// Request metrics
		RequestsTotal = defaultRegistry.NewCounter(
			"mockd_requests_total",
			"Total number of mock requests",
			"method", "path", "status",
		)

		RequestDuration = defaultRegistry.NewHistogram(
			"mockd_request_duration_seconds",
			"Duration of mock requests in seconds",
			DefaultBuckets,
			"method", "path",
		)

		// Mock metrics
		MocksTotal = defaultRegistry.NewGauge(
			"mockd_mocks_total",
			"Total number of mocks configured",
			"type",
		)

		MocksEnabled = defaultRegistry.NewGauge(
			"mockd_mocks_enabled",
			"Number of enabled mocks",
			"type",
		)

		// Connection metrics
		ActiveConnections = defaultRegistry.NewGauge(
			"mockd_active_connections",
			"Number of active connections",
			"protocol",
		)

		// Admin API metrics
		AdminRequestsTotal = defaultRegistry.NewCounter(
			"mockd_admin_requests_total",
			"Total number of admin API requests",
			"method", "path", "status",
		)

		AdminRequestDuration = defaultRegistry.NewHistogram(
			"mockd_admin_request_duration_seconds",
			"Duration of admin API requests in seconds",
			DefaultBuckets,
			"method", "path",
		)

		// Recording metrics
		RecordingsTotal = defaultRegistry.NewGauge(
			"mockd_recordings_total",
			"Total number of recordings",
			"type",
		)

		// Proxy metrics
		ProxyRequestsTotal = defaultRegistry.NewCounter(
			"mockd_proxy_requests_total",
			"Total number of proxied requests",
			"method", "status",
		)

		// Match metrics
		MatchHitsTotal = defaultRegistry.NewCounter(
			"mockd_match_hits_total",
			"Number of times mocks were matched",
			"mock_id",
		)

		MatchMissesTotal = defaultRegistry.NewCounter(
			"mockd_match_misses_total",
			"Number of requests that did not match any mock",
		)

		// Error metrics
		ErrorsTotal = defaultRegistry.NewCounter(
			"mockd_errors_total",
			"Total number of errors by type",
			"type",
		)

		// Uptime metric
		UptimeSeconds = defaultRegistry.NewGauge(
			"mockd_uptime_seconds",
			"Server uptime in seconds",
		)

		// Port info metric
		PortInfo = defaultRegistry.NewGauge(
			"mockd_port_info",
			"Information about ports in use by mockd (1=running, 0=stopped)",
			"port", "protocol", "component",
		)

		// Initialize Go runtime metrics collector (passing UptimeSeconds for it to update)
		RuntimeCollectorInstance = NewRuntimeCollector(defaultRegistry, UptimeSeconds)
		// Start collecting runtime metrics every 10 seconds
		runtimeCollectorStop = RuntimeCollectorInstance.StartCollector(10 * time.Second)
	})

	return defaultRegistry
}

// DefaultRegistry returns the default metrics registry.
// Returns nil if Init() has not been called.
func DefaultRegistry() *Registry {
	return defaultRegistry
}

// Reset resets all default metrics. Useful for testing.
// This also resets the initOnce, allowing Init() to be called again.
func Reset() {
	// Stop runtime collector if running
	if runtimeCollectorStop != nil {
		runtimeCollectorStop()
		runtimeCollectorStop = nil
	}

	initOnce = sync.Once{}
	defaultRegistry = nil
	RequestsTotal = nil
	RequestDuration = nil
	MocksTotal = nil
	MocksEnabled = nil
	ActiveConnections = nil
	AdminRequestsTotal = nil
	AdminRequestDuration = nil
	RecordingsTotal = nil
	ProxyRequestsTotal = nil
	MatchHitsTotal = nil
	MatchMissesTotal = nil
	ErrorsTotal = nil
	UptimeSeconds = nil
	PortInfo = nil
	RuntimeCollectorInstance = nil
}
