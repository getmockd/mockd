package validation

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testOpenAPISpec = `
openapi: "3.0.3"
info:
  title: Test API
  version: "1.0.0"
servers:
  - url: http://localhost:8080
paths:
  /users:
    get:
      summary: List users
      parameters:
        - name: limit
          in: query
          required: false
          schema:
            type: integer
            minimum: 1
            maximum: 100
        - name: offset
          in: query
          required: false
          schema:
            type: integer
            minimum: 0
      responses:
        "200":
          description: List of users
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: "#/components/schemas/User"
    post:
      summary: Create user
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/CreateUserRequest"
      responses:
        "201":
          description: User created
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
        "400":
          description: Invalid request
  /users/{id}:
    get:
      summary: Get user by ID
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: integer
        - name: X-Request-ID
          in: header
          required: false
          schema:
            type: string
            format: uuid
      responses:
        "200":
          description: User details
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
        "404":
          description: User not found
    put:
      summary: Update user
      parameters:
        - name: id
          in: path
          required: true
          schema:
            type: integer
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/UpdateUserRequest"
      responses:
        "200":
          description: User updated
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/User"
components:
  schemas:
    User:
      type: object
      required:
        - id
        - name
        - email
      properties:
        id:
          type: integer
        name:
          type: string
          minLength: 1
          maxLength: 100
        email:
          type: string
          format: email
        age:
          type: integer
          minimum: 0
          maximum: 150
    CreateUserRequest:
      type: object
      required:
        - name
        - email
      properties:
        name:
          type: string
          minLength: 1
          maxLength: 100
        email:
          type: string
          format: email
        age:
          type: integer
          minimum: 0
          maximum: 150
    UpdateUserRequest:
      type: object
      properties:
        name:
          type: string
          minLength: 1
          maxLength: 100
        email:
          type: string
          format: email
        age:
          type: integer
          minimum: 0
          maximum: 150
`

