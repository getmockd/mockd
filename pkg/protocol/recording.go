package protocol

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
