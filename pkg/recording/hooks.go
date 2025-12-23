// Package recording provides hook interfaces for stream recording events.
package recording

import (
	"fmt"
	"sync"
)

// RecordingHook is the base interface for all recording hooks.
// Implementations receive recording events and can process them as needed.
type RecordingHook interface {
	// OnFrame is called when a new frame/event is recorded.
	// The frame type depends on the protocol (WebSocketFrame for WS, SSEEvent for SSE).
	OnFrame(frame any) error

	// OnComplete is called when the recording is successfully completed.
	OnComplete() error

	// OnError is called when an error occurs during recording.
	OnError(err error)

	// ID returns a unique identifier for this hook instance.
	ID() string
}

// WebSocketRecordingHook extends RecordingHook for WebSocket-specific events.
type WebSocketRecordingHook interface {
	RecordingHook

	// OnConnect is called when the WebSocket connection is established.
	OnConnect(subprotocol string)

	// OnClose is called when the WebSocket connection is closed.
	OnClose(code int, reason string)
}

// SSERecordingHook extends RecordingHook for SSE-specific events.
type SSERecordingHook interface {
	RecordingHook

	// OnStreamStart is called when the SSE stream begins.
	OnStreamStart()

	// OnStreamEnd is called when the SSE stream ends.
	OnStreamEnd()
}

// FileStoreWebSocketHook is a WebSocketRecordingHook implementation that uses FileStore.
type FileStoreWebSocketHook struct {
	mu        sync.Mutex
	store     *FileStore
	session   *StreamRecordingSession
	metadata  RecordingMetadata
	id        string
	completed bool
	errored   bool
}

// NewFileStoreWebSocketHook creates a new WebSocket recording hook backed by FileStore.
func NewFileStoreWebSocketHook(store *FileStore, metadata RecordingMetadata) (*FileStoreWebSocketHook, error) {
	if store == nil {
		return nil, fmt.Errorf("store cannot be nil")
	}

	// Start a new recording session
	session, err := store.StartRecording(ProtocolWebSocket, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to start recording session: %w", err)
	}

	return &FileStoreWebSocketHook{
		store:    store,
		session:  session,
		metadata: metadata,
		id:       session.recording.ID,
	}, nil
}

// ID returns the recording ID for this hook.
func (h *FileStoreWebSocketHook) ID() string {
	return h.id
}

// OnConnect is called when the WebSocket connection is established.
func (h *FileStoreWebSocketHook) OnConnect(subprotocol string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.session != nil && h.session.recording != nil {
		h.session.recording.Metadata.Subprotocol = subprotocol
	}
}

// OnFrame is called when a WebSocket frame is recorded.
func (h *FileStoreWebSocketHook) OnFrame(frame any) error {
	h.mu.Lock()
	if h.completed || h.errored {
		h.mu.Unlock()
		return ErrNoActiveSession
	}
	session := h.session
	h.mu.Unlock()

	wsFrame, ok := frame.(WebSocketFrame)
	if !ok {
		// Try pointer
		if wsFramePtr, ok := frame.(*WebSocketFrame); ok {
			wsFrame = *wsFramePtr
		} else {
			return fmt.Errorf("expected WebSocketFrame, got %T", frame)
		}
	}

	// Add frame to the recording (only lock session, not hook)
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.closed {
		return ErrNoActiveSession
	}

	session.recording.AddWebSocketFrame(wsFrame)
	return nil
}

// OnClose is called when the WebSocket connection is closed.
func (h *FileStoreWebSocketHook) OnClose(code int, reason string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.completed || h.errored {
		return
	}

	// Record the close frame
	if err := h.store.AppendWebSocketCloseFrame(h.id, DirectionServerToClient, code, reason); err != nil {
		// Log error but don't fail - we still want to complete the recording
		return
	}
}

// OnComplete is called when the recording is successfully completed.
func (h *FileStoreWebSocketHook) OnComplete() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.completed {
		return nil
	}
	if h.errored {
		return ErrNoActiveSession
	}

	h.completed = true
	_, err := h.store.CompleteRecording(h.id)
	return err
}

// OnError is called when an error occurs during recording.
func (h *FileStoreWebSocketHook) OnError(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.completed || h.errored {
		return
	}

	h.errored = true

	// Mark the recording as incomplete
	_, _ = h.store.MarkIncomplete(h.id)
}

// Recording returns the underlying StreamRecording (for testing/inspection).
func (h *FileStoreWebSocketHook) Recording() *StreamRecording {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.session != nil {
		return h.session.recording
	}
	return nil
}

// FileStoreSSEHook is an SSERecordingHook implementation that uses FileStore.
type FileStoreSSEHook struct {
	mu        sync.Mutex
	store     *FileStore
	session   *StreamRecordingSession
	metadata  RecordingMetadata
	id        string
	completed bool
	errored   bool
	started   bool
}

// NewFileStoreSSEHook creates a new SSE recording hook backed by FileStore.
func NewFileStoreSSEHook(store *FileStore, metadata RecordingMetadata) (*FileStoreSSEHook, error) {
	if store == nil {
		return nil, fmt.Errorf("store cannot be nil")
	}

	// Start a new recording session
	session, err := store.StartRecording(ProtocolSSE, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to start recording session: %w", err)
	}

	return &FileStoreSSEHook{
		store:    store,
		session:  session,
		metadata: metadata,
		id:       session.recording.ID,
	}, nil
}

