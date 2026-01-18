package websocket

import (
	"context"
	"net/http"
	"strings"
	"time"

	ws "github.com/coder/websocket"
)

// HandleUpgrade handles an HTTP request upgrade to WebSocket.
// This is the main entry point for WebSocket connections.
func (e *Endpoint) HandleUpgrade(w http.ResponseWriter, r *http.Request) error {
	// Check if endpoint is enabled
	if !e.Enabled() {
		http.Error(w, "endpoint is disabled", http.StatusServiceUnavailable)
		return ErrEndpointDisabled
	}

	// Check if we can accept new connections
	if !e.CanAccept() {
		http.Error(w, "maximum connections reached", http.StatusServiceUnavailable)
		return ErrMaxConnectionsReached
	}

	// Parse client subprotocols
	var clientProtocols []string
	if proto := r.Header.Get("Sec-WebSocket-Protocol"); proto != "" {
		for _, p := range strings.Split(proto, ",") {
			clientProtocols = append(clientProtocols, strings.TrimSpace(p))
		}
	}

	// Negotiate subprotocol
	negotiatedProtocol, err := e.NegotiateSubprotocol(clientProtocols)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return err
	}

	// Accept options
	acceptOpts := &ws.AcceptOptions{
		Subprotocols:       e.subprotocols,
		InsecureSkipVerify: true,                   // Allow any origin for mocking
		CompressionMode:    ws.CompressionDisabled, // Per spec, no compression in v1
	}

	// Accept the WebSocket connection
	wsConn, err := ws.Accept(w, r, acceptOpts)
	if err != nil {
		return err
	}

	// Set message size limit
	wsConn.SetReadLimit(e.maxMessageSize)

	// Create our connection wrapper
	conn := NewConnection(wsConn, e, negotiatedProtocol, r)

	// Add to endpoint
	e.AddConnection(conn)

	// Add to manager if available
	if e.manager != nil {
		e.manager.Add(conn)
		// Log connection open
		e.manager.LogConnect(conn, r.RemoteAddr)
	}

	// Start connection handling in a goroutine
	go e.handleConnection(conn)

	return nil
}

// handleConnection handles the lifecycle of a WebSocket connection.
func (e *Endpoint) handleConnection(conn *Connection) {
	var closeCode CloseCode = CloseNormalClosure

	defer func() {
		// Log disconnect before cleanup
		if e.manager != nil {
			remoteAddr := ""
			if addr, ok := conn.Metadata()["remoteAddr"].(string); ok {
				remoteAddr = addr
			}
			e.manager.LogDisconnect(conn, closeCode, remoteAddr)
		}

		// Cleanup on exit
		e.RemoveConnection(conn.ID())
		if e.manager != nil {
			e.manager.Remove(conn.ID())
		}
		conn.CloseNormal()
	}()

	// Start scenario executor if configured
	var executor *ScenarioExecutor
	if e.scenario != nil {
		executor = NewScenarioExecutor(conn, e.scenario)
		go executor.Run()
		defer executor.Stop()
	}

	// Start heartbeat if configured
	var heartbeatCancel context.CancelFunc
	if e.heartbeat != nil && e.heartbeat.Enabled {
		var heartbeatCtx context.Context
		heartbeatCtx, heartbeatCancel = context.WithCancel(conn.Context())
		go e.runHeartbeat(heartbeatCtx, conn)
	}
	if heartbeatCancel != nil {
		defer heartbeatCancel()
	}

	// Start idle timeout watcher if configured
	var idleCancel context.CancelFunc
	if e.idleTimeout > 0 {
		var idleCtx context.Context
		idleCtx, idleCancel = context.WithCancel(conn.Context())
		go e.watchIdleTimeout(idleCtx, conn)
	}
	if idleCancel != nil {
		defer idleCancel()
	}

	// Get remote address for logging
	remoteAddr := ""
	if addr, ok := conn.Metadata()["remoteAddr"].(string); ok {
		remoteAddr = addr
	}

	// Read loop
	for {
		select {
		case <-conn.Context().Done():
			return
		default:
		}

		msgType, data, err := conn.Read()
		if err != nil {
			// Connection closed or error
			return
		}

		// Log inbound message
		if e.manager != nil {
			e.manager.LogMessageReceived(conn, msgType, data, remoteAddr)
		}

		// Handle the message
		e.handleMessage(conn, msgType, data, executor)
	}
}

