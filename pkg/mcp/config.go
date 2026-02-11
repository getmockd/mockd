package mcp

import (
	"errors"
	"fmt"
	"time"
)

// Config holds MCP server configuration.
type Config struct {
	// Enabled controls whether MCP server is started.
	Enabled bool `json:"enabled"`

	// Port is the TCP port to listen on.
	Port int `json:"port"`

	// Path is the HTTP endpoint path (e.g., "/mcp").
	Path string `json:"path"`

	// AdminURL is the Admin API URL for mock operations.
	// If empty, defaults to "http://localhost:4290".
	AdminURL string `json:"adminUrl"`

	// AllowRemote allows connections from non-localhost addresses.
	// Default: false (localhost only for security).
	AllowRemote bool `json:"allowRemote"`

	// AllowedOrigins is a list of allowed Origin headers.
	// Supports wildcards like "http://localhost:*".
	// Default: ["*"] (allow all when localhost-only).
	AllowedOrigins []string `json:"allowedOrigins"`

	// SessionTimeout is the idle timeout for sessions.
	// Sessions are expired after this duration of inactivity.
	SessionTimeout time.Duration `json:"sessionTimeout"`

	// MaxSessions is the maximum number of concurrent sessions.
	MaxSessions int `json:"maxSessions"`

	// ReadTimeout is the HTTP read timeout.
	ReadTimeout time.Duration `json:"readTimeout"`

	// WriteTimeout is the HTTP write timeout.
	WriteTimeout time.Duration `json:"writeTimeout"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Enabled:        false,
		Port:           9091,
		Path:           "/mcp",
		AdminURL:       "http://localhost:4290",
		AllowRemote:    false,
		AllowedOrigins: []string{"*"},
		SessionTimeout: 30 * time.Minute,
		MaxSessions:    100,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
	}
}

// Validate validates the configuration and returns an error if invalid.
func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}

	if c.Path == "" {
		return errors.New("path cannot be empty")
	}

	if c.Path[0] != '/' {
		return fmt.Errorf("path must start with '/', got %q", c.Path)
	}

	if c.MaxSessions < 1 {
		return fmt.Errorf("maxSessions must be at least 1, got %d", c.MaxSessions)
	}

	if c.SessionTimeout < time.Second {
		return errors.New("sessionTimeout must be at least 1 second")
	}

	return nil
}

// Address returns the listen address for the HTTP server.
func (c *Config) Address() string {
	if c.AllowRemote {
		return fmt.Sprintf(":%d", c.Port)
	}
	return fmt.Sprintf("127.0.0.1:%d", c.Port)
}
