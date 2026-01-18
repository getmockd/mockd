// Health and readiness probe handlers for the mock engine.

package engine

import (
	"encoding/json"
	"net/http"
	"time"
)

// handleHealth handles the liveness probe endpoint.
func (h *Handler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	response := map[string]string{"status": "healthy", "timestamp": time.Now().UTC().Format(time.RFC3339)}
	json.NewEncoder(w).Encode(response)
}

// handleReady handles the readiness probe endpoint.
func (h *Handler) handleReady(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	mockCount := len(h.store.List())
	response := map[string]any{"status": "ready", "checks": map[string]any{"mocks": map[string]any{"count": mockCount, "status": "ok"}}}
	json.NewEncoder(w).Encode(response)
}
