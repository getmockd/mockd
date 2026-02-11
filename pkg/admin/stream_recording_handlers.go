package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/recording"
)

// StreamRecordingManager manages stream recording operations for the Admin API.
type StreamRecordingManager struct {
	mu          sync.RWMutex
	log         *slog.Logger
	store       *recording.FileStore
	replay      *recording.ReplayController
	initialized bool
}

// NewStreamRecordingManager creates a new stream recording manager.
func NewStreamRecordingManager() *StreamRecordingManager {
	return &StreamRecordingManager{
		log: logging.Nop(),
	}
}

// SetLogger sets the logger under the manager's own lock.
func (m *StreamRecordingManager) SetLogger(log *slog.Logger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if log != nil {
		m.log = log
	} else {
		m.log = logging.Nop()
	}
}

// Initialize initializes the manager with a file store.
func (m *StreamRecordingManager) Initialize(dataDir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.initialized {
		return nil
	}

	cfg := recording.StorageConfig{
		DataDir:     dataDir,
		MaxBytes:    recording.DefaultMaxStorageBytes,
		WarnPercent: recording.DefaultWarnPercent,
	}

	store, err := recording.NewFileStore(cfg)
	if err != nil {
		return err
	}

	m.store = store
	m.replay = recording.NewReplayController(store)
	m.initialized = true
	return nil
}

// Store returns the underlying file store.
func (m *StreamRecordingManager) Store() *recording.FileStore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store
}

// SetStore sets the file store directly (for testing).
func (m *StreamRecordingManager) SetStore(store *recording.FileStore) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = store
	m.replay = recording.NewReplayController(store)
	m.initialized = true
}

// ReplayController returns the replay controller.
func (m *StreamRecordingManager) ReplayController() *recording.ReplayController {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.replay
}

// Request/Response types

// StreamRecordingListResponse represents a list of stream recordings.
type StreamRecordingListResponse struct {
	Recordings []*recording.RecordingSummary `json:"recordings"`
	Total      int                           `json:"total"`
	Limit      int                           `json:"limit,omitempty"`
	Offset     int                           `json:"offset,omitempty"`
}

// StreamRecordingStatsResponse represents storage statistics.
type StreamRecordingStatsResponse struct {
	recording.StorageStats
}

// StartRecordingRequest represents a request to start recording.
type StartRecordingRequest struct {
	Protocol string            `json:"protocol"` // "websocket" or "sse"
	Path     string            `json:"path"`
	Headers  map[string]string `json:"headers,omitempty"`
	Name     string            `json:"name,omitempty"`
}

// StartRecordingResponse represents the response from starting a recording.
type StartRecordingResponse struct {
	SessionID   string `json:"sessionId"`
	RecordingID string `json:"recordingId"`
}

// StopRecordingResponse represents the response from stopping a recording.
type StopRecordingResponse struct {
	Recording *recording.StreamRecording `json:"recording"`
}

// StartReplayRequest represents a request to start replay.
type StartReplayRequest struct {
	Mode           string  `json:"mode"`                     // "pure", "synchronized", "triggered"
	TimingScale    float64 `json:"timingScale,omitempty"`    // 1.0 = original, 0.5 = 2x speed
	StrictMatching bool    `json:"strictMatching,omitempty"` // For synchronized mode
	Timeout        int     `json:"timeout,omitempty"`        // For synchronized/triggered mode
}

// StartReplayResponse represents the response from starting a replay.
type StartReplayResponse struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
}

// AdvanceReplayRequest represents a request to advance triggered replay.
type AdvanceReplayRequest struct {
	Count int    `json:"count,omitempty"` // Number of frames to advance
	Until string `json:"until,omitempty"` // Advance until this event/data
}

// ReplayStatusResponse represents replay session status.
type ReplayStatusResponse struct {
	ID           string `json:"id"`
	RecordingID  string `json:"recordingId"`
	Status       string `json:"status"`
	Mode         string `json:"mode"`
	CurrentFrame int    `json:"currentFrame"`
	TotalFrames  int    `json:"totalFrames"`
	FramesSent   int    `json:"framesSent"`
	ElapsedMs    int64  `json:"elapsedMs"`
}

