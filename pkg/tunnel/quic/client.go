// Package quic provides a QUIC-based tunnel client.
package quic

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"golang.org/x/net/http2"

	"github.com/getmockd/mockd/pkg/tunnel/protocol"
)

// Client is a QUIC tunnel client that connects to the relay server.
type Client struct {
	conn          *quic.Conn
	controlStream *quic.Stream

	// Configuration
	relayAddr   string
	token       string
	localPort   int
	tlsInsecure bool
	tunnelAuth  *protocol.TunnelAuth
	protocols   []protocol.ProtocolPort

	// MQTT broker name → local port mapping (built from protocols)
	mqttPorts map[string]int

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
	RelayAddr   string                  // e.g., "relay.mockd.io:4443"
	Token       string                  // Auth token
	LocalPort   int                     // Local port being tunneled (HTTP/gRPC/WS)
	Handler     http.Handler            // HTTP handler for incoming requests
	TLSInsecure bool                    // Skip TLS verification (for testing)
	TunnelAuth  *protocol.TunnelAuth    // Incoming request auth config (optional)
	Protocols   []protocol.ProtocolPort // Protocol port mappings (e.g., MQTT brokers)
	Logger      *slog.Logger
}

// NewClient creates a new QUIC tunnel client.
func NewClient(cfg *ClientConfig) *Client {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	// Build MQTT broker name → port map from protocol list
	mqttPorts := make(map[string]int)
	for _, p := range cfg.Protocols {
		if p.Type == "mqtt" {
			mqttPorts[p.Name] = p.Port
		}
	}

	return &Client{
		relayAddr:   cfg.RelayAddr,
		token:       cfg.Token,
		localPort:   cfg.LocalPort,
		handler:     cfg.Handler,
		tlsInsecure: cfg.TLSInsecure,
		tunnelAuth:  cfg.TunnelAuth,
		protocols:   cfg.Protocols,
		mqttPorts:   mqttPorts,
		logger:      logger,
	}
}

