package engine

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/stateful"
)

func TestParseIntParam(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue int
		wantErr   bool
	}{
		{
			name:      "valid positive integer",
			input:     "42",
			wantValue: 42,
			wantErr:   false,
		},
		{
			name:      "zero",
			input:     "0",
			wantValue: 0,
			wantErr:   false,
		},
		{
			name:      "large number",
			input:     "999999",
			wantValue: 999999,
			wantErr:   false,
		},
		{
			name:      "negative number",
			input:     "-10",
			wantValue: -10,
			wantErr:   false,
		},
		{
			name:      "empty string",
			input:     "",
			wantValue: 0,
			wantErr:   true,
		},
		{
			name:      "non-numeric string",
			input:     "abc",
			wantValue: 0,
			wantErr:   true,
		},
		{
			name:      "mixed alphanumeric",
			input:     "123abc",
			wantValue: 0,
			wantErr:   true,
		},
		{
			name:      "decimal number",
			input:     "3.14",
			wantValue: 0,
			wantErr:   true,
		},
		{
			name:      "whitespace",
			input:     " 10 ",
			wantValue: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v int
			got, err := parseIntParam(tt.input, &v)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseIntParam() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got != tt.wantValue {
					t.Errorf("parseIntParam() returned = %v, want %v", got, tt.wantValue)
				}
				if v != tt.wantValue {
					t.Errorf("parseIntParam() set v = %v, want %v", v, tt.wantValue)
				}
			}
		})
	}
}

func TestHandleStatefulCreate_ConflictError(t *testing.T) {
	// This test verifies that handleStatefulCreate properly extracts the ID
	// from ConflictError instead of panicking on type assertion.

	// Create a handler with a logger
	h := &Handler{
		log: slog.Default(),
	}

	// Create a stateful resource with seed data via StateStore (which loads seed data)
	store := stateful.NewStateStore()
	cfg := &stateful.ResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "existing-user", "name": "John"},
		},
	}
	if err := store.Register(cfg); err != nil {
		t.Fatalf("Failed to register resource: %v", err)
	}
	resource := store.Get("users")

	// Test 1: Try to create a duplicate with explicit ID
	t.Run("duplicate with explicit ID", func(t *testing.T) {
		body := []byte(`{"id": "existing-user", "name": "Jane"}`)
		w := httptest.NewRecorder()

		status := h.handleStatefulCreate(w, resource, nil, body)

		if status != http.StatusConflict {
			t.Errorf("Expected status %d, got %d", http.StatusConflict, status)
		}

		var resp stateful.ErrorResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if resp.ID != "existing-user" {
			t.Errorf("Expected error response ID 'existing-user', got '%s'", resp.ID)
		}
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("Expected status code %d in response, got %d", http.StatusConflict, resp.StatusCode)
		}
	})

	// Test 2: Create without ID (should auto-generate and succeed)
	t.Run("create without ID succeeds", func(t *testing.T) {
		body := []byte(`{"name": "New User"}`)
		w := httptest.NewRecorder()

		status := h.handleStatefulCreate(w, resource, nil, body)

		if status != http.StatusCreated {
			t.Errorf("Expected status %d, got %d", http.StatusCreated, status)
		}
	})

	// Test 3: Verify no panic on conflict with auto-generated ID scenario
	// This is the bug we fixed - previously data["id"].(string) would panic
	// if ID wasn't in the request body
	t.Run("no panic on conflict error handling", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("handleStatefulCreate panicked: %v", r)
			}
		}()

		// Create a resource, then try to create another with same ID
		body := []byte(`{"id": "test-conflict", "name": "First"}`)
		w := httptest.NewRecorder()
		h.handleStatefulCreate(w, resource, nil, body)

		// Now try duplicate
		w = httptest.NewRecorder()
		h.handleStatefulCreate(w, resource, nil, body)

		// Should get conflict without panic
		if w.Code != http.StatusConflict {
			t.Errorf("Expected conflict status, got %d", w.Code)
		}
	})
}

func TestHandleStatefulCreate_InvalidJSON(t *testing.T) {
	h := &Handler{
		log: slog.Default(),
	}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	body := []byte(`{invalid json}`)
	w := httptest.NewRecorder()

	status := h.handleStatefulCreate(w, resource, nil, body)

	if status != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, status)
	}
}

func TestHandleStatefulCreate_BodyTooLarge(t *testing.T) {
	h := &Handler{
		log: slog.Default(),
	}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	// Create a body larger than MaxStatefulBodySize
	largeBody := bytes.Repeat([]byte("x"), MaxStatefulBodySize+1)
	w := httptest.NewRecorder()

	status := h.handleStatefulCreate(w, resource, nil, largeBody)

	if status != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status %d, got %d", http.StatusRequestEntityTooLarge, status)
	}
}

func TestHandleStatefulUpdate_NotFound(t *testing.T) {
	h := &Handler{
		log: slog.Default(),
	}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	body := []byte(`{"name": "Updated"}`)
	w := httptest.NewRecorder()

	status := h.handleStatefulUpdate(w, resource, "nonexistent-id", body)

	if status != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, status)
	}

	var resp stateful.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.ID != "nonexistent-id" {
		t.Errorf("Expected error response ID 'nonexistent-id', got '%s'", resp.ID)
	}
}

func TestHandleStatefulDelete_NotFound(t *testing.T) {
	h := &Handler{
		log: slog.Default(),
	}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	w := httptest.NewRecorder()

	status := h.handleStatefulDelete(w, resource, "nonexistent-id")

	if status != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, status)
	}
}

func TestHandleStatefulGet_NotFound(t *testing.T) {
	h := &Handler{
		log: slog.Default(),
	}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	w := httptest.NewRecorder()

	status := h.handleStatefulGet(w, resource, "nonexistent-id")

	if status != http.StatusNotFound {
		t.Errorf("Expected status %d, got %d", http.StatusNotFound, status)
	}
}

func TestHandleStateful_MethodRouting(t *testing.T) {
	h := &Handler{
		log: slog.Default(),
	}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "item-1", "name": "Test Item"},
		},
	}
	resource := stateful.NewStatefulResource(cfg)

	tests := []struct {
		name           string
		method         string
		itemID         string
		body           string
		expectedStatus int
	}{
		{
			name:           "GET item",
			method:         http.MethodGet,
			itemID:         "item-1",
			body:           "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "GET collection",
			method:         http.MethodGet,
			itemID:         "",
			body:           "",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "POST create",
			method:         http.MethodPost,
			itemID:         "",
			body:           `{"name": "New Item"}`,
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "PUT without ID returns error",
			method:         http.MethodPut,
			itemID:         "",
			body:           `{"name": "Updated"}`,
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "PUT with ID",
			method:         http.MethodPut,
			itemID:         "item-1",
			body:           `{"name": "Updated Item"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "DELETE without ID returns error",
			method:         http.MethodDelete,
			itemID:         "",
			body:           "",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "DELETE with ID",
			method:         http.MethodDelete,
			itemID:         "item-1",
			body:           "",
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "PATCH not allowed",
			method:         http.MethodPatch,
			itemID:         "item-1",
			body:           `{"name": "Patched"}`,
			expectedStatus: http.StatusMethodNotAllowed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset resource for each test
			resource.Reset()

			w := httptest.NewRecorder()
			req := httptest.NewRequest(tt.method, "/api/items", bytes.NewBufferString(tt.body))

			status := h.handleStateful(w, req, resource, tt.itemID, nil, []byte(tt.body))

			if status != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, status)
			}
		})
	}
}
