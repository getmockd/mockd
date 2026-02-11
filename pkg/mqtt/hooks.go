package mqtt

import (
	"bytes"
	"crypto/subtle"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/metrics"
	"github.com/getmockd/mockd/pkg/requestlog"
	"github.com/google/uuid"
	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"
)

const (
	// maxLogBodySize is the maximum body size to log (10KB)
	maxLogBodySize = 10 * 1024
)

// AuthHook handles authentication and ACL for the MQTT broker
type AuthHook struct {
	mqtt.HookBase
	config *MQTTAuthConfig
}

// NewAuthHook creates a new authentication hook
func NewAuthHook(config *MQTTAuthConfig) *AuthHook {
	return &AuthHook{
		config: config,
	}
}

// ID returns the hook identifier
func (h *AuthHook) ID() string {
	return "auth-hook"
}

// Provides indicates which hook methods this hook provides
func (h *AuthHook) Provides(b byte) bool {
	//nolint:gocritic // argument order is intentional
	return bytes.Contains([]byte{
		mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
	}, []byte{b})
}

// OnConnectAuthenticate handles client authentication
func (h *AuthHook) OnConnectAuthenticate(cl *mqtt.Client, pk packets.Packet) bool {
	if h.config == nil || !h.config.Enabled {
		return true // Allow all connections if auth is disabled
	}

	username := string(cl.Properties.Username)
	password := string(pk.Connect.Password)

	for _, user := range h.config.Users {
		usernameMatch := subtle.ConstantTimeCompare([]byte(user.Username), []byte(username)) == 1
		passwordMatch := subtle.ConstantTimeCompare([]byte(user.Password), []byte(password)) == 1
		if usernameMatch && passwordMatch {
			return true
		}
	}

	return false
}

// OnACLCheck verifies if a client has permission for a topic operation
func (h *AuthHook) OnACLCheck(cl *mqtt.Client, topic string, write bool) bool {
	if h.config == nil || !h.config.Enabled {
		return true // Allow all operations if auth is disabled
	}

	username := string(cl.Properties.Username)

	for _, user := range h.config.Users {
		if user.Username != username {
			continue
		}

		// Check ACL rules - most restrictive wins
		matched := false
		allowed := true
		for _, rule := range user.ACL {
			if matchTopic(rule.Topic, topic) {
				matched = true
				if !checkAccess(rule.Access, write) {
					allowed = false
					break // Any deny is final
				}
			}
		}
		if matched {
			return allowed
		}

		// If user has no ACL rules, allow all operations
		if len(user.ACL) == 0 {
			return true
		}
	}

	return false
}

// matchTopic checks if a topic pattern matches a topic
// Supports MQTT wildcards: + (single level) and # (multi-level)
func matchTopic(pattern, topic string) bool {
	patternParts := strings.Split(pattern, "/")
	topicParts := strings.Split(topic, "/")

	for i, part := range patternParts {
		if part == "#" {
			// # matches everything remaining
			return true
		}

		if i >= len(topicParts) {
			return false
		}

		if part == "+" {
			// + matches any single level
			continue
		}

		if part != topicParts[i] {
			return false
		}
	}

	return len(patternParts) == len(topicParts)
}

// checkAccess verifies if the access level allows the operation
func checkAccess(access string, write bool) bool {
	switch strings.ToLower(access) {
	case "readwrite", "all":
		return true
	case "read", "subscribe":
		return !write
	case "write", "publish":
		return write
	default:
		return false
	}
}

// MessageHook handles message events for the MQTT broker
type MessageHook struct {
	mqtt.HookBase
	broker *Broker
}

// NewMessageHook creates a new message hook
func NewMessageHook(broker *Broker) *MessageHook {
	return &MessageHook{
		broker: broker,
	}
}

// ID returns the hook identifier
func (h *MessageHook) ID() string {
	return "message-hook"
}

// Provides indicates which hook methods this hook provides
func (h *MessageHook) Provides(b byte) bool {
	//nolint:gocritic // argument order is intentional
	return bytes.Contains([]byte{
		mqtt.OnPublish,
		mqtt.OnSubscribed,
		mqtt.OnUnsubscribed,
	}, []byte{b})
}

