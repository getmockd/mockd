package validation

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ErrorCode constants for machine-readable error identification
const (
	ErrCodeRequired     = "required"
	ErrCodeType         = "type"
	ErrCodeMinLength    = "min_length"
	ErrCodeMaxLength    = "max_length"
	ErrCodePattern      = "pattern"
	ErrCodeFormat       = "format"
	ErrCodeMin          = "min"
	ErrCodeMax          = "max"
	ErrCodeExclusiveMin = "exclusive_min"
	ErrCodeExclusiveMax = "exclusive_max"
	ErrCodeMinItems     = "min_items"
	ErrCodeMaxItems     = "max_items"
	ErrCodeUniqueItems  = "unique_items"
	ErrCodeEnum         = "enum"
	ErrCodeSchema       = "schema"
	ErrCodeInvalidJSON  = "invalid_json"
	ErrCodeUnknownField = "unknown_field"
)

// ErrorLocation constants
const (
	LocationBody   = "body"
	LocationPath   = "path"
	LocationQuery  = "query"
	LocationHeader = "header"
)

// FieldError represents a detailed validation error for a single field.
type FieldError struct {
	// Field is the name of the field that failed validation
	Field string `json:"field"`

	// Location indicates where the field is: body, path, query, header
	Location string `json:"location"`

	// Code is a machine-readable error code
	Code string `json:"code"`

	// Message is a human-readable error description
	Message string `json:"message"`

	// Received is the actual value that was received (may be omitted for security)
	Received interface{} `json:"received,omitempty"`

	// Expected describes what was expected
	Expected string `json:"expected,omitempty"`

	// Hint provides a user-friendly suggestion for fixing the error
	Hint string `json:"hint,omitempty"`
}

// Error implements the error interface
func (e *FieldError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("%s.%s: %s", e.Location, e.Field, e.Message)
	}
	return e.Message
}

// Result contains the outcome of validation.
type Result struct {
	// Valid is true if validation passed
	Valid bool `json:"valid"`

	// Errors contains validation errors (when Valid is false)
	Errors []*FieldError `json:"errors,omitempty"`

	// Warnings contains non-fatal validation warnings
	Warnings []*FieldError `json:"warnings,omitempty"`
}

// AddError adds a validation error to the result
func (r *Result) AddError(err *FieldError) {
	r.Valid = false
	r.Errors = append(r.Errors, err)
}

// AddWarning adds a validation warning to the result
func (r *Result) AddWarning(warn *FieldError) {
	r.Warnings = append(r.Warnings, warn)
}

// HasErrors returns true if there are any validation errors
func (r *Result) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if there are any validation warnings
func (r *Result) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// Merge combines another result into this one
func (r *Result) Merge(other *Result) {
	if other == nil {
		return
	}
	if !other.Valid {
		r.Valid = false
	}
	r.Errors = append(r.Errors, other.Errors...)
	r.Warnings = append(r.Warnings, other.Warnings...)
}

// ErrorResponse is the HTTP response body for validation failures.
// It follows RFC 7807 Problem Details format.
type ErrorResponse struct {
	// Type identifies the error type
	Type string `json:"type"`

	// Title is a short summary
	Title string `json:"title"`

	// Status is the HTTP status code
	Status int `json:"status"`

	// Detail provides additional context
	Detail string `json:"detail,omitempty"`

	// Errors lists all validation errors
	Errors []*FieldError `json:"errors"`
}

// NewErrorResponse creates an ErrorResponse from a Result
func NewErrorResponse(result *Result, status int) *ErrorResponse {
	if status == 0 {
		status = http.StatusBadRequest
	}

	detail := ""
	if len(result.Errors) == 1 {
		detail = result.Errors[0].Message
	} else if len(result.Errors) > 1 {
		detail = fmt.Sprintf("%d validation errors", len(result.Errors))
	}

	return &ErrorResponse{
		Type:   "validation_error",
		Title:  "Request Validation Failed",
		Status: status,
		Detail: detail,
		Errors: result.Errors,
	}
}

// WriteResponse writes the error response as JSON to the http.ResponseWriter
func (e *ErrorResponse) WriteResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(e.Status)
	_ = json.NewEncoder(w).Encode(e)
}

// Error implements the error interface
func (e *ErrorResponse) Error() string {
	return fmt.Sprintf("%s: %s", e.Title, e.Detail)
}

// Helper functions for creating common errors

// NewRequiredError creates an error for a missing required field
func NewRequiredError(field, location string) *FieldError {
	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodeRequired,
		Message:  fmt.Sprintf("field '%s' is required", field),
		Expected: "non-null value",
		Hint:     fmt.Sprintf("Add the '%s' field to your request %s", field, location),
	}
}

// NewTypeError creates an error for a type mismatch
func NewTypeError(field, location, expected string, received interface{}) *FieldError {
	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodeType,
		Message:  fmt.Sprintf("expected type '%s'", expected),
		Received: received,
		Expected: expected,
		Hint:     fmt.Sprintf("Ensure '%s' is a valid %s", field, expected),
	}
}

