package admin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
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
func (a *API) getMockStore() store.MockStore {
	if a.dataStore == nil {
		return nil
	}
	return a.dataStore.Mocks()
}

// MockFilter contains filter criteria for listing mocks in-memory.
type MockFilter struct {
	Type        string
	ParentID    string
	Enabled     *bool
	WorkspaceID string
}

// applyMockFilter filters mocks in-memory based on filter criteria.
func applyMockFilter(mocks []*mock.Mock, filter *MockFilter) []*mock.Mock {
	if filter == nil {
		return mocks
	}

	if filter.Type != "" {
		filtered := make([]*mock.Mock, 0, len(mocks))
		for _, m := range mocks {
			if m.Type == mock.Type(filter.Type) {
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
			mEnabled := m.Enabled == nil || *m.Enabled
			if mEnabled == *filter.Enabled {
				filtered = append(filtered, m)
			}
		}
		mocks = filtered
	}

	if filter.WorkspaceID != "" {
		filtered := make([]*mock.Mock, 0, len(mocks))
		for _, m := range mocks {
			if m.WorkspaceID == filter.WorkspaceID {
				filtered = append(filtered, m)
			}
		}
		mocks = filtered
	}

	return mocks
}

// getMockPort extracts the port from a mock if it uses a dedicated port (MQTT, gRPC).
// Returns 0 if the mock type doesn't use a dedicated port.
func getMockPort(m *mock.Mock) int {
	switch m.Type { //nolint:exhaustive // only MQTT and gRPC use dedicated ports
	case mock.TypeMQTT:
		if m.MQTT != nil {
			return m.MQTT.Port
		}
	case mock.TypeGRPC:
		if m.GRPC != nil {
			return m.GRPC.Port
		}
	}
	return 0
}

// PortConflict represents a port conflict between mocks.
type PortConflict struct {
	Port         int
	ConflictID   string
	ConflictName string
	ConflictType mock.Type
}

// PortCheckResult represents the result of checking port availability.
type PortCheckResult struct {
	// Conflict is set if there's a blocking conflict (cross-workspace or cross-protocol)
	Conflict *PortConflict
	// MergeTarget is set if the new mock should be merged into an existing mock
	// (same port, same protocol, same workspace)
	MergeTarget *mock.Mock
}

// isPortError checks if an error message indicates a port-related issue
// (port in use, permission denied, bind failed, etc.)
func isPortError(errMsg string) bool {
	errLower := strings.ToLower(errMsg)
	portIndicators := []string{
		"port",
		"address already in use",
		"bind:",
		"listen:",
		"eaddrinuse",
		"permission denied",
		"cannot assign requested address",
	}
	for _, indicator := range portIndicators {
		if strings.Contains(errLower, indicator) {
			return true
		}
	}
	return false
}

// checkPortAvailability checks port availability and returns merge opportunities.
// This is the richer version of checkPortConflict that also identifies merge targets.
func (a *API) checkPortAvailability(ctx context.Context, m *mock.Mock, excludeID string) *PortCheckResult {
	port := getMockPort(m)
	if port == 0 {
		return &PortCheckResult{} // Mock doesn't use a dedicated port
	}

	// Get all mocks to check for conflicts
	var allMocks []*mock.Mock
	var err error

	if engine := a.localEngine.Load(); engine != nil {
		allMocks, err = engine.ListMocks(ctx)
	} else if a.dataStore != nil {
		allMocks, err = a.dataStore.Mocks().List(ctx, nil)
	}

	if err != nil {
		a.logger().Warn("failed to list mocks for port conflict check", "error", err)
		return &PortCheckResult{} // Don't block on error, let runtime catch it
	}

	// Determine which workspaces to check for conflicts.
	// If the workspace is assigned to an engine, check all sibling workspaces on that engine.
	// Otherwise, only check within the same workspace.
	workspacesToCheck := []string{m.WorkspaceID}
	if a.engineRegistry != nil && m.WorkspaceID != "" {
		siblings := a.engineRegistry.GetSiblingWorkspaceIDs(m.WorkspaceID)
		if len(siblings) > 0 {
			workspacesToCheck = siblings
		}
	}

	// Build a set of workspaces to check
	workspaceSet := make(map[string]bool)
	for _, wsID := range workspacesToCheck {
		workspaceSet[wsID] = true
	}

	for _, existing := range allMocks {
		// Skip the mock being updated
		if existing.ID == excludeID {
			continue
		}

		// Only check mocks in relevant workspaces
		if !workspaceSet[existing.WorkspaceID] {
			continue
		}

		existingPort := getMockPort(existing)
		if existingPort == port {
			// Same port found - determine if it's a conflict or merge opportunity

			// Cross-protocol conflict: gRPC and MQTT can't share ports
			if existing.Type != m.Type {
				return &PortCheckResult{
					Conflict: &PortConflict{
						Port:         port,
						ConflictID:   existing.ID,
						ConflictName: existing.Name,
						ConflictType: existing.Type,
					},
				}
			}

			// Cross-workspace conflict: same port can't be used across workspaces on same engine
			if existing.WorkspaceID != m.WorkspaceID {
				return &PortCheckResult{
					Conflict: &PortConflict{
						Port:         port,
						ConflictID:   existing.ID,
						ConflictName: existing.Name,
						ConflictType: existing.Type,
					},
				}
			}

			// Same port + same protocol + same workspace = merge opportunity
			// gRPC: multiple services on one server
			// MQTT: multiple topics on one broker
			return &PortCheckResult{
				MergeTarget: existing,
			}
		}
	}

	return &PortCheckResult{}
}

// MergeResult contains information about a merge operation.
type MergeResult struct {
	TargetMockID    string   `json:"targetMockId"`
	Action          string   `json:"action"`                    // "merged" or "created"
	AddedServices   []string `json:"addedServices,omitempty"`   // For gRPC
	AddedTopics     []string `json:"addedTopics,omitempty"`     // For MQTT
	TotalServices   []string `json:"totalServices,omitempty"`   // For gRPC
	TotalTopics     []string `json:"totalTopics,omitempty"`     // For MQTT
	ServiceConflict string   `json:"serviceConflict,omitempty"` // If service/method already exists
}

// mergeGRPCMock merges a new gRPC mock's services into an existing mock.
// Returns the merge result or an error if there's a service/method conflict.
func mergeGRPCMock(target *mock.Mock, source *mock.Mock) (*MergeResult, error) {
	if target.GRPC == nil || source.GRPC == nil {
		return nil, errors.New("both mocks must have gRPC configuration")
	}

	result := &MergeResult{
		TargetMockID: target.ID,
		Action:       "merged",
	}

	// Initialize services map if nil
	if target.GRPC.Services == nil {
		target.GRPC.Services = make(map[string]mock.ServiceConfig)
	}

	// Check for conflicts and merge services
	for serviceName, serviceConfig := range source.GRPC.Services {
		if existingService, exists := target.GRPC.Services[serviceName]; exists {
			// Service exists - check for method conflicts
			if existingService.Methods == nil {
				existingService.Methods = make(map[string]mock.MethodConfig)
			}
			for methodName := range serviceConfig.Methods {
				if _, methodExists := existingService.Methods[methodName]; methodExists {
					return nil, fmt.Errorf("service '%s' method '%s' already exists on port %d",
						serviceName, methodName, target.GRPC.Port)
				}
			}
			// Merge methods into existing service
			for methodName, methodConfig := range serviceConfig.Methods {
				existingService.Methods[methodName] = methodConfig
				result.AddedServices = append(result.AddedServices, serviceName+"/"+methodName)
			}
			target.GRPC.Services[serviceName] = existingService
		} else {
			// New service - add entirely
			target.GRPC.Services[serviceName] = serviceConfig
			for methodName := range serviceConfig.Methods {
				result.AddedServices = append(result.AddedServices, serviceName+"/"+methodName)
			}
		}
	}

	// Merge proto files
	if source.GRPC.ProtoFile != "" && target.GRPC.ProtoFile != source.GRPC.ProtoFile {
		// Add to ProtoFiles list if different
		found := false
		for _, pf := range target.GRPC.ProtoFiles {
			if pf == source.GRPC.ProtoFile {
				found = true
				break
			}
		}
		if !found {
			if target.GRPC.ProtoFile != "" && len(target.GRPC.ProtoFiles) == 0 {
				target.GRPC.ProtoFiles = []string{target.GRPC.ProtoFile}
			}
			target.GRPC.ProtoFiles = append(target.GRPC.ProtoFiles, source.GRPC.ProtoFile)
		}
	}
	for _, pf := range source.GRPC.ProtoFiles {
		found := false
		for _, existing := range target.GRPC.ProtoFiles {
			if existing == pf {
				found = true
				break
			}
		}
		if !found {
			target.GRPC.ProtoFiles = append(target.GRPC.ProtoFiles, pf)
		}
	}

	// Merge import paths
	for _, ip := range source.GRPC.ImportPaths {
		found := false
		for _, existing := range target.GRPC.ImportPaths {
			if existing == ip {
				found = true
				break
			}
		}
		if !found {
			target.GRPC.ImportPaths = append(target.GRPC.ImportPaths, ip)
		}
	}

	// Collect total services
	for serviceName, svc := range target.GRPC.Services {
		for methodName := range svc.Methods {
			result.TotalServices = append(result.TotalServices, serviceName+"/"+methodName)
		}
	}

	return result, nil
}

// mergeMQTTMock merges a new MQTT mock's topics into an existing mock.
// Returns the merge result or an error if there's a topic conflict.
func mergeMQTTMock(target *mock.Mock, source *mock.Mock) (*MergeResult, error) {
	if target.MQTT == nil || source.MQTT == nil {
		return nil, errors.New("both mocks must have MQTT configuration")
	}

	result := &MergeResult{
		TargetMockID: target.ID,
		Action:       "merged",
	}

	// Check for topic conflicts and merge
	existingTopics := make(map[string]bool)
	for _, topic := range target.MQTT.Topics {
		existingTopics[topic.Topic] = true
	}

	for _, topic := range source.MQTT.Topics {
		if existingTopics[topic.Topic] {
			return nil, fmt.Errorf("topic '%s' already exists on port %d", topic.Topic, target.MQTT.Port)
		}
		target.MQTT.Topics = append(target.MQTT.Topics, topic)
		result.AddedTopics = append(result.AddedTopics, topic.Topic)
	}

	// Collect total topics
	for _, topic := range target.MQTT.Topics {
		result.TotalTopics = append(result.TotalTopics, topic.Topic)
	}

	return result, nil
}

// WorkspacePortConflict represents a port conflict when assigning a workspace to an engine.
type WorkspacePortConflict struct {
	Port              int    `json:"port"`
	MockID            string `json:"mockId"`
	MockName          string `json:"mockName"`
	ConflictMockID    string `json:"conflictMockId"`
	ConflictMockName  string `json:"conflictMockName"`
	ConflictWorkspace string `json:"conflictWorkspace"`
}

// checkWorkspaceEnginePortConflicts checks if assigning a workspace to an engine would create
// port conflicts with mocks in other workspaces already on that engine.
// Returns a list of conflicts (empty if none).
func (a *API) checkWorkspaceEnginePortConflicts(ctx context.Context, engineID, workspaceID string) []WorkspacePortConflict {
	var conflicts []WorkspacePortConflict

	// Get the engine to find other workspaces
	engine, err := a.engineRegistry.Get(engineID)
	if err != nil {
		return conflicts // Engine not found, no conflicts to report
	}

	// Get mocks from the workspace being assigned
	var allMocks []*mock.Mock
	if a.dataStore != nil {
		allMocks, err = a.dataStore.Mocks().List(ctx, &store.MockFilter{WorkspaceID: workspaceID})
		if err != nil {
			a.logger().Warn("failed to list mocks for workspace port conflict check", "error", err)
			return conflicts
		}
	}

	// Collect ports used by the new workspace's mocks
	newWorkspacePorts := make(map[int]*mock.Mock) // port -> mock using it
	for _, m := range allMocks {
		port := getMockPort(m)
		if port > 0 {
			newWorkspacePorts[port] = m
		}
	}

	if len(newWorkspacePorts) == 0 {
		return conflicts // No dedicated ports in new workspace
	}

	// Check against mocks in existing workspaces on this engine
	for _, ws := range engine.Workspaces {
		if ws.WorkspaceID == workspaceID {
			continue // Skip the workspace being added (in case of re-assignment)
		}

		// Get mocks from this sibling workspace
		var siblingMocks []*mock.Mock
		if a.dataStore != nil {
			siblingMocks, _ = a.dataStore.Mocks().List(ctx, &store.MockFilter{WorkspaceID: ws.WorkspaceID})
		}

		for _, sibling := range siblingMocks {
			siblingPort := getMockPort(sibling)
			if siblingPort > 0 {
				if newMock, exists := newWorkspacePorts[siblingPort]; exists {
					conflicts = append(conflicts, WorkspacePortConflict{
						Port:              siblingPort,
						MockID:            newMock.ID,
						MockName:          newMock.Name,
						ConflictMockID:    sibling.ID,
						ConflictMockName:  sibling.Name,
						ConflictWorkspace: ws.WorkspaceID,
					})
				}
			}
		}
	}

	return conflicts
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
		m.Enabled = &enabled
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
func (a *API) handleListUnifiedMocks(w http.ResponseWriter, r *http.Request) {
	// Query from engine if available (engine is the runtime data plane)
	// Fall back to dataStore for persistence-only scenarios
	if engine := a.localEngine.Load(); engine != nil {
		mocks, err := engine.ListMocks(r.Context())
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.logger(), "list mocks"))
			return
		}

		query := r.URL.Query()

		// Apply filters (engine returns all, we filter locally)
		filter := &MockFilter{
			Type:        query.Get("type"),
			ParentID:    query.Get("parentId"),
			WorkspaceID: query.Get("workspaceId"),
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
		filter.Type = mock.Type(t)
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
		a.logger().Error("failed to list mocks from store", "error", err)
		writeError(w, http.StatusInternalServerError, "list_error", ErrMsgInternalError)
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
func (a *API) handleGetUnifiedMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "missing mock id")
		return
	}

	// Query from engine if available (engine is the runtime data plane)
	if engine := a.localEngine.Load(); engine != nil {
		m, err := engine.GetMock(r.Context(), id)
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
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		a.logger().Error("failed to get mock from store", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "get_error", ErrMsgInternalError)
		return
	}

	writeJSON(w, http.StatusOK, m)
}