// ConvertRecordingRequest represents a request to convert a recording.
type ConvertRecordingRequest struct {
	SimplifyTiming        *bool  `json:"simplifyTiming,omitempty"`
	MinDelay              int    `json:"minDelay,omitempty"`
	MaxDelay              int    `json:"maxDelay,omitempty"`
	IncludeClientMessages *bool  `json:"includeClientMessages,omitempty"`
	DeduplicateMessages   *bool  `json:"deduplicateMessages,omitempty"`
	Format                string `json:"format,omitempty"` // "json" or "yaml"
	AddToServer           bool   `json:"addToServer,omitempty"`
	EndpointPath          string `json:"endpointPath,omitempty"` // Path for the mock endpoint
	MockName              string `json:"mockName,omitempty"`     // Optional name for the created mock
}

// ConvertRecordingResponse represents the conversion result.
type ConvertRecordingResponse struct {
	Protocol string          `json:"protocol"`
	Config   json.RawMessage `json:"config"`
	MockID   string          `json:"mockId,omitempty"`  // Set when addToServer=true
	Added    bool            `json:"added,omitempty"`   // True if mock was added to server
	Message  string          `json:"message,omitempty"` // Status message
}

// Handlers

// handleListStreamRecordings handles GET /stream-recordings.
func (m *StreamRecordingManager) handleListStreamRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeJSON(w, http.StatusOK, StreamRecordingListResponse{
			Recordings: []*recording.RecordingSummary{},
			Total:      0,
		})
		return
	}

	filter := recording.StreamRecordingFilter{}

	// Parse query parameters
	if protocol := r.URL.Query().Get("protocol"); protocol != "" {
		filter.Protocol = recording.Protocol(protocol)
	}
	if path := r.URL.Query().Get("path"); path != "" {
		filter.Path = path
	}
	if status := r.URL.Query().Get("status"); status != "" {
		filter.Status = status
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
	if sortBy := r.URL.Query().Get("sortBy"); sortBy != "" {
		filter.SortBy = sortBy
	}
	if sortOrder := r.URL.Query().Get("sortOrder"); sortOrder != "" {
		filter.SortOrder = sortOrder
	}

	recordings, total, err := m.store.List(filter)
	if err != nil {
		m.log.Error("failed to list stream recordings", "error", err)
		writeError(w, http.StatusInternalServerError, "list_error", ErrMsgInternalError)
		return
	}

	writeJSON(w, http.StatusOK, StreamRecordingListResponse{
		Recordings: recordings,
		Total:      total,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	})
}

// handleGetStreamRecording handles GET /stream-recordings/{id}.
func (m *StreamRecordingManager) handleGetStreamRecording(w http.ResponseWriter, r *http.Request) {
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

	rec, err := m.store.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	writeJSON(w, http.StatusOK, rec)
}

// handleDeleteStreamRecording handles DELETE /stream-recordings/{id}.
func (m *StreamRecordingManager) handleDeleteStreamRecording(w http.ResponseWriter, r *http.Request) {
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

// handleGetStreamRecordingStats handles GET /stream-recordings/stats.
func (m *StreamRecordingManager) handleGetStreamRecordingStats(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeJSON(w, http.StatusOK, StreamRecordingStatsResponse{
			StorageStats: recording.StorageStats{},
		})
		return
	}

	stats, err := m.store.GetStats()
	if err != nil {
		m.log.Error("failed to get stream recording stats", "error", err)
		writeError(w, http.StatusInternalServerError, "stats_error", ErrMsgInternalError)
		return
	}
	writeJSON(w, http.StatusOK, StreamRecordingStatsResponse{
		StorageStats: *stats,
	})
}

// handleStartRecording handles POST /stream-recordings/start.
func (m *StreamRecordingManager) handleStartRecording(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store == nil {
		writeError(w, http.StatusServiceUnavailable, "not_initialized", "Stream recording manager not initialized")
		return
	}

	var req StartRecordingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", ErrMsgInvalidJSON)
		return
	}

	// Validate protocol
	protocol := recording.Protocol(req.Protocol)
	if !protocol.IsValid() || protocol == recording.ProtocolHTTP {
		writeError(w, http.StatusBadRequest, "invalid_protocol", "Protocol must be 'websocket' or 'sse'")
		return
	}

	// Create metadata
	metadata := recording.RecordingMetadata{
		Path:    req.Path,
		Headers: req.Headers,
		Source:  recording.RecordingSourceManual,
	}

	// Start recording session
	session, err := m.store.StartRecording(protocol, metadata)
	if err != nil {
		m.log.Error("failed to start stream recording", "error", err)
		writeError(w, http.StatusInternalServerError, "start_error", ErrMsgInternalError)
		return
	}

	// Apply custom name if provided
	if req.Name != "" {
		session.Recording().Name = req.Name
	}

	writeJSON(w, http.StatusCreated, StartRecordingResponse{
		SessionID:   session.ID(),
		RecordingID: session.Recording().ID,
	})
}

