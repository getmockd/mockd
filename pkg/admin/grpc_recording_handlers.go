package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"sync"

	"github.com/getmockd/mockd/pkg/grpc"
	"github.com/getmockd/mockd/pkg/recording"
)

// GRPCRecordingManager manages gRPC recording operations for the Admin API.
type GRPCRecordingManager struct {
	mu      sync.RWMutex
	store   *recording.GRPCStore
	servers map[string]*grpc.Server // server ID -> server
}

// NewGRPCRecordingManager creates a new gRPC recording manager.
func NewGRPCRecordingManager() *GRPCRecordingManager {
	return &GRPCRecordingManager{
		store:   recording.NewGRPCStore(1000),
		servers: make(map[string]*grpc.Server),
	}
}

// Store returns the gRPC recording store.
func (m *GRPCRecordingManager) Store() *recording.GRPCStore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store
}

// RegisterServer registers a gRPC server for recording management.
func (m *GRPCRecordingManager) RegisterServer(server *grpc.Server) {
	if server == nil || server.ID() == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.servers[server.ID()] = server
	// Set the recording store on the server
	server.SetRecordingStore(&grpcStoreAdapter{store: m.store})
}

// UnregisterServer removes a gRPC server from recording management.
func (m *GRPCRecordingManager) UnregisterServer(serverID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if server, ok := m.servers[serverID]; ok {
		server.DisableRecording()
		delete(m.servers, serverID)
	}
}

// GetServer returns a registered gRPC server by ID.
func (m *GRPCRecordingManager) GetServer(serverID string) *grpc.Server {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.servers[serverID]
}

// ListServers returns all registered server IDs.
func (m *GRPCRecordingManager) ListServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.servers))
	for id := range m.servers {
		ids = append(ids, id)
	}
	return ids
}

// grpcStoreAdapter adapts GRPCStore to the grpc.RecordingStore interface.
type grpcStoreAdapter struct {
	store *recording.GRPCStore
}

func (a *grpcStoreAdapter) Add(r *grpc.GRPCRecording) error {
	// Convert from grpc.GRPCRecording to recording.GRPCRecording
	rec := &recording.GRPCRecording{
		ID:         r.ID,
		Timestamp:  r.Timestamp,
		Service:    r.Service,
		Method:     r.Method,
		StreamType: recording.GRPCStreamType(r.StreamType),
		Request:    r.Request,
		Response:   r.Response,
		Metadata:   r.Metadata,
		Duration:   r.Duration,
		ProtoFile:  r.ProtoFile,
	}
	if r.Error != nil {
		rec.Error = &recording.GRPCRecordedError{
			Code:    r.Error.Code,
			Message: r.Error.Message,
		}
	}
	return a.store.Add(rec)
}

// Request/Response types

// GRPCRecordingListResponse represents a list of gRPC recordings.
type GRPCRecordingListResponse struct {
	Recordings []*recording.GRPCRecording `json:"recordings"`
	Total      int                        `json:"total"`
	Limit      int                        `json:"limit,omitempty"`
	Offset     int                        `json:"offset,omitempty"`
}

// GRPCRecordingStatsResponse represents gRPC recording statistics.
type GRPCRecordingStatsResponse struct {
	*recording.GRPCRecordingStats
}

// GRPCServerStatusResponse represents the status of a gRPC server.
type GRPCServerStatusResponse struct {
	ID               string `json:"id"`
	Address          string `json:"address"`
	Running          bool   `json:"running"`
	RecordingEnabled bool   `json:"recordingEnabled"`
}

// GRPCRecordingStartRequest represents a request to start recording.
type GRPCRecordingStartRequest struct {
	// No additional fields needed for now
}

// GRPCRecordingStartResponse represents the response after starting recording.
type GRPCRecordingStartResponse struct {
	Message string `json:"message"`
	Enabled bool   `json:"enabled"`
}

// GRPCRecordingStopResponse represents the response after stopping recording.
type GRPCRecordingStopResponse struct {
	Message string `json:"message"`
	Enabled bool   `json:"enabled"`
}

