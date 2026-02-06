// Package config provides configuration types for mockd.
// This file contains the v1 configuration schema for the new Docker Compose-style architecture.
//
// The v1 schema is defined in the design doc: docs/10-config-architecture.md
// It replaces the older MockCollection-based configuration.

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// ============================================================================
// Project Config Types (mockd.yaml)
// ============================================================================

// ProjectConfig is the root configuration structure for mockd.yaml files.
type ProjectConfig struct {
	// Version is the config format version (required, currently "1.0")
	Version string `json:"version" yaml:"version"`

	// Admins defines control plane instances
	Admins []AdminConfig `json:"admins,omitempty" yaml:"admins,omitempty"`

	// Engines defines data plane instances
	Engines []EngineConfig `json:"engines,omitempty" yaml:"engines,omitempty"`

	// Workspaces defines logical mock groupings
	Workspaces []WorkspaceConfig `json:"workspaces,omitempty" yaml:"workspaces,omitempty"`

	// Mocks contains mock definitions (inline or file references)
	Mocks []MockEntry `json:"mocks,omitempty" yaml:"mocks,omitempty"`

	// StatefulResources defines CRUD resources
	StatefulResources []StatefulResourceConfig `json:"statefulResources,omitempty" yaml:"statefulResources,omitempty"`
}

// AdminConfig defines a control plane instance.
// If URL is empty, mockd starts a local admin; otherwise it connects to the remote admin.
type AdminConfig struct {
	// Name is the unique identifier for this admin (required)
	Name string `json:"name" yaml:"name"`

	// Port is the port for local admin (required if URL is empty)
	Port int `json:"port,omitempty" yaml:"port,omitempty"`

	// URL is the address of a remote admin (if set, mockd connects rather than starts)
	URL string `json:"url,omitempty" yaml:"url,omitempty"`

	// APIKey is the API key for authenticating with a remote admin
	APIKey string `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`

	// Auth configures authentication for local admin
	Auth *AdminAuthConfig `json:"auth,omitempty" yaml:"auth,omitempty"`

	// Persistence configures storage for local admin
	Persistence *AdminPersistenceConfig `json:"persistence,omitempty" yaml:"persistence,omitempty"`

	// TLS configures TLS settings for remote admin connections
	TLS *AdminTLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`
}

// IsLocal returns true if this admin should be started locally (no URL specified).
func (a AdminConfig) IsLocal() bool {
	return a.URL == ""
}

// AdminAuthConfig defines authentication settings for a local admin.
type AdminAuthConfig struct {
	// Type is the authentication type: "api-key" or "none"
	Type string `json:"type,omitempty" yaml:"type,omitempty"`

	// KeyFile is the path to the API key file (auto-generated if missing)
	KeyFile string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
}

// AdminPersistenceConfig defines storage settings for a local admin.
type AdminPersistenceConfig struct {
	// Type is the storage type: "sqlite" or "memory"
	Type string `json:"type,omitempty" yaml:"type,omitempty"`

	// Path is the path to the SQLite database file
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

// AdminTLSConfig defines TLS settings for connecting to a remote admin.
type AdminTLSConfig struct {
	// Insecure skips certificate verification (not recommended for production)
	Insecure bool `json:"insecure,omitempty" yaml:"insecure,omitempty"`

	// CAFile is the path to a custom CA certificate file
	CAFile string `json:"caFile,omitempty" yaml:"caFile,omitempty"`

	// CertFile is the path to a client certificate file (for mTLS)
	CertFile string `json:"certFile,omitempty" yaml:"certFile,omitempty"`

	// KeyFile is the path to a client key file (for mTLS)
	KeyFile string `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
}

