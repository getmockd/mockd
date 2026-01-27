// Package recording provides conversion from recordings to mock configurations.
package recording

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/getmockd/mockd/internal/id"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// ErrRecordingNotFound indicates the recording was not found or is invalid.
var ErrRecordingNotFound = errors.New("recording not found or invalid for conversion")

// ConvertOptions configures how recordings are converted to mocks.
type ConvertOptions struct {
	IncludeHeaders bool // Include request headers in matcher
	Deduplicate    bool // Remove duplicate request patterns
	SmartMatch     bool // Convert dynamic path segments like /users/123 to /users/{id}
}

// DefaultConvertOptions returns the default conversion options.
func DefaultConvertOptions() ConvertOptions {
	return ConvertOptions{
		IncludeHeaders: false,
		Deduplicate:    false,
		SmartMatch:     false,
	}
}

// FilterOptions configures how recordings are filtered before conversion.
type FilterOptions struct {
	PathPattern string   // Glob pattern for path filtering (e.g., "/api/*")
	Methods     []string // Allowed HTTP methods (e.g., ["GET", "POST"])
	StatusCodes []int    // Specific status codes (e.g., [200, 201])
	StatusRange string   // Status code range: "2xx", "3xx", "4xx", "5xx"
}

// SessionConvertOptions configures how session recordings are converted.
type SessionConvertOptions struct {
	ConvertOptions
	Filter      FilterOptions
	Duplicates  string // Strategy for duplicates: "first", "last", "all"
	AddToServer bool   // Whether to add mocks directly to the server
}

// DefaultSessionConvertOptions returns the default session conversion options.
func DefaultSessionConvertOptions() SessionConvertOptions {
	return SessionConvertOptions{
		ConvertOptions: DefaultConvertOptions(),
		Duplicates:     "first",
		AddToServer:    false,
	}
}

// SensitiveDataWarning represents a warning about potentially sensitive data.
type SensitiveDataWarning struct {
	Type     string `json:"type"`     // "header", "cookie", "query", "body"
	Field    string `json:"field"`    // The specific field name
	Location string `json:"location"` // "request" or "response"
	Message  string `json:"message"`  // Human-readable description
}

// ConversionResult represents the result of converting recordings with warnings.
type ConversionResult struct {
	Mocks    []*config.MockConfiguration `json:"mocks"`
	Warnings []SensitiveDataWarning      `json:"warnings,omitempty"`
	Filtered int                         `json:"filtered"` // Number of recordings filtered out
	Total    int                         `json:"total"`    // Total recordings processed
}

// StreamConvertOptions configures how stream recordings are converted to configs.
type StreamConvertOptions struct {
	// SimplifyTiming normalizes timing to reduce noise
	SimplifyTiming bool `json:"simplifyTiming,omitempty"`

	// MinDelay is the minimum delay to preserve (ms) when simplifying
	MinDelay int `json:"minDelay,omitempty"`

	// MaxDelay caps delays at this value (ms) when simplifying
	MaxDelay int `json:"maxDelay,omitempty"`

	// IncludeClientMessages includes client-to-server messages as "expect" steps
	IncludeClientMessages bool `json:"includeClientMessages,omitempty"`

	// DeduplicateMessages removes consecutive duplicate messages
	DeduplicateMessages bool `json:"deduplicateMessages,omitempty"`

	// Format is the output format: "json" or "yaml"
	Format string `json:"format,omitempty"`
}

// DefaultStreamConvertOptions returns sensible defaults for stream conversion.
func DefaultStreamConvertOptions() StreamConvertOptions {
	return StreamConvertOptions{
		SimplifyTiming:        false,
		MinDelay:              10,
		MaxDelay:              5000,
		IncludeClientMessages: true,
		DeduplicateMessages:   false,
		Format:                "json",
	}
}

// ConversionMetadata contains info about the source recording.
// This is returned alongside the mock config in StreamConvertResult.
type ConversionMetadata struct {
	SourceRecordingID string    `json:"sourceRecordingId"`
	Protocol          Protocol  `json:"protocol"`
	OriginalPath      string    `json:"originalPath,omitempty"`
	RecordedAt        time.Time `json:"recordedAt"`
	ItemCount         int       `json:"itemCount"`                // frames for WS, events for SSE
	DurationMs        int64     `json:"durationMs"`               // total duration in milliseconds
	DetectedFormat    string    `json:"detectedFormat,omitempty"` // e.g., "openai" for SSE
}

