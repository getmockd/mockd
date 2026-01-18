// Package recording provides replay functionality for stream recordings.
package recording

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Replay errors
var (
	ErrReplayNotFound      = errors.New("replay session not found")
	ErrReplayAlreadyActive = errors.New("replay session already active")
	ErrReplayNotPaused     = errors.New("replay session not paused")
	ErrReplayNotPlaying    = errors.New("replay session not playing")
	ErrReplayComplete      = errors.New("replay session already complete")
	ErrInvalidReplayMode   = errors.New("invalid replay mode")
	ErrAdvanceNotAllowed   = errors.New("advance only allowed in triggered mode")
	ErrMatchTimeout        = errors.New("synchronized mode match timeout")
)

// ReplayConfig configures how a recording is replayed.
type ReplayConfig struct {
	RecordingID string     `json:"recordingId"`
	Mode        ReplayMode `json:"mode"`
	TimingScale float64    `json:"timingScale,omitempty"` // 1.0 = original, 0.5 = 2x speed

	// Synchronized mode
	StrictMatching bool `json:"strictMatching,omitempty"`
	Timeout        int  `json:"timeout,omitempty"` // ms, default 30000

	// Triggered mode
	AutoAdvanceOnConnect bool `json:"autoAdvanceOnConnect,omitempty"`
	InBandTrigger        bool `json:"inBandTrigger,omitempty"`
}

// ReplaySession represents an active replay.
type ReplaySession struct {
	ID           string       `json:"id"`
	RecordingID  string       `json:"recordingId"`
	Config       ReplayConfig `json:"config"`
	Status       ReplayStatus `json:"status"`
	CurrentFrame int          `json:"currentFrame"`
	TotalFrames  int          `json:"totalFrames"`
	FramesSent   int          `json:"framesSent"`
	StartedAt    time.Time    `json:"startedAt"`
	ElapsedMs    int64        `json:"elapsedMs"`

	recording *StreamRecording
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex

	// For triggered mode
	advanceChan chan AdvanceRequest

	// For synchronized mode
	matchChan chan []byte

	// For pause/resume
	pauseChan  chan struct{}
	resumeChan chan struct{}

	// Frame sender callback
	sendFrame func(frame interface{}) error

	// Timing tracking
	lastFrameTime time.Time
}

// AdvanceRequest for triggered replay.
type AdvanceRequest struct {
	Count    int    `json:"count,omitempty"`
	Until    string `json:"until,omitempty"`
	response chan AdvanceResponse
}

// AdvanceResponse from advancement.
type AdvanceResponse struct {
	FramesSent   int          `json:"framesSent"`
	CurrentFrame int          `json:"currentFrame"`
	TotalFrames  int          `json:"totalFrames"`
	Status       ReplayStatus `json:"status"`
	Complete     bool         `json:"complete"`
	Error        string       `json:"error,omitempty"`
}

// ReplayController manages replay sessions.
type ReplayController struct {
	store    *FileStore
	sessions map[string]*ReplaySession
	mu       sync.RWMutex
}

// NewReplayController creates a new replay controller.
func NewReplayController(store *FileStore) *ReplayController {
	return &ReplayController{
		store:    store,
		sessions: make(map[string]*ReplaySession),
	}
}

// StartReplay starts a new replay session.
func (c *ReplayController) StartReplay(config ReplayConfig) (*ReplaySession, error) {
	// Validate config
	if !config.Mode.IsValid() {
		return nil, ErrInvalidReplayMode
	}

	// Load recording
	recording, err := c.store.Get(config.RecordingID)
	if err != nil {
		return nil, err
	}

	// Apply defaults
	if config.TimingScale == 0 {
		config.TimingScale = 1.0
	}
	if config.Timeout == 0 {
		config.Timeout = 30000
	}

	// Count total frames
	totalFrames := c.countFrames(recording)

	// Create session
	ctx, cancel := context.WithCancel(context.Background())
	session := &ReplaySession{
		ID:           NewULID(),
		RecordingID:  config.RecordingID,
		Config:       config,
		Status:       ReplayStatusPending,
		CurrentFrame: 0,
		TotalFrames:  totalFrames,
		FramesSent:   0,
		StartedAt:    time.Now(),
		recording:    recording,
		ctx:          ctx,
		cancel:       cancel,
		advanceChan:  make(chan AdvanceRequest, 10),
		matchChan:    make(chan []byte, 10),
		pauseChan:    make(chan struct{}),
		resumeChan:   make(chan struct{}),
	}

	c.mu.Lock()
	c.sessions[session.ID] = session
	c.mu.Unlock()

	return session, nil
}

