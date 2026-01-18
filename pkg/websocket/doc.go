// Package websocket provides WebSocket protocol mocking capabilities for mockd.
//
// This package enables bidirectional message exchange over ws:// and wss:// endpoints,
// with support for message matching, scripted conversation scenarios, connection state
// management, subprotocol negotiation, and ping/pong heartbeat handling.
//
// Key features:
//   - WebSocket endpoint configuration and HTTP upgrade handling
//   - Text and binary frame support
//   - Message matching (exact, regex, JSON path)
//   - Scripted message scenarios with timing
//   - Connection state management and tracking
//   - Subprotocol negotiation
//   - Ping/pong heartbeat handling
//   - Broadcast to multiple connections
//
// Usage:
//
//	manager := websocket.NewConnectionManager()
//	endpoint := websocket.NewEndpoint("/ws/chat", &websocket.EndpointConfig{
//		Subprotocols: []string{"json"},
//		Matchers: []websocket.MessageMatcher{
//			{Type: "exact", Value: "ping", Response: &websocket.MessageResponse{Type: "text", Value: "pong"}},
//		},
//	})
//	endpoint.SetManager(manager)
//
//	// In HTTP handler:
//	endpoint.HandleUpgrade(w, r)
//
// The package uses github.com/coder/websocket for the underlying WebSocket protocol
// implementation, which is the only external dependency (justified by Go stdlib lacking
// WebSocket support).
package websocket
