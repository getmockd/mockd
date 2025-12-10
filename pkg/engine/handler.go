package engine

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/getmockd/mockd/internal/storage"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/sse"
	"github.com/getmockd/mockd/pkg/stateful"
	"github.com/getmockd/mockd/pkg/websocket"
)

// MaxStatefulBodySize is the maximum allowed request body size for stateful POST/PUT operations (1MB).
const MaxStatefulBodySize = 1 << 20 // 1MB

// Handler handles incoming HTTP requests and matches them against configured mocks.
type Handler struct {
	store          storage.MockStore
	statefulStore  *stateful.StateStore
	logger         RequestLogger
	sseHandler     *sse.SSEHandler
	chunkedHandler *sse.ChunkedHandler
	wsManager      *websocket.ConnectionManager
}

// NewHandler creates a new Handler.
func NewHandler(store storage.MockStore) *Handler {
	return &Handler{
		store:          store,
		sseHandler:     sse.NewSSEHandler(100), // 100 max SSE connections
		chunkedHandler: sse.NewChunkedHandler(),
		wsManager:      websocket.NewConnectionManager(),
	}
}

// SetLogger sets the request logger for the handler.
func (h *Handler) SetLogger(logger RequestLogger) {
	h.logger = logger
}

// SetStatefulStore sets the stateful resource store for the handler.
func (h *Handler) SetStatefulStore(store *stateful.StateStore) {
	h.statefulStore = store
}

// ServeHTTP implements the http.Handler interface.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Check for WebSocket upgrade first
	if websocket.IsWebSocketRequest(r) {
		h.handleWebSocket(w, r)
		return
	}

	// Capture request body for logging
	var bodyBytes []byte
	if r.Body != nil {
		bodyBytes, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(NewBodyReader(bodyBytes))
	}

	// Capture headers for logging
	headers := make(map[string][]string)
	for name, values := range r.Header {
		headers[name] = values
	}

	var statusCode int
	var matchedID string

	// Check stateful resources first
	if h.statefulStore != nil {
		if resource, itemID, pathParams := h.statefulStore.MatchPath(r.URL.Path); resource != nil {
			statusCode = h.handleStateful(w, r, resource, itemID, pathParams, bodyBytes)
			matchedID = "stateful:" + resource.Name()
			h.logRequest(startTime, r, headers, bodyBytes, matchedID, statusCode)
			return
		}
	}

	// Get all mocks (already sorted by priority)
	mocks := h.store.List()

	// Find best matching mock using scoring algorithm
	match := SelectBestMatch(mocks, r)

	if match != nil {
		matchedID = match.ID

		// Check for SSE streaming response
		if match.SSE != nil {
			h.sseHandler.ServeHTTP(w, r, match)
			statusCode = http.StatusOK
			h.logRequest(startTime, r, headers, bodyBytes, matchedID, statusCode)
			return
		}

		// Check for chunked streaming response
		if match.Chunked != nil {
			h.chunkedHandler.ServeHTTP(w, r, match)
			statusCode = http.StatusOK
			h.logRequest(startTime, r, headers, bodyBytes, matchedID, statusCode)
			return
		}

		// Standard response
		statusCode = match.Response.StatusCode
		h.writeResponse(w, match.Response)
	} else {
		// No match found - return 404 with informative message
		statusCode = http.StatusNotFound
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		w.Write([]byte(`{"error": "no_match", "message": "No mock matched the request", "path": "` + r.URL.Path + `", "method": "` + r.Method + `"}`))
	}

	// Log the request
	h.logRequest(startTime, r, headers, bodyBytes, matchedID, statusCode)
}

// logRequest logs a request to the logger.
func (h *Handler) logRequest(startTime time.Time, r *http.Request, headers map[string][]string, bodyBytes []byte, matchedID string, statusCode int) {
	if h.logger != nil {
		entry := &config.RequestLogEntry{
			Timestamp:      startTime,
			Method:         r.Method,
			Path:           r.URL.Path,
			QueryString:    r.URL.RawQuery,
			Headers:        headers,
			Body:           string(bodyBytes),
			BodySize:       len(bodyBytes),
			RemoteAddr:     r.RemoteAddr,
			MatchedMockID:  matchedID,
			ResponseStatus: statusCode,
			DurationMs:     int(time.Since(startTime).Milliseconds()),
		}
		h.logger.Log(entry)
	}
}

// handleStateful handles CRUD operations for stateful resources.
func (h *Handler) handleStateful(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, itemID string, pathParams map[string]string, bodyBytes []byte) int {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		if itemID != "" {
			return h.handleStatefulGet(w, resource, itemID)
		}
		return h.handleStatefulList(w, r, resource, pathParams)

	case http.MethodPost:
		return h.handleStatefulCreate(w, resource, pathParams, bodyBytes)

	case http.MethodPut:
		if itemID == "" {
			return h.writeStatefulError(w, http.StatusBadRequest, "ID required for PUT", resource.Name(), "")
		}
		return h.handleStatefulUpdate(w, resource, itemID, bodyBytes)

	case http.MethodDelete:
		if itemID == "" {
			return h.writeStatefulError(w, http.StatusBadRequest, "ID required for DELETE", resource.Name(), "")
		}
		return h.handleStatefulDelete(w, resource, itemID)

	default:
		return h.writeStatefulError(w, http.StatusMethodNotAllowed, "method not allowed", resource.Name(), "")
	}
}

