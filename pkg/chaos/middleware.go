package chaos

import (
	"context"
	"net/http"
	"time"
)

// Middleware wraps an http.Handler with chaos injection
type Middleware struct {
	handler  http.Handler
	injector *Injector
}

// NewMiddleware creates a new chaos middleware
func NewMiddleware(handler http.Handler, injector *Injector) *Middleware {
	return &Middleware{
		handler:  handler,
		injector: injector,
	}
}

// ServeHTTP implements http.Handler with chaos injection
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Skip if chaos is disabled or injector is nil
	if m.injector == nil || !m.injector.IsEnabled() {
		m.handler.ServeHTTP(w, r)
		return
	}

	// Determine which faults to inject
	faults := m.injector.ShouldInject(r)
	if len(faults) == 0 {
		m.handler.ServeHTTP(w, r)
		return
	}

	// Process faults
	ctx := r.Context()
	responseWriter := w

	for _, fault := range faults {
		switch fault.Type {
		case FaultLatency:
			// Inject latency before processing
			if err := m.injector.InjectLatencyFromConfig(ctx, fault.Config); err != nil {
				// Context cancelled, stop processing
				return
			}

		case FaultError:
			// Return error immediately
			m.injector.InjectErrorFromConfig(w, fault.Config)
			return

		case FaultTimeout:
			// Simulate timeout
			duration := 30 * time.Second // Default timeout
			if v, ok := fault.Config["duration"].(string); ok {
				if d, err := time.ParseDuration(v); err == nil {
					duration = d
				}
			}
			if err := m.injector.InjectTimeout(ctx, duration); err != nil {
				return
			}
			// After timeout, return gateway timeout
			http.Error(w, "Gateway Timeout", http.StatusGatewayTimeout)
			return

		case FaultEmptyResponse:
			// Return empty response
			w.WriteHeader(http.StatusOK)
			return

		case FaultConnectionReset:
			// Simulate connection reset by hijacking and closing
			if hijacker, ok := w.(http.Hijacker); ok {
				conn, _, err := hijacker.Hijack()
				if err == nil {
					_ = conn.Close()
				}
			}
			return

		case FaultSlowBody:
			// Wrap response writer for slow delivery
			bytesPerSecond := 1024 // Default 1KB/s
			if v, ok := fault.Config["bytesPerSecond"].(int); ok {
				bytesPerSecond = v
			} else if v, ok := fault.Config["bytesPerSecond"].(float64); ok {
				bytesPerSecond = int(v)
			}
			responseWriter = &SlowWriter{
				w:              responseWriter,
				bytesPerSecond: bytesPerSecond,
			}

		case FaultCorruptBody:
			// Wrap response writer for corruption
			corruptRate := 0.01 // Default 1%
			if v, ok := fault.Config["corruptRate"].(float64); ok {
				corruptRate = v
			}
			responseWriter = m.injector.WrapForCorruption(responseWriter, corruptRate)

		case FaultPartialResponse:
			// Wrap response writer for truncation
			maxBytes := 1024 // Default 1KB
			if v, ok := fault.Config["maxBytes"].(int); ok {
				maxBytes = v
			} else if v, ok := fault.Config["maxBytes"].(float64); ok {
				maxBytes = int(v)
			}
			responseWriter = m.injector.WrapForTruncation(responseWriter, maxBytes)
		}
	}

	// Call the underlying handler with potentially wrapped writer
	m.handler.ServeHTTP(responseWriter, r)
}

// ContextKey is a type for context keys
type ContextKey string

const (
	// ChaosContextKey is the key for storing chaos info in context
	ChaosContextKey ContextKey = "chaos"
)

// ChaosContext holds chaos information for a request
type ChaosContext struct {
	Faults   []FaultConfig
	Injected bool
}

// WithChaosContext adds chaos context to the request
func WithChaosContext(ctx context.Context, chaosCtx *ChaosContext) context.Context {
	return context.WithValue(ctx, ChaosContextKey, chaosCtx)
}

// GetChaosContext retrieves chaos context from the request
func GetChaosContext(ctx context.Context) *ChaosContext {
	if v := ctx.Value(ChaosContextKey); v != nil {
		if cc, ok := v.(*ChaosContext); ok {
			return cc
		}
	}
	return nil
}