// handleStopRecording handles POST /stream-recordings/{id}/stop.
func (m *StreamRecordingManager) handleStopRecording(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Session ID is required")
		return
	}

	if m.store == nil {
		writeError(w, http.StatusServiceUnavailable, "not_initialized", "Stream recording manager not initialized")
		return
	}

	rec, err := m.store.CompleteRecording(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Session not found")
		return
	}

	writeJSON(w, http.StatusOK, StopRecordingResponse{
		Recording: rec,
	})
}

// handleExportStreamRecording handles POST /stream-recordings/{id}/export.
func (m *StreamRecordingManager) handleExportStreamRecording(w http.ResponseWriter, r *http.Request) {
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

	data, err := m.store.Export(id, recording.ExportFormatJSON)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s.json", id))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// handleConvertStreamRecording handles POST /stream-recordings/{id}/convert.
func (m *StreamRecordingManager) handleConvertStreamRecording(w http.ResponseWriter, r *http.Request, client *engineclient.Client) {
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

	rec, err := m.store.Get(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Recording not found")
		return
	}

	// Parse options - use defaults, then override with request values
	opts := recording.DefaultStreamConvertOptions()
	var req ConvertRecordingRequest

	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", ErrMsgInvalidJSON)
			return
		}

		// Only override defaults if values are explicitly set (using pointers)
		if req.SimplifyTiming != nil {
			opts.SimplifyTiming = *req.SimplifyTiming
		}
		if req.MinDelay > 0 {
			opts.MinDelay = req.MinDelay
		}
		if req.MaxDelay > 0 {
			opts.MaxDelay = req.MaxDelay
		}
		if req.IncludeClientMessages != nil {
			opts.IncludeClientMessages = *req.IncludeClientMessages
		}
		if req.DeduplicateMessages != nil {
			opts.DeduplicateMessages = *req.DeduplicateMessages
		}
		if req.Format != "" {
			opts.Format = req.Format
		}
	}

	// Convert
	result, err := recording.ConvertStreamRecording(rec, opts)
	if err != nil {
		m.log.Error("failed to convert stream recording", "id", id, "error", err)
		writeError(w, http.StatusBadRequest, "convert_error", "Failed to convert recording")
		return
	}

	response := ConvertRecordingResponse{
		Protocol: string(result.Protocol),
		Config:   json.RawMessage(result.ConfigJSON),
	}

	// Add to engine if requested
	if req.AddToServer && client != nil {
		mockID, addErr := m.addStreamMockToEngine(r.Context(), rec.Protocol, result, req, client)
		if addErr != nil {
			m.log.Error("failed to add stream mock to engine", "error", addErr)
			writeError(w, http.StatusInternalServerError, "add_error", ErrMsgInternalError)
			return
		}
		response.MockID = mockID
		response.Added = true
		response.Message = fmt.Sprintf("%s mock added to engine at %s", rec.Protocol, req.EndpointPath)
	}

	writeJSON(w, http.StatusOK, response)
}

// addStreamMockToEngine adds the converted stream mock to the engine via HTTP client.
func (m *StreamRecordingManager) addStreamMockToEngine(ctx context.Context, protocol recording.Protocol, result *recording.StreamConvertResult, req ConvertRecordingRequest, client *engineclient.Client) (string, error) {
	switch protocol {
	case recording.ProtocolWebSocket:
		return m.addWebSocketMock(ctx, result, req, client)
	case recording.ProtocolSSE:
		return m.addSSEMock(ctx, result, req, client)
	default:
		return "", fmt.Errorf("unsupported protocol for addToServer: %s", protocol)
	}
}

