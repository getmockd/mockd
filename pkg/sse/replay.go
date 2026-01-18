// Package sse provides SSE replay functionality.
package sse

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// Replay errors.
var (
	// ErrSSEReplayNotStarted indicates the replay hasn't been started.
	ErrSSEReplayNotStarted = errors.New("SSE replay not started")
	// ErrSSEReplayAlreadyStarted indicates the replay is already running.
	ErrSSEReplayAlreadyStarted = errors.New("SSE replay already started")
	// ErrSSEReplayStopped indicates the replay was stopped.
	ErrSSEReplayStopped = errors.New("SSE replay stopped")
	// ErrSSEInvalidRecording indicates the recording is invalid for replay.
	ErrSSEInvalidRecording = errors.New("invalid SSE recording for replay")
	// ErrSSENoEventsToReplay indicates there are no events to replay.
	ErrSSENoEventsToReplay = errors.New("no SSE events to replay")
	// ErrSSETriggeredModeOnly indicates the operation is only valid in triggered mode.
	ErrSSETriggeredModeOnly = errors.New("operation only valid in triggered mode")
)

// SSEReplayConfig configures how an SSE recording is replayed.
type SSEReplayConfig struct {
	// Mode is the replay mode: "pure" or "triggered".
	// Note: SSE is server-to-client only, so "synchronized" mode doesn't apply.
	Mode recording.ReplayMode `json:"mode"`

	// TimingScale adjusts the replay speed (1.0 = original speed, 0.5 = half speed, 2.0 = double speed).
	// Only used in Pure mode.
	TimingScale float64 `json:"timingScale,omitempty"`

	// InitialDelay is the delay before sending the first event (ms).
	InitialDelay int `json:"initialDelay,omitempty"`
}

// DefaultSSEReplayConfig returns the default SSE replay configuration.
func DefaultSSEReplayConfig() SSEReplayConfig {
	return SSEReplayConfig{
		Mode:        recording.ReplayModePure,
		TimingScale: 1.0,
	}
}

// SSEReplayer replays an SSE recording through an HTTP response.
type SSEReplayer struct {
	recording *recording.StreamRecording
	writer    http.ResponseWriter
	flusher   http.Flusher
	encoder   *Encoder
	config    SSEReplayConfig

	currentEvent int
	eventsSent   int
	startTime    time.Time

	ctx    context.Context
	cancel context.CancelFunc

	// For triggered mode
	advanceChan chan int // number of events to send

	mu      sync.RWMutex
	started bool
	status  recording.ReplayStatus
}

// NewSSEReplayer creates a replayer for an SSE stream.
func NewSSEReplayer(rec *recording.StreamRecording, w http.ResponseWriter, config SSEReplayConfig) (*SSEReplayer, error) {
	if rec == nil {
		return nil, ErrSSEInvalidRecording
	}

	if rec.Protocol != recording.ProtocolSSE {
		return nil, ErrSSEInvalidRecording
	}

	if rec.SSE == nil || len(rec.SSE.Events) == 0 {
		return nil, ErrSSENoEventsToReplay
	}

	// Validate replay mode - SSE only supports pure and triggered
	if config.Mode != recording.ReplayModePure && config.Mode != recording.ReplayModeTriggered {
		config.Mode = recording.ReplayModePure
	}

	// Set defaults
	if config.TimingScale <= 0 {
		config.TimingScale = 1.0
	}

	// Check for flusher support
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, errors.New("streaming not supported")
	}

	ctx, cancel := context.WithCancel(context.Background())

	r := &SSEReplayer{
		recording:    rec,
		writer:       w,
		flusher:      flusher,
		encoder:      NewEncoder(),
		config:       config,
		currentEvent: 0,
		eventsSent:   0,
		ctx:          ctx,
		cancel:       cancel,
		status:       recording.ReplayStatusPending,
	}

	// Initialize mode-specific channels
	if config.Mode == recording.ReplayModeTriggered {
		r.advanceChan = make(chan int, 10)
	}

	return r, nil
}

// SetSSEHeaders sets the required SSE headers on the response.
func (r *SSEReplayer) SetSSEHeaders() {
	r.writer.Header().Set("Content-Type", ContentTypeEventStream)
	r.writer.Header().Set("Cache-Control", "no-cache")
	r.writer.Header().Set("Connection", "keep-alive")
	r.writer.Header().Set("X-Accel-Buffering", "no")
}

// Start begins replaying the recording.
// This blocks until replay is complete or stopped.
func (r *SSEReplayer) Start() error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return ErrSSEReplayAlreadyStarted
	}
	r.started = true
	r.startTime = time.Now()
	r.status = recording.ReplayStatusPlaying
	r.mu.Unlock()

	// Set headers before starting
	r.SetSSEHeaders()

	var err error
	switch r.config.Mode {
	case recording.ReplayModePure:
		err = r.playPure()
	case recording.ReplayModeTriggered:
		err = r.playTriggered()
	}

	r.mu.Lock()
	if err != nil && !errors.Is(err, ErrSSEReplayStopped) && !errors.Is(err, context.Canceled) {
		r.status = recording.ReplayStatusAborted
	} else if r.status == recording.ReplayStatusPlaying {
		r.status = recording.ReplayStatusComplete
	}
	r.mu.Unlock()

	return err
}

