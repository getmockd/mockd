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
		path := r.URL.Path

		// For SPA routing, if the file doesn't exist (no extension or
		// not a known static asset), serve index.html instead.
		if path != "/" && !strings.Contains(path, ".") {
			r.URL.Path = "/"
		}

		// Relax CSP for the dashboard — the Svelte frontend uses inline
		// styles (scoped component CSS) and fetches from the engine on a
		// different port. The strict "default-src 'self'" set by
		// SecurityHeadersMiddleware blocks both.
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self' data:; connect-src 'self' http://localhost:* ws://localhost:*")

		fileServer.ServeHTTP(w, r)
	})
}
