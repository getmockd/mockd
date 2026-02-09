// Option functions for configuring API.

package admin

import (
	"time"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/ratelimit"
	"github.com/getmockd/mockd/pkg/tracing"
	"github.com/getmockd/mockd/pkg/workspace"
)

// Option configures an API.
type Option func(*API)

// WithLocalEngine configures the admin API to use an HTTP client
// to communicate with a local engine at the given URL.
func WithLocalEngine(url string) Option {
	return func(a *API) {
		a.localEngine.Store(engineclient.New(url))
	}
}

// WithLocalEngineClient configures the admin API to use the given engine client.
func WithLocalEngineClient(client *engineclient.Client) Option {
	return func(a *API) {
		a.localEngine.Store(client)
	}
}

// WithRegistrationTokenExpiration sets the expiration duration for registration tokens.
func WithRegistrationTokenExpiration(d time.Duration) Option {
	return func(a *API) {
		if d > 0 {
			a.registrationTokenExpiration = d
		}
	}
}

// WithEngineTokenExpiration sets the expiration duration for engine tokens.
func WithEngineTokenExpiration(d time.Duration) Option {
	return func(a *API) {
		if d > 0 {
			a.engineTokenExpiration = d
		}
	}
}

// WithRateLimiter configures a custom rate limiter for the admin API.
// If not set, a default rate limiter (100 req/s, burst 200) is used.
func WithRateLimiter(rl *ratelimit.PerIPLimiter) Option {
	return func(a *API) {
		a.rateLimiter = rl
	}
}

// WithCORS configures the CORS settings for the admin API.
// If not set, a default permissive configuration (allow all origins) is used.
func WithCORS(config CORSConfig) Option {
	return func(a *API) {
		a.corsConfig = config
	}
}

// WithTracer sets the tracer for distributed tracing.
// When set, tracing middleware will be applied to capture request spans.
func WithTracer(t *tracing.Tracer) Option {
	return func(a *API) {
		a.tracer = t
	}
}

// WithAPIKey sets a specific API key for authentication.
// If not set, a random key will be generated on startup.
func WithAPIKey(key string) Option {
	return func(a *API) {
		a.apiKeyConfig.Key = key
		a.apiKeyConfig.Enabled = true
	}
}

// WithAPIKeyConfig sets the full API key configuration.
func WithAPIKeyConfig(config APIKeyConfig) Option {
	return func(a *API) {
		a.apiKeyConfig = config
	}
}

// WithAPIKeyDisabled disables API key authentication entirely.
// WARNING: This makes the admin API accessible without any authentication.
func WithAPIKeyDisabled() Option {
	return func(a *API) {
		a.apiKeyConfig.Enabled = false
	}
}

// WithAPIKeyAllowLocalhost allows requests from localhost without API key.
// This is useful for development but should not be used in production.
func WithAPIKeyAllowLocalhost(allow bool) Option {
	return func(a *API) {
		a.apiKeyConfig.AllowLocalhost = allow
	}
}

// WithAPIKeyFilePath sets a custom path for storing/loading the API key.
func WithAPIKeyFilePath(path string) Option {
	return func(a *API) {
		a.apiKeyConfig.KeyFilePath = path
	}
}

// WithDataDir sets a custom data directory for the Admin API's persistent store.
// This allows test isolation by using separate data directories.
func WithDataDir(dir string) Option {
	return func(a *API) {
		a.dataDir = dir
	}
}

// WithVersion sets the version string returned by the status endpoint.
// If not set, defaults to "dev".
func WithVersion(version string) Option {
	return func(a *API) {
		a.version = version
	}
}

// WithAllowLocalhostBypass enables unauthenticated access from localhost.
// This is useful for development but should not be used in production.
// Default is false - authentication is always required.
func WithAllowLocalhostBypass(allow bool) Option {
	return func(a *API) {
		a.allowLocalhostBypass = allow
	}
}

// WithWorkspaceManager sets the workspace manager for multi-workspace serving.
// If not set, workspace server endpoints will return errors.
func WithWorkspaceManager(m workspace.Manager) Option {
	return func(a *API) {
		a.workspaceManager = m
	}
}
