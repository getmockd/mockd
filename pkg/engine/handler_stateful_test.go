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
		req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))

		status := h.handleStatefulCreate(w, req, resource, nil, body)

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
		req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))

		status := h.handleStatefulCreate(w, req, resource, nil, body)

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
		req := httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
		h.handleStatefulCreate(w, req, resource, nil, body)

		// Now try duplicate
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPost, "/api/users", bytes.NewReader(body))
		h.handleStatefulCreate(w, req, resource, nil, body)

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
	req := httptest.NewRequest(http.MethodPost, "/api/items", bytes.NewReader(body))

	status := h.handleStatefulCreate(w, req, resource, nil, body)

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
	req := httptest.NewRequest(http.MethodPost, "/api/items", bytes.NewReader(largeBody))

	status := h.handleStatefulCreate(w, req, resource, nil, largeBody)

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
	req := httptest.NewRequest(http.MethodPut, "/api/items/nonexistent-id", bytes.NewReader(body))

	status := h.handleStatefulUpdate(w, req, resource, "nonexistent-id", nil, body)

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
			name:           "PATCH with ID",
			method:         http.MethodPatch,
			itemID:         "item-1",
			body:           `{"name": "Patched"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "PATCH without ID returns error",
			method:         http.MethodPatch,
			itemID:         "",
			body:           `{"name": "Patched"}`,
			expectedStatus: http.StatusBadRequest,
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

// ── Full CRUD lifecycle ──────────────────────────────────────────────────────

func TestHandleStateful_FullCRUDLifecycle(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "products",
		BasePath: "/api/products",
	}
	resource := stateful.NewStatefulResource(cfg)

	// 1. Create
	createBody := []byte(`{"id": "prod-1", "name": "Widget", "price": 9.99}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/products", bytes.NewReader(createBody))
	status := h.handleStatefulCreate(w, req, resource, nil, createBody)

	if status != http.StatusCreated {
		t.Fatalf("Create: expected %d, got %d", http.StatusCreated, status)
	}

	var created map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&created); err != nil {
		t.Fatalf("Create decode: %v", err)
	}
	if created["id"] != "prod-1" {
		t.Errorf("Create: expected id 'prod-1', got %v", created["id"])
	}
	if created["name"] != "Widget" {
		t.Errorf("Create: expected name 'Widget', got %v", created["name"])
	}
	if _, ok := created["createdAt"]; !ok {
		t.Error("Create: response should contain createdAt")
	}
	if _, ok := created["updatedAt"]; !ok {
		t.Error("Create: response should contain updatedAt")
	}

	// 2. Get
	w = httptest.NewRecorder()
	status = h.handleStatefulGet(w, resource, "prod-1")

	if status != http.StatusOK {
		t.Fatalf("Get: expected %d, got %d", http.StatusOK, status)
	}

	var fetched map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&fetched); err != nil {
		t.Fatalf("Get decode: %v", err)
	}
	if fetched["name"] != "Widget" {
		t.Errorf("Get: expected name 'Widget', got %v", fetched["name"])
	}

	// 3. List (should have 1 item)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/products", nil)
	status = h.handleStatefulList(w, req, resource, nil)

	if status != http.StatusOK {
		t.Fatalf("List: expected %d, got %d", http.StatusOK, status)
	}

	var listResp stateful.PaginatedResponse
	if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
		t.Fatalf("List decode: %v", err)
	}
	if listResp.Meta.Total != 1 {
		t.Errorf("List: expected total 1, got %d", listResp.Meta.Total)
	}
	if listResp.Meta.Count != 1 {
		t.Errorf("List: expected count 1, got %d", listResp.Meta.Count)
	}

	// 4. Update (PUT)
	updateBody := []byte(`{"name": "Super Widget", "price": 19.99}`)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/products/prod-1", bytes.NewReader(updateBody))
	status = h.handleStatefulUpdate(w, req, resource, "prod-1", nil, updateBody)

	if status != http.StatusOK {
		t.Fatalf("Update: expected %d, got %d", http.StatusOK, status)
	}

	var updated map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&updated); err != nil {
		t.Fatalf("Update decode: %v", err)
	}
	if updated["name"] != "Super Widget" {
		t.Errorf("Update: expected name 'Super Widget', got %v", updated["name"])
	}

	// 5. Delete
	w = httptest.NewRecorder()
	status = h.handleStatefulDelete(w, resource, "prod-1")

	if status != http.StatusNoContent {
		t.Fatalf("Delete: expected %d, got %d", http.StatusNoContent, status)
	}

	// 6. Verify deleted — Get should 404
	w = httptest.NewRecorder()
	status = h.handleStatefulGet(w, resource, "prod-1")

	if status != http.StatusNotFound {
		t.Errorf("Get after delete: expected %d, got %d", http.StatusNotFound, status)
	}
}

