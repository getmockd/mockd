// Package cliconfig provides configuration types and loading for the mockd CLI.
package cliconfig

// CLIConfig represents the complete configuration for the mockd CLI.
// Configuration values can come from multiple sources with the following precedence:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. Local config file (.mockdrc.yaml in current directory)
// 4. Global config file (~/.config/mockd/config.yaml)
// 5. Default values (lowest priority)
type CLIConfig struct {
	// Server settings
	Port         int    `yaml:"port" json:"port"`
	AdminPort    int    `yaml:"adminPort" json:"adminPort"`
	HTTPSPort    int    `yaml:"httpsPort" json:"httpsPort"`
	ConfigFile   string `yaml:"configFile,omitempty" json:"configFile,omitempty"`
	ReadTimeout  int    `yaml:"readTimeout" json:"readTimeout"`
	WriteTimeout int    `yaml:"writeTimeout" json:"writeTimeout"`

	// Admin client settings
	AdminURL string `yaml:"adminUrl" json:"adminUrl"`

	// Logging settings
	MaxLogEntries int `yaml:"maxLogEntries" json:"maxLogEntries"`

	// TLS settings
	AutoCert bool   `yaml:"autoCert" json:"autoCert"`
	CertFile string `yaml:"certFile,omitempty" json:"certFile,omitempty"`
	KeyFile  string `yaml:"keyFile,omitempty" json:"keyFile,omitempty"`

	// Output settings
	Verbose bool `yaml:"verbose" json:"verbose"`
	JSON    bool `yaml:"json" json:"json"`

	// Source tracks where each value came from (for debugging)
	Sources map[string]string `yaml:"-" json:"-"`
}

// ConfigSource identifies where a config value originated.
const (
	SourceDefault = "default"
	SourceEnv     = "env"
	SourceGlobal  = "global"
	SourceLocal   = "local"
	SourceFlag    = "flag"
)
