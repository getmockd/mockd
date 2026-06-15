package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// These tests cover the two hardening fixes:
//   Issue 1 — a cross-origin browser fetch() over the loopback socket must not
//             ride the keyless localhost bypass, and the admin CORS default must
//             not echo arbitrary web origins.
//   Issue 2 — the dashboard-asset auth exemption must not exempt /openapi.json
//             and /insomnia.json (or source maps) by file extension.

// authAPI builds an admin API with API-key auth enabled.
func authAPI(t *testing.T, allowLocalhost bool) *API {
	t.Helper()
	api := NewAPI(0,
		WithDataDir(t.TempDir()),
		WithAPIKey("test-api-key"),
		WithAPIKeyAllowLocalhost(allowLocalhost),
	)
	t.Cleanup(func() { api.Stop() })
	return api
}

func serveAdmin(t *testing.T, api *API, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	api.httpServer.Handler.ServeHTTP(rec, req)
	return rec
}

// ----------------------------------------------------------------------------
// Issue 1 — localhost bypass gating
// ----------------------------------------------------------------------------

func TestLocalhostBypass_CrossSiteBrowser_RequiresAuth(t *testing.T) {
	api := authAPI(t, true)
	req := httptest.NewRequest("GET", "/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "localhost:4290"
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Origin", "https://evil.example")

	rec := serveAdmin(t, api, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"a cross-site browser fetch over loopback must not bypass the API key")
}

func TestLocalhostBypass_LegacyBrowserForeignOrigin_RequiresAuth(t *testing.T) {
	// Older browser / WebView that omits Sec-Fetch-* but still sends Origin.
	api := authAPI(t, true)
	req := httptest.NewRequest("GET", "/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "localhost:4290"
	req.Header.Set("Origin", "https://evil.example")

	rec := serveAdmin(t, api, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"a foreign Origin over loopback must require auth even without Sec-Fetch-*")
}

func TestLocalhostBypass_CrossPortLocalhostDashboard_Allowed(t *testing.T) {
	// Vite dev server on :5173 calling the admin API on :4290. Browsers label
	// this "cross-site" because localhost is not on the Public Suffix List, but
	// it is the same machine and must keep working.
	api := authAPI(t, true)
	req := httptest.NewRequest("GET", "/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "localhost:4290"
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("Origin", "http://localhost:5173")

	rec := serveAdmin(t, api, req)
	assert.NotEqual(t, http.StatusUnauthorized, rec.Code,
		"a cross-port localhost dashboard must keep the localhost bypass")
}

func TestLocalhostBypass_SameOriginDashboard_Allowed(t *testing.T) {
	api := authAPI(t, true)
	req := httptest.NewRequest("GET", "/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "localhost:4290"
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Origin", "http://localhost:4290")

	rec := serveAdmin(t, api, req)
	assert.NotEqual(t, http.StatusUnauthorized, rec.Code,
		"the same-origin embedded dashboard must keep the localhost bypass")
}

func TestLocalhostBypass_DNSRebinding_Host_RequiresAuth(t *testing.T) {
	// A DNS-rebinding page keeps the attacker hostname in Host even though it
	// resolves to 127.0.0.1, and may claim Sec-Fetch-Site: same-origin. The
	// Host-header anti-rebinding layer must still deny the bypass.
	hosts := []string{"evil.com", "evil.com:4290", "127.0.0.1.attacker.com"}
	for _, host := range hosts {
		t.Run(host, func(t *testing.T) {
			api := authAPI(t, true)
			req := httptest.NewRequest("GET", "/status", nil)
			req.RemoteAddr = "127.0.0.1:12345"
			req.Host = host
			req.Header.Set("Sec-Fetch-Site", "same-origin")
			req.Header.Set("Origin", "http://"+host)

			rec := serveAdmin(t, api, req)
			assert.Equal(t, http.StatusUnauthorized, rec.Code,
				"a rebinding Host must deny the localhost bypass")
		})
	}
}

func TestLocalhostBypass_NonBrowserLoopback_Allowed(t *testing.T) {
	// CLI/curl/MCP send no Origin/Sec-Fetch-* headers. They keep keyless
	// localhost access even when their Host header is not loopback (here the
	// httptest default "example.com").
	api := authAPI(t, true)
	req := httptest.NewRequest("GET", "/status", nil)
	req.RemoteAddr = "127.0.0.1:12345"

	rec := serveAdmin(t, api, req)
	assert.NotEqual(t, http.StatusUnauthorized, rec.Code,
		"non-browser loopback clients must keep keyless localhost access")
}

func TestLocalhostBypass_HeaderlessWrite_LoopbackHost_Allowed(t *testing.T) {
	// A header-less write (e.g. the mockd CLI POSTing to localhost) keeps keyless
	// access when its Host is loopback.
	api := authAPI(t, true)
	req := httptest.NewRequest("POST", "/mocks", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "localhost:4290"

	rec := serveAdmin(t, api, req)
	assert.NotEqual(t, http.StatusUnauthorized, rec.Code,
		"a header-less loopback write with a loopback Host keeps keyless access")
}

func TestLocalhostBypass_HeaderlessWrite_NonLoopbackHost_RequiresAuth(t *testing.T) {
	// A state-changing request with a non-loopback Host must not keyless-write,
	// even with no browser fingerprint (closes the header-less write bypass).
	api := authAPI(t, true)
	req := httptest.NewRequest("POST", "/mocks", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.Host = "example.com"

	rec := serveAdmin(t, api, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code,
		"a header-less write with a non-loopback Host must require the API key")
}

// ----------------------------------------------------------------------------
// Issue 1 — predicate unit tests
// ----------------------------------------------------------------------------

func TestHostnameIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"localhost":              true,
		"LOCALHOST":              true,
		"127.0.0.1":              true,
		"127.0.0.2":              true,
		"::1":                    true,
		"[::1]":                  true,
		"app.localhost":          false, // *.localhost is NOT trusted (DNS-controllable)
		"evil.com":               false,
		"127.0.0.1.attacker.com": false,
		"localhost.attacker.com": false,
		"2130706433":             false, // decimal 127.0.0.1 — net.ParseIP rejects
		"0177.0.0.1":             false, // octal form — rejected
		"127.1":                  false, // short form — rejected
		"":                       false,
		"null":                   false,
		"192.168.1.10":           false,
	}
	for host, want := range cases {
		assert.Equalf(t, want, hostnameIsLoopback(host), "hostnameIsLoopback(%q)", host)
	}
}

func TestIsLoopbackOrigin(t *testing.T) {
	cases := map[string]bool{
		"http://localhost:5173":         true,
		"http://127.0.0.1:4290":         true,
		"http://[::1]:4290":             true,
		"https://localhost":             true,
		"http://app.localhost:3000":     false, // *.localhost not trusted
		"http://evil.com":               false,
		"https://evil.example":          false,
		"http://localhost.attacker.com": false,
		"http://2130706433":             false, // decimal IP — rejected
		"http://0177.0.0.1":             false, // octal IP — rejected
		"http://127.1":                  false, // short-form IP — rejected
		"null":                          false,
		"":                              false,
	}
	for origin, want := range cases {
		assert.Equalf(t, want, isLoopbackOrigin(origin), "isLoopbackOrigin(%q)", origin)
	}
}

func TestIsUntrustedBrowserRequest(t *testing.T) {
	mk := func(secFetchSite, origin string) *http.Request {
		req := httptest.NewRequest("GET", "/status", nil)
		if secFetchSite != "" {
			req.Header.Set("Sec-Fetch-Site", secFetchSite)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		return req
	}
	cases := []struct {
		name         string
		secFetchSite string
		origin       string
		want         bool
	}{
		{"no headers (cli)", "", "", false},
		{"same-origin", "same-origin", "http://localhost:4290", false},
		{"same-site", "same-site", "http://localhost:4290", false},
		{"user-initiated none", "none", "", false},
		{"cross-site loopback origin", "cross-site", "http://localhost:5173", false},
		{"cross-site foreign origin", "cross-site", "https://evil.example", true},
		{"legacy foreign origin", "", "https://evil.example", true},
		{"legacy loopback origin", "", "http://localhost:5173", false},
		{"same-origin foreign origin (forged)", "same-origin", "https://evil.example", true},
		{"same-site foreign origin (forged)", "same-site", "https://evil.example", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isUntrustedBrowserRequest(mk(tc.secFetchSite, tc.origin)))
		})
	}
}

// ----------------------------------------------------------------------------
// Issue 1 — admin CORS default no longer echoes arbitrary origins
// ----------------------------------------------------------------------------

func TestAdminCORS_Default_RejectsForeignOrigin(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	t.Cleanup(func() { api.Stop() })

	req := httptest.NewRequest("GET", "/status", nil)
	req.Header.Set("Origin", "https://evil.example")
	rec := serveAdmin(t, api, req)

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"),
		"foreign origins must not be echoed in Access-Control-Allow-Origin")
	assert.Contains(t, rec.Header().Values("Vary"), "Origin")
}

func TestAdminCORS_Default_EchoesLoopbackOrigin(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	t.Cleanup(func() { api.Stop() })

	req := httptest.NewRequest("GET", "/status", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	rec := serveAdmin(t, api, req)
	assert.Equal(t, "http://localhost:5173", rec.Header().Get("Access-Control-Allow-Origin"))

	reqNull := httptest.NewRequest("GET", "/status", nil)
	reqNull.Header.Set("Origin", "null")
	recNull := serveAdmin(t, api, reqNull)
	assert.Empty(t, recNull.Header().Get("Access-Control-Allow-Origin"),
		"the null origin must never be echoed")
}

func TestAdminCORS_Default_PreflightForeignOrigin_403(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	t.Cleanup(func() { api.Stop() })

	req := httptest.NewRequest("OPTIONS", "/mocks", nil)
	req.Header.Set("Origin", "https://evil.example")
	req.Header.Set("Access-Control-Request-Method", "GET")
	rec := serveAdmin(t, api, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"))
}

// ----------------------------------------------------------------------------
// Issue 2 — .json/.map no longer bypass auth
// ----------------------------------------------------------------------------

func TestExport_JSON_RequiresAuth_WhenAuthEnabled(t *testing.T) {
	api := authAPI(t, false) // localhost bypass OFF so auth is actually enforced
	for _, path := range []string{"/openapi.json", "/insomnia.json"} {
		t.Run("nokey "+path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			rec := serveAdmin(t, api, req)
			assert.Equal(t, http.StatusUnauthorized, rec.Code,
				"%s must require the API key when auth is enabled", path)
		})
		t.Run("withkey "+path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			req.RemoteAddr = "127.0.0.1:12345"
			req.Header.Set("X-API-Key", "test-api-key")
			rec := serveAdmin(t, api, req)
			assert.NotEqual(t, http.StatusUnauthorized, rec.Code,
				"%s must be reachable with a valid key", path)
		})
	}
}

func TestExport_YAML_StillRequiresAuth(t *testing.T) {
	api := authAPI(t, false)
	for _, path := range []string{"/openapi.yaml", "/insomnia.yaml"} {
		req := httptest.NewRequest("GET", path, nil)
		req.RemoteAddr = "127.0.0.1:12345"
		rec := serveAdmin(t, api, req)
		assert.Equalf(t, http.StatusUnauthorized, rec.Code,
			"%s must require auth (the .json twin now matches it)", path)
	}
}

func TestIsDashboardAsset(t *testing.T) {
	cases := map[string]bool{
		"/":                           true,
		"/index.html":                 true,
		"/manifest.json":              true,
		"/manifest.webmanifest":       true,
		"/assets/index-abc123.js":     true,
		"/assets/index-abc123.css":    true,
		"/logo.svg":                   true,
		"/favicon.ico":                true,
		"/fonts/inter.woff2":          true,
		"/openapi.json":               false,
		"/insomnia.json":              false,
		"/openapi.yaml":               false,
		"/assets/index-abc123.js.map": false,
		"/mocks":                      false,
		"/config":                     false,
		"/requests":                   false,
	}
	for path, want := range cases {
		assert.Equalf(t, want, isDashboardAsset(path), "isDashboardAsset(%q)", path)
	}
}
