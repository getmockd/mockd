// Package websocket provides WebSocket recording integration.
package websocket

import (
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// RecordingConfig holds recording configuration for a WebSocket endpoint.
type RecordingConfig struct {
	Enabled bool
	Store   *recording.FileStore
}

// ConnectionRecorder wraps a connection to record messages.
type ConnectionRecorder struct {
	conn      *Connection
	hook      recording.WebSocketRecordingHook
	startTime time.Time
	seq       int64
}

// NewConnectionRecorder creates a recorder for a connection.
func NewConnectionRecorder(conn *Connection, hook recording.WebSocketRecordingHook) *ConnectionRecorder {
	return &ConnectionRecorder{
		conn:      conn,
		hook:      hook,
		startTime: time.Now(),
		seq:       0,
	}
}

// RecordSend records an outgoing message.
func (r *ConnectionRecorder) RecordSend(msgType MessageType, data []byte) error {
	r.seq++
	frame := recording.NewWebSocketFrame(
		r.seq,
		r.startTime,
		recording.DirectionServerToClient,
		convertMessageType(msgType),
		data,
	)
	return r.hook.OnFrame(frame)
}

// RecordReceive records an incoming message.
func (r *ConnectionRecorder) RecordReceive(msgType MessageType, data []byte) error {
	r.seq++
	frame := recording.NewWebSocketFrame(
		r.seq,
		r.startTime,
		recording.DirectionClientToServer,
		convertMessageType(msgType),
		data,
	)
	return r.hook.OnFrame(frame)
}

// RecordClose records the connection close.
func (r *ConnectionRecorder) RecordClose(code int, reason string) error {
	r.hook.OnClose(code, reason)
	return nil
}

// Complete completes the recording.
func (r *ConnectionRecorder) Complete() error {
	return r.hook.OnComplete()
}

// convertMessageType converts websocket.MessageType to recording.MessageType.
func convertMessageType(msgType MessageType) recording.MessageType {
	switch msgType {
	case MessageText:
		return recording.MessageTypeText
	case MessageBinary:
		return recording.MessageTypeBinary
	default:
		return recording.MessageTypeText
	}
}
