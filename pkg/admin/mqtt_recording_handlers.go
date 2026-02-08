package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/getmockd/mockd/pkg/mqtt"
	"github.com/getmockd/mockd/pkg/recording"
)

// MQTTRecordingManager manages MQTT recording operations for the Admin API.
type MQTTRecordingManager struct {
	mu      sync.RWMutex
	store   *recording.MQTTStore
	brokers map[string]*mqtt.Broker // broker ID -> broker
}

// NewMQTTRecordingManager creates a new MQTT recording manager.
func NewMQTTRecordingManager() *MQTTRecordingManager {
	return &MQTTRecordingManager{
		store:   recording.NewMQTTStore(1000),
		brokers: make(map[string]*mqtt.Broker),
	}
}

// Store returns the MQTT recording store.
func (m *MQTTRecordingManager) Store() *recording.MQTTStore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.store
}

// RegisterBroker registers an MQTT broker for recording management.
func (m *MQTTRecordingManager) RegisterBroker(broker *mqtt.Broker) {
	if broker == nil || broker.ID() == "" {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.brokers[broker.ID()] = broker
	// Set the recording store on the broker
	broker.SetRecordingStore(&mqttStoreAdapter{store: m.store})
}

// UnregisterBroker removes an MQTT broker from recording management.
func (m *MQTTRecordingManager) UnregisterBroker(brokerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if broker, ok := m.brokers[brokerID]; ok {
		broker.DisableRecording()
		delete(m.brokers, brokerID)
	}
}

// GetBroker returns a registered MQTT broker by ID.
func (m *MQTTRecordingManager) GetBroker(brokerID string) *mqtt.Broker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.brokers[brokerID]
}

// ListBrokers returns all registered broker IDs.
func (m *MQTTRecordingManager) ListBrokers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ids := make([]string, 0, len(m.brokers))
	for id := range m.brokers {
		ids = append(ids, id)
	}
	return ids
}

// mqttStoreAdapter adapts MQTTStore to the mqtt.MQTTRecordingStore interface.
type mqttStoreAdapter struct {
	store *recording.MQTTStore
}

func (a *mqttStoreAdapter) Add(data mqtt.MQTTRecordingData) error {
	// Convert from mqtt.MQTTRecordingData to recording.MQTTRecording
	rec := recording.NewMQTTRecording(
		data.Topic,
		data.Payload,
		int(data.QoS),
		data.Retain,
		data.ClientID,
		recording.MQTTDirection(data.Direction),
	)
	return a.store.Add(rec)
}

// Request/Response types

// MQTTRecordingListResponse represents a list of MQTT recordings.
type MQTTRecordingListResponse struct {
	Recordings []*recording.MQTTRecording `json:"recordings"`
	Total      int                        `json:"total"`
	Limit      int                        `json:"limit,omitempty"`
	Offset     int                        `json:"offset,omitempty"`
}

// MQTTRecordingStatsResponse represents MQTT recording statistics.
type MQTTRecordingStatsResponse struct {
	*recording.MQTTRecordingStats
}

// MQTTBrokerStatusResponse represents the status of an MQTT broker.
type MQTTBrokerStatusResponse struct {
	ID               string `json:"id"`
	Port             int    `json:"port"`
	Running          bool   `json:"running"`
	RecordingEnabled bool   `json:"recordingEnabled"`
	ClientCount      int    `json:"clientCount"`
	TopicCount       int    `json:"topicCount"`
	TLSEnabled       bool   `json:"tlsEnabled"`
	AuthEnabled      bool   `json:"authEnabled"`
}

// MQTTGlobalStatusResponse represents the aggregate status of all MQTT brokers.
type MQTTGlobalStatusResponse struct {
	Running      bool                       `json:"running"`
	BrokerCount  int                        `json:"brokerCount"`
	TotalClients int                        `json:"totalClients"`
	TotalTopics  int                        `json:"totalTopics"`
	Brokers      []MQTTBrokerStatusResponse `json:"brokers"`
}

// MQTTRecordingStartResponse represents the response after starting recording.
type MQTTRecordingStartResponse struct {
	Message string `json:"message"`
	Enabled bool   `json:"enabled"`
}

// MQTTRecordingStopResponse represents the response after stopping recording.
type MQTTRecordingStopResponse struct {
	Message string `json:"message"`
	Enabled bool   `json:"enabled"`
}

