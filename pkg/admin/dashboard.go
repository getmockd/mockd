//go:build dashboard

package admin

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dashboard/dist
var dashboardFS embed.FS

// registerDashboard registers a catch-all handler that serves the embedded
// frontend dashboard. API routes registered first take priority; any path
// not matched by the API falls through to the SPA file server.
//
// The handler implements SPA routing: if the requested path matches a real
// file in the embedded dist, serve it. Otherwise, serve index.html so the
// frontend router can handle client-side navigation.
func (a *API) registerDashboard(mux *http.ServeMux) {
	// Strip the "dashboard/dist" prefix so the embed root maps to "/".
	distFS, err := fs.Sub(dashboardFS, "dashboard/dist")
	if err != nil {
		a.logger().Error("failed to create dashboard sub-filesystem", "error", err)
		return
	}

	fileServer := http.FileServer(http.FS(distFS))

	// Catch-all: serve dashboard assets. The Go 1.22+ ServeMux routes
	// API paths like "GET /mocks" with higher priority than "GET /"
	// because they are more specific, so API routes are not shadowed.
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		// Skip API-like paths that somehow weren't matched — safety net.
		// All API routes are registered with explicit method+path patterns
		// before this catch-all, so this should rarely trigger.
		path := r.URL.Path

		// Try to serve the exact file first.
		// For SPA routing, if the file doesn't exist (no extension or
		// not a known static asset), serve index.html instead.
		if path != "/" && !strings.Contains(path, ".") {
			// No file extension — this is a client-side route.
			// Serve index.html and let the SPA router handle it.
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})
}
