package cliconfig

import (
	"os"
	"path/filepath"
	"strings"
)

// API key file configuration
const (
	// DefaultKeyFileName is the default file name for storing the API key.
	DefaultKeyFileName = "admin-api-key"
)

// GetAPIKeyFilePath returns the default path for the API key file.
// Location: $XDG_DATA_HOME/mockd/admin-api-key (or ~/.local/share/mockd/admin-api-key)
func GetAPIKeyFilePath() string {
	dataDir := os.Getenv("XDG_DATA_HOME")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dataDir, "mockd", DefaultKeyFileName)
}

// LoadAPIKeyFromFile loads the API key from the default file location.
// Returns the key and nil error if successful, empty string if file doesn't exist.
func LoadAPIKeyFromFile() (string, error) {
	return LoadAPIKeyFromPath(GetAPIKeyFilePath())
}

// LoadAPIKeyFromPath loads the API key from a specific file path.
func LoadAPIKeyFromPath(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil // File doesn't exist, not an error
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// GetAPIKey returns the API key, checking sources in priority order:
// 1. Environment variable (MOCKD_API_KEY)
// 2. Current context's auth token
// 3. Local API key file (~/.local/share/mockd/admin-api-key)
// 4. Empty string (no auth)
func GetAPIKey() string {
	// 1. Environment variable (highest priority)
	if key := GetAPIKeyFromEnv(); key != "" {
		return key
	}

	// 2. Context auth token
	cfg, err := LoadContextConfig()
	if err == nil {
		if ctx := cfg.GetCurrentContext(); ctx != nil && ctx.AuthToken != "" {
			return ctx.AuthToken
		}
	}

	// 3. Local API key file
	if key, err := LoadAPIKeyFromFile(); err == nil && key != "" {
		return key
	}

	return ""
}

// ClientConfig holds resolved configuration for creating an admin client.
// This is the single source of truth for CLI commands needing to connect.
type ClientConfig struct {
	// AdminURL is the resolved admin API URL
	AdminURL string

	// APIKey is the resolved API key (may be empty if auth disabled)
	APIKey string

	// Workspace is the resolved workspace ID (may be empty)
	Workspace string

	// TLSInsecure skips TLS verification (from context config)
	TLSInsecure bool
}

// ResolveClientConfig resolves all client configuration from various sources.
// Pass empty strings for flag values that weren't provided.
// Priority for each field: flag > env > context > config > default
func ResolveClientConfig(flagAdminURL, flagWorkspace, flagContext string) *ClientConfig {
	// Resolve context first (affects other lookups)
	contextName := ResolveContext(flagContext)
	ctx := GetContextByName(contextName)

	cfg := &ClientConfig{}

	// Resolve admin URL
	cfg.AdminURL = ResolveAdminURLWithContext(flagAdminURL, flagContext)

	// Resolve workspace
	cfg.Workspace = ResolveWorkspaceWithContext(flagWorkspace, flagContext)

	// Resolve API key (uses same priority: env > context > file)
	cfg.APIKey = GetAPIKey()

	// TLS settings from context
	if ctx != nil {
		cfg.TLSInsecure = ctx.TLSInsecure
	}

	return cfg
}

// ResolveClientConfigSimple is a convenience function when you only have admin URL flag.
// Most CLI commands use this.
func ResolveClientConfigSimple(flagAdminURL string) *ClientConfig {
	return ResolveClientConfig(flagAdminURL, "", "")
}
