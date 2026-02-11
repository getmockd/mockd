package tunnel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/getmockd/mockd/pkg/logging"
)

// Client is a tunnel client that connects to the relay server.
type Client struct {
	cfg       *Config
	conn      *websocket.Conn
	handler   RequestHandler
	publicURL string
	sessionID string
	subdomain string
	log       *slog.Logger

	// Metrics
	requestsServed    atomic.Int64
	bytesIn           atomic.Int64
	bytesOut          atomic.Int64
	totalLatencyNanos atomic.Int64
	minLatencyNanos   atomic.Int64
	maxLatencyNanos   atomic.Int64
	connectedAt       time.Time

	// State
	connected        atomic.Bool
	reconnects       atomic.Int32
	disconnectCalled atomic.Bool
	mu               sync.RWMutex
	done             chan struct{}
	closeOnce        sync.Once
}

// RequestHandler handles incoming requests from the tunnel.
type RequestHandler interface {
	// HandleRequest processes an incoming request and returns a response.
	HandleRequest(ctx context.Context, req *TunnelMessage) *TunnelMessage
}

// NewClient creates a new tunnel client.
func NewClient(cfg *Config, handler RequestHandler) (*Client, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if handler == nil {
		return nil, errors.New("handler is required")
	}

	return &Client{
		cfg:     cfg,
		handler: handler,
		done:    make(chan struct{}),
		log:     logging.Nop(),
	}, nil
}

// Connect establishes a connection to the relay server.
func (c *Client) Connect(ctx context.Context) error {
	if c.connected.Load() {
		return errors.New("already connected")
	}

	// Build headers
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+c.cfg.Token)
	if c.cfg.Subdomain != "" {
		headers.Set("X-Tunnel-ID", c.cfg.Subdomain)
	}
	if c.cfg.CustomDomain != "" {
		headers.Set("X-Custom-Domain", c.cfg.CustomDomain)
	}
	headers.Set("X-Client-Version", c.cfg.ClientVersion)

	// Connect to relay
	conn, resp, err := websocket.Dial(ctx, c.cfg.RelayURL, &websocket.DialOptions{
		HTTPHeader: headers,
	})
	if resp != nil && resp.Body != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return fmt.Errorf("failed to connect to relay: %w", err)
	}

	// Read connected message
	_, data, err := conn.Read(ctx)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "failed to read connected message")
		return fmt.Errorf("failed to read connected message: %w", err)
	}

	connMsg, err := DecodeConnectedMessage(data)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "invalid connected message")
		return fmt.Errorf("invalid connected message: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.publicURL = connMsg.PublicURL
	c.sessionID = connMsg.SessionID
	c.subdomain = connMsg.Subdomain
	c.connectedAt = time.Now()
	c.mu.Unlock()

	c.connected.Store(true)

	// Call connect callback
	if c.cfg.OnConnect != nil {
		c.cfg.OnConnect(c.publicURL)
	}

	// Start message pump
	go c.readPump(ctx)

	return nil
}

// Disconnect closes the connection to the relay server.
func (c *Client) Disconnect() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.connected.Store(false)

		c.mu.Lock()
		if c.conn != nil {
			_ = c.conn.Close(websocket.StatusNormalClosure, "client disconnect")
			c.conn = nil
		}
		c.mu.Unlock()

		if c.cfg.OnDisconnect != nil && c.disconnectCalled.CompareAndSwap(false, true) {
			c.cfg.OnDisconnect(nil)
		}
	})
}

// IsConnected returns true if the client is connected.
func (c *Client) IsConnected() bool {
	return c.connected.Load()
}

// PublicURL returns the public URL for this tunnel.
func (c *Client) PublicURL() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.publicURL
}

// SessionID returns the session ID.
func (c *Client) SessionID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.sessionID
}

// Subdomain returns the assigned subdomain.
func (c *Client) Subdomain() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.subdomain
}

// Stats returns tunnel statistics.
func (c *Client) Stats() *TunnelStats {
	c.mu.RLock()
	connectedAt := c.connectedAt
	c.mu.RUnlock()

	reqs := c.requestsServed.Load()
	totalLatency := c.totalLatencyNanos.Load()

	var avgLatency time.Duration
	if reqs > 0 {
		avgLatency = time.Duration(totalLatency / reqs)
	}

	return &TunnelStats{
		RequestsServed: reqs,
		BytesIn:        c.bytesIn.Load(),
		BytesOut:       c.bytesOut.Load(),
		TotalLatency:   time.Duration(totalLatency),
		AvgLatency:     avgLatency,
		MinLatency:     time.Duration(c.minLatencyNanos.Load()),
		MaxLatency:     time.Duration(c.maxLatencyNanos.Load()),
		ConnectedAt:    connectedAt,
		Reconnects:     int(c.reconnects.Load()),
		IsConnected:    c.connected.Load(),
	}
}

// TunnelStats holds tunnel statistics.
type TunnelStats struct {
	RequestsServed int64
	BytesIn        int64
	BytesOut       int64
	TotalLatency   time.Duration
	AvgLatency     time.Duration
	MinLatency     time.Duration
	MaxLatency     time.Duration
	ConnectedAt    time.Time
	Reconnects     int
	IsConnected    bool
}

