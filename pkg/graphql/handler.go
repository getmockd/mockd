package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/metrics"
	"github.com/getmockd/mockd/pkg/protocol"
	"github.com/getmockd/mockd/pkg/requestlog"
)

// Interface compliance checks
var _ protocol.Handler = (*Handler)(nil)
var _ protocol.HTTPHandler = (*Handler)(nil)
var _ protocol.RequestLoggable = (*Handler)(nil)

// MaxRequestBodySize is the maximum allowed request body size (1MB).
const MaxRequestBodySize = 1 << 20 // 1MB

// MaxLogBodySize is the maximum body size to include in logs (10KB).
const MaxLogBodySize = 10 * 1024

// Handler handles GraphQL HTTP requests.
type Handler struct {
	executor        *Executor
	config          *GraphQLConfig
	requestLoggerMu sync.RWMutex
	requestLogger   requestlog.Logger
}

// NewHandler creates a new GraphQL HTTP handler.
func NewHandler(executor *Executor, config *GraphQLConfig) *Handler {
	return &Handler{
		executor: executor,
		config:   config,
	}
}

// SetRequestLogger sets the request logger for this handler.
// This method is thread-safe.
func (h *Handler) SetRequestLogger(logger requestlog.Logger) {
	h.requestLoggerMu.Lock()
	defer h.requestLoggerMu.Unlock()
	h.requestLogger = logger
}

// GetRequestLogger returns the request logger for this handler.
// This method is thread-safe.
func (h *Handler) GetRequestLogger() requestlog.Logger {
	h.requestLoggerMu.RLock()
	defer h.requestLoggerMu.RUnlock()
	return h.requestLogger
}

// Protocol returns the protocol type for this handler.
func (h *Handler) Protocol() protocol.Protocol {
	return protocol.ProtocolGraphQL
}

// ID returns the unique identifier for this handler instance.
func (h *Handler) ID() string {
	if h.config == nil {
		return ""
	}
	return h.config.ID
}

// Metadata returns descriptive information about this handler.
func (h *Handler) Metadata() protocol.Metadata {
	id := ""
	name := ""
	if h.config != nil {
		id = h.config.ID
		name = h.config.Name
	}
	return protocol.Metadata{
		ID:                   id,
		Name:                 name,
		Protocol:             protocol.ProtocolGraphQL,
		Version:              "0.2.4",
		TransportType:        protocol.TransportHTTP1,
		ConnectionModel:      protocol.ConnectionModelStateless,
		CommunicationPattern: protocol.PatternRequestResponse,
		Capabilities: []protocol.Capability{
			protocol.CapabilitySchemaIntrospect,
			protocol.CapabilitySubscriptions,
			protocol.CapabilityMocking,
		},
	}
}

// Start activates the handler.
// For HTTP-based handlers, this is a no-op as the handler is activated
// by registering it with an http.ServeMux.
func (h *Handler) Start(ctx context.Context) error {
	return nil
}

// Stop gracefully shuts down the handler.
// For HTTP-based handlers, this is a no-op as shutdown is handled
// by the HTTP server.
func (h *Handler) Stop(ctx context.Context, timeout time.Duration) error {
	return nil
}

// Health returns the current health status of the handler.
func (h *Handler) Health(ctx context.Context) protocol.HealthStatus {
	return protocol.HealthStatus{
		Status:    protocol.HealthHealthy,
		CheckedAt: time.Now(),
	}
}

// Pattern returns the URL pattern this handler serves.
func (h *Handler) Pattern() string {
	if h.config == nil {
		return "/graphql"
	}
	return h.config.Path
}

