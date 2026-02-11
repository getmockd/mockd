// Package recording provides the unified StreamRecording type.
package recording

import (
	"time"

	"github.com/getmockd/mockd/internal/id"
)

// StreamRecording is the unified container for all stream recording types.
// It extends the existing HTTP recording infrastructure to support WebSocket and SSE.
type StreamRecording struct {
	// Identity (immutable after creation)
	ID      string `json:"id"`      // ULID
	Version string `json:"version"` // Format version, currently "1.0"

	// Protocol discriminator
	Protocol Protocol `json:"protocol"` // "http", "websocket", "sse"

	// Mutable metadata
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	UpdatedAt   string   `json:"updatedAt"` // ISO8601

	// Sync state (for future CRDT)
	SyncStatus SyncStatus `json:"syncStatus"`

	// Lifecycle
	Status    RecordingStatus `json:"status"`
	StartTime time.Time       `json:"startTime"`
	EndTime   *time.Time      `json:"endTime,omitempty"`
	Duration  int64           `json:"duration,omitempty"` // Milliseconds

	// Soft delete (tombstone pattern for CRDT)
	Deleted   bool   `json:"deleted,omitempty"`
	DeletedAt string `json:"deletedAt,omitempty"`

	// Request metadata (how recording was initiated)
	Metadata RecordingMetadata `json:"metadata"`

	// Protocol-specific data (exactly one should be non-nil)
	HTTP      *HTTPRecordingData      `json:"http,omitempty"`
	WebSocket *WebSocketRecordingData `json:"websocket,omitempty"`
	SSE       *SSERecordingData       `json:"sse,omitempty"`

	// Statistics
	Stats RecordingStats `json:"stats"`
}

// RecordingMetadata contains context about how recording was initiated.
type RecordingMetadata struct {
	// Request info (for WS upgrade or SSE initial request)
	Method      string            `json:"method,omitempty"`
	Path        string            `json:"path"`
	Host        string            `json:"host,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"queryParams,omitempty"`

	// Connection info
	ClientIP   string `json:"clientIp,omitempty"`
	ServerAddr string `json:"serverAddr,omitempty"`

	// Recording source
	Source RecordingSource `json:"source"`

	// Correlation
	RequestID     string `json:"requestId,omitempty"`
	ParentID      string `json:"parentId,omitempty"`
	CorrelationID string `json:"correlationId,omitempty"`

	// WebSocket specific
	Subprotocol string `json:"subprotocol,omitempty"`

	// SSE specific - detected template (e.g., "openai-chat")
	DetectedTemplate string `json:"detectedTemplate,omitempty"`
}

// RecordingStats contains aggregate statistics.
type RecordingStats struct {
	FrameCount    int   `json:"frameCount"`
	BytesSent     int64 `json:"bytesSent"`
	BytesReceived int64 `json:"bytesReceived"`
	FileSizeBytes int64 `json:"fileSizeBytes"`

	// WebSocket specific
	TextFrames   int `json:"textFrames,omitempty"`
	BinaryFrames int `json:"binaryFrames,omitempty"`
	PingPongs    int `json:"pingPongs,omitempty"`

	// SSE specific
	EventCount int `json:"eventCount,omitempty"`
}

// NewStreamRecording creates a new stream recording with a ULID.
func NewStreamRecording(protocol Protocol, metadata RecordingMetadata) *StreamRecording {
	now := time.Now()
	return &StreamRecording{
		ID:         id.ULID(),
		Version:    FormatVersion,
		Protocol:   protocol,
		Name:       generateRecordingName(protocol, metadata.Path, now),
		UpdatedAt:  now.Format(time.RFC3339),
		SyncStatus: SyncStatusLocal,
		Status:     RecordingStatusRecording,
		StartTime:  now,
		Metadata:   metadata,
		Stats:      RecordingStats{},
	}
}

// generateRecordingName creates a default name for a recording.
func generateRecordingName(protocol Protocol, path string, t time.Time) string {
	if path == "" {
		path = "/"
	}
	return string(protocol) + " " + path + " " + t.Format("2006-01-02 15:04:05")
}

// Complete marks the recording as complete and calculates duration.
func (r *StreamRecording) Complete() {
	now := time.Now()
	r.EndTime = &now
	r.Duration = now.Sub(r.StartTime).Milliseconds()
	r.Status = RecordingStatusComplete
	r.UpdatedAt = now.Format(time.RFC3339)
	r.updateStats()
}

// MarkIncomplete marks the recording as incomplete.
func (r *StreamRecording) MarkIncomplete() {
	now := time.Now()
	r.EndTime = &now
	r.Duration = now.Sub(r.StartTime).Milliseconds()
	r.Status = RecordingStatusIncomplete
	r.UpdatedAt = now.Format(time.RFC3339)
	r.updateStats()
}

