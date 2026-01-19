package cliconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ContextConfigFileName is the name of the context configuration file.
const ContextConfigFileName = "contexts.json"

// ContextConfigVersion is the current version of the context config schema.
const ContextConfigVersion = 1

// DefaultContextName is the name of the default context.
const DefaultContextName = "local"

// ContextConfig holds the user's context configuration (admin servers + workspaces).
// This is stored separately from CLIConfig to keep context/workspace selection
// independent from server settings.
type ContextConfig struct {
	// Version is the config schema version for future migrations
	Version int `json:"version"`

	// CurrentContext is the name of the currently active context
	CurrentContext string `json:"currentContext"`

	// Contexts maps context names to their configuration
	Contexts map[string]*Context `json:"contexts"`
}

// Context represents a named admin server + workspace pair.
// Similar to kubectl contexts - allows quick switching between different
// mockd deployments (local, staging, CI, etc.)
type Context struct {
	// AdminURL is the base URL of the admin API (e.g., "http://localhost:4290")
	AdminURL string `json:"adminUrl"`

	// Workspace is the current workspace ID (empty = no workspace filtering)
	Workspace string `json:"workspace,omitempty"`

	// Description is an optional human-readable description
	Description string `json:"description,omitempty"`

	// AuthToken is an optional authentication token for cloud/enterprise deployments
	AuthToken string `json:"authToken,omitempty"`

	// TLSInsecure skips TLS certificate verification (for self-signed certs)
	TLSInsecure bool `json:"tlsInsecure,omitempty"`
}

// NewDefaultContextConfig creates a new ContextConfig with default values.
func NewDefaultContextConfig() *ContextConfig {
	return &ContextConfig{
		Version:        ContextConfigVersion,
		CurrentContext: DefaultContextName,
		Contexts: map[string]*Context{
			DefaultContextName: {
				AdminURL:    DefaultAdminURL(DefaultAdminPort),
				Description: "Local mockd server",
			},
		},
	}
}

// GetContextConfigPath returns the path to the context config file.
func GetContextConfigPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine config directory: %w", err)
	}
	return filepath.Join(configDir, GlobalConfigDir, ContextConfigFileName), nil
}

// LoadContextConfig loads the context configuration from disk.
// If the file doesn't exist, returns a default configuration.
func LoadContextConfig() (*ContextConfig, error) {
	path, err := GetContextConfigPath()
	if err != nil {
		return NewDefaultContextConfig(), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return NewDefaultContextConfig(), nil
		}
		return nil, fmt.Errorf("failed to read context config: %w", err)
	}

	var cfg ContextConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, &ConfigError{
			Path:    path,
			Message: fmt.Sprintf("invalid JSON: %s", err.Error()),
		}
	}

	// Initialize map if nil
	if cfg.Contexts == nil {
		cfg.Contexts = make(map[string]*Context)
	}

	// Ensure default context exists
	if _, exists := cfg.Contexts[DefaultContextName]; !exists && len(cfg.Contexts) == 0 {
		cfg.Contexts[DefaultContextName] = &Context{
			AdminURL:    DefaultAdminURL(DefaultAdminPort),
			Description: "Local mockd server",
		}
		if cfg.CurrentContext == "" {
			cfg.CurrentContext = DefaultContextName
		}
	}

	return &cfg, nil
}

// SaveContextConfig saves the context configuration to disk.
func SaveContextConfig(cfg *ContextConfig) error {
	path, err := GetContextConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode context config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write context config: %w", err)
	}

	return nil
}

// GetCurrentContext returns the currently active context.
// Returns nil if no context is set or the context doesn't exist.
func (c *ContextConfig) GetCurrentContext() *Context {
	if c.CurrentContext == "" {
		return nil
	}
	return c.Contexts[c.CurrentContext]
}

// SetCurrentContext switches to the named context.
// Returns an error if the context doesn't exist.
func (c *ContextConfig) SetCurrentContext(name string) error {
	if _, exists := c.Contexts[name]; !exists {
		return fmt.Errorf("context not found: %s", name)
	}
	c.CurrentContext = name
	return nil
}

// AddContext adds a new context with the given name.
// Returns an error if a context with that name already exists.
func (c *ContextConfig) AddContext(name string, ctx *Context) error {
	if _, exists := c.Contexts[name]; exists {
		return fmt.Errorf("context already exists: %s", name)
	}
	if c.Contexts == nil {
		c.Contexts = make(map[string]*Context)
	}
	c.Contexts[name] = ctx
	return nil
}

