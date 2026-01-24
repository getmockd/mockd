package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
)

// AuditLogger defines the interface for audit logging implementations.
type AuditLogger interface {
	// Log records an audit entry. Implementations must be thread-safe.
	Log(entry AuditEntry) error

	// Close releases any resources held by the logger.
	Close() error
}

// NoOpLogger is an AuditLogger that discards all entries.
// Use this when audit logging is disabled.
type NoOpLogger struct{}

// Log discards the entry. Always returns nil.
func (l *NoOpLogger) Log(_ AuditEntry) error {
	return nil
}

// Close is a no-op. Always returns nil.
func (l *NoOpLogger) Close() error {
	return nil
}

// Ensure NoOpLogger implements AuditLogger.
var _ AuditLogger = (*NoOpLogger)(nil)

// FileLogger writes audit entries as JSON lines to a file.
type FileLogger struct {
	file     *os.File
	encoder  *json.Encoder
	sequence int64
	mu       sync.Mutex
}

// NewFileLogger creates a new FileLogger that writes to the specified path.
// The file is created if it doesn't exist, or appended to if it does.
func NewFileLogger(path string) (*FileLogger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("audit: failed to open log file: %w", err)
	}

	return &FileLogger{
		file:    file,
		encoder: json.NewEncoder(file),
	}, nil
}

// Log writes an audit entry to the file as a JSON line.
// The entry's Sequence field is set automatically.
func (l *FileLogger) Log(entry AuditEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return fmt.Errorf("audit: logger is closed")
	}

	// Set the sequence number atomically
	entry.Sequence = atomic.AddInt64(&l.sequence, 1)

	if err := l.encoder.Encode(entry); err != nil {
		return fmt.Errorf("audit: failed to encode entry: %w", err)
	}

	return nil
}

// Close flushes and closes the underlying file.
func (l *FileLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}

	// Sync to ensure all data is written
	if err := l.file.Sync(); err != nil {
		// Log but don't fail - we still want to close
		_ = err
	}

	err := l.file.Close()
	l.file = nil
	return err
}

// Ensure FileLogger implements AuditLogger.
var _ AuditLogger = (*FileLogger)(nil)

// StdoutLogger writes audit entries as JSON lines to stdout.
// Useful for containerized deployments where logs are collected from stdout.
type StdoutLogger struct {
	encoder  *json.Encoder
	sequence int64
	mu       sync.Mutex
}

// NewStdoutLogger creates a new StdoutLogger.
func NewStdoutLogger() *StdoutLogger {
	return &StdoutLogger{
		encoder: json.NewEncoder(os.Stdout),
	}
}

// Log writes an audit entry to stdout as a JSON line.
func (l *StdoutLogger) Log(entry AuditEntry) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry.Sequence = atomic.AddInt64(&l.sequence, 1)

	if err := l.encoder.Encode(entry); err != nil {
		return fmt.Errorf("audit: failed to encode entry: %w", err)
	}

	return nil
}

// Close is a no-op for stdout logger.
func (l *StdoutLogger) Close() error {
	return nil
}

// Ensure StdoutLogger implements AuditLogger.
var _ AuditLogger = (*StdoutLogger)(nil)

// NewLogger creates an appropriate AuditLogger based on the configuration.
// Returns a NoOpLogger if audit logging is disabled.
// If multiple outputs are configured (file/stdout and registered writers),
// returns a MultiWriter that writes to all of them.
func NewLogger(config *AuditConfig) (AuditLogger, error) {
	if config == nil || !config.Enabled {
		return &NoOpLogger{}, nil
	}

	if err := config.Validate(); err != nil {
		return nil, err
	}

	var writers []AuditLogger

	// Add primary output (file or stdout)
	if config.OutputFile != "" {
		fileLogger, err := NewFileLogger(config.OutputFile)
		if err != nil {
			return nil, err
		}
		writers = append(writers, fileLogger)
	} else {
		writers = append(writers, NewStdoutLogger())
	}

	// Add any registered writers from extensions
	if config.Extensions != nil {
		for name, extConfig := range config.Extensions {
			factory, ok := GetRegisteredWriter(name)
			if !ok {
				continue
			}
			// Convert extension config to map[string]interface{} if possible
			extMap, ok := extConfig.(map[string]interface{})
			if !ok {
				continue
			}
			writer, err := factory(extMap)
			if err != nil {
				// Close already created writers on error
				for _, w := range writers {
					_ = w.Close()
				}
				return nil, err
			}
			writers = append(writers, writer)
		}
	}

	// If only one writer, return it directly
	if len(writers) == 1 {
		return writers[0], nil
	}

	// Multiple writers, use MultiWriter
	return NewMultiWriter(writers...), nil
}
