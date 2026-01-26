// Package httputil provides shared HTTP utilities for consistent response handling.
package httputil

import (
	"encoding/json"
	"net/http"
)

// WriteJSON writes a JSON response with the given status code.
// It sets the Content-Type header to application/json.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		_ = json.NewEncoder(w).Encode(data)
	}
}

// WriteError writes a JSON error response with the given status code.
// The error response includes an error code and a human-readable message.
func WriteError(w http.ResponseWriter, status int, errCode, message string) {
	WriteJSON(w, status, map[string]string{
		"error":   errCode,
		"message": message,
	})
}

// WriteErrorWithDetails writes a JSON error response with additional details.
// Useful for validation errors that need to include field-specific information.
func WriteErrorWithDetails(w http.ResponseWriter, status int, errCode, message string, details any) {
	WriteJSON(w, status, map[string]any{
		"error":   errCode,
		"message": message,
		"details": details,
	})
}

// WriteSuccess writes a success response with optional data.
// If data is nil, only the status code is written.
func WriteSuccess(w http.ResponseWriter, status int, data any) {
	WriteJSON(w, status, data)
}

// WriteNoContent writes a 204 No Content response.
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// WriteCreated writes a 201 Created response with the created resource.
func WriteCreated(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusCreated, data)
}

// WriteOK writes a 200 OK response with data.
func WriteOK(w http.ResponseWriter, data any) {
	WriteJSON(w, http.StatusOK, data)
}

// WriteBadRequest writes a 400 Bad Request error response.
func WriteBadRequest(w http.ResponseWriter, errCode, message string) {
	WriteError(w, http.StatusBadRequest, errCode, message)
}

// WriteNotFound writes a 404 Not Found error response.
func WriteNotFound(w http.ResponseWriter, errCode, message string) {
	WriteError(w, http.StatusNotFound, errCode, message)
}

// WriteInternalError writes a 500 Internal Server Error response.
func WriteInternalError(w http.ResponseWriter, errCode, message string) {
	WriteError(w, http.StatusInternalServerError, errCode, message)
}

// WriteServiceUnavailable writes a 503 Service Unavailable response.
func WriteServiceUnavailable(w http.ResponseWriter, errCode, message string) {
	WriteError(w, http.StatusServiceUnavailable, errCode, message)
}

// WriteConflict writes a 409 Conflict response.
func WriteConflict(w http.ResponseWriter, errCode, message string) {
	WriteError(w, http.StatusConflict, errCode, message)
}

// WriteTooManyRequests writes a 429 Too Many Requests response.
func WriteTooManyRequests(w http.ResponseWriter, errCode, message string) {
	WriteError(w, http.StatusTooManyRequests, errCode, message)
}