// ID returns the recording ID for this hook.
func (h *FileStoreSSEHook) ID() string {
	return h.id
}

// OnStreamStart is called when the SSE stream begins.
func (h *FileStoreSSEHook) OnStreamStart() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.started = true
}

// OnFrame is called when an SSE event is recorded.
func (h *FileStoreSSEHook) OnFrame(frame any) error {
	h.mu.Lock()
	if h.completed || h.errored {
		h.mu.Unlock()
		return ErrNoActiveSession
	}
	session := h.session
	h.mu.Unlock()

	sseEvent, ok := frame.(SSEEvent)
	if !ok {
		// Try pointer
		if sseEventPtr, ok := frame.(*SSEEvent); ok {
			sseEvent = *sseEventPtr
		} else {
			return fmt.Errorf("expected SSEEvent, got %T", frame)
		}
	}

	// Add event to the recording (only lock session, not hook)
	session.mu.Lock()
	defer session.mu.Unlock()

	if session.closed {
		return ErrNoActiveSession
	}

	session.recording.AddSSEEvent(sseEvent)
	return nil
}

// OnStreamEnd is called when the SSE stream ends.
func (h *FileStoreSSEHook) OnStreamEnd() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.completed || h.errored {
		return
	}

	// Mark the stream end time
	if h.session != nil && h.session.recording != nil {
		h.session.recording.SetSSEEnd()
	}
}

// OnComplete is called when the recording is successfully completed.
func (h *FileStoreSSEHook) OnComplete() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.completed {
		return nil
	}
	if h.errored {
		return ErrNoActiveSession
	}

	h.completed = true
	_, err := h.store.CompleteRecording(h.id)
	return err
}

// OnError is called when an error occurs during recording.
func (h *FileStoreSSEHook) OnError(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.completed || h.errored {
		return
	}

	h.errored = true

	// Mark the recording as incomplete
	_, _ = h.store.MarkIncomplete(h.id)
}

// Recording returns the underlying StreamRecording (for testing/inspection).
func (h *FileStoreSSEHook) Recording() *StreamRecording {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.session != nil {
		return h.session.recording
	}
	return nil
}

// Ensure implementations satisfy interfaces at compile time.
var (
	_ WebSocketRecordingHook = (*FileStoreWebSocketHook)(nil)
	_ SSERecordingHook       = (*FileStoreSSEHook)(nil)
)

// HookManager manages multiple recording hooks for a single connection.
type HookManager struct {
	mu    sync.RWMutex
	hooks []RecordingHook
}

// NewHookManager creates a new hook manager.
func NewHookManager() *HookManager {
	return &HookManager{
		hooks: make([]RecordingHook, 0),
	}
}

// Add adds a hook to the manager.
func (m *HookManager) Add(hook RecordingHook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = append(m.hooks, hook)
}

// Remove removes a hook by ID.
func (m *HookManager) Remove(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, hook := range m.hooks {
		if hook.ID() == id {
			m.hooks = append(m.hooks[:i], m.hooks[i+1:]...)
			return true
		}
	}
	return false
}

// NotifyFrame notifies all hooks of a new frame.
func (m *HookManager) NotifyFrame(frame any) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastErr error
	for _, hook := range m.hooks {
		if err := hook.OnFrame(frame); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// NotifyComplete notifies all hooks of completion.
func (m *HookManager) NotifyComplete() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var lastErr error
	for _, hook := range m.hooks {
		if err := hook.OnComplete(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// NotifyError notifies all hooks of an error.
func (m *HookManager) NotifyError(err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hook := range m.hooks {
		hook.OnError(err)
	}
}

// NotifyWebSocketConnect notifies all WebSocket hooks of a connection.
func (m *HookManager) NotifyWebSocketConnect(subprotocol string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hook := range m.hooks {
		if wsHook, ok := hook.(WebSocketRecordingHook); ok {
			wsHook.OnConnect(subprotocol)
		}
	}
}

// NotifyWebSocketClose notifies all WebSocket hooks of a close.
func (m *HookManager) NotifyWebSocketClose(code int, reason string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hook := range m.hooks {
		if wsHook, ok := hook.(WebSocketRecordingHook); ok {
			wsHook.OnClose(code, reason)
		}
	}
}

// NotifySSEStreamStart notifies all SSE hooks of stream start.
func (m *HookManager) NotifySSEStreamStart() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hook := range m.hooks {
		if sseHook, ok := hook.(SSERecordingHook); ok {
			sseHook.OnStreamStart()
		}
	}
}

// NotifySSEStreamEnd notifies all SSE hooks of stream end.
func (m *HookManager) NotifySSEStreamEnd() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, hook := range m.hooks {
		if sseHook, ok := hook.(SSERecordingHook); ok {
			sseHook.OnStreamEnd()
		}
	}
}

// Count returns the number of hooks.
func (m *HookManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.hooks)
}

// Clear removes all hooks.
func (m *HookManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hooks = make([]RecordingHook, 0)
}
