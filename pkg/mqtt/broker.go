package mqtt

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getmockd/mockd/internal/id"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/protocol"
	"github.com/getmockd/mockd/pkg/requestlog"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
)

// Interface compliance checks.
var (
	_ protocol.Handler          = (*Broker)(nil)
	_ protocol.StandaloneServer = (*Broker)(nil)
	_ protocol.Recordable       = (*Broker)(nil)
	_ protocol.RequestLoggable  = (*Broker)(nil)
	_ protocol.Loggable         = (*Broker)(nil)
	_ protocol.Observable       = (*Broker)(nil)
)

// SubscriptionHandler is a callback for received messages
type SubscriptionHandler func(topic string, payload []byte)

// MQTTRecordingData represents a recorded MQTT message
type MQTTRecordingData struct {
	Topic     string `json:"topic"`
	Payload   []byte `json:"payload"`
	QoS       byte   `json:"qos"`
	Retain    bool   `json:"retain"`
	ClientID  string `json:"clientId"`
	Direction string `json:"direction"` // "publish", "subscribe"
}

// MQTTRecordingStore is an interface for storing recorded MQTT messages
type MQTTRecordingStore interface {
	Add(MQTTRecordingData) error
}

// Broker is a mock MQTT broker
type Broker struct {
	config                     *MQTTConfig
	server                     *mqtt.Server
	mu                         sync.RWMutex
	running                    bool
	startedAt                  time.Time
	log                        *slog.Logger
	internalSubscribers        map[string][]SubscriptionHandler
	clientSubscriptions        map[string][]string
	simulator                  *Simulator
	recordingEnabled           bool
	recordingStore             MQTTRecordingStore
	requestLogger              requestlog.Logger
	responseHandler            *ResponseHandler
	conditionalResponseHandler *ConditionalResponseHandler
	sessionManager             *SessionManager
	// mockResponseTopics tracks topics currently being published as mock responses
	// to prevent infinite loops when a response triggers the same or related patterns.
	mockResponseTopics   map[string]struct{}
	mockResponseTopicsMu sync.Mutex
	// stopping is set to 1 during shutdown to prevent hook callbacks from
	// acquiring the broker mutex, which would deadlock with server.Close().
	stopping atomic.Int32
}

// markMockResponseTopic marks a topic as currently being published by a mock response.
// Returns true if the topic was successfully marked (not already active).
// Returns false if the topic is already active (loop detected).
func (b *Broker) markMockResponseTopic(topic string) bool {
	b.mockResponseTopicsMu.Lock()
	defer b.mockResponseTopicsMu.Unlock()
	if b.mockResponseTopics == nil {
		b.mockResponseTopics = make(map[string]struct{})
	}
	if _, exists := b.mockResponseTopics[topic]; exists {
		return false // loop detected
	}
	b.mockResponseTopics[topic] = struct{}{}
	return true
}

// unmarkMockResponseTopic removes the active mock response marker for a topic.
func (b *Broker) unmarkMockResponseTopic(topic string) {
	b.mockResponseTopicsMu.Lock()
	defer b.mockResponseTopicsMu.Unlock()
	delete(b.mockResponseTopics, topic)
}

// isMockResponseActive checks if a topic is currently being published by a mock response.
func (b *Broker) isMockResponseActive(topic string) bool {
	b.mockResponseTopicsMu.Lock()
	defer b.mockResponseTopicsMu.Unlock()
	if b.mockResponseTopics == nil {
		return false
	}
	_, exists := b.mockResponseTopics[topic]
	return exists
}

