// Package protocol defines the unified interface hierarchy for all mockd protocol handlers.
//
// This package establishes contracts that enable:
//   - Uniform lifecycle management across protocols
//   - Generic Admin API endpoints
//   - Capability-based feature detection
//   - Clean extension points for new protocols
//
// # Interface Hierarchy
//
// The package provides a layered interface hierarchy where Handler is the base
// interface that all protocols must implement:
//
//	Handler (base - all protocols)
//	├── Loggable              - Structured logging support
//	├── RequestLoggable       - Request logging for user inspection
//	├── Observable            - Metrics exposure
//	├── ConnectionManager     - Persistent connection management
//	│   └── GroupableConnections
//	├── Broadcaster           - Message broadcasting
//	│   └── GroupBroadcaster
//	├── Recordable            - Recording support
//	├── HTTPHandler           - HTTP-based protocols
//	│   └── StreamingHTTPHandler
//	├── StandaloneServer      - TCP server protocols
//	└── RPCHandler            - RPC protocols (gRPC, Thrift)
//
// # Basic Usage
//
// Implementing a new protocol handler:
//
//	type MyHandler struct {
//	    id string
//	}
//
//	func (h *MyHandler) Metadata() protocol.Metadata {
//	    return protocol.Metadata{
//	        ID:                   h.id,
//	        Protocol:             protocol.ProtocolWebSocket,
//	        Capabilities:         []protocol.Capability{protocol.CapabilityConnections},
//	        TransportType:        protocol.TransportWebSocket,
//	        ConnectionModel:      protocol.ConnectionModelUpgrade,
//	        CommunicationPattern: protocol.PatternBidirectional,
//	    }
//	}
//
//	func (h *MyHandler) Start(ctx context.Context) error { return nil }
//	func (h *MyHandler) Stop(ctx context.Context, timeout time.Duration) error { return nil }
//	func (h *MyHandler) Health(ctx context.Context) protocol.HealthStatus {
//	    return protocol.HealthStatus{Status: protocol.HealthHealthy, CheckedAt: time.Now()}
//	}
//
// # Registry Usage
//
// The Registry manages all protocol handlers:
//
//	reg := protocol.NewRegistry()
//
//	// Register handlers
//	reg.Register(grpcHandler)
//	reg.Register(mqttHandler)
//	reg.Register(wsHandler)
//
//	// Start all handlers
//	if err := reg.StartAll(ctx); err != nil {
//	    log.Fatal(err)
//	}
//
//	// Query handlers by capability
//	recordableHandlers := reg.ListByCapability(protocol.CapabilityRecording)
//	for _, h := range recordableHandlers {
//	    if r, ok := h.(protocol.Recordable); ok {
//	        r.EnableRecording()
//	    }
//	}
//
// # Capability Detection
//
// Use type assertions to detect optional capabilities:
//
//	handler, ok := reg.Get("mqtt-broker")
//	if !ok {
//	    return errors.New("handler not found")
//	}
//
//	// Check if handler supports connections
//	if cm, ok := handler.(protocol.ConnectionManager); ok {
//	    conns := cm.ListConnections()
//	    fmt.Printf("Active connections: %d\n", len(conns))
//	}
package protocol
