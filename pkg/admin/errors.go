// Error handling utilities for the admin API.
// This file provides error sanitization to prevent information leakage.

package admin

import (
	"errors"
	"log/slog"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

// Safe error messages for client responses.
// These messages are generic enough to not leak internal details.
const (
	// ErrMsgEngineUnavailable is returned when the engine cannot be reached.
	ErrMsgEngineUnavailable = "Mock engine is temporarily unavailable"

	// ErrMsgInternalError is returned for unexpected internal errors.
	ErrMsgInternalError = "An internal error occurred"

	// ErrMsgInvalidJSON is returned for JSON parsing errors.
	ErrMsgInvalidJSON = "Invalid JSON in request body"

	// ErrMsgInvalidRequest is returned for malformed requests.
	ErrMsgInvalidRequest = "Invalid request format"

	// ErrMsgOperationFailed is returned for generic operation failures.
	ErrMsgOperationFailed = "Operation failed"

	// ErrMsgValidationFailed is returned for validation errors.
	ErrMsgValidationFailed = "Request validation failed"

	// ErrMsgNotFound is returned when a resource is not found.
	ErrMsgNotFound = "Resource not found"

	// ErrMsgConflict is returned for duplicate resource conflicts.
	ErrMsgConflict = "Resource already exists"
)

// sanitizeError returns a safe error message for client responses.
// The full error is logged server-side for debugging, but only a
// generic message is returned to prevent information leakage.
//
// Parameters:
//   - err: The original error (logged server-side)
//   - log: Logger for server-side error logging (can be nil)
//   - operation: Human-readable description of the operation (e.g., "get mock")
//   - details: Additional context for logging (e.g., mock ID)
//
// Returns a sanitized message safe for client responses.
func sanitizeError(err error, log *slog.Logger, operation string, details ...any) string {
	// Log full error details server-side
	if log != nil {
		args := []any{"operation", operation, "error", err}
		args = append(args, details...)
		log.Error("operation failed", args...)
	}

	// Check for known error types and return appropriate messages
	if errors.Is(err, engineclient.ErrNotFound) {
		return ErrMsgNotFound
	}
	if errors.Is(err, engineclient.ErrDuplicate) {
		return ErrMsgConflict
	}

	// Return generic message to client
	return ErrMsgOperationFailed
}

// sanitizeEngineError returns a safe error message for engine-related errors.
func sanitizeEngineError(err error, log *slog.Logger, operation string) string {
	if log != nil {
		log.Error("engine operation failed", "operation", operation, "error", err)
	}
	return ErrMsgEngineUnavailable
}

// sanitizeValidationError returns a safe error message for validation errors.
// Unlike other errors, validation errors may include some details about what
// failed validation, as this information is useful for the client and doesn't
// expose internal implementation details.
func sanitizeValidationError(err error, log *slog.Logger) string {
	if log != nil {
		log.Warn("validation failed", "error", err)
	}
	// For validation errors, we can be slightly more specific
	// but still avoid exposing internal details
	return ErrMsgValidationFailed
}

// sanitizeJSONError returns a safe error message for JSON parsing errors.
func sanitizeJSONError(err error, log *slog.Logger) string {
	if log != nil {
		log.Debug("JSON parsing failed", "error", err)
	}
	return ErrMsgInvalidJSON
}

// logAndSanitize is a convenience function that logs an error and returns
// a sanitized message. Use this for one-line error handling.
func logAndSanitize(log *slog.Logger, err error, operation string) string {
	if log != nil {
		log.Error(operation+" failed", "error", err)
	}
	return ErrMsgOperationFailed
}
