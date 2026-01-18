// Package recording provides storage for SOAP recordings.
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

// SOAPStore provides storage for SOAP recordings.
type SOAPStore struct {
	mu         sync.RWMutex
	recordings map[string]*SOAPRecording // id -> recording
	order      []string                  // Maintains insertion order
	maxSize    int                       // Maximum number of recordings to keep
	dataDir    string                    // Directory for persistent storage
}

// NewSOAPStore creates a new SOAP recording store.
func NewSOAPStore(maxSize int) *SOAPStore {
	if maxSize <= 0 {
		maxSize = 1000 // Default max recordings
	}
	return &SOAPStore{
		recordings: make(map[string]*SOAPRecording),
		order:      make([]string, 0),
		maxSize:    maxSize,
	}
}

// NewSOAPStoreWithDir creates a new SOAP store with persistent storage.
func NewSOAPStoreWithDir(maxSize int, dataDir string) (*SOAPStore, error) {
	store := NewSOAPStore(maxSize)
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
func (s *SOAPStore) Add(r *SOAPRecording) error {
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
func (s *SOAPStore) Get(id string) *SOAPRecording {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recordings[id]
}

// List returns recordings matching the filter.
func (s *SOAPStore) List(filter SOAPRecordingFilter) ([]*SOAPRecording, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*SOAPRecording

	// Iterate in reverse order (newest first)
	for i := len(s.order) - 1; i >= 0; i-- {
		r := s.recordings[s.order[i]]
		if r == nil {
			continue
		}

		// Apply filters
		if filter.Endpoint != "" && r.Endpoint != filter.Endpoint {
			continue
		}
		if filter.Operation != "" && r.Operation != filter.Operation {
			continue
		}
		if filter.SOAPAction != "" && r.SOAPAction != filter.SOAPAction {
			continue
		}
		if filter.HasFault != nil {
			if *filter.HasFault != r.HasFault {
				continue
			}
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

// Delete removes a recording by ID.
func (s *SOAPStore) Delete(id string) error {
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
func (s *SOAPStore) Clear() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.recordings)
	s.recordings = make(map[string]*SOAPRecording)
	s.order = make([]string, 0)

	// Clear disk if persistent
	if s.dataDir != "" {
		entries, err := os.ReadDir(s.dataDir)
		if err == nil {
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "soap_") && strings.HasSuffix(entry.Name(), ".json") {
					os.Remove(filepath.Join(s.dataDir, entry.Name()))
				}
			}
		}
	}

	return count
}

// Count returns the number of recordings.
func (s *SOAPStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.recordings)
}

// ListByEndpoint returns recordings for a specific endpoint.
func (s *SOAPStore) ListByEndpoint(endpoint string) []*SOAPRecording {
	recordings, _ := s.List(SOAPRecordingFilter{Endpoint: endpoint})
	return recordings
}

// ListByOperation returns recordings for a specific operation.
func (s *SOAPStore) ListByOperation(operation string) []*SOAPRecording {
	recordings, _ := s.List(SOAPRecordingFilter{Operation: operation})
	return recordings
}

// Export exports all recordings as JSON.
func (s *SOAPStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	recordings := make([]*SOAPRecording, 0, len(s.order))
	for _, id := range s.order {
		if r := s.recordings[id]; r != nil {
			recordings = append(recordings, r)
		}
	}

	return json.MarshalIndent(recordings, "", "  ")
}

// recordingFilename returns the filename for a recording.
func (s *SOAPStore) recordingFilename(id string) string {
	return filepath.Join(s.dataDir, "soap_"+id+".json")
}

// saveRecording saves a recording to disk.
func (s *SOAPStore) saveRecording(r *SOAPRecording) error {
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
func (s *SOAPStore) loadFromDisk() error {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var recordings []*SOAPRecording

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "soap_") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.dataDir, entry.Name()))
		if err != nil {
			continue // Skip files we can't read
		}

		var r SOAPRecording
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

// SOAPRecordingStats contains statistics about SOAP recordings.
type SOAPRecordingStats struct {
	TotalRecordings int            `json:"totalRecordings"`
	ByEndpoint      map[string]int `json:"byEndpoint"`
	ByOperation     map[string]int `json:"byOperation"`
	FaultCount      int            `json:"faultCount"`
	OldestTimestamp *time.Time     `json:"oldestTimestamp,omitempty"`
	NewestTimestamp *time.Time     `json:"newestTimestamp,omitempty"`
}

// Stats returns statistics about the recordings.
func (s *SOAPStore) Stats() *SOAPRecordingStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &SOAPRecordingStats{
		TotalRecordings: len(s.recordings),
		ByEndpoint:      make(map[string]int),
		ByOperation:     make(map[string]int),
	}

	for _, r := range s.recordings {
		stats.ByEndpoint[r.Endpoint]++
		stats.ByOperation[r.Operation]++

		if r.HasFault {
			stats.FaultCount++
		}

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