// Connect establishes a QUIC connection to the relay and authenticates.
func (c *Client) Connect(ctx context.Context) error {
	if c.connected.Load() {
		return errors.New("already connected")
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
		Protocols:  c.protocols,
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
		return errors.New("not connected")
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

// handleStream processes an incoming QUIC stream from the relay.
// Dispatches to the appropriate handler based on stream type and flags.
func (c *Client) handleStream(ctx context.Context, stream *quic.Stream) {
	// Read request header
	header, err := protocol.DecodeHeader(stream)
	if err != nil {
		c.logger.Error("decode header error", "error", err)
		_ = stream.Close()
		return
	}

	// Bidirectional streams (WebSocket, gRPC, MQTT) get their own handler
	if header.Flags&protocol.FlagBidirectional != 0 {
		c.handleBidirectionalStream(ctx, stream, header)
		return
	}

	// Half-duplex HTTP request/response (existing behavior)
	defer func() { _ = stream.Close() }()

	if header.Type != protocol.StreamTypeHTTP {
		c.logger.Warn("unexpected stream type for half-duplex", "type", header.Type)
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

	// Build HTTP request.
	// IMPORTANT: Wrap stream in NopCloser so the reverse proxy's HTTP transport
	// doesn't close the QUIC stream's write side when it finishes reading the body.
	// The stream is bidirectional: relay writes body → agent reads; agent writes
	// response → relay reads. If we let http.Transport.Close() the body, it calls
	// stream.Close() which closes the agent's WRITE side, breaking response writes.
	req, err := http.NewRequestWithContext(ctx, meta.Method, meta.Path, io.NopCloser(stream))
	if err != nil {
		c.logger.Error("build request error", "error", err)
		c.sendErrorResponse(stream, http.StatusBadRequest, "Bad Request")
		return
	}

	req.Host = meta.Host
	for key, values := range meta.Header {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}

	// Set ContentLength from the Content-Length header so the reverse proxy
	// uses Content-Length (not chunked encoding) when forwarding to local service.
	// http.NewRequestWithContext leaves ContentLength at -1 (unknown).
	if cl := req.Header.Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil {
			req.ContentLength = n
		}
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

// handleBidirectionalStream handles streams that need concurrent read/write
// (WebSocket, gRPC, MQTT). Dispatches to protocol-specific handlers.
func (c *Client) handleBidirectionalStream(ctx context.Context, stream *quic.Stream, header *protocol.StreamHeader) {
	// Decode HTTP metadata (WebSocket, gRPC all start with HTTP)
	meta, err := protocol.DecodeHTTPMetadata(header.Metadata)
	if err != nil {
		c.logger.Error("decode bidir http metadata error", "error", err)
		c.sendErrorResponse(stream, http.StatusBadRequest, "Bad Request")
		_ = stream.Close()
		return
	}

	if c.OnRequest != nil {
		c.OnRequest(meta.Method, meta.Path)
	}

	c.logger.Debug("handling bidirectional stream",
		"type", header.Type,
		"method", meta.Method,
		"path", meta.Path,
	)

	switch header.Type {
	case protocol.StreamTypeGRPC:
		c.handleGRPCStream(ctx, stream, header, meta)
	case protocol.StreamTypeMQTT:
		c.handleMQTTStream(ctx, stream, header)
	default:
		// WebSocket and other bidirectional protocols use raw TCP bridge
		c.handleRawBidirectionalStream(ctx, stream, header, meta)
	}
}

// handleRawBidirectionalStream handles bidirectional streams using raw TCP.
// Used for WebSocket (and future MQTT). Dials the local service, sends an
// HTTP/1.1 request (with Upgrade headers), and bridges bytes in both directions.
func (c *Client) handleRawBidirectionalStream(_ context.Context, stream *quic.Stream, header *protocol.StreamHeader, meta *protocol.HTTPMetadata) {
	// Dial the local service
	localAddr := fmt.Sprintf("127.0.0.1:%d", c.localPort)
	localConn, err := net.DialTimeout("tcp", localAddr, 5*time.Second)
	if err != nil {
		c.logger.Error("failed to dial local service", "addr", localAddr, "error", err)
		c.sendErrorResponse(stream, http.StatusBadGateway, "Failed to connect to local service")
		_ = stream.Close()
		return
	}

	// Build and send the raw HTTP/1.1 request to the local service.
	// The local service handles the protocol upgrade (e.g., WebSocket 101).
	rawReq := buildRawHTTPRequest(meta, localAddr)
	if _, err := localConn.Write(rawReq); err != nil {
		c.logger.Error("failed to write request to local service", "error", err)
		c.sendErrorResponse(stream, http.StatusBadGateway, "Failed to send request to local service")
		_ = localConn.Close()
		_ = stream.Close()
		return
	}

	// Read the HTTP response from the local service (status + headers)
	localBuf := bufio.NewReader(localConn)
	localResp, err := http.ReadResponse(localBuf, nil) //nolint:bodyclose // Body flows raw through localBuf/localConn into bridgeRawBidir; closing it would sever the connection.
	if err != nil {
		c.logger.Error("failed to read response from local service", "error", err)
		c.sendErrorResponse(stream, http.StatusBadGateway, "Failed to read response from local service")
		_ = localConn.Close()
		_ = stream.Close()
		return
	}

	// Send response metadata back to relay
	if err := c.sendBidirResponseMeta(stream, header.Type, localResp); err != nil {
		c.logger.Error("failed to send response to relay", "error", err)
		_ = localConn.Close()
		_ = stream.Close()
		return
	}

	// Create a reader that yields any buffered data, then reads from the raw connection
	var localReader io.Reader
	if localBuf.Buffered() > 0 {
		localReader = io.MultiReader(localBuf, localConn)
	} else {
		localReader = localConn
	}

	// Bridge bytes bidirectionally: relay stream ↔ local service
	c.bridgeRawBidir(stream, header.Type, localConn, localReader)
}

// handleGRPCStream handles gRPC bidirectional streams. gRPC requires HTTP/2,
// so we use h2c (HTTP/2 Cleartext) to talk to the local gRPC server instead
// of raw TCP with HTTP/1.1.
//
// Wire format (agent → relay on QUIC stream):
//
//	[StreamHeader: status + headers metadata]
//	[4-byte length][chunk data]...   ← body chunks
//	[4-byte 0x00000000]              ← end-of-body sentinel
//	[StreamHeader: FlagTrailer + trailer metadata]
func (c *Client) handleGRPCStream(ctx context.Context, stream *quic.Stream, _ *protocol.StreamHeader, meta *protocol.HTTPMetadata) {
	localAddr := fmt.Sprintf("127.0.0.1:%d", c.localPort)

	// Create an HTTP/2 transport for h2c (cleartext HTTP/2 to localhost)
	h2Transport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return net.DialTimeout(network, addr, 5*time.Second)
		},
	}

	// Build the HTTP request for the local gRPC service.
	// The body comes from the QUIC stream (relay → agent → local gRPC server).
	// Wrap in io.NopCloser so the h2 transport's body-close doesn't close the
	// QUIC stream's write side — we need it to send the response back to the relay.
	reqURL := fmt.Sprintf("http://%s%s", localAddr, meta.Path)
	req, err := http.NewRequestWithContext(ctx, meta.Method, reqURL, io.NopCloser(stream))
	if err != nil {
		c.logger.Error("failed to build gRPC request", "error", err)
		c.sendErrorResponse(stream, http.StatusInternalServerError, "Failed to build gRPC request")
		_ = stream.Close()
		return
	}

	// Copy headers from metadata
	for key, values := range meta.Header {
		for _, v := range values {
			req.Header.Add(key, v)
		}
	}
	req.Host = meta.Host

	// Send the request to the local gRPC server via h2c
	localResp, err := h2Transport.RoundTrip(req)
	if err != nil {
		c.logger.Error("gRPC request to local service failed", "error", err)
		c.sendErrorResponse(stream, http.StatusBadGateway, "Failed to reach local gRPC service")
		_ = stream.Close()
		return
	}
	defer func() { _ = localResp.Body.Close() }()

	// Send response StreamHeader (status + headers, no trailers yet — those
	// come after the body because Go's h2 transport populates them lazily).
	respMeta := &protocol.HTTPMetadata{
		StatusCode: localResp.StatusCode,
		Header:     map[string][]string(localResp.Header),
	}
	respMetaBytes, err := protocol.EncodeHTTPMetadata(respMeta)
	if err != nil {
		c.logger.Error("failed to encode gRPC response metadata", "error", err)
		c.sendErrorResponse(stream, http.StatusInternalServerError, "Failed to encode gRPC response")
		_ = stream.Close()
		return
	}

	respHeader := &protocol.StreamHeader{
		Version:  protocol.ProtocolVersion,
		Type:     protocol.StreamTypeGRPC,
		Flags:    protocol.FlagBidirectional,
		Metadata: respMetaBytes,
	}
	if err := protocol.EncodeHeader(stream, respHeader); err != nil {
		c.logger.Error("failed to write gRPC response header to relay", "error", err)
		_ = stream.Close()
		return
	}

	// Stream body as length-prefixed chunks so the relay can forward each
	// chunk immediately without buffering the full response.
	buf := make([]byte, 32*1024) // 32KB read buffer
	for {
		n, readErr := localResp.Body.Read(buf)
		if n > 0 {
			if writeErr := protocol.WriteBodyChunk(stream, buf[:n]); writeErr != nil {
				c.logger.Debug("gRPC body chunk write to relay failed", "error", writeErr)
				_ = stream.Close()
				return
			}
		}
		if readErr != nil {
			if readErr != io.EOF {
				c.logger.Warn("gRPC response body read error", "error", readErr)
			}
			break
		}
	}

	// Write end-of-body sentinel (length 0)
	if err := protocol.WriteBodyChunk(stream, nil); err != nil {
		c.logger.Debug("gRPC end-of-body sentinel write failed", "error", err)
		_ = stream.Close()
		return
	}

	// After body EOF, Go's h2 transport populates resp.Trailer.
	// Send a trailer StreamHeader with FlagTrailer set.
	trailerMeta := &protocol.HTTPMetadata{}
	if len(localResp.Trailer) > 0 {
		trailerMeta.Trailer = map[string][]string(localResp.Trailer)
	}
	trailerMetaBytes, err := protocol.EncodeHTTPMetadata(trailerMeta)
	if err != nil {
		c.logger.Warn("failed to encode gRPC trailer metadata", "error", err)
		_ = stream.Close()
		return
	}

	trailerHeader := &protocol.StreamHeader{
		Version:  protocol.ProtocolVersion,
		Type:     protocol.StreamTypeGRPC,
		Flags:    protocol.FlagTrailer,
		Metadata: trailerMetaBytes,
	}
	if err := protocol.EncodeHeader(stream, trailerHeader); err != nil {
		c.logger.Debug("gRPC trailer header write to relay failed", "error", err)
	}

	_ = stream.Close()
	c.requestCount.Add(1)
	c.logger.Debug("gRPC stream completed")
}

// handleMQTTStream handles native MQTT bidirectional streams. The relay sends
// MQTTMetadata with a broker name; we look up the local MQTT port and bridge bytes.
func (c *Client) handleMQTTStream(_ context.Context, stream *quic.Stream, header *protocol.StreamHeader) {
	// Decode MQTT metadata to get broker name
	mqttMeta, err := protocol.DecodeMQTTMetadata(header.Metadata)
	if err != nil {
		c.logger.Error("failed to decode MQTT metadata", "error", err)
		_ = stream.Close()
		return
	}

	// Look up local MQTT port from broker name
	localPort, ok := c.mqttPorts[mqttMeta.BrokerName]
	if !ok {
		// Try default (empty name) if named broker not found
		localPort, ok = c.mqttPorts[""]
		if !ok {
			c.logger.Error("no MQTT port configured for broker",
				"broker", mqttMeta.BrokerName,
				"available", c.mqttPorts,
			)
			_ = stream.Close()
			return
		}
	}

	c.logger.Debug("handling MQTT stream",
		"broker", mqttMeta.BrokerName,
		"local_port", localPort,
	)

	if c.OnRequest != nil {
		broker := mqttMeta.BrokerName
		if broker == "" {
			broker = "default"
		}
		c.OnRequest("MQTT", broker)
	}

	// Dial local MQTT broker
	localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	localConn, err := net.DialTimeout("tcp", localAddr, 5*time.Second)
	if err != nil {
		c.logger.Error("failed to dial local MQTT broker", "addr", localAddr, "error", err)
		_ = stream.Close()
		return
	}

	// Send response header back to relay (confirmation that we connected)
	respHeader := &protocol.StreamHeader{
		Version: protocol.ProtocolVersion,
		Type:    protocol.StreamTypeMQTT,
		Flags:   protocol.FlagBidirectional,
	}
	if err := protocol.EncodeHeader(stream, respHeader); err != nil {
		c.logger.Error("failed to send MQTT response header", "error", err)
		_ = localConn.Close()
		_ = stream.Close()
		return
	}

	// Bridge bytes: QUIC stream ↔ local MQTT broker
	c.bridgeRawBidir(stream, protocol.StreamTypeMQTT, localConn, localConn)
}

// sendBidirResponseMeta sends response metadata (status + headers) back to the
// relay as a protocol StreamHeader. Used by all bidirectional stream handlers.
func (c *Client) sendBidirResponseMeta(stream *quic.Stream, streamType protocol.StreamType, resp *http.Response) error {
	respMeta := &protocol.HTTPMetadata{
		StatusCode: resp.StatusCode,
		Header:     map[string][]string(resp.Header),
	}
	respMetaBytes, err := protocol.EncodeHTTPMetadata(respMeta)
	if err != nil {
		return fmt.Errorf("encode response metadata: %w", err)
	}

	respHeader := &protocol.StreamHeader{
		Version:  protocol.ProtocolVersion,
		Type:     streamType,
		Flags:    protocol.FlagBidirectional,
		Metadata: respMetaBytes,
	}

	if err := protocol.EncodeHeader(stream, respHeader); err != nil {
		return fmt.Errorf("write response header: %w", err)
	}

	return nil
}

// bridgeRawBidir bridges bytes bidirectionally between a QUIC stream and a raw
// TCP connection. Used for WebSocket and MQTT.
func (c *Client) bridgeRawBidir(stream *quic.Stream, streamType protocol.StreamType, localConn net.Conn, localReader io.Reader) {
	var wg sync.WaitGroup
	wg.Add(2)

	// relay → local (data from public client through relay to local service)
	go func() {
		defer wg.Done()
		_, err := io.Copy(localConn, stream)
		if err != nil {
			c.logger.Debug("bidir relay→local ended", "type", streamType, "error", err)
		}
		// Signal to local service that relay side is done writing.
		if tc, ok := localConn.(*net.TCPConn); ok {
			_ = tc.CloseWrite()
		}
	}()

	// local → relay (data from local service through relay to public client)
	go func() {
		defer wg.Done()
		_, err := io.Copy(stream, localReader)
		if err != nil {
			c.logger.Debug("bidir local→relay ended", "type", streamType, "error", err)
		}
		_ = stream.Close()
	}()

	wg.Wait()

	// Ensure everything is cleaned up
	_ = localConn.Close()
	_ = stream.Close()

	c.requestCount.Add(1)
	c.logger.Debug("bidirectional stream completed", "type", streamType)
}

// buildRawHTTPRequest constructs a raw HTTP/1.1 request bytes from metadata.
// Used for bidirectional streams where we dial the local service directly
// instead of going through http.Handler.
func buildRawHTTPRequest(meta *protocol.HTTPMetadata, host string) []byte {
	path := meta.Path
	if path == "" {
		path = "/"
	}

	var buf []byte
	// Request line
	method := meta.Method
	if method == "" {
		method = "GET"
	}
	buf = append(buf, []byte(fmt.Sprintf("%s %s HTTP/1.1\r\n", method, path))...)

	// Host header
	if meta.Host != "" {
		buf = append(buf, []byte(fmt.Sprintf("Host: %s\r\n", meta.Host))...)
	} else {
		buf = append(buf, []byte(fmt.Sprintf("Host: %s\r\n", host))...)
	}

	// Other headers
	for key, values := range meta.Header {
		// Skip Host (already written) and hop-by-hop headers
		lower := http.CanonicalHeaderKey(key)
		if lower == "Host" {
			continue
		}
		for _, v := range values {
			buf = append(buf, []byte(fmt.Sprintf("%s: %s\r\n", key, v))...)
		}
	}

	// End of headers
	buf = append(buf, []byte("\r\n")...)
	return buf
}

// sendErrorResponse sends an error response back through the stream.
func (c *Client) sendErrorResponse(stream *quic.Stream, status int, message string) {
	meta := &protocol.HTTPMetadata{
		StatusCode: status,
		Header: map[string][]string{
			"Content-Type": {"text/plain"},
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
			c.handleDisconnect(errors.New("relay disconnect"))
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
	stream        *quic.Stream
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

	// Build response metadata — pass all header values through (no flattening)
	meta := &protocol.HTTPMetadata{
		StatusCode: statusCode,
		Header:     map[string][]string(w.header),
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
