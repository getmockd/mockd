//go:build !dashboard

package admin

import "net/http"

// registerDashboard is a no-op when the "dashboard" build tag is not set.
// Build with: go build -tags dashboard
func (a *API) registerDashboard(_ *http.ServeMux) {}