// countFrames returns the total number of frames in a recording.
func (c *ReplayController) countFrames(recording *StreamRecording) int {
	switch recording.Protocol {
	case ProtocolWebSocket:
		if recording.WebSocket != nil {
			return len(recording.WebSocket.Frames)
		}
	case ProtocolSSE:
		if recording.SSE != nil {
			return len(recording.SSE.Events)
		}
	}
	return 0
}

// GetSession returns a replay session by ID.
func (c *ReplayController) GetSession(id string) (*ReplaySession, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	session, ok := c.sessions[id]
	return session, ok
}

// StopReplay stops a replay session.
func (c *ReplayController) StopReplay(id string) error {
	c.mu.Lock()
	session, ok := c.sessions[id]
	if !ok {
		c.mu.Unlock()
		return ErrReplayNotFound
	}
	delete(c.sessions, id)
	c.mu.Unlock()

	session.mu.Lock()
	defer session.mu.Unlock()

	session.Status = ReplayStatusAborted
	session.cancel()

	return nil
}

// Advance advances a triggered replay session.
func (c *ReplayController) Advance(id string, req AdvanceRequest) (*AdvanceResponse, error) {
	session, ok := c.GetSession(id)
	if !ok {
		return nil, ErrReplayNotFound
	}

	session.mu.Lock()
	if session.Config.Mode != ReplayModeTriggered {
		session.mu.Unlock()
		return nil, ErrAdvanceNotAllowed
	}

	if session.Status == ReplayStatusComplete {
		session.mu.Unlock()
		return &AdvanceResponse{
			FramesSent:   0,
			CurrentFrame: session.CurrentFrame,
			TotalFrames:  session.TotalFrames,
			Status:       session.Status,
			Complete:     true,
		}, nil
	}
	session.mu.Unlock()

	// Create response channel for synchronous response
	responseChan := make(chan AdvanceResponse, 1)
	req.response = responseChan

	// Send advance request
	select {
	case session.advanceChan <- req:
	case <-session.ctx.Done():
		return nil, ErrReplayComplete
	}

	// Wait for response
	select {
	case resp := <-responseChan:
		return &resp, nil
	case <-session.ctx.Done():
		return nil, ErrReplayComplete
	case <-time.After(time.Duration(session.Config.Timeout) * time.Millisecond):
		return nil, ErrMatchTimeout
	}
}

