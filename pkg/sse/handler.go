package sse

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/metrics"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/protocol"
	"github.com/getmockd/mockd/pkg/recording"
	"github.com/getmockd/mockd/pkg/requestlog"
	"github.com/getmockd/mockd/pkg/util"
)

// SSERecordingHookFactory creates SSE recording hooks for new connections.
type SSERecordingHookFactory func(streamID string, mockID string, path string) (recording.SSERecordingHook, error)

// SSEHandler handles SSE streaming responses for mock endpoints.
type SSEHandler struct {
	encoder              *Encoder
	manager              *SSEConnectionManager
	buffersMu            sync.RWMutex            // mutex for thread-safe buffers access
	buffers              map[string]*EventBuffer // buffers by mock ID
	templates            *TemplateRegistry
	nextConnID           atomic.Int64
	recordingHookFactory SSERecordingHookFactory // factory to create per-connection hooks
	id                   string                  // handler ID for protocol.Handler interface
	requestLoggerMu      sync.RWMutex            // mutex for thread-safe requestLogger access
	requestLogger        requestlog.Logger       // logger for SSE request events
}

// NewSSEHandler creates a new SSE handler.
func NewSSEHandler(maxConnections int) *SSEHandler {
	return &SSEHandler{
		encoder:   NewEncoder(),
		manager:   NewConnectionManager(maxConnections),
		buffers:   make(map[string]*EventBuffer),
		templates: NewTemplateRegistry(),
	}
}

// SetRecordingHookFactory sets a factory for creating per-connection recording hooks.
func (h *SSEHandler) SetRecordingHookFactory(factory SSERecordingHookFactory) {
	h.recordingHookFactory = factory
}

// GetRecordingHookFactory returns the current recording hook factory.
func (h *SSEHandler) GetRecordingHookFactory() SSERecordingHookFactory {
	return h.recordingHookFactory
}

// SetRequestLogger sets the request logger for SSE events.
// This method is thread-safe.
func (h *SSEHandler) SetRequestLogger(logger requestlog.Logger) {
	h.requestLoggerMu.Lock()
	defer h.requestLoggerMu.Unlock()
	h.requestLogger = logger
}

// GetRequestLogger returns the current request logger.
// This method is thread-safe.
func (h *SSEHandler) GetRequestLogger() requestlog.Logger {
	h.requestLoggerMu.RLock()
	defer h.requestLoggerMu.RUnlock()
	return h.requestLogger
}

// logConnection logs an SSE connection open or close event.
func (h *SSEHandler) logConnection(stream *SSEStream, path string, mockID string, isOpen bool, eventCount int, durationMs int) {
	if h.requestLogger == nil {
		return
	}

	method := "CONNECT"
	if !isOpen {
		method = "DISCONNECT"
	}

	entry := &requestlog.Entry{
		ID:            generateLogID(),
		Timestamp:     time.Now(),
		Protocol:      requestlog.ProtocolSSE,
		Method:        method,
		Path:          path,
		MatchedMockID: mockID,
		DurationMs:    durationMs,
		SSE: &requestlog.SSEMeta{
			ConnectionID: stream.ID,
			IsConnection: true,
			EventCount:   eventCount,
		},
	}

	h.requestLogger.Log(entry)
}

// logEvent logs an SSE event sent to a client.
func (h *SSEHandler) logEvent(stream *SSEStream, path string, mockID string, event *SSEEventDef) {
	if h.requestLogger == nil {
		return
	}

	// Truncate event data for logging
	body := util.TruncateBody(formatEventData(event.Data), 1024)

	entry := &requestlog.Entry{
		ID:            generateLogID(),
		Timestamp:     time.Now(),
		Protocol:      requestlog.ProtocolSSE,
		Method:        "EVENT",
		Path:          path,
		Body:          body,
		BodySize:      len(body),
		MatchedMockID: mockID,
		SSE: &requestlog.SSEMeta{
			ConnectionID: stream.ID,
			EventType:    event.Type,
			EventID:      event.ID,
		},
	}

	h.requestLogger.Log(entry)
}