// NewBroker creates a new MQTT broker
func NewBroker(config *MQTTConfig) (*Broker, error) {
	if config == nil {
		return nil, errors.New("config cannot be nil")
	}

	if config.Port <= 0 {
		config.Port = 1883 // Default MQTT port
	}

	server := mqtt.New(&mqtt.Options{
		InlineClient: true,
	})

	broker := &Broker{
		config:              config,
		server:              server,
		log:                 logging.Nop(),
		internalSubscribers: make(map[string][]SubscriptionHandler),
		clientSubscriptions: make(map[string][]string),
		sessionManager:      NewSessionManager(),
	}
	broker.responseHandler = NewResponseHandler(broker)
	broker.conditionalResponseHandler = NewConditionalResponseHandler(broker)

	// Add auth hook - use AllowHook if authentication is disabled, custom hook otherwise
	if config.Auth != nil && config.Auth.Enabled {
		authHook := NewAuthHook(config.Auth)
		if err := server.AddHook(authHook, nil); err != nil {
			return nil, fmt.Errorf("failed to add auth hook: %w", err)
		}
	} else {
		// mochi-mqtt requires an auth hook - use AllowHook to allow all connections
		if err := server.AddHook(new(auth.AllowHook), nil); err != nil {
			return nil, fmt.Errorf("failed to add allow hook: %w", err)
		}
	}

	// Add message hook for handling publish events
	messageHook := NewMessageHook(broker)
	if err := server.AddHook(messageHook, nil); err != nil {
		return nil, fmt.Errorf("failed to add message hook: %w", err)
	}

	return broker, nil
}

// Start starts the MQTT broker.
// The context can be used for cancellation during startup.
func (b *Broker) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return errors.New("broker is already running")
	}

	// Check for cancellation before proceeding
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Create listener based on TLS configuration
	var listener listeners.Listener
	var err error

	listenerID := fmt.Sprintf("mqtt-%d", b.config.Port)
	addr := fmt.Sprintf(":%d", b.config.Port)

	if b.config.TLS != nil && b.config.TLS.Enabled {
		// Load TLS certificates
		cert, err := tls.LoadX509KeyPair(b.config.TLS.CertFile, b.config.TLS.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to load TLS certificates: %w", err)
		}

		tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}

		listener = listeners.NewTCP(listeners.Config{
			ID:        listenerID,
			Address:   addr,
			TLSConfig: tlsConfig,
		})
	} else {
		listener = listeners.NewTCP(listeners.Config{
			ID:      listenerID,
			Address: addr,
		})
	}

	if err = b.server.AddListener(listener); err != nil {
		return fmt.Errorf("failed to add listener: %w", err)
	}

	// Start the server
	go func() {
		if err := b.server.Serve(); err != nil {
			b.log.Error("MQTT server error", "error", err)
		}
	}()

	b.running = true
	b.startedAt = time.Now()

	// Start simulator if topics are configured
	if len(b.config.Topics) > 0 {
		b.simulator = NewSimulator(b, b.config.Topics, nil)
		b.simulator.Start()
	}

	return nil
}

// Stop gracefully shuts down the MQTT broker.
// The timeout parameter specifies the maximum time to wait for graceful shutdown.
// If the timeout expires, the handler will force shutdown.
func (b *Broker) Stop(ctx context.Context, timeout time.Duration) error {
	b.mu.Lock()

	if !b.running {
		b.mu.Unlock()
		return nil
	}

	// Stop simulator first
	if b.simulator != nil {
		b.simulator.Stop()
		b.simulator = nil
	}

	// Signal that we're stopping so hook callbacks (OnUnsubscribed, OnSubscribed)
	// skip acquiring b.mu, which would deadlock with server.Close().
	b.stopping.Store(1)
	b.mu.Unlock()

	// Create a timeout context for graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Close the server — this triggers client disconnections which call hooks.
	// The mutex is NOT held here to avoid deadlock with hook callbacks.
	done := make(chan error, 1)
	go func() {
		done <- b.server.Close()
	}()

	var closeErr error
	select {
	case err := <-done:
		closeErr = err
	case <-shutdownCtx.Done():
		closeErr = fmt.Errorf("shutdown timed out: %w", shutdownCtx.Err())
	}

	b.mu.Lock()
	b.running = false
	b.startedAt = time.Time{}
	b.mu.Unlock()

	if closeErr != nil {
		return fmt.Errorf("failed to close server: %w", closeErr)
	}
	return nil
}

// IsRunning returns true if broker is running
func (b *Broker) IsRunning() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.running
}

// Publish publishes a message to a topic
func (b *Broker) Publish(topic string, payload []byte, qos byte, retain bool) error {
	b.mu.RLock()
	running := b.running
	b.mu.RUnlock()

	if !running {
		return errors.New("broker is not running")
	}

	return b.server.Publish(topic, payload, retain, qos)
}

