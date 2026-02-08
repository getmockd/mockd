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

// Note: OpenAPI validation now uses the unified Result and FieldError types
// from errors.go for consistency across all validation methods.

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
func (v *OpenAPIValidator) ValidateRequest(r *http.Request) *Result {
	result := &Result{Valid: true}

	// If validation is disabled or no spec loaded, return valid
	if v.doc == nil || v.router == nil || !v.config.ValidateRequest {
		return result
	}

	ctx := r.Context()

	// Find the route matching this request
	route, pathParams, err := v.router.FindRoute(r)
	if err != nil {
		result.AddError(&FieldError{
			Location: LocationPath,
			Code:     "no_route",
			Message:  fmt.Sprintf("no matching route found: %s", err.Error()),
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
		const maxValidationBodySize = 10 << 20 // 10MB defense-in-depth
		bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, maxValidationBodySize))
		if err != nil {
			result.AddError(&FieldError{
				Location: LocationBody,
				Code:     "read_error",
				Message:  fmt.Sprintf("failed to read request body: %s", err.Error()),
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
func (v *OpenAPIValidator) ValidateResponse(r *http.Request, status int, headers http.Header, body []byte) *Result {
	result := &Result{Valid: true}

	// If validation is disabled or no spec loaded, return valid
	if v.doc == nil || v.router == nil || !v.config.ValidateResponse {
		return result
	}

	ctx := r.Context()

	// Find the route matching this request
	route, pathParams, err := v.router.FindRoute(r)
	if err != nil {
		result.AddError(&FieldError{
			Location: "response",
			Code:     "no_route",
			Message:  fmt.Sprintf("no matching route found: %s", err.Error()),
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
		// Mark response errors with location if not set
		for _, e := range result.Errors {
			if e.Location == "" {
				e.Location = "response"
			}
		}
	}

	return result
}

// parseValidationErrors converts kin-openapi errors to FieldErrors
func (v *OpenAPIValidator) parseValidationErrors(err error, result *Result) {
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
		fe := &FieldError{
			Message: reqErr.Error(),
			Code:    "openapi_validation",
		}

		// Determine location based on parameter location
		if reqErr.Parameter != nil {
			fe.Field = reqErr.Parameter.Name
			switch reqErr.Parameter.In {
			case "path":
				fe.Location = LocationPath
			case "query":
				fe.Location = LocationQuery
			case "header":
				fe.Location = LocationHeader
			case "cookie":
				fe.Location = "cookie"
			default:
				fe.Location = "parameter"
			}
		} else if reqErr.RequestBody != nil {
			fe.Location = LocationBody
		} else {
			fe.Location = "request"
		}

		// Extract more specific error info if available
		if reqErr.Err != nil {
			fe.Message = reqErr.Err.Error()

			// Check for schema validation error
			if schemaErr, ok := reqErr.Err.(*openapi3.SchemaError); ok {
				jsonPath := formatJSONPath(schemaErr.JSONPointer())
				if jsonPath != "" && jsonPath != "$" {
					fe.Field = jsonPath
				}
				fe.Message = schemaErr.Reason
				fe.Code = ErrCodeSchema
			}
		}

		result.Errors = append(result.Errors, fe)
		return
	}

	// Handle response error
	if respErr, ok := err.(*openapi3filter.ResponseError); ok {
		fe := &FieldError{
			Location: "response",
			Code:     "openapi_validation",
			Message:  respErr.Error(),
		}

		if respErr.Err != nil {
			fe.Message = respErr.Err.Error()

			// Check for schema validation error
			if schemaErr, ok := respErr.Err.(*openapi3.SchemaError); ok {
				jsonPath := formatJSONPath(schemaErr.JSONPointer())
				if jsonPath != "" && jsonPath != "$" {
					fe.Field = jsonPath
				}
				fe.Message = schemaErr.Reason
				fe.Code = ErrCodeSchema
			}
		}

		result.Errors = append(result.Errors, fe)
		return
	}

	// Handle security error
	if secErr, ok := err.(*openapi3filter.SecurityRequirementsError); ok {
		result.Errors = append(result.Errors, &FieldError{
			Location: "security",
			Code:     "security",
			Message:  secErr.Error(),
		})
		return
	}

	// Handle schema error directly
	if schemaErr, ok := err.(*openapi3.SchemaError); ok {
		fe := &FieldError{
			Location: LocationBody,
			Code:     ErrCodeSchema,
			Message:  schemaErr.Reason,
		}
		jsonPath := formatJSONPath(schemaErr.JSONPointer())
		if jsonPath != "" && jsonPath != "$" {
			fe.Field = jsonPath
		}
		result.Errors = append(result.Errors, fe)
		return
	}

	// Generic error
	result.Errors = append(result.Errors, &FieldError{
		Location: "validation",
		Code:     "openapi_validation",
		Message:  err.Error(),
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
