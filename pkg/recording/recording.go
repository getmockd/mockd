// Package recording provides types and utilities for capturing HTTP traffic.
package recording

import (
	"crypto/rand"
	"fmt"
	"net/http"
	"time"
)

// Recording represents a captured HTTP request/response pair.
type Recording struct {
	ID        string    `json:"id"`
	SessionID string    `json:"sessionId"`
	Timestamp time.Time `json:"timestamp"`

	Request  RecordedRequest  `json:"request"`
	Response RecordedResponse `json:"response"`

	Duration time.Duration `json:"duration"`
}

// RecordedRequest represents the captured request details.
type RecordedRequest struct {
	Method  string      `json:"method"`
	URL     string      `json:"url"`
	Path    string      `json:"path"`
	Host    string      `json:"host"`
	Scheme  string      `json:"scheme"`
	Headers http.Header `json:"headers"`
	Body    []byte      `json:"body,omitempty"`
}

// RecordedResponse represents the captured response details.
type RecordedResponse struct {
	StatusCode int         `json:"statusCode"`
	Status     string      `json:"statusText"`
	Headers    http.Header `json:"headers"`
	Body       []byte      `json:"body,omitempty"`
}

// NewRecording creates a new recording with a unique ID.
func NewRecording(sessionID string) *Recording {
	return &Recording{
		ID:        generateID(),
		SessionID: sessionID,
		Timestamp: time.Now(),
	}
}

// generateID generates a unique identifier using crypto/rand.
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// CaptureRequest captures details from an HTTP request.
func (r *Recording) CaptureRequest(req *http.Request, body []byte) {
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}

	r.Request = RecordedRequest{
		Method:  req.Method,
		URL:     req.URL.String(),
		Path:    req.URL.Path,
		Host:    req.Host,
		Scheme:  scheme,
		Headers: req.Header.Clone(),
		Body:    body,
	}
}

// CaptureResponse captures details from an HTTP response.
func (r *Recording) CaptureResponse(resp *http.Response, body []byte, duration time.Duration) {
	r.Response = RecordedResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    resp.Header.Clone(),
		Body:       body,
	}
	r.Duration = duration
}

// DurationString returns a human-readable duration string.
func (r *Recording) DurationString() string {
	return r.Duration.String()
}
