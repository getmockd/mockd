package tunnel

import (
	"errors"
	"time"
)

// Default configuration values.
const (
	DefaultRelayURL          = "wss://relay.mockd.io/tunnel"
	DefaultReconnectDelay    = 1 * time.Second
	DefaultMaxReconnectDelay = 30 * time.Second
	DefaultPingInterval      = 30 * time.Second
	DefaultRequestTimeout    = 30 * time.Second
)

// AuthConfig holds authentication configuration for incoming tunnel requests.
type AuthConfig struct {
	// Type is the authentication type: none, token, basic, ip
	Type string

	// Token is the required token for token auth.
	Token string

	// Username is the username for basic auth.
	Username string

	// Password is the password for basic auth.
	Password string

	// AllowedIPs is a list of allowed IPs/CIDRs for IP auth.
	AllowedIPs []string
}

// Config holds tunnel client configuration.
type Config struct {
	// RelayURL is the WebSocket URL of the relay server.
	RelayURL string

	// Token is the JWT authentication token.
	Token string

	// Subdomain is the requested subdomain (e.g., "my-api").
	// If empty, the relay will auto-assign based on username.
	Subdomain string

	// CustomDomain is an optional verified custom domain to use.
	// If set, this takes precedence over Subdomain.
	CustomDomain string

	// Auth is the authentication configuration for incoming requests.
	// If nil, no authentication is required.
	Auth *AuthConfig

	// ReconnectDelay is the initial delay before reconnecting after disconnect.
	ReconnectDelay time.Duration

	// MaxReconnectDelay is the maximum delay between reconnect attempts.
	MaxReconnectDelay time.Duration

	// PingInterval is the interval between ping messages.
	PingInterval time.Duration

	// RequestTimeout is the timeout for forwarding requests to the local engine.
	RequestTimeout time.Duration

	// AutoReconnect enables automatic reconnection on disconnect.
	AutoReconnect bool

	// OnConnect is called when the tunnel connects.
	OnConnect func(publicURL string)

	// OnDisconnect is called when the tunnel disconnects.
	OnDisconnect func(err error)

	// OnRequest is called for each request received through the tunnel.
	OnRequest func(method, path string)

	// ClientVersion is the client version string.
	ClientVersion string
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		RelayURL:          DefaultRelayURL,
		ReconnectDelay:    DefaultReconnectDelay,
		MaxReconnectDelay: DefaultMaxReconnectDelay,
		PingInterval:      DefaultPingInterval,
		RequestTimeout:    DefaultRequestTimeout,
		AutoReconnect:     true,
		ClientVersion:     "0.2.0",
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.RelayURL == "" {
		return errors.New("RelayURL is required")
	}
	if c.Token == "" {
		return errors.New("Token is required")
	}
	return nil
}

// WithToken returns a copy of the config with the token set.
func (c *Config) WithToken(token string) *Config {
	c.Token = token
	return c
}

// WithSubdomain returns a copy of the config with the subdomain set.
func (c *Config) WithSubdomain(subdomain string) *Config {
	c.Subdomain = subdomain
	return c
}

// WithCustomDomain returns a copy of the config with the custom domain set.
func (c *Config) WithCustomDomain(domain string) *Config {
	c.CustomDomain = domain
	return c
}

// WithRelayURL returns a copy of the config with the relay URL set.
func (c *Config) WithRelayURL(url string) *Config {
	c.RelayURL = url
	return c
}

// WithAuth returns a copy of the config with authentication settings.
func (c *Config) WithAuth(auth *AuthConfig) *Config {
	c.Auth = auth
	return c
}

// WithTokenAuth returns a copy of the config with token authentication.
func (c *Config) WithTokenAuth(token string) *Config {
	c.Auth = &AuthConfig{
		Type:  "token",
		Token: token,
	}
	return c
}

// WithBasicAuth returns a copy of the config with basic authentication.
func (c *Config) WithBasicAuth(username, password string) *Config {
	c.Auth = &AuthConfig{
		Type:     "basic",
		Username: username,
		Password: password,
	}
	return c
}

// WithIPAuth returns a copy of the config with IP whitelist authentication.
func (c *Config) WithIPAuth(allowedIPs []string) *Config {
	c.Auth = &AuthConfig{
		Type:       "ip",
		AllowedIPs: allowedIPs,
	}
	return c
}