// RemoveContext removes a context by name.
// Returns an error if the context doesn't exist or is the current context.
func (c *ContextConfig) RemoveContext(name string) error {
	if _, exists := c.Contexts[name]; !exists {
		return fmt.Errorf("context not found: %s", name)
	}
	if c.CurrentContext == name {
		return errors.New("cannot remove current context; switch to another context first")
	}
	delete(c.Contexts, name)
	return nil
}

// SetWorkspace sets the workspace for the current context.
func (c *ContextConfig) SetWorkspace(workspace string) error {
	ctx := c.GetCurrentContext()
	if ctx == nil {
		return errors.New("no current context set")
	}
	ctx.Workspace = workspace
	return nil
}

// GetAdminURL returns the admin URL for the current context.
// Falls back to default if no context is set.
func GetAdminURLFromContext() string {
	cfg, err := LoadContextConfig()
	if err != nil {
		return DefaultAdminURL(DefaultAdminPort)
	}

	ctx := cfg.GetCurrentContext()
	if ctx == nil || ctx.AdminURL == "" {
		return DefaultAdminURL(DefaultAdminPort)
	}

	return ctx.AdminURL
}

// GetWorkspaceFromContext returns the workspace for the current context.
// Returns empty string if no workspace is set.
func GetWorkspaceFromContext() string {
	cfg, err := LoadContextConfig()
	if err != nil {
		return ""
	}

	ctx := cfg.GetCurrentContext()
	if ctx == nil {
		return ""
	}

	return ctx.Workspace
}

// GetWorkspace returns the workspace, checking sources in priority order:
// 1. Environment variable (MOCKD_WORKSPACE)
// 2. Context config (current context's workspace)
// 3. Empty string (no workspace filtering)
func GetWorkspace() string {
	// First check environment variable
	if ws := GetWorkspaceFromEnv(); ws != "" {
		return ws
	}

	// Then check context config
	return GetWorkspaceFromContext()
}

// GetAdminURL returns the admin URL, checking sources in priority order:
// 1. Environment variable (MOCKD_ADMIN_URL)
// 2. Context config (current context's adminUrl)
// 3. CLI config (adminUrl from config files)
// 4. Default value
// This is the primary function other packages should use.
func GetAdminURL() string {
	// First check environment variable (highest priority for ad-hoc overrides)
	if url := GetAdminURLFromEnv(); url != "" {
		return url
	}

	// Then check context config
	if url := GetAdminURLFromContext(); url != "" {
		return url
	}

	// Fall back to CLI config
	cfg, err := LoadAll()
	if err != nil {
		return DefaultAdminURL(DefaultAdminPort)
	}

	if cfg.AdminURL != "" {
		return cfg.AdminURL
	}

	return DefaultAdminURL(DefaultAdminPort)
}

// ResolveAdminURL resolves the admin URL from various sources.
// Priority: explicit flag > context > config > default
func ResolveAdminURL(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return GetAdminURL()
}

// ResolveWorkspace resolves the workspace from various sources.
// Priority: explicit flag > env var > context > empty
func ResolveWorkspace(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return GetWorkspace()
}

// ResolveContext resolves which context to use.
// Priority: explicit flag > env var > current context
func ResolveContext(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envCtx := GetContextFromEnv(); envCtx != "" {
		return envCtx
	}
	cfg, err := LoadContextConfig()
	if err != nil {
		return DefaultContextName
	}
	return cfg.CurrentContext
}

// GetContextByName returns a specific context by name.
// Returns nil if not found.
func GetContextByName(name string) *Context {
	cfg, err := LoadContextConfig()
	if err != nil {
		return nil
	}
	return cfg.Contexts[name]
}

// ResolveAdminURLWithContext resolves admin URL considering context override.
// Priority: flag > env > specified context > current context > default
func ResolveAdminURLWithContext(flagAdminURL, flagContext string) string {
	if flagAdminURL != "" {
		return flagAdminURL
	}
	if url := GetAdminURLFromEnv(); url != "" {
		return url
	}

	// If context flag specified, use that context
	contextName := ResolveContext(flagContext)
	if ctx := GetContextByName(contextName); ctx != nil && ctx.AdminURL != "" {
		return ctx.AdminURL
	}

	return DefaultAdminURL(DefaultAdminPort)
}

// ResolveWorkspaceWithContext resolves workspace considering context override.
// Priority: flag > env > specified context > current context > empty
func ResolveWorkspaceWithContext(flagWorkspace, flagContext string) string {
	if flagWorkspace != "" {
		return flagWorkspace
	}
	if ws := GetWorkspaceFromEnv(); ws != "" {
		return ws
	}

	// If context flag specified, use that context's workspace
	contextName := ResolveContext(flagContext)
	if ctx := GetContextByName(contextName); ctx != nil {
		return ctx.Workspace
	}

	return ""
}