func TestNewOpenAPIValidator(t *testing.T) {
	tests := []struct {
		name        string
		config      *ValidationConfig
		expectError bool
	}{
		{
			name:        "nil config",
			config:      nil,
			expectError: true,
		},
		{
			name: "disabled validator",
			config: &ValidationConfig{
				Enabled: false,
			},
			expectError: false,
		},
		{
			name: "enabled but no spec source",
			config: &ValidationConfig{
				Enabled: true,
			},
			expectError: true,
		},
		{
			name: "valid inline spec",
			config: &ValidationConfig{
				Enabled:         true,
				Spec:            testOpenAPISpec,
				ValidateRequest: true,
			},
			expectError: false,
		},
		{
			name: "invalid spec",
			config: &ValidationConfig{
				Enabled: true,
				Spec:    "invalid: yaml: content",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOpenAPIValidator(tt.config)
			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateRequest_PathParams(t *testing.T) {
	validator := mustCreateValidator(t)

	tests := []struct {
		name        string
		method      string
		path        string
		expectValid bool
		errorType   string
	}{
		{
			name:        "valid path param",
			method:      "GET",
			path:        "/users/123",
			expectValid: true,
		},
		{
			name:        "invalid path param (non-integer)",
			method:      "GET",
			path:        "/users/abc",
			expectValid: false,
			errorType:   "path",
		},
		{
			name:        "unknown path",
			method:      "GET",
			path:        "/unknown",
			expectValid: false,
			errorType:   "path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "http://localhost:8080"+tt.path, nil)
			result := validator.ValidateRequest(req)

			if result.Valid != tt.expectValid {
				t.Errorf("expected valid=%v, got valid=%v, errors=%v", tt.expectValid, result.Valid, result.Errors)
			}

			if !tt.expectValid && tt.errorType != "" {
				if len(result.Errors) == 0 {
					t.Error("expected errors but got none")
				} else if result.Errors[0].Type != tt.errorType {
					t.Errorf("expected error type %s, got %s", tt.errorType, result.Errors[0].Type)
				}
			}
		})
	}
}

func TestValidateRequest_QueryParams(t *testing.T) {
	validator := mustCreateValidator(t)

	tests := []struct {
		name        string
		path        string
		expectValid bool
		errorType   string
	}{
		{
			name:        "no query params",
			path:        "/users",
			expectValid: true,
		},
		{
			name:        "valid query params",
			path:        "/users?limit=10&offset=0",
			expectValid: true,
		},
		{
			name:        "invalid limit (too high)",
			path:        "/users?limit=200",
			expectValid: false,
			errorType:   "query",
		},
		{
			name:        "invalid limit (negative)",
			path:        "/users?limit=-1",
			expectValid: false,
			errorType:   "query",
		},
		{
			name:        "invalid limit (non-integer)",
			path:        "/users?limit=abc",
			expectValid: false,
			errorType:   "query",
		},
		{
			name:        "invalid offset (negative)",
			path:        "/users?offset=-5",
			expectValid: false,
			errorType:   "query",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://localhost:8080"+tt.path, nil)
			result := validator.ValidateRequest(req)

			if result.Valid != tt.expectValid {
				t.Errorf("expected valid=%v, got valid=%v, errors=%v", tt.expectValid, result.Valid, result.Errors)
			}

			if !tt.expectValid && tt.errorType != "" && len(result.Errors) > 0 {
				if result.Errors[0].Type != tt.errorType {
					t.Errorf("expected error type %s, got %s", tt.errorType, result.Errors[0].Type)
				}
			}
		})
	}
}

func TestValidateRequest_Headers(t *testing.T) {
	validator := mustCreateValidator(t)

	tests := []struct {
		name        string
		path        string
		headers     map[string]string
		expectValid bool
		errorType   string
	}{
		{
			name:        "no optional header",
			path:        "/users/123",
			headers:     nil,
			expectValid: true,
		},
		{
			name: "valid UUID header",
			path: "/users/123",
			headers: map[string]string{
				"X-Request-ID": "550e8400-e29b-41d4-a716-446655440000",
			},
			expectValid: true,
		},
		{
			name: "invalid UUID header format",
			path: "/users/123",
			headers: map[string]string{
				"X-Request-ID": "not-a-uuid",
			},
			// Note: By default, kin-openapi doesn't strictly validate string formats
			// as the OpenAPI spec considers format as a hint, not strict validation.
			// Format validation can be enabled with custom format validators.
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://localhost:8080"+tt.path, nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			result := validator.ValidateRequest(req)

			if result.Valid != tt.expectValid {
				t.Errorf("expected valid=%v, got valid=%v, errors=%v", tt.expectValid, result.Valid, result.Errors)
			}

			if !tt.expectValid && tt.errorType != "" && len(result.Errors) > 0 {
				if result.Errors[0].Type != tt.errorType {
					t.Errorf("expected error type %s, got %s", tt.errorType, result.Errors[0].Type)
				}
			}
		})
	}
}

func TestValidateRequest_Body(t *testing.T) {
	validator := mustCreateValidator(t)

	tests := []struct {
		name        string
		method      string
		path        string
		body        string
		contentType string
		expectValid bool
		errorType   string
	}{
		{
			name:        "valid create user",
			method:      "POST",
			path:        "/users",
			body:        `{"name": "John Doe", "email": "john@example.com"}`,
			contentType: "application/json",
			expectValid: true,
		},
		{
			name:        "valid create user with age",
			method:      "POST",
			path:        "/users",
			body:        `{"name": "John Doe", "email": "john@example.com", "age": 30}`,
			contentType: "application/json",
			expectValid: true,
		},
		{
			name:        "missing required field (name)",
			method:      "POST",
			path:        "/users",
			body:        `{"email": "john@example.com"}`,
			contentType: "application/json",
			expectValid: false,
			errorType:   "body",
		},
		{
			name:        "missing required field (email)",
			method:      "POST",
			path:        "/users",
			body:        `{"name": "John Doe"}`,
			contentType: "application/json",
			expectValid: false,
			errorType:   "body",
		},
		{
			name:        "invalid email format",
			method:      "POST",
			path:        "/users",
			body:        `{"name": "John Doe", "email": "not-an-email"}`,
			contentType: "application/json",
			// Note: By default, kin-openapi doesn't strictly validate string formats
			// as the OpenAPI spec considers format as a hint, not strict validation.
			expectValid: true,
		},
		{
			name:        "name too short",
			method:      "POST",
			path:        "/users",
			body:        `{"name": "", "email": "john@example.com"}`,
			contentType: "application/json",
			expectValid: false,
			errorType:   "body",
		},
		{
			name:        "age too high",
			method:      "POST",
			path:        "/users",
			body:        `{"name": "John Doe", "email": "john@example.com", "age": 200}`,
			contentType: "application/json",
			expectValid: false,
			errorType:   "body",
		},
		{
			name:        "invalid JSON",
			method:      "POST",
			path:        "/users",
			body:        `{"name": "John Doe"`,
			contentType: "application/json",
			expectValid: false,
			errorType:   "body",
		},
		{
			name:        "empty body when required",
			method:      "POST",
			path:        "/users",
			body:        "",
			contentType: "application/json",
			expectValid: false,
		},
		{
			name:        "valid update user (partial)",
			method:      "PUT",
			path:        "/users/123",
			body:        `{"name": "Jane Doe"}`,
			contentType: "application/json",
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}

			req := httptest.NewRequest(tt.method, "http://localhost:8080"+tt.path, body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			result := validator.ValidateRequest(req)

			if result.Valid != tt.expectValid {
				t.Errorf("expected valid=%v, got valid=%v, errors=%v", tt.expectValid, result.Valid, result.Errors)
			}

			if !tt.expectValid && tt.errorType != "" && len(result.Errors) > 0 {
				if result.Errors[0].Type != tt.errorType {
					t.Errorf("expected error type %s, got %s", tt.errorType, result.Errors[0].Type)
				}
			}
		})
	}
}

