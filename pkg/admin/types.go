package admin

import "time"

// MockVerification represents the verification state for a mock.
type MockVerification struct {
	MockID       string     `json:"mockId"`
	CallCount    int        `json:"callCount"`
	LastCalledAt *time.Time `json:"lastCalledAt,omitempty"`
}

// MockInvocation represents a single invocation of a mock.
type MockInvocation struct {
	ID          string            `json:"id"`
	Timestamp   time.Time         `json:"timestamp"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Headers     map[string]string `json:"headers,omitempty"`
	QueryParams map[string]string `json:"queryParams,omitempty"`
	Body        string            `json:"body,omitempty"`
}

// VerifyRequest represents a verification request with call count criteria.
type VerifyRequest struct {
	AtLeast *int  `json:"atLeast,omitempty"`
	AtMost  *int  `json:"atMost,omitempty"`
	Exactly *int  `json:"exactly,omitempty"`
	Never   *bool `json:"never,omitempty"`
}

// VerifyResponse represents the result of a verification check.
type VerifyResponse struct {
	Passed   bool   `json:"passed"`
	Actual   int    `json:"actual"`
	Expected string `json:"expected,omitempty"`
	Message  string `json:"message,omitempty"`
}

// MockInvocationListResponse represents a paginated list of mock invocations.
type MockInvocationListResponse struct {
	Invocations []MockInvocation `json:"invocations"`
	Count       int              `json:"count"`
	Total       int              `json:"total"`
}

// WebSocketSendRequest represents a request to send a message to a WebSocket connection.
type WebSocketSendRequest struct {
	// Type is the message type: "text" (default) or "binary".
	Type string `json:"type"`
	// Data is the message payload.
	// For type "text", Data is a plain UTF-8 string.
	// For type "binary", Data must be a base64-encoded string; the server decodes it
	// before sending the raw bytes over the WebSocket connection.
	Data string `json:"data"`
}