// ServeHTTP handles POST /graphql requests.
// It supports both application/json and application/graphql content types.
// Note: CORS is handled by the engine's CORSMiddleware, not directly here.
// This ensures CORS configuration is respected rather than using hardcoded wildcards.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()

	// Handle preflight requests (CORS headers are set by middleware)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	// Only allow GET and POST methods
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		h.recordMetrics(r.URL.Path, "unknown", http.StatusMethodNotAllowed, time.Since(startTime))
		return
	}

	var req *GraphQLRequest
	var err error
	var rawBody string

	if r.Method == http.MethodGet {
		req, err = h.parseGetRequest(r)
		// For GET requests, reconstruct the query body for logging
		if req != nil {
			rawBody = h.formatRequestBody(req)
		}
	} else {
		req, rawBody, err = h.parsePostRequestWithBody(r)
	}

	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		h.logRequest(r, startTime, rawBody, nil, req, http.StatusBadRequest, err.Error())
		h.recordMetrics(r.URL.Path, h.getOperationType(req), http.StatusBadRequest, time.Since(startTime))
		return
	}

	// Execute the GraphQL request
	resp := h.executor.Execute(r.Context(), req)

	// Write the response and capture response body
	respBody := h.writeResponseWithCapture(w, resp)

	// Log the request
	h.logRequest(r, startTime, rawBody, resp, req, http.StatusOK, respBody)

	// Record metrics
	h.recordMetrics(r.URL.Path, h.getOperationType(req), http.StatusOK, time.Since(startTime))
}

// recordMetrics records GraphQL request metrics.
func (h *Handler) recordMetrics(path, _ string, status int, duration time.Duration) {
	if metrics.RequestsTotal != nil {
		statusStr := strconv.Itoa(status)
		if vec, err := metrics.RequestsTotal.WithLabels("graphql", path, statusStr); err == nil {
			_ = vec.Inc()
		}
	}
	if metrics.RequestDuration != nil {
		if vec, err := metrics.RequestDuration.WithLabels("graphql", path); err == nil {
			vec.Observe(duration.Seconds())
		}
	}
}

// getOperationType extracts the operation type from a GraphQL request.
func (h *Handler) getOperationType(req *GraphQLRequest) string {
	if req == nil || req.Query == "" {
		return "unknown"
	}
	query := strings.TrimSpace(req.Query)
	if strings.HasPrefix(query, "mutation") {
		return "mutation"
	}
	if strings.HasPrefix(query, "subscription") {
		return "subscription"
	}
	return "query"
}

// parseGetRequest parses a GraphQL request from GET query parameters.
func (h *Handler) parseGetRequest(r *http.Request) (*GraphQLRequest, error) {
	query := r.URL.Query()

	req := &GraphQLRequest{
		Query:         query.Get("query"),
		OperationName: query.Get("operationName"),
	}

	// Parse variables if provided
	if varsStr := query.Get("variables"); varsStr != "" {
		var variables map[string]interface{}
		if err := json.Unmarshal([]byte(varsStr), &variables); err != nil {
			return nil, &parseError{message: "invalid variables JSON"}
		}
		req.Variables = variables
	}

	return req, nil
}

// writeError writes an error response.
func (h *Handler) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := &GraphQLResponse{
		Errors: []GraphQLError{{Message: message}},
	}

	_ = json.NewEncoder(w).Encode(resp)
}

// writeResponseWithCapture writes a GraphQL response and returns the response body as a string.
func (h *Handler) writeResponseWithCapture(w http.ResponseWriter, resp *GraphQLResponse) string {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		return ""
	}

	respBody := buf.String()
	_, _ = w.Write(buf.Bytes())

	return respBody
}

// parsePostRequestWithBody parses a GraphQL request from POST body and returns the raw body.
func (h *Handler) parsePostRequestWithBody(r *http.Request) (*GraphQLRequest, string, error) {
	// Check content type
	contentType := r.Header.Get("Content-Type")

	// Limit request body size
	body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestBodySize))
	if err != nil {
		return nil, "", &parseError{message: "failed to read request body"}
	}
	defer func() { _ = r.Body.Close() }()

	rawBody := string(body)

	if len(body) == 0 {
		return nil, rawBody, &parseError{message: "empty request body"}
	}

	// Handle application/graphql content type
	if strings.HasPrefix(contentType, "application/graphql") {
		return &GraphQLRequest{Query: string(body)}, rawBody, nil
	}

	// Default to application/json
	var req GraphQLRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, rawBody, &parseError{message: "invalid JSON request body"}
	}

	return &req, rawBody, nil
}

