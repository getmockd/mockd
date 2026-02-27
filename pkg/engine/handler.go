// Core HTTP request handler for the mock engine.

package engine

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"maps"
	mathrand "math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/getmockd/mockd/internal/matching"
	"github.com/getmockd/mockd/internal/storage"
	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/mtls"
	"github.com/getmockd/mockd/pkg/oauth"
	"github.com/getmockd/mockd/pkg/requestlog"
	"github.com/getmockd/mockd/pkg/soap"
	"github.com/getmockd/mockd/pkg/sse"
	"github.com/getmockd/mockd/pkg/stateful"
	"github.com/getmockd/mockd/pkg/template"
	"github.com/getmockd/mockd/pkg/util"
	"github.com/getmockd/mockd/pkg/validation"
	"github.com/getmockd/mockd/pkg/websocket"
)

// MaxStatefulBodySize is the maximum allowed request body size for stateful POST/PUT operations (1MB).
const MaxStatefulBodySize = 1 << 20 // 1MB

// MaxRequestBodySize is the maximum allowed request body size for mock matching (10MB).
// This prevents denial-of-service via oversized request bodies.
const MaxRequestBodySize = 10 << 20 // 10MB

// Handler handles incoming HTTP requests and matches them against configured mocks.
type Handler struct {
	store          storage.MockStore
	statefulStore  *stateful.StateStore
	statefulBridge *stateful.Bridge // Bridge for custom operation execution
	logger         RequestLogger
	log            *slog.Logger // Operational logger for errors/warnings
	sseHandler     *sse.SSEHandler
	chunkedHandler *sse.ChunkedHandler
	wsManager      *websocket.ConnectionManager
	templateEngine *template.Engine

	// baseDir is the base directory for resolving relative file paths (e.g., bodyFile).
	// When set, relative paths in bodyFile are resolved against this directory.
	// When empty, relative paths are resolved against the process working directory.
	baseDir string

	// Enterprise feature routing
	graphqlMu       sync.RWMutex
	graphqlHandlers map[string]*graphql.Handler
	graphqlSubMu    sync.RWMutex
	graphqlSubs     map[string]*graphql.SubscriptionHandler
	oauthMu         sync.RWMutex
	oauthHandlers   map[string]*oauth.Handler
	soapMu          sync.RWMutex
	soapHandlers    map[string]*soap.Handler
}

// NewHandler creates a new Handler.
func NewHandler(store storage.MockStore) *Handler {
	tmplEngine := template.New()
	h := &Handler{
		store:           store,
		log:             logging.Nop(),          // Default to no-op logger
		sseHandler:      sse.NewSSEHandler(100), // 100 max SSE connections
		chunkedHandler:  sse.NewChunkedHandler(),
		wsManager:       websocket.NewConnectionManager(),
		templateEngine:  tmplEngine,
		graphqlHandlers: make(map[string]*graphql.Handler),
		graphqlSubs:     make(map[string]*graphql.SubscriptionHandler),
		oauthHandlers:   make(map[string]*oauth.Handler),
		soapHandlers:    make(map[string]*soap.Handler),
	}
	// Wire template engine so SSE responses can use template variables
	h.sseHandler.SetTemplateEngine(tmplEngine)
	return h
}

// SetLogger sets the request logger for the handler.
func (h *Handler) SetLogger(logger RequestLogger) {
	h.logger = logger
}

// SetOperationalLogger sets the operational logger for error/warning messages.
func (h *Handler) SetOperationalLogger(log *slog.Logger) {
	if log != nil {
		h.log = log
	} else {
		h.log = logging.Nop()
	}
}

// SetStatefulStore sets the stateful resource store for the handler.
func (h *Handler) SetStatefulStore(store *stateful.StateStore) {
	h.statefulStore = store
}

// SetStatefulBridge sets the stateful bridge for custom operation execution.
func (h *Handler) SetStatefulBridge(bridge *stateful.Bridge) {
	h.statefulBridge = bridge
}

// SetStore sets the mock store for the handler.
func (h *Handler) SetStore(store storage.MockStore) {
	h.store = store
}

// SetBaseDir sets the base directory for resolving relative file paths (e.g., bodyFile).
func (h *Handler) SetBaseDir(dir string) {
	h.baseDir = dir
}