// Uptime returns the duration since connection.
func (s *TunnelStats) Uptime() time.Duration {
	if s.ConnectedAt.IsZero() {
		return 0
	}
	return time.Since(s.ConnectedAt)
}

// AvgLatencyMs returns average latency in milliseconds.
func (s *TunnelStats) AvgLatencyMs() float64 {
	return float64(s.AvgLatency.Nanoseconds()) / 1e6
}

// MinLatencyMs returns minimum latency in milliseconds.
func (s *TunnelStats) MinLatencyMs() float64 {
	return float64(s.MinLatency.Nanoseconds()) / 1e6
}

// MaxLatencyMs returns maximum latency in milliseconds.
func (s *TunnelStats) MaxLatencyMs() float64 {
	return float64(s.MaxLatency.Nanoseconds()) / 1e6
}

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.connected.Store(false)
		if c.cfg.OnDisconnect != nil && c.disconnectCalled.CompareAndSwap(false, true) {
			c.cfg.OnDisconnect(errors.New("connection closed"))
		}

		// Auto-reconnect if enabled
		if c.cfg.AutoReconnect {
			go c.reconnectLoop(ctx)
		}
	}()

	for {
		select {
		case <-c.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}

		c.bytesIn.Add(int64(len(data)))

		msg, err := DecodeMessage(data)
		if err != nil {
			continue
		}

		go c.handleMessage(ctx, msg)
	}
}

// handleMessage processes an incoming message.
func (c *Client) handleMessage(ctx context.Context, msg *TunnelMessage) {
	switch msg.Type {
	case MessageTypeRequest:
		c.handleRequest(ctx, msg)
	case MessageTypePing:
		c.sendPong(ctx, msg.ID)
	case MessageTypeError:
		// Log error but continue
		c.log.Error("tunnel error", "error", msg.Error)
	case MessageTypeDisconnect:
		c.Disconnect()
	}
}

// handleRequest handles an incoming HTTP request.
func (c *Client) handleRequest(ctx context.Context, req *TunnelMessage) {
	startTime := time.Now()

	// Call request callback
	if c.cfg.OnRequest != nil {
		c.cfg.OnRequest(req.Method, req.Path)
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, c.cfg.RequestTimeout)
	defer cancel()

	// Forward to handler
	resp := c.handler.HandleRequest(ctx, req)

	// Send response
	if err := c.sendMessage(ctx, resp); err != nil {
		// Log error but don't crash
		c.log.Error("failed to send response", "error", err)
	}

	c.requestsServed.Add(1)

	// Record latency
	c.recordLatency(time.Since(startTime))
}

// recordLatency records a request latency measurement.
func (c *Client) recordLatency(d time.Duration) {
	nanos := d.Nanoseconds()
	c.totalLatencyNanos.Add(nanos)

	// Update min latency (using CAS loop)
	for {
		current := c.minLatencyNanos.Load()
		if current != 0 && current <= nanos {
			break
		}
		if c.minLatencyNanos.CompareAndSwap(current, nanos) {
			break
		}
	}

	// Update max latency (using CAS loop)
	for {
		current := c.maxLatencyNanos.Load()
		if current >= nanos {
			break
		}
		if c.maxLatencyNanos.CompareAndSwap(current, nanos) {
			break
		}
	}
}

// sendPong sends a pong message.
func (c *Client) sendPong(ctx context.Context, pingID string) {
	msg := NewPongMessage(pingID)
	_ = c.sendMessage(ctx, msg)
}

// sendMessage sends a message to the relay.
func (c *Client) sendMessage(ctx context.Context, msg *TunnelMessage) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return errors.New("not connected")
	}

	data, err := msg.Encode()
	if err != nil {
		return err
	}

	c.bytesOut.Add(int64(len(data)))

	return conn.Write(ctx, websocket.MessageText, data)
}

// reconnectLoop attempts to reconnect after disconnect.
func (c *Client) reconnectLoop(ctx context.Context) {
	delay := c.cfg.ReconnectDelay

	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}

		// Check and reset state atomically
		c.mu.Lock()
		select {
		case <-c.done:
			// Already disconnected permanently
			c.mu.Unlock()
			return
		default:
		}
		c.done = make(chan struct{})
		c.closeOnce = sync.Once{}
		c.disconnectCalled.Store(false) // Reset for new connection
		c.mu.Unlock()

		if err := c.Connect(ctx); err != nil {
			// Exponential backoff
			delay *= 2
			if delay > c.cfg.MaxReconnectDelay {
				delay = c.cfg.MaxReconnectDelay
			}
			c.reconnects.Add(1)
			continue
		}

		// Successfully reconnected
		return
	}
}

// Run connects and blocks until disconnect or context cancellation.
func (c *Client) Run(ctx context.Context) error {
	if err := c.Connect(ctx); err != nil {
		return err
	}

	select {
	case <-c.done:
		return nil
	case <-ctx.Done():
		c.Disconnect()
		return ctx.Err()
	}
}

// SetLogger sets the operational logger for the client.
func (c *Client) SetLogger(log *slog.Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if log != nil {
		c.log = log
	} else {
		c.log = logging.Nop()
	}
}
