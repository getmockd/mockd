package validation

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"strconv"
)

// Middleware wraps an http.Handler with request validation
type Middleware struct {
	handler   http.Handler
	validator *OpenAPIValidator
	config    *ValidationConfig
}

// responseRecorder captures the response for validation
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
	headers    http.Header
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{
		ResponseWriter: w,
		statusCode:     http.StatusOK,
		body:           &bytes.Buffer{},
		headers:        make(http.Header),
	}
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseRecorder) Header() http.Header {
	return r.ResponseWriter.Header()
}

// NewMiddleware creates a new validation middleware
func NewMiddleware(handler http.Handler, validator *OpenAPIValidator, config *ValidationConfig) *Middleware {
	if config == nil {
		config = DefaultValidationConfig()
	}
	return &Middleware{
		handler:   handler,
		validator: validator,
		config:    config,
	}
}

// ServeHTTP implements http.Handler
func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Skip validation if disabled or no validator
	if m.validator == nil || !m.validator.IsEnabled() {
		m.handler.ServeHTTP(w, r)
		return
	}

	// In permissive mode, skip validation entirely and pass through
	if m.config.GetMode() == ModePermissive {
		m.handler.ServeHTTP(w, r)
		return
	}

	// Store original body for re-reading
	var bodyBytes []byte
	if r.Body != nil && r.Body != http.NoBody {
		var err error
		const maxValidationBodySize = 10 << 20 // 10MB defense-in-depth
		bodyBytes, err = io.ReadAll(io.LimitReader(r.Body, maxValidationBodySize))
		if err != nil {
			result := &Result{Valid: false}
			result.AddError(&FieldError{
				Location: LocationBody,
				Code:     "read_error",
				Message:  "failed to read request body",
			})
			m.handleError(w, result)
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// Validate request
	if m.config.ValidateRequest {
		result := m.validator.ValidateRequest(r, bodyBytes)

		// Restore body for handler
		if len(bodyBytes) > 0 {
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		if !result.Valid {
			if m.config.LogWarnings {
				for _, err := range result.Errors {
					slog.Warn("validation: request error",
						"code", err.Code, "field", err.Field,
						"message", err.Message, "location", err.Location)
				}
			}

			// In warn mode, add warning headers but allow request through
			if m.config.GetMode() == ModeWarn {
				w.Header().Set("X-Mockd-Validation-Warnings", "true")
				w.Header().Set("X-Mockd-Validation-Errors", strconv.Itoa(len(result.Errors)))
				// Fall through to handler — do not return
			} else if m.config.FailOnError {
				m.handleError(w, result)
				return
			}
		}
	}

	// If response validation is not enabled, just call handler
	if !m.config.ValidateResponse {
		m.handler.ServeHTTP(w, r)
		return
	}

	// Record response for validation
	recorder := newResponseRecorder(w)
	m.handler.ServeHTTP(recorder, r)

	// Validate response
	result := m.validator.ValidateResponse(r, recorder.statusCode, recorder.Header(), recorder.body.Bytes())
	if !result.Valid && m.config.LogWarnings {
		for _, err := range result.Errors {
			slog.Warn("validation: response error",
				"code", err.Code, "field", err.Field,
				"message", err.Message, "location", err.Location)
		}
	}

	// Note: Response is already written, so we can only log errors
	// We can't change the response at this point
}

// handleError writes a validation error response using RFC 7807 Problem Details.
func (m *Middleware) handleError(w http.ResponseWriter, result *Result) {
	resp := NewErrorResponse(result, http.StatusBadRequest)
	resp.WriteResponse(w)
}

// MiddlewareFunc is an adapter to use Middleware as middleware function
func (m *Middleware) MiddlewareFunc(next http.Handler) http.Handler {
	m.handler = next
	return m
}

// NewMiddlewareFunc creates a middleware function for use with routers
func NewMiddlewareFunc(validator *OpenAPIValidator, config *ValidationConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return NewMiddleware(next, validator, config)
	}
}

// WithValidation is a convenience wrapper that creates and applies validation middleware
func WithValidation(handler http.Handler, config *ValidationConfig) (http.Handler, error) {
	if config == nil || !config.Enabled {
		return handler, nil
	}

	validator, err := NewOpenAPIValidator(config)
	if err != nil {
		return nil, err
	}

	return NewMiddleware(handler, validator, config), nil
}

// ValidateRequestOnly validates a request without wrapping a handler
// Useful for manual validation in handlers
func ValidateRequestOnly(validator *OpenAPIValidator, r *http.Request) (*Result, error) {
	if validator == nil || !validator.IsEnabled() {
		return &Result{Valid: true}, nil
	}

	return validator.ValidateRequest(r, nil), nil
}
