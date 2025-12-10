package admin

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/getmockd/mockd/pkg/websocket"
)

// ============================================================================
// Connection Management Handlers
// ============================================================================

// handleListWSConnections returns all WebSocket connections.
// GET /admin/ws/connections?endpoint=/ws/chat&group=room:general
func (a *AdminAPI) handleListWSConnections(w http.ResponseWriter, r *http.Request) {
	manager := a.server.Handler().WebSocketManager()

	endpointFilter := r.URL.Query().Get("endpoint")
	groupFilter := r.URL.Query().Get("group")

	infos := manager.ListConnectionInfos(endpointFilter, groupFilter)

	response := map[string]interface{}{
		"connections": infos,
		"total":       len(infos),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetWSConnection returns details for a specific connection.
// GET /admin/ws/connections/{id}
func (a *AdminAPI) handleGetWSConnection(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeWSError(w, http.StatusBadRequest, "connection ID required", "")
		return
	}

	manager := a.server.Handler().WebSocketManager()
	info, err := manager.GetConnectionInfo(id)
	if err != nil {
		writeWSError(w, http.StatusNotFound, "connection not found", id)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// handleDisconnectWS closes a WebSocket connection.
// DELETE /admin/ws/connections/{id}?code=1000&reason=admin%20disconnect
func (a *AdminAPI) handleDisconnectWS(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeWSError(w, http.StatusBadRequest, "connection ID required", "")
		return
	}

	code := websocket.CloseNormalClosure
	if codeStr := r.URL.Query().Get("code"); codeStr != "" {
		if c, err := strconv.Atoi(codeStr); err == nil {
			code = websocket.CloseCode(c)
		}
	}

	reason := r.URL.Query().Get("reason")
	if reason == "" {
		reason = "admin disconnect"
	}

	manager := a.server.Handler().WebSocketManager()
	err := manager.DisconnectByID(id, code, reason)
	if err != nil {
		writeWSError(w, http.StatusNotFound, "connection not found", id)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"disconnected": true,
		"id":           id,
		"closeCode":    int(code),
		"reason":       reason,
	})
}

// handleSendWSMessage sends a message to a specific connection.
// POST /admin/ws/connections/{id}/send
func (a *AdminAPI) handleSendWSMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeWSError(w, http.StatusBadRequest, "connection ID required", "")
		return
	}

	var req struct {
		Type  string      `json:"type"`
		Value interface{} `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeWSError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "")
		return
	}

	// Build message response and get data
	msgResp := &websocket.MessageResponse{
		Type:  req.Type,
		Value: req.Value,
	}

	data, msgType, err := msgResp.GetData()
	if err != nil {
		writeWSError(w, http.StatusBadRequest, "invalid message: "+err.Error(), "")
		return
	}

	manager := a.server.Handler().WebSocketManager()
	err = manager.SendToConnection(id, msgType, data)
	if err != nil {
		writeWSError(w, http.StatusNotFound, "connection not found", id)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"sent":         true,
		"connectionId": id,
	})
}

// handleJoinWSGroup adds a connection to a group.
// POST /admin/ws/connections/{id}/groups
func (a *AdminAPI) handleJoinWSGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeWSError(w, http.StatusBadRequest, "connection ID required", "")
		return
	}

	var req struct {
		Group string `json:"group"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeWSError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "")
		return
	}

	if req.Group == "" {
		writeWSError(w, http.StatusBadRequest, "group name required", "")
		return
	}

	manager := a.server.Handler().WebSocketManager()
	err := manager.JoinGroup(id, req.Group)
	if err != nil {
		if err == websocket.ErrConnectionNotFound {
			writeWSError(w, http.StatusNotFound, "connection not found", id)
			return
		}
		writeWSError(w, http.StatusConflict, err.Error(), id)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"joined":       true,
		"group":        req.Group,
		"connectionId": id,
	})
}

