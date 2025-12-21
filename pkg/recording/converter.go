// Package recording provides conversion from recordings to mock configurations.
package recording

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/getmockd/mockd/pkg/config"
)

// ErrRecordingNotFound indicates the recording was not found or is invalid.
var ErrRecordingNotFound = errors.New("recording not found or invalid for conversion")

// ConvertOptions configures how recordings are converted to mocks.
type ConvertOptions struct {
	IncludeHeaders bool // Include request headers in matcher
	Deduplicate    bool // Remove duplicate request patterns
}

// DefaultConvertOptions returns the default conversion options.
func DefaultConvertOptions() ConvertOptions {
	return ConvertOptions{
		IncludeHeaders: false,
		Deduplicate:    false,
	}
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

// WebSocketScenarioConfig is the target format for WebSocket conversion.
// This matches the websocket.ScenarioConfig structure.
type WebSocketScenarioConfig struct {
	Name             string                     `json:"name"`
	Steps            []WebSocketScenarioStep    `json:"steps"`
	Loop             bool                       `json:"loop,omitempty"`
	ResetOnReconnect *bool                      `json:"resetOnReconnect,omitempty"`
	Metadata         *WebSocketScenarioMetadata `json:"metadata,omitempty"`
}

// WebSocketScenarioStep represents a step in the scenario.
type WebSocketScenarioStep struct {
	Type     string                  `json:"type"`               // "send", "expect", "wait"
	Message  *WebSocketMessageConfig `json:"message,omitempty"`  // for "send"
	Match    *WebSocketMatchConfig   `json:"match,omitempty"`    // for "expect"
	Duration string                  `json:"duration,omitempty"` // for "wait"
	Timeout  string                  `json:"timeout,omitempty"`
	Optional bool                    `json:"optional,omitempty"`
}

// WebSocketMessageConfig defines a message to send.
type WebSocketMessageConfig struct {
	Type  string      `json:"type"` // "text", "binary", "json"
	Value interface{} `json:"value"`
	Delay string      `json:"delay,omitempty"`
}

// WebSocketMatchConfig defines expected message criteria.
type WebSocketMatchConfig struct {
	Type    string `json:"type,omitempty"`
	Exact   string `json:"exact,omitempty"`
	Pattern string `json:"pattern,omitempty"`
}

// WebSocketScenarioMetadata contains info about the source recording.
type WebSocketScenarioMetadata struct {
	SourceRecordingID string    `json:"sourceRecordingId"`
	OriginalPath      string    `json:"originalPath,omitempty"`
	RecordedAt        time.Time `json:"recordedAt"`
	TotalFrames       int       `json:"totalFrames"`
	Duration          int64     `json:"durationMs"`
}

// SSEMockConfig is the target format for SSE conversion.
// This matches the sse.SSEConfig structure.
type SSEMockConfig struct {
	Events    []SSEEventConfig   `json:"events"`
	Timing    SSETimingConfig    `json:"timing"`
	Lifecycle SSELifecycleConfig `json:"lifecycle,omitempty"`
	Resume    SSEResumeConfig    `json:"resume,omitempty"`
	Metadata  *SSEMockMetadata   `json:"metadata,omitempty"`
}

// SSEEventConfig defines a single event in the stream.
type SSEEventConfig struct {
	Type    string      `json:"type,omitempty"`
	Data    interface{} `json:"data"`
	ID      string      `json:"id,omitempty"`
	Retry   int         `json:"retry,omitempty"`
	Comment string      `json:"comment,omitempty"`
	Delay   *int        `json:"delay,omitempty"` // per-event delay override
}

// SSETimingConfig controls event delivery timing.
type SSETimingConfig struct {
	FixedDelay     *int  `json:"fixedDelay,omitempty"`
	PerEventDelays []int `json:"perEventDelays,omitempty"`
	InitialDelay   int   `json:"initialDelay,omitempty"`
}

// SSELifecycleConfig controls connection behavior.
type SSELifecycleConfig struct {
	MaxEvents int `json:"maxEvents,omitempty"`
}

// SSEResumeConfig controls Last-Event-ID resumption.
type SSEResumeConfig struct {
	Enabled    bool `json:"enabled"`
	BufferSize int  `json:"bufferSize,omitempty"`
}

// SSEMockMetadata contains info about the source recording.
type SSEMockMetadata struct {
	SourceRecordingID string    `json:"sourceRecordingId"`
	OriginalPath      string    `json:"originalPath,omitempty"`
	RecordedAt        time.Time `json:"recordedAt"`
	TotalEvents       int       `json:"totalEvents"`
	Duration          int64     `json:"durationMs"`
	Format            string    `json:"format,omitempty"` // e.g., "openai"
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
	matcher := &config.RequestMatcher{
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

	response := &config.ResponseDefinition{
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
	return &config.MockConfiguration{
		ID:        generateID(),
		Matcher:   matcher,
		Response:  response,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
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

// ToWebSocketScenario converts a WebSocket stream recording to a scenario config.
func ToWebSocketScenario(recording *StreamRecording, opts StreamConvertOptions) (*WebSocketScenarioConfig, error) {
	if recording == nil || recording.Protocol != ProtocolWebSocket || recording.WebSocket == nil {
		return nil, ErrRecordingNotFound
	}

	ws := recording.WebSocket
	scenario := &WebSocketScenarioConfig{
		Name:  "Recorded: " + recording.ID,
		Steps: make([]WebSocketScenarioStep, 0, len(ws.Frames)),
		Metadata: &WebSocketScenarioMetadata{
			SourceRecordingID: recording.ID,
			OriginalPath:      recording.Metadata.Path,
			RecordedAt:        recording.StartTime,
			TotalFrames:       len(ws.Frames),
		},
	}

	// Calculate duration
	if len(ws.Frames) > 0 {
		scenario.Metadata.Duration = ws.Frames[len(ws.Frames)-1].RelativeMs
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
			step := WebSocketScenarioStep{
				Type: "send",
				Message: &WebSocketMessageConfig{
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
			step := WebSocketScenarioStep{
				Type:     "expect",
				Timeout:  "30s",
				Optional: false,
				Match: &WebSocketMatchConfig{
					Type:  frameMessageTypeToString(frame.MessageType),
					Exact: msgStr,
				},
			}
			scenario.Steps = append(scenario.Steps, step)
		}
	}

	return scenario, nil
}

// ToSSEConfig converts an SSE stream recording to an SSE mock config.
func ToSSEConfig(recording *StreamRecording, opts StreamConvertOptions) (*SSEMockConfig, error) {
	if recording == nil || recording.Protocol != ProtocolSSE || recording.SSE == nil {
		return nil, ErrRecordingNotFound
	}

	sse := recording.SSE
	cfg := &SSEMockConfig{
		Events: make([]SSEEventConfig, 0, len(sse.Events)),
		Timing: SSETimingConfig{
			PerEventDelays: make([]int, 0, len(sse.Events)),
		},
		Lifecycle: SSELifecycleConfig{
			MaxEvents: len(sse.Events),
		},
		Resume: SSEResumeConfig{
			Enabled:    true,
			BufferSize: len(sse.Events),
		},
		Metadata: &SSEMockMetadata{
			SourceRecordingID: recording.ID,
			OriginalPath:      recording.Metadata.Path,
			RecordedAt:        recording.StartTime,
			TotalEvents:       len(sse.Events),
			Format:            recording.Metadata.DetectedTemplate,
		},
	}

	// Calculate duration
	if len(sse.Events) > 0 {
		cfg.Metadata.Duration = sse.Events[len(sse.Events)-1].RelativeMs
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

		eventCfg := SSEEventConfig{
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

	return cfg, nil
}

// StreamConvertResult holds the result of converting a stream recording.
type StreamConvertResult struct {
	Protocol   Protocol    `json:"protocol"`
	Config     interface{} `json:"config"`     // WebSocketScenarioConfig or SSEMockConfig
	ConfigJSON []byte      `json:"configJson"` // JSON-encoded config
	ConfigYAML []byte      `json:"configYaml"` // YAML-encoded config (if requested)
}

// ConvertStreamRecording converts a stream recording to the appropriate mock config.
func ConvertStreamRecording(recording *StreamRecording, opts StreamConvertOptions) (*StreamConvertResult, error) {
	result := &StreamConvertResult{
		Protocol: recording.Protocol,
	}

	var err error
	switch recording.Protocol {
	case ProtocolWebSocket:
		result.Config, err = ToWebSocketScenario(recording, opts)
	case ProtocolSSE:
		result.Config, err = ToSSEConfig(recording, opts)
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
