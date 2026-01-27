package graphql

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
	"github.com/getmockd/mockd/pkg/template"
)

// Pre-compiled regex patterns for performance.
var (
	// subscriptionFieldPattern matches the first field in a subscription query.
	subscriptionFieldPattern = regexp.MustCompile(`subscription\s*(?:\w+)?\s*(?:\([^)]*\))?\s*\{\s*(\w+)`)
	// varsPattern matches {{vars.fieldName}} patterns.
	varsPattern = regexp.MustCompile(`\{\{vars\.([a-zA-Z_][a-zA-Z0-9_]*)\}\}`)
)

// SubscriptionConfig configures a subscription resolver.
type SubscriptionConfig struct {
	// Events is a list of events to stream to the client.
	Events []EventConfig `json:"events,omitempty" yaml:"events,omitempty"`
	// Timing configures the timing behavior for events.
	Timing *TimingConfig `json:"timing,omitempty" yaml:"timing,omitempty"`
}

// EventConfig configures a single subscription event.
type EventConfig struct {
	// Data is the event payload to send.
	Data interface{} `json:"data" yaml:"data"`
	// Delay is the delay before sending this event (e.g., "100ms", "2s").
	Delay string `json:"delay,omitempty" yaml:"delay,omitempty"`
}

// TimingConfig configures timing behavior for subscription events.
type TimingConfig struct {
	// FixedDelay is a fixed delay between events (e.g., "100ms", "1s").
	FixedDelay string `json:"fixedDelay,omitempty" yaml:"fixedDelay,omitempty"`
	// RandomDelay is a random delay range between events (e.g., "100ms-500ms").
	RandomDelay string `json:"randomDelay,omitempty" yaml:"randomDelay,omitempty"`
	// Repeat indicates whether to repeat the events after the sequence completes.
	Repeat bool `json:"repeat,omitempty" yaml:"repeat,omitempty"`
}

// WebSocket message types for graphql-ws protocol (modern) and subscriptions-transport-ws (legacy)
const (
	// Common message types (used by both protocols)
	msgTypeConnectionInit = "connection_init"
	msgTypeConnectionAck  = "connection_ack"

	// graphql-transport-ws protocol (modern)
	msgTypePing      = "ping"
	msgTypePong      = "pong"
	msgTypeSubscribe = "subscribe"
	msgTypeNext      = "next"
	msgTypeError     = "error"
	msgTypeComplete  = "complete"

	// subscriptions-transport-ws protocol (legacy) - additional types
	msgTypeConnectionKeepAlive = "ka"
	msgTypeStart               = "start"
	msgTypeData                = "data"
	msgTypeStop                = "stop"
	msgTypeConnectionTerminate = "connection_terminate"
)