// generateLogID generates a unique log entry ID.
func generateLogID() string {
	return "sse-log-" + formatInt64(time.Now().UnixNano())
}

// formatEventData formats event data as a string for logging.
func formatEventData(data interface{}) string {
	switch v := data.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case map[string]interface{}:
		// Simple JSON-like formatting without importing encoding/json
		return formatMapData(v)
	default:
		return "<data>"
	}
}

// formatMapData formats a map as a simple JSON-like string.
func formatMapData(m map[string]interface{}) string {
	if len(m) == 0 {
		return "{}"
	}
	result := "{"
	first := true
	for k, v := range m {
		if !first {
			result += ","
		}
		first = false
		result += "\"" + k + "\":"
		switch val := v.(type) {
		case string:
			result += "\"" + val + "\""
		case map[string]interface{}:
			result += formatMapData(val)
		default:
			result += "<value>"
		}
	}
	result += "}"
	return result
}

// ServeHTTP handles an SSE request for a given mock configuration.
func (h *SSEHandler) ServeHTTP(w http.ResponseWriter, r *http.Request, m *config.MockConfiguration) {
	if m.HTTP == nil || m.HTTP.SSE == nil {
		http.Error(w, "SSE configuration missing", http.StatusInternalServerError)
		return
	}

	// Check if Flusher is supported
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	h.setSSEHeaders(w)

	// Create stream context
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Create stream
	stream := &SSEStream{
		ID:        h.generateStreamID(),
		MockID:    m.ID,
		ClientIP:  r.RemoteAddr,
		UserAgent: r.UserAgent(),
		StartTime: time.Now(),
		Status:    StreamStatusConnecting,
		ctx:       ctx,
		cancel:    cancel,
		writer:    w,
		flusher:   flusher,
		config:    h.configFromMock(m.HTTP.SSE),
	}

	// Check for Last-Event-ID header for resumption
	if lastEventID := r.Header.Get("Last-Event-ID"); lastEventID != "" {
		stream.ResumedFrom = lastEventID
	}

	// Register connection
	if err := h.manager.Register(stream); err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer h.manager.Deregister(stream.ID)

	// Extract path for logging and store in stream
	if m.HTTP != nil && m.HTTP.Matcher != nil {
		stream.Path = m.HTTP.Matcher.Path
	}

	// Set up recording if hook factory is configured
	if h.recordingHookFactory != nil {
		hook, err := h.recordingHookFactory(stream.ID, m.ID, stream.Path)
		if err == nil && hook != nil {
			stream.recorder = NewStreamRecorder(stream, hook)
			defer func() {
				if stream.recorder != nil {
					_ = stream.recorder.Complete()
				}
			}()
		}
	}

	// Log connection open
	h.logConnection(stream, stream.Path, m.ID, true, 0, 0)

	// Run the stream
	stream.Status = StreamStatusActive
	h.runStream(stream, m.HTTP.SSE)

	// Log connection close with duration and event count
	durationMs := int(time.Since(stream.StartTime).Milliseconds())
	h.logConnection(stream, stream.Path, m.ID, false, int(stream.EventsSent), durationMs)
}

// setSSEHeaders sets required HTTP headers for SSE.
func (h *SSEHandler) setSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", ContentTypeEventStream)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Disable nginx buffering
}

// generateStreamID generates a unique stream identifier.
func (h *SSEHandler) generateStreamID() string {
	id := h.nextConnID.Add(1)
	return formatStreamID(id)
}

// formatStreamID formats a stream ID.
func formatStreamID(id int64) string {
	return "sse-" + formatInt64(id)
}

// formatInt64 formats an int64 as a string.
func formatInt64(n int64) string {
	if n == 0 {
		return "0"
	}

	var digits [20]byte
	i := len(digits)

	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}

	return string(digits[i:])
}

