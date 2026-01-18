// Package recording provides storage for gRPC recordings.
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

// GRPCStore provides storage for gRPC recordings.
type GRPCStore struct {
	mu         sync.RWMutex
	recordings map[string]*GRPCRecording // id -> recording
	order      []string                  // Maintains insertion order
	maxSize    int                       // Maximum number of recordings to keep
	dataDir    string                    // Directory for persistent storage
}

// NewGRPCStore creates a new gRPC recording store.
func NewGRPCStore(maxSize int) *GRPCStore {
	if maxSize <= 0 {
		maxSize = 1000 // Default max recordings
	}
	return &GRPCStore{
		recordings: make(map[string]*GRPCRecording),
		order:      make([]string, 0),
		maxSize:    maxSize,
	}
}

// NewGRPCStoreWithDir creates a new gRPC store with persistent storage.
func NewGRPCStoreWithDir(maxSize int, dataDir string) (*GRPCStore, error) {
	store := NewGRPCStore(maxSize)
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
func (s *GRPCStore) Add(r *GRPCRecording) error {
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
func (s *GRPCStore) Get(id string) *GRPCRecording {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recordings[id]
}

// List returns recordings matching the filter.
func (s *GRPCStore) List(filter GRPCRecordingFilter) ([]*GRPCRecording, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*GRPCRecording

	// Iterate in reverse order (newest first)
	for i := len(s.order) - 1; i >= 0; i-- {
		r := s.recordings[s.order[i]]
		if r == nil {
			continue
		}

		// Apply filters
		if filter.Service != "" && r.Service != filter.Service {
			continue
		}
		if filter.Method != "" && r.Method != filter.Method {
			continue
		}
		if filter.StreamType != "" && string(r.StreamType) != filter.StreamType {
			continue
		}
		if filter.HasError != nil {
			hasError := r.Error != nil
			if *filter.HasError != hasError {
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
func (s *GRPCStore) Delete(id string) error {
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
func (s *GRPCStore) Clear() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.recordings)
	s.recordings = make(map[string]*GRPCRecording)
	s.order = make([]string, 0)

	// Clear disk if persistent
	if s.dataDir != "" {
		entries, err := os.ReadDir(s.dataDir)
		if err == nil {
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), "grpc_") && strings.HasSuffix(entry.Name(), ".json") {
					os.Remove(filepath.Join(s.dataDir, entry.Name()))
				}
			}
		}
	}

	return count
}

// Count returns the number of recordings.
func (s *GRPCStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.recordings)
}

// ListByService returns recordings for a specific service.
func (s *GRPCStore) ListByService(service string) []*GRPCRecording {
	recordings, _ := s.List(GRPCRecordingFilter{Service: service})
	return recordings
}

// ListByMethod returns recordings for a specific method.
func (s *GRPCStore) ListByMethod(service, method string) []*GRPCRecording {
	recordings, _ := s.List(GRPCRecordingFilter{Service: service, Method: method})
	return recordings
}

// Export exports all recordings as JSON.
func (s *GRPCStore) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	recordings := make([]*GRPCRecording, 0, len(s.order))
	for _, id := range s.order {
		if r := s.recordings[id]; r != nil {
			recordings = append(recordings, r)
		}
	}

	return json.MarshalIndent(recordings, "", "  ")
}

// recordingFilename returns the filename for a recording.
func (s *GRPCStore) recordingFilename(id string) string {
	return filepath.Join(s.dataDir, "grpc_"+id+".json")
}

// saveRecording saves a recording to disk.
func (s *GRPCStore) saveRecording(r *GRPCRecording) error {
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
func (s *GRPCStore) loadFromDisk() error {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var recordings []*GRPCRecording

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "grpc_") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.dataDir, entry.Name()))
		if err != nil {
			continue // Skip files we can't read
		}

		var r GRPCRecording
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

// GRPCRecordingStats contains statistics about gRPC recordings.
type GRPCRecordingStats struct {
	TotalRecordings int            `json:"totalRecordings"`
	ByService       map[string]int `json:"byService"`
	ByStreamType    map[string]int `json:"byStreamType"`
	ErrorCount      int            `json:"errorCount"`
	OldestTimestamp *time.Time     `json:"oldestTimestamp,omitempty"`
	NewestTimestamp *time.Time     `json:"newestTimestamp,omitempty"`
}

// Stats returns statistics about the recordings.
func (s *GRPCStore) Stats() *GRPCRecordingStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &GRPCRecordingStats{
		TotalRecordings: len(s.recordings),
		ByService:       make(map[string]int),
		ByStreamType:    make(map[string]int),
	}

	for _, r := range s.recordings {
		stats.ByService[r.Service]++
		stats.ByStreamType[string(r.StreamType)]++

		if r.Error != nil {
			stats.ErrorCount++
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