// HasMatch checks if any mock matches the given request without recording metrics.
func (h *Handler) HasMatch(r *http.Request) bool {
	mocks := h.store.ListByType(mock.TypeHTTP)
	return SelectBestMatchWithCaptures(mocks, r) != nil
}

// ServeHTTP implements the http.Handler interface.
// Note: CORS is handled by the CORSMiddleware wrapper, not directly in this handler.
// This ensures CORS configuration is respected rather than using hardcoded wildcards.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) { //nolint:gocyclo // request routing logic
	startTime := time.Now()

	// Check for health/ready endpoints with /__mockd/ prefix first (always takes priority)
	switch r.URL.Path {
	case "/__mockd/health":
		h.handleHealth(w, r)
		return
	case "/__mockd/ready":
		h.handleReady(w, r)
		return
	}

	// Check for WebSocket upgrade first
	if websocket.IsWebSocketRequest(r) {
		// Check for GraphQL subscription WebSocket first
		if subHandler := h.getGraphQLSubscriptionHandler(r.URL.Path); subHandler != nil {
			subHandler.ServeHTTP(w, r)
			return
		}
		h.handleWebSocket(w, r)
		return
	}

	// Check for GraphQL handler
	if gqlHandler := h.getGraphQLHandler(r.URL.Path); gqlHandler != nil {
		gqlHandler.ServeHTTP(w, r)
		return
	}

	// Check for OAuth handler
	if oauthHandler := h.getOAuthHandler(r.URL.Path); oauthHandler != nil {
		h.routeOAuthRequest(w, r, oauthHandler)
		return
	}

	// Check for SOAP handler
	if soapHandler := h.getSOAPHandler(r.URL.Path); soapHandler != nil {
		soapHandler.ServeHTTP(w, r)
		return
	}

	// Extract mTLS identity if available and inject into context
	if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
		identity := mtls.ExtractIdentity(r.TLS.PeerCertificates[0], len(r.TLS.VerifiedChains) > 0)
		r = r.WithContext(mtls.WithIdentity(r.Context(), identity))
	}

	// Enforce maximum body size to prevent denial-of-service via oversized payloads.
	// MaxBytesReader returns an error when the limit is exceeded, unlike LimitReader
	// which silently truncates.
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)

	// Capture request body for logging (bounded to prevent memory exhaustion)
	var bodyBytes []byte
	if r.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(r.Body)
		if err != nil {
			// Check if the error is due to body size limit exceeded
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				h.log.Warn("request body too large", "path", r.URL.Path, "limit", MaxRequestBodySize)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				errResp := map[string]string{
					"error":   "body_too_large",
					"message": "Request body exceeds maximum allowed size",
				}
				if jsonBytes, jsonErr := json.Marshal(errResp); jsonErr == nil {
					_, _ = w.Write(jsonBytes)
				}
				h.logRequest(startTime, r, nil, bodyBytes, "", http.StatusRequestEntityTooLarge, nil)
				return
			}
			h.log.Warn("failed to read request body", "path", r.URL.Path, "error", err)
		}
		r.Body = io.NopCloser(NewBodyReader(bodyBytes))
	}

	// Capture headers for logging
	headers := make(map[string][]string)
	maps.Copy(headers, r.Header)

	var statusCode int
	var matchedID string
	var nearMissInfos []requestlog.NearMissInfo

	// Check stateful resources first
	if h.statefulStore != nil {
		if resource, itemID, pathParams := h.statefulStore.MatchPath(r.URL.Path); resource != nil {
			statusCode = h.handleStateful(w, r, resource, itemID, pathParams, bodyBytes)
			matchedID = "stateful:" + resource.Name()
			h.logRequest(startTime, r, headers, bodyBytes, matchedID, statusCode, nil)
			return
		}
	}

	// Get all mocks (already sorted by priority) - only HTTP type mocks
	mocks := h.store.ListByType(mock.TypeHTTP)

	// Find best matching mock using scoring algorithm (with regex captures).
	// Pass the already-read bodyBytes to avoid a second 10 MB body read inside
	// the matcher — this halves peak memory per request for large bodies.
	matchResult := SelectBestMatchWithCaptures(mocks, r, bodyBytes)

	// HEAD fallback: if no match for HEAD, retry as GET
	if matchResult == nil && r.Method == http.MethodHead {
		getFallback := r.Clone(r.Context())
		getFallback.Method = http.MethodGet
		matchResult = SelectBestMatchWithCaptures(mocks, getFallback, bodyBytes)
	}

	if matchResult != nil {
		match := matchResult.Mock
		matchedID = match.ID

		h.log.Debug("request matched",
			"method", r.Method,
			"path", r.URL.Path,
			"mock_id", matchedID,
			"score", matchResult.Score,
		)

		// Record mock hit for metrics
		RecordMatchHit(matchedID)

		// Extract path parameters from the matched pattern
		matchPath := ""
		if match.HTTP != nil && match.HTTP.Matcher != nil {
			matchPath = match.HTTP.Matcher.Path
		}
		pathParams := matching.MatchPathVariable(matchPath, r.URL.Path)

		// Get path pattern captures from regex matching
		pathPatternCaptures := matchResult.PathPatternCaptures

		// Check for SSE streaming response
		if match.HTTP != nil && match.HTTP.SSE != nil {
			h.sseHandler.ServeHTTP(w, r, match)
			statusCode = http.StatusOK
			h.logRequest(startTime, r, headers, bodyBytes, matchedID, statusCode, nil)
			return
		}

		// Check for chunked streaming response
		if match.HTTP != nil && match.HTTP.Chunked != nil {
			h.chunkedHandler.ServeHTTP(w, r, match)
			statusCode = http.StatusOK
			h.logRequest(startTime, r, headers, bodyBytes, matchedID, statusCode, nil)
			return
		}

		// Run per-mock validation if configured
		if match.HTTP != nil && match.HTTP.Validation != nil && !match.HTTP.Validation.IsEmpty() {
			validationResult := h.validateHTTPRequest(r, bodyBytes, pathParams, match.HTTP.Validation)
			if validationResult != nil && !validationResult.Valid {
				statusCode = h.writeHTTPValidationError(w, validationResult, match.HTTP.Validation)
				if statusCode != 0 {
					h.logRequest(startTime, r, headers, bodyBytes, matchedID, statusCode, nil)
					return
				}
				// statusCode 0 means permissive/warn mode — continue to response
			}
		}

		// Check for stateful custom operation
		if match.HTTP != nil && match.HTTP.StatefulOperation != "" {
			statusCode = h.handleCustomOperation(w, r, match.HTTP.StatefulOperation, bodyBytes)
			h.logRequest(startTime, r, headers, bodyBytes, matchedID, statusCode, nil)
			return
		}

		// Standard response
		if match.HTTP != nil && match.HTTP.Response != nil {
			statusCode = h.writeResponse(w, r, bodyBytes, pathParams, pathPatternCaptures, match.HTTP.Response)
		}
	} else {
		// No match found - check for fallback health endpoints
		switch r.URL.Path {
		case "/health":
			h.handleHealth(w, r)
			h.logRequest(startTime, r, headers, bodyBytes, "__mockd:health", http.StatusOK, nil)
			return
		case "/ready":
			h.handleReady(w, r)
			h.logRequest(startTime, r, headers, bodyBytes, "__mockd:ready", http.StatusOK, nil)
			return
		}
		// No match found — run near-miss analysis to help debugging
		nearMisses := matching.CollectNearMisses(mocks, r, bodyBytes, 3)

		statusCode = http.StatusNotFound
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Mockd-Near-Misses", strconv.Itoa(len(nearMisses)))
		w.WriteHeader(statusCode)

		// Build response with near-miss data
		errResp := map[string]interface{}{
			"error":   "no_match",
			"message": "No mock matched the request",
			"path":    r.URL.Path,
			"method":  r.Method,
		}
		if len(nearMisses) > 0 {
			errResp["nearMisses"] = nearMisses
		}
		if jsonBytes, err := json.Marshal(errResp); err == nil {
			_, _ = w.Write(jsonBytes)
		} else {
			_, _ = w.Write([]byte(`{"error": "no_match", "message": "No mock matched the request"}`))
		}

		// Convert near-misses to log-friendly format
		if len(nearMisses) > 0 {
			nearMissInfos = make([]requestlog.NearMissInfo, len(nearMisses))
			for i, nm := range nearMisses {
				nearMissInfos[i] = requestlog.NearMissInfo{
					MockID:          nm.MockID,
					MockName:        nm.MockName,
					MatchPercentage: nm.MatchPercentage,
					Reason:          nm.Reason,
				}
			}
		}
	}

	// Log the request (with near-miss data if any)
	h.logRequest(startTime, r, headers, bodyBytes, matchedID, statusCode, nearMissInfos)
}