func TestValidateResponse(t *testing.T) {
	config := &ValidationConfig{
		Enabled:          true,
		Spec:             testOpenAPISpec,
		ValidateRequest:  true,
		ValidateResponse: true,
	}

	validator, err := NewOpenAPIValidator(config)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	tests := []struct {
		name        string
		method      string
		path        string
		status      int
		body        string
		contentType string
		expectValid bool
	}{
		{
			name:        "valid user response",
			method:      "GET",
			path:        "/users/123",
			status:      200,
			body:        `{"id": 123, "name": "John Doe", "email": "john@example.com"}`,
			contentType: "application/json",
			expectValid: true,
		},
		{
			name:        "valid user list response",
			method:      "GET",
			path:        "/users",
			status:      200,
			body:        `[{"id": 1, "name": "John", "email": "john@example.com"}]`,
			contentType: "application/json",
			expectValid: true,
		},
		{
			name:        "invalid response - missing required field",
			method:      "GET",
			path:        "/users/123",
			status:      200,
			body:        `{"id": 123, "name": "John Doe"}`,
			contentType: "application/json",
			expectValid: false,
		},
		{
			name:        "invalid response - wrong type",
			method:      "GET",
			path:        "/users/123",
			status:      200,
			body:        `{"id": "not-a-number", "name": "John", "email": "john@example.com"}`,
			contentType: "application/json",
			expectValid: false,
		},
		{
			name:        "404 response (no schema check needed)",
			method:      "GET",
			path:        "/users/999",
			status:      404,
			body:        `{"error": "not found"}`,
			contentType: "application/json",
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "http://localhost:8080"+tt.path, nil)

			headers := http.Header{}
			headers.Set("Content-Type", tt.contentType)

			result := validator.ValidateResponse(req, tt.status, headers, []byte(tt.body))

			if result.Valid != tt.expectValid {
				t.Errorf("expected valid=%v, got valid=%v, errors=%v", tt.expectValid, result.Valid, result.Errors)
			}
		})
	}
}

