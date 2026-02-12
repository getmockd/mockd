// Package cliconfig provides configuration types and loading for the mockd CLI.
package cliconfig

import "fmt"

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

	// SetFields tracks which YAML keys were explicitly present when this
	// config was loaded from a file. This lets MergeConfig distinguish
	// "field absent" from "field explicitly set to zero value" (e.g.,
	// autoCert: false vs. autoCert not mentioned at all).
	SetFields map[string]bool `yaml:"-" json:"-"`
}

// ConfigSource identifies where a config value originated.
const (
	SourceDefault = "default"
	SourceEnv     = "env"
	SourceGlobal  = "global"
	SourceLocal   = "local"
	SourceFlag    = "flag"
)

// Validate checks that all numeric config values are within acceptable ranges.
// Returns nil if the configuration is valid.
func (c *CLIConfig) Validate() error {
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("port %d is out of range (0-65535)", c.Port)
	}
	if c.AdminPort < 0 || c.AdminPort > 65535 {
		return fmt.Errorf("adminPort %d is out of range (0-65535)", c.AdminPort)
	}
	if c.HTTPSPort < 0 || c.HTTPSPort > 65535 {
		return fmt.Errorf("httpsPort %d is out of range (0-65535)", c.HTTPSPort)
	}
	if c.ReadTimeout < 0 || c.ReadTimeout > 3600 {
		return fmt.Errorf("readTimeout %d is out of range (0-3600 seconds)", c.ReadTimeout)
	}
	if c.WriteTimeout < 0 || c.WriteTimeout > 3600 {
		return fmt.Errorf("writeTimeout %d is out of range (0-3600 seconds)", c.WriteTimeout)
	}
	if c.MaxLogEntries < 0 || c.MaxLogEntries > 100000 {
		return fmt.Errorf("maxLogEntries %d is out of range (0-100000)", c.MaxLogEntries)
	}
	if c.Port != 0 && c.AdminPort != 0 && c.Port == c.AdminPort {
		return fmt.Errorf("port and adminPort cannot be the same (%d)", c.Port)
	}
	return nil
}