// handleCreateUnifiedMock creates a new mock.
// POST /mocks
func (a *API) handleCreateUnifiedMock(w http.ResponseWriter, r *http.Request) {
	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Unified mocks API requires persistent storage - coming soon")
		return
	}

	var m mock.Mock
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", sanitizeJSONError(err, a.logger()))
		return
	}

	// Validate required fields
	if m.Type == "" {
		writeError(w, http.StatusBadRequest, "missing_type", "type is required")
		return
	}

	// Generate ID if not provided (needed before Validate since ID is required)
	if m.ID == "" {
		m.ID = generateMockID(m.Type)
	}

	// Validate the full mock configuration
	if err := m.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	if engine := a.localEngine.Load(); engine != nil {
		// Check engine for duplicate ID (engine is the runtime truth)
		if existing, err := engine.GetMock(r.Context(), m.ID); err == nil && existing != nil {
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

	// Set workspaceId: use request body, then query param, then default
	if m.WorkspaceID == "" {
		m.WorkspaceID = r.URL.Query().Get("workspaceId")
	}
	if m.WorkspaceID == "" {
		m.WorkspaceID = store.DefaultWorkspaceID
	}

	// Check for port conflicts or merge opportunities
	portResult := a.checkPortAvailability(r.Context(), &m, "")

	// Handle blocking conflicts (cross-workspace or cross-protocol)
	if portResult.Conflict != nil {
		conflict := portResult.Conflict
		// Determine the type of conflict for better error message
		if conflict.ConflictType != m.Type {
			writeError(w, http.StatusConflict, "port_conflict",
				fmt.Sprintf("Port %d is in use by protocol '%s'. Different protocols cannot share ports.",
					conflict.Port, conflict.ConflictType))
		} else {
			conflictName := conflict.ConflictName
			if conflictName == "" {
				conflictName = conflict.ConflictID
			}
			writeError(w, http.StatusConflict, "port_conflict",
				fmt.Sprintf("Port %d is in use by workspace '%s'. Ports cannot be shared across workspaces.",
					conflict.Port, conflictName))
		}
		return
	}

	// Handle merge opportunity (same port, same protocol, same workspace)
	if portResult.MergeTarget != nil {
		a.handleMergeMock(w, r, &m, portResult.MergeTarget, mockStore)
		return
	}

	// No conflict, no merge - create new mock
	if err := mockStore.Create(r.Context(), &m); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			writeError(w, http.StatusConflict, "duplicate_id", "Mock with this ID already exists")
			return
		}
		a.logger().Error("failed to create mock in store", "id", m.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "create_error", ErrMsgInternalError)
		return
	}

	// Notify the engine so it can serve the mock (Admin = control plane, Engine = data plane)
	if engine := a.localEngine.Load(); engine != nil {
		// config.MockConfiguration is an alias for mock.Mock, so pass directly
		if _, err := engine.CreateMock(r.Context(), &m); err != nil {
			// Rollback the store operation - mock can't actually run
			if deleteErr := mockStore.Delete(r.Context(), m.ID); deleteErr != nil {
				a.logger().Warn("failed to rollback mock after engine error", "id", m.ID, "error", deleteErr)
			}

			a.logger().Error("failed to activate mock in engine", "id", m.ID, "error", err)
			errMsg := err.Error()
			if isPortError(errMsg) {
				writeError(w, http.StatusConflict, "port_unavailable",
					"Failed to start mock: the port may be in use by another process")
			} else {
				writeError(w, http.StatusServiceUnavailable, "engine_error", ErrMsgEngineUnavailable)
			}
			return
		}
	}

	// Return created response with action indicator
	response := map[string]interface{}{
		"id":      m.ID,
		"action":  "created",
		"message": fmt.Sprintf("Created %s mock", m.Type),
		"mock":    m,
	}
	writeJSON(w, http.StatusCreated, response)
}