// configFromMock creates an internal SSEConfig from mock configuration.
func (h *SSEHandler) configFromMock(cfg *mock.SSEConfig) *SSEConfig {
	sseConfig := &SSEConfig{
		Template:       cfg.Template,
		TemplateParams: cfg.TemplateParams,
	}

	// Copy events
	sseConfig.Events = append(sseConfig.Events, cfg.Events...)

	// Copy timing config
	sseConfig.Timing.InitialDelay = cfg.Timing.InitialDelay
	sseConfig.Timing.FixedDelay = cfg.Timing.FixedDelay
	sseConfig.Timing.PerEventDelays = cfg.Timing.PerEventDelays

	if cfg.Timing.RandomDelay != nil {
		sseConfig.Timing.RandomDelay = &RandomDelayConfig{
			Min: cfg.Timing.RandomDelay.Min,
			Max: cfg.Timing.RandomDelay.Max,
		}
	}

	if cfg.Timing.Burst != nil {
		sseConfig.Timing.Burst = &BurstConfig{
			Count:    cfg.Timing.Burst.Count,
			Interval: cfg.Timing.Burst.Interval,
			Pause:    cfg.Timing.Burst.Pause,
		}
	}

	// Copy lifecycle config
	sseConfig.Lifecycle.KeepaliveInterval = cfg.Lifecycle.KeepaliveInterval
	sseConfig.Lifecycle.MaxEvents = cfg.Lifecycle.MaxEvents
	sseConfig.Lifecycle.Timeout = cfg.Lifecycle.Timeout
	sseConfig.Lifecycle.ConnectionTimeout = cfg.Lifecycle.ConnectionTimeout
	sseConfig.Lifecycle.SimulateDisconnect = cfg.Lifecycle.SimulateDisconnect

	sseConfig.Lifecycle.Termination.Type = cfg.Lifecycle.Termination.Type
	sseConfig.Lifecycle.Termination.CloseDelay = cfg.Lifecycle.Termination.CloseDelay

	if cfg.Lifecycle.Termination.FinalEvent != nil {
		sseConfig.Lifecycle.Termination.FinalEvent = &SSEEventDef{
			Type:    cfg.Lifecycle.Termination.FinalEvent.Type,
			Data:    cfg.Lifecycle.Termination.FinalEvent.Data,
			ID:      cfg.Lifecycle.Termination.FinalEvent.ID,
			Retry:   cfg.Lifecycle.Termination.FinalEvent.Retry,
			Comment: cfg.Lifecycle.Termination.FinalEvent.Comment,
		}
	}

	// Copy resume config
	sseConfig.Resume.Enabled = cfg.Resume.Enabled
	sseConfig.Resume.BufferSize = cfg.Resume.BufferSize
	sseConfig.Resume.MaxAge = cfg.Resume.MaxAge

	// Copy rate limit config
	if cfg.RateLimit != nil {
		sseConfig.RateLimit = &RateLimitConfig{
			EventsPerSecond: cfg.RateLimit.EventsPerSecond,
			BurstSize:       cfg.RateLimit.BurstSize,
			Strategy:        cfg.RateLimit.Strategy,
			Headers:         cfg.RateLimit.Headers,
		}
	}

	// Copy generator config
	if cfg.Generator != nil {
		sseConfig.Generator = &EventGenerator{
			Type:  cfg.Generator.Type,
			Count: cfg.Generator.Count,
		}

		if cfg.Generator.Sequence != nil {
			sseConfig.Generator.Sequence = &SequenceGenerator{
				Start:     cfg.Generator.Sequence.Start,
				Increment: cfg.Generator.Sequence.Increment,
				Format:    cfg.Generator.Sequence.Format,
			}
		}

		if cfg.Generator.Random != nil {
			sseConfig.Generator.Random = &RandomGenerator{
				Schema: cfg.Generator.Random.Schema,
			}
		}

		if cfg.Generator.Template != nil {
			sseConfig.Generator.Template = &TemplateGenerator{
				Repeat: cfg.Generator.Template.Repeat,
			}
			sseConfig.Generator.Template.Events = append(sseConfig.Generator.Template.Events, cfg.Generator.Template.Events...)
		}
	}

	return sseConfig
}

