package protocol

import "net/http"

// HTTPHandler is the interface for HTTP-based protocol handlers.
// This includes GraphQL and SOAP (which implement standard http.Handler).
// SSE has a non-standard ServeHTTP signature (requires mock config) and
// does not implement this interface.
//
// Status: Capability contract for future Admin API extensions.
// GraphQL and SOAP satisfy this interface but it is not type-asserted at runtime.
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
// SSE is the canonical example, but its non-standard ServeHTTP(w, r, mock)
// signature prevents it from satisfying this interface directly.
//
// Status: Capability contract for future Admin API extensions.
// Not type-asserted at runtime. The engine uses IsStreamingRequest to
// determine if special handling is needed (e.g., disabling response
// buffering, setting timeouts).
type StreamingHTTPHandler interface {
	HTTPHandler

	// IsStreamingRequest returns true if the request should be handled
	// as a streaming response (e.g., SSE Accept header).
	IsStreamingRequest(r *http.Request) bool
}