// MQTTConvertRequest represents a request to convert recordings to mock config.
type MQTTConvertRequest struct {
	RecordingIDs  []string `json:"recordingIds,omitempty"`
	TopicPattern  string   `json:"topicPattern,omitempty"`
	Deduplicate   bool     `json:"deduplicate,omitempty"`
	IncludeQoS    bool     `json:"includeQoS,omitempty"`
	IncludeRetain bool     `json:"includeRetain,omitempty"`
}

// MQTTConvertResponse represents the result of converting recordings.
type MQTTConvertResponse struct {
	Config       *mqtt.MQTTConfig `json:"config"`
	TopicCount   int              `json:"topicCount"`
	MessageCount int              `json:"messageCount"`
	Total        int              `json:"total"`
	Warnings     []string         `json:"warnings,omitempty"`
}

// Handlers

// handleListMQTTRecordings handles GET /mqtt-recordings.
func (m *MQTTRecordingManager) handleListMQTTRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeJSON(w, http.StatusOK, MQTTRecordingListResponse{
			Recordings: []*recording.MQTTRecording{},
			Total:      0,
		})
		return
	}

	filter := recording.MQTTRecordingFilter{}

	// Parse query parameters
	if topicPattern := r.URL.Query().Get("topicPattern"); topicPattern != "" {
		filter.TopicPattern = topicPattern
	}
	if clientID := r.URL.Query().Get("clientId"); clientID != "" {
		filter.ClientID = clientID
	}
	if direction := r.URL.Query().Get("direction"); direction != "" {
		filter.Direction = recording.MQTTDirection(direction)
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

	writeJSON(w, http.StatusOK, MQTTRecordingListResponse{
		Recordings: recordings,
		Total:      total,
		Limit:      filter.Limit,
		Offset:     filter.Offset,
	})
}

// handleGetMQTTRecording handles GET /mqtt-recordings/{id}.
func (m *MQTTRecordingManager) handleGetMQTTRecording(w http.ResponseWriter, r *http.Request) {
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

// handleDeleteMQTTRecording handles DELETE /mqtt-recordings/{id}.
func (m *MQTTRecordingManager) handleDeleteMQTTRecording(w http.ResponseWriter, r *http.Request) {
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

// handleClearMQTTRecordings handles DELETE /mqtt-recordings.
func (m *MQTTRecordingManager) handleClearMQTTRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store == nil {
		writeJSON(w, http.StatusOK, map[string]int{"deleted": 0})
		return
	}

	count := m.store.Clear()
	writeJSON(w, http.StatusOK, map[string]int{"deleted": count})
}

// handleGetMQTTRecordingStats handles GET /mqtt-recordings/stats.
func (m *MQTTRecordingManager) handleGetMQTTRecordingStats(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeJSON(w, http.StatusOK, MQTTRecordingStatsResponse{
			MQTTRecordingStats: &recording.MQTTRecordingStats{},
		})
		return
	}

	stats := m.store.Stats()
	writeJSON(w, http.StatusOK, MQTTRecordingStatsResponse{
		MQTTRecordingStats: stats,
	})
}

// handleStartMQTTRecording handles POST /mqtt/{id}/record/start.
func (m *MQTTRecordingManager) handleStartMQTTRecording(w http.ResponseWriter, r *http.Request) {
	brokerID := r.PathValue("id")
	if brokerID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Broker ID is required")
		return
	}

	m.mu.RLock()
	broker := m.brokers[brokerID]
	m.mu.RUnlock()

	if broker == nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("MQTT broker '%s' not found", brokerID))
		return
	}

	if !broker.IsRunning() {
		writeError(w, http.StatusBadRequest, "not_running", "MQTT broker is not running")
		return
	}

	broker.EnableRecording()

	writeJSON(w, http.StatusOK, MQTTRecordingStartResponse{
		Message: fmt.Sprintf("Recording enabled for MQTT broker '%s'", brokerID),
		Enabled: true,
	})
}

// handleStopMQTTRecording handles POST /mqtt/{id}/record/stop.
func (m *MQTTRecordingManager) handleStopMQTTRecording(w http.ResponseWriter, r *http.Request) {
	brokerID := r.PathValue("id")
	if brokerID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Broker ID is required")
		return
	}

	m.mu.RLock()
	broker := m.brokers[brokerID]
	m.mu.RUnlock()

	if broker == nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("MQTT broker '%s' not found", brokerID))
		return
	}

	broker.DisableRecording()

	writeJSON(w, http.StatusOK, MQTTRecordingStopResponse{
		Message: fmt.Sprintf("Recording disabled for MQTT broker '%s'", brokerID),
		Enabled: false,
	})
}

