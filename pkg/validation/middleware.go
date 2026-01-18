package validation

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
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

	// Store original body for re-reading
	var bodyBytes []byte
	if r.Body != nil && r.Body != http.NoBody {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			m.handleError(w, &ValidationResult{
				Valid: false,
				Errors: []ValidationError{{
					Type:    "body",
					Message: "failed to read request body",
				}},
			})
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// Validate request
	if m.config.ValidateRequest {
		result := m.validator.ValidateRequest(r)

		// Restore body for handler
		if len(bodyBytes) > 0 {
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}

		if !result.Valid {
			if m.config.LogWarnings {
				for _, err := range result.Errors {
					log.Printf("[validation] request error: type=%s field=%s message=%s location=%s",
						err.Type, err.Field, err.Message, err.Location)
				}
			}
			if m.config.FailOnError {
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
			log.Printf("[validation] response error: type=%s field=%s message=%s location=%s",
				err.Type, err.Field, err.Message, err.Location)
		}
	}

	// Note: Response is already written, so we can only log errors
	// We can't change the response at this point
}

// handleError writes a validation error response
func (m *Middleware) handleError(w http.ResponseWriter, result *ValidationResult) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	response := ValidationErrorResponse{
		Error:   "Request validation failed",
		Details: result.Errors,
	}

	json.NewEncoder(w).Encode(response)
}

// ValidationErrorResponse is the error response format
type ValidationErrorResponse struct {
	Error   string            `json:"error"`
	Details []ValidationError `json:"details,omitempty"`
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
func ValidateRequestOnly(validator *OpenAPIValidator, r *http.Request) (*ValidationResult, error) {
	if validator == nil || !validator.IsEnabled() {
		return &ValidationResult{Valid: true}, nil
	}

	return validator.ValidateRequest(r), nil
}

// MustValidateRequest returns an error if request validation fails
// Useful for tests and handlers that need to validate requests manually
func MustValidateRequest(validator *OpenAPIValidator, r *http.Request) error {
	result := validator.ValidateRequest(r)
	if !result.Valid {
		var messages []string
		for _, err := range result.Errors {
			messages = append(messages, err.Message)
		}
		return &RequestValidationError{Message: "request validation failed: " + joinStrings(messages, "; ")}
	}
	return nil
}

// RequestValidationError represents a validation error
type RequestValidationError struct {
	Message string
}

func (e *RequestValidationError) Error() string {
	return e.Message
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}