func TestMiddleware(t *testing.T) {
	validator := mustCreateValidator(t)

	// Create a simple handler that returns success
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 123, "name": "John", "email": "john@example.com"}`))
	})

	config := &ValidationConfig{
		Enabled:         true,
		Spec:            testOpenAPISpec,
		ValidateRequest: true,
		FailOnError:     true,
	}

	middleware := NewMiddleware(handler, validator, config)

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		contentType    string
		expectedStatus int
	}{
		{
			name:           "valid GET request passes",
			method:         "GET",
			path:           "/users",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "valid GET with params passes",
			method:         "GET",
			path:           "/users?limit=10",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid query param returns 400",
			method:         "GET",
			path:           "/users?limit=999",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "valid POST request passes",
			method:         "POST",
			path:           "/users",
			body:           `{"name": "John", "email": "john@example.com"}`,
			contentType:    "application/json",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid POST body returns 400",
			method:         "POST",
			path:           "/users",
			body:           `{"name": ""}`,
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "unknown path returns 400",
			method:         "GET",
			path:           "/unknown",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}

			req := httptest.NewRequest(tt.method, "http://localhost:8080"+tt.path, body)
			if tt.contentType != "" {
				req.Header.Set("Content-Type", tt.contentType)
			}

			rec := httptest.NewRecorder()
			middleware.ServeHTTP(rec, req)

			if rec.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d, body=%s", tt.expectedStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestMiddleware_FailOnErrorFalse(t *testing.T) {
	validator := mustCreateValidator(t)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	config := &ValidationConfig{
		Enabled:         true,
		Spec:            testOpenAPISpec,
		ValidateRequest: true,
		FailOnError:     false, // Don't fail, just log
		LogWarnings:     true,
	}

	middleware := NewMiddleware(handler, validator, config)

	// Send an invalid request
	req := httptest.NewRequest("GET", "http://localhost:8080/users?limit=999", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	// Handler should be called even though validation failed
	if !handlerCalled {
		t.Error("handler should have been called when FailOnError=false")
	}

	// Should return handler's status, not 400
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestMiddleware_Disabled(t *testing.T) {
	config := &ValidationConfig{
		Enabled: false,
	}

	validator, _ := NewOpenAPIValidator(config)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewMiddleware(handler, validator, config)

	// Send a request to unknown path (would fail if validation was enabled)
	req := httptest.NewRequest("GET", "http://localhost:8080/anything", nil)
	rec := httptest.NewRecorder()

	middleware.ServeHTTP(rec, req)

	if !handlerCalled {
		t.Error("handler should have been called when validation is disabled")
	}
}

func TestWithValidation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test with nil config
	h, err := WithValidation(handler, nil)
	if err != nil {
		t.Errorf("unexpected error with nil config: %v", err)
	}
	if h == nil {
		t.Error("should return a handler when config is nil")
	}

	// Test with disabled config
	h, err = WithValidation(handler, &ValidationConfig{Enabled: false})
	if err != nil {
		t.Errorf("unexpected error with disabled config: %v", err)
	}
	if h == nil {
		t.Error("should return a handler when disabled")
	}

	// Test with valid config
	h, err = WithValidation(handler, &ValidationConfig{
		Enabled:         true,
		Spec:            testOpenAPISpec,
		ValidateRequest: true,
	})
	if err != nil {
		t.Errorf("unexpected error with valid config: %v", err)
	}
	// Check that we got a middleware wrapper
	_, isMiddleware := h.(*Middleware)
	if !isMiddleware {
		t.Error("should return wrapped handler (Middleware) when enabled")
	}
}

func TestFormatJSONPath(t *testing.T) {
	tests := []struct {
		input    []string
		expected string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"foo"}, "$.foo"},
		{[]string{"foo", "bar"}, "$.foo.bar"},
		{[]string{"foo", "0"}, "$.foo[0]"},
		{[]string{"foo", "0", "bar"}, "$.foo[0].bar"},
		{[]string{"items", "0", "name"}, "$.items[0].name"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatJSONPath(tt.input)
			if result != tt.expected {
				t.Errorf("formatJSONPath(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadSpecFromString(t *testing.T) {
	// Test valid YAML spec
	doc, err := LoadSpecFromString(testOpenAPISpec)
	if err != nil {
		t.Fatalf("failed to load valid spec: %v", err)
	}
	if doc.Info.Title != "Test API" {
		t.Errorf("expected title 'Test API', got '%s'", doc.Info.Title)
	}

	// Test valid JSON spec
	jsonSpec := `{
		"openapi": "3.0.3",
		"info": {"title": "JSON API", "version": "1.0.0"},
		"paths": {}
	}`
	doc, err = LoadSpecFromString(jsonSpec)
	if err != nil {
		t.Fatalf("failed to load JSON spec: %v", err)
	}
	if doc.Info.Title != "JSON API" {
		t.Errorf("expected title 'JSON API', got '%s'", doc.Info.Title)
	}

	// Test invalid spec
	_, err = LoadSpecFromString("not valid yaml or json")
	if err == nil {
		t.Error("expected error for invalid spec")
	}
}

func TestDefaultValidationConfig(t *testing.T) {
	config := DefaultValidationConfig()

	if config.Enabled {
		t.Error("expected Enabled to be false by default")
	}
	if !config.ValidateRequest {
		t.Error("expected ValidateRequest to be true by default")
	}
	if config.ValidateResponse {
		t.Error("expected ValidateResponse to be false by default")
	}
	if !config.FailOnError {
		t.Error("expected FailOnError to be true by default")
	}
	if !config.LogWarnings {
		t.Error("expected LogWarnings to be true by default")
	}
}

func TestValidateRequest_BodyReusable(t *testing.T) {
	validator := mustCreateValidator(t)

	body := `{"name": "John Doe", "email": "john@example.com"}`
	req := httptest.NewRequest("POST", "http://localhost:8080/users", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Validate request (this reads the body)
	result := validator.ValidateRequest(req)
	if !result.Valid {
		t.Errorf("expected valid request, got errors: %v", result.Errors)
	}

	// Body should still be readable after validation
	readBody, err := io.ReadAll(req.Body)
	if err != nil {
		t.Errorf("failed to read body after validation: %v", err)
	}
	if string(readBody) != body {
		t.Errorf("body changed after validation: got %q, want %q", string(readBody), body)
	}
}

func TestMiddleware_ResponseValidation(t *testing.T) {
	config := &ValidationConfig{
		Enabled:          true,
		Spec:             testOpenAPISpec,
		ValidateRequest:  true,
		ValidateResponse: true,
		FailOnError:      true,
		LogWarnings:      true,
	}

	validator, err := NewOpenAPIValidator(config)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	// Handler that returns valid response
	validHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 123, "name": "John", "email": "john@example.com"}`))
	})

	middleware := NewMiddleware(validHandler, validator, config)

	req := httptest.NewRequest("GET", "http://localhost:8080/users/123", nil)
	rec := httptest.NewRecorder()
	middleware.ServeHTTP(rec, req)

	// Response validation happens but doesn't change the response
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

