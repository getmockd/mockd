// Package quic provides a QUIC-based tunnel client.
package quic

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"

	"github.com/getmockd/mockd/pkg/tunnel/protocol"
)

// Client is a QUIC tunnel client that connects to the relay server.
type Client struct {
	conn          quic.Connection
	controlStream quic.Stream

	// Configuration
	relayAddr   string
	token       string
	localPort   int
	tlsInsecure bool
	tunnelAuth  *protocol.TunnelAuth

	// Connection info (set after auth)
	sessionID string
	subdomain string
	publicURL string

	// Local HTTP handler
	handler http.Handler

	// Stats
	requestCount atomic.Int64

	// Lifecycle
	connected atomic.Bool
	closed    atomic.Bool
	mu        sync.Mutex

	logger *slog.Logger

	// Callbacks
	OnConnect    func(publicURL string)
	OnDisconnect func(err error)
	OnRequest    func(method, path string)
	OnGoaway     func(payload protocol.GoawayPayload)
}

// ClientConfig configures the QUIC client.
type ClientConfig struct {
	RelayAddr   string               // e.g., "relay.mockd.io:4443"
	Token       string               // Auth token
	LocalPort   int                  // Local port being tunneled
	Handler     http.Handler         // HTTP handler for incoming requests
	TLSInsecure bool                 // Skip TLS verification (for testing)
	TunnelAuth  *protocol.TunnelAuth // Incoming request auth config (optional)
	Logger      *slog.Logger
}

// NewClient creates a new QUIC tunnel client.
func NewClient(cfg *ClientConfig) *Client {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		relayAddr:   cfg.RelayAddr,
		token:       cfg.Token,
		localPort:   cfg.LocalPort,
		handler:     cfg.Handler,
		tlsInsecure: cfg.TLSInsecure,
		tunnelAuth:  cfg.TunnelAuth,
		logger:      logger,
	}
}

// Connect establishes a QUIC connection to the relay and authenticates.
func (c *Client) Connect(ctx context.Context) error {
	if c.connected.Load() {
		return fmt.Errorf("already connected")
	}

	c.logger.Info("connecting to relay", "addr", c.relayAddr)

	// Configure TLS
	tlsConfig := &tls.Config{
		NextProtos:         []string{"mockd-relay"},
		InsecureSkipVerify: c.tlsInsecure, //nolint:gosec // InsecureSkipVerify is intentionally configurable for dev/testing
	}

	// Configure QUIC
	quicConfig := &quic.Config{
		MaxIdleTimeout:  30 * time.Second,
		KeepAlivePeriod: 10 * time.Second,
		Allow0RTT:       true,
	}

	// Connect
	conn, err := quic.DialAddr(ctx, c.relayAddr, tlsConfig, quicConfig)
	if err != nil {
		return fmt.Errorf("dial relay: %w", err)
	}

	c.conn = conn

	// Open control stream
	controlStream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		_ = conn.CloseWithError(1, "failed to open control stream")
		return fmt.Errorf("open control stream: %w", err)
	}

	c.controlStream = controlStream

	// Send auth message
	authPayload := protocol.AuthPayload{
		Token:      c.token,
		LocalPort:  c.localPort,
		TunnelAuth: c.tunnelAuth,
	}

	authMsg := &protocol.ControlMessage{
		Type:    protocol.ControlTypeAuth,
		Payload: authPayload,
	}

	msgBytes, err := protocol.EncodeControlMessage(authMsg)
	if err != nil {
		_ = conn.CloseWithError(2, "failed to encode auth")
		return fmt.Errorf("encode auth: %w", err)
	}

	header := &protocol.StreamHeader{
		Version:  protocol.ProtocolVersion,
		Type:     protocol.StreamTypeControl,
		Metadata: msgBytes,
	}

	if err := protocol.EncodeHeader(controlStream, header); err != nil {
		_ = conn.CloseWithError(3, "failed to send auth")
		return fmt.Errorf("send auth: %w", err)
	}

	// Read auth response
	respHeader, err := protocol.DecodeHeader(controlStream)
	if err != nil {
		_ = conn.CloseWithError(4, "failed to read auth response")
		return fmt.Errorf("read auth response: %w", err)
	}

	respMsg, err := protocol.DecodeControlMessage(respHeader.Metadata)
	if err != nil {
		_ = conn.CloseWithError(5, "invalid auth response")
		return fmt.Errorf("decode auth response: %w", err)
	}

	if respMsg.Type == protocol.ControlTypeAuthError {
		payloadBytes, _ := json.Marshal(respMsg.Payload)
		var errPayload protocol.AuthErrorPayload
		_ = json.Unmarshal(payloadBytes, &errPayload)
		_ = conn.CloseWithError(6, "auth failed")
		return fmt.Errorf("auth failed: %s - %s", errPayload.Code, errPayload.Message)
	}

	if respMsg.Type != protocol.ControlTypeAuthOK {
		_ = conn.CloseWithError(7, "unexpected response")
		return fmt.Errorf("unexpected auth response: %s", respMsg.Type)
	}

	// Parse auth OK payload
	payloadBytes, _ := json.Marshal(respMsg.Payload)
	var okPayload protocol.AuthOKPayload
	if err := json.Unmarshal(payloadBytes, &okPayload); err != nil {
		_ = conn.CloseWithError(8, "invalid auth ok payload")
		return fmt.Errorf("decode auth ok: %w", err)
	}

	c.sessionID = okPayload.SessionID
	c.subdomain = okPayload.Subdomain
	c.publicURL = okPayload.PublicURL
	c.connected.Store(true)

	// Start control stream reader for GOAWAY, ping/pong, etc.
	go c.readControlStream()

	c.logger.Info("connected to relay",
		"session", c.sessionID,
		"subdomain", c.subdomain,
		"public_url", c.publicURL,
	)

	if c.OnConnect != nil {
		c.OnConnect(c.publicURL)
	}

	return nil
}

