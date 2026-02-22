// Package recording provides persistent file-based storage for stream recordings.
package recording

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/store"
)

// FileStore provides persistent file-based storage for stream recordings.
type FileStore struct {
	mu     sync.RWMutex
	config StorageConfig

	// Active recording sessions
	sessions map[string]*StreamRecordingSession

	// Cache of recording summaries for faster listing
	summaryCache map[string]*RecordingSummary
	cacheTTL     time.Duration
}

// StreamRecordingSession represents an active stream recording session.
type StreamRecordingSession struct {
	mu        sync.Mutex
	recording *StreamRecording
	closed    bool
	startTime time.Time
	seq       int64
}

// ID returns the recording ID (which is used as the session ID).
func (s *StreamRecordingSession) ID() string {
	return s.recording.ID
}

// Recording returns the underlying recording.
func (s *StreamRecordingSession) Recording() *StreamRecording {
	return s.recording
}

// NewFileStore creates a new file-based recording store.
func NewFileStore(config StorageConfig) (*FileStore, error) {
	// Apply defaults - recordings go in data dir per XDG spec
	if config.DataDir == "" {
		config.DataDir = store.DefaultRecordingsDir()
	}
	if config.MaxBytes == 0 {
		config.MaxBytes = DefaultMaxStorageBytes
	}
	if config.WarnPercent == 0 {
		config.WarnPercent = DefaultWarnPercent
	}
	if len(config.FilterHeaders) == 0 {
		config.FilterHeaders = DefaultFilterHeaders
	}
	if config.RedactValue == "" {
		config.RedactValue = "[REDACTED]"
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.DataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	store := &FileStore{
		config:       config,
		sessions:     make(map[string]*StreamRecordingSession),
		summaryCache: make(map[string]*RecordingSummary),
		cacheTTL:     5 * time.Second,
	}

	return store, nil
}

// StartRecording starts a new recording session.
func (s *FileStore) StartRecording(protocol Protocol, metadata RecordingMetadata) (*StreamRecordingSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check storage limits
	stats, err := s.getStatsLocked()
	if err != nil {
		return nil, fmt.Errorf("failed to check storage: %w", err)
	}
	if stats.UsedBytes >= s.config.MaxBytes {
		return nil, ErrStorageFull
	}

	// Filter sensitive headers
	if metadata.Headers != nil {
		metadata.Headers = s.filterHeaders(metadata.Headers)
	}

	recording := NewStreamRecording(protocol, metadata)
	session := &StreamRecordingSession{
		recording: recording,
		startTime: recording.StartTime,
		seq:       0,
	}

	s.sessions[recording.ID] = session

	// Invalidate cache
	s.summaryCache = make(map[string]*RecordingSummary)

	return session, nil
}

// filterHeaders removes sensitive headers from the map.
func (s *FileStore) filterHeaders(headers map[string]string) map[string]string {
	filtered := make(map[string]string)
	for k, v := range headers {
		isFiltered := false
		for _, fh := range s.config.FilterHeaders {
			if strings.EqualFold(k, fh) {
				isFiltered = true
				break
			}
		}
		if isFiltered {
			filtered[k] = s.config.RedactValue
		} else {
			filtered[k] = v
		}
	}
	return filtered
}

// AppendWebSocketFrame appends a WebSocket frame to a recording session.
func (s *FileStore) AppendWebSocketFrame(sessionID string, dir Direction, msgType MessageType, data []byte) error {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok {
		return ErrNotFound
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.closed {
		return ErrNoActiveSession
	}

	session.seq++
	frame := NewWebSocketFrame(session.seq, session.startTime, dir, msgType, data)
	session.recording.AddWebSocketFrame(frame)

	return nil
}

// AppendWebSocketCloseFrame appends a close frame to a recording session.
func (s *FileStore) AppendWebSocketCloseFrame(sessionID string, dir Direction, code int, reason string) error {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok {
		return ErrNotFound
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.closed {
		return ErrNoActiveSession
	}

	session.seq++
	frame := NewWebSocketCloseFrame(session.seq, session.startTime, dir, code, reason)
	session.recording.AddWebSocketFrame(frame)
	session.recording.SetWebSocketClose(code, reason)

	return nil
}

// AppendSSEEvent appends an SSE event to a recording session.
func (s *FileStore) AppendSSEEvent(sessionID string, eventType, data, id string, retry *int) error {
	s.mu.RLock()
	session, ok := s.sessions[sessionID]
	s.mu.RUnlock()

	if !ok {
		return ErrNotFound
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.closed {
		return ErrNoActiveSession
	}

	session.seq++

	// Calculate first event time for relative timestamps
	var firstEventTime time.Time
	if session.recording.SSE != nil && len(session.recording.SSE.Events) > 0 {
		firstEventTime = session.recording.SSE.Events[0].Timestamp
	}

	event := NewSSEEvent(session.seq, firstEventTime, eventType, data, id, retry)
	session.recording.AddSSEEvent(event)

	return nil
}

// CompleteRecording completes a recording session and persists it.
//
//nolint:dupl // structural similarity with MarkIncomplete is intentional
func (s *FileStore) CompleteRecording(sessionID string) (*StreamRecording, error) {
	s.mu.Lock()
	session, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	// Don't delete session until save succeeds
	s.mu.Unlock()

	session.mu.Lock()

	if session.closed {
		session.mu.Unlock()
		return nil, ErrNoActiveSession
	}

	session.recording.Complete()

	// Save to disk BEFORE removing from sessions map
	if err := s.saveRecording(session.recording); err != nil {
		// Recording failed to save - don't mark as closed so it can be retried
		session.recording.Status = RecordingStatusRecording // Revert status
		session.mu.Unlock()
		return nil, fmt.Errorf("failed to save recording: %w", err)
	}

	// Only mark closed after successful save
	session.closed = true
	// Capture recording before releasing lock
	recording := session.recording
	session.mu.Unlock()

	// Now safe to acquire store lock (not holding session lock)
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.summaryCache = make(map[string]*RecordingSummary)
	s.mu.Unlock()

	return recording, nil
}

// CancelRecording cancels a recording session without saving.
func (s *FileStore) CancelRecording(sessionID string) error {
	s.mu.Lock()
	session, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	delete(s.sessions, sessionID)
	s.mu.Unlock()

	session.mu.Lock()
	defer session.mu.Unlock()

	session.closed = true
	return nil
}

// MarkIncomplete marks a recording as incomplete and saves it.
//
//nolint:dupl // structural similarity with CompleteRecording is intentional
func (s *FileStore) MarkIncomplete(sessionID string) (*StreamRecording, error) {
	s.mu.Lock()
	session, ok := s.sessions[sessionID]
	if !ok {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	// Don't delete session until save succeeds
	s.mu.Unlock()

	session.mu.Lock()

	if session.closed {
		session.mu.Unlock()
		return nil, ErrNoActiveSession
	}

	session.recording.MarkIncomplete()

	// Save to disk BEFORE removing from sessions map
	if err := s.saveRecording(session.recording); err != nil {
		// Recording failed to save - revert status so it can be retried
		session.recording.Status = RecordingStatusRecording
		session.mu.Unlock()
		return nil, fmt.Errorf("failed to save recording: %w", err)
	}

	// Only mark closed after successful save
	session.closed = true
	// Capture recording before releasing lock
	recording := session.recording
	session.mu.Unlock()

	// Now safe to acquire store lock (not holding session lock)
	s.mu.Lock()
	delete(s.sessions, sessionID)
	s.summaryCache = make(map[string]*RecordingSummary)
	s.mu.Unlock()

	return recording, nil
}

// saveRecording saves a recording to disk.
func (s *FileStore) saveRecording(r *StreamRecording) error {
	filename := s.recordingFilename(r.ID)

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal recording: %w", err)
	}

	// Update file size stat
	r.Stats.FileSizeBytes = int64(len(data))

	// Re-marshal with updated stats
	data, err = json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal recording: %w", err)
	}

	if err := os.WriteFile(filename, data, 0600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Invalidate cache to ensure fresh data on next List
	s.mu.Lock()
	delete(s.summaryCache, r.ID)
	s.mu.Unlock()

	return nil
}

// recordingFilename returns the filename for a recording.
func (s *FileStore) recordingFilename(id string) string {
	return filepath.Join(s.config.DataDir, "rec_"+id+".json")
}

// Get retrieves a recording by ID.
func (s *FileStore) Get(id string) (*StreamRecording, error) {
	filename := s.recordingFilename(id)

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var recording StreamRecording
	if err := json.Unmarshal(data, &recording); err != nil {
		return nil, fmt.Errorf("failed to parse recording: %w", err)
	}

	// Validate
	if err := recording.Validate(); err != nil {
		recording.MarkCorrupted()
		return &recording, ErrCorrupted
	}

	return &recording, nil
}

// StreamRecordingFilter defines filtering options for listing.
type StreamRecordingFilter struct {
	Protocol       Protocol `json:"protocol,omitempty"`
	Path           string   `json:"path,omitempty"`
	Tag            string   `json:"tag,omitempty"`
	Status         string   `json:"status,omitempty"`
	IncludeDeleted bool     `json:"includeDeleted,omitempty"`
	Limit          int      `json:"limit,omitempty"`
	Offset         int      `json:"offset,omitempty"`
	SortBy         string   `json:"sortBy,omitempty"`
	SortOrder      string   `json:"sortOrder,omitempty"`
}

// List returns recordings matching the filter.
func (s *FileStore) List(filter StreamRecordingFilter) ([]*RecordingSummary, int, error) {
	// Use write lock since loadSummaryLocked may update cache
	s.mu.Lock()
	defer s.mu.Unlock()

	// Scan directory for recording files
	entries, err := os.ReadDir(s.config.DataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, nil
		}
		return nil, 0, fmt.Errorf("failed to read directory: %w", err)
	}

	var summaries []*RecordingSummary
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "rec_") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Extract ID from filename
		id := strings.TrimPrefix(entry.Name(), "rec_")
		id = strings.TrimSuffix(id, ".json")

		// Load summary (use cache if available) - caller holds write lock
		summary, err := s.loadSummaryLocked(id)
		if err != nil {
			continue // Skip corrupted files
		}

		// Apply filters
		if !s.matchesFilter(summary, filter) {
			continue
		}

		summaries = append(summaries, summary)
	}

	total := len(summaries)

	// Sort
	s.sortSummaries(summaries, filter.SortBy, filter.SortOrder)

	// Apply pagination
	if filter.Offset > 0 {
		if filter.Offset >= len(summaries) {
			return []*RecordingSummary{}, total, nil
		}
		summaries = summaries[filter.Offset:]
	}
	if filter.Limit > 0 && len(summaries) > filter.Limit {
		summaries = summaries[:filter.Limit]
	}

	return summaries, total, nil
}

// loadSummaryLocked loads a recording summary, using cache if available.
// Caller must hold s.mu write lock.
func (s *FileStore) loadSummaryLocked(id string) (*RecordingSummary, error) {
	// Check cache
	if cached, ok := s.summaryCache[id]; ok {
		return cached, nil
	}

	// Load full recording from disk (Get doesn't acquire locks)
	recording, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	summary := recording.ToSummary()
	s.summaryCache[id] = &summary

	return &summary, nil
}

// matchesFilter checks if a summary matches the filter criteria.
func (s *FileStore) matchesFilter(summary *RecordingSummary, filter StreamRecordingFilter) bool {
	// Exclude deleted unless requested
	if !filter.IncludeDeleted && summary.Deleted {
		return false
	}

	// Protocol filter
	if filter.Protocol != "" && summary.Protocol != filter.Protocol {
		return false
	}

	// Path filter (prefix match)
	if filter.Path != "" && !strings.HasPrefix(summary.Path, filter.Path) {
		return false
	}

	// Tag filter
	if filter.Tag != "" {
		hasTag := false
		for _, t := range summary.Tags {
			if t == filter.Tag {
				hasTag = true
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	// Status filter
	if filter.Status != "" && string(summary.Status) != filter.Status {
		return false
	}

	return true
}

// sortSummaries sorts summaries by the specified field.
func (s *FileStore) sortSummaries(summaries []*RecordingSummary, sortBy, sortOrder string) {
	if sortBy == "" {
		sortBy = "startTime"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	sort.Slice(summaries, func(i, j int) bool {
		var less bool
		switch sortBy {
		case "name":
			less = summaries[i].Name < summaries[j].Name
		case "size":
			less = summaries[i].FileSize < summaries[j].FileSize
		case "startTime":
			fallthrough
		default:
			less = summaries[i].StartTime.Before(summaries[j].StartTime)
		}
		if sortOrder == "desc" {
			return !less
		}
		return less
	})
}

// Delete soft-deletes a recording.
func (s *FileStore) Delete(id string) error {
	recording, err := s.Get(id)
	if err != nil {
		return err
	}

	recording.SoftDelete()
	return s.saveRecording(recording)
}

// Purge permanently deletes a recording.
func (s *FileStore) Purge(id string) error {
	filename := s.recordingFilename(id)
	if err := os.Remove(filename); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Invalidate cache
	s.mu.Lock()
	delete(s.summaryCache, id)
	s.mu.Unlock()

	return nil
}

// Update updates recording metadata.
func (s *FileStore) Update(id string, name, description *string, tags []string) error {
	recording, err := s.Get(id)
	if err != nil {
		return err
	}

	if name != nil {
		recording.Name = *name
	}
	if description != nil {
		recording.Description = *description
	}
	if tags != nil {
		recording.Tags = tags
	}
	recording.UpdatedAt = time.Now().Format(time.RFC3339)

	return s.saveRecording(recording)
}

// GetStats returns storage statistics.
func (s *FileStore) GetStats() (*StorageStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getStatsLocked()
}

// getStatsLocked returns stats (caller must hold lock).
func (s *FileStore) getStatsLocked() (*StorageStats, error) {
	stats := &StorageStats{
		MaxBytes: s.config.MaxBytes,
	}

	var oldestTime, newestTime time.Time

	err := filepath.WalkDir(s.config.DataDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			//nolint:nilerr // intentionally skip inaccessible files during directory walk
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			//nolint:nilerr // intentionally skip files where info cannot be retrieved
			return nil
		}

		stats.UsedBytes += info.Size()
		stats.RecordingCount++

		// Extract ID and load for protocol info
		name := d.Name()
		if strings.HasPrefix(name, "rec_") {
			id := strings.TrimPrefix(name, "rec_")
			id = strings.TrimSuffix(id, ".json")

			if summary, err := s.loadSummaryLocked(id); err == nil {
				switch summary.Protocol {
				case ProtocolHTTP:
					stats.HTTPCount++
				case ProtocolWebSocket:
					stats.WebSocketCount++
				case ProtocolSSE:
					stats.SSECount++
				}

				// Track oldest/newest
				if oldestTime.IsZero() || summary.StartTime.Before(oldestTime) {
					oldestTime = summary.StartTime
					stats.OldestRecording = id
					stats.OldestDate = &oldestTime
				}
				if newestTime.IsZero() || summary.StartTime.After(newestTime) {
					newestTime = summary.StartTime
					stats.NewestRecording = id
					stats.NewestDate = &newestTime
				}
			}
		}

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	if stats.MaxBytes > 0 {
		stats.UsedPercent = float64(stats.UsedBytes) / float64(stats.MaxBytes) * 100
	}

	return stats, nil
}

// Vacuum permanently removes soft-deleted recordings.
func (s *FileStore) Vacuum() (int, int64, error) {
	entries, err := os.ReadDir(s.config.DataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, 0, nil
		}
		return 0, 0, fmt.Errorf("failed to read directory: %w", err)
	}

	var removed int
	var freedBytes int64

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "rec_") || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		id := strings.TrimPrefix(entry.Name(), "rec_")
		id = strings.TrimSuffix(id, ".json")

		recording, err := s.Get(id)
		if err != nil {
			continue
		}

		if recording.Deleted {
			info, _ := entry.Info()
			if info != nil {
				freedBytes += info.Size()
			}
			if err := s.Purge(id); err == nil {
				removed++
			}
		}
	}

	return removed, freedBytes, nil
}

// GetActiveSessions returns info about active recording sessions.
func (s *FileStore) GetActiveSessions() []*StreamRecordingSessionInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*StreamRecordingSessionInfo, 0, len(s.sessions))
	for id, session := range s.sessions {
		session.mu.Lock()
		info := &StreamRecordingSessionInfo{
			ID:         id,
			Protocol:   session.recording.Protocol,
			Path:       session.recording.Metadata.Path,
			StartTime:  session.startTime,
			Duration:   time.Since(session.startTime).String(),
			FrameCount: session.seq,
		}
		session.mu.Unlock()
		sessions = append(sessions, info)
	}

	return sessions
}