// Subscribe registers an internal subscription for testing
func (b *Broker) Subscribe(topic string, handler SubscriptionHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.internalSubscribers[topic] = append(b.internalSubscribers[topic], handler)
}

// Unsubscribe removes an internal subscription
func (b *Broker) Unsubscribe(topic string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.internalSubscribers, topic)
}

// notifySubscribers notifies all internal subscribers of a published message
func (b *Broker) notifySubscribers(topic string, payload []byte) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for pattern, handlers := range b.internalSubscribers {
		if matchTopic(pattern, topic) {
			for _, handler := range handlers {
				go handler(topic, payload)
			}
		}
	}
}

// GetClients returns connected client IDs
func (b *Broker) GetClients() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getClientsLocked()
}

// getClientsLocked returns connected client IDs.
// The caller must hold b.mu (read or write).
func (b *Broker) getClientsLocked() []string {
	if b.server == nil {
		return nil
	}

	clients := b.server.Clients.GetAll()
	ids := make([]string, 0, len(clients))
	for id := range clients {
		ids = append(ids, id)
	}
	return ids
}

// GetSubscriptions returns active subscriptions by client ID
func (b *Broker) GetSubscriptions() map[string][]string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Create a copy to avoid race conditions
	result := make(map[string][]string)
	for clientID, subs := range b.clientSubscriptions {
		result[clientID] = append([]string{}, subs...)
	}
	return result
}

// Config returns the broker configuration
func (b *Broker) Config() *MQTTConfig {
	return b.config
}

// ID returns the broker's configuration ID
func (b *Broker) ID() string {
	if b.config == nil {
		return ""
	}
	return b.config.ID
}

// EnableRecording enables message recording
func (b *Broker) EnableRecording() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.recordingEnabled = true
}

// DisableRecording disables message recording
func (b *Broker) DisableRecording() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.recordingEnabled = false
}

// IsRecordingEnabled returns true if recording is enabled
func (b *Broker) IsRecordingEnabled() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.recordingEnabled
}

// SetRecordingStore sets the recording store for captured messages
func (b *Broker) SetRecordingStore(store MQTTRecordingStore) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.recordingStore = store
}

// GetRecordingStore returns the current recording store
func (b *Broker) GetRecordingStore() MQTTRecordingStore {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.recordingStore
}

// SetRequestLogger sets the request logger for unified logging
func (b *Broker) SetRequestLogger(logger requestlog.Logger) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.requestLogger = logger
}

// GetRequestLogger returns the current request logger
func (b *Broker) GetRequestLogger() requestlog.Logger {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.requestLogger
}

// SetLogger sets the operational logger for the broker.
func (b *Broker) SetLogger(log *slog.Logger) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if log != nil {
		b.log = log
	} else {
		b.log = logging.Nop()
	}
}

// GetStats returns broker statistics
func (b *Broker) GetStats() BrokerStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	stats := BrokerStats{
		Running:     b.running,
		ClientCount: len(b.getClientsLocked()),
		TopicCount:  len(b.config.Topics),
		Port:        b.config.Port,
		TLSEnabled:  b.config.TLS != nil && b.config.TLS.Enabled,
		AuthEnabled: b.config.Auth != nil && b.config.Auth.Enabled,
	}

	return stats
}

// BrokerStats contains broker statistics
type BrokerStats struct {
	Running     bool `json:"running"`
	ClientCount int  `json:"clientCount"`
	TopicCount  int  `json:"topicCount"`
	Port        int  `json:"port"`
	TLSEnabled  bool `json:"tlsEnabled"`
	AuthEnabled bool `json:"authEnabled"`
}

// Metadata returns descriptive information about the MQTT broker.
// This includes the unique ID, protocol type, version, and capabilities.
func (b *Broker) Metadata() protocol.Metadata {
	return protocol.Metadata{
		ID:                   b.ID(),
		Name:                 b.config.Name,
		Protocol:             protocol.ProtocolMQTT,
		Version:              "0.2.4",
		TransportType:        protocol.TransportTCP,
		ConnectionModel:      protocol.ConnectionModelStandalone,
		CommunicationPattern: protocol.PatternPubSub,
		Capabilities: []protocol.Capability{
			protocol.CapabilityConnections,
			protocol.CapabilityPubSub,
			protocol.CapabilityRecording,
			protocol.CapabilitySessions,
			protocol.CapabilityMetrics,
		},
	}
}