// runStream runs the SSE stream event loop.
func (h *SSEHandler) runStream(stream *SSEStream, cfg *mock.SSEConfig) {
	ctx := stream.ctx
	timing := NewTimingScheduler(&stream.config.Timing)

	// Initialize rate limiter if configured
	var rateLimiter *RateLimiter
	if stream.config.RateLimit != nil && stream.config.RateLimit.EventsPerSecond > 0 {
		rateLimiter = NewRateLimiter(stream.config.RateLimit)
	}

	// Apply initial delay
	if stream.config.Timing.InitialDelay > 0 {
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Duration(stream.config.Timing.InitialDelay) * time.Millisecond):
		}
	}

	// Set up keepalive ticker if configured
	var keepaliveTicker *time.Ticker
	var keepaliveCh <-chan time.Time
	if stream.config.Lifecycle.KeepaliveInterval > 0 {
		keepaliveTicker = time.NewTicker(time.Duration(stream.config.Lifecycle.KeepaliveInterval) * time.Second)
		keepaliveCh = keepaliveTicker.C
		defer keepaliveTicker.Stop()
	}

	// Set up connection timeout if configured
	var timeoutTimer *time.Timer
	var timeoutCh <-chan time.Time
	if stream.config.Lifecycle.ConnectionTimeout > 0 {
		timeoutTimer = time.NewTimer(time.Duration(stream.config.Lifecycle.ConnectionTimeout) * time.Second)
		timeoutCh = timeoutTimer.C
		defer timeoutTimer.Stop()
	}

	// Get events to send
	events := h.getEventsForStream(stream, cfg)

	// Handle Last-Event-ID resumption
	startIndex := 0
	if stream.ResumedFrom != "" && stream.config.Resume.Enabled {
		startIndex = h.findResumePosition(stream, events)
	}

	// Main event loop
	eventIndex := startIndex
	var eventCount int64

	for {
		// Check max events limit
		if stream.config.Lifecycle.MaxEvents > 0 && eventCount >= int64(stream.config.Lifecycle.MaxEvents) {
			h.handleTermination(stream, TerminationGraceful)
			return
		}

		// Check simulated disconnect
		if stream.config.Lifecycle.SimulateDisconnect != nil && eventCount >= int64(*stream.config.Lifecycle.SimulateDisconnect) {
			h.handleTermination(stream, TerminationAbrupt)
			return
		}

		// Check if we have more events
		if eventIndex >= len(events) {
			// For generators, we might loop or generate more
			if stream.config.Generator != nil {
				// Regenerate events for continuous streams
				events = h.generateEvents(stream.config.Generator)
				eventIndex = 0
				if len(events) == 0 {
					h.handleTermination(stream, TerminationGraceful)
					return
				}
			} else {
				// No more events, terminate gracefully
				h.handleTermination(stream, TerminationGraceful)
				return
			}
		}

		// Get next delay
		event := events[eventIndex]
		delay := timing.NextDelay(eventIndex, event.Delay)

		select {
		case <-ctx.Done():
			// Client disconnected
			stream.Status = StreamStatusClosed
			return

		case <-timeoutCh:
			// Connection timeout
			h.handleTermination(stream, TerminationGraceful)
			return

		case <-keepaliveCh:
			// Send keepalive
			if err := h.sendKeepalive(stream); err != nil {
				stream.Status = StreamStatusClosed
				return
			}

		case <-time.After(delay):
			// Apply rate limiting if configured
			if rateLimiter != nil {
				if err := rateLimiter.Wait(ctx); err != nil {
					if err == ErrRateLimited {
						// Drop strategy - skip this event
						eventIndex++
						continue
					}
					// Context cancelled or error strategy
					stream.Status = StreamStatusClosed
					return
				}
			}

			// Send event
			if err := h.sendEvent(stream, &event, eventCount); err != nil {
				stream.Status = StreamStatusClosed
				return
			}
			eventIndex++
			eventCount++
			stream.EventsSent = eventCount
		}
	}
}

