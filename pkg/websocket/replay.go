// Package websocket provides WebSocket replay functionality.
package websocket

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// Replay errors.
var (
	// ErrReplayNotStarted indicates the replay hasn't been started.
	ErrReplayNotStarted = errors.New("replay not started")
	// ErrReplayAlreadyStarted indicates the replay is already running.
	ErrReplayAlreadyStarted = errors.New("replay already started")
	// ErrReplayStopped indicates the replay was stopped.
	ErrReplayStopped = errors.New("replay stopped")
	// ErrInvalidRecording indicates the recording is invalid for replay.
	ErrInvalidRecording = errors.New("invalid recording for replay")
	// ErrNoFramesToReplay indicates there are no frames to replay.
	ErrNoFramesToReplay = errors.New("no frames to replay")
	// ErrInvalidReplayMode indicates an invalid replay mode.
	ErrInvalidReplayMode = errors.New("invalid replay mode")
	// ErrTriggeredModeOnly indicates the operation is only valid in triggered mode.
	ErrTriggeredModeOnly = errors.New("operation only valid in triggered mode")
	// ErrSynchronizedModeOnly indicates the operation is only valid in synchronized mode.
	ErrSynchronizedModeOnly = errors.New("operation only valid in synchronized mode")
)

// ReplayConfig configures how a recording is replayed.
type ReplayConfig struct {
	// Mode is the replay mode: "pure", "synchronized", or "triggered".
	Mode recording.ReplayMode `json:"mode"`

	// TimingScale adjusts the replay speed (1.0 = original speed, 0.5 = half speed, 2.0 = double speed).
	// Only used in Pure mode.
	TimingScale float64 `json:"timingScale,omitempty"`

	// SkipClientFrames skips client-to-server frames during replay.
	// If false, client frames are included in synchronization matching.
	SkipClientFrames bool `json:"skipClientFrames,omitempty"`

	// Timeout is the maximum time to wait for a client message in synchronized mode.
	Timeout time.Duration `json:"timeout,omitempty"`
}

// DefaultReplayConfig returns the default replay configuration.
func DefaultReplayConfig() ReplayConfig {
	return ReplayConfig{
		Mode:             recording.ReplayModePure,
		TimingScale:      1.0,
		SkipClientFrames: true,
		Timeout:          30 * time.Second,
	}
}

// WebSocketReplayer replays a WebSocket recording through a connection.
type WebSocketReplayer struct {
	recording *recording.StreamRecording
	conn      *Connection
	config    ReplayConfig

	currentFrame int
	framesSent   int
	startTime    time.Time

	ctx    context.Context
	cancel context.CancelFunc

	// For triggered mode
	advanceChan chan int // number of frames to send

	// For synchronized mode
	expectedMsg chan []byte

	mu      sync.RWMutex
	started bool
	status  recording.ReplayStatus
}

// NewWebSocketReplayer creates a replayer for a WebSocket connection.
func NewWebSocketReplayer(rec *recording.StreamRecording, conn *Connection, config ReplayConfig) (*WebSocketReplayer, error) {
	if rec == nil {
		return nil, ErrInvalidRecording
	}

	if rec.Protocol != recording.ProtocolWebSocket {
		return nil, ErrInvalidRecording
	}

	if rec.WebSocket == nil || len(rec.WebSocket.Frames) == 0 {
		return nil, ErrNoFramesToReplay
	}

	if !config.Mode.IsValid() {
		return nil, ErrInvalidReplayMode
	}

	// Set defaults
	if config.TimingScale <= 0 {
		config.TimingScale = 1.0
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())

	r := &WebSocketReplayer{
		recording:    rec,
		conn:         conn,
		config:       config,
		currentFrame: 0,
		framesSent:   0,
		ctx:          ctx,
		cancel:       cancel,
		status:       recording.ReplayStatusPending,
	}

	// Initialize mode-specific channels
	if config.Mode == recording.ReplayModeTriggered {
		r.advanceChan = make(chan int, 10)
	}

	if config.Mode == recording.ReplayModeSynchronized {
		r.expectedMsg = make(chan []byte, 1)
	}

	return r, nil
}

// Start begins replaying the recording.
// This runs in a goroutine and returns immediately.
func (r *WebSocketReplayer) Start() error {
	r.mu.Lock()
	if r.started {
		r.mu.Unlock()
		return ErrReplayAlreadyStarted
	}
	r.started = true
	r.startTime = time.Now()
	r.status = recording.ReplayStatusPlaying
	r.mu.Unlock()

	go func() {
		var err error
		switch r.config.Mode {
		case recording.ReplayModePure:
			err = r.PlayPure()
		case recording.ReplayModeSynchronized:
			err = r.PlaySynchronized()
		case recording.ReplayModeTriggered:
			err = r.PlayTriggered()
		}

		r.mu.Lock()
		if err != nil && !errors.Is(err, ErrReplayStopped) && !errors.Is(err, context.Canceled) {
			r.status = recording.ReplayStatusAborted
		} else if r.status == recording.ReplayStatusPlaying {
			r.status = recording.ReplayStatusComplete
		}
		r.mu.Unlock()
	}()

	return nil
}

