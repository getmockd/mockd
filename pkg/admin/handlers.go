package admin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	types "github.com/getmockd/mockd/pkg/api/types"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/httputil"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/requestlog"
	"github.com/getmockd/mockd/pkg/store"
	"gopkg.in/yaml.v3"
)

// Type aliases pointing to the canonical shared types.
type (
	ErrorResponse    = types.ErrorResponse
	HealthResponse   = types.HealthResponse
	ServerStatus     = types.ServerStatus
	MockListResponse = types.MockListResponse
)

// writeJSON writes a JSON response using the shared httputil package.
func writeJSON(w http.ResponseWriter, status int, data any) {
	httputil.WriteJSON(w, status, data)
}

// writeError writes an error response.
func writeError(w http.ResponseWriter, status int, errCode, message string) {
	httputil.WriteJSON(w, status, ErrorResponse{
		Error:   errCode,
		Message: message,
	})
}

// handleHealth handles GET /health.
func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:    "ok",
		Uptime:    a.Uptime(),
		Timestamp: time.Now().UTC(),
	})
}

// handleGetStatus handles GET /status and returns detailed server status.
func (a *API) handleGetStatus(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	engineStatus, err := engine.Status(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.logger(), "get engine status"))
		return
	}

	// Count active mocks from engine status
	mocks, err := engine.ListMocks(ctx)
	if err != nil {
		// Log the error but continue with zero active mocks count
		// This is non-critical for the status endpoint
		mocks = nil
	}
	activeMocks := 0
	for _, mock := range mocks {
		if mock.Enabled == nil || *mock.Enabled {
			activeMocks++
		}
	}

	version := a.version
	if version == "" {
		version = "dev"
	}

	writeJSON(w, http.StatusOK, ServerStatus{
		Status:       engineStatus.Status,
		HTTPPort:     0, // TODO: Get from engine config when available via HTTP
		HTTPSPort:    0,
		AdminPort:    a.port,
		Uptime:       engineStatus.Uptime,
		MockCount:    engineStatus.MockCount,
		ActiveMocks:  activeMocks,
		RequestCount: engineStatus.RequestCount,
		TLSEnabled:   false,
		Version:      version,
	})
}

// ConfigImportRequest represents a config import request.
type ConfigImportRequest struct {
	Replace bool                   `json:"replace"`
	Config  *config.MockCollection `json:"config"`
}

// handleExportConfig handles GET /config.
func (a *API) handleExportConfig(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "mockd-export"
	}

	collection, err := engine.ExportConfig(ctx, name)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.logger(), "export config"))
		return
	}

	// Support YAML export via ?format=yaml query parameter.
	if strings.EqualFold(r.URL.Query().Get("format"), "yaml") {
		out, err := yaml.Marshal(collection)
		if err != nil {
			a.logger().Error("failed to marshal YAML export", "error", err)
			writeError(w, http.StatusInternalServerError, "export_error", ErrMsgInternalError)
			return
		}
		w.Header().Set("Content-Type", "application/x-yaml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(out)
		return
	}

	writeJSON(w, http.StatusOK, collection)
}

