// Package recording provides storage for MQTT recordings.
package recording

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MQTTStore provides storage for MQTT recordings.
type MQTTStore struct {
	mu         sync.RWMutex
	recordings map[string]*MQTTRecording // id -> recording
	order      []string                  // Maintains insertion order
	maxSize    int                       // Maximum number of recordings to keep
	dataDir    string                    // Directory for persistent storage
}

// NewMQTTStore creates a new MQTT recording store.
func NewMQTTStore(maxSize int) *MQTTStore {
	if maxSize <= 0 {
		maxSize = 1000 // Default max recordings
	}
	return &MQTTStore{
		recordings: make(map[string]*MQTTRecording),
		order:      make([]string, 0),
		maxSize:    maxSize,
	}
}

// NewMQTTStoreWithDir creates a new MQTT store with persistent storage.
func NewMQTTStoreWithDir(maxSize int, dataDir string) (*MQTTStore, error) {
	store := NewMQTTStore(maxSize)
	store.dataDir = dataDir

	if dataDir != "" {
		if err := os.MkdirAll(dataDir, 0700); err != nil {
			return nil, fmt.Errorf("failed to create data directory: %w", err)
		}
		// Load existing recordings
		if err := store.loadFromDisk(); err != nil {
			return nil, fmt.Errorf("failed to load recordings: %w", err)
		}
	}

	return store, nil
}

// Add adds a recording to the store.
func (s *MQTTStore) Add(r *MQTTRecording) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if we need to evict old recordings
	if len(s.order) >= s.maxSize {
		// Remove oldest recording
		oldest := s.order[0]
		delete(s.recordings, oldest)
		s.order = s.order[1:]

		// Remove from disk if persistent
		if s.dataDir != "" {
			os.Remove(s.recordingFilename(oldest))
		}
	}

	s.recordings[r.ID] = r
	s.order = append(s.order, r.ID)

	// Save to disk if persistent
	if s.dataDir != "" {
		return s.saveRecording(r)
	}

	return nil
}

// Get retrieves a recording by ID.
func (s *MQTTStore) Get(id string) *MQTTRecording {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recordings[id]
}

// List returns recordings matching the filter.
func (s *MQTTStore) List(filter MQTTRecordingFilter) ([]*MQTTRecording, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*MQTTRecording

	// Iterate in reverse order (newest first)
	for i := len(s.order) - 1; i >= 0; i-- {
		r := s.recordings[s.order[i]]
		if r == nil {
			continue
		}

		// Apply filters
		if filter.TopicPattern != "" && !matchMQTTTopic(filter.TopicPattern, r.Topic) {
			continue
		}
		if filter.ClientID != "" && r.ClientID != filter.ClientID {
			continue
		}
		if filter.Direction != "" && r.Direction != filter.Direction {
			continue
		}

		result = append(result, r)
	}

	total := len(result)

	// Apply pagination
	if filter.Offset > 0 {
		if filter.Offset >= len(result) {
			return nil, total
		}
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, total
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

// Delete removes a recording by ID.
func (s *MQTTStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.recordings[id]; !ok {
		return ErrNotFound
	}

	delete(s.recordings, id)

	// Remove from order
	for i, oid := range s.order {
		if oid == id {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}

	// Remove from disk if persistent
	if s.dataDir != "" {
		os.Remove(s.recordingFilename(id))
	}

	return nil
}

// Clear removes all recordings.
func (s *MQTTStore) Clear() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.recordings)
	s.recordings = make(map[string]*MQTTRecording)
	s.order = make([]string, 0)

	// Clear disk if persistent
	if s.dataDir != "" {
		entries, err := os.ReadDir(s.dataDir)
		if err == nil {
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "mqtt_") && strings.HasSuffix(entry.Name(), ".json") {
					os.Remove(filepath.Join(s.dataDir, entry.Name()))
				}
			}
		}
	}

	return count
}

// Count returns the number of recordings.
func (s *MQTTStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.recordings)
}

// ListByTopic returns recordings for a specific topic.
func (s *MQTTStore) ListByTopic(topic string) []*MQTTRecording {
	recordings, _ := s.List(MQTTRecordingFilter{TopicPattern: topic})
	return recordings
}

// Export exports all recordings as JSON.
func (s *MQTTStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	recordings := make([]*MQTTRecording, 0, len(s.order))
	for _, id := range s.order {
		if r := s.recordings[id]; r != nil {
			recordings = append(recordings, r)
		}
	}

	return json.MarshalIndent(recordings, "", "  ")
}

// recordingFilename returns the filename for a recording.
func (s *MQTTStore) recordingFilename(id string) string {
	return filepath.Join(s.dataDir, "mqtt_"+id+".json")
}

// saveRecording saves a recording to disk.
func (s *MQTTStore) saveRecording(r *MQTTRecording) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal recording: %w", err)
	}

	filename := s.recordingFilename(r.ID)
	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// loadFromDisk loads recordings from the data directory.
func (s *MQTTStore) loadFromDisk() error {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var recordings []*MQTTRecording

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "mqtt_") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.dataDir, entry.Name()))
		if err != nil {
			continue // Skip files we can't read
		}

		var r MQTTRecording
		if err := json.Unmarshal(data, &r); err != nil {
			continue // Skip invalid files
		}

		recordings = append(recordings, &r)
	}

	// Sort by timestamp (oldest first)
	sort.Slice(recordings, func(i, j int) bool {
		return recordings[i].Timestamp.Before(recordings[j].Timestamp)
	})

	// Add to store
	for _, r := range recordings {
		s.recordings[r.ID] = r
		s.order = append(s.order, r.ID)
	}

	// Trim if over max size
	for len(s.order) > s.maxSize {
		oldest := s.order[0]
		delete(s.recordings, oldest)
		s.order = s.order[1:]
		os.Remove(s.recordingFilename(oldest))
	}

	return nil
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &MQTTRecordingStats{
		TotalRecordings: len(s.recordings),
		ByTopic:         make(map[string]int),
		ByDirection:     make(map[string]int),
		ByQoS:           make(map[int]int),
	}

	for _, r := range s.recordings {
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
