// Package recording provides types for SOAP call recording.
package recording

import (
	"crypto/rand"
	"fmt"
	"time"
)

// SOAPRecording represents a captured SOAP request/response pair.
type SOAPRecording struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`

	// Endpoint is the path of the SOAP service (e.g., "/soap/service")
	Endpoint string `json:"endpoint"`

	// Operation is the SOAP operation name extracted from the SOAP body
	Operation string `json:"operation"`

	// SOAPAction is the value from the SOAPAction header
	SOAPAction string `json:"soapAction,omitempty"`

	// SOAPVersion is the SOAP version ("1.1" or "1.2")
	SOAPVersion string `json:"soapVersion"`

	// RequestBody is the full SOAP envelope XML
	RequestBody string `json:"requestBody"`

	// ResponseBody is the full SOAP envelope XML
	ResponseBody string `json:"responseBody"`

	// ResponseStatus is the HTTP status code
	ResponseStatus int `json:"responseStatus"`

	// RequestHeaders contains HTTP headers from the request
	RequestHeaders map[string]string `json:"requestHeaders,omitempty"`

	// ResponseHeaders contains HTTP headers from the response
	ResponseHeaders map[string]string `json:"responseHeaders,omitempty"`

	// Duration is the time taken for the SOAP call
	Duration time.Duration `json:"duration"`

	// HasFault indicates whether the response contained a SOAP fault
	HasFault bool `json:"hasFault"`

	// FaultCode is the SOAP fault code (if HasFault is true)
	FaultCode string `json:"faultCode,omitempty"`

	// FaultMessage is the SOAP fault message (if HasFault is true)
	FaultMessage string `json:"faultMessage,omitempty"`
}

// NewSOAPRecording creates a new SOAP recording with a unique ID.
func NewSOAPRecording(endpoint, operation, soapVersion string) *SOAPRecording {
	return &SOAPRecording{
		ID:          generateSOAPID(),
		Timestamp:   time.Now(),
		Endpoint:    endpoint,
		Operation:   operation,
		SOAPVersion: soapVersion,
	}
}

// generateSOAPID generates a unique identifier for SOAP recordings.
func generateSOAPID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("soap-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// GetID returns the recording ID (implements Recordable).
func (r SOAPRecording) GetID() string { return r.ID }

// GetTimestamp returns the recording timestamp (implements Recordable).
func (r SOAPRecording) GetTimestamp() time.Time { return r.Timestamp }

// SetSOAPAction sets the SOAPAction header value.
func (r *SOAPRecording) SetSOAPAction(action string) {
	r.SOAPAction = action
}

// SetRequestBody sets the request SOAP envelope XML.
func (r *SOAPRecording) SetRequestBody(body string) {
	r.RequestBody = body
}

// SetResponseBody sets the response SOAP envelope XML.
func (r *SOAPRecording) SetResponseBody(body string) {
	r.ResponseBody = body
}

// SetResponseStatus sets the HTTP response status code.
func (r *SOAPRecording) SetResponseStatus(status int) {
	r.ResponseStatus = status
}

// SetRequestHeaders sets the HTTP request headers.
func (r *SOAPRecording) SetRequestHeaders(headers map[string]string) {
	r.RequestHeaders = headers
}

// SetResponseHeaders sets the HTTP response headers.
func (r *SOAPRecording) SetResponseHeaders(headers map[string]string) {
	r.ResponseHeaders = headers
}

// SetDuration sets the call duration.
func (r *SOAPRecording) SetDuration(d time.Duration) {
	r.Duration = d
}

// SetFault sets the fault information.
func (r *SOAPRecording) SetFault(code, message string) {
	r.HasFault = true
	r.FaultCode = code
	r.FaultMessage = message
}

// ClearFault clears any fault information.
func (r *SOAPRecording) ClearFault() {
	r.HasFault = false
	r.FaultCode = ""
	r.FaultMessage = ""
}

// SOAPRecordingFilter defines filtering options for SOAP recordings.
type SOAPRecordingFilter struct {
	Endpoint   string `json:"endpoint,omitempty"`
	Operation  string `json:"operation,omitempty"`
	SOAPAction string `json:"soapAction,omitempty"`
	HasFault   *bool  `json:"hasFault,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	Offset     int    `json:"offset,omitempty"`
}