// getEventsForStream returns events for the stream based on configuration.
func (h *SSEHandler) getEventsForStream(stream *SSEStream, cfg *mock.SSEConfig) []SSEEventDef {
	// Check for template first
	if cfg.Template != "" {
		if generator, ok := h.templates.Get(cfg.Template); ok {
			return generator(cfg.TemplateParams)
		}
	}

	// Check for generator
	if stream.config.Generator != nil {
		return h.generateEvents(stream.config.Generator)
	}

	// Return static events
	return stream.config.Events
}

// generateEvents generates events using the configured generator.
func (h *SSEHandler) generateEvents(gen *EventGenerator) []SSEEventDef {
	if gen == nil {
		return nil
	}

	switch gen.Type {
	case GeneratorSequence:
		return h.generateSequenceEvents(gen)
	case GeneratorRandom:
		return h.generateRandomEvents(gen)
	case GeneratorTemplate:
		return h.generateTemplateEvents(gen)
	default:
		return nil
	}
}

// generateSequenceEvents generates sequential events.
func (h *SSEHandler) generateSequenceEvents(gen *EventGenerator) []SSEEventDef {
	if gen.Sequence == nil {
		return nil
	}

	count := gen.Count
	if count == 0 {
		count = 100 // Default batch size for unlimited
	}

	events := make([]SSEEventDef, 0, count)
	seq := gen.Sequence

	for i := 0; i < count; i++ {
		value := seq.Start + (i * seq.Increment)
		var data interface{}
		if seq.Format != "" {
			data = formatSequenceValue(seq.Format, value)
		} else {
			data = value
		}
		events = append(events, SSEEventDef{
			Data: data,
			ID:   formatInt64(int64(value)),
		})
	}

	return events
}

// formatSequenceValue formats a sequence value using the format string.
func formatSequenceValue(format string, value int) string {
	// Simple %d substitution
	result := format
	for i := 0; i < len(result)-1; i++ {
		if result[i] == '%' && result[i+1] == 'd' {
			result = result[:i] + formatInt(value) + result[i+2:]
		}
	}
	return result
}

// formatInt formats an int as a string.
func formatInt(n int) string {
	if n == 0 {
		return "0"
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var digits [20]byte
	i := len(digits)

	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}

	if negative {
		i--
		digits[i] = '-'
	}

	return string(digits[i:])
}

// generateRandomEvents generates random events.
func (h *SSEHandler) generateRandomEvents(gen *EventGenerator) []SSEEventDef {
	if gen.Random == nil {
		return nil
	}

	count := gen.Count
	if count == 0 {
		count = 100 // Default batch size for unlimited
	}

	events := make([]SSEEventDef, 0, count)
	for i := 0; i < count; i++ {
		data := h.processRandomSchema(gen.Random.Schema)
		events = append(events, SSEEventDef{
			Data: data,
			ID:   formatInt64(int64(i + 1)),
		})
	}

	return events
}

// processRandomSchema processes a schema with random placeholders.
func (h *SSEHandler) processRandomSchema(schema map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range schema {
		result[key] = h.processRandomValue(value)
	}
	return result
}

// processRandomValue processes a single value with random placeholders.
func (h *SSEHandler) processRandomValue(value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		return processRandomPlaceholder(v)
	case map[string]interface{}:
		return h.processRandomSchema(v)
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = h.processRandomValue(item)
		}
		return result
	default:
		return value
	}
}

