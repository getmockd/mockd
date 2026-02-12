// Package httputil provides shared HTTP utilities for consistent response handling.
//
// Design Decision (2026-02-11): This package intentionally exposes only WriteJSON
// and WriteNoContent. Both pkg/admin and pkg/engine/api define their own local
// writeJSON() and writeError() wrappers that use typed ErrorResponse structs
// (from pkg/api/types) instead of untyped maps. Convenience wrappers like
// WriteBadRequest, WriteNotFound, etc. were removed because (a) they encoded
// errors as map[string]string which is structurally incompatible with the typed
// pattern both consumers use, and (b) they had zero external callers.
//
// WriteJSON remains valuable: ~180 call sites across admin and engine avoid
// duplicating Content-Type, WriteHeader, and nil-check logic.
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

// WriteNoContent writes a 204 No Content response.
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}
