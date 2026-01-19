package admin

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
)

// ============================================================================
// Unified Mocks API Handlers
// These handlers provide a single API for all mock types (HTTP, WebSocket,
// GraphQL, gRPC, SOAP, MQTT) using the unified Mock type.
// ============================================================================

// MocksListResponse is the response for GET /mocks (unified API uses Total, legacy uses Count)
// Note: For backward compatibility with tests, we also include Count in the handlers
type MocksListResponse struct {
	Mocks []*mock.Mock `json:"mocks"`
	Total int          `json:"total"`
	Count int          `json:"count"`
}

// getMockStore returns the mock store to use.
func (a *AdminAPI) getMockStore() store.MockStore {
	if a.dataStore == nil {
		return nil
	}
	return a.dataStore.Mocks()
}

// MockFilter contains filter criteria for listing mocks in-memory.
type MockFilter struct {
	Type     string
	ParentID string
	Enabled  *bool
}

// applyMockFilter filters mocks in-memory based on filter criteria.
func applyMockFilter(mocks []*mock.Mock, filter *MockFilter) []*mock.Mock {
	if filter == nil {
		return mocks
	}

	if filter.Type != "" {
		filtered := make([]*mock.Mock, 0, len(mocks))
		for _, m := range mocks {
			if m.Type == mock.MockType(filter.Type) {
				filtered = append(filtered, m)
			}
		}
		mocks = filtered
	}

	if filter.ParentID != "" {
		filtered := make([]*mock.Mock, 0, len(mocks))
		for _, m := range mocks {
			if m.ParentID == filter.ParentID {
				filtered = append(filtered, m)
			}
		}
		mocks = filtered
	}

	if filter.Enabled != nil {
		filtered := make([]*mock.Mock, 0, len(mocks))
		for _, m := range mocks {
			if m.Enabled == *filter.Enabled {
				filtered = append(filtered, m)
			}
		}
		mocks = filtered
	}

	return mocks
}

// applyMockPatch applies common patch fields to a mock.
func applyMockPatch(m *mock.Mock, patch map[string]interface{}) {
	if name, ok := patch["name"].(string); ok {
		m.Name = name
	}
	if description, ok := patch["description"].(string); ok {
		m.Description = description
	}
	if enabled, ok := patch["enabled"].(bool); ok {
		m.Enabled = enabled
	}
	if parentID, ok := patch["parentId"].(string); ok {
		m.ParentID = parentID
	}
	if metaSortKey, ok := patch["metaSortKey"].(float64); ok {
		m.MetaSortKey = metaSortKey
	}
	m.UpdatedAt = time.Now()
}

// handleListUnifiedMocks returns all mocks with optional filtering.
// GET /mocks?type=http&parentId=folder123&enabled=true&search=user
func (a *AdminAPI) handleListUnifiedMocks(w http.ResponseWriter, r *http.Request) {
	// Query from engine if available (engine is the runtime data plane)
	// Fall back to dataStore for persistence-only scenarios
	if a.localEngine != nil {
		mocks, err := a.localEngine.ListMocks(r.Context())
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "engine_unavailable", "Failed to list mocks: "+err.Error())
			return
		}

		query := r.URL.Query()

		// Apply filters (engine returns all, we filter locally)
		filter := &MockFilter{
			Type:     query.Get("type"),
			ParentID: query.Get("parentId"),
		}
		if enabled := query.Get("enabled"); enabled != "" {
			b := enabled == "true"
			filter.Enabled = &b
		}
		mocks = applyMockFilter(mocks, filter)

		writeJSON(w, http.StatusOK, MocksListResponse{
			Mocks: mocks,
			Total: len(mocks),
			Count: len(mocks),
		})
		return
	}

	// Fallback to dataStore if no engine
	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Unified mocks API requires persistent storage or engine connection")
		return
	}

	query := r.URL.Query()

	filter := &store.MockFilter{}

	// Filter by type
	if t := query.Get("type"); t != "" {
		filter.Type = mock.MockType(t)
	}

	// Filter by parent folder
	if parentID := query.Get("parentId"); parentID != "" {
		filter.ParentID = &parentID
	} else if query.Has("parentId") {
		// Explicitly set to root level (empty string)
		empty := ""
		filter.ParentID = &empty
	}

	// Filter by enabled state
	if enabled := query.Get("enabled"); enabled != "" {
		b := enabled == "true"
		filter.Enabled = &b
	}

	// Filter by search query
	if search := query.Get("search"); search != "" {
		filter.Search = search
	}

	// Filter by workspace
	if wsID := query.Get("workspaceId"); wsID != "" {
		filter.WorkspaceID = wsID
	}

	mocks, err := mockStore.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, MocksListResponse{
		Mocks: mocks,
		Total: len(mocks),
		Count: len(mocks),
	})
}