// PauseReplay pauses a replay session.
func (c *ReplayController) PauseReplay(id string) error {
	session, ok := c.GetSession(id)
	if !ok {
		return ErrReplayNotFound
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status != ReplayStatusPlaying {
		return ErrReplayNotPlaying
	}

	session.Status = ReplayStatusPaused

	select {
	case session.pauseChan <- struct{}{}:
	default:
	}

	return nil
}

// ResumeReplay resumes a paused session.
func (c *ReplayController) ResumeReplay(id string) error {
	session, ok := c.GetSession(id)
	if !ok {
		return ErrReplayNotFound
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	if session.Status != ReplayStatusPaused {
		return ErrReplayNotPaused
	}

	session.Status = ReplayStatusPlaying

	select {
	case session.resumeChan <- struct{}{}:
	default:
	}

	return nil
}

// ListSessions returns all active replay sessions.
func (c *ReplayController) ListSessions() []*ReplaySession {
	c.mu.RLock()
	defer c.mu.RUnlock()

	sessions := make([]*ReplaySession, 0, len(c.sessions))
	for _, session := range c.sessions {
		session.mu.Lock()
		session.ElapsedMs = time.Since(session.StartedAt).Milliseconds()
		session.mu.Unlock()
		sessions = append(sessions, session)
	}
	return sessions
}

// SetFrameSender sets the callback for sending frames during replay.
func (s *ReplaySession) SetFrameSender(sender func(frame interface{}) error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sendFrame = sender
}

// Run starts the replay playback loop.
func (s *ReplaySession) Run() error {
	s.mu.Lock()
	if s.Status != ReplayStatusPending {
		s.mu.Unlock()
		return ErrReplayAlreadyActive
	}
	s.Status = ReplayStatusPlaying
	s.mu.Unlock()

	switch s.Config.Mode {
	case ReplayModePure:
		return s.runPureMode()
	case ReplayModeSynchronized:
		return s.runSynchronizedMode()
	case ReplayModeTriggered:
		return s.runTriggeredMode()
	default:
		return ErrInvalidReplayMode
	}
}

// runPureMode replays with original timing.
func (s *ReplaySession) runPureMode() error {
	frames := s.getServerFrames()
	if len(frames) == 0 {
		s.markComplete()
		return nil
	}

	var lastRelativeMs int64

	for i, frame := range frames {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-s.pauseChan:
			// Wait for resume
			select {
			case <-s.resumeChan:
			case <-s.ctx.Done():
				return s.ctx.Err()
			}
		default:
		}

		// Calculate delay based on timing
		relativeMs := s.getFrameRelativeMs(frame)
		if i > 0 && relativeMs > lastRelativeMs {
			delay := time.Duration(float64(relativeMs-lastRelativeMs)/s.Config.TimingScale) * time.Millisecond
			select {
			case <-time.After(delay):
			case <-s.pauseChan:
				select {
				case <-s.resumeChan:
				case <-s.ctx.Done():
					return s.ctx.Err()
				}
			case <-s.ctx.Done():
				return s.ctx.Err()
			}
		}
		lastRelativeMs = relativeMs

		// Send frame
		if err := s.sendFrameData(frame); err != nil {
			return err
		}

		s.mu.Lock()
		s.CurrentFrame = i + 1
		s.FramesSent++
		s.ElapsedMs = time.Since(s.StartedAt).Milliseconds()
		s.mu.Unlock()
	}

	s.markComplete()
	return nil
}

// runSynchronizedMode waits for client messages before server responses.
func (s *ReplaySession) runSynchronizedMode() error {
	s.mu.Lock()
	s.Status = ReplayStatusWaiting
	s.mu.Unlock()

	allFrames := s.getAllFrames()
	timeout := time.Duration(s.Config.Timeout) * time.Millisecond

	for i, frame := range allFrames {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		default:
		}

		dir := s.getFrameDirection(frame)

		if dir == DirectionClientToServer {
			// Wait for matching client message
			s.mu.Lock()
			s.Status = ReplayStatusWaiting
			s.mu.Unlock()

			select {
			case msg := <-s.matchChan:
				if s.Config.StrictMatching {
					expectedData := s.getFrameData(frame)
					if string(msg) != string(expectedData) {
						// Message doesn't match, could handle mismatch here
						// For now, continue
					}
				}
			case <-time.After(timeout):
				return ErrMatchTimeout
			case <-s.ctx.Done():
				return s.ctx.Err()
			}
		} else {
			// Server message - send it
			s.mu.Lock()
			s.Status = ReplayStatusPlaying
			s.mu.Unlock()

			if err := s.sendFrameData(frame); err != nil {
				return err
			}
			s.mu.Lock()
			s.FramesSent++
			s.mu.Unlock()
		}

		s.mu.Lock()
		s.CurrentFrame = i + 1
		s.ElapsedMs = time.Since(s.StartedAt).Milliseconds()
		s.mu.Unlock()
	}

	s.markComplete()
	return nil
}

// runTriggeredMode waits for explicit advance commands.
func (s *ReplaySession) runTriggeredMode() error {
	serverFrames := s.getServerFrames()

	// Auto-advance on connect if configured
	if s.Config.AutoAdvanceOnConnect && len(serverFrames) > 0 {
		if err := s.sendFrameData(serverFrames[0]); err != nil {
			return err
		}
		s.mu.Lock()
		s.CurrentFrame = 1
		s.FramesSent = 1
		s.mu.Unlock()
	}

	s.mu.Lock()
	s.Status = ReplayStatusWaiting
	s.mu.Unlock()

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()

		case req := <-s.advanceChan:
			resp := s.handleAdvance(req, serverFrames)
			if req.response != nil {
				req.response <- resp
			}
			if resp.Complete {
				return nil
			}
		}
	}
}

