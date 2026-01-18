package protocol

import "context"

// Recordable handlers support request/message recording.
// Implement this interface for protocols that can record and replay traffic.
//
// Note: SSE uses a factory pattern and is intentionally excluded from this interface.
// SSE handlers create per-connection hooks instead of using a centralized recording toggle.
//
// Example implementation:
//
//	type MyHandler struct {
//	    recordingEnabled bool
//	    recordingMu      sync.RWMutex
//	}
//
//	func (h *MyHandler) EnableRecording() {
//	    h.recordingMu.Lock()
//	    defer h.recordingMu.Unlock()
//	    h.recordingEnabled = true
//	}
//
//	func (h *MyHandler) DisableRecording() {
//	    h.recordingMu.Lock()
//	    defer h.recordingMu.Unlock()
//	    h.recordingEnabled = false
//	}
//
//	func (h *MyHandler) IsRecordingEnabled() bool {
//	    h.recordingMu.RLock()
//	    defer h.recordingMu.RUnlock()
//	    return h.recordingEnabled
//	}
type Recordable interface {
	// EnableRecording starts recording requests/messages.
	EnableRecording()

	// DisableRecording stops recording requests/messages.
	DisableRecording()

	// IsRecordingEnabled returns true if recording is currently enabled.
	IsRecordingEnabled() bool
}

// RecordingStoreAware handlers can receive a recording store.
// The store type is protocol-specific (uses any to avoid circular imports).
// Handlers should type-assert to their expected store type.
//
// Example implementation:
//
//	func (h *MyHandler) SetRecordingStore(store any) {
//	    if s, ok := store.(*recording.Store); ok {
//	        h.recordingStore = s
//	    }
//	}
type RecordingStoreAware interface {
	// SetRecordingStore sets the recording store for the handler.
	// The store type is protocol-specific; implementations should type-assert.
	SetRecordingStore(store any)
}

// Replayable handlers can replay recorded sessions.
// This extends recording support with playback capabilities.
//
// Example implementation:
//
//	func (h *MyHandler) StartReplay(ctx context.Context, recordingID string) (string, error) {
//	    recording, err := h.recordingStore.Get(recordingID)
//	    if err != nil {
//	        return "", err
//	    }
//	    sessionID := uuid.New().String()
//	    h.replaySessions[sessionID] = &replaySession{
//	        recording: recording,
//	        ctx:       ctx,
//	    }
//	    go h.runReplay(sessionID)
//	    return sessionID, nil
//	}
type Replayable interface {
	// StartReplay begins replaying a recorded session.
	// Returns a session ID that can be used to stop the replay.
	// The context can be used for cancellation.
	StartReplay(ctx context.Context, recordingID string) (sessionID string, err error)

	// StopReplay stops an active replay session.
	// Returns ErrReplayNotFound if the session does not exist.
	StopReplay(sessionID string) error
}