// decodeImportRequest reads and decodes a ConfigImportRequest from the HTTP
// request body, handling both YAML and JSON content types. It writes an HTTP
// error and returns a non-nil error on failure.
func (a *API) decodeImportRequest(w http.ResponseWriter, r *http.Request) (*ConfigImportRequest, error) {
	var req ConfigImportRequest

	// Override the default body limit — config imports can be large.
	const maxImportBodySize = 10 << 20 // 10MB
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBodySize)

	// Read the full body so we can try multiple decode strategies.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "body_too_large", "Request body too large")
			return nil, err
		}
		writeError(w, http.StatusBadRequest, "read_error", "Failed to read request body")
		return nil, err
	}

	// Detect YAML content type and decode accordingly.
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "yaml") {
		if err := yaml.Unmarshal(body, &req); err != nil {
			a.logger().Debug("YAML parsing failed", "error", err)
			writeError(w, http.StatusBadRequest, "invalid_yaml", "Invalid YAML in request body")
			return nil, err
		}
	} else {
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSONDecodeError(w, err, a.logger())
			return nil, err
		}
	}

	// Support unwrapped format: if the body has no "config" wrapper but
	// contains a "mocks" key, treat the entire body as a MockCollection.
	// This allows the output of GET /config (export) to be directly
	// re-imported via POST /config without manual wrapping.
	if req.Config == nil {
		var collection config.MockCollection
		if strings.Contains(ct, "yaml") {
			if err := yaml.Unmarshal(body, &collection); err == nil && collection.Mocks != nil {
				req.Config = &collection
			}
		} else {
			if err := json.Unmarshal(body, &collection); err == nil && collection.Mocks != nil {
				req.Config = &collection
			}
		}
	}

	if req.Config == nil {
		writeError(w, http.StatusBadRequest, "missing_config", "config field is required")
		return nil, errors.New("missing config")
	}

	return &req, nil
}

// persistStatefulResources dual-writes stateful resources from the imported
// config into the file store so they survive restarts.
func (a *API) persistStatefulResources(ctx context.Context, cfg *config.MockCollection, replace bool) {
	if len(cfg.StatefulResources) == 0 || a.dataStore == nil {
		return
	}
	resStore := a.dataStore.StatefulResources()
	if replace {
		_ = resStore.DeleteAll(ctx)
	}
	for _, res := range cfg.StatefulResources {
		if res == nil {
			continue
		}
		if err := resStore.Create(ctx, res); err != nil {
			if errors.Is(err, store.ErrAlreadyExists) {
				// Resource already exists; on replace we already cleared, so this
				// shouldn't happen, but handle gracefully.
				a.logger().Debug("stateful resource already exists in file store", "name", res.Name)
			} else {
				a.logger().Warn("failed to write stateful resource to file store",
					"name", res.Name, "error", err)
			}
		}
	}
}

// preValidateImportMocks validates each mock that has a type and protocol config,
// returning an error that identifies the failing mock by index and ID.
func preValidateImportMocks(mocks []*mock.Mock) error {
	for idx, m := range mocks {
		if m == nil || m.Type == "" {
			continue
		}
		// Only validate mocks that carry protocol-specific config; bare
		// stubs (type set but no spec) are allowed to pass through.
		hasSpec := m.HTTP != nil || m.GRPC != nil || m.MQTT != nil ||
			m.GraphQL != nil || m.WebSocket != nil || m.SOAP != nil || m.OAuth != nil
		if !hasSpec {
			continue
		}
		if m.ID == "" {
			m.ID = generateMockID(m.Type)
		}
		if err := m.Validate(); err != nil {
			return fmt.Errorf("mock at index %d (id=%q): %s", idx, m.ID, err.Error())
		}
	}
	return nil
}