// EngineConfig defines a data plane instance.
// If HTTPPort/GRPCPort are set, mockd starts a local engine that registers with the specified admin.
type EngineConfig struct {
	// Name is the unique identifier for this engine (required)
	Name string `json:"name" yaml:"name"`

	// HTTPPort is the port for HTTP mock serving (0 = disabled)
	HTTPPort int `json:"httpPort,omitempty" yaml:"httpPort,omitempty"`

	// HTTPSPort is the port for HTTPS mock serving (0 = disabled)
	HTTPSPort int `json:"httpsPort,omitempty" yaml:"httpsPort,omitempty"`

	// GRPCPort is the port for gRPC mock serving (0 = disabled)
	GRPCPort int `json:"grpcPort,omitempty" yaml:"grpcPort,omitempty"`

	// Admin is the name of the admin this engine registers with (required)
	Admin string `json:"admin" yaml:"admin"`

	// Registration configures engine registration with remote admins
	Registration *EngineRegistrationConfig `json:"registration,omitempty" yaml:"registration,omitempty"`

	// TLS configures TLS settings for the engine's HTTPS server
	TLS *TLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"`

	// Tunnel configures tunnel exposure for this engine
	Tunnel *TunnelYAMLConfig `json:"tunnel,omitempty" yaml:"tunnel,omitempty"`
}

// TunnelYAMLConfig is the tunnel section in mockd.yaml engine config.
type TunnelYAMLConfig struct {
	// Enabled enables tunnel for this engine (default: false)
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Relay is the relay server address (default: relay.mockd.io)
	Relay string `json:"relay,omitempty" yaml:"relay,omitempty"`

	// Token is the authentication token (supports ${ENV_VAR} syntax)
	Token string `json:"token,omitempty" yaml:"token,omitempty"`

	// Subdomain is a custom subdomain (Pro+)
	Subdomain string `json:"subdomain,omitempty" yaml:"subdomain,omitempty"`

	// Domain is a custom domain (Pro+)
	Domain string `json:"domain,omitempty" yaml:"domain,omitempty"`

	// Insecure skips TLS certificate verification (for local dev with mkcert)
	Insecure bool `json:"insecure,omitempty" yaml:"insecure,omitempty"`

	// Expose configures what to expose through the tunnel (nil = mode "all")
	Expose *TunnelYAMLExposure `json:"expose,omitempty" yaml:"expose,omitempty"`

	// Auth configures authentication for incoming tunnel requests
	Auth *TunnelYAMLAuth `json:"auth,omitempty" yaml:"auth,omitempty"`
}