// formatRequestBody formats a GraphQL request for logging.
func (h *Handler) formatRequestBody(req *GraphQLRequest) string {
	if req == nil {
		return ""
	}

	data := map[string]interface{}{
		"query": req.Query,
	}
	if req.OperationName != "" {
		data["operationName"] = req.OperationName
	}
	if len(req.Variables) > 0 {
		data["variables"] = req.Variables
	}

	b, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	return string(b)
}

// logRequest logs a GraphQL request if a logger is configured.
func (h *Handler) logRequest(r *http.Request, startTime time.Time, rawBody string, resp *GraphQLResponse, req *GraphQLRequest, statusCode int, respBody string) {
	if h.requestLogger == nil {
		return
	}

	durationMs := int(time.Since(startTime).Milliseconds())

	// Truncate body if too large
	logBody := rawBody
	if len(logBody) > MaxLogBodySize {
		logBody = logBody[:MaxLogBodySize] + "...[truncated]"
	}

	logRespBody := respBody
	if len(logRespBody) > MaxLogBodySize {
		logRespBody = logRespBody[:MaxLogBodySize] + "...[truncated]"
	}

	// Build GraphQL metadata
	var gqlMeta *requestlog.GraphQLMeta
	if req != nil {
		gqlMeta = &requestlog.GraphQLMeta{
			OperationType: h.detectOperationType(req.Query),
			OperationName: req.OperationName,
		}

		// Serialize variables
		if len(req.Variables) > 0 {
			if varsBytes, err := json.Marshal(req.Variables); err == nil {
				vars := string(varsBytes)
				if len(vars) > MaxLogBodySize {
					vars = vars[:MaxLogBodySize] + "...[truncated]"
				}
				gqlMeta.Variables = vars
			}
		}
	}

	// Check for errors in response
	if resp != nil && len(resp.Errors) > 0 {
		if gqlMeta == nil {
			gqlMeta = &requestlog.GraphQLMeta{}
		}
		gqlMeta.HasErrors = true
		gqlMeta.ErrorCount = len(resp.Errors)
	}

	entry := &requestlog.Entry{
		Timestamp:      startTime,
		Protocol:       requestlog.ProtocolGraphQL,
		Method:         r.Method,
		Path:           h.config.Path,
		QueryString:    r.URL.RawQuery,
		Headers:        r.Header,
		Body:           logBody,
		BodySize:       len(rawBody),
		RemoteAddr:     r.RemoteAddr,
		ResponseStatus: statusCode,
		ResponseBody:   logRespBody,
		DurationMs:     durationMs,
		GraphQL:        gqlMeta,
	}

	h.requestLogger.Log(entry)
}

// detectOperationType detects the GraphQL operation type from a query string.
func (h *Handler) detectOperationType(query string) string {
	query = strings.TrimSpace(query)

	// Handle shorthand query syntax (no "query" keyword)
	if strings.HasPrefix(query, "{") {
		return "query"
	}

	// Look for operation keywords
	queryLower := strings.ToLower(query)
	if strings.HasPrefix(queryLower, "mutation") {
		return "mutation"
	}
	if strings.HasPrefix(queryLower, "subscription") {
		return "subscription"
	}
	if strings.HasPrefix(queryLower, "query") {
		return "query"
	}

	return "query" // Default to query
}

// parseError represents a request parsing error.
type parseError struct {
	message string
}

func (e *parseError) Error() string {
	return e.message
}

// Endpoint creates a complete GraphQL endpoint from a configuration.
// This is a convenience function that creates a schema, executor, and handler.
func Endpoint(config *GraphQLConfig) (*Handler, error) {
	var schema *Schema
	var err error

	// Parse schema from inline or file
	if config.Schema != "" {
		schema, err = ParseSchema(config.Schema)
	} else if config.SchemaFile != "" {
		schema, err = ParseSchemaFile(config.SchemaFile)
	} else {
		return nil, &parseError{message: "either schema or schemaFile must be provided"}
	}

	if err != nil {
		return nil, err
	}

	// Create executor and handler
	executor := NewExecutor(schema, config)
	handler := NewHandler(executor, config)

	return handler, nil
}