// handleStatefulGet retrieves a single item by ID.
func (h *Handler) handleStatefulGet(w http.ResponseWriter, resource *stateful.StatefulResource, itemID string) int {
	item := resource.Get(itemID)
	if item == nil {
		return h.writeStatefulError(w, http.StatusNotFound, "resource not found", resource.Name(), itemID)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(item.ToJSON())
	return http.StatusOK
}

// handleStatefulList returns a paginated collection of items.
func (h *Handler) handleStatefulList(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, pathParams map[string]string) int {
	filter := h.parseQueryFilter(r, resource, pathParams)
	result := resource.List(filter)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
	return http.StatusOK
}

// handleStatefulCreate creates a new item.
func (h *Handler) handleStatefulCreate(w http.ResponseWriter, resource *stateful.StatefulResource, pathParams map[string]string, bodyBytes []byte) int {
	// Check body size limit
	if len(bodyBytes) > MaxStatefulBodySize {
		return h.writeStatefulErrorWithHint(w, http.StatusRequestEntityTooLarge,
			"request body too large",
			resource.Name(), "",
			"Reduce request body size to under 1MB")
	}

	var data map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return h.writeStatefulError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), resource.Name(), "")
	}

	item, err := resource.Create(data, pathParams)
	if err != nil {
		if _, ok := err.(*stateful.ConflictError); ok {
			return h.writeStatefulError(w, http.StatusConflict, "resource already exists", resource.Name(), data["id"].(string))
		}
		return h.writeStatefulError(w, http.StatusInternalServerError, err.Error(), resource.Name(), "")
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item.ToJSON())
	return http.StatusCreated
}

// handleStatefulUpdate updates an existing item.
func (h *Handler) handleStatefulUpdate(w http.ResponseWriter, resource *stateful.StatefulResource, itemID string, bodyBytes []byte) int {
	// Check body size limit
	if len(bodyBytes) > MaxStatefulBodySize {
		return h.writeStatefulErrorWithHint(w, http.StatusRequestEntityTooLarge,
			"request body too large",
			resource.Name(), itemID,
			"Reduce request body size to under 1MB")
	}

	var data map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return h.writeStatefulError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), resource.Name(), itemID)
	}

	item, err := resource.Update(itemID, data)
	if err != nil {
		if _, ok := err.(*stateful.NotFoundError); ok {
			return h.writeStatefulError(w, http.StatusNotFound, "resource not found", resource.Name(), itemID)
		}
		return h.writeStatefulError(w, http.StatusInternalServerError, err.Error(), resource.Name(), itemID)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(item.ToJSON())
	return http.StatusOK
}

// handleStatefulDelete removes an item.
func (h *Handler) handleStatefulDelete(w http.ResponseWriter, resource *stateful.StatefulResource, itemID string) int {
	err := resource.Delete(itemID)
	if err != nil {
		if _, ok := err.(*stateful.NotFoundError); ok {
			return h.writeStatefulError(w, http.StatusNotFound, "resource not found", resource.Name(), itemID)
		}
		return h.writeStatefulError(w, http.StatusInternalServerError, err.Error(), resource.Name(), itemID)
	}

	w.WriteHeader(http.StatusNoContent)
	return http.StatusNoContent
}

// writeStatefulError writes a JSON error response.
func (h *Handler) writeStatefulError(w http.ResponseWriter, statusCode int, errorMsg, resource, id string) int {
	w.WriteHeader(statusCode)
	resp := stateful.ErrorResponse{
		Error:      errorMsg,
		Resource:   resource,
		ID:         id,
		StatusCode: statusCode,
	}
	json.NewEncoder(w).Encode(resp)
	return statusCode
}

// writeStatefulErrorWithHint writes a JSON error response with a resolution hint.
func (h *Handler) writeStatefulErrorWithHint(w http.ResponseWriter, statusCode int, errorMsg, resource, id, hint string) int {
	w.WriteHeader(statusCode)
	resp := stateful.ErrorResponse{
		Error:      errorMsg,
		Resource:   resource,
		ID:         id,
		StatusCode: statusCode,
		Hint:       hint,
	}
	json.NewEncoder(w).Encode(resp)
	return statusCode
}