// handleGetUnifiedMock returns a single mock by ID.
// GET /mocks/{id}
func (a *AdminAPI) handleGetUnifiedMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "missing mock id")
		return
	}

	// Query from engine if available (engine is the runtime data plane)
	if a.localEngine != nil {
		m, err := a.localEngine.GetMock(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		writeJSON(w, http.StatusOK, m)
		return
	}

	// Fallback to dataStore if no engine
	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Unified mocks API requires persistent storage or engine connection")
		return
	}

	m, err := mockStore.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, m)
}

// handleCreateUnifiedMock creates a new mock.
// POST /mocks
func (a *AdminAPI) handleCreateUnifiedMock(w http.ResponseWriter, r *http.Request) {
	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Unified mocks API requires persistent storage - coming soon")
		return
	}

	var m mock.Mock
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	// Validate required fields
	if m.Type == "" {
		writeError(w, http.StatusBadRequest, "missing_type", "type is required")
		return
	}

	// Generate ID if not provided
	if m.ID == "" {
		m.ID = generateMockID(m.Type)
	} else if a.localEngine != nil {
		// Check engine for duplicate ID (engine is the runtime truth)
		if existing, err := a.localEngine.GetMock(r.Context(), m.ID); err == nil && existing != nil {
			writeError(w, http.StatusConflict, "duplicate_id", "Mock with this ID already exists")
			return
		}
	}

	// Set timestamps
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now

	// Set default metaSortKey if not set (negative timestamp = newest first)
	if m.MetaSortKey == 0 {
		m.MetaSortKey = float64(-now.UnixMilli())
	}

	if err := mockStore.Create(r.Context(), &m); err != nil {
		if err == store.ErrAlreadyExists {
			writeError(w, http.StatusConflict, "duplicate_id", "Mock with this ID already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "create_error", err.Error())
		return
	}

	// Notify the engine so it can serve the mock (Admin = control plane, Engine = data plane)
	if a.localEngine != nil {
		// config.MockConfiguration is an alias for mock.Mock, so pass directly
		if _, err := a.localEngine.CreateMock(r.Context(), &m); err != nil {
			// Log but don't fail - the mock is stored, just not active yet
			a.log.Warn("failed to notify engine of new mock", "id", m.ID, "error", err)
		}
	}

	writeJSON(w, http.StatusCreated, m)
}

// handleUpdateUnifiedMock updates an existing mock (full replacement).
// PUT /mocks/{id}
func (a *AdminAPI) handleUpdateUnifiedMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "missing mock id")
		return
	}

	var m mock.Mock
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	// Ensure ID matches path
	m.ID = id
	m.UpdatedAt = time.Now()

	// If engine is available, update there (engine is the runtime data plane)
	if a.localEngine != nil {
		// Get existing to preserve createdAt
		existing, err := a.localEngine.GetMock(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		m.CreatedAt = existing.CreatedAt

		updated, err := a.localEngine.UpdateMock(r.Context(), id, &m)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "update_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}

	// Fallback to dataStore if no engine
	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Unified mocks API requires persistent storage or engine connection")
		return
	}

	// Get existing mock to preserve createdAt
	existing, err := mockStore.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_error", err.Error())
		return
	}

	// Preserve createdAt
	m.CreatedAt = existing.CreatedAt

	if err := mockStore.Update(r.Context(), &m); err != nil {
		writeError(w, http.StatusInternalServerError, "update_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, m)
}

// handlePatchUnifiedMock partially updates a mock.
// PATCH /mocks/{id}
func (a *AdminAPI) handlePatchUnifiedMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "missing mock id")
		return
	}

	// Decode patch into a map first to see which fields are being updated
	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "invalid request body: "+err.Error())
		return
	}

	// If engine is available, patch there (engine is the runtime data plane)
	if a.localEngine != nil {
		existing, err := a.localEngine.GetMock(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}

		applyMockPatch(existing, patch)

		updated, err := a.localEngine.UpdateMock(r.Context(), id, existing)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "update_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}

	// Fallback to dataStore if no engine
	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Unified mocks API requires persistent storage or engine connection")
		return
	}

	existing, err := mockStore.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_error", err.Error())
		return
	}

	applyMockPatch(existing, patch)

	if err := mockStore.Update(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "update_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

// handleDeleteUnifiedMock deletes a mock by ID.
// DELETE /mocks/{id}
func (a *AdminAPI) handleDeleteUnifiedMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "missing mock id")
		return
	}

	// If engine is available, delete there (engine is the runtime data plane)
	if a.localEngine != nil {
		if err := a.localEngine.DeleteMock(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Fallback to dataStore if no engine
	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Unified mocks API requires persistent storage or engine connection")
		return
	}

	if err := mockStore.Delete(r.Context(), id); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteAllUnifiedMocks deletes all mocks, optionally filtered by type.
// DELETE /mocks?type=http
func (a *AdminAPI) handleDeleteAllUnifiedMocks(w http.ResponseWriter, r *http.Request) {
	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Unified mocks API requires persistent storage - coming soon")
		return
	}

	mockType := mock.MockType(r.URL.Query().Get("type"))

	var err error
	if mockType != "" {
		err = mockStore.DeleteByType(r.Context(), mockType)
	} else {
		err = mockStore.DeleteAll(r.Context())
	}

	if err != nil {
		writeError(w, http.StatusInternalServerError, "delete_error", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleToggleUnifiedMock toggles the enabled state of a mock.
// POST /mocks/{id}/toggle
func (a *AdminAPI) handleToggleUnifiedMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "missing mock id")
		return
	}

	// If engine is available, toggle there (engine is the runtime data plane)
	if a.localEngine != nil {
		// Get current state to determine new state
		existing, err := a.localEngine.GetMock(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}

		newEnabled := !existing.Enabled
		updated, err := a.localEngine.ToggleMock(r.Context(), id, newEnabled)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "toggle_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, updated)
		return
	}

	// Fallback to dataStore if no engine
	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Unified mocks API requires persistent storage or engine connection")
		return
	}

	m, err := mockStore.Get(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_error", err.Error())
		return
	}

	m.Enabled = !m.Enabled
	m.UpdatedAt = time.Now()

	if err := mockStore.Update(r.Context(), m); err != nil {
		writeError(w, http.StatusInternalServerError, "update_error", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, m)
}