// GRPCConvertRequest represents a request to convert recordings to mock config.
type GRPCConvertRequest struct {
	RecordingIDs    []string `json:"recordingIds,omitempty"`
	Service         string   `json:"service,omitempty"`
	IncludeMetadata bool     `json:"includeMetadata,omitempty"`
	IncludeDelay    bool     `json:"includeDelay,omitempty"`
	Deduplicate     bool     `json:"deduplicate,omitempty"`
}

// GRPCConvertResponse represents the result of converting recordings.
type GRPCConvertResponse struct {
	Config   *grpc.GRPCConfig `json:"config"`
	Services int              `json:"services"`
	Methods  int              `json:"methods"`
	Total    int              `json:"total"`
	Warnings []string         `json:"warnings,omitempty"`
}

// Handlers

// handleListGRPCRecordings handles GET /grpc-recordings.
func (m *GRPCRecordingManager) handleListGRPCRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeJSON(w, http.StatusOK, GRPCRecordingListResponse{
			Recordings: []*recording.GRPCRecording{},
			Total:      0,
		})
		return
	}

	filter := recording.GRPCRecordingFilter{}

	// Parse query parameters
	if service := r.URL.Query().Get("service"); service != "" {
		filter.Service = service
	}
	if method := r.URL.Query().Get("method"); method != "" {
		filter.Method = method
	}
	if streamType := r.URL.Query().Get("streamType"); streamType != "" {
		filter.StreamType = streamType
	}
	if hasError := r.URL.Query().Get("hasError"); hasError != "" {
		b := hasError == "true"
		filter.HasError = &b
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = limit
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			filter.Offset = offset
		}
	}

	recordings, total := m.store.List(filter)

	writeJSON(w, http.StatusOK, GRPCRecordingListResponse{
		Recordings: recordings,
		Total:      total,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	})
}

// handleGetGRPCRecording handles GET /grpc-recordings/{id}.
func (m *GRPCRecordingManager) handleGetGRPCRecording(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Recording ID is required")
		return
	}

	if m.store == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	rec := m.store.Get(id)
	if rec == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	writeJSON(w, http.StatusOK, rec)
}

// handleDeleteGRPCRecording handles DELETE /grpc-recordings/{id}.
func (m *GRPCRecordingManager) handleDeleteGRPCRecording(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Recording ID is required")
		return
	}

	if m.store == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	if err := m.store.Delete(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleClearGRPCRecordings handles DELETE /grpc-recordings.
func (m *GRPCRecordingManager) handleClearGRPCRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store == nil {
		writeJSON(w, http.StatusOK, map[string]int{"deleted": 0})
		return
	}

	count := m.store.Clear()
	writeJSON(w, http.StatusOK, map[string]int{"deleted": count})
}

// handleGetGRPCRecordingStats handles GET /grpc-recordings/stats.
func (m *GRPCRecordingManager) handleGetGRPCRecordingStats(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeJSON(w, http.StatusOK, GRPCRecordingStatsResponse{
			GRPCRecordingStats: &recording.GRPCRecordingStats{},
		})
		return
	}

	stats := m.store.Stats()
	writeJSON(w, http.StatusOK, GRPCRecordingStatsResponse{
		GRPCRecordingStats: stats,
	})
}

// handleStartGRPCRecording handles POST /grpc/{id}/record/start.
func (m *GRPCRecordingManager) handleStartGRPCRecording(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")
	if serverID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Server ID is required")
		return
	}

	m.mu.RLock()
	server := m.servers[serverID]
	m.mu.RUnlock()

	if server == nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("gRPC server '%s' not found", serverID))
		return
	}

	if !server.IsRunning() {
		writeError(w, http.StatusBadRequest, "not_running", "gRPC server is not running")
		return
	}

	server.EnableRecording()

	writeJSON(w, http.StatusOK, GRPCRecordingStartResponse{
		Message: fmt.Sprintf("Recording enabled for gRPC server '%s'", serverID),
		Enabled: true,
	})
}

