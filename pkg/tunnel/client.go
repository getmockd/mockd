package tunnel

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

// Client is a tunnel client that connects to the relay server.
type Client struct {
	cfg       *Config
	conn      *websocket.Conn
	handler   RequestHandler
	publicURL string
	sessionID string
	subdomain string

	// Metrics
	requestsServed atomic.Int64
	bytesIn        atomic.Int64
	bytesOut       atomic.Int64
	connectedAt    time.Time

	// State
	connected  atomic.Bool
	reconnects atomic.Int32
	mu         sync.RWMutex
	done       chan struct{}
	closeOnce  sync.Once
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
		return nil, fmt.Errorf("handler is required")
	}

	return &Client{
		cfg:     cfg,
		handler: handler,
		done:    make(chan struct{}),
	}, nil
}

// Connect establishes a connection to the relay server.
func (c *Client) Connect(ctx context.Context) error {
	if c.connected.Load() {
		return fmt.Errorf("already connected")
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
	conn, _, err := websocket.Dial(ctx, c.cfg.RelayURL, &websocket.DialOptions{
		HTTPHeader: headers,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to relay: %w", err)
	}

	// Read connected message
	_, data, err := conn.Read(ctx)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "failed to read connected message")
		return fmt.Errorf("failed to read connected message: %w", err)
	}

	connMsg, err := DecodeConnectedMessage(data)
	if err != nil {
		conn.Close(websocket.StatusInternalError, "invalid connected message")
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
			c.conn.Close(websocket.StatusNormalClosure, "client disconnect")
			c.conn = nil
		}
		c.mu.Unlock()

		if c.cfg.OnDisconnect != nil {
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

	return &TunnelStats{
		RequestsServed: c.requestsServed.Load(),
		BytesIn:        c.bytesIn.Load(),
		BytesOut:       c.bytesOut.Load(),
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

// readPump reads messages from the WebSocket connection.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.connected.Store(false)
		if c.cfg.OnDisconnect != nil {
			c.cfg.OnDisconnect(fmt.Errorf("connection closed"))
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
		fmt.Printf("tunnel error: %s\n", msg.Error)
	case MessageTypeDisconnect:
		c.Disconnect()
	}
}

// handleRequest handles an incoming HTTP request.
func (c *Client) handleRequest(ctx context.Context, req *TunnelMessage) {
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
		fmt.Printf("failed to send response: %v\n", err)
	}

	c.requestsServed.Add(1)
}

// sendPong sends a pong message.
func (c *Client) sendPong(ctx context.Context, pingID string) {
	msg := NewPongMessage(pingID)
	c.sendMessage(ctx, msg)
}

// sendMessage sends a message to the relay.
func (c *Client) sendMessage(ctx context.Context, msg *TunnelMessage) error {
	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		return fmt.Errorf("not connected")
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
		case <-c.done:
			return
		case <-ctx.Done():
			return
		default:
		}

		time.Sleep(delay)

		c.mu.Lock()
		c.done = make(chan struct{})
		c.closeOnce = sync.Once{}
		c.mu.Unlock()

		if err := c.Connect(ctx); err != nil {
			// Exponential backoff
			delay = delay * 2
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
