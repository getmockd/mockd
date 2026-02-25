package stateful

import (
	"fmt"
	"net/http"
)

// NotFoundError is returned when a resource or item is not found.
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	if e.ID != "" {
		return fmt.Sprintf("resource %q item %q not found", e.Resource, e.ID)
	}
	return fmt.Sprintf("resource %q not found", e.Resource)
}

// StatusCode returns the HTTP status code for this error.
func (e *NotFoundError) StatusCode() int {
	return http.StatusNotFound
}

// ErrorCode returns the protocol-agnostic error code.
func (e *NotFoundError) ErrorCode() ErrorCode {
	return ErrCodeNotFound
}

// Hint returns a user-friendly suggestion for resolving this error.
func (e *NotFoundError) Hint() string {
	if e.ID != "" {
		return fmt.Sprintf("Check that item ID %q exists in resource %q. Use GET /%s to list available items.", e.ID, e.Resource, e.Resource)
	}
	return fmt.Sprintf("Resource %q is not registered. Check your configuration.", e.Resource)
}

// ConflictError is returned when an item with the same ID already exists.
type ConflictError struct {
	Resource string
	ID       string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("resource %q item %q already exists", e.Resource, e.ID)
}

// StatusCode returns the HTTP status code for this error.
func (e *ConflictError) StatusCode() int {
	return http.StatusConflict
}

// ErrorCode returns the protocol-agnostic error code.
func (e *ConflictError) ErrorCode() ErrorCode {
	return ErrCodeConflict
}

// Hint returns a user-friendly suggestion for resolving this error.
func (e *ConflictError) Hint() string {
	return fmt.Sprintf("Item with ID %q already exists. Use PUT to update or provide a different ID.", e.ID)
}

// ValidationError is returned when input validation fails.
type ValidationError struct {
	Message string
	Field   string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation failed for field %q: %s", e.Field, e.Message)
	}
	return e.Message
}

// StatusCode returns the HTTP status code for this error.
func (e *ValidationError) StatusCode() int {
	return http.StatusBadRequest
}

// ErrorCode returns the protocol-agnostic error code.
func (e *ValidationError) ErrorCode() ErrorCode {
	return ErrCodeValidation
}

// Hint returns a user-friendly suggestion for resolving this error.
func (e *ValidationError) Hint() string {
	if e.Field != "" {
		return fmt.Sprintf("Check the value of field %q in your request body.", e.Field)
	}
	return "Check your request body format and required fields."
}

// PayloadTooLargeError is returned when request body exceeds size limits.
type PayloadTooLargeError struct {
	MaxSize     int64
	ActualSize  int64
	ContentType string
}

func (e *PayloadTooLargeError) Error() string {
	return fmt.Sprintf("request body too large: max %d bytes allowed", e.MaxSize)
}

// StatusCode returns the HTTP status code for this error.
func (e *PayloadTooLargeError) StatusCode() int {
	return http.StatusRequestEntityTooLarge
}

// ErrorCode returns the protocol-agnostic error code.
func (e *PayloadTooLargeError) ErrorCode() ErrorCode {
	return ErrCodePayloadTooLarge
}

// Hint returns a user-friendly suggestion for resolving this error.
func (e *PayloadTooLargeError) Hint() string {
	return fmt.Sprintf("Reduce request body size to under %d bytes.", e.MaxSize)
}

// CapacityError is returned when a resource has reached its maximum item limit.
type CapacityError struct {
	Resource string
	MaxItems int
}

func (e *CapacityError) Error() string {
	return fmt.Sprintf("resource %q has reached its maximum capacity of %d items", e.Resource, e.MaxItems)
}

// StatusCode returns the HTTP status code for this error.
func (e *CapacityError) StatusCode() int {
	return http.StatusInsufficientStorage // 507
}

// ErrorCode returns the protocol-agnostic error code.
func (e *CapacityError) ErrorCode() ErrorCode {
	return ErrCodeCapacityExceeded
}

// Hint returns a user-friendly suggestion for resolving this error.
func (e *CapacityError) Hint() string {
	return fmt.Sprintf("Delete existing items or increase the maxItems limit (currently %d) for resource %q.", e.MaxItems, e.Resource)
}