// addWebSocketMock adds a WebSocket mock to the engine via HTTP client.
func (m *StreamRecordingManager) addWebSocketMock(ctx context.Context, result *recording.StreamConvertResult, req ConvertRecordingRequest, client *engineclient.Client) (string, error) {
	if req.EndpointPath == "" {
		return "", errors.New("endpointPath is required for WebSocket mocks")
	}

	// Config is already a *mock.WSScenarioConfig from the converter
	scenario, ok := result.Config.(*mock.WSScenarioConfig)
	if !ok {
		return "", fmt.Errorf("unexpected config type for WebSocket: %T", result.Config)
	}

	// Override name if provided in request
	scenarioName := req.MockName
	if scenarioName == "" {
		scenarioName = "recorded-scenario"
	}
	scenario.Name = scenarioName

	// Create a unified WebSocket mock configuration
	wsEnabled := true
	wsMock := &config.MockConfiguration{
		Name:    scenarioName,
		Type:    mock.TypeWebSocket,
		Enabled: &wsEnabled,
		WebSocket: &mock.WebSocketSpec{
			Path:     req.EndpointPath,
			Scenario: scenario,
		},
	}

	created, err := client.CreateMock(ctx, wsMock)
	if err != nil {
		return "", err
	}

	return created.ID, nil
}

// addSSEMock adds an SSE mock to the engine via HTTP client.
func (m *StreamRecordingManager) addSSEMock(ctx context.Context, result *recording.StreamConvertResult, req ConvertRecordingRequest, client *engineclient.Client) (string, error) {
	// Config is already a *mock.SSEConfig from the converter
	sseConfig, ok := result.Config.(*mock.SSEConfig)
	if !ok {
		return "", fmt.Errorf("unexpected config type for SSE: %T", result.Config)
	}

	// Determine endpoint path
	endpointPath := req.EndpointPath
	if endpointPath == "" {
		endpointPath = "/sse/recorded"
	}

	// Create a mock with SSE configuration
	mockName := req.MockName
	if mockName == "" {
		mockName = "SSE recording " + endpointPath
	}

	sseEnabled := true
	sseMock := &config.MockConfiguration{
		Name:    mockName,
		Type:    mock.TypeHTTP,
		Enabled: &sseEnabled,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   endpointPath,
			},
			SSE: sseConfig,
		},
	}

	created, err := client.CreateMock(ctx, sseMock)
	if err != nil {
		return "", err
	}

	return created.ID, nil
}

// handleStartReplay handles POST /stream-recordings/{id}/replay.
func (m *StreamRecordingManager) handleStartReplay(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Recording ID is required")
		return
	}

	if m.replay == nil {
		writeError(w, http.StatusServiceUnavailable, "not_initialized", "Replay controller not initialized")
		return
	}

	var req StartReplayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", ErrMsgInvalidJSON)
		return
	}

	// Parse mode
	var mode recording.ReplayMode
	switch req.Mode {
	case "pure", "":
		mode = recording.ReplayModePure
	case "synchronized":
		mode = recording.ReplayModeSynchronized
	case "triggered":
		mode = recording.ReplayModeTriggered
	default:
		writeError(w, http.StatusBadRequest, "invalid_mode", "Mode must be 'pure', 'synchronized', or 'triggered'")
		return
	}

	config := recording.ReplayConfig{
		RecordingID:    id,
		Mode:           mode,
		TimingScale:    req.TimingScale,
		StrictMatching: req.StrictMatching,
		Timeout:        req.Timeout,
	}

	session, err := m.replay.StartReplay(config)
	if err != nil {
		m.log.Error("failed to start replay for recording", "id", id, "error", err)
		writeError(w, http.StatusBadRequest, "replay_error", "Failed to start replay")
		return
	}

	writeJSON(w, http.StatusCreated, StartReplayResponse{
		SessionID: session.ID,
		Status:    string(session.Status),
	})
}

