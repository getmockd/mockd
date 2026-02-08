// Package recording provides storage for MQTT recordings.
package recording

import (
	"strings"
	"time"
)

// MQTTStore provides storage for MQTT recordings.
type MQTTStore struct {
	*RecordingStore[MQTTRecording]
}

// NewMQTTStore creates a new MQTT recording store.
func NewMQTTStore(maxSize int) *MQTTStore {
	return &MQTTStore{NewRecordingStore[MQTTRecording](maxSize, "mqtt_")}
}

// NewMQTTStoreWithDir creates a new MQTT store with persistent storage.
func NewMQTTStoreWithDir(maxSize int, dataDir string) (*MQTTStore, error) {
	s, err := NewRecordingStoreWithDir[MQTTRecording](maxSize, dataDir, "mqtt_")
	if err != nil {
		return nil, err
	}
	return &MQTTStore{s}, nil
}

// List returns recordings matching the filter.
func (s *MQTTStore) List(filter MQTTRecordingFilter) ([]*MQTTRecording, int) {
	return s.ListFiltered(func(r *MQTTRecording) bool {
		if filter.TopicPattern != "" && !matchMQTTTopic(filter.TopicPattern, r.Topic) {
			return false
		}
		if filter.ClientID != "" && r.ClientID != filter.ClientID {
			return false
		}
		if filter.Direction != "" && r.Direction != filter.Direction {
			return false
		}
		return true
	}, filter.Offset, filter.Limit)
}

// ListByTopic returns recordings for a specific topic.
func (s *MQTTStore) ListByTopic(topic string) []*MQTTRecording {
	recordings, _ := s.List(MQTTRecordingFilter{TopicPattern: topic})
	return recordings
}

// MQTTRecordingStats contains statistics about MQTT recordings.
type MQTTRecordingStats struct {
	TotalRecordings int            `json:"totalRecordings"`
	ByTopic         map[string]int `json:"byTopic"`
	ByDirection     map[string]int `json:"byDirection"`
	ByQoS           map[int]int    `json:"byQoS"`
	OldestTimestamp *time.Time     `json:"oldestTimestamp,omitempty"`
	NewestTimestamp *time.Time     `json:"newestTimestamp,omitempty"`
}

// Stats returns statistics about the recordings.
func (s *MQTTStore) Stats() *MQTTRecordingStats {
	all := s.All()

	stats := &MQTTRecordingStats{
		TotalRecordings: len(all),
		ByTopic:         make(map[string]int),
		ByDirection:     make(map[string]int),
		ByQoS:           make(map[int]int),
	}

	for _, r := range all {
		stats.ByTopic[r.Topic]++
		stats.ByDirection[string(r.Direction)]++
		stats.ByQoS[r.QoS]++

		if stats.OldestTimestamp == nil || r.Timestamp.Before(*stats.OldestTimestamp) {
			t := r.Timestamp
			stats.OldestTimestamp = &t
		}
		if stats.NewestTimestamp == nil || r.Timestamp.After(*stats.NewestTimestamp) {
			t := r.Timestamp
			stats.NewestTimestamp = &t
		}
	}

	return stats
}

// matchMQTTTopic matches a topic against a pattern using MQTT wildcard rules.
// + matches exactly one level
// # matches zero or more levels (must be at end)
func matchMQTTTopic(pattern, topic string) bool {
	// If no wildcards, do exact match
	if !strings.Contains(pattern, "+") && !strings.Contains(pattern, "#") {
		return pattern == topic
	}

	patternParts := strings.Split(pattern, "/")
	topicParts := strings.Split(topic, "/")

	patternIdx := 0
	topicIdx := 0

	for patternIdx < len(patternParts) && topicIdx < len(topicParts) {
		patternPart := patternParts[patternIdx]

		if patternPart == "#" {
			// # matches everything from here to the end
			return true
		}

		if patternPart == "+" {
			// + matches exactly one level
			patternIdx++
			topicIdx++
			continue
		}

		// Exact match required
		if patternPart != topicParts[topicIdx] {
			return false
		}

		patternIdx++
		topicIdx++
	}

	// Check if we consumed both pattern and topic
	if patternIdx == len(patternParts) && topicIdx == len(topicParts) {
		return true
	}

	// Special case: pattern ends with # and we consumed all pattern parts
	if patternIdx < len(patternParts) && patternParts[patternIdx] == "#" {
		return true
	}

	return false
}