// GetActiveSessionForPath returns an active recording session ID for a given path and protocol.
// Returns empty string if no active session exists for that path.
func (s *FileStore) GetActiveSessionForPath(path string, protocol Protocol) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for id, session := range s.sessions {
		session.mu.Lock()
		matches := session.recording.Metadata.Path == path && session.recording.Protocol == protocol && !session.closed
		session.mu.Unlock()
		if matches {
			return id
		}
	}
	return ""
}

// StreamRecordingSessionInfo is the API view of an active session.
type StreamRecordingSessionInfo struct {
	ID         string    `json:"id"`
	Protocol   Protocol  `json:"protocol"`
	Path       string    `json:"path"`
	StartTime  time.Time `json:"startTime"`
	Duration   string    `json:"duration"`
	FrameCount int64     `json:"frameCount"`
}

// Export exports a recording to the specified format.
func (s *FileStore) Export(id string, format ExportFormat) ([]byte, error) {
	recording, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	switch format {
	case ExportFormatJSON:
		return json.MarshalIndent(recording, "", "  ")
	case ExportFormatYAML:
		// For now, just return JSON. YAML support can be added later.
		return json.MarshalIndent(recording, "", "  ")
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// CanRecord checks if there's enough storage for a new recording.
func (s *FileStore) CanRecord() (bool, string) {
	stats, err := s.GetStats()
	if err != nil {
		return false, "failed to check storage"
	}

	if stats.UsedBytes >= s.config.MaxBytes {
		return false, "storage limit exceeded"
	}

	if stats.UsedPercent >= float64(s.config.WarnPercent) {
		return true, fmt.Sprintf("warning: storage at %.0f%% capacity", stats.UsedPercent)
	}

	return true, ""
}

// Config returns the store configuration.
func (s *FileStore) Config() StorageConfig {
	return s.config
}
