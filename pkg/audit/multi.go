package audit

import (
	"errors"
	"strings"
	"sync"
)

// MultiWriter writes audit entries to multiple outputs simultaneously.
// It implements the AuditLogger interface and fans out log entries
// to all configured writers.
type MultiWriter struct {
	writers []AuditLogger
	mu      sync.RWMutex
}

// NewMultiWriter creates a new MultiWriter that writes to all provided loggers.
// At least one writer must be provided.
func NewMultiWriter(writers ...AuditLogger) *MultiWriter {
	// Filter out nil writers
	validWriters := make([]AuditLogger, 0, len(writers))
	for _, w := range writers {
		if w != nil {
			validWriters = append(validWriters, w)
		}
	}

	return &MultiWriter{
		writers: validWriters,
	}
}

// Log writes an audit entry to all configured writers.
// Errors are collected and returned as a combined error.
// All writers receive the entry even if some fail.
func (m *MultiWriter) Log(entry AuditEntry) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var errs []error

	for _, w := range m.writers {
		if err := w.Log(entry); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return &MultiError{Errors: errs}
	}

	return nil
}

// Close closes all underlying writers.
// All writers are closed even if some fail to close.
func (m *MultiWriter) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errs []error

	for _, w := range m.writers {
		if err := w.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return &MultiError{Errors: errs}
	}

	return nil
}

// Add adds a writer to the MultiWriter.
// This is safe to call concurrently with Log.
func (m *MultiWriter) Add(w AuditLogger) {
	if w == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.writers = append(m.writers, w)
}

// Remove removes a writer from the MultiWriter.
// The writer is identified by pointer equality.
func (m *MultiWriter) Remove(w AuditLogger) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, writer := range m.writers {
		if writer == w {
			m.writers = append(m.writers[:i], m.writers[i+1:]...)
			return true
		}
	}
	return false
}

// Len returns the number of writers.
func (m *MultiWriter) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.writers)
}

// MultiError represents multiple errors from MultiWriter operations.
type MultiError struct {
	Errors []error
}

// Error returns a string representation of all errors.
func (e *MultiError) Error() string {
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	var b strings.Builder
	b.WriteString("multiple errors:")
	for _, err := range e.Errors {
		b.WriteString("\n  - ")
		b.WriteString(err.Error())
	}
	return b.String()
}

// Unwrap returns the underlying errors for use with errors.Is/As.
func (e *MultiError) Unwrap() []error {
	return e.Errors
}

// Is reports whether any error in the chain matches target.
func (e *MultiError) Is(target error) bool {
	for _, err := range e.Errors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// Ensure MultiWriter implements AuditLogger.
var _ AuditLogger = (*MultiWriter)(nil)