// ── Capacity limit tests ─────────────────────────────────────────────────────

func TestHandleStatefulCreate_CapacityLimit(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "limited",
		BasePath: "/api/limited",
		MaxItems: 2,
	}
	resource := stateful.NewStatefulResource(cfg)

	// Create 2 items (at capacity)
	for i := 0; i < 2; i++ {
		body := []byte(`{"name": "item"}`)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/limited", bytes.NewReader(body))
		status := h.handleStatefulCreate(w, req, resource, nil, body)
		if status != http.StatusCreated {
			t.Fatalf("Create %d: expected %d, got %d", i, http.StatusCreated, status)
		}
	}

	// Third create should fail with 507
	body := []byte(`{"name": "one too many"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/limited", bytes.NewReader(body))
	status := h.handleStatefulCreate(w, req, resource, nil, body)

	if status != http.StatusInsufficientStorage {
		t.Errorf("Expected %d for capacity exceeded, got %d", http.StatusInsufficientStorage, status)
	}

	var resp stateful.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode error response: %v", err)
	}
	if resp.Hint == "" {
		t.Error("Capacity error response should include a hint")
	}
	if resp.Resource != "limited" {
		t.Errorf("Error response resource: expected 'limited', got %q", resp.Resource)
	}
}

// ── Method not allowed ───────────────────────────────────────────────────────

func TestHandleStateful_MethodNotAllowed(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("OPTIONS", "/api/items", nil)
	status := h.handleStateful(w, req, resource, "", nil, nil)

	if status != http.StatusMethodNotAllowed {
		t.Errorf("Expected %d, got %d", http.StatusMethodNotAllowed, status)
	}
}

// ── Update/Patch edge cases ──────────────────────────────────────────────────

func TestHandleStatefulUpdate_BodyTooLarge(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "item-1", "name": "Original"},
		},
	}
	store := stateful.NewStateStore()
	if err := store.Register(cfg); err != nil {
		t.Fatalf("Register: %v", err)
	}
	resource := store.Get("items")

	largeBody := bytes.Repeat([]byte("x"), MaxStatefulBodySize+1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/items/item-1", bytes.NewReader(largeBody))
	status := h.handleStatefulUpdate(w, req, resource, "item-1", nil, largeBody)

	if status != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected %d, got %d", http.StatusRequestEntityTooLarge, status)
	}
}

func TestHandleStatefulUpdate_InvalidJSON(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "item-1", "name": "Original"},
		},
	}
	store := stateful.NewStateStore()
	if err := store.Register(cfg); err != nil {
		t.Fatalf("Register: %v", err)
	}
	resource := store.Get("items")

	body := []byte(`{bad json}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/items/item-1", bytes.NewReader(body))
	status := h.handleStatefulUpdate(w, req, resource, "item-1", nil, body)

	if status != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, status)
	}
}

