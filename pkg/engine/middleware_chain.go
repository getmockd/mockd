// Package engine provides the core mock server engine.
package engine

import (
	"fmt"
	"net/http"

	"github.com/getmockd/mockd/pkg/audit"
	"github.com/getmockd/mockd/pkg/chaos"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/tracing"
	"github.com/getmockd/mockd/pkg/validation"
)

// MiddlewareChain manages the HTTP middleware stack for the mock server.
type MiddlewareChain struct {
	cfg           *config.ServerConfiguration
	chaosInjector *chaos.Injector
	validator     *validation.OpenAPIValidator
	auditLogger   audit.AuditLogger
	tracer        *tracing.Tracer
}

// MiddlewareChainOption configures a MiddlewareChain.
type MiddlewareChainOption func(*MiddlewareChain)

// WithChainTracer sets the tracer for the middleware chain.
// When set, tracing middleware will be applied to wrap all requests.
func WithChainTracer(t *tracing.Tracer) MiddlewareChainOption {
	return func(mc *MiddlewareChain) {
		mc.tracer = t
	}
}

// NewMiddlewareChain creates a new middleware chain from configuration.
// It initializes chaos, validation, and audit components if configured.
func NewMiddlewareChain(cfg *config.ServerConfiguration, opts ...MiddlewareChainOption) (*MiddlewareChain, error) {
	mc := &MiddlewareChain{cfg: cfg}

	// Apply options
	for _, opt := range opts {
		opt(mc)
	}

	// Initialize OpenAPI validator if configured
	if cfg.Validation != nil && cfg.Validation.Enabled {
		validator, err := validation.NewOpenAPIValidator(cfg.Validation)
		if err != nil {
			return nil, fmt.Errorf("failed to create OpenAPI validator: %w", err)
		}
		mc.validator = validator
	}

	// Initialize chaos injector if configured
	if cfg.Chaos != nil && cfg.Chaos.Enabled {
		injector, err := chaos.NewInjector(cfg.Chaos)
		if err != nil {
			return nil, fmt.Errorf("failed to create chaos injector: %w", err)
		}
		mc.chaosInjector = injector
	}

	// Initialize audit logger if configured
	if cfg.Audit != nil && cfg.Audit.Enabled {
		auditLogger, err := audit.NewLogger(cfg.Audit)
		if err != nil {
			return nil, fmt.Errorf("failed to create audit logger: %w", err)
		}
		mc.auditLogger = auditLogger
	}

	return mc, nil
}

// Wrap wraps the given handler with all configured middleware.
// The order is: tracing -> metrics -> audit -> dynamic chaos -> validation -> handler
func (mc *MiddlewareChain) Wrap(handler http.Handler) http.Handler {
	h := handler

	// Validation middleware (innermost, closest to handler)
	if mc.validator != nil {
		h = validation.NewMiddleware(h, mc.validator, mc.cfg.Validation)
	}

	// Dynamic chaos middleware (always wrap to support runtime configuration)
	h = &dynamicChaosHandler{chain: mc, handler: h}

	// Audit middleware
	if mc.auditLogger != nil {
		h = audit.NewMiddleware(h, mc.auditLogger, mc.cfg.Audit)
	}

	// Metrics middleware (measures total request time)
	h = MetricsMiddleware(h)

	// Tracing middleware (outermost, captures full request lifecycle)
	if mc.tracer != nil {
		h = TracingMiddleware(mc.tracer)(h)
	}

	return h
}

// Tracer returns the tracer, if configured.
func (mc *MiddlewareChain) Tracer() *tracing.Tracer {
	return mc.tracer
}

// SetTracer sets the tracer for the middleware chain.
func (mc *MiddlewareChain) SetTracer(t *tracing.Tracer) {
	mc.tracer = t
}

// ChaosInjector returns the chaos injector.
func (mc *MiddlewareChain) ChaosInjector() *chaos.Injector {
	return mc.chaosInjector
}

// SetChaosInjector sets the chaos injector for dynamic chaos injection.
func (mc *MiddlewareChain) SetChaosInjector(injector *chaos.Injector) {
	mc.chaosInjector = injector
}

// ChaosEnabled returns whether chaos injection is enabled.
func (mc *MiddlewareChain) ChaosEnabled() bool {
	return mc.chaosInjector != nil && mc.chaosInjector.IsEnabled()
}

// Validator returns the OpenAPI validator.
func (mc *MiddlewareChain) Validator() *validation.OpenAPIValidator {
	return mc.validator
}

// ValidationEnabled returns whether OpenAPI validation is enabled.
func (mc *MiddlewareChain) ValidationEnabled() bool {
	return mc.validator != nil && mc.validator.IsEnabled()
}

// AuditLogger returns the audit logger.
func (mc *MiddlewareChain) AuditLogger() audit.AuditLogger {
	return mc.auditLogger
}

// Close closes the middleware chain and releases resources.
func (mc *MiddlewareChain) Close() error {
	if mc.auditLogger != nil {
		return mc.auditLogger.Close()
	}
	return nil
}

// dynamicChaosHandler wraps a handler and applies chaos injection dynamically.
type dynamicChaosHandler struct {
	chain   *MiddlewareChain
	handler http.Handler
}

func (h *dynamicChaosHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.chain.chaosInjector != nil && h.chain.chaosInjector.IsEnabled() {
		chaosMiddleware := chaos.NewMiddleware(h.handler, h.chain.chaosInjector)
		chaosMiddleware.ServeHTTP(w, r)
		return
	}
	h.handler.ServeHTTP(w, r)
}