// playPure replays with original timing.
func (r *SSEReplayer) playPure() error {
	events := r.recording.SSE.Events
	if len(events) == 0 {
		return nil
	}

	// Initial delay
	if r.config.InitialDelay > 0 {
		select {
		case <-r.ctx.Done():
			return ErrSSEReplayStopped
		case <-time.After(time.Duration(r.config.InitialDelay) * time.Millisecond):
		}
	}

	var lastRelativeMs int64 = 0

	for i := range events {
		event := &events[i]

		// Check if stopped
		select {
		case <-r.ctx.Done():
			return ErrSSEReplayStopped
		default:
		}

		// Calculate sleep time based on relative timing
		if i > 0 {
			deltaMs := event.RelativeMs - lastRelativeMs
			if deltaMs > 0 {
				// Apply timing scale
				sleepDuration := time.Duration(float64(deltaMs)/r.config.TimingScale) * time.Millisecond
				select {
				case <-r.ctx.Done():
					return ErrSSEReplayStopped
				case <-time.After(sleepDuration):
				}
			}
		}
		lastRelativeMs = event.RelativeMs

		// Send the event
		if err := r.sendEvent(event); err != nil {
			return err
		}

		r.mu.Lock()
		r.currentEvent = i + 1
		r.eventsSent++
		r.mu.Unlock()
	}

	return nil
}

// playTriggered replays event-by-event on command.
func (r *SSEReplayer) playTriggered() error {
	events := r.recording.SSE.Events
	if len(events) == 0 {
		return nil
	}

	r.mu.Lock()
	r.status = recording.ReplayStatusWaiting
	r.mu.Unlock()

	eventIdx := 0
	for eventIdx < len(events) {
		// Wait for advance command
		select {
		case <-r.ctx.Done():
			return ErrSSEReplayStopped
		case count := <-r.advanceChan:
			if count <= 0 {
				continue
			}

			r.mu.Lock()
			r.status = recording.ReplayStatusPlaying
			r.mu.Unlock()

			// Send the requested number of events
			for j := 0; j < count && eventIdx < len(events); j++ {
				event := &events[eventIdx]
				if err := r.sendEvent(event); err != nil {
					return err
				}

				r.mu.Lock()
				r.currentEvent = eventIdx + 1
				r.eventsSent++
				r.mu.Unlock()
				eventIdx++
			}

			r.mu.Lock()
			if eventIdx < len(events) {
				r.status = recording.ReplayStatusWaiting
			}
			r.mu.Unlock()
		}
	}

	return nil
}

// Advance sends the next N events (for triggered mode).
// Returns the actual number of events that will be sent.
func (r *SSEReplayer) Advance(count int) (int, error) {
	r.mu.RLock()
	if r.config.Mode != recording.ReplayModeTriggered {
		r.mu.RUnlock()
		return 0, ErrSSETriggeredModeOnly
	}
	if !r.started {
		r.mu.RUnlock()
		return 0, ErrSSEReplayNotStarted
	}
	r.mu.RUnlock()

	// Calculate how many events can actually be sent
	events := r.recording.SSE.Events
	r.mu.RLock()
	remaining := len(events) - r.currentEvent
	r.mu.RUnlock()

	if remaining <= 0 {
		return 0, nil
	}

	toSend := count
	if toSend > remaining {
		toSend = remaining
	}

	// Send the advance command
	select {
	case r.advanceChan <- toSend:
		return toSend, nil
	case <-r.ctx.Done():
		return 0, ErrSSEReplayStopped
	}
}

// Stop stops the replay.
func (r *SSEReplayer) Stop() {
	r.mu.Lock()
	if r.status == recording.ReplayStatusPlaying || r.status == recording.ReplayStatusWaiting {
		r.status = recording.ReplayStatusAborted
	}
	r.mu.Unlock()

	r.cancel()
}

// Progress returns current replay progress.
func (r *SSEReplayer) Progress() (currentEvent, totalEvents, eventsSent int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.currentEvent, len(r.recording.SSE.Events), r.eventsSent
}

// Status returns the current replay status.
func (r *SSEReplayer) Status() recording.ReplayStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// sendEvent sends a single SSE event to the client.
func (r *SSEReplayer) sendEvent(event *recording.SSEEvent) error {
	// Convert recording.SSEEvent to SSEEventDef
	eventDef := &SSEEventDef{
		Type: event.EventType,
		Data: event.Data,
		ID:   event.ID,
	}

	if event.Retry != nil {
		eventDef.Retry = *event.Retry
	}

	// Format the event
	formatted, err := r.encoder.FormatEvent(eventDef)
	if err != nil {
		return err
	}

	// Write to response
	if _, err := r.writer.Write([]byte(formatted)); err != nil {
		return err
	}

	// Flush to send immediately
	r.flusher.Flush()

	return nil
}

// SSEReplayProgress contains information about replay progress.
type SSEReplayProgress struct {
	CurrentEvent int                    `json:"currentEvent"`
	TotalEvents  int                    `json:"totalEvents"`
	EventsSent   int                    `json:"eventsSent"`
	Status       recording.ReplayStatus `json:"status"`
	Elapsed      time.Duration          `json:"elapsed"`
}

// GetProgress returns detailed replay progress.
func (r *SSEReplayer) GetProgress() SSEReplayProgress {
	r.mu.RLock()
	defer r.mu.RUnlock()

	elapsed := time.Duration(0)
	if r.started {
		elapsed = time.Since(r.startTime)
	}

	return SSEReplayProgress{
		CurrentEvent: r.currentEvent,
		TotalEvents:  len(r.recording.SSE.Events),
		EventsSent:   r.eventsSent,
		Status:       r.status,
		Elapsed:      elapsed,
	}
}

// Context returns the replay context.
func (r *SSEReplayer) Context() context.Context {
	return r.ctx
}