// generateTemplateEvents generates events from a template.
func (h *SSEHandler) generateTemplateEvents(gen *EventGenerator) []SSEEventDef {
	if gen.Template == nil || len(gen.Template.Events) == 0 {
		return nil
	}

	repeat := gen.Template.Repeat
	if repeat == 0 {
		repeat = 1 // At least one iteration
	}

	count := gen.Count
	if count > 0 {
		// Limit total events
		totalEvents := len(gen.Template.Events) * repeat
		if totalEvents > count {
			repeat = (count / len(gen.Template.Events)) + 1
		}
	}

	events := make([]SSEEventDef, 0, len(gen.Template.Events)*repeat)
	eventID := int64(1)

	for r := 0; r < repeat; r++ {
		for _, e := range gen.Template.Events {
			event := SSEEventDef{
				Type:    e.Type,
				Data:    e.Data,
				ID:      formatInt64(eventID),
				Retry:   e.Retry,
				Comment: e.Comment,
				Delay:   e.Delay,
			}
			events = append(events, event)
			eventID++

			if count > 0 && len(events) >= count {
				return events
			}
		}
	}

	return events
}

// findResumePosition finds the position to resume from based on Last-Event-ID.
func (h *SSEHandler) findResumePosition(stream *SSEStream, events []SSEEventDef) int {
	lastID := stream.ResumedFrom

	// First check the buffer (thread-safe access)
	h.buffersMu.RLock()
	buffer := h.buffers[stream.MockID]
	h.buffersMu.RUnlock()
	if buffer != nil {
		bufferedEvents := buffer.GetEventsAfterID(lastID)
		if len(bufferedEvents) > 0 {
			// Found in buffer - prepend to events
			return 0 // Start from beginning, we'll handle buffered events separately
		}
	}

	// Search in events array
	for i, e := range events {
		if e.ID == lastID {
			return i + 1 // Resume from next event
		}
	}

	return 0 // Not found, start from beginning
}

// sendEvent sends an event to the client.
func (h *SSEHandler) sendEvent(stream *SSEStream, event *SSEEventDef, eventIndex int64) error {
	// Assign ID if not set and resume is enabled
	if event.ID == "" && stream.config.Resume.Enabled {
		event.ID = formatInt64(eventIndex + 1)
	}

	// Format the event
	formatted, err := h.encoder.FormatEvent(event)
	if err != nil {
		return err
	}

	// Write to response
	stream.mu.Lock()
	_, err = stream.writer.Write([]byte(formatted))
	if err != nil {
		stream.mu.Unlock()
		return err
	}
	stream.flusher.Flush()
	stream.mu.Unlock()

	// Log the event
	h.logEvent(stream, stream.Path, stream.MockID, event)

	// Record the event if recording is enabled
	if stream.recorder != nil {
		if recordErr := stream.recorder.RecordEventDef(event); recordErr != nil {
			// Recording errors are non-fatal, but we signal the error to the hook
			stream.recorder.Error(recordErr)
		}
	}

	// Update stream state
	now := time.Now()
	stream.LastEventTime = &now
	stream.LastEventID = event.ID
	stream.BytesSent += int64(len(formatted))

	// Buffer event for resumption
	if stream.config.Resume.Enabled {
		h.bufferEvent(stream.MockID, event, eventIndex)
	}

	// Update manager metrics
	h.manager.recordEventSent(int64(len(formatted)))

	// Record Prometheus metric
	if metrics.RequestsTotal != nil {
		if vec, err := metrics.RequestsTotal.WithLabels("sse", stream.Path, "event"); err == nil {
			_ = vec.Inc()
		}
	}

	return nil
}

