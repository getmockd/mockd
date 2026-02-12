package template

import "sync"

// SequenceStore manages auto-incrementing named sequences for
// {{sequence("name")}} template expressions.
// It is thread-safe via an internal RWMutex.
type SequenceStore struct {
	sequences map[string]int64
	mu        sync.RWMutex
}

// NewSequenceStore creates a new sequence store.
func NewSequenceStore() *SequenceStore {
	return &SequenceStore{
		sequences: make(map[string]int64),
	}
}

// Next returns the current value of a sequence and then increments it.
// If the sequence doesn't exist yet, it starts at the given start value.
func (s *SequenceStore) Next(name string, start int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.sequences[name]; !exists {
		s.sequences[name] = start
	}
	val := s.sequences[name]
	s.sequences[name]++
	return val
}

// Reset removes a sequence, causing it to restart from its start value
// on the next call to Next.
func (s *SequenceStore) Reset(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sequences, name)
}

// Current returns the current value of a sequence without incrementing.
// Returns 0 if the sequence doesn't exist.
func (s *SequenceStore) Current(name string) int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sequences[name]
}
