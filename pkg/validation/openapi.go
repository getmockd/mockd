package validation

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// OpenAPIValidator validates requests against OpenAPI specs
type OpenAPIValidator struct {
	doc    *openapi3.T
	router routers.Router
	config *ValidationConfig
}

// ValidationConfig configures request validation
type ValidationConfig struct {
	Enabled          bool   `json:"enabled" yaml:"enabled"`
	SpecFile         string `json:"specFile,omitempty" yaml:"specFile,omitempty"`
	SpecURL          string `json:"specUrl,omitempty" yaml:"specUrl,omitempty"`
	Spec             string `json:"spec,omitempty" yaml:"spec,omitempty"` // Inline spec
	ValidateRequest  bool   `json:"validateRequest" yaml:"validateRequest"`
	ValidateResponse bool   `json:"validateResponse" yaml:"validateResponse"`
	FailOnError      bool   `json:"failOnError" yaml:"failOnError"` // Return 400 on validation failure
	LogWarnings      bool   `json:"logWarnings" yaml:"logWarnings"` // Log warnings but don't fail
}

// ValidationResult contains validation results
type ValidationResult struct {
	Valid    bool              `json:"valid"`
	Errors   []ValidationError `json:"errors,omitempty"`
	Warnings []ValidationError `json:"warnings,omitempty"`
}

// ValidationError represents a single validation error
type ValidationError struct {
	Type     string `json:"type"`               // path, query, header, body, response
	Field    string `json:"field,omitempty"`    // Field name that failed validation
	Message  string `json:"message"`            // Human-readable error message
	Location string `json:"location,omitempty"` // JSONPath for body errors
}

// NewOpenAPIValidator creates a validator from config
func NewOpenAPIValidator(config *ValidationConfig) (*OpenAPIValidator, error) {
	if config == nil {
		return nil, fmt.Errorf("validation config is required")
	}

	if !config.Enabled {
		return &OpenAPIValidator{config: config}, nil
	}

	var doc *openapi3.T
	var err error

	// Load spec from one of the sources
	switch {
	case config.SpecFile != "":
		doc, err = LoadSpec(config.SpecFile)
	case config.SpecURL != "":
		doc, err = LoadSpecFromURL(config.SpecURL)
	case config.Spec != "":
		doc, err = LoadSpecFromString(config.Spec)
	default:
		return nil, fmt.Errorf("no OpenAPI spec source provided (specFile, specUrl, or spec required)")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load OpenAPI spec: %w", err)
	}

	// Validate the loaded spec
	ctx := context.Background()
	if err := doc.Validate(ctx); err != nil {
		return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
	}

	// Create router for matching requests to operations
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	return &OpenAPIValidator{
		doc:    doc,
		router: router,
		config: config,
	}, nil
}

// LoadSpec loads an OpenAPI spec from a file path
func LoadSpec(path string) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load spec from file %s: %w", path, err)
	}

	return doc, nil
}

// LoadSpecFromURL loads an OpenAPI spec from a URL
func LoadSpecFromURL(specURL string) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	parsedURL, err := url.Parse(specURL)
	if err != nil {
		return nil, fmt.Errorf("invalid spec URL: %w", err)
	}

	doc, err := loader.LoadFromURI(parsedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to load spec from URL %s: %w", specURL, err)
	}

	return doc, nil
}

// LoadSpecFromString loads an OpenAPI spec from a string
func LoadSpecFromString(spec string) (*openapi3.T, error) {
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	doc, err := loader.LoadFromData([]byte(spec))
	if err != nil {
		return nil, fmt.Errorf("failed to load spec from string: %w", err)
	}

	return doc, nil
}

// ValidateRequest validates an incoming HTTP request
func (v *OpenAPIValidator) ValidateRequest(r *http.Request) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// If validation is disabled or no spec loaded, return valid
	if v.doc == nil || v.router == nil || !v.config.ValidateRequest {
		return result
	}

	ctx := r.Context()

	// Find the route matching this request
	route, pathParams, err := v.router.FindRoute(r)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Type:    "path",
			Message: fmt.Sprintf("no matching route found: %s", err.Error()),
		})
		return result
	}

	// Build request validation input
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			MultiError:            true,
			IncludeResponseStatus: true,
		},
	}

	// Store body for potential re-read (needed if body validation is performed)
	if r.Body != nil && r.Body != http.NoBody {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Type:    "body",
				Message: fmt.Sprintf("failed to read request body: %s", err.Error()),
			})
			return result
		}
		r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		requestValidationInput.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	// Validate the request
	if err := openapi3filter.ValidateRequest(ctx, requestValidationInput); err != nil {
		v.parseValidationErrors(err, result)
	}

	return result
}

// ValidateResponse validates an HTTP response
func (v *OpenAPIValidator) ValidateResponse(r *http.Request, status int, headers http.Header, body []byte) *ValidationResult {
	result := &ValidationResult{Valid: true}

	// If validation is disabled or no spec loaded, return valid
	if v.doc == nil || v.router == nil || !v.config.ValidateResponse {
		return result
	}

	ctx := r.Context()

	// Find the route matching this request
	route, pathParams, err := v.router.FindRoute(r)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, ValidationError{
			Type:    "response",
			Message: fmt.Sprintf("no matching route found: %s", err.Error()),
		})
		return result
	}

	// Build request validation input (needed for response validation)
	requestValidationInput := &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: pathParams,
		Route:      route,
	}

	// Build response validation input
	responseValidationInput := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestValidationInput,
		Status:                 status,
		Header:                 headers,
		Options: &openapi3filter.Options{
			MultiError:            true,
			IncludeResponseStatus: true,
		},
	}

	// Set body if present
	if len(body) > 0 {
		responseValidationInput.SetBodyBytes(body)
	}

	// Validate the response
	if err := openapi3filter.ValidateResponse(ctx, responseValidationInput); err != nil {
		v.parseValidationErrors(err, result)
		// Mark response errors specifically
		for i := range result.Errors {
			if result.Errors[i].Type == "" {
				result.Errors[i].Type = "response"
			}
		}
	}

	return result
}