// logRequest logs a request to the logger.
func (h *Handler) logRequest(startTime time.Time, r *http.Request, headers map[string][]string, bodyBytes []byte, matchedID string, statusCode int, nearMisses []requestlog.NearMissInfo) {
	if h.logger != nil {
		entry := &requestlog.Entry{
			Timestamp:      startTime,
			Protocol:       requestlog.ProtocolHTTP,
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
			NearMisses:     nearMisses,
		}
		h.logger.Log(entry)
	}
}

// writeResponse writes the mock response to the HTTP response writer.
// It processes template variables in both response headers and body using the request context.
func (h *Handler) writeResponse(w http.ResponseWriter, r *http.Request, bodyBytes []byte, pathParams map[string]string, pathPatternCaptures map[string]string, resp *mock.HTTPResponse) int {
	// Apply delay if specified
	if resp.DelayMs > 0 {
		time.Sleep(time.Duration(resp.DelayMs) * time.Millisecond)
	}

	// Build template context once, reuse for both headers and body.
	var tmplCtx *template.Context
	if h.templateEngine != nil {
		tmplCtx = template.NewContext(r, bodyBytes)
		tmplCtx.Request.PathParams = pathParams
		tmplCtx.SetPathPatternCaptures(pathPatternCaptures)
		if identity := mtls.FromContext(r.Context()); identity != nil {
			tmplCtx.SetMTLSFromIdentity(identity)
		}

		// Seed the template RNG for deterministic responses.
		// Priority: query param > header > config field.
		if seed, ok := resolveSeed(r, resp); ok {
			tmplCtx.Rand = mathrand.New(mathrand.NewPCG(uint64(seed), 0))
		}
	}

	// Set headers (with template expansion).
	// Track whether the user explicitly set Content-Type so auto-detection
	// knows whether to override the value.
	userSetContentType := false
	for name, value := range resp.Headers {
		if tmplCtx != nil {
			if processed, err := h.templateEngine.Process(value, tmplCtx); err == nil {
				value = processed
			}
		}
		w.Header().Set(name, value)
		if strings.EqualFold(name, "Content-Type") {
			userSetContentType = true
		}
	}

	// Determine body content - check inline body first, then file
	body := resp.Body
	if body == "" && resp.BodyFile != "" {
		// Prevent path traversal but allow absolute paths (bodyFile is config-sourced)
		cleanPath, safe := util.SafeFilePathAllowAbsolute(resp.BodyFile)
		if !safe {
			h.log.Error("unsafe path in bodyFile (traversal)", "file", resp.BodyFile)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			errResp := map[string]string{
				"error":   "body_file_error",
				"message": "bodyFile path contains path traversal",
			}
			if jsonBytes, jsonErr := json.Marshal(errResp); jsonErr == nil {
				_, _ = w.Write(jsonBytes)
			}
			return http.StatusBadGateway
		}
		// Resolve relative paths against the handler's base directory
		if !filepath.IsAbs(cleanPath) && h.baseDir != "" {
			cleanPath = filepath.Join(h.baseDir, cleanPath)
		}
		data, err := os.ReadFile(cleanPath)
		if err != nil {
			h.log.Error("failed to read body file", "file", cleanPath, "error", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			errResp := map[string]string{
				"error":   "body_file_error",
				"message": "failed to read bodyFile: " + err.Error(),
			}
			if jsonBytes, jsonErr := json.Marshal(errResp); jsonErr == nil {
				_, _ = w.Write(jsonBytes)
			}
			return http.StatusBadGateway
		}
		body = string(data)
	}

	// Process body templates before setting Content-Type (so detection works on final content)
	if body != "" && tmplCtx != nil {
		if processedBody, err := h.templateEngine.Process(body, tmplCtx); err == nil {
			body = processedBody
		}
		// On error, use the original body (graceful degradation)
	}

	// Set default Content-Type based on body content if not explicitly specified by the user.
	// Auto-detect when Content-Type is empty or was defaulted to text/plain by Go's HTTP
	// stack (not explicitly set by the user in mock headers).
	currentCT := w.Header().Get("Content-Type")
	if !userSetContentType && (currentCT == "" || currentCT == "text/plain") {
		switch {
		case looksLikeJSON(body):
			w.Header().Set("Content-Type", "application/json")
		case looksLikeXML(body):
			w.Header().Set("Content-Type", "application/xml")
		default:
			w.Header().Set("Content-Type", "text/plain")
		}
	}

	// Write status code and body
	w.WriteHeader(resp.StatusCode)
	if body != "" {
		_, _ = w.Write([]byte(body))
	}
	return resp.StatusCode
}

// looksLikeJSON returns true if the string appears to be JSON content.
func looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

// looksLikeXML returns true if the string appears to be XML content.
func looksLikeXML(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "<?xml") || strings.HasPrefix(s, "<")
}

