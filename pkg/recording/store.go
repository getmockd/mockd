// Package recording provides in-memory storage for recordings and sessions.
package recording

import (
	"encoding/json"
	"errors"
	"sync"

	"github.com/getmockd/mockd/pkg/store"
)

// ErrNotFound is an alias for store.ErrNotFound so that errors.Is works
// consistently across packages.
var ErrNotFound = store.ErrNotFound

// Store provides in-memory storage for recordings and sessions.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	active   *Session
}

// NewStore creates a new recording store.
func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
	}
}

// CreateSession creates a new session and makes it active.
func (s *Store) CreateSession(name string, filters interface{}) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	// End any active session
	if s.active != nil {
		s.active.End()
	}

	session := NewSession(name, nil) // TODO: Pass actual filter config
	s.sessions[session.ID] = session
	s.active = session

	return session
}

// ActiveSession returns the currently active session.
func (s *Store) ActiveSession() *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.active
}

// GetSession returns a session by ID.
func (s *Store) GetSession(id string) *Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil
	}
	return session
}

// ListSessions returns all sessions.
func (s *Store) ListSessions() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// DeleteSession deletes a session by ID.
func (s *Store) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.sessions[id]; !ok {
		return ErrNotFound
	}

	delete(s.sessions, id)

	if s.active != nil && s.active.ID == id {
		s.active = nil
	}

	return nil
}

// AddRecording adds a recording to the active session.
func (s *Store) AddRecording(r *Recording) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.active == nil {
		return errors.New("no active session")
	}

	s.active.AddRecording(r)
	return nil
}

// GetRecording returns a recording by ID from any session.
func (s *Store) GetRecording(id string) *Recording {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, session := range s.sessions {
		if r := session.GetRecording(id); r != nil {
			return r
		}
	}
	return nil
}

// RecordingFilter specifies criteria for filtering recordings.
type RecordingFilter struct {
	SessionID string
	Method    string
	Path      string
	Limit     int
	Offset    int
}

// ListRecordings returns recordings matching the filter.
func (s *Store) ListRecordings(filter RecordingFilter) ([]*Recording, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var all []*Recording

	for _, session := range s.sessions {
		if filter.SessionID != "" && session.ID != filter.SessionID {
			continue
		}
		for _, r := range session.Recordings() {
			if filter.Method != "" && r.Request.Method != filter.Method {
				continue
			}
			if filter.Path != "" && r.Request.Path != filter.Path {
				continue
			}
			all = append(all, r)
		}
	}

	total := len(all)

	// Apply pagination
	if filter.Offset > 0 {
		if filter.Offset >= len(all) {
			return []*Recording{}, total
		}
		all = all[filter.Offset:]
	}
	if filter.Limit > 0 && len(all) > filter.Limit {
		all = all[:filter.Limit]
	}

	return all, total
}

// DeleteRecording deletes a recording by ID.
func (s *Store) DeleteRecording(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, session := range s.sessions {
		session.mu.Lock()
		for i, r := range session.recordings {
			if r.ID == id {
				session.recordings = append(session.recordings[:i], session.recordings[i+1:]...)
				session.mu.Unlock()
				return nil
			}
		}
		session.mu.Unlock()
	}
	return ErrNotFound
}

// Clear removes all recordings from all sessions.
func (s *Store) Clear() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, session := range s.sessions {
		count += session.RecordingCount()
	}

	s.sessions = make(map[string]*Session)
	s.active = nil

	return count
}

// ExportSession exports a session as JSON.
func (s *Store) ExportSession(id string) ([]byte, error) {
	session := s.GetSession(id)
	if session == nil {
		return nil, ErrNotFound
	}

	return json.MarshalIndent(session.Export(), "", "  ")
}

// ExportRecordings exports recordings as JSON.
func (s *Store) ExportRecordings(filter RecordingFilter) ([]byte, error) {
	recordings, _ := s.ListRecordings(filter)
	return json.MarshalIndent(recordings, "", "  ")
}