// TunnelYAMLExposure defines exposure config in mockd.yaml.
type TunnelYAMLExposure struct {
	Mode       string             `json:"mode,omitempty" yaml:"mode,omitempty"` // "all", "selected", "none"
	Workspaces []string           `json:"workspaces,omitempty" yaml:"workspaces,omitempty"`
	Folders    []string           `json:"folders,omitempty" yaml:"folders,omitempty"`
	Mocks      []string           `json:"mocks,omitempty" yaml:"mocks,omitempty"`
	Types      []string           `json:"types,omitempty" yaml:"types,omitempty"`
	Exclude    *TunnelYAMLExclude `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

// TunnelYAMLExclude defines exclusion rules in mockd.yaml.
type TunnelYAMLExclude struct {
	Workspaces []string `json:"workspaces,omitempty" yaml:"workspaces,omitempty"`
	Folders    []string `json:"folders,omitempty" yaml:"folders,omitempty"`
	Mocks      []string `json:"mocks,omitempty" yaml:"mocks,omitempty"`
}

// TunnelYAMLAuth defines auth config in mockd.yaml.
type TunnelYAMLAuth struct {
	Type       string   `json:"type,omitempty" yaml:"type,omitempty"` // "none","token","basic","ip"
	Token      string   `json:"token,omitempty" yaml:"token,omitempty"`
	Username   string   `json:"username,omitempty" yaml:"username,omitempty"`
	Password   string   `json:"password,omitempty" yaml:"password,omitempty"`
	AllowedIPs []string `json:"allowedIPs,omitempty" yaml:"allowedIPs,omitempty"`
}

// EngineRegistrationConfig defines registration settings for engines connecting to remote admins.
type EngineRegistrationConfig struct {
	// Token is the registration token for authenticating with the admin
	Token string `json:"token,omitempty" yaml:"token,omitempty"`

	// Fingerprint is the unique identity for this engine ("auto" or explicit ID)
	// If "auto" or empty, a fingerprint is generated based on machine identity.
	Fingerprint string `json:"fingerprint,omitempty" yaml:"fingerprint,omitempty"`
}

// WorkspaceConfig defines a logical grouping of mocks.
type WorkspaceConfig struct {
	// Name is the unique identifier for this workspace (required)
	Name string `json:"name" yaml:"name"`

	// Description is a human-readable description
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Engines is a list of engine names this workspace is assigned to
	Engines []string `json:"engines,omitempty" yaml:"engines,omitempty"`
}

// MockEntry represents either an inline mock definition, a file reference, or a glob pattern.
// Only one of ID/Type/HTTP, File, or Files should be set.
type MockEntry struct {
	// Inline mock fields (when defining a mock directly)
	ID        string          `json:"id,omitempty" yaml:"id,omitempty"`
	Workspace string          `json:"workspace,omitempty" yaml:"workspace,omitempty"`
	Type      string          `json:"type,omitempty" yaml:"type,omitempty"` // "http", "grpc", etc.
	HTTP      *HTTPMockConfig `json:"http,omitempty" yaml:"http,omitempty"`

	// File reference (loads mocks from a single file)
	File string `json:"file,omitempty" yaml:"file,omitempty"`

	// Files glob pattern (loads mocks from multiple files)
	Files string `json:"files,omitempty" yaml:"files,omitempty"`
}

// IsInline returns true if this is an inline mock definition.
func (m MockEntry) IsInline() bool {
	return m.ID != "" || m.Type != ""
}

// IsFileRef returns true if this is a single file reference.
func (m MockEntry) IsFileRef() bool {
	return m.File != ""
}

// IsGlob returns true if this is a glob pattern for multiple files.
func (m MockEntry) IsGlob() bool {
	return m.Files != ""
}

// HTTPMockConfig defines an HTTP mock within a MockEntry.
type HTTPMockConfig struct {
	// Matcher defines request matching rules
	Matcher HTTPMatcher `json:"matcher" yaml:"matcher"`

	// Response defines the mock response
	Response HTTPResponse `json:"response" yaml:"response"`
}

// HTTPMatcher defines rules for matching HTTP requests.
type HTTPMatcher struct {
	// Method is the HTTP method to match (GET, POST, etc.)
	Method string `json:"method,omitempty" yaml:"method,omitempty"`

	// Path is the URL path to match (supports path parameters like /users/{id})
	Path string `json:"path,omitempty" yaml:"path,omitempty"`

	// PathPattern is a regex pattern for path matching
	PathPattern string `json:"pathPattern,omitempty" yaml:"pathPattern,omitempty"`

	// Headers are required headers to match
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`

	// QueryParams are required query parameters to match
	QueryParams map[string]string `json:"queryParams,omitempty" yaml:"queryParams,omitempty"`
}

