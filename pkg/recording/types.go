// Package recording provides types for stream recording and replay.
package recording

import (
	"errors"
	"time"
)

// Errors for stream recording
var (
	ErrInvalidULID     = errors.New("invalid ULID format")
	ErrStorageFull     = errors.New("recording storage limit exceeded")
	ErrNoActiveSession = errors.New("no active recording session")
	ErrSessionActive   = errors.New("recording session already active")
	ErrInvalidProtocol = errors.New("invalid protocol")
	ErrInvalidStatus   = errors.New("invalid recording status")
	ErrCorrupted       = errors.New("recording file corrupted")
)

// Protocol identifies the type of recording.
type Protocol string

const (
	ProtocolHTTP      Protocol = "http"
	ProtocolWebSocket Protocol = "websocket"
	ProtocolSSE       Protocol = "sse"
)

// IsValid checks if the protocol is valid.
func (p Protocol) IsValid() bool {
	switch p {
	case ProtocolHTTP, ProtocolWebSocket, ProtocolSSE:
		return true
	default:
		return false
	}
}

// RecordingStatus represents the lifecycle state of a recording.
type RecordingStatus string

const (
	RecordingStatusRecording  RecordingStatus = "recording"
	RecordingStatusComplete   RecordingStatus = "complete"
	RecordingStatusIncomplete RecordingStatus = "incomplete"
	RecordingStatusCorrupted  RecordingStatus = "corrupted"
)

// IsValid checks if the status is valid.
func (s RecordingStatus) IsValid() bool {
	switch s {
	case RecordingStatusRecording, RecordingStatusComplete, RecordingStatusIncomplete, RecordingStatusCorrupted:
		return true
	default:
		return false
	}
}

// SyncStatus represents cloud sync state (for future CRDT sync).
type SyncStatus string

const (
	SyncStatusLocal   SyncStatus = "local"
	SyncStatusPending SyncStatus = "pending"
	SyncStatusSynced  SyncStatus = "synced"
)

// Direction indicates message direction.
type Direction string

const (
	DirectionClientToServer Direction = "c2s"
	DirectionServerToClient Direction = "s2c"
)

// MessageType for WebSocket frames.
type MessageType string

const (
	MessageTypeText   MessageType = "text"
	MessageTypeBinary MessageType = "binary"
	MessageTypePing   MessageType = "ping"
	MessageTypePong   MessageType = "pong"
	MessageTypeClose  MessageType = "close"
)

// DataEncoding indicates how data is encoded.
type DataEncoding string

const (
	DataEncodingUTF8   DataEncoding = "utf8"
	DataEncodingBase64 DataEncoding = "base64"
)

// ReplayMode defines how recordings are replayed.
type ReplayMode string

const (
	ReplayModePure         ReplayMode = "pure"
	ReplayModeSynchronized ReplayMode = "synchronized"
	ReplayModeTriggered    ReplayMode = "triggered"
)

// IsValid checks if the replay mode is valid.
func (m ReplayMode) IsValid() bool {
	switch m {
	case ReplayModePure, ReplayModeSynchronized, ReplayModeTriggered:
		return true
	default:
		return false
	}
}

// ReplayStatus represents the state of a replay session.
type ReplayStatus string

const (
	ReplayStatusPending  ReplayStatus = "pending"
	ReplayStatusPlaying  ReplayStatus = "playing"
	ReplayStatusWaiting  ReplayStatus = "waiting"
	ReplayStatusPaused   ReplayStatus = "paused"
	ReplayStatusComplete ReplayStatus = "complete"
	ReplayStatusAborted  ReplayStatus = "aborted"
)

// RecordingSource indicates how the recording was created.
type RecordingSource string

const (
	RecordingSourceProxy  RecordingSource = "proxy"
	RecordingSourceMock   RecordingSource = "mock"
	RecordingSourceManual RecordingSource = "manual"
)

// ExportFormat defines export file formats.
type ExportFormat string

const (
	ExportFormatJSON ExportFormat = "json"
	ExportFormatYAML ExportFormat = "yaml"
)

// FormatVersion is the current recording format version.
const FormatVersion = "1.0"

// DefaultMaxStorageBytes is the default storage limit (500MB).
const DefaultMaxStorageBytes = 500 * 1024 * 1024

// DefaultWarnPercent is the default warning threshold.
const DefaultWarnPercent = 80

// StorageConfig configures the recording storage.
type StorageConfig struct {
	// DataDir is the directory for storing recordings.
	DataDir string `json:"dataDir"`

	// MaxBytes is the maximum total size of recordings.
	MaxBytes int64 `json:"maxBytes"`

	// WarnPercent is the percentage at which to warn about storage usage.
	WarnPercent int `json:"warnPercent"`

	// FilterHeaders are headers to redact from recordings.
	FilterHeaders []string `json:"filterHeaders"`

	// FilterBodyKeys are JSON keys to redact from request/response bodies.
	FilterBodyKeys []string `json:"filterBodyKeys"`

	// RedactValue is the replacement value for redacted content.
	RedactValue string `json:"redactValue"`
}

// DefaultFilterHeaders are headers filtered by default for security.
var DefaultFilterHeaders = []string{
	"Authorization",
	"Cookie",
	"Set-Cookie",
	"X-API-Key",
	"X-Auth-Token",
}

// StorageStats contains storage usage information.
type StorageStats struct {
	UsedBytes      int64   `json:"usedBytes"`
	MaxBytes       int64   `json:"maxBytes"`
	UsedPercent    float64 `json:"usedPercent"`
	RecordingCount int     `json:"recordingCount"`

	// Counts by protocol
	HTTPCount      int `json:"httpCount"`
	WebSocketCount int `json:"websocketCount"`
	SSECount       int `json:"sseCount"`

	// Age info
	OldestRecording string     `json:"oldestRecording,omitempty"`
	OldestDate      *time.Time `json:"oldestDate,omitempty"`
	NewestRecording string     `json:"newestRecording,omitempty"`
	NewestDate      *time.Time `json:"newestDate,omitempty"`
}
