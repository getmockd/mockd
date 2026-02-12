package protocol

import "net/http"

// HTTPHandler is the interface for HTTP-based protocol handlers.
// This includes GraphQL, SOAP, and SSE (via StreamingHTTPHandler).
//
// Handlers that implement this interface can be registered with an
// http.ServeMux using the Pattern() method to determine the route.
//
// Example implementation:
//
//	type MyHandler struct {
//	    path string
//	}
//
//	func (h *MyHandler) Pattern() string {
//	    return h.path
//	}
//
//	func (h *MyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
//	    // Handle request
//	}
type HTTPHandler interface {
	Handler
	http.Handler

	// Pattern returns the URL pattern this handler serves.
	// This is used for registering with http.ServeMux.
	// Examples: "/graphql", "/soap", "/events", "/ws"
	Pattern() string
}

// StreamingHTTPHandler is for HTTP handlers that support streaming responses.
// SSE is the primary example of this pattern.
//
// The engine uses IsStreamingRequest to determine if special handling
// is needed (e.g., disabling response buffering, setting timeouts).
type StreamingHTTPHandler interface {
	HTTPHandler

	// IsStreamingRequest returns true if the request should be handled
	// as a streaming response (e.g., SSE Accept header).
	IsStreamingRequest(r *http.Request) bool
}