// handleStopReplay handles DELETE /replay/{id}.
func (m *StreamRecordingManager) handleStopReplay(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Replay session ID is required")
		return
	}

	if m.replay == nil {
		writeError(w, http.StatusNotFound, "not_found", "Replay session not found")
		return
	}

	if err := m.replay.StopReplay(id); err != nil {
		writeError(w, http.StatusNotFound, "not_found", "Replay session not found")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetReplayStatus handles GET /replay/{id}.
func (m *StreamRecordingManager) handleGetReplayStatus(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Replay session ID is required")
		return
	}

	if m.replay == nil {
		writeError(w, http.StatusNotFound, "not_found", "Replay session not found")
		return
	}

	session, ok := m.replay.GetSession(id)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Replay session not found")
		return
	}

	writeJSON(w, http.StatusOK, ReplayStatusResponse{
		ID:           session.ID,
		RecordingID:  session.RecordingID,
		Status:       string(session.Status),
		Mode:         string(session.Config.Mode),
		CurrentFrame: session.CurrentFrame,
		TotalFrames:  session.TotalFrames,
		FramesSent:   session.FramesSent,
		ElapsedMs:    session.ElapsedMs,
	})
}

// handleAdvanceReplay handles POST /replay/{id}/advance.
func (m *StreamRecordingManager) handleAdvanceReplay(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Replay session ID is required")
		return
	}

	if m.replay == nil {
		writeError(w, http.StatusNotFound, "not_found", "Replay session not found")
		return
	}

	var req AdvanceReplayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", ErrMsgInvalidJSON)
		return
	}

	advReq := recording.AdvanceRequest{
		Count: req.Count,
		Until: req.Until,
	}

	resp, err := m.replay.Advance(id, advReq)
	if err != nil {
		m.log.Error("failed to advance replay", "id", id, "error", err)
		writeError(w, http.StatusBadRequest, "advance_error", "Failed to advance replay")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// handleListReplaySessions handles GET /replay.
func (m *StreamRecordingManager) handleListReplaySessions(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.replay == nil {
		writeJSON(w, http.StatusOK, []ReplayStatusResponse{})
		return
	}

	sessions := m.replay.ListSessions()
	result := make([]ReplayStatusResponse, 0, len(sessions))

	for _, s := range sessions {
		result = append(result, ReplayStatusResponse{
			ID:           s.ID,
			RecordingID:  s.RecordingID,
			Status:       string(s.Status),
			Mode:         string(s.Config.Mode),
			CurrentFrame: s.CurrentFrame,
			TotalFrames:  s.TotalFrames,
			FramesSent:   s.FramesSent,
			ElapsedMs:    s.ElapsedMs,
		})
	}

	writeJSON(w, http.StatusOK, result)
}

// handlePauseReplay handles POST /replay/{id}/pause.
func (m *StreamRecordingManager) handlePauseReplay(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Replay session ID is required")
		return
	}

	if m.replay == nil {
		writeError(w, http.StatusNotFound, "not_found", "Replay session not found")
		return
	}

	if err := m.replay.PauseReplay(id); err != nil {
		m.log.Error("failed to pause replay", "id", id, "error", err)
		writeError(w, http.StatusBadRequest, "pause_error", "Failed to pause replay")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleResumeReplay handles POST /replay/{id}/resume.
func (m *StreamRecordingManager) handleResumeReplay(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Replay session ID is required")
		return
	}

	if m.replay == nil {
		writeError(w, http.StatusNotFound, "not_found", "Replay session not found")
		return
	}

	if err := m.replay.ResumeReplay(id); err != nil {
		m.log.Error("failed to resume replay", "id", id, "error", err)
		writeError(w, http.StatusBadRequest, "resume_error", "Failed to resume replay")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleVacuum handles POST /stream-recordings/vacuum.
func (m *StreamRecordingManager) handleVacuum(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store == nil {
		writeError(w, http.StatusServiceUnavailable, "not_initialized", "Stream recording manager not initialized")
		return
	}

	count, freedBytes, err := m.store.Vacuum()
	if err != nil {
		m.log.Error("failed to vacuum stream recordings", "error", err)
		writeError(w, http.StatusInternalServerError, "vacuum_error", ErrMsgInternalError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "Vacuum completed",
		"removed":    count,
		"freedBytes": freedBytes,
	})
}

// handleGetActiveSessions handles GET /stream-recordings/sessions.
func (m *StreamRecordingManager) handleGetActiveSessions(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeJSON(w, http.StatusOK, []*recording.StreamRecordingSessionInfo{})
		return
	}

	sessions := m.store.GetActiveSessions()
	writeJSON(w, http.StatusOK, sessions)
}