// parseQueryFilter extracts filter parameters from query string.
func (h *Handler) parseQueryFilter(r *http.Request, resource *stateful.StatefulResource, pathParams map[string]string) *stateful.QueryFilter {
	filter := stateful.DefaultQueryFilter()

	query := r.URL.Query()

	// Parse pagination params
	if limit := query.Get("limit"); limit != "" {
		var l int
		if _, err := parseIntParam(limit, &l); err == nil && l > 0 {
			filter.Limit = l
		}
	}

	if offset := query.Get("offset"); offset != "" {
		var o int
		if _, err := parseIntParam(offset, &o); err == nil && o >= 0 {
			filter.Offset = o
		}
	}

	// Parse sort params
	if sort := query.Get("sort"); sort != "" {
		filter.Sort = sort
	}

	if order := query.Get("order"); order != "" {
		filter.Order = order
	}

	// Set parent filter for nested resources
	if parentField := resource.ParentField(); parentField != "" {
		if parentID, ok := pathParams[parentField]; ok {
			filter.ParentField = parentField
			filter.ParentID = parentID
		}
	}

	// All other params are field filters
	reserved := map[string]bool{"limit": true, "offset": true, "sort": true, "order": true}
	for key, values := range query {
		if !reserved[key] && len(values) > 0 {
			filter.Filters[key] = values[0]
		}
	}

	return filter
}

// parseIntParam parses an integer parameter.
func parseIntParam(s string, v *int) (int, error) {
	var n int
	_, err := func() (int, error) {
		for _, c := range s {
			if c < '0' || c > '9' {
				return 0, io.EOF
			}
			n = n*10 + int(c-'0')
		}
		*v = n
		return n, nil
	}()
	return n, err
}

// matches checks if the request matches the given matcher.
func (h *Handler) matches(matcher *config.RequestMatcher, r *http.Request) bool {
	if matcher == nil {
		return false
	}

	// Check method
	if matcher.Method != "" {
		if !strings.EqualFold(r.Method, matcher.Method) {
			return false
		}
	}

	// Check path
	if matcher.Path != "" {
		if !h.matchPath(matcher.Path, r.URL.Path) {
			return false
		}
	}

	// Check headers
	for name, value := range matcher.Headers {
		reqValue := r.Header.Get(name)
		if reqValue != value {
			return false
		}
	}

	// Check query parameters
	for name, value := range matcher.QueryParams {
		reqValue := r.URL.Query().Get(name)
		if reqValue != value {
			return false
		}
	}

	// Body matching will be implemented in Phase 5 (Request Matching)
	// For now, skip body matching

	return true
}

// matchPath checks if the request path matches the matcher path.
// Supports exact matching and simple wildcards (*).
func (h *Handler) matchPath(pattern, path string) bool {
	// Exact match
	if pattern == path {
		return true
	}

	// Simple wildcard matching (e.g., /api/users/*)
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(path, prefix+"/") || path == prefix
	}

	// Wildcard anywhere (e.g., /api/*/users)
	if strings.Contains(pattern, "*") {
		return h.matchWildcard(pattern, path)
	}

	return false
}

// matchWildcard performs simple wildcard matching.
func (h *Handler) matchWildcard(pattern, path string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == path
	}

	// Check prefix
	if !strings.HasPrefix(path, parts[0]) {
		return false
	}

	remaining := strings.TrimPrefix(path, parts[0])

	// Check suffix
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		idx := strings.Index(remaining, parts[i])
		if idx == -1 {
			return false
		}
		remaining = remaining[idx+len(parts[i]):]
	}

	return true
}

// writeResponse writes the mock response to the HTTP response writer.
func (h *Handler) writeResponse(w http.ResponseWriter, resp *config.ResponseDefinition) {
	// Apply delay if specified
	if resp.DelayMs > 0 {
		time.Sleep(time.Duration(resp.DelayMs) * time.Millisecond)
	}

	// Set headers
	for name, value := range resp.Headers {
		w.Header().Set(name, value)
	}

	// Set default Content-Type if not specified
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "text/plain")
	}

	// Write status code
	w.WriteHeader(resp.StatusCode)

	// Write body
	if resp.Body != "" {
		_, _ = w.Write([]byte(resp.Body))
	}
}

// SSEHandler returns the SSE handler for admin API access.
func (h *Handler) SSEHandler() *sse.SSEHandler {
	return h.sseHandler
}

// ChunkedHandler returns the chunked transfer handler.
func (h *Handler) ChunkedHandler() *sse.ChunkedHandler {
	return h.chunkedHandler
}

// WebSocketManager returns the WebSocket connection manager.
func (h *Handler) WebSocketManager() *websocket.ConnectionManager {
	return h.wsManager
}

// handleWebSocket handles WebSocket upgrade requests.
func (h *Handler) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Find matching endpoint
	endpoint := h.wsManager.GetEndpoint(r.URL.Path)
	if endpoint == nil {
		http.Error(w, `{"error": "websocket_endpoint_not_found", "path": "`+r.URL.Path+`"}`, http.StatusNotFound)
		return
	}

	// Handle upgrade
	if err := endpoint.HandleUpgrade(w, r); err != nil {
		// Error already written to response by HandleUpgrade
		return
	}
}

// RegisterWebSocketEndpoint registers a WebSocket endpoint from config.
func (h *Handler) RegisterWebSocketEndpoint(cfg *config.WebSocketEndpointConfig) error {
	endpoint, err := websocket.EndpointFromConfig(cfg)
	if err != nil {
		return err
	}
	h.wsManager.RegisterEndpoint(endpoint)
	return nil
}