// sendKeepalive sends a keepalive comment to the client.
func (h *SSEHandler) sendKeepalive(stream *SSEStream) error {
	keepalive := h.encoder.FormatKeepalive()

	stream.mu.Lock()
	_, err := stream.writer.Write([]byte(keepalive))
	if err != nil {
		stream.mu.Unlock()
		return err
	}
	stream.flusher.Flush()
	stream.mu.Unlock()

	stream.BytesSent += int64(len(keepalive))
	return nil
}

// handleTermination handles stream termination.
func (h *SSEHandler) handleTermination(stream *SSEStream, terminationType string) {
	stream.Status = StreamStatusClosing

	termConfig := stream.config.Lifecycle.Termination

	switch terminationType {
	case TerminationGraceful:
		// Send final event if configured
		if termConfig.FinalEvent != nil {
			_ = h.sendEvent(stream, termConfig.FinalEvent, stream.EventsSent)
		}
		// Apply close delay
		if termConfig.CloseDelay > 0 {
			time.Sleep(time.Duration(termConfig.CloseDelay) * time.Millisecond)
		}

	case TerminationAbrupt:
		// Just close immediately without any final message

	case TerminationError:
		// Send error event if configured
		if termConfig.ErrorEvent != nil {
			_ = h.sendEvent(stream, termConfig.ErrorEvent, stream.EventsSent)
		}
	}

	stream.Status = StreamStatusClosed
}

// bufferEvent adds an event to the resumption buffer.
// This method is thread-safe.
func (h *SSEHandler) bufferEvent(mockID string, event *SSEEventDef, index int64) {
	h.buffersMu.Lock()
	buffer, ok := h.buffers[mockID]
	if !ok {
		// Create buffer with default size
		buffer = NewEventBuffer(DefaultBufferSize, 0)
		h.buffers[mockID] = buffer
	}
	h.buffersMu.Unlock()

	buffer.Add(BufferedEvent{
		ID:        event.ID,
		Event:     *event,
		Timestamp: time.Now(),
		Index:     index,
	})
}

// GetManager returns the connection manager.
func (h *SSEHandler) GetManager() *SSEConnectionManager {
	return h.manager
}

// GetTemplates returns the template registry.
func (h *SSEHandler) GetTemplates() *TemplateRegistry {
	return h.templates
}

// GetBuffer returns the event buffer for a mock.
// This method is thread-safe.
func (h *SSEHandler) GetBuffer(mockID string) *EventBuffer {
	h.buffersMu.RLock()
	defer h.buffersMu.RUnlock()
	return h.buffers[mockID]
}

// ClearBuffer clears the event buffer for a mock.
// This method is thread-safe.
func (h *SSEHandler) ClearBuffer(mockID string) {
	h.buffersMu.Lock()
	defer h.buffersMu.Unlock()
	delete(h.buffers, mockID)
}

// --- protocol.Handler interface implementation ---

// ID returns the unique identifier for this handler.
func (h *SSEHandler) ID() string {
	return h.id
}

// SetID sets the unique identifier for this handler.
func (h *SSEHandler) SetID(id string) {
	h.id = id
}

// Protocol returns the protocol type for this handler.
func (h *SSEHandler) Protocol() protocol.Protocol {
	return protocol.ProtocolSSE
}

// Metadata returns descriptive information about this handler.
func (h *SSEHandler) Metadata() protocol.Metadata {
	return protocol.Metadata{
		ID:                   h.id,
		Protocol:             protocol.ProtocolSSE,
		Version:              "0.2.4",
		TransportType:        protocol.TransportHTTP1,
		ConnectionModel:      protocol.ConnectionModelPersistent,
		CommunicationPattern: protocol.PatternServerPush,
		Capabilities: []protocol.Capability{
			protocol.CapabilityConnections,
			protocol.CapabilityStreaming,
			protocol.CapabilityMetrics,
		},
	}
}

// Start activates the handler. For SSE (HTTP-based), this is a no-op.
func (h *SSEHandler) Start(ctx context.Context) error {
	return nil
}