// handleBulkCreateUnifiedMocks creates multiple mocks in a single request.
// POST /mocks/bulk
func (a *AdminAPI) handleBulkCreateUnifiedMocks(w http.ResponseWriter, r *http.Request) {
	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Unified mocks API requires persistent storage - coming soon")
		return
	}

	var mocks []*mock.Mock
	if err := json.NewDecoder(r.Body).Decode(&mocks); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid request body: "+err.Error())
		return
	}

	now := time.Now()
	for _, m := range mocks {
		if m.ID == "" {
			m.ID = generateMockID(m.Type)
		}
		m.CreatedAt = now
		m.UpdatedAt = now
		if m.MetaSortKey == 0 {
			m.MetaSortKey = float64(-now.UnixMilli())
		}
	}

	if err := mockStore.BulkCreate(r.Context(), mocks); err != nil {
		if err == store.ErrAlreadyExists {
			writeError(w, http.StatusConflict, "already_exists", "one or more mocks already exist")
			return
		}
		writeError(w, http.StatusInternalServerError, "bulk_create_error", err.Error())
		return
	}

	// Notify the engine for each mock (Admin = control plane, Engine = data plane)
	if a.localEngine != nil {
		for _, m := range mocks {
			if _, err := a.localEngine.CreateMock(r.Context(), m); err != nil {
				a.log.Warn("failed to notify engine of bulk mock create", "id", m.ID, "error", err)
			}
		}
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"created": len(mocks),
		"mocks":   mocks,
	})
}

// generateMockID generates a unique ID for a mock based on its type.
func generateMockID(t mock.MockType) string {
	// Use type-prefixed IDs for easier identification
	prefix := string(t)
	if prefix == "" {
		prefix = "mock"
	}
	return prefix + "_" + generateShortID()
}

// generateShortID generates a short unique ID.
func generateShortID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}