// handleGetMQTTBrokerStatus handles GET /mqtt/{id}/status.
func (m *MQTTRecordingManager) handleGetMQTTBrokerStatus(w http.ResponseWriter, r *http.Request) {
	brokerID := r.PathValue("id")
	if brokerID == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Broker ID is required")
		return
	}

	m.mu.RLock()
	broker := m.brokers[brokerID]
	m.mu.RUnlock()

	if broker == nil {
		writeError(w, http.StatusNotFound, "not_found", fmt.Sprintf("MQTT broker '%s' not found", brokerID))
		return
	}

	stats := broker.GetStats()

	writeJSON(w, http.StatusOK, MQTTBrokerStatusResponse{
		ID:               brokerID,
		Port:             stats.Port,
		Running:          stats.Running,
		RecordingEnabled: broker.IsRecordingEnabled(),
		ClientCount:      stats.ClientCount,
		TopicCount:       stats.TopicCount,
		TLSEnabled:       stats.TLSEnabled,
		AuthEnabled:      stats.AuthEnabled,
	})
}

// handleListMQTTBrokersInternal handles listing brokers from locally registered brokers.
func (m *MQTTRecordingManager) handleListMQTTBrokersInternal(w http.ResponseWriter, _ *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	brokers := make([]MQTTBrokerStatusResponse, 0, len(m.brokers))
	for id, broker := range m.brokers {
		stats := broker.GetStats()
		brokers = append(brokers, MQTTBrokerStatusResponse{
			ID:               id,
			Port:             stats.Port,
			Running:          stats.Running,
			RecordingEnabled: broker.IsRecordingEnabled(),
			ClientCount:      stats.ClientCount,
			TopicCount:       stats.TopicCount,
			TLSEnabled:       stats.TLSEnabled,
			AuthEnabled:      stats.AuthEnabled,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"brokers": brokers,
		"count":   len(brokers),
	})
}

// handleGetMQTTStatusInternal handles status from locally registered brokers.
func (m *MQTTRecordingManager) handleGetMQTTStatusInternal(w http.ResponseWriter, _ *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	brokers := make([]MQTTBrokerStatusResponse, 0, len(m.brokers))
	totalClients := 0
	totalTopics := 0
	anyRunning := false

	for id, broker := range m.brokers {
		stats := broker.GetStats()
		if stats.Running {
			anyRunning = true
		}
		totalClients += stats.ClientCount
		totalTopics += stats.TopicCount
		brokers = append(brokers, MQTTBrokerStatusResponse{
			ID:               id,
			Port:             stats.Port,
			Running:          stats.Running,
			RecordingEnabled: broker.IsRecordingEnabled(),
			ClientCount:      stats.ClientCount,
			TopicCount:       stats.TopicCount,
			TLSEnabled:       stats.TLSEnabled,
			AuthEnabled:      stats.AuthEnabled,
		})
	}

	writeJSON(w, http.StatusOK, MQTTGlobalStatusResponse{
		Running:      anyRunning,
		BrokerCount:  len(m.brokers),
		TotalClients: totalClients,
		TotalTopics:  totalTopics,
		Brokers:      brokers,
	})
}

// handleListMQTTBrokers handles GET /mqtt.
// Queries MQTT mocks from the engine (over HTTP) to build broker list.
func (a *API) handleListMQTTBrokers(w http.ResponseWriter, r *http.Request) {
	// First check local recording manager for directly registered brokers.
	a.mqttRecordingManager.mu.RLock()
	localBrokers := len(a.mqttRecordingManager.brokers)
	a.mqttRecordingManager.mu.RUnlock()

	if localBrokers > 0 {
		a.mqttRecordingManager.handleListMQTTBrokersInternal(w, r)
		return
	}

	// No local brokers — query engine for MQTT mocks over HTTP.
	if a.localEngine == nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"brokers": []interface{}{}, "count": 0})
		return
	}

	mocks, err := a.localEngine.ListMocks(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"brokers": []interface{}{}, "count": 0})
		return
	}

	brokers := make([]MQTTBrokerStatusResponse, 0)
	for _, m := range mocks {
		if m.Type == "mqtt" && m.MQTT != nil {
			brokers = append(brokers, MQTTBrokerStatusResponse{
				ID:      m.ID,
				Port:    m.MQTT.Port,
				Running: m.Enabled == nil || *m.Enabled,
			})
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"brokers": brokers, "count": len(brokers)})
}