// wsMessage represents a WebSocket message for GraphQL subscriptions.
type wsMessage struct {
	ID      string          `json:"id,omitempty"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// subscribePayload is the payload for subscribe/start messages.
type subscribePayload struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

// SubscriptionHandler handles GraphQL subscriptions over WebSocket.
type SubscriptionHandler struct {
	schema         *Schema
	config         *GraphQLConfig
	upgrader       websocket.AcceptOptions
	conns          map[string]*subscriptionConn
	mu             sync.RWMutex
	connID         atomic.Uint64
	templateEngine *template.Engine
}

// subscriptionConn represents an active WebSocket connection.
type subscriptionConn struct {
	id       string
	conn     *websocket.Conn
	subs     map[string]context.CancelFunc
	protocol string // "graphql-ws" or "graphql-transport-ws"
	mu       sync.Mutex
}

// NewSubscriptionHandler creates a subscription handler.
func NewSubscriptionHandler(schema *Schema, config *GraphQLConfig) *SubscriptionHandler {
	// Determine skipOriginVerify (default: true for dev-friendly behavior)
	skipOriginVerify := true
	if config != nil && config.SkipOriginVerify != nil {
		skipOriginVerify = *config.SkipOriginVerify
	}

	return &SubscriptionHandler{
		schema: schema,
		config: config,
		upgrader: websocket.AcceptOptions{
			Subprotocols:       []string{"graphql-transport-ws", "graphql-ws"},
			InsecureSkipVerify: skipOriginVerify, // Configurable origin verification (default: true for mocking)
		},
		conns:          make(map[string]*subscriptionConn),
		templateEngine: template.New(),
	}
}

// ServeHTTP upgrades HTTP to WebSocket and handles subscriptions.
func (h *SubscriptionHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &h.upgrader)
	if err != nil {
		http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
		return
	}

	h.handleConnection(conn, r)
}

// handleConnection handles a WebSocket connection.
func (h *SubscriptionHandler) handleConnection(conn *websocket.Conn, r *http.Request) {
	// Generate connection ID
	id := fmt.Sprintf("conn-%d", h.connID.Add(1))

	// Determine protocol from negotiated subprotocol
	protocol := conn.Subprotocol()
	if protocol == "" {
		protocol = "graphql-transport-ws" // Default to modern protocol
	}

	sc := &subscriptionConn{
		id:       id,
		conn:     conn,
		subs:     make(map[string]context.CancelFunc),
		protocol: protocol,
	}

	h.mu.Lock()
	h.conns[id] = sc
	h.mu.Unlock()

	defer func() {
		// Cancel all active subscriptions
		sc.mu.Lock()
		for _, cancel := range sc.subs {
			cancel()
		}
		sc.mu.Unlock()

		// Remove from connections map
		h.mu.Lock()
		delete(h.conns, id)
		h.mu.Unlock()

		_ = conn.Close(websocket.StatusNormalClosure, "connection closed")
	}()

	// Handle messages
	ctx := r.Context()
	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			return // Connection closed or error
		}

		if msgType != websocket.MessageText {
			continue // Only handle text messages
		}

		var msg wsMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			h.sendError(sc, "", "invalid message format")
			continue
		}

		h.handleMessage(ctx, sc, &msg)
	}
}

// handleMessage handles a single WebSocket message.
func (h *SubscriptionHandler) handleMessage(ctx context.Context, sc *subscriptionConn, msg *wsMessage) {
	switch msg.Type {
	case msgTypeConnectionInit:
		h.handleConnectionInit(sc, msg)

	case msgTypePing:
		h.handlePing(sc, msg)

	case msgTypeSubscribe, msgTypeStart:
		h.handleSubscribe(ctx, sc, msg.ID, msg.Payload)

	case msgTypeComplete, msgTypeStop:
		h.handleUnsubscribe(sc, msg.ID)

	case msgTypeConnectionTerminate:
		h.handleConnectionTerminate(sc)

	case msgTypePong:
		// Ignore pong messages
	}
}

// handleConnectionInit handles connection_init message.
func (h *SubscriptionHandler) handleConnectionInit(sc *subscriptionConn, _ *wsMessage) {
	// Send connection_ack
	ack := wsMessage{Type: msgTypeConnectionAck}
	_ = h.sendMessage(sc, &ack)

	// For legacy protocol, also send keep-alive
	if sc.protocol == "graphql-ws" {
		ka := wsMessage{Type: msgTypeConnectionKeepAlive}
		_ = h.sendMessage(sc, &ka)
	}
}

// handlePing handles ping message (modern protocol).
func (h *SubscriptionHandler) handlePing(sc *subscriptionConn, msg *wsMessage) {
	pong := wsMessage{
		Type:    msgTypePong,
		Payload: msg.Payload, // Echo back any payload
	}
	_ = h.sendMessage(sc, &pong)
}

// handleSubscribe handles a subscription request.
func (h *SubscriptionHandler) handleSubscribe(ctx context.Context, sc *subscriptionConn, id string, payload json.RawMessage) {
	if id == "" {
		h.sendError(sc, "", "subscription id is required")
		return
	}

	var subPayload subscribePayload
	if err := json.Unmarshal(payload, &subPayload); err != nil {
		h.sendError(sc, id, "invalid subscription payload")
		return
	}

	// Parse the subscription query to extract the field name
	subscriptionName, variables, err := h.parseSubscriptionQuery(subPayload.Query, subPayload.Variables)
	if err != nil {
		h.sendError(sc, id, err.Error())
		return
	}

	// Find the subscription configuration
	subConfig := h.findSubscriptionConfig(subscriptionName)
	if subConfig == nil {
		h.sendError(sc, id, fmt.Sprintf("no subscription configured for %q", subscriptionName))
		return
	}

	// Create cancellable context for this subscription
	subCtx, cancel := context.WithCancel(ctx)

	// Register the subscription
	sc.mu.Lock()
	if _, exists := sc.subs[id]; exists {
		sc.mu.Unlock()
		cancel()
		h.sendError(sc, id, "subscription already exists")
		return
	}
	sc.subs[id] = cancel
	sc.mu.Unlock()

	// Stream events in a goroutine
	go h.streamEvents(subCtx, sc, id, subConfig, variables)
}

// handleUnsubscribe handles an unsubscribe request.
func (h *SubscriptionHandler) handleUnsubscribe(sc *subscriptionConn, id string) {
	sc.mu.Lock()
	cancel, exists := sc.subs[id]
	if exists {
		delete(sc.subs, id)
	}
	sc.mu.Unlock()

	if exists && cancel != nil {
		cancel()
	}
}

// handleConnectionTerminate handles connection_terminate message (legacy protocol).
func (h *SubscriptionHandler) handleConnectionTerminate(sc *subscriptionConn) {
	// Cancel all subscriptions
	sc.mu.Lock()
	for _, cancel := range sc.subs {
		cancel()
	}
	sc.subs = make(map[string]context.CancelFunc)
	sc.mu.Unlock()

	// Close the connection
	_ = sc.conn.Close(websocket.StatusNormalClosure, "connection terminated")
}

// streamEvents streams subscription events to the client.
func (h *SubscriptionHandler) streamEvents(ctx context.Context, sc *subscriptionConn, id string, config *SubscriptionConfig, variables map[string]interface{}) {
	defer func() {
		// Send complete message when done
		h.sendComplete(sc, id)

		// Clean up subscription
		sc.mu.Lock()
		delete(sc.subs, id)
		sc.mu.Unlock()
	}()

	for {
		for _, event := range config.Events {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}

			// Apply event-specific delay first
			if event.Delay != "" {
				if delay, err := time.ParseDuration(event.Delay); err == nil {
					select {
					case <-ctx.Done():
						return
					case <-time.After(delay):
					}
				}
			}

			// Apply timing delay
			delay := h.calculateDelay(config.Timing)
			if delay > 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(delay):
				}
			}

			// Apply variable substitution to event data
			data := h.applyVariables(event.Data, variables)

			// Send the event
			h.sendNext(sc, id, data)
		}

		// If repeat is not enabled, we're done
		if config.Timing == nil || !config.Timing.Repeat {
			break
		}

		// Small sleep before repeating to prevent tight loops
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// calculateDelay calculates the delay based on timing configuration.
func (h *SubscriptionHandler) calculateDelay(timing *TimingConfig) time.Duration {
	if timing == nil {
		return 0
	}

	// Fixed delay takes precedence
	if timing.FixedDelay != "" {
		if d, err := time.ParseDuration(timing.FixedDelay); err == nil {
			return d
		}
	}

	// Random delay range (e.g., "100ms-500ms")
	if timing.RandomDelay != "" {
		return h.parseRandomDelay(timing.RandomDelay)
	}

	return 0
}

// parseRandomDelay parses a random delay range like "100ms-500ms".
func (h *SubscriptionHandler) parseRandomDelay(rangeStr string) time.Duration {
	parts := strings.Split(rangeStr, "-")
	if len(parts) != 2 {
		return 0
	}

	minDelay, err := time.ParseDuration(strings.TrimSpace(parts[0]))
	if err != nil {
		return 0
	}

	maxDelay, err := time.ParseDuration(strings.TrimSpace(parts[1]))
	if err != nil {
		return 0
	}

	if maxDelay <= minDelay {
		return minDelay
	}

	// Generate random delay in range
	rangeMs := maxDelay.Milliseconds() - minDelay.Milliseconds()
	randomMs := rand.Int63n(rangeMs)
	return minDelay + time.Duration(randomMs)*time.Millisecond
}

// parseSubscriptionQuery parses a GraphQL subscription query and returns the subscription field name.
func (h *SubscriptionHandler) parseSubscriptionQuery(query string, inputVars map[string]interface{}) (string, map[string]interface{}, error) {
	// Use pre-compiled regex to extract subscription field name
	// Matches patterns like: subscription { fieldName } or subscription Name { fieldName }
	matches := subscriptionFieldPattern.FindStringSubmatch(query)
	if len(matches) < 2 {
		return "", nil, fmt.Errorf("could not parse subscription query")
	}

	fieldName := matches[1]

	// Extract variables from query arguments if present
	vars := make(map[string]interface{})
	for k, v := range inputVars {
		vars[k] = v
	}

	// Try to extract inline arguments like subscription { messages(channel: "test") }
	// This pattern is dynamic based on field name, so we compile it here
	argRe := regexp.MustCompile(fieldName + `\s*\(\s*([^)]+)\s*\)`)
	argMatches := argRe.FindStringSubmatch(query)
	if len(argMatches) >= 2 {
		// Parse simple key: value or key: "value" patterns
		argStr := argMatches[1]
		argPairs := strings.Split(argStr, ",")
		for _, pair := range argPairs {
			parts := strings.SplitN(pair, ":", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				// Remove quotes if present
				value = strings.Trim(value, `"'`)
				// Check if it's a variable reference
				if strings.HasPrefix(value, "$") {
					varName := strings.TrimPrefix(value, "$")
					if v, ok := inputVars[varName]; ok {
						vars[key] = v
					}
				} else {
					// Try to parse as number or boolean
					vars[key] = parseValue(value)
				}
			}
		}
	}

	return fieldName, vars, nil
}

