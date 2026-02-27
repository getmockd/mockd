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

// faultResult indicates how the middleware should proceed after processing a fault.
type faultResult int

const (
	faultContinue faultResult = iota // Continue processing faults / call handler
	faultAbort                       // Fault wrote the response; stop processing
)

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
		var result faultResult
		result, responseWriter = m.processFault(fault, ctx, w, responseWriter)
		if result == faultAbort {
			return
		}
	}

	// Call the underlying handler with potentially wrapped writer
	m.handler.ServeHTTP(responseWriter, r)
}

// processFault handles a single fault. Returns the action to take and the
// (possibly wrapped) response writer for composable faults.
func (m *Middleware) processFault(
	fault FaultConfig,
	ctx context.Context,
	origWriter http.ResponseWriter,
	responseWriter http.ResponseWriter,
) (faultResult, http.ResponseWriter) {
	switch fault.Type {
	case FaultLatency:
		if err := m.injector.InjectLatencyFromConfig(ctx, fault.Config); err != nil {
			return faultAbort, responseWriter
		}

	case FaultError:
		m.injector.InjectErrorFromConfig(origWriter, fault.Config)
		return faultAbort, responseWriter

	case FaultTimeout:
		duration := 30 * time.Second
		if v, ok := fault.Config["duration"].(string); ok {
			if d, err := time.ParseDuration(v); err == nil {
				duration = d
			}
		}
		if err := m.injector.InjectTimeout(ctx, duration); err != nil {
			return faultAbort, responseWriter
		}
		http.Error(origWriter, "Gateway Timeout", http.StatusGatewayTimeout)
		return faultAbort, responseWriter

	case FaultEmptyResponse:
		origWriter.WriteHeader(http.StatusOK)
		return faultAbort, responseWriter

	case FaultConnectionReset:
		if hijacker, ok := origWriter.(http.Hijacker); ok {
			conn, _, err := hijacker.Hijack()
			if err == nil {
				_ = conn.Close()
			}
		}
		return faultAbort, responseWriter

	case FaultSlowBody:
		responseWriter = wrapSlowBody(fault.Config, responseWriter)

	case FaultCorruptBody:
		corruptRate := 0.01
		if v, ok := fault.Config["corruptRate"].(float64); ok {
			corruptRate = v
		}
		responseWriter = m.injector.WrapForCorruption(responseWriter, corruptRate)

	case FaultPartialResponse:
		responseWriter = wrapPartialResponse(fault.Config, responseWriter)

	// --- Stateful Fault Types ---

	case FaultCircuitBreaker:
		key, _ := fault.Config["_stateKey"].(string)
		if m.injector.HandleCircuitBreaker(key, origWriter) {
			return faultAbort, responseWriter
		}

	case FaultRetryAfter:
		key, _ := fault.Config["_stateKey"].(string)
		if m.injector.HandleRetryAfter(key, origWriter) {
			return faultAbort, responseWriter
		}

	case FaultProgressiveDegradation:
		key, _ := fault.Config["_stateKey"].(string)
		if m.injector.HandleProgressiveDegradation(key, ctx, origWriter) {
			return faultAbort, responseWriter
		}

	case FaultChunkedDribble:
		responseWriter = wrapChunkedDribble(fault.Config, responseWriter)
	}

	return faultContinue, responseWriter
}

// wrapSlowBody wraps a response writer for bandwidth-limited delivery.
func wrapSlowBody(cfg map[string]interface{}, w http.ResponseWriter) http.ResponseWriter {
	bytesPerSecond := 1024
	if v, ok := cfg["bytesPerSecond"].(int); ok {
		bytesPerSecond = v
	} else if v, ok := cfg["bytesPerSecond"].(float64); ok {
		bytesPerSecond = int(v)
	}
	return &SlowWriter{w: w, bytesPerSecond: bytesPerSecond}
}

// wrapPartialResponse wraps a response writer for truncation.
func wrapPartialResponse(cfg map[string]interface{}, w http.ResponseWriter) http.ResponseWriter {
	maxBytes := 1024
	if v, ok := cfg["maxBytes"].(int); ok {
		maxBytes = v
	} else if v, ok := cfg["maxBytes"].(float64); ok {
		maxBytes = int(v)
	}
	return &TruncatingWriter{w: w, maxBytes: maxBytes}
}

// wrapChunkedDribble wraps a response writer for chunked timed delivery.
func wrapChunkedDribble(cfg map[string]interface{}, w http.ResponseWriter) http.ResponseWriter {
	chunkSize := getIntOrDefault(cfg, "chunkSize", 1024)
	chunkDelayStr := getStringOrDefault(cfg, "chunkDelay", "500ms")
	chunkDelay, err := time.ParseDuration(chunkDelayStr)
	if err != nil {
		chunkDelay = 500 * time.Millisecond
	}
	initialDelayStr := getStringOrDefault(cfg, "initialDelay", "0ms")
	initialDelay, err := time.ParseDuration(initialDelayStr)
	if err != nil {
		initialDelay = 0
	}
	return NewChunkedDribbleWriter(w, chunkSize, chunkDelay, initialDelay)
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