// NewMinLengthError creates an error for string too short
func NewMinLengthError(field, location string, minLength int, actual int) *FieldError {
	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodeMinLength,
		Message:  fmt.Sprintf("must be at least %d characters", minLength),
		Received: actual,
		Expected: fmt.Sprintf(">= %d characters", minLength),
		Hint:     fmt.Sprintf("Add more characters to '%s'", field),
	}
}

// NewMaxLengthError creates an error for string too long
func NewMaxLengthError(field, location string, maxLength int, actual int) *FieldError {
	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodeMaxLength,
		Message:  fmt.Sprintf("must be at most %d characters", maxLength),
		Received: actual,
		Expected: fmt.Sprintf("<= %d characters", maxLength),
		Hint:     fmt.Sprintf("Reduce the length of '%s'", field),
	}
}

// NewPatternError creates an error for regex pattern mismatch
func NewPatternError(field, location, pattern string, received interface{}) *FieldError {
	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodePattern,
		Message:  fmt.Sprintf("must match pattern '%s'", pattern),
		Received: received,
		Expected: fmt.Sprintf("pattern: %s", pattern),
		Hint:     fmt.Sprintf("Ensure '%s' matches the required format", field),
	}
}

// NewFormatError creates an error for format validation failure
func NewFormatError(field, location, format string, received interface{}) *FieldError {
	hints := map[string]string{
		"email":    "Example: user@example.com",
		"uuid":     "Example: 550e8400-e29b-41d4-a716-446655440000",
		"date":     "Example: 2024-01-15",
		"datetime": "Example: 2024-01-15T10:30:00Z",
		"uri":      "Example: https://example.com/path",
		"ipv4":     "Example: 192.168.1.1",
		"ipv6":     "Example: 2001:db8::1",
		"hostname": "Example: api.example.com",
	}

	hint := hints[format]
	if hint == "" {
		hint = fmt.Sprintf("Ensure '%s' is a valid %s", field, format)
	}

	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodeFormat,
		Message:  fmt.Sprintf("must be a valid %s", format),
		Received: received,
		Expected: fmt.Sprintf("format: %s", format),
		Hint:     hint,
	}
}

// NewMinError creates an error for number below minimum
func NewMinError(field, location string, min float64, received interface{}) *FieldError {
	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodeMin,
		Message:  fmt.Sprintf("must be >= %v", min),
		Received: received,
		Expected: fmt.Sprintf(">= %v", min),
		Hint:     fmt.Sprintf("Increase the value of '%s'", field),
	}
}

// NewMaxError creates an error for number above maximum
func NewMaxError(field, location string, max float64, received interface{}) *FieldError {
	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodeMax,
		Message:  fmt.Sprintf("must be <= %v", max),
		Received: received,
		Expected: fmt.Sprintf("<= %v", max),
		Hint:     fmt.Sprintf("Decrease the value of '%s'", field),
	}
}

// NewMinItemsError creates an error for array with too few items
func NewMinItemsError(field, location string, minItems int, actual int) *FieldError {
	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodeMinItems,
		Message:  fmt.Sprintf("must have at least %d items", minItems),
		Received: actual,
		Expected: fmt.Sprintf(">= %d items", minItems),
		Hint:     fmt.Sprintf("Add more items to '%s'", field),
	}
}

// NewMaxItemsError creates an error for array with too many items
func NewMaxItemsError(field, location string, maxItems int, actual int) *FieldError {
	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodeMaxItems,
		Message:  fmt.Sprintf("must have at most %d items", maxItems),
		Received: actual,
		Expected: fmt.Sprintf("<= %d items", maxItems),
		Hint:     fmt.Sprintf("Remove some items from '%s'", field),
	}
}

// NewUniqueItemsError creates an error for duplicate items in array
func NewUniqueItemsError(field, location string, duplicate interface{}) *FieldError {
	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodeUniqueItems,
		Message:  "items must be unique",
		Received: duplicate,
		Expected: "unique items",
		Hint:     fmt.Sprintf("Remove duplicate values from '%s'", field),
	}
}

// NewEnumError creates an error for value not in enum
func NewEnumError(field, location string, allowed []interface{}, received interface{}) *FieldError {
	allowedStrs := make([]string, len(allowed))
	for i, v := range allowed {
		allowedStrs[i] = fmt.Sprintf("%v", v)
	}

	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodeEnum,
		Message:  fmt.Sprintf("must be one of: %s", strings.Join(allowedStrs, ", ")),
		Received: received,
		Expected: fmt.Sprintf("one of: %s", strings.Join(allowedStrs, ", ")),
		Hint:     fmt.Sprintf("Use an allowed value for '%s'", field),
	}
}

// NewSchemaError creates an error for JSON Schema validation failure
func NewSchemaError(field, location, message string) *FieldError {
	return &FieldError{
		Field:    field,
		Location: location,
		Code:     ErrCodeSchema,
		Message:  message,
		Hint:     "Check your request against the JSON Schema",
	}
}

// NewInvalidJSONError creates an error for malformed JSON
func NewInvalidJSONError(message string) *FieldError {
	return &FieldError{
		Field:    "",
		Location: LocationBody,
		Code:     ErrCodeInvalidJSON,
		Message:  fmt.Sprintf("invalid JSON: %s", message),
		Hint:     "Ensure your request body is valid JSON",
	}
}