// PlayPure replays with original timing.
func (r *WebSocketReplayer) PlayPure() error {
	frames := r.getServerToClientFrames()
	if len(frames) == 0 {
		return nil
	}

	var lastRelativeMs int64 = 0

	for i, frame := range frames {
		// Check if stopped
		select {
		case <-r.ctx.Done():
			return ErrReplayStopped
		default:
		}

		// Calculate sleep time based on relative timing
		if i > 0 {
			deltaMs := frame.RelativeMs - lastRelativeMs
			if deltaMs > 0 {
				// Apply timing scale
				sleepDuration := time.Duration(float64(deltaMs)/r.config.TimingScale) * time.Millisecond
				select {
				case <-r.ctx.Done():
					return ErrReplayStopped
				case <-time.After(sleepDuration):
				}
			}
		}
		lastRelativeMs = frame.RelativeMs

		// Send the frame
		if err := r.sendFrame(frame); err != nil {
			return err
		}

		r.mu.Lock()
		r.currentFrame = i + 1
		r.framesSent++
		r.mu.Unlock()
	}

	return nil
}

// PlaySynchronized replays waiting for client messages.
func (r *WebSocketReplayer) PlaySynchronized() error {
	frames := r.recording.WebSocket.Frames
	if len(frames) == 0 {
		return nil
	}

	r.mu.Lock()
	r.status = recording.ReplayStatusWaiting
	r.mu.Unlock()

	i := 0
	for i < len(frames) {
		frame := frames[i]

		// Check if stopped
		select {
		case <-r.ctx.Done():
			return ErrReplayStopped
		default:
		}

		if frame.Direction == recording.DirectionClientToServer {
			// Wait for client message that matches this pattern
			r.mu.Lock()
			r.status = recording.ReplayStatusWaiting
			r.mu.Unlock()

			matched, err := r.waitForClientMessage(frame)
			if err != nil {
				return err
			}

			if !matched {
				// Continue waiting for the right message
				continue
			}

			r.mu.Lock()
			r.status = recording.ReplayStatusPlaying
			r.currentFrame = i + 1
			r.mu.Unlock()
			i++
		} else {
			// Server-to-client frame: send it
			if err := r.sendFrame(frame); err != nil {
				return err
			}

			r.mu.Lock()
			r.currentFrame = i + 1
			r.framesSent++
			r.mu.Unlock()
			i++
		}
	}

	return nil
}

// waitForClientMessage waits for a client message that matches the expected frame.
func (r *WebSocketReplayer) waitForClientMessage(expectedFrame recording.WebSocketFrame) (bool, error) {
	select {
	case <-r.ctx.Done():
		return false, ErrReplayStopped
	case <-time.After(r.config.Timeout):
		return false, ErrScenarioTimeout
	case data := <-r.expectedMsg:
		// Check if the message matches the expected frame
		expectedData, err := expectedFrame.GetData()
		if err != nil {
			return false, err
		}

		// Simple matching: check if the received data matches expected
		// For more sophisticated matching, use the Matcher infrastructure
		return r.messagesMatch(data, expectedData, expectedFrame.MessageType), nil
	}
}

// messagesMatch checks if two messages match based on the message type.
func (r *WebSocketReplayer) messagesMatch(received, expected []byte, msgType recording.MessageType) bool {
	// For text messages, try JSON-aware matching first
	if msgType == recording.MessageTypeText {
		return string(received) == string(expected)
	}

	// For binary messages, exact byte match
	if len(received) != len(expected) {
		return false
	}
	for i := range received {
		if received[i] != expected[i] {
			return false
		}
	}
	return true
}

// PlayTriggered replays frame-by-frame on command.
func (r *WebSocketReplayer) PlayTriggered() error {
	frames := r.getServerToClientFrames()
	if len(frames) == 0 {
		return nil
	}

	r.mu.Lock()
	r.status = recording.ReplayStatusWaiting
	r.mu.Unlock()

	frameIdx := 0
	for frameIdx < len(frames) {
		// Wait for advance command
		select {
		case <-r.ctx.Done():
			return ErrReplayStopped
		case count := <-r.advanceChan:
			if count <= 0 {
				continue
			}

			r.mu.Lock()
			r.status = recording.ReplayStatusPlaying
			r.mu.Unlock()

			// Send the requested number of frames
			for j := 0; j < count && frameIdx < len(frames); j++ {
				frame := frames[frameIdx]
				if err := r.sendFrame(frame); err != nil {
					return err
				}

				r.mu.Lock()
				r.currentFrame = frameIdx + 1
				r.framesSent++
				r.mu.Unlock()
				frameIdx++
			}

			r.mu.Lock()
			if frameIdx < len(frames) {
				r.status = recording.ReplayStatusWaiting
			}
			r.mu.Unlock()
		}
	}

	return nil
}