// handleGetMQTTStatus handles GET /mqtt/status.
// Queries MQTT mocks from the engine (over HTTP) to build aggregate status.
func (a *API) handleGetMQTTStatus(w http.ResponseWriter, r *http.Request) {
	// First check local recording manager for directly registered brokers.
	a.mqttRecordingManager.mu.RLock()
	localBrokers := len(a.mqttRecordingManager.brokers)
	a.mqttRecordingManager.mu.RUnlock()

	if localBrokers > 0 {
		a.mqttRecordingManager.handleGetMQTTStatusInternal(w, r)
		return
	}

	// No local brokers — query engine for MQTT mocks over HTTP.
	if a.localEngine == nil {
		writeJSON(w, http.StatusOK, MQTTGlobalStatusResponse{Brokers: []MQTTBrokerStatusResponse{}})
		return
	}

	mocks, err := a.localEngine.ListMocks(r.Context())
	if err != nil {
		writeJSON(w, http.StatusOK, MQTTGlobalStatusResponse{Brokers: []MQTTBrokerStatusResponse{}})
		return
	}

	brokers := make([]MQTTBrokerStatusResponse, 0)
	anyRunning := false
	for _, m := range mocks {
		if m.Type == "mqtt" && m.MQTT != nil {
			running := m.Enabled == nil || *m.Enabled
			if running {
				anyRunning = true
			}
			brokers = append(brokers, MQTTBrokerStatusResponse{
				ID:      m.ID,
				Port:    m.MQTT.Port,
				Running: running,
			})
		}
	}

	writeJSON(w, http.StatusOK, MQTTGlobalStatusResponse{
		Running:     anyRunning,
		BrokerCount: len(brokers),
		Brokers:     brokers,
	})
}

// handleConvertMQTTRecording handles POST /mqtt-recordings/{id}/convert.
func (m *MQTTRecordingManager) handleConvertMQTTRecording(w http.ResponseWriter, r *http.Request) {
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
	var req MQTTConvertRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_json", ErrMsgInvalidJSON)
			return
		}
	}

	opts := recording.MQTTConvertOptions{
		Deduplicate:   true,
		IncludeQoS:    req.IncludeQoS,
		IncludeRetain: req.IncludeRetain,
	}

	// Convert single recording
	result := recording.ConvertMQTTRecordings([]*recording.MQTTRecording{rec}, opts)

	writeJSON(w, http.StatusOK, MQTTConvertResponse{
		Config:       result.Config,
		TopicCount:   result.TopicCount,
		MessageCount: result.MessageCount,
		Total:        result.Total,
		Warnings:     result.Warnings,
	})
}

// handleConvertMQTTRecordings handles POST /mqtt-recordings/convert.
func (m *MQTTRecordingManager) handleConvertMQTTRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	// Parse request
	var req MQTTConvertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", ErrMsgInvalidJSON)
		return
	}

	// Get recordings to convert
	var recordings []*recording.MQTTRecording
	if len(req.RecordingIDs) > 0 {
		for _, id := range req.RecordingIDs {
			if rec := m.store.Get(id); rec != nil {
				recordings = append(recordings, rec)
			}
		}
	} else if req.TopicPattern != "" {
		recordings = m.store.ListByTopic(req.TopicPattern)
	} else {
		recordings, _ = m.store.List(recording.MQTTRecordingFilter{})
	}

	if len(recordings) == 0 {
		writeError(w, http.StatusBadRequest, "no_recordings", "No recordings to convert")
		return
	}

	opts := recording.MQTTConvertOptions{
		Deduplicate:   req.Deduplicate,
		IncludeQoS:    req.IncludeQoS,
		IncludeRetain: req.IncludeRetain,
	}

	result := recording.ConvertMQTTRecordings(recordings, opts)

	writeJSON(w, http.StatusOK, MQTTConvertResponse{
		Config:       result.Config,
		TopicCount:   result.TopicCount,
		MessageCount: result.MessageCount,
		Total:        result.Total,
		Warnings:     result.Warnings,
	})
}

// handleExportMQTTRecordings handles POST /mqtt-recordings/export.
func (m *MQTTRecordingManager) handleExportMQTTRecordings(w http.ResponseWriter, r *http.Request) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.store == nil {
		writeError(w, http.StatusBadRequest, "no_store", "No recording store available")
		return
	}

	data, err := m.store.Export()
	if err != nil {
		log.Printf("Failed to export MQTT recordings: %v\n", err)
		writeError(w, http.StatusInternalServerError, "export_error", ErrMsgInternalError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=mqtt-recordings.json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}