// handleStopGRPCRecording handles POST /grpc/{id}/record/stop.
func (m *GRPCRecordingManager) handleStopGRPCRecording(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")
	if serverID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Server ID is required")
		return
	}

	m.mu.RLock()
	server := m.servers[serverID]
	m.mu.RUnlock()

	if server == nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("gRPC server '%s' not found", serverID))
		return
	}

	server.DisableRecording()

	writeJSON(w, http.StatusOK, GRPCRecordingStopResponse{
		Message: fmt.Sprintf("Recording disabled for gRPC server '%s'", serverID),
		Enabled: false,
	})
}

// handleGetGRPCServerStatus handles GET /grpc/{id}/status.
func (m *GRPCRecordingManager) handleGetGRPCServerStatus(w http.ResponseWriter, r *http.Request) {
	serverID := r.PathValue("id")
	if serverID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Server ID is required")
		return
	}

	m.mu.RLock()
	server := m.servers[serverID]
	m.mu.RUnlock()

	if server == nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("gRPC server '%s' not found", serverID))
		return
	}

	writeJSON(w, http.StatusOK, GRPCServerStatusResponse{
		ID:               serverID,
		Address:          server.Address(),
		Running:          server.IsRunning(),
		RecordingEnabled: server.IsRecordingEnabled(),
	})
}

// handleListGRPCServers handles GET /grpc.
func (m *GRPCRecordingManager) handleListGRPCServers(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	servers := make([]GRPCServerStatusResponse, 0, len(m.servers))
	for id, server := range m.servers {
		servers = append(servers, GRPCServerStatusResponse{
			ID:               id,
			Address:          server.Address(),
			Running:          server.IsRunning(),
			RecordingEnabled: server.IsRecordingEnabled(),
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"servers": servers,
		"count":   len(servers),
	})
}

// handleConvertGRPCRecording handles POST /grpc-recordings/{id}/convert.
func (m *GRPCRecordingManager) handleConvertGRPCRecording(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Recording ID is required")
		return
	}

	if m.store == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	rec := m.store.Get(id)
	if rec == nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	// Parse options
	var req GRPCConvertRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
			return
		}
	}

	opts := recording.GRPCConvertOptions{
		IncludeMetadata: req.IncludeMetadata,
		IncludeDelay:    req.IncludeDelay,
		Deduplicate:     true,
	}

	// Convert single recording
	result := recording.ConvertGRPCRecordings([]*recording.GRPCRecording{rec}, opts)

	writeJSON(w, http.StatusOK, GRPCConvertResponse{
		Config:   result.Config,
		Services: result.Services,
		Methods:  result.Methods,
		Total:    result.Total,
		Warnings: result.Warnings,
	})
}

// handleConvertGRPCRecordings handles POST /grpc-recordings/convert.
func (m *GRPCRecordingManager) handleConvertGRPCRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	// Parse request
	var req GRPCConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	// Get recordings to convert
	var recordings []*recording.GRPCRecording
	if len(req.RecordingIDs) > 0 {
		for _, id := range req.RecordingIDs {
			if rec := m.store.Get(id); rec != nil {
				recordings = append(recordings, rec)
			}
		}
	} else if req.Service != "" {
		recordings = m.store.ListByService(req.Service)
	} else {
		recordings, _ = m.store.List(recording.GRPCRecordingFilter{})
	}

	if len(recordings) == 0 {
		writeError(w, http.StatusBadRequest, "no_recordings", "No recordings to convert")
		return
	}

	opts := recording.GRPCConvertOptions{
		IncludeMetadata: req.IncludeMetadata,
		IncludeDelay:    req.IncludeDelay,
		Deduplicate:     req.Deduplicate,
	}

	result := recording.ConvertGRPCRecordings(recordings, opts)

	writeJSON(w, http.StatusOK, GRPCConvertResponse{
		Config:   result.Config,
		Services: result.Services,
		Methods:  result.Methods,
		Total:    result.Total,
		Warnings: result.Warnings,
	})
}

// handleExportGRPCRecordings handles POST /grpc-recordings/export.
func (m *GRPCRecordingManager) handleExportGRPCRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	data, err := m.store.Export()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_error", "Failed to export recordings: "+err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=grpc-recordings.json")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