func TestHandleStatefulPatch_NotFound(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	body := []byte(`{"name": "Patched"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/items/ghost", bytes.NewReader(body))
	status := h.handleStatefulPatch(w, req, resource, "ghost", nil, body)

	if status != http.StatusNotFound {
		t.Errorf("Expected %d, got %d", http.StatusNotFound, status)
	}
}

func TestHandleStatefulPatch_InvalidJSON(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "item-1", "name": "Original"},
		},
	}
	store := stateful.NewStateStore()
	if err := store.Register(cfg); err != nil {
		t.Fatalf("Register: %v", err)
	}
	resource := store.Get("items")

	body := []byte(`not json`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/items/item-1", bytes.NewReader(body))
	status := h.handleStatefulPatch(w, req, resource, "item-1", nil, body)

	if status != http.StatusBadRequest {
		t.Errorf("Expected %d, got %d", http.StatusBadRequest, status)
	}
}

func TestHandleStatefulPatch_BodyTooLarge(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "item-1", "name": "Original"},
		},
	}
	store := stateful.NewStateStore()
	if err := store.Register(cfg); err != nil {
		t.Fatalf("Register: %v", err)
	}
	resource := store.Get("items")

	largeBody := bytes.Repeat([]byte("x"), MaxStatefulBodySize+1)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/items/item-1", bytes.NewReader(largeBody))
	status := h.handleStatefulPatch(w, req, resource, "item-1", nil, largeBody)

	if status != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected %d, got %d", http.StatusRequestEntityTooLarge, status)
	}
}

func TestHandleStatefulPatch_MergesFields(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "item-1", "name": "Original", "color": "blue"},
		},
	}
	store := stateful.NewStateStore()
	if err := store.Register(cfg); err != nil {
		t.Fatalf("Register: %v", err)
	}
	resource := store.Get("items")

	// Patch only the name — color should be preserved
	body := []byte(`{"name": "Patched"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPatch, "/api/items/item-1", bytes.NewReader(body))
	status := h.handleStatefulPatch(w, req, resource, "item-1", nil, body)

	if status != http.StatusOK {
		t.Fatalf("Patch: expected %d, got %d", http.StatusOK, status)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if resp["name"] != "Patched" {
		t.Errorf("Expected patched name, got %v", resp["name"])
	}
	if resp["color"] != "blue" {
		t.Errorf("Expected color 'blue' to be preserved, got %v", resp["color"])
	}
}

// ── Error response format ────────────────────────────────────────────────────

func TestWriteStatefulError_ResponseBody(t *testing.T) {
	h := &Handler{log: slog.Default()}

	w := httptest.NewRecorder()
	status := h.writeStatefulError(w, http.StatusNotFound, "resource not found", "users", "user-42")

	if status != http.StatusNotFound {
		t.Errorf("Expected %d, got %d", http.StatusNotFound, status)
	}

	var resp stateful.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if resp.Error != "resource not found" {
		t.Errorf("Error: got %q", resp.Error)
	}
	if resp.Resource != "users" {
		t.Errorf("Resource: got %q", resp.Resource)
	}
	if resp.ID != "user-42" {
		t.Errorf("ID: got %q", resp.ID)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode: got %d", resp.StatusCode)
	}
}

func TestWriteStatefulErrorWithHint_ResponseBody(t *testing.T) {
	h := &Handler{log: slog.Default()}

	w := httptest.NewRecorder()
	status := h.writeStatefulErrorWithHint(w, http.StatusInsufficientStorage, "capacity exceeded", "products", "", "Delete some items")

	if status != http.StatusInsufficientStorage {
		t.Errorf("Expected %d, got %d", http.StatusInsufficientStorage, status)
	}

	var resp stateful.ErrorResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if resp.Hint != "Delete some items" {
		t.Errorf("Hint: got %q", resp.Hint)
	}
}

// ── Query filter parsing ─────────────────────────────────────────────────────

func TestParseQueryFilter_Defaults(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	if filter.Limit != 100 {
		t.Errorf("Default limit: expected 100, got %d", filter.Limit)
	}
	if filter.Offset != 0 {
		t.Errorf("Default offset: expected 0, got %d", filter.Offset)
	}
}

func TestParseQueryFilter_WithParams(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=25&offset=10&sort=name&order=asc&status=active", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	if filter.Limit != 25 {
		t.Errorf("Limit: expected 25, got %d", filter.Limit)
	}
	if filter.Offset != 10 {
		t.Errorf("Offset: expected 10, got %d", filter.Offset)
	}
	if filter.Sort != "name" {
		t.Errorf("Sort: expected 'name', got %q", filter.Sort)
	}
	if filter.Order != "asc" {
		t.Errorf("Order: expected 'asc', got %q", filter.Order)
	}
	if filter.Filters["status"] != "active" {
		t.Errorf("Custom filter 'status': expected 'active', got %q", filter.Filters["status"])
	}
}

func TestParseQueryFilter_InvalidLimitIgnored(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=abc&offset=-5", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	// Invalid limit should use default
	if filter.Limit != 100 {
		t.Errorf("Invalid limit should default to 100, got %d", filter.Limit)
	}
}

func TestParseQueryFilter_ZeroLimitIgnored(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=0", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	// limit=0 is not > 0, so default applies
	if filter.Limit != 100 {
		t.Errorf("Zero limit should default to 100, got %d", filter.Limit)
	}
}

func TestParseQueryFilter_WithParentField(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:        "comments",
		BasePath:    "/api/posts/:postId/comments",
		ParentField: "postId",
	}
	resource := stateful.NewStatefulResource(cfg)

	pathParams := map[string]string{"postId": "post-99"}
	req := httptest.NewRequest(http.MethodGet, "/api/posts/post-99/comments", nil)
	filter := h.parseQueryFilter(req, resource, pathParams)

	if filter.ParentField != "postId" {
		t.Errorf("ParentField: expected 'postId', got %q", filter.ParentField)
	}
	if filter.ParentID != "post-99" {
		t.Errorf("ParentID: expected 'post-99', got %q", filter.ParentID)
	}
}

func TestParseQueryFilter_ReservedKeysExcludedFromFilters(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=10&offset=5&sort=name&order=asc&category=books", nil)
	filter := h.parseQueryFilter(req, resource, nil)

	// Reserved keys should not appear in Filters map
	for _, reserved := range []string{"limit", "offset", "sort", "order"} {
		if _, ok := filter.Filters[reserved]; ok {
			t.Errorf("Reserved key %q should not be in Filters map", reserved)
		}
	}

	// Custom query params should be in Filters
	if filter.Filters["category"] != "books" {
		t.Errorf("Custom filter 'category': expected 'books', got %q", filter.Filters["category"])
	}
}