// Advance sends the next N frames (for triggered mode).
// Returns the actual number of frames that will be sent.
func (r *WebSocketReplayer) Advance(count int) (int, error) {
	r.mu.RLock()
	if r.config.Mode != recording.ReplayModeTriggered {
		r.mu.RUnlock()
		return 0, ErrTriggeredModeOnly
	}
	if !r.started {
		r.mu.RUnlock()
		return 0, ErrReplayNotStarted
	}
	r.mu.RUnlock()

	// Calculate how many frames can actually be sent
	frames := r.getServerToClientFrames()
	r.mu.RLock()
	remaining := len(frames) - r.currentFrame
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
		return 0, ErrReplayStopped
	}
}

// OnClientMessage handles incoming client messages (for synchronized mode).
func (r *WebSocketReplayer) OnClientMessage(data []byte) {
	r.mu.RLock()
	mode := r.config.Mode
	started := r.started
	r.mu.RUnlock()

	if mode != recording.ReplayModeSynchronized || !started {
		return
	}

	// Non-blocking send to avoid deadlock
	select {
	case r.expectedMsg <- data:
	default:
		// Channel full, drop the message
	}
}

// Stop stops the replay.
func (r *WebSocketReplayer) Stop() {
	r.mu.Lock()
	if r.status == recording.ReplayStatusPlaying || r.status == recording.ReplayStatusWaiting {
		r.status = recording.ReplayStatusAborted
	}
	r.mu.Unlock()

	r.cancel()
}

// Progress returns current replay progress.
func (r *WebSocketReplayer) Progress() (currentFrame, totalFrames, framesSent int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	frames := r.getServerToClientFramesLocked()
	return r.currentFrame, len(frames), r.framesSent
}

// Status returns the current replay status.
func (r *WebSocketReplayer) Status() recording.ReplayStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.status
}

// sendFrame sends a single frame to the connection.
func (r *WebSocketReplayer) sendFrame(frame recording.WebSocketFrame) error {
	// Get the frame data
	data, err := frame.GetData()
	if err != nil {
		return err
	}

	// Convert recording.MessageType to websocket.MessageType
	msgType := convertRecordingMessageType(frame.MessageType)

	// Handle special frame types
	switch frame.MessageType {
	case recording.MessageTypeClose:
		code := CloseNormalClosure
		reason := ""
		if frame.CloseCode != nil {
			code = CloseCode(*frame.CloseCode)
		}
		if frame.CloseReason != nil {
			reason = *frame.CloseReason
		}
		return r.conn.Close(code, reason)

	case recording.MessageTypePing:
		return r.conn.Ping(r.ctx)

	case recording.MessageTypePong:
		// Pong is typically handled automatically by the library
		return nil

	default:
		return r.conn.Send(msgType, data)
	}
}

// getServerToClientFrames returns only server-to-client frames from the recording.
func (r *WebSocketReplayer) getServerToClientFrames() []recording.WebSocketFrame {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.getServerToClientFramesLocked()
}

// getServerToClientFramesLocked returns server-to-client frames (caller must hold lock).
func (r *WebSocketReplayer) getServerToClientFramesLocked() []recording.WebSocketFrame {
	if r.recording.WebSocket == nil {
		return nil
	}

	var frames []recording.WebSocketFrame
	for _, frame := range r.recording.WebSocket.Frames {
		if frame.Direction == recording.DirectionServerToClient {
			frames = append(frames, frame)
		}
	}
	return frames
}

// convertRecordingMessageType converts recording.MessageType to websocket.MessageType.
func convertRecordingMessageType(msgType recording.MessageType) MessageType {
	switch msgType {
	case recording.MessageTypeText:
		return MessageText
	case recording.MessageTypeBinary:
		return MessageBinary
	default:
		return MessageText
	}
}

// ReplayProgress contains information about replay progress.
type ReplayProgress struct {
	CurrentFrame int                    `json:"currentFrame"`
	TotalFrames  int                    `json:"totalFrames"`
	FramesSent   int                    `json:"framesSent"`
	Status       recording.ReplayStatus `json:"status"`
	Elapsed      time.Duration          `json:"elapsed"`
}

// GetProgress returns detailed replay progress.
func (r *WebSocketReplayer) GetProgress() ReplayProgress {
	r.mu.RLock()
	defer r.mu.RUnlock()

	frames := r.getServerToClientFramesLocked()
	elapsed := time.Duration(0)
	if r.started {
		elapsed = time.Since(r.startTime)
	}

	return ReplayProgress{
		CurrentFrame: r.currentFrame,
		TotalFrames:  len(frames),
		FramesSent:   r.framesSent,
		Status:       r.status,
		Elapsed:      elapsed,
	}
}