// ErrorCode represents a protocol-agnostic error classification.
// Protocol adapters map these to protocol-specific error representations:
//
//	HTTP: ErrorCode → HTTP status code
//	SOAP: ErrorCode → SOAP fault code (Client/Sender or Server/Receiver)
//	gRPC: ErrorCode → gRPC status code (NOT_FOUND, ALREADY_EXISTS, etc.)
//	GraphQL: ErrorCode → GraphQL error extensions code
type ErrorCode int

const (
	// ErrCodeNotFound indicates the requested resource or item was not found.
	ErrCodeNotFound ErrorCode = iota
	// ErrCodeConflict indicates a duplicate ID or state conflict.
	ErrCodeConflict
	// ErrCodeValidation indicates invalid input data.
	ErrCodeValidation
	// ErrCodePayloadTooLarge indicates the request body exceeds size limits.
	ErrCodePayloadTooLarge
	// ErrCodeCapacityExceeded indicates the resource has reached its maximum capacity.
	ErrCodeCapacityExceeded
	// ErrCodeInternal indicates an unexpected internal error.
	ErrCodeInternal
)

// String returns a human-readable representation of the error code.
func (c ErrorCode) String() string {
	switch c {
	case ErrCodeNotFound:
		return "NOT_FOUND"
	case ErrCodeConflict:
		return "CONFLICT"
	case ErrCodeValidation:
		return "VALIDATION_ERROR"
	case ErrCodePayloadTooLarge:
		return "PAYLOAD_TOO_LARGE"
	case ErrCodeCapacityExceeded:
		return "CAPACITY_EXCEEDED"
	case ErrCodeInternal:
		return "INTERNAL_ERROR"
	default:
		return "UNKNOWN"
	}
}

// ErrorCodeError is an interface for errors that provide a protocol-agnostic error code.
// This is the preferred interface for protocol adapters (SOAP, gRPC, GraphQL).
// The existing StatusCodeError interface is preserved for backward compatibility with HTTP handlers.
type ErrorCodeError interface {
	error
	ErrorCode() ErrorCode
}

// StatusCodeError is an interface for errors that have an HTTP status code.
// Retained for backward compatibility with the HTTP stateful handler.
type StatusCodeError interface {
	error
	StatusCode() int
}

// HintError is an interface for errors that provide resolution hints.
type HintError interface {
	error
	Hint() string
}

// GetErrorCode extracts the ErrorCode from an error.
// Returns ErrCodeInternal if the error does not implement ErrorCodeError.
func GetErrorCode(err error) ErrorCode {
	if ec, ok := err.(ErrorCodeError); ok {
		return ec.ErrorCode()
	}
	return ErrCodeInternal
}

// ToErrorResponse converts an error to an ErrorResponse struct.
func ToErrorResponse(err error) *ErrorResponse {
	resp := &ErrorResponse{}

	switch e := err.(type) {
	case *NotFoundError:
		resp.Error = "resource not found"
		resp.Resource = e.Resource
		resp.ID = e.ID
		resp.StatusCode = e.StatusCode()
		resp.Hint = e.Hint()
	case *ConflictError:
		resp.Error = "resource already exists"
		resp.Resource = e.Resource
		resp.ID = e.ID
		resp.StatusCode = e.StatusCode()
		resp.Hint = e.Hint()
	case *ValidationError:
		resp.Error = "invalid request"
		resp.Detail = e.Message
		resp.Field = e.Field
		resp.StatusCode = e.StatusCode()
		resp.Hint = e.Hint()
	case *PayloadTooLargeError:
		resp.Error = "payload too large"
		resp.Detail = e.Error()
		resp.StatusCode = e.StatusCode()
		resp.Hint = e.Hint()
	case *CapacityError:
		resp.Error = "resource capacity exceeded"
		resp.Resource = e.Resource
		resp.Detail = e.Error()
		resp.StatusCode = e.StatusCode()
		resp.Hint = e.Hint()
	default:
		resp.Error = "internal error"
		resp.Detail = err.Error()
		resp.StatusCode = http.StatusInternalServerError
	}

	return resp
}
