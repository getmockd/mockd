// Package sse provides SSE recording integration.
package sse

import (
	"encoding/json"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// RecordingConfig holds recording configuration for SSE endpoints.
type RecordingConfig struct {
	Enabled bool
	Store   *recording.FileStore
}

// StreamRecorder wraps an SSE stream to record events.
type StreamRecorder struct {
	stream        *SSEStream
	hook          recording.SSERecordingHook
	startTime     time.Time
	eventSequence int64
}

// NewStreamRecorder creates a recorder for an SSE stream.
func NewStreamRecorder(stream *SSEStream, hook recording.SSERecordingHook) *StreamRecorder {
	recorder := &StreamRecorder{
		stream:    stream,
		hook:      hook,
		startTime: time.Now(),
	}

	// Notify hook that stream has started
	hook.OnStreamStart()

	return recorder
}

// RecordEvent records an outgoing SSE event.
func (r *StreamRecorder) RecordEvent(eventType, data, id string, retry *int) error {
	if r.hook == nil {
		return nil
	}

	r.eventSequence++

	// Create recording event
	event := recording.NewSSEEvent(
		r.eventSequence,
		r.startTime,
		eventType,
		data,
		id,
		retry,
	)

	// Pass to hook
	return r.hook.OnFrame(event)
}

// RecordEventDef records an SSEEventDef as a recording event.
func (r *StreamRecorder) RecordEventDef(event *SSEEventDef) error {
	if r.hook == nil {
		return nil
	}

	// Convert data to string
	var dataStr string
	switch d := event.Data.(type) {
	case string:
		dataStr = d
	case []byte:
		dataStr = string(d)
	default:
		// Marshal to JSON for other types
		bytes, err := json.Marshal(d)
		if err != nil {
			dataStr = ""
		} else {
			dataStr = string(bytes)
		}
	}

	// Convert retry pointer
	var retry *int
	if event.Retry > 0 {
		retry = &event.Retry
	}

	return r.RecordEvent(event.Type, dataStr, event.ID, retry)
}

// Complete completes the recording.
func (r *StreamRecorder) Complete() error {
	if r.hook == nil {
		return nil
	}

	// Notify hook that stream has ended
	r.hook.OnStreamEnd()

	// Complete the recording
	return r.hook.OnComplete()
}

// Error signals an error occurred during recording.
func (r *StreamRecorder) Error(err error) {
	if r.hook != nil {
		r.hook.OnError(err)
	}
}

// Hook returns the underlying recording hook.
func (r *StreamRecorder) Hook() recording.SSERecordingHook {
	return r.hook
}