// handleImportConfig handles POST /config.
func (a *API) handleImportConfig(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	req, err := a.decodeImportRequest(w, r)
	if err != nil {
		return
	}

	// Allow replace=true via query param (in addition to JSON body field).
	if replace := parseOptionalBool(r.URL.Query().Get("replace")); replace != nil && *replace {
		req.Replace = true
	}

	// If dryRun=true, validate and return a preview without applying changes.
	if dryRun := parseOptionalBool(r.URL.Query().Get("dryRun")); dryRun != nil && *dryRun {
		mockCount := 0
		for _, m := range req.Config.Mocks {
			if m != nil {
				mockCount++
			}
		}
		result := map[string]any{
			"dryRun": true,
			"mocks":  mockCount,
		}
		if len(req.Config.StatefulResources) > 0 {
			result["statefulResources"] = len(req.Config.StatefulResources)
		}
		writeJSON(w, http.StatusOK, result)
		return
	}

	mockStore := a.getMockStore()

	// Default workspaceId for imported mocks (consistent with POST /mocks and POST /mocks/bulk).
	workspaceID := r.URL.Query().Get("workspaceId")
	if workspaceID == "" {
		workspaceID = store.DefaultWorkspaceID
	}

	now := time.Now()
	for _, m := range req.Config.Mocks {
		if m == nil {
			continue
		}
		if m.WorkspaceID == "" {
			m.WorkspaceID = workspaceID
		}
		// Ensure timestamps are set so imported mocks look like normal mocks.
		if m.CreatedAt.IsZero() {
			m.CreatedAt = now
		}
		m.UpdatedAt = now
	}

	// If replacing, clear the file store first so we don't leave stale entries.
	if req.Replace && mockStore != nil {
		// Delete all existing mocks from the file store in this workspace.
		existing, _ := mockStore.List(ctx, nil)
		for _, em := range existing {
			_ = mockStore.Delete(ctx, em.ID)
		}
	}

	// Write imported mocks to the admin file store FIRST (dual-write pattern).
	// This ensures DELETE /mocks/{id} can find them later.
	imported := 0
	if mockStore != nil {
		for _, m := range req.Config.Mocks {
			if m == nil {
				continue
			}
			// Generate ID if not provided
			if m.ID == "" {
				m.ID = generateMockID(m.Type)
			}
			// Use Create (skip duplicates) to populate the file store.
			if err := mockStore.Create(ctx, m); err != nil {
				// If it already exists, update it instead.
				if errors.Is(err, store.ErrAlreadyExists) {
					_ = mockStore.Update(ctx, m)
				} else {
					a.logger().Warn("failed to write imported mock to file store",
						"id", m.ID, "error", err)
				}
			}
		}
	}

	// Dual-write stateful resources to the file store so they survive restarts.
	a.persistStatefulResources(ctx, req.Config, req.Replace)

	// Pre-validate mocks so we can surface which mock (by index) is invalid.
	if err := preValidateImportMocks(req.Config.Mocks); err != nil {
		writeError(w, http.StatusBadRequest, "validation_error", err.Error())
		return
	}

	// Forward to engine for runtime registration (starts gRPC/MQTT servers, registers handlers).
	importResult, err := engine.ImportConfig(ctx, req.Config, req.Replace)
	if err != nil {
		writeError(w, http.StatusBadRequest, "import_error", sanitizeError(err, a.logger(), "import config"))
		return
	}

	// Use the engine's actual import counts, not the submitted count.
	imported = importResult.Imported

	// Get the updated state
	collection, _ := engine.ExportConfig(ctx, "imported")
	total := 0
	if collection != nil {
		total = len(collection.Mocks)
	}
	response := map[string]any{
		"message":  "Configuration imported successfully",
		"imported": imported,
		"total":    total,
	}
	if len(req.Config.StatefulResources) > 0 {
		response["statefulResources"] = len(req.Config.StatefulResources)
	}
	if len(importResult.Errors) > 0 {
		response["warnings"] = importResult.Errors
		response["message"] = fmt.Sprintf("Imported %d of %d mocks (%d failed)", imported, importResult.Total, len(importResult.Errors))
	}
	writeJSON(w, http.StatusOK, response)
}