// ── List with query parameters (end-to-end through handler) ──────────────────

func TestHandleStatefulList_Pagination(t *testing.T) {
	h := &Handler{log: slog.Default()}

	seeds := make([]map[string]interface{}, 10)
	for i := 0; i < 10; i++ {
		seeds[i] = map[string]interface{}{
			"id":   "item-" + string(rune('a'+i)),
			"name": "Item " + string(rune('A'+i)),
		}
	}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: seeds,
	}
	store := stateful.NewStateStore()
	if err := store.Register(cfg); err != nil {
		t.Fatalf("Register: %v", err)
	}
	resource := store.Get("items")

	// Get page with limit=3&offset=0
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/items?limit=3&offset=0", nil)
	status := h.handleStatefulList(w, req, resource, nil)

	if status != http.StatusOK {
		t.Fatalf("List: expected %d, got %d", http.StatusOK, status)
	}

	var resp stateful.PaginatedResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if resp.Meta.Total != 10 {
		t.Errorf("Total: expected 10, got %d", resp.Meta.Total)
	}
	if resp.Meta.Count != 3 {
		t.Errorf("Count: expected 3, got %d", resp.Meta.Count)
	}
	if resp.Meta.Limit != 3 {
		t.Errorf("Limit in meta: expected 3, got %d", resp.Meta.Limit)
	}
	if len(resp.Data) != 3 {
		t.Errorf("Data length: expected 3, got %d", len(resp.Data))
	}
}

func TestHandleStatefulList_EmptyCollection(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "empty",
		BasePath: "/api/empty",
	}
	resource := stateful.NewStatefulResource(cfg)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/empty", nil)
	status := h.handleStatefulList(w, req, resource, nil)

	if status != http.StatusOK {
		t.Fatalf("List empty: expected %d, got %d", http.StatusOK, status)
	}

	var resp stateful.PaginatedResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if resp.Meta.Total != 0 {
		t.Errorf("Total: expected 0, got %d", resp.Meta.Total)
	}
	if len(resp.Data) != 0 {
		t.Errorf("Data: expected empty, got %d items", len(resp.Data))
	}
}

// ── Get success with response body validation ────────────────────────────────

func TestHandleStatefulGet_Success_ResponseContainsAllFields(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "item-1", "name": "Test", "tags": []interface{}{"a", "b"}},
		},
	}
	store := stateful.NewStateStore()
	if err := store.Register(cfg); err != nil {
		t.Fatalf("Register: %v", err)
	}
	resource := store.Get("items")

	w := httptest.NewRecorder()
	status := h.handleStatefulGet(w, resource, "item-1")

	if status != http.StatusOK {
		t.Fatalf("Get: expected %d, got %d", http.StatusOK, status)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	if resp["id"] != "item-1" {
		t.Errorf("id: got %v", resp["id"])
	}
	if resp["name"] != "Test" {
		t.Errorf("name: got %v", resp["name"])
	}
	if resp["createdAt"] == nil {
		t.Error("createdAt should be present")
	}
	if resp["updatedAt"] == nil {
		t.Error("updatedAt should be present")
	}

	// Note: Content-Type is set by the parent handleStateful dispatcher,
	// not by handleStatefulGet directly, so we skip that check here.
}

// ── Delete success ───────────────────────────────────────────────────────────

func TestHandleStatefulDelete_Success_NoResponseBody(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "item-1", "name": "Doomed"},
		},
	}
	store := stateful.NewStateStore()
	if err := store.Register(cfg); err != nil {
		t.Fatalf("Register: %v", err)
	}
	resource := store.Get("items")

	w := httptest.NewRecorder()
	status := h.handleStatefulDelete(w, resource, "item-1")

	if status != http.StatusNoContent {
		t.Errorf("Delete: expected %d, got %d", http.StatusNoContent, status)
	}

	// 204 should have no response body
	if w.Body.Len() != 0 {
		t.Errorf("Delete should return empty body, got %d bytes", w.Body.Len())
	}

	// Verify item is gone
	if resource.Get("item-1") != nil {
		t.Error("Item should have been deleted")
	}
}

// ── Create auto-generates ID ─────────────────────────────────────────────────

func TestHandleStatefulCreate_AutoID(t *testing.T) {
	h := &Handler{log: slog.Default()}

	cfg := &stateful.ResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	}
	resource := stateful.NewStatefulResource(cfg)

	body := []byte(`{"name": "No ID Provided"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/items", bytes.NewReader(body))
	status := h.handleStatefulCreate(w, req, resource, nil, body)

	if status != http.StatusCreated {
		t.Fatalf("Create: expected %d, got %d", http.StatusCreated, status)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Decode: %v", err)
	}

	id, ok := resp["id"].(string)
	if !ok || id == "" {
		t.Error("Create without explicit ID should auto-generate a UUID")
	}
}