// handleMergeMock handles merging a new mock's services/topics into an existing mock.
// This is called when creating a gRPC/MQTT mock on a port that already has a mock.
func (a *API) handleMergeMock(w http.ResponseWriter, r *http.Request, newMock *mock.Mock, target *mock.Mock, mockStore store.MockStore) {
	var mergeResult *MergeResult
	var err error

	switch newMock.Type {
	case mock.TypeGRPC:
		mergeResult, err = mergeGRPCMock(target, newMock)
		if err != nil {
			writeError(w, http.StatusConflict, "service_conflict", err.Error())
			return
		}
	case mock.TypeMQTT:
		mergeResult, err = mergeMQTTMock(target, newMock)
		if err != nil {
			writeError(w, http.StatusConflict, "topic_conflict", err.Error())
			return
		}
	default:
		// Non-mergeable protocol - this shouldn't happen based on checkPortAvailability
		writeError(w, http.StatusConflict, "port_conflict",
			fmt.Sprintf("Port %d is already in use and cannot be shared for protocol %s",
				getMockPort(newMock), newMock.Type))
		return
	}

	// Update the target mock in the store
	target.UpdatedAt = time.Now()
	if err := mockStore.Update(r.Context(), target); err != nil {
		a.logger().Error("failed to merge mock into store", "targetId", target.ID, "error", err)
		writeError(w, http.StatusInternalServerError, "update_error", ErrMsgInternalError)
		return
	}

	// Notify engine to reload the mock with new services/topics
	if engine := a.localEngine.Load(); engine != nil {
		if _, err := engine.UpdateMock(r.Context(), target.ID, target); err != nil {
			a.logger().Warn("failed to notify engine of merged mock", "id", target.ID, "error", err)
			// Don't fail - the mock is updated in store, engine will pick it up on next load
		}
	}

	// Build response
	port := getMockPort(target)
	var message string
	if newMock.Type == mock.TypeGRPC {
		message = fmt.Sprintf("Merged into existing gRPC server on port %d", port)
	} else {
		message = fmt.Sprintf("Merged into existing MQTT broker on port %d", port)
	}

	response := map[string]interface{}{
		"id":           target.ID,
		"action":       "merged",
		"message":      message,
		"targetMockId": target.ID,
		"mock":         target,
	}

	if len(mergeResult.AddedServices) > 0 {
		response["addedServices"] = mergeResult.AddedServices
		response["totalServices"] = mergeResult.TotalServices
	}
	if len(mergeResult.AddedTopics) > 0 {
		response["addedTopics"] = mergeResult.AddedTopics
		response["totalTopics"] = mergeResult.TotalTopics
	}

	writeJSON(w, http.StatusOK, response)
}