// Stop gracefully shuts down the handler, closing all connections.
func (h *SSEHandler) Stop(ctx context.Context, timeout time.Duration) error {
	if h.manager != nil {
		h.manager.CloseAll()
	}
	return nil
}

// Health returns the current health status of the handler.
func (h *SSEHandler) Health(ctx context.Context) protocol.HealthStatus {
	return protocol.HealthStatus{
		Status:    protocol.HealthHealthy,
		CheckedAt: time.Now(),
	}
}

// --- protocol.StreamingHTTPHandler interface implementation ---

// Pattern returns the URL pattern for this handler.
// SSE paths are dynamic per mock, so this returns empty string.
func (h *SSEHandler) Pattern() string {
	return "" // SSE paths are dynamic per mock
}

// IsStreamingRequest returns true if the request is a streaming request.
// All SSE requests are streaming requests.
func (h *SSEHandler) IsStreamingRequest(r *http.Request) bool {
	return true // All SSE requests are streaming
}

// --- protocol.ConnectionManager interface implementation ---

// ConnectionCount returns the number of active connections.
func (h *SSEHandler) ConnectionCount() int {
	if h.manager == nil {
		return 0
	}
	return h.manager.Count()
}

// ListConnections returns information about all active connections.
func (h *SSEHandler) ListConnections() []protocol.ConnectionInfo {
	if h.manager == nil {
		return nil
	}

	streams := h.manager.GetConnections()
	conns := make([]protocol.ConnectionInfo, 0, len(streams))
	for _, stream := range streams {
		var lastActivity time.Time
		if stream.LastEventTime != nil {
			lastActivity = *stream.LastEventTime
		} else {
			lastActivity = stream.StartTime
		}
		conns = append(conns, protocol.ConnectionInfo{
			ID:           stream.ID,
			RemoteAddr:   stream.ClientIP,
			ConnectedAt:  stream.StartTime,
			LastActivity: lastActivity,
			BytesSent:    stream.BytesSent,
			Metadata: map[string]any{
				"mockId":     stream.MockID,
				"path":       stream.Path,
				"userAgent":  stream.UserAgent,
				"eventsSent": stream.EventsSent,
				"status":     string(stream.Status),
			},
		})
	}
	return conns
}

// CloseConnection closes a connection by ID with the given reason.
func (h *SSEHandler) CloseConnection(id string, reason string) error {
	if h.manager == nil {
		return ErrStreamClosed
	}
	return h.manager.Close(id, true, nil)
}

// GetConnection returns information about a specific connection.
func (h *SSEHandler) GetConnection(id string) (*protocol.ConnectionInfo, error) {
	if h.manager == nil {
		return nil, ErrStreamClosed
	}

	stream := h.manager.Get(id)
	if stream == nil {
		return nil, ErrStreamClosed
	}

	var lastActivity time.Time
	if stream.LastEventTime != nil {
		lastActivity = *stream.LastEventTime
	} else {
		lastActivity = stream.StartTime
	}

	return &protocol.ConnectionInfo{
		ID:           stream.ID,
		RemoteAddr:   stream.ClientIP,
		ConnectedAt:  stream.StartTime,
		LastActivity: lastActivity,
		BytesSent:    stream.BytesSent,
		Metadata: map[string]any{
			"mockId":     stream.MockID,
			"path":       stream.Path,
			"userAgent":  stream.UserAgent,
			"eventsSent": stream.EventsSent,
			"status":     string(stream.Status),
		},
	}, nil
}

// CloseAllConnections closes all connections with the given reason.
func (h *SSEHandler) CloseAllConnections(reason string) int {
	if h.manager == nil {
		return 0
	}
	count := h.manager.Count()
	h.manager.CloseAll()
	return count
}

// --- Interface compliance checks ---

var _ protocol.Handler = (*SSEHandler)(nil)
var _ protocol.RequestLoggable = (*SSEHandler)(nil)
var _ protocol.ConnectionManager = (*SSEHandler)(nil)
