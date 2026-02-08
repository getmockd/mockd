// Route registration for the Admin API.

package admin

import (
	"net/http"
)

// registerRoutes sets up all API routes.
func (a *API) registerRoutes(mux *http.ServeMux) {
	// Health check, status, metrics, and ports
	mux.HandleFunc("GET /health", a.handleHealth)
	mux.HandleFunc("GET /status", a.requireEngine(a.handleGetStatus))
	mux.HandleFunc("GET /ports", a.handleListPorts)
	mux.Handle("GET /metrics", a.metricsRegistry.Handler())

	// Workspace management
	mux.HandleFunc("GET /workspaces", a.handleListWorkspaces)
	mux.HandleFunc("POST /workspaces", a.handleCreateWorkspace)
	mux.HandleFunc("GET /workspaces/{id}", a.handleGetWorkspace)
	mux.HandleFunc("PUT /workspaces/{id}", a.handleUpdateWorkspace)
	mux.HandleFunc("DELETE /workspaces/{id}", a.handleDeleteWorkspace)

	// Folder management (for organizing mocks/endpoints)
	mux.HandleFunc("GET /folders", a.handleListFolders)
	mux.HandleFunc("POST /folders", a.handleCreateFolder)
	mux.HandleFunc("GET /folders/{id}", a.handleGetFolder)
	mux.HandleFunc("PUT /folders/{id}", a.handleUpdateFolder)
	mux.HandleFunc("DELETE /folders/{id}", a.handleDeleteFolder)

	// Unified Mocks API - all operations use store interface with file persistence
	mux.HandleFunc("GET /mocks", a.handleListUnifiedMocks)
	mux.HandleFunc("POST /mocks", a.handleCreateUnifiedMock)
	mux.HandleFunc("DELETE /mocks", a.handleDeleteAllUnifiedMocks)
	mux.HandleFunc("POST /mocks/bulk", a.handleBulkCreateUnifiedMocks)
	mux.HandleFunc("GET /mocks/{id}", a.handleGetUnifiedMock)
	mux.HandleFunc("PUT /mocks/{id}", a.handleUpdateUnifiedMock)
	mux.HandleFunc("PATCH /mocks/{id}", a.handlePatchUnifiedMock)
	mux.HandleFunc("DELETE /mocks/{id}", a.handleDeleteUnifiedMock)
	mux.HandleFunc("POST /mocks/{id}/toggle", a.handleToggleUnifiedMock)

	// Mock verification
	mux.HandleFunc("GET /mocks/{id}/verify", a.requireEngine(a.handleGetMockVerification))
	mux.HandleFunc("POST /mocks/{id}/verify", a.requireEngine(a.handleVerifyMock))
	mux.HandleFunc("GET /mocks/{id}/invocations", a.requireEngine(a.handleListMockInvocations))
	mux.HandleFunc("DELETE /mocks/{id}/invocations", a.requireEngine(a.handleResetMockVerification))
	mux.HandleFunc("DELETE /verify", a.requireEngine(a.handleResetAllVerification))

	// Configuration import/export
	mux.HandleFunc("GET /config", a.requireEngine(a.handleExportConfig))
	mux.HandleFunc("POST /config", a.requireEngine(a.handleImportConfig))

	// OpenAPI/Insomnia export (for importing mocks into external tools)
	mux.HandleFunc("GET /openapi.json", a.handleGetOpenAPISpec)
	mux.HandleFunc("GET /openapi.yaml", a.handleGetOpenAPISpec)
	mux.HandleFunc("GET /insomnia.json", a.handleGetInsomniaExport) // v4 JSON format (legacy)
	mux.HandleFunc("GET /insomnia.yaml", a.handleGetInsomniaExport) // v5 YAML format (recommended)

	// Request logging
	mux.HandleFunc("GET /requests", a.requireEngine(a.handleListRequests))
	mux.HandleFunc("GET /requests/stream", a.requireEngine(a.handleStreamRequests))
	mux.HandleFunc("GET /requests/{id}", a.requireEngine(a.handleGetRequest))
	mux.HandleFunc("DELETE /requests", a.requireEngine(a.handleClearRequests))

	// Proxy management
	mux.HandleFunc("POST /proxy/start", a.proxyManager.handleProxyStart)
	mux.HandleFunc("POST /proxy/stop", a.proxyManager.handleProxyStop)
	mux.HandleFunc("GET /proxy/status", a.proxyManager.handleProxyStatus)
	mux.HandleFunc("PUT /proxy/mode", a.proxyManager.handleProxyMode)
	mux.HandleFunc("GET /proxy/filters", a.proxyManager.handleGetFilters)
	mux.HandleFunc("PUT /proxy/filters", a.proxyManager.handleSetFilters)
	mux.HandleFunc("GET /proxy/ca", a.proxyManager.handleGetCA)
	mux.HandleFunc("POST /proxy/ca", a.proxyManager.handleGenerateCA)
	mux.HandleFunc("GET /proxy/ca/download", a.proxyManager.handleDownloadCA)

	// Recording management
	mux.HandleFunc("GET /recordings", a.proxyManager.handleListRecordings)
	mux.HandleFunc("DELETE /recordings", a.proxyManager.handleClearRecordings)
	mux.HandleFunc("GET /recordings/{id}", a.proxyManager.handleGetRecording)
	mux.HandleFunc("DELETE /recordings/{id}", a.proxyManager.handleDeleteRecording)
	mux.HandleFunc("POST /recordings/convert", a.handleConvertRecordings)
	mux.HandleFunc("POST /recordings/export", a.proxyManager.handleExportRecordings)
	mux.HandleFunc("POST /recordings/{id}/to-mock", a.handleConvertSingleRecording)
	mux.HandleFunc("GET /recordings/{id}/check-sensitive", a.proxyManager.handleCheckSensitiveData)
	mux.HandleFunc("POST /recordings/{id}/preview-smart-match", a.proxyManager.handlePreviewSmartMatch)
	mux.HandleFunc("POST /recordings/sessions/{id}/to-mocks", a.handleConvertSession)

	// Session management
	mux.HandleFunc("GET /sessions", a.proxyManager.handleListSessions)
	mux.HandleFunc("POST /sessions", a.proxyManager.handleCreateSession)
	mux.HandleFunc("DELETE /sessions", a.proxyManager.handleDeleteSessions)
	mux.HandleFunc("GET /sessions/{id}", a.proxyManager.handleGetSession)
	mux.HandleFunc("DELETE /sessions/{id}", a.proxyManager.handleDeleteSession)

	// State management (stateful resources)
	mux.HandleFunc("GET /state", a.requireEngine(a.handleStateOverview))
	mux.HandleFunc("POST /state/reset", a.requireEngine(a.handleStateReset))
	mux.HandleFunc("GET /state/resources", a.requireEngine(a.handleListStateResources))
	mux.HandleFunc("GET /state/resources/{name}", a.requireEngine(a.handleGetStateResource))
	mux.HandleFunc("POST /state/resources/{name}/reset", a.requireEngine(a.handleResetStateResource))
	mux.HandleFunc("DELETE /state/resources/{name}", a.requireEngine(a.handleClearStateResource))

	// SSE connection management
	mux.HandleFunc("GET /sse/connections", a.handleListSSEConnections)
	mux.HandleFunc("GET /sse/connections/{id}", a.handleGetSSEConnection)
	mux.HandleFunc("DELETE /sse/connections/{id}", a.handleCloseSSEConnection)
	mux.HandleFunc("GET /sse/stats", a.handleGetSSEStats)

	// Mock-specific SSE endpoints
	mux.HandleFunc("GET /mocks/{id}/sse/connections", a.requireEngine(a.handleListMockSSEConnections))
	mux.HandleFunc("DELETE /mocks/{id}/sse/connections", a.requireEngine(a.handleCloseMockSSEConnections))
	mux.HandleFunc("GET /mocks/{id}/sse/buffer", a.handleGetMockSSEBuffer)
	mux.HandleFunc("DELETE /mocks/{id}/sse/buffer", a.handleClearMockSSEBuffer)

	// Protocol endpoints (use the unified /mocks API for all mock types)

	// Stream recording management (WebSocket/SSE)
	mux.HandleFunc("GET /stream-recordings", a.streamRecordingManager.handleListStreamRecordings)
	mux.HandleFunc("GET /stream-recordings/stats", a.streamRecordingManager.handleGetStreamRecordingStats)
	mux.HandleFunc("GET /stream-recordings/sessions", a.streamRecordingManager.handleGetActiveSessions)
	mux.HandleFunc("POST /stream-recordings/start", a.streamRecordingManager.handleStartRecording)
	mux.HandleFunc("POST /stream-recordings/vacuum", a.streamRecordingManager.handleVacuum)
	mux.HandleFunc("GET /stream-recordings/{id}", a.streamRecordingManager.handleGetStreamRecording)
	mux.HandleFunc("DELETE /stream-recordings/{id}", a.streamRecordingManager.handleDeleteStreamRecording)
	mux.HandleFunc("POST /stream-recordings/{id}/stop", a.streamRecordingManager.handleStopRecording)
	mux.HandleFunc("POST /stream-recordings/{id}/export", a.streamRecordingManager.handleExportStreamRecording)
	mux.HandleFunc("POST /stream-recordings/{id}/convert", a.handleConvertStreamRecording)
	mux.HandleFunc("POST /stream-recordings/{id}/replay", a.streamRecordingManager.handleStartReplay)

	// Replay session management
	mux.HandleFunc("GET /replay", a.streamRecordingManager.handleListReplaySessions)
	mux.HandleFunc("GET /replay/{id}", a.streamRecordingManager.handleGetReplayStatus)
	mux.HandleFunc("DELETE /replay/{id}", a.streamRecordingManager.handleStopReplay)
	mux.HandleFunc("POST /replay/{id}/advance", a.streamRecordingManager.handleAdvanceReplay)
	mux.HandleFunc("POST /replay/{id}/pause", a.streamRecordingManager.handlePauseReplay)
	mux.HandleFunc("POST /replay/{id}/resume", a.streamRecordingManager.handleResumeReplay)

	// Chaos injection management
	mux.HandleFunc("GET /chaos", a.requireEngine(a.handleGetChaos))
	mux.HandleFunc("PUT /chaos", a.requireEngine(a.handleSetChaos))

	// gRPC server management (convenience — proxies to /mocks?type=grpc)
	mux.HandleFunc("GET /grpc", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		q.Set("type", "grpc")
		r.URL.RawQuery = q.Encode()
		a.handleListUnifiedMocks(w, r)
	})

	// MQTT broker management
	mux.HandleFunc("GET /mqtt", a.handleListMQTTBrokers)
	mux.HandleFunc("GET /mqtt/status", a.handleGetMQTTStatus)
	mux.HandleFunc("GET /mqtt/{id}/status", a.mqttRecordingManager.handleGetMQTTBrokerStatus)
	mux.HandleFunc("POST /mqtt/{id}/record/start", a.mqttRecordingManager.handleStartMQTTRecording)
	mux.HandleFunc("POST /mqtt/{id}/record/stop", a.mqttRecordingManager.handleStopMQTTRecording)

	// MQTT recording management
	mux.HandleFunc("GET /mqtt-recordings", a.mqttRecordingManager.handleListMQTTRecordings)
	mux.HandleFunc("GET /mqtt-recordings/stats", a.mqttRecordingManager.handleGetMQTTRecordingStats)
	mux.HandleFunc("DELETE /mqtt-recordings", a.mqttRecordingManager.handleClearMQTTRecordings)
	mux.HandleFunc("POST /mqtt-recordings/convert", a.mqttRecordingManager.handleConvertMQTTRecordings)
	mux.HandleFunc("POST /mqtt-recordings/export", a.mqttRecordingManager.handleExportMQTTRecordings)
	mux.HandleFunc("GET /mqtt-recordings/{id}", a.mqttRecordingManager.handleGetMQTTRecording)
	mux.HandleFunc("DELETE /mqtt-recordings/{id}", a.mqttRecordingManager.handleDeleteMQTTRecording)
	mux.HandleFunc("POST /mqtt-recordings/{id}/convert", a.mqttRecordingManager.handleConvertMQTTRecording)

	// SOAP handler management
	mux.HandleFunc("GET /soap", a.soapRecordingManager.handleListSOAPHandlers)
	mux.HandleFunc("GET /soap/{id}/status", a.soapRecordingManager.handleGetSOAPHandlerStatus)
	mux.HandleFunc("POST /soap/{id}/record/start", a.soapRecordingManager.handleStartSOAPRecording)
	mux.HandleFunc("POST /soap/{id}/record/stop", a.soapRecordingManager.handleStopSOAPRecording)

	// SOAP recording management
	mux.HandleFunc("GET /soap-recordings", a.soapRecordingManager.handleListSOAPRecordings)
	mux.HandleFunc("GET /soap-recordings/stats", a.soapRecordingManager.handleGetSOAPRecordingStats)
	mux.HandleFunc("DELETE /soap-recordings", a.soapRecordingManager.handleClearSOAPRecordings)
	mux.HandleFunc("POST /soap-recordings/convert", a.soapRecordingManager.handleConvertSOAPRecordings)
	mux.HandleFunc("POST /soap-recordings/export", a.soapRecordingManager.handleExportSOAPRecordings)
	mux.HandleFunc("GET /soap-recordings/{id}", a.soapRecordingManager.handleGetSOAPRecording)
	mux.HandleFunc("DELETE /soap-recordings/{id}", a.soapRecordingManager.handleDeleteSOAPRecording)
	mux.HandleFunc("POST /soap-recordings/{id}/convert", a.soapRecordingManager.handleConvertSOAPRecording)

	// Preferences management
	mux.HandleFunc("GET /preferences", a.handleGetPreferences)
	mux.HandleFunc("PUT /preferences", a.handleUpdatePreferences)

	// Metadata endpoints (formats and templates)
	mux.HandleFunc("GET /formats", a.handleListFormats)
	mux.HandleFunc("GET /templates", a.handleListTemplates)
	mux.HandleFunc("POST /templates/{name}", a.requireEngine(a.handleGenerateFromTemplate))

	// Engine registry management
	mux.HandleFunc("GET /engines", a.handleListEngines)
	mux.HandleFunc("POST /engines/register", a.handleRegisterEngine)
	mux.HandleFunc("GET /engines/{id}", a.handleGetEngine)
	mux.HandleFunc("DELETE /engines/{id}", a.handleUnregisterEngine)
	mux.HandleFunc("POST /engines/{id}/heartbeat", a.handleEngineHeartbeat)
	mux.HandleFunc("PUT /engines/{id}/workspace", a.handleAssignWorkspace)
	mux.HandleFunc("GET /engines/{id}/config", a.handleGetEngineConfig)

	// Engine tunnel management
	mux.HandleFunc("POST /engines/{id}/tunnel/enable", a.handleEnableTunnel)
	mux.HandleFunc("POST /engines/{id}/tunnel/disable", a.handleDisableTunnel)
	mux.HandleFunc("GET /engines/{id}/tunnel/config", a.handleGetTunnelConfig)
	mux.HandleFunc("PUT /engines/{id}/tunnel/config", a.handleUpdateTunnelConfig)
	mux.HandleFunc("GET /engines/{id}/tunnel/status", a.handleGetTunnelStatus)
	mux.HandleFunc("POST /engines/{id}/tunnel/preview", a.handleTunnelPreview)

	// Global tunnel listing
	mux.HandleFunc("GET /tunnels", a.handleListTunnels)

	// Engine workspace management
	mux.HandleFunc("POST /engines/{id}/workspaces", a.handleAddEngineWorkspace)
	mux.HandleFunc("DELETE /engines/{id}/workspaces/{workspaceId}", a.handleRemoveEngineWorkspace)
	mux.HandleFunc("PUT /engines/{id}/workspaces/{workspaceId}", a.handleUpdateEngineWorkspace)
	mux.HandleFunc("POST /engines/{id}/workspaces/{workspaceId}/sync", a.handleSyncEngineWorkspace)

	// Workspace server control (start/stop individual workspace servers)
	mux.HandleFunc("POST /engines/{id}/workspaces/{workspaceId}/start", a.handleStartWorkspaceServer)
	mux.HandleFunc("POST /engines/{id}/workspaces/{workspaceId}/stop", a.handleStopWorkspaceServer)
	mux.HandleFunc("GET /engines/{id}/workspaces/{workspaceId}/status", a.handleGetWorkspaceServerStatus)
	mux.HandleFunc("POST /engines/{id}/workspaces/{workspaceId}/reload", a.handleReloadWorkspaceServer)

	// Token management for engine authentication
	mux.HandleFunc("POST /admin/tokens/registration", a.handleGenerateRegistrationToken)
	mux.HandleFunc("GET /admin/tokens/registration", a.handleListRegistrationTokens)

	// API key management
	mux.HandleFunc("GET /admin/api-key", a.handleGetAPIKey)
	mux.HandleFunc("POST /admin/api-key/rotate", a.handleRotateAPIKey)

	// Protocol handler management
	mux.HandleFunc("GET /handlers", a.handleListHandlers)
	mux.HandleFunc("GET /handlers/{id}", a.requireEngine(a.handleGetHandler))
	mux.HandleFunc("GET /handlers/{id}/health", a.requireEngine(a.handleGetHandlerHealth))
	mux.HandleFunc("GET /handlers/{id}/stats", a.requireEngine(a.handleGetHandlerStats))
}

// handleConvertRecordings wraps the convert handler to pass the engine client.
func (a *API) handleConvertRecordings(w http.ResponseWriter, r *http.Request) {
	a.proxyManager.handleConvertRecordings(w, r, a.localEngine)
}

// handleConvertSingleRecording wraps the single recording convert handler.
func (a *API) handleConvertSingleRecording(w http.ResponseWriter, r *http.Request) {
	a.proxyManager.handleConvertSingleRecording(w, r, a.localEngine)
}

// handleConvertSession wraps the session convert handler.
func (a *API) handleConvertSession(w http.ResponseWriter, r *http.Request) {
	a.proxyManager.handleConvertSession(w, r, a.localEngine)
}

// handleConvertStreamRecording wraps the stream recording convert handler.
func (a *API) handleConvertStreamRecording(w http.ResponseWriter, r *http.Request) {
	a.streamRecordingManager.handleConvertStreamRecording(w, r, a.localEngine)
}