// handleListRequests handles GET /requests.
// Supports filtering by protocol, method, path, and protocol-specific fields.
//
// Query Parameters:
//   - protocol: Filter by protocol (http, grpc, websocket, sse, mqtt, soap, graphql)
//   - method: Filter by method (HTTP method, gRPC method, MQTT PUBLISH/SUBSCRIBE, etc.)
//   - path: Filter by path prefix (or topic pattern for MQTT)
//   - matched: Filter by matched mock ID
//   - status: Filter by response status code
//   - hasError: Filter by error presence (true/false)
//   - limit: Maximum number of entries to return
//   - offset: Pagination offset
//
// Protocol-specific filters:
//   - grpcService: Filter gRPC by service name
//   - mqttTopic: Filter MQTT by topic (supports wildcards + and #)
//   - mqttClientId: Filter MQTT by client ID
//   - soapOperation: Filter SOAP by operation name
//   - graphqlOpType: Filter GraphQL by operation type (query, mutation, subscription)
//   - wsConnectionId: Filter WebSocket by connection ID
//   - sseConnectionId: Filter SSE by connection ID
func (a *API) handleListRequests(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	// Build filter from query parameters
	clientFilter := buildRequestFilter(r.URL.Query())
	result, err := engine.ListRequests(ctx, clientFilter)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.logger(), "list requests"))
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func buildRequestFilter(query interface{ Get(string) string }) *requestlog.Filter {
	clientFilter := &requestlog.Filter{}
	if limitStr := query.Get("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil {
			clientFilter.Limit = limit
		}
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		var offset int
		if _, err := fmt.Sscanf(offsetStr, "%d", &offset); err == nil {
			clientFilter.Offset = offset
		}
	}
	if protocol := query.Get("protocol"); protocol != "" {
		clientFilter.Protocol = protocol
	}
	if method := query.Get("method"); method != "" {
		clientFilter.Method = method
	}
	if path := query.Get("path"); path != "" {
		clientFilter.Path = path
	}
	if matched := query.Get("matched"); matched != "" {
		clientFilter.MatchedID = matched
	}
	if status := query.Get("status"); status != "" {
		if code, err := strconv.Atoi(status); err == nil {
			clientFilter.StatusCode = code
		}
	}
	if hasError := query.Get("hasError"); hasError != "" {
		if parsed, err := strconv.ParseBool(hasError); err == nil {
			clientFilter.HasError = &parsed
		}
	}
	// Protocol-specific filters
	if v := query.Get("grpcService"); v != "" {
		clientFilter.GRPCService = v
	}
	if v := query.Get("mqttTopic"); v != "" {
		clientFilter.MQTTTopic = v
	}
	if v := query.Get("mqttClientId"); v != "" {
		clientFilter.MQTTClientID = v
	}
	if v := query.Get("soapOperation"); v != "" {
		clientFilter.SOAPOperation = v
	}
	if v := query.Get("graphqlOpType"); v != "" {
		clientFilter.GraphQLOpType = v
	}
	if v := query.Get("wsConnectionId"); v != "" {
		clientFilter.WSConnectionID = v
	}
	if v := query.Get("sseConnectionId"); v != "" {
		clientFilter.SSEConnectionID = v
	}
	return clientFilter
}

// handleGetRequest handles GET /requests/{id}.
func (a *API) handleGetRequest(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Request ID is required")
		return
	}

	entry, err := engine.GetRequest(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", ErrMsgNotFound)
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.logger(), "get request"))
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

// handleClearRequests handles DELETE /requests.
func (a *API) handleClearRequests(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	count, err := engine.ClearRequests(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_unavailable", sanitizeEngineError(err, a.logger(), "clear requests"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "Request logs cleared",
		"cleared": count,
	})
}

// handleStreamRequests handles GET /requests/stream - SSE endpoint for streaming new requests.
func (a *API) handleStreamRequests(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	// Set SSE headers (CORS is handled by the middleware — do not duplicate here)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Get the flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "sse_error", "Streaming not supported")
		return
	}

	// Send initial connection message
	_, _ = fmt.Fprintf(w, "event: connected\ndata: {\"message\": \"Connected to request stream\"}\n\n")
	flusher.Flush()

	// Poll for request log updates
	ctx := r.Context()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastID := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get latest requests from engine
			filter := &requestlog.Filter{Limit: 10}
			result, err := engine.ListRequests(ctx, filter)
			if err != nil {
				continue
			}

			// Send new entries (result.Requests is newest first)
			for i := len(result.Requests) - 1; i >= 0; i-- {
				entry := result.Requests[i]
				if entry.ID == lastID {
					break
				}
				if lastID == "" && i < len(result.Requests)-1 {
					// First iteration, only send most recent
					continue
				}

				data, _ := json.Marshal(entry)
				_, _ = fmt.Fprintf(w, "event: request\ndata: %s\n\n", data)
				flusher.Flush()

				if i == 0 {
					lastID = entry.ID
				}
			}
			if len(result.Requests) > 0 && lastID == "" {
				lastID = result.Requests[0].ID
			}
		}
	}
}