// handleAdvance processes an advance request.
func (s *ReplaySession) handleAdvance(req AdvanceRequest, serverFrames []interface{}) AdvanceResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.CurrentFrame >= len(serverFrames) {
		s.Status = ReplayStatusComplete
		return AdvanceResponse{
			FramesSent:   0,
			CurrentFrame: s.CurrentFrame,
			TotalFrames:  s.TotalFrames,
			Status:       s.Status,
			Complete:     true,
		}
	}

	// Determine how many frames to send
	count := req.Count
	if count <= 0 {
		count = 1
	}

	startFrame := s.CurrentFrame
	endFrame := startFrame + count
	if endFrame > len(serverFrames) {
		endFrame = len(serverFrames)
	}

	// Handle "until" condition
	if req.Until != "" {
		for i := startFrame; i < len(serverFrames); i++ {
			frameData := s.getFrameData(serverFrames[i])
			if string(frameData) == req.Until {
				endFrame = i + 1
				break
			}
		}
	}

	// Send frames
	framesSent := 0
	for i := startFrame; i < endFrame; i++ {
		if s.sendFrame != nil {
			if err := s.sendFrame(serverFrames[i]); err != nil {
				return AdvanceResponse{
					FramesSent:   framesSent,
					CurrentFrame: s.CurrentFrame,
					TotalFrames:  s.TotalFrames,
					Status:       s.Status,
					Complete:     false,
					Error:        err.Error(),
				}
			}
		}
		s.CurrentFrame = i + 1
		s.FramesSent++
		framesSent++
	}

	s.ElapsedMs = time.Since(s.StartedAt).Milliseconds()

	complete := s.CurrentFrame >= len(serverFrames)
	if complete {
		s.Status = ReplayStatusComplete
	}

	return AdvanceResponse{
		FramesSent:   framesSent,
		CurrentFrame: s.CurrentFrame,
		TotalFrames:  s.TotalFrames,
		Status:       s.Status,
		Complete:     complete,
	}
}

// ReceiveMessage handles incoming client messages (for synchronized mode).
func (s *ReplaySession) ReceiveMessage(data []byte) {
	select {
	case s.matchChan <- data:
	default:
		// Channel full, drop message
	}
}

// getServerFrames returns only server-to-client frames.
func (s *ReplaySession) getServerFrames() []interface{} {
	var frames []interface{}

	switch s.recording.Protocol {
	case ProtocolWebSocket:
		if s.recording.WebSocket != nil {
			for i := range s.recording.WebSocket.Frames {
				f := &s.recording.WebSocket.Frames[i]
				if f.Direction == DirectionServerToClient {
					frames = append(frames, f)
				}
			}
		}
	case ProtocolSSE:
		if s.recording.SSE != nil {
			for i := range s.recording.SSE.Events {
				frames = append(frames, &s.recording.SSE.Events[i])
			}
		}
	}

	return frames
}

// getAllFrames returns all frames in order.
func (s *ReplaySession) getAllFrames() []interface{} {
	var frames []interface{}

	switch s.recording.Protocol {
	case ProtocolWebSocket:
		if s.recording.WebSocket != nil {
			for i := range s.recording.WebSocket.Frames {
				frames = append(frames, &s.recording.WebSocket.Frames[i])
			}
		}
	case ProtocolSSE:
		if s.recording.SSE != nil {
			for i := range s.recording.SSE.Events {
				frames = append(frames, &s.recording.SSE.Events[i])
			}
		}
	}

	return frames
}

// getFrameDirection returns the direction of a frame.
func (s *ReplaySession) getFrameDirection(frame interface{}) Direction {
	switch f := frame.(type) {
	case *WebSocketFrame:
		return f.Direction
	case *SSEEvent:
		return DirectionServerToClient // SSE is always server-to-client
	}
	return DirectionServerToClient
}

// getFrameRelativeMs returns the relative timestamp of a frame.
func (s *ReplaySession) getFrameRelativeMs(frame interface{}) int64 {
	switch f := frame.(type) {
	case *WebSocketFrame:
		return f.RelativeMs
	case *SSEEvent:
		return f.RelativeMs
	}
	return 0
}

// getFrameData returns the data content of a frame.
func (s *ReplaySession) getFrameData(frame interface{}) []byte {
	switch f := frame.(type) {
	case *WebSocketFrame:
		data, _ := f.GetData()
		return data
	case *SSEEvent:
		return []byte(f.Data)
	}
	return nil
}

// sendFrameData sends a frame using the configured sender.
func (s *ReplaySession) sendFrameData(frame interface{}) error {
	if s.sendFrame != nil {
		return s.sendFrame(frame)
	}
	return nil
}

// markComplete marks the session as complete.
func (s *ReplaySession) markComplete() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = ReplayStatusComplete
	s.ElapsedMs = time.Since(s.StartedAt).Milliseconds()
}

// GetRecording returns the underlying recording.
func (s *ReplaySession) GetRecording() *StreamRecording {
	return s.recording
}

// Context returns the session's context.
func (s *ReplaySession) Context() context.Context {
	return s.ctx
}

// Cancel cancels the session.
func (s *ReplaySession) Cancel() {
	s.cancel()
}

// CleanupSession removes a completed session from the controller.
func (c *ReplayController) CleanupSession(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.sessions, id)
}

// GetStore returns the underlying file store.
func (c *ReplayController) GetStore() *FileStore {
	return c.store
}