// parseValue attempts to parse a string value into appropriate Go type.
func parseValue(s string) interface{} {
	// Try boolean
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	// Try integer
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	// Try float
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	// Return as string
	return s
}

// findSubscriptionConfig finds the subscription configuration for a field.
func (h *SubscriptionHandler) findSubscriptionConfig(fieldName string) *SubscriptionConfig {
	if h.config == nil || h.config.Subscriptions == nil {
		return nil
	}

	// Try exact field name
	if config, ok := h.config.Subscriptions[fieldName]; ok {
		return &config
	}

	// Try with Subscription prefix
	if config, ok := h.config.Subscriptions["Subscription."+fieldName]; ok {
		return &config
	}

	return nil
}

// applyVariables substitutes template variables in data.
// It handles {{args.fieldName}}, {{vars.fieldName}}, and general template
// variables like {{uuid}}, {{now}}, etc.
func (h *SubscriptionHandler) applyVariables(data interface{}, vars map[string]interface{}) interface{} {
	if data == nil {
		return nil
	}

	switch v := data.(type) {
	case string:
		// Replace {{args.fieldName}} patterns
		result := templatePattern.ReplaceAllStringFunc(v, func(match string) string {
			parts := templatePattern.FindStringSubmatch(match)
			if len(parts) < 2 {
				return match
			}
			fieldName := parts[1]
			if val, ok := vars[fieldName]; ok {
				return fmt.Sprintf("%v", val)
			}
			return match
		})

		// Replace {{vars.X}} patterns
		result = varsPattern.ReplaceAllStringFunc(result, func(match string) string {
			parts := varsPattern.FindStringSubmatch(match)
			if len(parts) < 2 {
				return match
			}
			if val, ok := vars[parts[1]]; ok {
				return fmt.Sprintf("%v", val)
			}
			return match
		})

		// Process general template variables ({{uuid}}, {{now}}, etc.)
		ctx := template.NewContextFromMap(vars, nil)
		processed, _ := h.templateEngine.Process(result, ctx)
		return processed

	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, val := range v {
			result[key] = h.applyVariables(val, vars)
		}
		return result

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = h.applyVariables(val, vars)
		}
		return result

	default:
		return data
	}
}