// mustCreateValidator creates a validator for testing, failing on error
func mustCreateValidator(t *testing.T) *OpenAPIValidator {
	t.Helper()
	config := &ValidationConfig{
		Enabled:         true,
		Spec:            testOpenAPISpec,
		ValidateRequest: true,
	}

	validator, err := NewOpenAPIValidator(config)
	if err != nil {
		t.Fatalf("failed to create validator: %v", err)
	}

	return validator
}

func BenchmarkValidateRequest(b *testing.B) {
	config := &ValidationConfig{
		Enabled:         true,
		Spec:            testOpenAPISpec,
		ValidateRequest: true,
	}

	validator, err := NewOpenAPIValidator(config)
	if err != nil {
		b.Fatalf("failed to create validator: %v", err)
	}

	body := `{"name": "John Doe", "email": "john@example.com"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "http://localhost:8080/users", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		validator.ValidateRequest(req)
	}
}

func BenchmarkMiddleware(b *testing.B) {
	config := &ValidationConfig{
		Enabled:         true,
		Spec:            testOpenAPISpec,
		ValidateRequest: true,
		FailOnError:     true,
	}

	validator, err := NewOpenAPIValidator(config)
	if err != nil {
		b.Fatalf("failed to create validator: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := NewMiddleware(handler, validator, config)
	body := `{"name": "John Doe", "email": "john@example.com"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest("POST", "http://localhost:8080/users", bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		middleware.ServeHTTP(rec, req)
	}
}