// handleLeaveWSGroup removes a connection from a group.
// DELETE /admin/ws/connections/{id}/groups?group=room:general
func (a *AdminAPI) handleLeaveWSGroup(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeWSError(w, http.StatusBadRequest, "connection ID required", "")
		return
	}

	group := r.URL.Query().Get("group")
	if group == "" {
		writeWSError(w, http.StatusBadRequest, "group name required", "")
		return
	}

	manager := a.server.Handler().WebSocketManager()
	err := manager.LeaveGroup(id, group)
	if err != nil {
		if err == websocket.ErrConnectionNotFound {
			writeWSError(w, http.StatusNotFound, "connection not found", id)
			return
		}
		writeWSError(w, http.StatusConflict, err.Error(), id)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"left":         true,
		"group":        group,
		"connectionId": id,
	})
}

// ============================================================================
// Endpoint Management Handlers
// ============================================================================

// handleListWSEndpoints returns all configured WebSocket endpoints.
// GET /admin/ws/endpoints
func (a *AdminAPI) handleListWSEndpoints(w http.ResponseWriter, r *http.Request) {
	manager := a.server.Handler().WebSocketManager()
	infos := manager.ListEndpointInfos()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"endpoints": infos,
	})
}

// handleGetWSEndpoint returns details for a specific endpoint.
// GET /admin/ws/endpoints/{path...}
// Note: Path must be URL-encoded (e.g., /admin/ws/endpoints/%2Fws%2Fchat)
func (a *AdminAPI) handleGetWSEndpoint(w http.ResponseWriter, r *http.Request) {
	path := r.PathValue("path")
	if path == "" {
		writeWSError(w, http.StatusBadRequest, "endpoint path required", "")
		return
	}

	// URL decode the path
	decodedPath, err := url.PathUnescape(path)
	if err != nil {
		decodedPath = path
	}

	// Ensure path starts with /
	if len(decodedPath) > 0 && decodedPath[0] != '/' {
		decodedPath = "/" + decodedPath
	}

	manager := a.server.Handler().WebSocketManager()
	endpoint := manager.GetEndpoint(decodedPath)
	if endpoint == nil {
		writeWSError(w, http.StatusNotFound, "endpoint not found", decodedPath)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(endpoint.Info())
}

// ============================================================================
// Broadcast Handler
// ============================================================================

// handleWSBroadcast sends a message to multiple connections.
// POST /admin/ws/broadcast
func (a *AdminAPI) handleWSBroadcast(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint,omitempty"`
		Group    string `json:"group,omitempty"`
		Message  struct {
			Type  string      `json:"type"`
			Value interface{} `json:"value"`
		} `json:"message"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeWSError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), "")
		return
	}

	if req.Endpoint == "" && req.Group == "" {
		writeWSError(w, http.StatusBadRequest, "endpoint or group required", "")
		return
	}

	// Build message
	msgResp := &websocket.MessageResponse{
		Type:  req.Message.Type,
		Value: req.Message.Value,
	}

	data, msgType, err := msgResp.GetData()
	if err != nil {
		writeWSError(w, http.StatusBadRequest, "invalid message: "+err.Error(), "")
		return
	}

	manager := a.server.Handler().WebSocketManager()
	var recipients int

	if req.Group != "" {
		recipients = manager.BroadcastToGroup(req.Group, msgType, data)
	} else {
		recipients = manager.Broadcast(req.Endpoint, msgType, data)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"broadcast":  true,
		"recipients": recipients,
		"endpoint":   req.Endpoint,
		"group":      req.Group,
	})
}

// ============================================================================
// Statistics Handler
// ============================================================================

// handleWSStats returns aggregate WebSocket statistics.
// GET /admin/ws/stats
func (a *AdminAPI) handleWSStats(w http.ResponseWriter, r *http.Request) {
	manager := a.server.Handler().WebSocketManager()
	stats := manager.Stats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// ============================================================================
// Helper Functions
// ============================================================================

// writeWSError writes a JSON error response.
func writeWSError(w http.ResponseWriter, statusCode int, errorMsg, id string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := map[string]interface{}{
		"error": errorMsg,
	}
	if id != "" {
		resp["id"] = id
	}

	json.NewEncoder(w).Encode(resp)
}