// OnPublish handles incoming publish messages
func (h *MessageHook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	startTime := time.Now()
	topic := pk.TopicName
	payload := pk.Payload

	// Record metrics (deferred to capture duration)
	defer func() {
		recordMQTTMetrics("PUBLISH", topic, time.Since(startTime))
	}()

	// Record message if recording is enabled
	h.broker.mu.RLock()
	recordingEnabled := h.broker.recordingEnabled
	recordingStore := h.broker.recordingStore
	requestLogger := h.broker.requestLogger
	h.broker.mu.RUnlock()

	if recordingEnabled && recordingStore != nil {
		recordingData := MQTTRecordingData{
			Topic:     topic,
			Payload:   payload,
			QoS:       pk.FixedHeader.Qos,
			Retain:    pk.FixedHeader.Retain,
			ClientID:  cl.ID,
			Direction: "publish",
		}
		// Record asynchronously to avoid blocking the message flow
		go func() {
			_ = recordingStore.Add(recordingData)
		}()
	}

	// Log the publish event using unified request logger
	if requestLogger != nil {
		entry := h.createPublishLogEntry(cl, pk)
		go requestLogger.Log(entry)
	}

	// Notify internal subscribers
	h.broker.notifySubscribers(topic, payload)

	// Notify test panel sessions
	h.broker.notifyTestPanelPublish(topic, payload, int(pk.FixedHeader.Qos), pk.FixedHeader.Retain, cl.ID)

	// Skip mock response handling if this topic is currently being published
	// as a mock response (prevents infinite loops)
	if !h.broker.isMockResponseActive(topic) {
		// Handle conditional responses first (they take priority)
		conditionalHandled := false
		if h.broker.conditionalResponseHandler != nil {
			conditionalHandled = h.broker.conditionalResponseHandler.HandlePublish(topic, payload, cl.ID)
		}

		// Handle regular mock responses only if no conditional response was triggered
		if !conditionalHandled && h.broker.responseHandler != nil {
			h.broker.responseHandler.HandlePublish(topic, payload, cl.ID)
		}
	}

	// Check for configured publish handlers
	h.broker.mu.RLock()
	topicConfigs := h.broker.config.Topics
	h.broker.mu.RUnlock()

	for _, tc := range topicConfigs {
		if matchTopic(tc.Topic, topic) && tc.OnPublish != nil {
			// Handle forwarding
			if tc.OnPublish.Forward != "" {
				go func(forwardTopic string, data []byte, qos byte, retain bool) {
					if err := h.broker.Publish(forwardTopic, data, qos, retain); err != nil {
						h.broker.log.Error("failed to forward message",
							"topic", forwardTopic,
							"error", err)
					}
				}(tc.OnPublish.Forward, payload, byte(tc.QoS), tc.Retain)
			}

			// Handle response
			if tc.OnPublish.Response != nil {
				go func(respTopic string, resp *MessageConfig, qos byte, retain bool) {
					if err := h.broker.Publish(respTopic, []byte(resp.Payload), qos, retain); err != nil {
						h.broker.log.Error("failed to publish response",
							"topic", respTopic,
							"error", err)
					}
				}(topic, tc.OnPublish.Response, byte(tc.QoS), tc.Retain)
			}
		}
	}

	return pk, nil
}

// createPublishLogEntry creates a log entry for a publish event
func (h *MessageHook) createPublishLogEntry(cl *mqtt.Client, pk packets.Packet) *requestlog.Entry {
	body := string(pk.Payload)
	bodySize := len(pk.Payload)

	// Truncate body if too large
	if len(body) > maxLogBodySize {
		body = body[:maxLogBodySize] + "... (truncated)"
	}

	return &requestlog.Entry{
		ID:         uuid.New().String(),
		Timestamp:  time.Now(),
		Protocol:   requestlog.ProtocolMQTT,
		Method:     "PUBLISH",
		Path:       pk.TopicName,
		Body:       body,
		BodySize:   bodySize,
		RemoteAddr: cl.Net.Remote,
		DurationMs: 0,
		MQTT: &requestlog.MQTTMeta{
			ClientID:  cl.ID,
			Topic:     pk.TopicName,
			QoS:       int(pk.FixedHeader.Qos),
			Retain:    pk.FixedHeader.Retain,
			Direction: "publish",
			MessageID: pk.PacketID,
		},
	}
}