// handleUpdateUnifiedMock updates an existing mock (full replacement).
// PUT /mocks/{id}
func (a *API) handleUpdateUnifiedMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "missing mock id")
		return
	}

	var m mock.Mock
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", sanitizeJSONError(err, a.logger()))
		return
	}

	// Ensure ID matches path
	m.ID = id
	m.UpdatedAt = time.Now()

	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "Mock store is not available")
		return
	}

	// Get existing mock to preserve createdAt and workspaceID
	existing, err := mockStore.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		a.logger().Error("failed to get mock for update", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "get_error", ErrMsgInternalError)
		return
	}

	// Preserve createdAt and workspaceID if not provided
	m.CreatedAt = existing.CreatedAt
	if m.WorkspaceID == "" {
		m.WorkspaceID = existing.WorkspaceID
	}

	// Check for port conflicts (excluding this mock)
	// For updates, we don't support merging - any port collision is a conflict.
	portResult := a.checkPortAvailability(r.Context(), &m, id)
	if portResult.Conflict != nil {
		conflictName := portResult.Conflict.ConflictName
		if conflictName == "" {
			conflictName = portResult.Conflict.ConflictID
		}
		writeError(w, http.StatusConflict, "port_conflict",
			fmt.Sprintf("Port %d is already in use by '%s' (%s) in this workspace",
				portResult.Conflict.Port, conflictName, portResult.Conflict.ConflictType))
		return
	}
	// Treat merge targets as conflicts for updates (no auto-merging on update)
	if portResult.MergeTarget != nil {
		conflictName := portResult.MergeTarget.Name
		if conflictName == "" {
			conflictName = portResult.MergeTarget.ID
		}
		writeError(w, http.StatusConflict, "port_conflict",
			fmt.Sprintf("Port %d is already in use by '%s' (%s) in this workspace",
				getMockPort(&m), conflictName, portResult.MergeTarget.Type))
		return
	}

	// Persist to store (source of truth)
	if err := mockStore.Update(r.Context(), &m); err != nil {
		a.logger().Error("failed to update mock in store", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "update_error", ErrMsgInternalError)
		return
	}

	// Notify engine so it can update the running mock
	if engine := a.localEngine.Load(); engine != nil {
		if _, err := engine.UpdateMock(r.Context(), id, &m); err != nil {
			a.logger().Error("failed to update mock in engine", "id", m.ID, "error", err)
			errMsg := err.Error()
			// Check if this is a port-related error
			if isPortError(errMsg) {
				// Rollback the store operation - restore the existing mock
				if rollbackErr := mockStore.Update(r.Context(), existing); rollbackErr != nil {
					a.logger().Warn("failed to rollback mock update after engine error", "id", m.ID, "error", rollbackErr)
				}
				writeError(w, http.StatusConflict, "port_unavailable",
					"Failed to update mock: the port may be in use by another process")
				return
			}
			// For other errors, log but don't fail (already logged above)
		}
	}

	writeJSON(w, http.StatusOK, m)
}