// skipResponseHeaders are headers that should not be copied from recordings
// as they are dynamically generated or managed by the server.
var skipResponseHeaders = map[string]bool{
	"Date":              true,
	"Content-Length":    true,
	"Transfer-Encoding": true,
	"Connection":        true,
	"Keep-Alive":        true,
	"Server":            true,
	"X-Powered-By":      true,
	"Age":               true,
	"Expires":           true,
	"Last-Modified":     true,
	"ETag":              true,
}

// ToMock converts a recording to a mock configuration.
func ToMock(r *Recording, opts ConvertOptions) *config.MockConfiguration {
	matcher := &mock.HTTPMatcher{
		Method: r.Request.Method,
		Path:   r.Request.Path,
	}

	if opts.IncludeHeaders && len(r.Request.Headers) > 0 {
		headers := make(map[string]string)
		for key, values := range r.Request.Headers {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
		matcher.Headers = headers
	}

	response := &mock.HTTPResponse{
		StatusCode: r.Response.StatusCode,
		Body:       string(r.Response.Body),
	}

	if len(r.Response.Headers) > 0 {
		headers := make(map[string]string)
		for key, values := range r.Response.Headers {
			// Skip headers that shouldn't be static in mocks
			if skipResponseHeaders[key] {
				continue
			}
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
		if len(headers) > 0 {
			response.Headers = headers
		}
	}

	now := time.Now()
	enabled := true
	return &config.MockConfiguration{
		ID:        id.Short(),
		Type:      mock.MockTypeHTTP,
		Enabled:   &enabled,
		CreatedAt: now,
		UpdatedAt: now,
		HTTP: &mock.HTTPSpec{
			Matcher:  matcher,
			Response: response,
		},
	}
}

// ToMocks converts multiple recordings to mock configurations.
func ToMocks(recordings []*Recording, opts ConvertOptions) []*config.MockConfiguration {
	mocks := make([]*config.MockConfiguration, 0, len(recordings))
	seen := make(map[string]bool)

	for _, r := range recordings {
		// Generate a key for deduplication
		key := r.Request.Method + ":" + r.Request.Path
		if opts.Deduplicate && seen[key] {
			continue
		}
		seen[key] = true

		mocks = append(mocks, ToMock(r, opts))
	}

	return mocks
}

// ConvertSession converts all recordings in a session to mocks.
func ConvertSession(session *Session, opts ConvertOptions) []*config.MockConfiguration {
	return ToMocks(session.Recordings(), opts)
}

// ConvertSessionWithOptions converts session recordings with advanced filtering and options.
func ConvertSessionWithOptions(session *Session, opts SessionConvertOptions) *ConversionResult {
	recordings := session.Recordings()
	return ConvertRecordingsWithOptions(recordings, opts)
}

// ConvertRecordingsWithOptions converts recordings with filtering, smart matching, and warnings.
func ConvertRecordingsWithOptions(recordings []*Recording, opts SessionConvertOptions) *ConversionResult {
	result := &ConversionResult{
		Total:    len(recordings),
		Mocks:    make([]*config.MockConfiguration, 0),
		Warnings: make([]SensitiveDataWarning, 0),
	}

	// Apply filters
	filtered := FilterRecordings(recordings, opts.Filter)
	result.Filtered = len(recordings) - len(filtered)

	// Collect all warnings
	for _, r := range filtered {
		warnings := CheckSensitiveData(r)
		result.Warnings = append(result.Warnings, warnings...)
	}

	// Convert with deduplication strategy
	mocks := ToMocksWithStrategy(filtered, opts.ConvertOptions, opts.Duplicates)

	// Apply smart matching if enabled
	if opts.SmartMatch {
		for _, m := range mocks {
			if m.HTTP != nil && m.HTTP.Matcher != nil {
				m.HTTP.Matcher.Path = SmartPathMatcher(m.HTTP.Matcher.Path)
			}
		}
		mocks = DeduplicatePaths(mocks, opts.Duplicates)
	}

	result.Mocks = mocks
	return result
}

// FilterRecordings filters recordings based on the provided options.
func FilterRecordings(recordings []*Recording, opts FilterOptions) []*Recording {
	if len(recordings) == 0 {
		return recordings
	}

	// Check if any filters are set
	hasFilters := opts.PathPattern != "" || len(opts.Methods) > 0 ||
		len(opts.StatusCodes) > 0 || opts.StatusRange != ""

	if !hasFilters {
		return recordings
	}

	result := make([]*Recording, 0, len(recordings))

	for _, r := range recordings {
		if matchesFilter(r, opts) {
			result = append(result, r)
		}
	}

	return result
}

// matchesFilter checks if a recording matches the filter options.
func matchesFilter(r *Recording, opts FilterOptions) bool {
	// Check path pattern (glob)
	if opts.PathPattern != "" {
		matched := matchPathPattern(opts.PathPattern, r.Request.Path)
		if !matched {
			return false
		}
	}

	// Check method filter
	if len(opts.Methods) > 0 {
		methodMatch := false
		for _, m := range opts.Methods {
			if strings.EqualFold(r.Request.Method, m) {
				methodMatch = true
				break
			}
		}
		if !methodMatch {
			return false
		}
	}

	// Check specific status codes
	if len(opts.StatusCodes) > 0 {
		statusMatch := false
		for _, code := range opts.StatusCodes {
			if r.Response.StatusCode == code {
				statusMatch = true
				break
			}
		}
		if !statusMatch {
			return false
		}
	}

	// Check status code range (2xx, 3xx, 4xx, 5xx)
	if opts.StatusRange != "" {
		if !matchesStatusRange(r.Response.StatusCode, opts.StatusRange) {
			return false
		}
	}

	return true
}

// matchPathPattern matches a path against a glob-like pattern.
// Supports * as wildcard for any characters.
func matchPathPattern(pattern, path string) bool {
	// Handle empty pattern
	if pattern == "" {
		return true
	}

	// Handle simple prefix matching with trailing *
	if strings.HasSuffix(pattern, "*") && !strings.Contains(pattern[:len(pattern)-1], "*") {
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(path, prefix)
	}

	// Handle simple suffix matching with leading *
	if strings.HasPrefix(pattern, "*") && !strings.Contains(pattern[1:], "*") {
		suffix := pattern[1:]
		return strings.HasSuffix(path, suffix)
	}

	// Handle contains matching with * on both ends
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		middle := pattern[1 : len(pattern)-1]
		if !strings.Contains(middle, "*") {
			return strings.Contains(path, middle)
		}
	}

	// Convert glob pattern to regex for complex patterns
	regexPattern := "^"
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			regexPattern += ".*"
		case '?':
			regexPattern += "."
		case '.', '+', '^', '$', '[', ']', '(', ')', '{', '}', '|', '\\':
			regexPattern += "\\" + string(pattern[i])
		default:
			regexPattern += string(pattern[i])
		}
	}
	regexPattern += "$"

	re, err := regexp.Compile(regexPattern)
	if err != nil {
		return false
	}

	return re.MatchString(path)
}

