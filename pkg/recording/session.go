// Package recording provides session management for grouped recordings.
package recording

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// FilterSnapshot captures the filter configuration at session start.
type FilterSnapshot struct {
	IncludePaths []string `json:"includePaths,omitempty"`
	ExcludePaths []string `json:"excludePaths,omitempty"`
	IncludeHosts []string `json:"includeHosts,omitempty"`
	ExcludeHosts []string `json:"excludeHosts,omitempty"`
}

// Session represents a collection of recordings from a time period.
type Session struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	StartTime time.Time       `json:"startTime"`
	EndTime   *time.Time      `json:"endTime,omitempty"`
	Filters   *FilterSnapshot `json:"filters,omitempty"`

	mu         sync.RWMutex
	recordings []*Recording
}

// NewSession creates a new recording session.
func NewSession(name string, filters *FilterSnapshot) *Session {
	return &Session{
		ID:         generateSessionID(),
		Name:       name,
		StartTime:  time.Now(),
		Filters:    filters,
		recordings: make([]*Recording, 0),
	}
}

// generateSessionID generates a unique session identifier.
func generateSessionID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// AddRecording adds a recording to the session.
func (s *Session) AddRecording(r *Recording) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r.SessionID = s.ID
	s.recordings = append(s.recordings, r)
}

// Recordings returns a copy of all recordings in the session.
func (s *Session) Recordings() []*Recording {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Recording, len(s.recordings))
	copy(result, s.recordings)
	return result
}

// RecordingCount returns the number of recordings in the session.
func (s *Session) RecordingCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.recordings)
}

// End marks the session as ended.
func (s *Session) End() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.EndTime = &now
}

// IsActive returns true if the session has not ended.
func (s *Session) IsActive() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.EndTime == nil
}

// GetRecording returns a recording by ID.
func (s *Session) GetRecording(id string) *Recording {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.recordings {
		if r.ID == id {
			return r
		}
	}
	return nil
}

// SessionSummary represents a summary of a recording session.
type SessionSummary struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	StartTime      time.Time  `json:"startTime"`
	EndTime        *time.Time `json:"endTime,omitempty"`
	RecordingCount int        `json:"recordingCount"`
}

// Summary returns a summary view of the session.
func (s *Session) Summary() SessionSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return SessionSummary{
		ID:             s.ID,
		Name:           s.Name,
		StartTime:      s.StartTime,
		EndTime:        s.EndTime,
		RecordingCount: len(s.recordings),
	}
}

// SessionExport is the JSON export format for a session.
type SessionExport struct {
	ID         string          `json:"id"`
	Name       string          `json:"name"`
	StartTime  time.Time       `json:"startTime"`
	EndTime    *time.Time      `json:"endTime,omitempty"`
	Filters    *FilterSnapshot `json:"filters,omitempty"`
	Recordings []*Recording    `json:"recordings"`
}

// Export returns an exportable representation of the session.
func (s *Session) Export() SessionExport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	recordings := make([]*Recording, len(s.recordings))
	copy(recordings, s.recordings)

	return SessionExport{
		ID:         s.ID,
		Name:       s.Name,
		StartTime:  s.StartTime,
		EndTime:    s.EndTime,
		Filters:    s.Filters,
		Recordings: recordings,
	}
}
