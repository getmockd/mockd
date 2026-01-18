package protocol

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/requestlog"
)

// Registry manages protocol handlers and provides lifecycle management.
// It is thread-safe and can be used concurrently.
type Registry struct {
	handlers map[string]Handler
	mu       sync.RWMutex
}

// NewRegistry creates a new empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		handlers: make(map[string]Handler),
	}
}

// Register adds a handler to the registry.
// Returns an error if a handler with the same ID already exists.
func (r *Registry) Register(h Handler) error {
	if h == nil {
		return ErrNilHandler
	}

	meta := h.Metadata()
	if meta.ID == "" {
		return ErrEmptyHandlerID
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[meta.ID]; exists {
		return fmt.Errorf("%w: %s", ErrHandlerExists, meta.ID)
	}

	r.handlers[meta.ID] = h
	return nil
}

// Unregister removes a handler from the registry.
// Returns an error if the handler is not found.
func (r *Registry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.handlers[id]; !exists {
		return fmt.Errorf("%w: %s", ErrHandlerNotFound, id)
	}

	delete(r.handlers, id)
	return nil
}

// Get returns a handler by ID.
// Returns the handler and true if found, nil and false otherwise.
func (r *Registry) Get(id string) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	h, exists := r.handlers[id]
	return h, exists
}

// List returns all registered handlers.
func (r *Registry) List() []Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	handlers := make([]Handler, 0, len(r.handlers))
	for _, h := range r.handlers {
		handlers = append(handlers, h)
	}
	return handlers
}

// ListByProtocol returns all handlers of a specific protocol type.
func (r *Registry) ListByProtocol(proto Protocol) []Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var handlers []Handler
	for _, h := range r.handlers {
		if h.Metadata().Protocol == proto {
			handlers = append(handlers, h)
		}
	}
	return handlers
}

// ListByCapability returns all handlers that have a specific capability.
func (r *Registry) ListByCapability(cap Capability) []Handler {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var handlers []Handler
	for _, h := range r.handlers {
		if h.Metadata().HasCapability(cap) {
			handlers = append(handlers, h)
		}
	}
	return handlers
}

// Count returns the number of registered handlers.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers)
}

// StartAll starts all registered handlers.
// Returns an error if any handler fails to start.
// Handlers are started in no particular order.
func (r *Registry) StartAll(ctx context.Context) error {
	r.mu.RLock()
	handlers := make([]Handler, 0, len(r.handlers))
	for _, h := range r.handlers {
		handlers = append(handlers, h)
	}
	r.mu.RUnlock()

	for _, h := range handlers {
		if err := h.Start(ctx); err != nil {
			return fmt.Errorf("failed to start handler %s: %w", h.Metadata().ID, err)
		}
	}
	return nil
}

// StopAll stops all registered handlers.
// Returns an error if any handler fails to stop cleanly.
// Handlers are stopped in no particular order.
func (r *Registry) StopAll(ctx context.Context, timeout time.Duration) error {
	r.mu.RLock()
	handlers := make([]Handler, 0, len(r.handlers))
	for _, h := range r.handlers {
		handlers = append(handlers, h)
	}
	r.mu.RUnlock()

	var errs []error
	for _, h := range handlers {
		if err := h.Stop(ctx, timeout); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop handler %s: %w", h.Metadata().ID, err))
		}
	}

	if len(errs) > 0 {
		return errs[0] // Return first error for simplicity
	}
	return nil
}

// HealthAll returns the health status of all registered handlers.
// Returns a map of handler ID to health status.
func (r *Registry) HealthAll(ctx context.Context) map[string]HealthStatus {
	r.mu.RLock()
	handlers := make(map[string]Handler, len(r.handlers))
	for id, h := range r.handlers {
		handlers[id] = h
	}
	r.mu.RUnlock()

	results := make(map[string]HealthStatus, len(handlers))
	for id, h := range handlers {
		results[id] = h.Health(ctx)
	}
	return results
}

// EnableRecordingAll enables recording on all handlers that support it.
func (r *Registry) EnableRecordingAll() {
	r.mu.RLock()
	handlers := make([]Handler, 0, len(r.handlers))
	for _, h := range r.handlers {
		handlers = append(handlers, h)
	}
	r.mu.RUnlock()

	for _, h := range handlers {
		if rec, ok := h.(Recordable); ok {
			rec.EnableRecording()
		}
	}
}

// DisableRecordingAll disables recording on all handlers that support it.
func (r *Registry) DisableRecordingAll() {
	r.mu.RLock()
	handlers := make([]Handler, 0, len(r.handlers))
	for _, h := range r.handlers {
		handlers = append(handlers, h)
	}
	r.mu.RUnlock()

	for _, h := range handlers {
		if rec, ok := h.(Recordable); ok {
			rec.DisableRecording()
		}
	}
}

// ForEach executes a function for each handler.
// Return false from the function to stop iteration.
func (r *Registry) ForEach(fn func(Handler) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, h := range r.handlers {
		if !fn(h) {
			break
		}
	}
}

// SetRequestLoggerAll sets the request logger on all RequestLoggable handlers.
func (r *Registry) SetRequestLoggerAll(logger requestlog.Logger) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, h := range r.handlers {
		if rl, ok := h.(RequestLoggable); ok {
			rl.SetRequestLogger(logger)
		}
	}
}