// matchesStatusRange checks if a status code matches a range like "2xx", "4xx".
func matchesStatusRange(statusCode int, rangeStr string) bool {
	rangeStr = strings.ToLower(strings.TrimSpace(rangeStr))

	switch rangeStr {
	case "2xx":
		return statusCode >= 200 && statusCode < 300
	case "3xx":
		return statusCode >= 300 && statusCode < 400
	case "4xx":
		return statusCode >= 400 && statusCode < 500
	case "5xx":
		return statusCode >= 500 && statusCode < 600
	case "success", "ok":
		return statusCode >= 200 && statusCode < 300
	case "error":
		return statusCode >= 400
	case "client-error":
		return statusCode >= 400 && statusCode < 500
	case "server-error":
		return statusCode >= 500
	default:
		return true
	}
}

// ToMocksWithStrategy converts recordings to mocks with a deduplication strategy.
func ToMocksWithStrategy(recordings []*Recording, opts ConvertOptions, strategy string) []*config.MockConfiguration {
	if strategy == "all" || strategy == "" {
		opts.Deduplicate = false
		return ToMocks(recordings, opts)
	}

	// Group recordings by method + path
	groups := make(map[string][]*Recording)
	order := make([]string, 0)

	for _, r := range recordings {
		key := r.Request.Method + ":" + r.Request.Path
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], r)
	}

	// Select based on strategy
	selected := make([]*Recording, 0, len(order))
	for _, key := range order {
		group := groups[key]
		switch strategy {
		case "last":
			selected = append(selected, group[len(group)-1])
		default: // "first"
			selected = append(selected, group[0])
		}
	}

	opts.Deduplicate = false // Already deduplicated
	return ToMocks(selected, opts)
}