// Health returns the current health status of the MQTT broker.
func (b *Broker) Health(ctx context.Context) protocol.HealthStatus {
	b.mu.RLock()
	running := b.running
	b.mu.RUnlock()

	status := protocol.HealthUnhealthy
	if running {
		status = protocol.HealthHealthy
	}
	return protocol.HealthStatus{
		Status:    status,
		CheckedAt: time.Now(),
	}
}

// Port returns the port the broker is listening on.
// Returns 0 if the broker is not running.
func (b *Broker) Port() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.config == nil {
		return 0
	}
	return b.config.Port
}

// Address returns the full address the broker is listening on.
// Returns empty string if the broker is not running.
func (b *Broker) Address() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if !b.running || b.config == nil {
		return ""
	}
	return fmt.Sprintf(":%d", b.config.Port)
}

// Stats returns operational metrics for the broker.
func (b *Broker) Stats() protocol.Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var uptime time.Duration
	if b.running && !b.startedAt.IsZero() {
		uptime = time.Since(b.startedAt)
	}

	custom := map[string]any{
		"clientCount": len(b.server.Clients.GetAll()),
		"topicCount":  len(b.config.Topics),
		"port":        b.config.Port,
		"tlsEnabled":  b.config.TLS != nil && b.config.TLS.Enabled,
		"authEnabled": b.config.Auth != nil && b.config.Auth.Enabled,
	}

	return protocol.Stats{
		Running:   b.running,
		StartedAt: b.startedAt,
		Uptime:    uptime,
		Custom:    custom,
	}
}

// GetResponseHandler returns the mock response handler
func (b *Broker) GetResponseHandler() *ResponseHandler {
	return b.responseHandler
}

// GetSessionManager returns the test panel session manager
func (b *Broker) GetSessionManager() *SessionManager {
	return b.sessionManager
}

// SetMockResponses updates the broker's mock responses
func (b *Broker) SetMockResponses(responses []*MockResponse) {
	if b.responseHandler != nil {
		b.responseHandler.SetResponses(responses)
	}
}

// GetConditionalResponseHandler returns the conditional response handler
func (b *Broker) GetConditionalResponseHandler() *ConditionalResponseHandler {
	return b.conditionalResponseHandler
}

// SetConditionalResponses updates the broker's conditional responses
func (b *Broker) SetConditionalResponses(responses []*ConditionalResponse) {
	if b.conditionalResponseHandler != nil {
		b.conditionalResponseHandler.SetConditionalResponses(responses)
	}
}

// notifyMockResponse notifies sessions about a mock response being sent
func (b *Broker) notifyMockResponse(topic string, payload []byte, responseID string) {
	if b.sessionManager == nil {
		return
	}

	msg := MQTTMessage{
		ID:             id.Short(),
		Timestamp:      time.Now(),
		Topic:          topic,
		Payload:        string(payload),
		PayloadFormat:  detectPayloadFormat(payload),
		QoS:            0,
		Retain:         false,
		Direction:      "incoming",
		IsMockResponse: true,
		MockResponseID: responseID,
	}

	b.sessionManager.NotifyMessage(b.config.ID, msg)
}

// notifyTestPanelPublish notifies sessions about a published message
func (b *Broker) notifyTestPanelPublish(topic string, payload []byte, qos int, retain bool, clientID string) {
	if b.sessionManager == nil {
		return
	}

	msg := MQTTMessage{
		ID:            id.Short(),
		Timestamp:     time.Now(),
		Topic:         topic,
		Payload:       string(payload),
		PayloadFormat: detectPayloadFormat(payload),
		QoS:           qos,
		Retain:        retain,
		Direction:     "incoming",
		ClientID:      clientID,
	}

	b.sessionManager.NotifyMessage(b.config.ID, msg)
}

// detectPayloadFormat detects the format of a payload
func detectPayloadFormat(payload []byte) string {
	// Try to parse as JSON
	if len(payload) > 0 && (payload[0] == '{' || payload[0] == '[') {
		return "json"
	}
	// Check if it's valid UTF-8 text
	for _, b := range payload {
		if b < 32 && b != '\t' && b != '\n' && b != '\r' {
			return "hex"
		}
	}
	return "text"
}