// parseValidationErrors converts kin-openapi errors to ValidationErrors
func (v *OpenAPIValidator) parseValidationErrors(err error, result *ValidationResult) {
	if err == nil {
		return
	}

	result.Valid = false

	// Handle multi-error
	if multiErr, ok := err.(openapi3.MultiError); ok {
		for _, e := range multiErr {
			v.parseValidationErrors(e, result)
		}
		// Remove duplicate valid=false assignments
		result.Valid = false
		return
	}

	// Handle request error
	if reqErr, ok := err.(*openapi3filter.RequestError); ok {
		ve := ValidationError{
			Message: reqErr.Error(),
		}

		// Determine error type based on parameter location
		if reqErr.Parameter != nil {
			ve.Field = reqErr.Parameter.Name
			switch reqErr.Parameter.In {
			case "path":
				ve.Type = "path"
			case "query":
				ve.Type = "query"
			case "header":
				ve.Type = "header"
			case "cookie":
				ve.Type = "cookie"
			default:
				ve.Type = "parameter"
			}
		} else if reqErr.RequestBody != nil {
			ve.Type = "body"
		} else {
			ve.Type = "request"
		}

		// Extract more specific error info if available
		if reqErr.Err != nil {
			ve.Message = reqErr.Err.Error()

			// Check for schema validation error
			if schemaErr, ok := reqErr.Err.(*openapi3.SchemaError); ok {
				ve.Location = formatJSONPath(schemaErr.JSONPointer())
				ve.Message = schemaErr.Reason
			}
		}

		result.Errors = append(result.Errors, ve)
		return
	}

	// Handle response error
	if respErr, ok := err.(*openapi3filter.ResponseError); ok {
		ve := ValidationError{
			Type:    "response",
			Message: respErr.Error(),
		}

		if respErr.Err != nil {
			ve.Message = respErr.Err.Error()

			// Check for schema validation error
			if schemaErr, ok := respErr.Err.(*openapi3.SchemaError); ok {
				ve.Location = formatJSONPath(schemaErr.JSONPointer())
				ve.Message = schemaErr.Reason
			}
		}

		result.Errors = append(result.Errors, ve)
		return
	}

	// Handle security error
	if secErr, ok := err.(*openapi3filter.SecurityRequirementsError); ok {
		ve := ValidationError{
			Type:    "security",
			Message: secErr.Error(),
		}
		result.Errors = append(result.Errors, ve)
		return
	}

	// Handle schema error directly
	if schemaErr, ok := err.(*openapi3.SchemaError); ok {
		ve := ValidationError{
			Type:     "schema",
			Message:  schemaErr.Reason,
			Location: formatJSONPath(schemaErr.JSONPointer()),
		}
		result.Errors = append(result.Errors, ve)
		return
	}

	// Generic error
	result.Errors = append(result.Errors, ValidationError{
		Type:    "validation",
		Message: err.Error(),
	})
}

// formatJSONPath converts a JSON pointer parts array to a more readable format
func formatJSONPath(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	// Convert ["foo", "bar", "0"] to $.foo.bar[0]
	var sb strings.Builder
	sb.WriteString("$")
	for _, part := range parts {
		if part == "" {
			continue
		}
		// Check if it's an array index
		if isNumeric(part) {
			sb.WriteString("[")
			sb.WriteString(part)
			sb.WriteString("]")
		} else {
			sb.WriteString(".")
			sb.WriteString(part)
		}
	}
	return sb.String()
}

// isNumeric checks if a string is a number
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// IsEnabled returns whether validation is enabled
func (v *OpenAPIValidator) IsEnabled() bool {
	return v.config != nil && v.config.Enabled
}

// GetSpec returns the loaded OpenAPI document
func (v *OpenAPIValidator) GetSpec() *openapi3.T {
	return v.doc
}

// GetConfig returns the validation configuration
func (v *OpenAPIValidator) GetConfig() *ValidationConfig {
	return v.config
}

// DefaultValidationConfig returns a sensible default configuration
func DefaultValidationConfig() *ValidationConfig {
	return &ValidationConfig{
		Enabled:          false,
		ValidateRequest:  true,
		ValidateResponse: false,
		FailOnError:      true,
		LogWarnings:      true,
	}
}

// LoadSpecFromEnv loads spec configuration from environment variables
// MOCKD_OPENAPI_SPEC_FILE, MOCKD_OPENAPI_SPEC_URL, or MOCKD_OPENAPI_SPEC
func LoadSpecFromEnv() *ValidationConfig {
	config := DefaultValidationConfig()

	if specFile := os.Getenv("MOCKD_OPENAPI_SPEC_FILE"); specFile != "" {
		config.SpecFile = specFile
		config.Enabled = true
	} else if specURL := os.Getenv("MOCKD_OPENAPI_SPEC_URL"); specURL != "" {
		config.SpecURL = specURL
		config.Enabled = true
	} else if spec := os.Getenv("MOCKD_OPENAPI_SPEC"); spec != "" {
		config.Spec = spec
		config.Enabled = true
	}

	if os.Getenv("MOCKD_OPENAPI_VALIDATE_RESPONSE") == "true" {
		config.ValidateResponse = true
	}

	if os.Getenv("MOCKD_OPENAPI_FAIL_ON_ERROR") == "false" {
		config.FailOnError = false
	}

	return config
}
