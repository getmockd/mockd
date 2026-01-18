// Package recording provides types for gRPC call recording.
package recording

import (
	"crypto/rand"
	"fmt"
	"time"
)

// GRPCRecording represents a captured gRPC request/response pair.
type GRPCRecording struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	// Service and method identification
	Service string `json:"service"` // e.g., "mypackage.MyService"
	Method  string `json:"method"`  // e.g., "GetUser"

	// Stream type
	StreamType GRPCStreamType `json:"streamType"` // unary, client_stream, server_stream, bidi

	// Request data - JSON-compatible map for unary, []interface{} for streaming
	Request interface{} `json:"request"`

	// Response data - JSON-compatible map for unary, []interface{} for streaming
	Response interface{} `json:"response"`

	// gRPC metadata (headers)
	Metadata map[string][]string `json:"metadata,omitempty"`

	// Error if call returned error
	Error *GRPCRecordedError `json:"error,omitempty"`

	// Call duration
	Duration time.Duration `json:"duration"`

	// Path to proto file used for this call
	ProtoFile string `json:"protoFile,omitempty"`
}

// GRPCStreamType identifies the type of gRPC call.
type GRPCStreamType string

const (
	GRPCStreamUnary        GRPCStreamType = "unary"
	GRPCStreamClientStream GRPCStreamType = "client_stream"
	GRPCStreamServerStream GRPCStreamType = "server_stream"
	GRPCStreamBidi         GRPCStreamType = "bidi"
)

// GRPCRecordedError represents a gRPC error that was returned.
type GRPCRecordedError struct {
	Code    string `json:"code"`    // gRPC status code name (e.g., "NOT_FOUND")
	Message string `json:"message"` // Error message
}

// NewGRPCRecording creates a new gRPC recording with a unique ID.
func NewGRPCRecording(service, method string, streamType GRPCStreamType) *GRPCRecording {
	return &GRPCRecording{
		ID:         generateGRPCID(),
		Timestamp:  time.Now(),
		Service:    service,
		Method:     method,
		StreamType: streamType,
	}
}

// generateGRPCID generates a unique identifier for gRPC recordings.
func generateGRPCID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("grpc-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// SetRequest sets the request data.
func (r *GRPCRecording) SetRequest(data interface{}) {
	r.Request = data
}

// SetResponse sets the response data.
func (r *GRPCRecording) SetResponse(data interface{}) {
	r.Response = data
}

// SetMetadata sets the gRPC metadata.
func (r *GRPCRecording) SetMetadata(md map[string][]string) {
	r.Metadata = md
}

// SetError sets the error information.
func (r *GRPCRecording) SetError(code, message string) {
	r.Error = &GRPCRecordedError{
		Code:    code,
		Message: message,
	}
}

// SetDuration sets the call duration.
func (r *GRPCRecording) SetDuration(d time.Duration) {
	r.Duration = d
}

// SetProtoFile sets the proto file path.
func (r *GRPCRecording) SetProtoFile(path string) {
	r.ProtoFile = path
}

// FullMethod returns the full gRPC method name.
func (r *GRPCRecording) FullMethod() string {
	return "/" + r.Service + "/" + r.Method
}

// GRPCRecordingFilter defines filtering options for gRPC recordings.
type GRPCRecordingFilter struct {
	Service    string `json:"service,omitempty"`
	Method     string `json:"method,omitempty"`
	StreamType string `json:"streamType,omitempty"`
	HasError   *bool  `json:"hasError,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}