// validateHTTPRequest validates an HTTP request against validation rules.
func (h *Handler) validateHTTPRequest(r *http.Request, bodyBytes []byte, pathParams map[string]string, config *validation.RequestValidation) *validation.Result {
	if config == nil || config.IsEmpty() {
		return nil
	}

	validator := validation.NewHTTPValidator(config)
	if validator == nil {
		return nil
	}

	// Parse body as JSON for validation
	var body map[string]interface{}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &body); err != nil {
			// If body is not valid JSON, return parsing error
			result := &validation.Result{Valid: false}
			result.AddError(validation.NewInvalidJSONError(err.Error()))
			return result
		}
	}

	// Extract query params
	queryParams := make(map[string]string)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			queryParams[key] = values[0]
		}
	}

	// Extract headers
	headers := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			headers[strings.ToLower(key)] = values[0]
		}
	}

	return validator.Validate(r.Context(), body, pathParams, queryParams, headers)
}

// writeHTTPValidationError writes a validation error response for HTTP mocks.
func (h *Handler) writeHTTPValidationError(w http.ResponseWriter, result *validation.Result, config *validation.RequestValidation) int {
	mode := config.GetMode()

	// In warn mode, log but don't fail
	if mode == validation.ModeWarn {
		for _, err := range result.Errors {
			h.log.Warn("http validation warning", "field", err.Field, "message", err.Message)
		}
		return 0 // Continue to response
	}

	// In permissive mode, only fail on required field errors
	if mode == validation.ModePermissive {
		hasRequired := false
		for _, err := range result.Errors {
			if err.Code == validation.ErrCodeRequired {
				hasRequired = true
				break
			}
		}
		if !hasRequired {
			for _, err := range result.Errors {
				h.log.Warn("http validation warning (permissive)", "field", err.Field, "message", err.Message)
			}
			return 0 // Continue to response
		}
	}

	// Strict mode (default) - return error response
	status := config.GetFailStatus()
	resp := validation.NewErrorResponse(result, status)
	resp.WriteResponse(w)
	return status
}

// resolveSeed extracts a seed value for deterministic template output.
// Priority order: ?_mockd_seed query param > X-Mockd-Seed header > resp.Seed config.
// Returns the seed and true if one was found, or 0 and false otherwise.
func resolveSeed(r *http.Request, resp *mock.HTTPResponse) (int64, bool) {
	// 1. Query parameter (highest priority)
	if seedStr := r.URL.Query().Get("_mockd_seed"); seedStr != "" {
		if seed, err := strconv.ParseInt(seedStr, 10, 64); err == nil {
			return seed, true
		}
	}

	// 2. HTTP header
	if seedStr := r.Header.Get("X-Mockd-Seed"); seedStr != "" {
		if seed, err := strconv.ParseInt(seedStr, 10, 64); err == nil {
			return seed, true
		}
	}

	// 3. Config-level seed
	if resp.Seed != nil {
		return *resp.Seed, true
	}

	return 0, false
}
