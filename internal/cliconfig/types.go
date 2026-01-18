// Package cliconfig provides configuration types and loading for the mockd CLI.
package cliconfig

// CLIConfig represents the complete configuration for the mockd CLI.
// Configuration values can come from multiple sources with the following precedence:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. Local config file (.mockdrc.json in current directory)
// 4. Global config file (~/.config/mockd/config.json)
// 5. Default values (lowest priority)
type CLIConfig struct {
	// Server settings
	Port         int    `json:"port"`
	AdminPort    int    `json:"adminPort"`
	HTTPSPort    int    `json:"httpsPort"`
	ConfigFile   string `json:"configFile,omitempty"`
	ReadTimeout  int    `json:"readTimeout"`
	WriteTimeout int    `json:"writeTimeout"`

	// Admin client settings
	AdminURL string `json:"adminUrl"`

	// Logging settings
	MaxLogEntries int `json:"maxLogEntries"`

	// TLS settings
	AutoCert bool   `json:"autoCert"`
	CertFile string `json:"certFile,omitempty"`
	KeyFile  string `json:"keyFile,omitempty"`

	// Output settings
	Verbose bool `json:"verbose"`
	JSON    bool `json:"json"`

	// Source tracks where each value came from (for debugging)
	Sources map[string]string `json:"-"`
}

// ConfigSource identifies where a config value originated.
const (
	SourceDefault = "default"
	SourceEnv     = "env"
	SourceGlobal  = "global"
	SourceLocal   = "local"
	SourceFlag    = "flag"
)
