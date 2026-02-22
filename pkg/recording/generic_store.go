// Package recording provides a generic, type-safe store for protocol recordings.
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

// Recordable is the interface that recording types must satisfy for the generic store.
type Recordable interface {
	GetID() string
	GetTimestamp() time.Time
}

// RecordingStore is a generic, concurrency-safe, ordered store for recordings.
// It supports LRU eviction, optional disk persistence, and filtered listing.
type RecordingStore[T Recordable] struct {
	mu         sync.RWMutex
	recordings map[string]*T
	order      []string // Maintains insertion order
	maxSize    int      // Maximum number of recordings to keep
	dataDir    string   // Directory for persistent storage
	filePrefix string   // Filename prefix for disk files (e.g. "mqtt_", "soap_")
}

// NewRecordingStore creates a new in-memory recording store.
func NewRecordingStore[T Recordable](maxSize int, filePrefix string) *RecordingStore[T] {
	if maxSize <= 0 {
		maxSize = 1000 // Default max recordings
	}
	return &RecordingStore[T]{
		recordings: make(map[string]*T),
		order:      make([]string, 0),
		maxSize:    maxSize,
		filePrefix: filePrefix,
	}
}

// NewRecordingStoreWithDir creates a new store with persistent storage.
func NewRecordingStoreWithDir[T Recordable](maxSize int, dataDir, filePrefix string) (*RecordingStore[T], error) {
	store := NewRecordingStore[T](maxSize, filePrefix)
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

// Add adds a recording to the store with LRU eviction and optional disk persistence.
func (s *RecordingStore[T]) Add(r *T) error {
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
			_ = os.Remove(s.recordingFilename(oldest))
		}
	}

	id := (*r).GetID()
	s.recordings[id] = r
	s.order = append(s.order, id)

	// Save to disk if persistent
	if s.dataDir != "" {
		return s.saveRecording(r)
	}

	return nil
}

// Get retrieves a recording by ID.
func (s *RecordingStore[T]) Get(id string) *T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.recordings[id]
}

// Delete removes a recording by ID.
func (s *RecordingStore[T]) Delete(id string) error {
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
		_ = os.Remove(s.recordingFilename(id))
	}

	return nil
}

// Clear removes all recordings and returns the count removed.
func (s *RecordingStore[T]) Clear() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := len(s.recordings)
	s.recordings = make(map[string]*T)
	s.order = make([]string, 0)

	// Clear disk if persistent
	if s.dataDir != "" {
		entries, err := os.ReadDir(s.dataDir)
		if err == nil {
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), s.filePrefix) && strings.HasSuffix(entry.Name(), ".json") {
					_ = os.Remove(filepath.Join(s.dataDir, entry.Name()))
				}
			}
		}
	}

	return count
}

// Count returns the number of recordings.
func (s *RecordingStore[T]) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.recordings)
}

// Export exports all recordings as JSON in insertion order.
func (s *RecordingStore[T]) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	recordings := make([]*T, 0, len(s.order))
	for _, id := range s.order {
		if r := s.recordings[id]; r != nil {
			recordings = append(recordings, r)
		}
	}

	return json.MarshalIndent(recordings, "", "  ")
}

// ListFiltered returns recordings matching the predicate, newest first, with pagination.
func (s *RecordingStore[T]) ListFiltered(predicate func(*T) bool, offset, limit int) ([]*T, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*T

	// Iterate in reverse order (newest first)
	for i := len(s.order) - 1; i >= 0; i-- {
		r := s.recordings[s.order[i]]
		if r == nil {
			continue
		}

		if predicate != nil && !predicate(r) {
			continue
		}

		result = append(result, r)
	}

	total := len(result)

	// Apply pagination
	if offset > 0 {
		if offset >= len(result) {
			return []*T{}, total
		}
		result = result[offset:]
	}
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}

	return result, total
}

// All returns all recordings in insertion order (oldest first).
func (s *RecordingStore[T]) All() []*T {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*T, 0, len(s.order))
	for _, id := range s.order {
		if r := s.recordings[id]; r != nil {
			result = append(result, r)
		}
	}
	return result
}

// recordingFilename returns the filename for a recording.
func (s *RecordingStore[T]) recordingFilename(id string) string {
	return filepath.Join(s.dataDir, s.filePrefix+id+".json")
}

// saveRecording saves a recording to disk.
func (s *RecordingStore[T]) saveRecording(r *T) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal recording: %w", err)
	}

	filename := s.recordingFilename((*r).GetID())
	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// UpdateTimestampRange updates oldest/newest timestamp pointers
// given a candidate timestamp. This is used by protocol-specific Stats
// methods to deduplicate min/max timestamp tracking.
func UpdateTimestampRange(oldest, newest **time.Time, t time.Time) {
	if *oldest == nil || t.Before(**oldest) {
		ts := t
		*oldest = &ts
	}
	if *newest == nil || t.After(**newest) {
		ts := t
		*newest = &ts
	}
}

// loadFromDisk loads recordings from the data directory.
func (s *RecordingStore[T]) loadFromDisk() error {
	entries, err := os.ReadDir(s.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var recordings []*T

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), s.filePrefix) || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(s.dataDir, entry.Name()))
		if err != nil {
			continue // Skip files we can't read
		}

		var r T
		if err := json.Unmarshal(data, &r); err != nil {
			continue // Skip invalid files
		}

		recordings = append(recordings, &r)
	}

	// Sort by timestamp (oldest first)
	sort.Slice(recordings, func(i, j int) bool {
		return (*recordings[i]).GetTimestamp().Before((*recordings[j]).GetTimestamp())
	})

	// Add to store
	for _, r := range recordings {
		id := (*r).GetID()
		s.recordings[id] = r
		s.order = append(s.order, id)
	}

	// Trim if over max size
	for len(s.order) > s.maxSize {
		oldest := s.order[0]
		delete(s.recordings, oldest)
		s.order = s.order[1:]
		_ = os.Remove(s.recordingFilename(oldest))
	}

	return nil
}