// sendMessage sends a WebSocket message.
func (h *SubscriptionHandler) sendMessage(sc *subscriptionConn, msg *wsMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return sc.conn.Write(ctx, websocket.MessageText, data)
}

// sendNext sends a next/data message.
func (h *SubscriptionHandler) sendNext(sc *subscriptionConn, id string, data interface{}) {
	var msgType string
	var payload interface{}

	if sc.protocol == "graphql-ws" {
		// Legacy protocol uses "data" type with nested data field
		msgType = msgTypeData
		payload = map[string]interface{}{
			"data": data,
		}
	} else {
		// Modern protocol uses "next" type
		msgType = msgTypeNext
		payload = map[string]interface{}{
			"data": data,
		}
	}

	payloadBytes, _ := json.Marshal(payload)
	msg := wsMessage{
		ID:      id,
		Type:    msgType,
		Payload: payloadBytes,
	}
	_ = h.sendMessage(sc, &msg)
}

// sendError sends an error message.
func (h *SubscriptionHandler) sendError(sc *subscriptionConn, id string, message string) {
	// Both protocols use "error" type for error messages
	msgType := msgTypeError

	payload := []GraphQLError{{Message: message}}
	payloadBytes, _ := json.Marshal(payload)

	msg := wsMessage{
		ID:      id,
		Type:    msgType,
		Payload: payloadBytes,
	}
	_ = h.sendMessage(sc, &msg)
}

// sendComplete sends a complete message.
func (h *SubscriptionHandler) sendComplete(sc *subscriptionConn, id string) {
	msg := wsMessage{
		ID:   id,
		Type: msgTypeComplete,
	}
	_ = h.sendMessage(sc, &msg)
}

// ConnectionCount returns the number of active connections.
func (h *SubscriptionHandler) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// SubscriptionCount returns the total number of active subscriptions across all connections.
func (h *SubscriptionHandler) SubscriptionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for _, sc := range h.conns {
		sc.mu.Lock()
		count += len(sc.subs)
		sc.mu.Unlock()
	}
	return count
}

// CloseAll closes all active connections.
func (h *SubscriptionHandler) CloseAll(reason string) {
	h.mu.Lock()
	conns := make([]*subscriptionConn, 0, len(h.conns))
	for _, sc := range h.conns {
		conns = append(conns, sc)
	}
	h.mu.Unlock()

	for _, sc := range conns {
		sc.mu.Lock()
		for _, cancel := range sc.subs {
			cancel()
		}
		sc.mu.Unlock()
		_ = sc.conn.Close(websocket.StatusGoingAway, reason)
	}
}