// handleMessage processes an incoming message.
func (e *Endpoint) handleMessage(conn *Connection, msgType MessageType, data []byte, executor *ScenarioExecutor) {
	// Let scenario handle it first if active
	if executor != nil {
		if executor.HandleMessage(msgType, data) {
			// Scenario consumed the message
			return
		}
	}

	// Try to match against matchers
	response := e.MatchMessage(msgType, data)
	if response != nil {
		// Send matched response
		e.sendResponse(conn, response)
		return
	}

	// Echo mode - send back the same message
	if e.echoMode {
		conn.Send(msgType, data)
		return
	}

	// No response configured and echo mode disabled - silently ignore
}

// sendResponse sends a MessageResponse to the connection.
func (e *Endpoint) sendResponse(conn *Connection, response *MessageResponse) {
	// Apply delay if specified
	if response.Delay > 0 {
		time.Sleep(response.Delay.Duration())
	}

	data, msgType, err := response.GetData()
	if err != nil {
		return
	}

	conn.Send(msgType, data)
}

// runHeartbeat sends periodic pings to keep the connection alive.
func (e *Endpoint) runHeartbeat(ctx context.Context, conn *Connection) {
	interval := e.heartbeat.Interval.Duration()
	if interval == 0 {
		interval = 30 * time.Second
	}

	timeout := e.heartbeat.Timeout.Duration()
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send ping with timeout
			pingCtx, cancel := context.WithTimeout(ctx, timeout)
			err := conn.Ping(pingCtx)
			cancel()

			if err != nil {
				// Ping failed - close connection
				conn.Close(CloseGoingAway, "ping timeout")
				return
			}
		}
	}
}

// watchIdleTimeout closes the connection if idle for too long.
func (e *Endpoint) watchIdleTimeout(ctx context.Context, conn *Connection) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			idleTime := time.Since(conn.LastMessageAt())
			if idleTime > e.idleTimeout {
				conn.Close(CloseGoingAway, "idle timeout")
				return
			}
		}
	}
}

// WebSocketHandler is an http.Handler that routes WebSocket requests to endpoints.
type WebSocketHandler struct {
	manager *ConnectionManager
}

// NewWebSocketHandler creates a new WebSocketHandler.
func NewWebSocketHandler(manager *ConnectionManager) *WebSocketHandler {
	return &WebSocketHandler{
		manager: manager,
	}
}

// ServeHTTP implements http.Handler.
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Check for WebSocket upgrade
	if !isWebSocketUpgrade(r) {
		http.Error(w, "WebSocket upgrade required", http.StatusBadRequest)
		return
	}

	// Find matching endpoint
	endpoint := h.manager.GetEndpoint(r.URL.Path)
	if endpoint == nil {
		http.Error(w, "WebSocket endpoint not found", http.StatusNotFound)
		return
	}

	// Check if endpoint is enabled
	if !endpoint.Enabled() {
		http.Error(w, "WebSocket endpoint is disabled", http.StatusServiceUnavailable)
		return
	}

	// Handle upgrade
	if err := endpoint.HandleUpgrade(w, r); err != nil {
		// Error already written to response
		return
	}
}

// isWebSocketUpgrade checks if the request is a WebSocket upgrade request.
func isWebSocketUpgrade(r *http.Request) bool {
	// Check Connection header
	conn := r.Header.Get("Connection")
	if !strings.Contains(strings.ToLower(conn), "upgrade") {
		return false
	}

	// Check Upgrade header
	upgrade := r.Header.Get("Upgrade")
	if strings.ToLower(upgrade) != "websocket" {
		return false
	}

	return true
}

// IsWebSocketRequest returns true if the request is a WebSocket upgrade request.
func IsWebSocketRequest(r *http.Request) bool {
	return isWebSocketUpgrade(r)
}