// Run starts accepting incoming requests. Blocks until disconnected.
func (c *Client) Run(ctx context.Context) error {
	if !c.connected.Load() {
		return fmt.Errorf("not connected")
	}

	// Accept incoming streams from relay
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		stream, err := c.conn.AcceptStream(ctx)
		if err != nil {
			if c.closed.Load() {
				return nil
			}
			c.logger.Error("accept stream error", "error", err)
			c.handleDisconnect(err)
			return err
		}

		go c.handleStream(ctx, stream)
	}
}

// handleStream processes an incoming QUIC stream (HTTP request from relay).
func (c *Client) handleStream(ctx context.Context, stream quic.Stream) {
	defer func() { _ = stream.Close() }()

	// Read request header
	header, err := protocol.DecodeHeader(stream)
	if err != nil {
		c.logger.Error("decode header error", "error", err)
		return
	}

	if header.Type != protocol.StreamTypeHTTP {
		c.logger.Warn("unexpected stream type", "type", header.Type)
		return
	}

	// Decode HTTP metadata
	meta, err := protocol.DecodeHTTPMetadata(header.Metadata)
	if err != nil {
		c.logger.Error("decode http metadata error", "error", err)
		return
	}

	if c.OnRequest != nil {
		c.OnRequest(meta.Method, meta.Path)
	}

	// Build HTTP request
	req, err := http.NewRequestWithContext(ctx, meta.Method, meta.Path, stream)
	if err != nil {
		c.logger.Error("build request error", "error", err)
		c.sendErrorResponse(stream, http.StatusBadRequest, "Bad Request")
		return
	}

	req.Host = meta.Host
	for key, value := range meta.Header {
		req.Header.Set(key, value)
	}

	// Create response recorder
	rw := &responseWriter{
		stream:     stream,
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}

	// Call handler
	c.handler.ServeHTTP(rw, req)

	// Ensure response is flushed
	if !rw.headerWritten {
		rw.WriteHeader(http.StatusOK)
	}

	c.requestCount.Add(1)
}

// sendErrorResponse sends an error response back through the stream.
func (c *Client) sendErrorResponse(stream quic.Stream, status int, message string) {
	meta := &protocol.HTTPMetadata{
		StatusCode: status,
		Header: map[string]string{
			"Content-Type": "text/plain",
		},
	}

	metaBytes, _ := protocol.EncodeHTTPMetadata(meta)
	header := &protocol.StreamHeader{
		Version:  protocol.ProtocolVersion,
		Type:     protocol.StreamTypeHTTP,
		Metadata: metaBytes,
	}

	_ = protocol.EncodeHeader(stream, header)
	_, _ = stream.Write([]byte(message))
}

