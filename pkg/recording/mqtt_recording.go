// Package recording provides types for MQTT message recording.
package recording

import (
	"crypto/rand"
	"fmt"
	"time"
)

// MQTTDirection indicates the direction of an MQTT message.
type MQTTDirection string

const (
	// MQTTDirectionPublish indicates a message published by a client to the broker.
	MQTTDirectionPublish MQTTDirection = "publish"
	// MQTTDirectionSubscribe indicates a message received by a client from the broker.
	MQTTDirectionSubscribe MQTTDirection = "subscribe"
)

// MQTTRecording represents a captured MQTT message.
type MQTTRecording struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	// Topic the message was published to or received from
	Topic string `json:"topic"`

	// Payload is the message content
	Payload []byte `json:"payload"`

	// QoS level (0, 1, or 2)
	QoS int `json:"qos"`

	// Retain flag indicates if the message should be retained by the broker
	Retain bool `json:"retain"`

	// ClientID identifies the MQTT client
	ClientID string `json:"clientId"`

	// Direction indicates if message is publish (client->broker) or subscribe (broker->client)
	Direction MQTTDirection `json:"direction"`

	// MessageID is the MQTT packet identifier (optional, used for QoS > 0)
	MessageID uint16 `json:"messageId,omitempty"`
}

// MQTTRecordingFilter defines filtering options for MQTT recordings.
type MQTTRecordingFilter struct {
	// TopicPattern filters by topic (supports MQTT wildcards: + for single level, # for multi-level)
	TopicPattern string `json:"topicPattern,omitempty"`

	// ClientID filters by specific client ID
	ClientID string `json:"clientId,omitempty"`

	// Direction filters by message direction (publish or subscribe)
	Direction MQTTDirection `json:"direction,omitempty"`

	// Limit is the maximum number of recordings to return
	Limit int `json:"limit,omitempty"`

	// Offset is the number of recordings to skip
	Offset int `json:"offset,omitempty"`
}

// NewMQTTRecording creates a new MQTT recording with a unique ID.
func NewMQTTRecording(topic string, payload []byte, qos int, retain bool, clientID string, direction MQTTDirection) *MQTTRecording {
	return &MQTTRecording{
		ID:        generateMQTTID(),
		Timestamp: time.Now(),
		Topic:     topic,
		Payload:   payload,
		QoS:       qos,
		Retain:    retain,
		ClientID:  clientID,
		Direction: direction,
	}
}

// generateMQTTID generates a unique identifier for MQTT recordings.
func generateMQTTID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("mqtt-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// SetMessageID sets the MQTT packet identifier.
func (r *MQTTRecording) SetMessageID(id uint16) {
	r.MessageID = id
}

// PayloadString returns the payload as a string.
func (r *MQTTRecording) PayloadString() string {
	return string(r.Payload)
}

// IsPublish returns true if the message direction is publish (client->broker).
func (r *MQTTRecording) IsPublish() bool {
	return r.Direction == MQTTDirectionPublish
}

// IsSubscribe returns true if the message direction is subscribe (broker->client).
func (r *MQTTRecording) IsSubscribe() bool {
	return r.Direction == MQTTDirectionSubscribe
}
