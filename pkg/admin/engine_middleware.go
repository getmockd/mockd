// Engine middleware provides utilities for handlers that require a connected engine.

package admin

import (
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

// EngineHandlerFunc is a handler function that requires an engine client.
// The engine client is guaranteed to be non-nil when the handler is called.
type EngineHandlerFunc func(w http.ResponseWriter, r *http.Request, engine *engineclient.Client)

// requireEngine wraps a handler that requires an engine connection.
// If no engine is connected, it returns a 503 Service Unavailable response.
// This eliminates the repeated "if a.localEngine == nil" pattern across handlers.
//
// Usage:
//
//	mux.HandleFunc("GET /mocks", a.requireEngine(a.handleListMocksWithEngine))
//
//	func (a *AdminAPI) handleListMocksWithEngine(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
//	    // engine is guaranteed to be non-nil here
//	    mocks, err := engine.ListMocks(r.Context())
//	    ...
//	}
func (a *AdminAPI) requireEngine(handler EngineHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.localEngine == nil {
			writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
			return
		}
		handler(w, r, a.localEngine)
	}
}

// requireEngineOr wraps a handler that requires an engine connection, with custom error handling.
// If no engine is connected, it calls the fallback function instead.
//
// Usage:
//
//	mux.HandleFunc("GET /mocks", a.requireEngineOr(a.handleListMocksWithEngine, a.handleNoEngine))
func (a *AdminAPI) requireEngineOr(handler EngineHandlerFunc, fallback http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if a.localEngine == nil {
			fallback(w, r)
			return
		}
		handler(w, r, a.localEngine)
	}
}

// HasEngine returns true if an engine is connected.
// Useful for conditional logic in templates or status endpoints.
func (a *AdminAPI) HasEngine() bool {
	return a.localEngine != nil
}

// Engine returns the engine client, or nil if not connected.
// Prefer using requireEngine() for handlers instead of direct access.
func (a *AdminAPI) Engine() *engineclient.Client {
	return a.localEngine
}

// withEngine is a helper for inline use when you need the engine check
// but want to continue with custom logic. Returns nil if no engine.
//
// Usage:
//
//	func (a *AdminAPI) handleSomething(w http.ResponseWriter, r *http.Request) {
//	    engine := a.withEngine(w)
//	    if engine == nil {
//	        return // Error already written
//	    }
//	    // Continue with engine...
//	}
func (a *AdminAPI) withEngine(w http.ResponseWriter) *engineclient.Client {
	if a.localEngine == nil {
		writeError(w, http.StatusServiceUnavailable, "no_engine", "No engine connected")
		return nil
	}
	return a.localEngine
}