// HTTPResponse defines a mock HTTP response.
type HTTPResponse struct {
	// StatusCode is the HTTP status code (default: 200)
	StatusCode int `json:"statusCode,omitempty" yaml:"statusCode,omitempty"`

	// Headers are response headers
	Headers map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`

	// Body is the response body (supports templating)
	Body string `json:"body,omitempty" yaml:"body,omitempty"`

	// BodyFile is a path to a file containing the response body
	BodyFile string `json:"bodyFile,omitempty" yaml:"bodyFile,omitempty"`

	// Delay adds a fixed delay before responding (e.g., "100ms", "1s")
	Delay string `json:"delay,omitempty" yaml:"delay,omitempty"`
}

// StatefulResourceEntry is an alias for StatefulResourceConfig for backward compatibility.
// Deprecated: Use StatefulResourceConfig directly.
type StatefulResourceEntry = StatefulResourceConfig

// CLIContextConfig is the structure for ~/.config/mockd/contexts.yaml.
type CLIContextConfig struct {
	// Version is the config format version
	Version string `json:"version" yaml:"version"`

	// Current is the name of the currently active context
	Current string `json:"current" yaml:"current"`

	// Contexts is a map of context name to context configuration
	Contexts map[string]CLIContext `json:"contexts" yaml:"contexts"`
}

// CLIContext defines a single CLI context (connection to an admin).
type CLIContext struct {
	// AdminURL is the URL of the admin server
	AdminURL string `json:"adminUrl" yaml:"adminUrl"`

	// APIKey is the API key for authentication (can use ${ENV_VAR} syntax)
	APIKey string `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`

	// Workspace is the default workspace for this context
	Workspace string `json:"workspace,omitempty" yaml:"workspace,omitempty"`
}

// PIDFile is the structure written to ~/.mockd/mockd.pid when running in detached mode.
type PIDFile struct {
	// PID is the main process ID
	PID int `json:"pid"`

	// StartedAt is when the services were started
	StartedAt string `json:"startedAt"`

	// Config is the path to the config file used
	Config string `json:"config"`

	// Services lists all running services
	Services []PIDFileService `json:"services"`
}

// PIDFileService describes a single running service in the PID file.
type PIDFileService struct {
	// Name is the service name
	Name string `json:"name"`

	// Type is "admin" or "engine"
	Type string `json:"type"`

	// Port is the primary port the service listens on
	Port int `json:"port"`

	// PID is the process ID
	PID int `json:"pid"`
}

// DefaultProjectConfig returns a ProjectConfig with sensible defaults for a minimal local setup.
func DefaultProjectConfig() *ProjectConfig {
	return &ProjectConfig{
		Version: "1.0",
		Admins: []AdminConfig{
			{
				Name: "local",
				Port: 4290,
			},
		},
		Engines: []EngineConfig{
			{
				Name:     "default",
				HTTPPort: 4280,
				Admin:    "local",
			},
		},
		Workspaces: []WorkspaceConfig{
			{
				Name:        "default",
				Description: "Default workspace",
				Engines:     []string{"default"},
			},
		},
	}
}

// ============================================================================
// Project Config Loader
// ============================================================================

// ProjectConfigDiscoveryOrder defines the priority order for finding config files.
var ProjectConfigDiscoveryOrder = []string{
	"mockd.yaml",
	"mockd.yml",
}

// envVarPattern matches ${VAR_NAME} or ${VAR_NAME:-default}
var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

// LoadProjectConfig loads a config from the given path, applying environment variable substitution.
// If path is empty, it tries to discover a config file in the current directory.
func LoadProjectConfig(path string) (*ProjectConfig, error) {
	// Discover config file if not specified
	if path == "" {
		discovered, err := DiscoverProjectConfig()
		if err != nil {
			return nil, err
		}
		path = discovered
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Apply environment variable substitution
	expanded := ExpandEnvVars(string(data))

	// Parse YAML
	var cfg ProjectConfig
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return &cfg, nil
}

// LoadProjectConfigFromBytes loads a config from raw bytes, applying environment variable substitution.
func LoadProjectConfigFromBytes(data []byte) (*ProjectConfig, error) {
	// Apply environment variable substitution
	expanded := ExpandEnvVars(string(data))

	// Parse YAML
	var cfg ProjectConfig
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return &cfg, nil
}

// DiscoverProjectConfig finds a config file in the current directory or via MOCKD_CONFIG env var.
// Returns the path to the config file, or an error if none is found.
func DiscoverProjectConfig() (string, error) {
	// Check MOCKD_CONFIG env var first
	if envPath := os.Getenv("MOCKD_CONFIG"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath, nil
		}
		return "", fmt.Errorf("MOCKD_CONFIG points to non-existent file: %s", envPath)
	}

	// Try discovery order in current directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting current directory: %w", err)
	}

	for _, name := range ProjectConfigDiscoveryOrder {
		path := filepath.Join(cwd, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("no config found. Run 'mockd init' to create one, or specify --config")
}

// ExpandEnvVars expands environment variables in the input string.
// Supports ${VAR_NAME} and ${VAR_NAME:-default} syntax.
func ExpandEnvVars(input string) string {
	return envVarPattern.ReplaceAllStringFunc(input, func(match string) string {
		// Parse the match
		submatch := envVarPattern.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}

		varName := submatch[1]
		defaultVal := ""
		if len(submatch) >= 3 {
			defaultVal = submatch[2]
		}

		// Get environment variable
		if val := os.Getenv(varName); val != "" {
			return val
		}

		// Return default if specified
		return defaultVal
	})
}

// MergeProjectConfigs merges multiple configs together.
// Later configs override earlier ones. Arrays merge by name/id key.
func MergeProjectConfigs(configs ...*ProjectConfig) *ProjectConfig {
	if len(configs) == 0 {
		return nil
	}

	result := &ProjectConfig{
		Version:           "1.0",
		Admins:            []AdminConfig{},
		Engines:           []EngineConfig{},
		Workspaces:        []WorkspaceConfig{},
		Mocks:             []MockEntry{},
		StatefulResources: []StatefulResourceConfig{},
	}

	for _, cfg := range configs {
		if cfg == nil {
			continue
		}

		// Use latest version
		if cfg.Version != "" {
			result.Version = cfg.Version
		}

		// Merge admins by name
		result.Admins = mergeAdmins(result.Admins, cfg.Admins)

		// Merge engines by name
		result.Engines = mergeEngines(result.Engines, cfg.Engines)

		// Merge workspaces by name
		result.Workspaces = mergeWorkspaces(result.Workspaces, cfg.Workspaces)

		// Merge mocks by id (or append if file/files ref)
		result.Mocks = mergeMocks(result.Mocks, cfg.Mocks)

		// Merge stateful resources by name
		result.StatefulResources = mergeStatefulResources(result.StatefulResources, cfg.StatefulResources)
	}

	return result
}

func mergeAdmins(base, overlay []AdminConfig) []AdminConfig {
	byName := make(map[string]int)
	for i, a := range base {
		byName[a.Name] = i
	}

	for _, a := range overlay {
		if idx, exists := byName[a.Name]; exists {
			// Merge into existing
			base[idx] = mergeAdmin(base[idx], a)
		} else {
			// Add new
			base = append(base, a)
			byName[a.Name] = len(base) - 1
		}
	}

	return base
}

func mergeAdmin(base, overlay AdminConfig) AdminConfig {
	if overlay.Port != 0 {
		base.Port = overlay.Port
	}
	if overlay.URL != "" {
		base.URL = overlay.URL
	}
	if overlay.APIKey != "" {
		base.APIKey = overlay.APIKey
	}
	if overlay.Auth != nil {
		base.Auth = overlay.Auth
	}
	if overlay.Persistence != nil {
		base.Persistence = overlay.Persistence
	}
	if overlay.TLS != nil {
		base.TLS = overlay.TLS
	}
	return base
}

func mergeEngines(base, overlay []EngineConfig) []EngineConfig {
	byName := make(map[string]int)
	for i, e := range base {
		byName[e.Name] = i
	}

	for _, e := range overlay {
		if idx, exists := byName[e.Name]; exists {
			base[idx] = mergeEngine(base[idx], e)
		} else {
			base = append(base, e)
			byName[e.Name] = len(base) - 1
		}
	}

	return base
}

func mergeEngine(base, overlay EngineConfig) EngineConfig {
	if overlay.HTTPPort != 0 {
		base.HTTPPort = overlay.HTTPPort
	}
	if overlay.HTTPSPort != 0 {
		base.HTTPSPort = overlay.HTTPSPort
	}
	if overlay.GRPCPort != 0 {
		base.GRPCPort = overlay.GRPCPort
	}
	if overlay.Admin != "" {
		base.Admin = overlay.Admin
	}
	if overlay.Registration != nil {
		base.Registration = overlay.Registration
	}
	if overlay.TLS != nil {
		base.TLS = overlay.TLS
	}
	if overlay.Tunnel != nil {
		base.Tunnel = overlay.Tunnel
	}
	return base
}

func mergeWorkspaces(base, overlay []WorkspaceConfig) []WorkspaceConfig {
	byName := make(map[string]int)
	for i, w := range base {
		byName[w.Name] = i
	}

	for _, w := range overlay {
		if idx, exists := byName[w.Name]; exists {
			base[idx] = mergeWorkspace(base[idx], w)
		} else {
			base = append(base, w)
			byName[w.Name] = len(base) - 1
		}
	}

	return base
}

func mergeWorkspace(base, overlay WorkspaceConfig) WorkspaceConfig {
	if overlay.Description != "" {
		base.Description = overlay.Description
	}
	if len(overlay.Engines) > 0 {
		base.Engines = overlay.Engines
	}
	return base
}

func mergeMocks(base, overlay []MockEntry) []MockEntry {
	byID := make(map[string]int)
	for i, m := range base {
		if m.ID != "" {
			byID[m.ID] = i
		}
	}

	for _, m := range overlay {
		if m.ID != "" {
			if idx, exists := byID[m.ID]; exists {
				// Replace by ID
				base[idx] = m
			} else {
				base = append(base, m)
				byID[m.ID] = len(base) - 1
			}
		} else {
			// File refs are always appended
			base = append(base, m)
		}
	}

	return base
}

func mergeStatefulResources(base, overlay []StatefulResourceConfig) []StatefulResourceConfig {
	byName := make(map[string]int)
	for i, r := range base {
		byName[r.Name] = i
	}

	for _, r := range overlay {
		if idx, exists := byName[r.Name]; exists {
			base[idx] = mergeStatefulResource(base[idx], r)
		} else {
			base = append(base, r)
			byName[r.Name] = len(base) - 1
		}
	}

	return base
}

func mergeStatefulResource(base, overlay StatefulResourceConfig) StatefulResourceConfig {
	if overlay.Workspace != "" {
		base.Workspace = overlay.Workspace
	}
	if overlay.BasePath != "" {
		base.BasePath = overlay.BasePath
	}
	if overlay.IDField != "" {
		base.IDField = overlay.IDField
	}
	if overlay.ParentField != "" {
		base.ParentField = overlay.ParentField
	}
	if len(overlay.SeedData) > 0 {
		base.SeedData = overlay.SeedData
	}
	if overlay.Validation != nil {
		base.Validation = overlay.Validation
	}
	return base
}

// LoadAndMergeProjectConfigs loads multiple config files and merges them together.
// Files are loaded in order, with later files overriding earlier ones.
func LoadAndMergeProjectConfigs(paths []string) (*ProjectConfig, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("no config files specified")
	}

	var configs []*ProjectConfig
	for _, path := range paths {
		cfg, err := LoadProjectConfig(path)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", path, err)
		}
		configs = append(configs, cfg)
	}

	return MergeProjectConfigs(configs...), nil
}

// ResolvePath resolves a potentially relative path against a base directory.
func ResolvePath(basePath, targetPath string) string {
	if filepath.IsAbs(targetPath) {
		return targetPath
	}
	// Handle ~ expansion
	if strings.HasPrefix(targetPath, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, targetPath[2:])
		}
	}
	return filepath.Join(basePath, targetPath)
}
