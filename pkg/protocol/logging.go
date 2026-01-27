package protocol

import (
	"log/slog"
	"time"

	"github.com/getmockd/mockd/pkg/requestlog"
)

// Loggable handlers can receive a structured logger.
// This is for internal operational logging (debug, errors, etc.),
// not for user-visible request logging.
//
// Example implementation:
//
//	type MyHandler struct {
//	    log *slog.Logger
//	}
//
//	func (h *MyHandler) SetLogger(log *slog.Logger) {
//	    h.log = log
//	}
type Loggable interface {
	// SetLogger sets the structured logger for the handler.
	// Handlers should use this logger for operational logging.
	SetLogger(log *slog.Logger)
}

// RequestLoggable handlers support request logging for user inspection.
// This captures request/response data for display in the Admin API.
//
// Implementations MUST be thread-safe. The engine may call SetRequestLogger
// at any time, and GetRequestLogger may be called concurrently from
// multiple request handlers.
//
// Example implementation:
//
//	type MyHandler struct {
//	    requestLogger   requestlog.Logger
//	    requestLoggerMu sync.RWMutex
//	}
//
//	func (h *MyHandler) SetRequestLogger(logger requestlog.Logger) {
//	    h.requestLoggerMu.Lock()
//	    defer h.requestLoggerMu.Unlock()
//	    h.requestLogger = logger
//	}
//
//	func (h *MyHandler) GetRequestLogger() requestlog.Logger {
//	    h.requestLoggerMu.RLock()
//	    defer h.requestLoggerMu.RUnlock()
//	    return h.requestLogger
//	}
type RequestLoggable interface {
	// SetRequestLogger sets the request logger for user-visible logging.
	// Must be thread-safe.
	SetRequestLogger(logger requestlog.Logger)

	// GetRequestLogger returns the current request logger.
	// Returns nil if no logger is set.
	// Must be thread-safe.
	GetRequestLogger() requestlog.Logger
}

// Observable handlers expose operational metrics.
// These metrics are used by the Admin API stats endpoints and
// can be displayed in the Admin API.
//
// Example implementation:
//
//	func (h *MyHandler) Stats() protocol.Stats {
//	    return protocol.Stats{
//	        Running:       h.isRunning,
//	        StartedAt:     h.startedAt,
//	        Uptime:        time.Since(h.startedAt),
//	        RequestCount:  atomic.LoadInt64(&h.requestCount),
//	        ErrorCount:    atomic.LoadInt64(&h.errorCount),
//	        BytesReceived: atomic.LoadInt64(&h.bytesReceived),
//	        BytesSent:     atomic.LoadInt64(&h.bytesSent),
//	    }
//	}
type Observable interface {
	// Stats returns operational metrics for the handler.
	Stats() Stats
}

// Stats holds common operational metrics.
// Protocol handlers can extend this with custom metrics in the Custom map.
type Stats struct {
	// Running indicates whether the handler is currently active.
	Running bool `json:"running"`

	// StartedAt is when the handler was started.
	// Zero value if handler has never been started.
	StartedAt time.Time `json:"startedAt,omitempty"`

	// Uptime is the duration since the handler was started.
	// Zero if not running.
	Uptime time.Duration `json:"uptime,omitempty"`

	// RequestCount is the total number of requests/messages handled.
	RequestCount int64 `json:"requestCount,omitempty"`

	// ErrorCount is the total number of errors encountered.
	ErrorCount int64 `json:"errorCount,omitempty"`

	// BytesReceived is the total bytes received from clients.
	BytesReceived int64 `json:"bytesReceived,omitempty"`

	// BytesSent is the total bytes sent to clients.
	BytesSent int64 `json:"bytesSent,omitempty"`

	// Custom holds protocol-specific metrics.
	// Examples: connection count, topic count, queue depth, etc.
	Custom map[string]any `json:"custom,omitempty"`
}