// SmartPathMatcher converts a concrete path to a parameterized pattern.
// For example: /users/123 -> /users/{id}, /orders/abc-def-ghi -> /orders/{id}
func SmartPathMatcher(path string) string {
	segments := strings.Split(path, "/")
	result := make([]string, len(segments))

	for i, segment := range segments {
		if segment == "" {
			result[i] = segment
			continue
		}

		// Check for UUID pattern (8-4-4-4-12)
		if isUUID(segment) {
			result[i] = "{id}"
			continue
		}

		// Check for numeric ID
		if isNumericID(segment) {
			result[i] = "{id}"
			continue
		}

		// Check for alphanumeric ID patterns (e.g., base64, hashes)
		if isAlphanumericID(segment) {
			result[i] = "{id}"
			continue
		}

		result[i] = segment
	}

	return strings.Join(result, "/")
}

// UUID regex pattern
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// isUUID checks if a string is a UUID.
func isUUID(s string) bool {
	return uuidPattern.MatchString(s)
}

// isNumericID checks if a string is a numeric ID.
func isNumericID(s string) bool {
	if len(s) == 0 {
		return false
	}
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

// Patterns for alphanumeric IDs (hashes, base64, etc.)
var alphanumericIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{16,}$|^[0-9a-zA-Z_-]{20,}$`)

// isAlphanumericID checks if a string looks like a hash or encoded ID.
func isAlphanumericID(s string) bool {
	// Must be at least 16 chars and look like a hash/encoded value
	if len(s) < 16 {
		return false
	}

	// Check for hex hash (md5, sha1, etc.)
	if alphanumericIDPattern.MatchString(s) {
		return true
	}

	return false
}

// DeduplicatePaths merges mocks with the same parameterized path pattern.
func DeduplicatePaths(mocks []*config.MockConfiguration, strategy string) []*config.MockConfiguration {
	if len(mocks) <= 1 {
		return mocks
	}

	// Group by method + parameterized path
	groups := make(map[string][]*config.MockConfiguration)
	order := make([]string, 0)

	for _, m := range mocks {
		method, path := "", ""
		if m.HTTP != nil && m.HTTP.Matcher != nil {
			method = m.HTTP.Matcher.Method
			path = m.HTTP.Matcher.Path
		}
		key := method + ":" + path
		if _, exists := groups[key]; !exists {
			order = append(order, key)
		}
		groups[key] = append(groups[key], m)
	}

	// Select based on strategy
	result := make([]*config.MockConfiguration, 0, len(order))
	for _, key := range order {
		group := groups[key]
		switch strategy {
		case "last":
			result = append(result, group[len(group)-1])
		case "all":
			result = append(result, group...)
		default: // "first"
			result = append(result, group[0])
		}
	}

	return result
}

// Sensitive header patterns to check
var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"x-api-key":           true,
	"x-auth-token":        true,
	"x-access-token":      true,
	"api-key":             true,
	"apikey":              true,
	"x-csrf-token":        true,
	"x-xsrf-token":        true,
	"proxy-authorization": true,
}

// Sensitive cookie name patterns
var sensitiveCookiePatterns = []string{
	"session",
	"token",
	"auth",
	"jwt",
	"sid",
	"csrf",
	"xsrf",
}

// Sensitive query parameter patterns
var sensitiveQueryParams = map[string]bool{
	"api_key":      true,
	"apikey":       true,
	"api-key":      true,
	"access_token": true,
	"token":        true,
	"auth":         true,
	"key":          true,
	"secret":       true,
	"password":     true,
	"passwd":       true,
	"pwd":          true,
}

// CheckSensitiveData checks a recording for potentially sensitive data.
func CheckSensitiveData(r *Recording) []SensitiveDataWarning {
	warnings := make([]SensitiveDataWarning, 0)

	// Check request headers
	for header := range r.Request.Headers {
		headerLower := strings.ToLower(header)
		if sensitiveHeaders[headerLower] {
			warnings = append(warnings, SensitiveDataWarning{
				Type:     "header",
				Field:    header,
				Location: "request",
				Message:  "Request contains potentially sensitive header: " + header,
			})
		}
	}

	// Check response headers
	for header := range r.Response.Headers {
		headerLower := strings.ToLower(header)
		if sensitiveHeaders[headerLower] {
			warnings = append(warnings, SensitiveDataWarning{
				Type:     "header",
				Field:    header,
				Location: "response",
				Message:  "Response contains potentially sensitive header: " + header,
			})
		}
	}

	// Check for cookies
	for header, values := range r.Request.Headers {
		if strings.ToLower(header) == "cookie" {
			for _, value := range values {
				for _, pattern := range sensitiveCookiePatterns {
					if strings.Contains(strings.ToLower(value), pattern) {
						warnings = append(warnings, SensitiveDataWarning{
							Type:     "cookie",
							Field:    pattern,
							Location: "request",
							Message:  "Request contains cookie with sensitive pattern: " + pattern,
						})
						break
					}
				}
			}
		}
	}

	// Check Set-Cookie in response
	for header, values := range r.Response.Headers {
		if strings.ToLower(header) == "set-cookie" {
			for _, value := range values {
				for _, pattern := range sensitiveCookiePatterns {
					if strings.Contains(strings.ToLower(value), pattern) {
						warnings = append(warnings, SensitiveDataWarning{
							Type:     "cookie",
							Field:    pattern,
							Location: "response",
							Message:  "Response sets cookie with sensitive pattern: " + pattern,
						})
						break
					}
				}
			}
		}
	}

	// Check query parameters in URL
	if strings.Contains(r.Request.URL, "?") {
		queryPart := r.Request.URL[strings.Index(r.Request.URL, "?")+1:]
		params := strings.Split(queryPart, "&")
		for _, param := range params {
			parts := strings.SplitN(param, "=", 2)
			if len(parts) > 0 {
				paramName := strings.ToLower(parts[0])
				if sensitiveQueryParams[paramName] {
					warnings = append(warnings, SensitiveDataWarning{
						Type:     "query",
						Field:    parts[0],
						Location: "request",
						Message:  "Request URL contains sensitive query parameter: " + parts[0],
					})
				}
			}
		}
	}

	return warnings
}

// ParseMethodFilter parses a comma-separated list of HTTP methods.
func ParseMethodFilter(methodStr string) []string {
	if methodStr == "" {
		return nil
	}

	methods := strings.Split(methodStr, ",")
	result := make([]string, 0, len(methods))

	for _, m := range methods {
		m = strings.TrimSpace(strings.ToUpper(m))
		if m != "" {
			result = append(result, m)
		}
	}

	return result
}

// ParseStatusFilter parses a status code filter string.
// Accepts: "200", "200,201,404", "2xx", "4xx,5xx"
func ParseStatusFilter(statusStr string) (codes []int, rangeStr string) {
	if statusStr == "" {
		return nil, ""
	}

	statusStr = strings.TrimSpace(strings.ToLower(statusStr))

	// Check for range patterns
	if strings.Contains(statusStr, "xx") {
		return nil, statusStr
	}

	// Parse individual codes
	parts := strings.Split(statusStr, ",")
	codes = make([]int, 0, len(parts))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		if code, err := strconv.Atoi(p); err == nil && code >= 100 && code < 600 {
			codes = append(codes, code)
		}
	}

	return codes, ""
}

// ToWebSocketScenario converts a WebSocket stream recording to a mock scenario config.
// Returns the scenario config and metadata about the conversion.
func ToWebSocketScenario(recording *StreamRecording, opts StreamConvertOptions) (*mock.WSScenarioConfig, *ConversionMetadata, error) {
	if recording == nil || recording.Protocol != ProtocolWebSocket || recording.WebSocket == nil {
		return nil, nil, ErrRecordingNotFound
	}

	ws := recording.WebSocket
	scenario := &mock.WSScenarioConfig{
		Name:  "Recorded: " + recording.ID,
		Steps: make([]mock.WSScenarioStepConfig, 0, len(ws.Frames)),
	}

	// Build metadata
	metadata := &ConversionMetadata{
		SourceRecordingID: recording.ID,
		Protocol:          ProtocolWebSocket,
		OriginalPath:      recording.Metadata.Path,
		RecordedAt:        recording.StartTime,
		ItemCount:         len(ws.Frames),
	}
	if len(ws.Frames) > 0 {
		metadata.DurationMs = ws.Frames[len(ws.Frames)-1].RelativeMs
	}

	var lastMsg string
	var lastRelativeMs int64

	for i, frame := range ws.Frames {
		// Skip ping/pong/close frames for scenario conversion
		if frame.MessageType == MessageTypePing || frame.MessageType == MessageTypePong || frame.MessageType == MessageTypeClose {
			continue
		}

		// Get message data
		data, err := frame.GetData()
		if err != nil {
			continue
		}
		msgStr := string(data)

		// Deduplicate if enabled
		if opts.DeduplicateMessages && msgStr == lastMsg {
			continue
		}
		lastMsg = msgStr

		// Calculate delay from previous frame
		var delayMs int64
		if i > 0 && frame.RelativeMs > lastRelativeMs {
			delayMs = frame.RelativeMs - lastRelativeMs
		}
		lastRelativeMs = frame.RelativeMs

		// Apply timing simplification
		if opts.SimplifyTiming {
			if delayMs < int64(opts.MinDelay) {
				delayMs = 0
			} else if delayMs > int64(opts.MaxDelay) {
				delayMs = int64(opts.MaxDelay)
			}
		}

		if frame.Direction == DirectionServerToClient {
			// Server message -> "send" step
			step := mock.WSScenarioStepConfig{
				Type: "send",
				Message: &mock.WSMessageResponse{
					Type:  frameMessageTypeToString(frame.MessageType),
					Value: frameValueForConfig(frame),
				},
			}

			// Add delay if significant
			if delayMs > 0 {
				step.Message.Delay = formatDuration(delayMs)
			}

			scenario.Steps = append(scenario.Steps, step)

		} else if opts.IncludeClientMessages {
			// Client message -> "expect" step
			step := mock.WSScenarioStepConfig{
				Type:     "expect",
				Timeout:  "30s",
				Optional: false,
				Match: &mock.WSMatchCriteria{
					Type:  frameMessageTypeToString(frame.MessageType),
					Value: msgStr,
				},
			}
			scenario.Steps = append(scenario.Steps, step)
		}
	}

	return scenario, metadata, nil
}

// ToSSEConfig converts an SSE stream recording to an SSE mock config.
// Returns the SSE config and metadata about the conversion.
func ToSSEConfig(recording *StreamRecording, opts StreamConvertOptions) (*mock.SSEConfig, *ConversionMetadata, error) {
	if recording == nil || recording.Protocol != ProtocolSSE || recording.SSE == nil {
		return nil, nil, ErrRecordingNotFound
	}

	sse := recording.SSE
	cfg := &mock.SSEConfig{
		Events: make([]mock.SSEEventDef, 0, len(sse.Events)),
		Timing: mock.SSETimingConfig{
			PerEventDelays: make([]int, 0, len(sse.Events)),
		},
		Lifecycle: mock.SSELifecycleConfig{
			MaxEvents: len(sse.Events),
		},
		Resume: mock.SSEResumeConfig{
			Enabled:    true,
			BufferSize: len(sse.Events),
		},
	}

	// Build metadata
	metadata := &ConversionMetadata{
		SourceRecordingID: recording.ID,
		Protocol:          ProtocolSSE,
		OriginalPath:      recording.Metadata.Path,
		RecordedAt:        recording.StartTime,
		ItemCount:         len(sse.Events),
		DetectedFormat:    recording.Metadata.DetectedTemplate,
	}
	if len(sse.Events) > 0 {
		metadata.DurationMs = sse.Events[len(sse.Events)-1].RelativeMs
	}

	var lastData string
	var lastRelativeMs int64
	usePerEventDelays := !opts.SimplifyTiming
	var totalDelay int64
	var delayCount int

	for i, event := range sse.Events {
		// Deduplicate if enabled
		if opts.DeduplicateMessages && event.Data == lastData {
			continue
		}
		lastData = event.Data

		// Calculate delay from previous event
		var delayMs int64
		if i > 0 && event.RelativeMs > lastRelativeMs {
			delayMs = event.RelativeMs - lastRelativeMs
		}
		lastRelativeMs = event.RelativeMs

		// Apply timing simplification
		if opts.SimplifyTiming {
			if delayMs < int64(opts.MinDelay) {
				delayMs = 0
			} else if delayMs > int64(opts.MaxDelay) {
				delayMs = int64(opts.MaxDelay)
			}
		}

		// Track for average calculation
		totalDelay += delayMs
		delayCount++

		eventCfg := mock.SSEEventDef{
			Type:    event.EventType,
			Data:    parseEventData(event.Data),
			ID:      event.ID,
			Comment: event.Comment,
		}

		// Handle retry pointer
		if event.Retry != nil {
			eventCfg.Retry = *event.Retry
		}

		cfg.Events = append(cfg.Events, eventCfg)

		if usePerEventDelays {
			cfg.Timing.PerEventDelays = append(cfg.Timing.PerEventDelays, int(delayMs))
		}
	}

	// If simplifying, use average fixed delay instead of per-event
	if opts.SimplifyTiming && delayCount > 0 {
		avgDelay := int(totalDelay / int64(delayCount))
		cfg.Timing.FixedDelay = &avgDelay
		cfg.Timing.PerEventDelays = nil
	}

	return cfg, metadata, nil
}

// StreamConvertResult holds the result of converting a stream recording.
type StreamConvertResult struct {
	Protocol   Protocol            `json:"protocol"`
	Config     interface{}         `json:"config"`     // *mock.WSScenarioConfig or *mock.SSEConfig
	Metadata   *ConversionMetadata `json:"metadata"`   // Info about the source recording
	ConfigJSON []byte              `json:"configJson"` // JSON-encoded config
	ConfigYAML []byte              `json:"configYaml"` // YAML-encoded config (if requested)
}

// ConvertStreamRecording converts a stream recording to the appropriate mock config.
func ConvertStreamRecording(recording *StreamRecording, opts StreamConvertOptions) (*StreamConvertResult, error) {
	result := &StreamConvertResult{
		Protocol: recording.Protocol,
	}

	var err error
	switch recording.Protocol {
	case ProtocolWebSocket:
		result.Config, result.Metadata, err = ToWebSocketScenario(recording, opts)
	case ProtocolSSE:
		result.Config, result.Metadata, err = ToSSEConfig(recording, opts)
	default:
		return nil, ErrRecordingNotFound
	}

	if err != nil {
		return nil, err
	}

	// Encode to JSON
	result.ConfigJSON, err = json.MarshalIndent(result.Config, "", "  ")
	if err != nil {
		return nil, err
	}

	// TODO: Add YAML encoding if opts.Format == "yaml"

	return result, nil
}

// Helper functions

// frameMessageTypeToString converts MessageType to string for config.
func frameMessageTypeToString(mt MessageType) string {
	switch mt {
	case MessageTypeText:
		return "text"
	case MessageTypeBinary:
		return "binary"
	default:
		return "text"
	}
}

// frameValueForConfig returns the appropriate value for the config.
func frameValueForConfig(frame WebSocketFrame) interface{} {
	data, err := frame.GetData()
	if err != nil {
		return ""
	}

	if frame.MessageType == MessageTypeBinary {
		// Return base64 for binary
		return base64.StdEncoding.EncodeToString(data)
	}

	// Try to parse as JSON for better formatting
	var jsonVal interface{}
	if err := json.Unmarshal(data, &jsonVal); err == nil {
		return jsonVal
	}

	return string(data)
}

// formatDuration formats milliseconds as a duration string.
func formatDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	return d.String()
}

// parseEventData tries to parse SSE data as JSON, otherwise returns as string.
func parseEventData(data string) interface{} {
	var jsonVal interface{}
	if err := json.Unmarshal([]byte(data), &jsonVal); err == nil {
		return jsonVal
	}
	return data
}