// handlePatchUnifiedMock partially updates a mock.
// PATCH /mocks/{id}
func (a *API) handlePatchUnifiedMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "missing mock id")
		return
	}

	// Decode patch into a map first to see which fields are being updated
	var patch map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", sanitizeJSONError(err, a.logger()))
		return
	}

	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "Mock store is not available")
		return
	}

	// Get existing mock from store
	existing, err := mockStore.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		a.logger().Error("failed to get mock for patch", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "get_error", ErrMsgInternalError)
		return
	}

	// Apply patch to existing mock
	applyMockPatch(existing, patch)

	// Persist to store (source of truth)
	if err := mockStore.Update(r.Context(), existing); err != nil {
		a.logger().Error("failed to update mock in store (patch)", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "update_error", ErrMsgInternalError)
		return
	}

	// Notify engine so it can update the running mock
	if engine := a.localEngine.Load(); engine != nil {
		if _, err := engine.UpdateMock(r.Context(), id, existing); err != nil {
			a.logger().Warn("failed to notify engine of mock patch", "id", existing.ID, "error", err)
		}
	}

	writeJSON(w, http.StatusOK, existing)
}

// handleDeleteUnifiedMock deletes a mock by ID.
// DELETE /mocks/{id}
func (a *API) handleDeleteUnifiedMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "missing mock id")
		return
	}

	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusServiceUnavailable, "store_unavailable", "Mock store is not available")
		return
	}

	// Delete from store (source of truth)
	if err := mockStore.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		a.logger().Error("failed to delete mock from store", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "delete_error", ErrMsgInternalError)
		return
	}

	// Notify engine so it can stop serving the mock
	if engine := a.localEngine.Load(); engine != nil {
		if err := engine.DeleteMock(r.Context(), id); err != nil {
			a.logger().Warn("failed to notify engine of mock deletion", "id", id, "error", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteAllUnifiedMocks deletes all mocks, optionally filtered by type.
// DELETE /mocks?type=http
func (a *API) handleDeleteAllUnifiedMocks(w http.ResponseWriter, r *http.Request) {
	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Unified mocks API requires persistent storage - coming soon")
		return
	}

	mockType := mock.Type(r.URL.Query().Get("type"))

	var err error
	if mockType != "" {
		err = mockStore.DeleteByType(r.Context(), mockType)
	} else {
		err = mockStore.DeleteAll(r.Context())
	}

	if err != nil {
		a.logger().Error("failed to delete mocks from store", "type", mockType, "error", err)
		writeError(w, http.StatusInternalServerError, "delete_error", ErrMsgInternalError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleToggleUnifiedMock toggles the enabled state of a mock.
// POST /mocks/{id}/toggle
func (a *API) handleToggleUnifiedMock(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "missing mock id")
		return
	}

	// If engine is available, toggle there (engine is the runtime data plane)
	if engine := a.localEngine.Load(); engine != nil {
		// Get current state to determine new state
		existing, err := engine.GetMock(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}

		currentEnabled := existing.Enabled == nil || *existing.Enabled
		newEnabled := !currentEnabled
		updated, err := engine.ToggleMock(r.Context(), id, newEnabled)
		if err != nil {
			a.logger().Error("failed to toggle mock in engine", "id", id, "error", err)
			writeError(w, http.StatusInternalServerError, "toggle_error", ErrMsgInternalError)
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
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "mock not found")
			return
		}
		a.logger().Error("failed to get mock for toggle", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "get_error", ErrMsgInternalError)
		return
	}

	currentEnabled := m.Enabled == nil || *m.Enabled
	newEnabled := !currentEnabled
	m.Enabled = &newEnabled
	m.UpdatedAt = time.Now()

	if err := mockStore.Update(r.Context(), m); err != nil {
		a.logger().Error("failed to update mock toggle in store", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "update_error", ErrMsgInternalError)
		return
	}

	writeJSON(w, http.StatusOK, m)
}

// BulkPortConflict represents a port conflict found during bulk operations.
type BulkPortConflict struct {
	MockIndex    int       `json:"mockIndex"`
	MockID       string    `json:"mockId"`
	MockName     string    `json:"mockName"`
	Port         int       `json:"port"`
	ConflictWith string    `json:"conflictWith"` // "existing" or mock ID from the batch
	ConflictID   string    `json:"conflictId,omitempty"`
	ConflictName string    `json:"conflictName,omitempty"`
	ConflictType mock.Type `json:"conflictType,omitempty"`
}

// checkBulkPortConflicts validates port conflicts for a batch of mocks.
// It checks both conflicts within the batch and against existing mocks.
// Returns a list of conflicts if any are found.
//
// Simple rule: one port = one mock. No port sharing allowed, even for same protocol.
// If you want multiple topics on one MQTT broker, define them all in one mock config.
func (a *API) checkBulkPortConflicts(ctx context.Context, mocks []*mock.Mock) []BulkPortConflict {
	var conflicts []BulkPortConflict

	// Track ports used within the batch (grouped by workspace and port)
	// Key: "workspaceID:port" -> first mock using it
	type portUsage struct {
		index int
	}
	batchPorts := make(map[string]portUsage)

	for i, m := range mocks {
		port := getMockPort(m)
		if port == 0 {
			continue // Skip mocks without dedicated ports
		}

		key := fmt.Sprintf("%s:%d", m.WorkspaceID, port)

		// Check for conflict within the batch - any duplicate port is a conflict
		if firstUsage, exists := batchPorts[key]; exists {
			firstMock := mocks[firstUsage.index]

			conflicts = append(conflicts, BulkPortConflict{
				MockIndex:    i,
				MockID:       m.ID,
				MockName:     m.Name,
				Port:         port,
				ConflictWith: firstMock.ID,
				ConflictID:   firstMock.ID,
				ConflictName: firstMock.Name,
				ConflictType: firstMock.Type,
			})
			continue
		}

		// Check for conflict with existing mocks
		// For bulk creates, we don't support merging - any port collision is a conflict.
		// Use checkPortAvailability to detect both conflicts and merge targets.
		portResult := a.checkPortAvailability(ctx, m, "")
		if portResult.Conflict != nil {
			conflicts = append(conflicts, BulkPortConflict{
				MockIndex:    i,
				MockID:       m.ID,
				MockName:     m.Name,
				Port:         port,
				ConflictWith: "existing",
				ConflictID:   portResult.Conflict.ConflictID,
				ConflictName: portResult.Conflict.ConflictName,
				ConflictType: portResult.Conflict.ConflictType,
			})
			continue
		}
		// Treat merge targets as conflicts for bulk creates (no auto-merging)
		if portResult.MergeTarget != nil {
			conflicts = append(conflicts, BulkPortConflict{
				MockIndex:    i,
				MockID:       m.ID,
				MockName:     m.Name,
				Port:         port,
				ConflictWith: "existing",
				ConflictID:   portResult.MergeTarget.ID,
				ConflictName: portResult.MergeTarget.Name,
				ConflictType: portResult.MergeTarget.Type,
			})
			continue
		}

		// No conflict, record this port as used
		batchPorts[key] = portUsage{index: i}
	}

	return conflicts
}

// handleBulkCreateUnifiedMocks creates multiple mocks in a single request.
// POST /mocks/bulk
func (a *API) handleBulkCreateUnifiedMocks(w http.ResponseWriter, r *http.Request) {
	mockStore := a.getMockStore()
	if mockStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Unified mocks API requires persistent storage - coming soon")
		return
	}

	// Override the default body limit — bulk imports can be large.
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB

	var mocks []*mock.Mock
	if err := json.NewDecoder(r.Body).Decode(&mocks); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", sanitizeJSONError(err, a.logger()))
		return
	}

	// Check for duplicate IDs within the batch
	seenIDs := make(map[string]bool)
	for i, m := range mocks {
		if m.ID != "" {
			if seenIDs[m.ID] {
				writeError(w, http.StatusBadRequest, "duplicate_id",
					fmt.Sprintf("duplicate mock ID '%s' at index %d", m.ID, i))
				return
			}
			seenIDs[m.ID] = true
		}
	}

	now := time.Now()
	queryWorkspaceID := r.URL.Query().Get("workspaceId")
	for _, m := range mocks {
		if m.ID == "" {
			m.ID = generateMockID(m.Type)
		}
		m.CreatedAt = now
		m.UpdatedAt = now
		if m.MetaSortKey == 0 {
			m.MetaSortKey = float64(-now.UnixMilli())
		}
		// Set workspaceId from query param if not provided in body
		if m.WorkspaceID == "" && queryWorkspaceID != "" {
			m.WorkspaceID = queryWorkspaceID
		}
		// Default to "local" workspace if still not set
		if m.WorkspaceID == "" {
			m.WorkspaceID = store.DefaultWorkspaceID
		}
	}

	// If replace=true, delete existing mocks with matching IDs first.
	// This enables idempotent `mockd up` — re-running with the same config works.
	if r.URL.Query().Get("replace") == "true" {
		for _, m := range mocks {
			if m.ID != "" {
				_ = mockStore.Delete(r.Context(), m.ID) // Ignore not-found errors
				// Also remove from engine so it re-creates cleanly
				if engine := a.localEngine.Load(); engine != nil {
					_ = engine.DeleteMock(r.Context(), m.ID)
				}
			}
		}
	}

	// Check for port conflicts within the batch and against existing mocks
	if conflicts := a.checkBulkPortConflicts(r.Context(), mocks); len(conflicts) > 0 {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"error":     "port_conflict",
			"message":   fmt.Sprintf("Found %d port conflict(s) in the batch", len(conflicts)),
			"conflicts": conflicts,
		})
		return
	}

	if err := mockStore.BulkCreate(r.Context(), mocks); err != nil {
		if errors.Is(err, store.ErrAlreadyExists) {
			writeError(w, http.StatusConflict, "already_exists", "one or more mocks already exist")
			return
		}
		a.logger().Error("failed to bulk create mocks in store", "error", err)
		writeError(w, http.StatusInternalServerError, "bulk_create_error", ErrMsgInternalError)
		return
	}

	// Notify the engine for each mock (Admin = control plane, Engine = data plane)
	// Track any engine errors for reporting
	var engineErrors []string
	if engine := a.localEngine.Load(); engine != nil {
		for _, m := range mocks {
			if _, err := engine.CreateMock(r.Context(), m); err != nil {
				a.logger().Warn("failed to notify engine of bulk mock create", "id", m.ID, "error", err)
				if isPortError(err.Error()) {
					// Port error from engine - this shouldn't happen if our validation is correct
					// but handle it gracefully
					engineErrors = append(engineErrors, m.ID+": port may be in use by another process")
				}
			}
		}
	}

	response := map[string]interface{}{
		"created": len(mocks),
		"mocks":   mocks,
	}
	if len(engineErrors) > 0 {
		response["warnings"] = engineErrors
	}

	writeJSON(w, http.StatusCreated, response)
}

// generateMockID generates a unique ID for a mock based on its type.
func generateMockID(t mock.Type) string {
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
	return hex.EncodeToString(b)
}