// OnSubscribed handles client subscriptions
func (h *MessageHook) OnSubscribed(cl *mqtt.Client, pk packets.Packet, reasonCodes []byte) {
	startTime := time.Now()
	clientID := cl.ID

	// Record metrics for each subscription (deferred to capture duration)
	defer func() {
		duration := time.Since(startTime)
		for _, sub := range pk.Filters {
			recordMQTTMetrics("SUBSCRIBE", sub.Filter, duration)
		}
	}()

	// Record subscription if recording is enabled
	h.broker.mu.RLock()
	recordingEnabled := h.broker.recordingEnabled
	recordingStore := h.broker.recordingStore
	requestLogger := h.broker.requestLogger
	h.broker.mu.RUnlock()

	if recordingEnabled && recordingStore != nil {
		for _, sub := range pk.Filters {
			recordingData := MQTTRecordingData{
				Topic:     sub.Filter,
				Payload:   nil,
				QoS:       sub.Qos,
				Retain:    false,
				ClientID:  clientID,
				Direction: "subscribe",
			}
			// Record asynchronously to avoid blocking
			go func(data MQTTRecordingData) {
				_ = recordingStore.Add(data)
			}(recordingData)
		}
	}

	// Log subscription events using unified request logger
	if requestLogger != nil {
		for _, sub := range pk.Filters {
			entry := h.createSubscribeLogEntry(cl, sub.Filter, int(sub.Qos))
			go requestLogger.Log(entry)
		}
	}

	// Track subscriptions for monitoring (skip during shutdown to avoid deadlock)
	if h.broker.stopping.Load() != 0 {
		return
	}
	h.broker.mu.Lock()
	defer h.broker.mu.Unlock()

	if h.broker.clientSubscriptions == nil {
		h.broker.clientSubscriptions = make(map[string][]string)
	}

	for _, sub := range pk.Filters {
		h.broker.clientSubscriptions[clientID] = append(
			h.broker.clientSubscriptions[clientID],
			sub.Filter,
		)
	}
}

// createSubscribeLogEntry creates a log entry for a subscribe event
func (h *MessageHook) createSubscribeLogEntry(cl *mqtt.Client, topicFilter string, qos int) *requestlog.Entry {
	return &requestlog.Entry{
		ID:         uuid.New().String(),
		Timestamp:  time.Now(),
		Protocol:   requestlog.ProtocolMQTT,
		Method:     "SUBSCRIBE",
		Path:       topicFilter,
		Body:       "",
		BodySize:   0,
		RemoteAddr: cl.Net.Remote,
		DurationMs: 0,
		MQTT: &requestlog.MQTTMeta{
			ClientID:  cl.ID,
			Topic:     topicFilter,
			QoS:       qos,
			Retain:    false,
			Direction: "subscribe",
			MessageID: 0,
		},
	}
}

// OnUnsubscribed handles client unsubscriptions
func (h *MessageHook) OnUnsubscribed(cl *mqtt.Client, pk packets.Packet) {
	// Skip during shutdown to avoid deadlock — server.Close() triggers
	// unsubscriptions while Stop() may be waiting for Close() to finish.
	if h.broker.stopping.Load() != 0 {
		return
	}
	h.broker.mu.Lock()
	defer h.broker.mu.Unlock()

	clientID := cl.ID
	if h.broker.clientSubscriptions == nil {
		return
	}

	// Remove unsubscribed topics
	currentSubs := h.broker.clientSubscriptions[clientID]
	var newSubs []string

	for _, filter := range pk.Filters {
		for _, sub := range currentSubs {
			if sub != filter.Filter {
				newSubs = append(newSubs, sub)
			}
		}
	}

	if len(newSubs) == 0 {
		delete(h.broker.clientSubscriptions, clientID)
	} else {
		h.broker.clientSubscriptions[clientID] = newSubs
	}
}

// recordMQTTMetrics records MQTT message metrics.
func recordMQTTMetrics(_, topic string, duration time.Duration) {
	if metrics.RequestsTotal != nil {
		if vec, err := metrics.RequestsTotal.WithLabels("mqtt", topic, "ok"); err == nil {
			_ = vec.Inc()
		}
	}
	if metrics.RequestDuration != nil {
		if vec, err := metrics.RequestDuration.WithLabels("mqtt", topic); err == nil {
			vec.Observe(duration.Seconds())
		}
	}
}