// MarkCorrupted marks the recording as corrupted.
func (r *StreamRecording) MarkCorrupted() {
	r.Status = RecordingStatusCorrupted
	r.UpdatedAt = time.Now().Format(time.RFC3339)
}

// SoftDelete marks the recording as deleted.
func (r *StreamRecording) SoftDelete() {
	r.Deleted = true
	r.DeletedAt = time.Now().Format(time.RFC3339)
	r.UpdatedAt = r.DeletedAt
}

// updateStats recalculates statistics based on frames/events.
func (r *StreamRecording) updateStats() {
	switch r.Protocol { //nolint:exhaustive // only stream protocols have stats to update
	case ProtocolWebSocket:
		if r.WebSocket != nil {
			r.Stats.FrameCount = len(r.WebSocket.Frames)
			for _, f := range r.WebSocket.Frames {
				switch f.MessageType { //nolint:exhaustive // close frames don't need stat tracking
				case MessageTypeText:
					r.Stats.TextFrames++
				case MessageTypeBinary:
					r.Stats.BinaryFrames++
				case MessageTypePing, MessageTypePong:
					r.Stats.PingPongs++
				}
				if f.Direction == DirectionServerToClient {
					r.Stats.BytesSent += int64(f.DataSize)
				} else {
					r.Stats.BytesReceived += int64(f.DataSize)
				}
			}
		}
	case ProtocolSSE:
		if r.SSE != nil {
			r.Stats.FrameCount = len(r.SSE.Events)
			r.Stats.EventCount = len(r.SSE.Events)
			for _, e := range r.SSE.Events {
				r.Stats.BytesSent += int64(e.DataSize)
			}
		}
	}
}

// AddWebSocketFrame adds a frame to a WebSocket recording.
func (r *StreamRecording) AddWebSocketFrame(frame WebSocketFrame) {
	if r.WebSocket == nil {
		r.WebSocket = &WebSocketRecordingData{
			ConnectedAt: r.StartTime,
			Frames:      make([]WebSocketFrame, 0),
		}
	}
	r.WebSocket.Frames = append(r.WebSocket.Frames, frame)
}

// AddSSEEvent adds an event to an SSE recording.
func (r *StreamRecording) AddSSEEvent(event SSEEvent) {
	if r.SSE == nil {
		r.SSE = &SSERecordingData{
			StreamStartedAt: event.Timestamp,
			Events:          make([]SSEEvent, 0),
		}
	}
	r.SSE.Events = append(r.SSE.Events, event)
}

// SetWebSocketClose records the WebSocket close info.
func (r *StreamRecording) SetWebSocketClose(code int, reason string) {
	if r.WebSocket != nil {
		now := time.Now()
		r.WebSocket.DisconnectedAt = &now
		r.WebSocket.CloseCode = &code
		r.WebSocket.CloseReason = &reason
	}
}

// SetSSEEnd records when the SSE stream ended.
func (r *StreamRecording) SetSSEEnd() {
	if r.SSE != nil {
		now := time.Now()
		r.SSE.StreamEndedAt = &now
	}
}

// RecordingSummary is a lightweight view for listing.
type RecordingSummary struct {
	ID         string          `json:"id"`
	Protocol   Protocol        `json:"protocol"`
	Name       string          `json:"name"`
	Path       string          `json:"path"`
	Status     RecordingStatus `json:"status"`
	StartTime  time.Time       `json:"startTime"`
	Duration   int64           `json:"duration"`
	FrameCount int             `json:"frameCount"`
	FileSize   int64           `json:"fileSize"`
	Tags       []string        `json:"tags,omitempty"`
	SyncStatus SyncStatus      `json:"syncStatus"`
	Deleted    bool            `json:"deleted,omitempty"`
}

// ToSummary creates a summary view of the recording.
func (r *StreamRecording) ToSummary() RecordingSummary {
	return RecordingSummary{
		ID:         r.ID,
		Protocol:   r.Protocol,
		Name:       r.Name,
		Path:       r.Metadata.Path,
		Status:     r.Status,
		StartTime:  r.StartTime,
		Duration:   r.Duration,
		FrameCount: r.Stats.FrameCount,
		FileSize:   r.Stats.FileSizeBytes,
		Tags:       r.Tags,
		SyncStatus: r.SyncStatus,
		Deleted:    r.Deleted,
	}
}

// Validate checks if the recording is valid.
func (r *StreamRecording) Validate() error {
	if !id.IsValidULID(r.ID) {
		return ErrInvalidULID
	}
	if !r.Protocol.IsValid() {
		return ErrInvalidProtocol
	}
	if !r.Status.IsValid() {
		return ErrInvalidStatus
	}
	return nil
}