// readControlStream reads control messages from the relay after auth.
// Runs as a goroutine for the lifetime of the connection.
func (c *Client) readControlStream() {
	for {
		if c.closed.Load() || !c.connected.Load() {
			return
		}

		header, err := protocol.DecodeHeader(c.controlStream)
		if err != nil {
			if c.closed.Load() {
				return // Expected during shutdown
			}
			c.logger.Debug("control stream read error", "error", err)
			return
		}

		if header.Type != protocol.StreamTypeControl {
			c.logger.Warn("unexpected type on control stream", "type", header.Type)
			continue
		}

		msg, err := protocol.DecodeControlMessage(header.Metadata)
		if err != nil {
			c.logger.Warn("failed to decode control message", "error", err)
			continue
		}

		switch msg.Type {
		case protocol.ControlTypeGoaway:
			c.handleGoaway(msg)
		case protocol.ControlTypePing:
			// Future: respond with pong
			c.logger.Debug("received ping")
		case protocol.ControlTypeDisconnect:
			c.logger.Info("relay requested disconnect")
			c.handleDisconnect(fmt.Errorf("relay disconnect"))
			return
		default:
			c.logger.Debug("unhandled control message", "type", msg.Type)
		}
	}
}

// handleGoaway processes a GOAWAY control message from the relay.
func (c *Client) handleGoaway(msg *protocol.ControlMessage) {
	// Parse the payload
	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		c.logger.Warn("failed to marshal goaway payload", "error", err)
		return
	}

	var payload protocol.GoawayPayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		c.logger.Warn("failed to decode goaway payload", "error", err)
		return
	}

	c.logger.Info("received GOAWAY from relay",
		"reason", payload.Reason,
		"drain_timeout_ms", payload.DrainTimeoutMs,
		"message", payload.Message,
	)

	if c.OnGoaway != nil {
		c.OnGoaway(payload)
	}
}

// handleDisconnect handles disconnection.
func (c *Client) handleDisconnect(err error) {
	if c.connected.Swap(false) {
		c.logger.Info("disconnected from relay")
		if c.OnDisconnect != nil {
			c.OnDisconnect(err)
		}
	}
}

// Close disconnects from the relay.
func (c *Client) Close() error {
	if c.closed.Swap(true) {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.controlStream != nil {
		_ = c.controlStream.Close()
	}
	if c.conn != nil {
		_ = c.conn.CloseWithError(0, "client closing")
	}

	c.handleDisconnect(nil)
	return nil
}

// PublicURL returns the public URL for this tunnel.
func (c *Client) PublicURL() string {
	return c.publicURL
}

// SessionID returns the session ID.
func (c *Client) SessionID() string {
	return c.sessionID
}

// Subdomain returns the assigned subdomain.
func (c *Client) Subdomain() string {
	return c.subdomain
}

// RequestCount returns the number of requests processed.
func (c *Client) RequestCount() int64 {
	return c.requestCount.Load()
}

// IsConnected returns true if connected.
func (c *Client) IsConnected() bool {
	return c.connected.Load()
}

// responseWriter implements http.ResponseWriter for QUIC streams.
type responseWriter struct {
	stream        quic.Stream
	header        http.Header
	statusCode    int
	headerWritten bool
}

func (w *responseWriter) Header() http.Header {
	return w.header
}

func (w *responseWriter) WriteHeader(statusCode int) {
	if w.headerWritten {
		return
	}
	w.headerWritten = true
	w.statusCode = statusCode

	// Build response metadata
	headers := make(map[string]string)
	for key, values := range w.header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	meta := &protocol.HTTPMetadata{
		StatusCode: statusCode,
		Header:     headers,
	}

	metaBytes, _ := protocol.EncodeHTTPMetadata(meta)
	header := &protocol.StreamHeader{
		Version:  protocol.ProtocolVersion,
		Type:     protocol.StreamTypeHTTP,
		Metadata: metaBytes,
	}

	_ = protocol.EncodeHeader(w.stream, header)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}
	return w.stream.Write(data)
}

// Flush implements http.Flusher.
func (w *responseWriter) Flush() {
	// QUIC streams auto-flush
}
