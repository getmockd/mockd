// Package config provides proxy configuration types.
package config

import (
	"fmt"
	"regexp"

	"github.com/getmockd/mockd/pkg/mock"
)

// ProxyConfiguration defines settings for proxy server operation.
type ProxyConfiguration struct {
	// Port is the proxy listen port (default: 8888)
	Port int `json:"port"`
	// Mode is the operating mode (record, passthrough)
	Mode string `json:"mode"`
	// SessionName is the name for current/new session
	SessionName string `json:"sessionName,omitempty"`
	// Filters configure selective recording
	Filters *ProxyFilterConfig `json:"filters,omitempty"`
	// CAPath is the path to CA certificate directory
	CAPath string `json:"caPath,omitempty"`
	// MaxBodySize is the max body size to capture in bytes (default: 10MB)
	MaxBodySize int `json:"maxBodySize,omitempty"`
}

// ProxyFilterConfig defines include/exclude patterns for traffic filtering.
type ProxyFilterConfig struct {
	IncludePaths []string `json:"includePaths,omitempty"`
	ExcludePaths []string `json:"excludePaths,omitempty"`
	IncludeHosts []string `json:"includeHosts,omitempty"`
	ExcludeHosts []string `json:"excludeHosts,omitempty"`
}

// CAConfiguration defines Certificate Authority settings.
type CAConfiguration struct {
	// CertPath is the path to CA certificate file
	CertPath string `json:"certPath"`
	// KeyPath is the path to CA private key file
	KeyPath string `json:"keyPath"`
	// Organization is the certificate organization name
	Organization string `json:"organization,omitempty"`
	// ValidityDays is the certificate validity in days
	ValidityDays int `json:"validityDays,omitempty"`
}

// DefaultProxyConfiguration returns default proxy settings.
func DefaultProxyConfiguration() *ProxyConfiguration {
	return &ProxyConfiguration{
		Port:        8888,
		Mode:        "record",
		MaxBodySize: 10 * 1024 * 1024, // 10MB
	}
}

// DefaultCAConfiguration returns default CA settings.
func DefaultCAConfiguration(configDir string) *CAConfiguration {
	return &CAConfiguration{
		CertPath:     configDir + "/ca/mockd-ca.crt",
		KeyPath:      configDir + "/ca/mockd-ca.key",
		Organization: "mockd Local CA",
		ValidityDays: 3650, // 10 years
	}
}

// validProxyModes are the allowed proxy operating modes.
var validProxyModes = map[string]bool{
	"record":      true,
	"passthrough": true,
}

// Validate checks if the ProxyConfiguration is valid.
func (p *ProxyConfiguration) Validate() error {
	if p.Port <= 0 || p.Port > 65535 {
		return &mock.ValidationError{Field: "port", Message: "port must be between 1 and 65535"}
	}

	if p.Mode != "" && !validProxyModes[p.Mode] {
		return &mock.ValidationError{
			Field:   "mode",
			Message: fmt.Sprintf("invalid mode: %s (must be 'record' or 'passthrough')", p.Mode),
		}
	}

	if p.MaxBodySize < 0 {
		return &mock.ValidationError{Field: "maxBodySize", Message: "maxBodySize must be >= 0"}
	}

	// Validate filters if present
	if p.Filters != nil {
		if err := p.Filters.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// Validate checks if the ProxyFilterConfig is valid.
func (f *ProxyFilterConfig) Validate() error {
	// Validate path patterns are valid regex
	for i, pattern := range f.IncludePaths {
		if _, err := regexp.Compile(pattern); err != nil {
			return &mock.ValidationError{
				Field:   fmt.Sprintf("filters.includePaths[%d]", i),
				Message: "invalid regex pattern: " + err.Error(),
			}
		}
	}

	for i, pattern := range f.ExcludePaths {
		if _, err := regexp.Compile(pattern); err != nil {
			return &mock.ValidationError{
				Field:   fmt.Sprintf("filters.excludePaths[%d]", i),
				Message: "invalid regex pattern: " + err.Error(),
			}
		}
	}

	for i, pattern := range f.IncludeHosts {
		if _, err := regexp.Compile(pattern); err != nil {
			return &mock.ValidationError{
				Field:   fmt.Sprintf("filters.includeHosts[%d]", i),
				Message: "invalid regex pattern: " + err.Error(),
			}
		}
	}

	for i, pattern := range f.ExcludeHosts {
		if _, err := regexp.Compile(pattern); err != nil {
			return &mock.ValidationError{
				Field:   fmt.Sprintf("filters.excludeHosts[%d]", i),
				Message: "invalid regex pattern: " + err.Error(),
			}
		}
	}

	return nil
}

// Validate checks if the CAConfiguration is valid.
func (c *CAConfiguration) Validate() error {
	if c.CertPath == "" {
		return &mock.ValidationError{Field: "certPath", Message: "certPath is required"}
	}

	if c.KeyPath == "" {
		return &mock.ValidationError{Field: "keyPath", Message: "keyPath is required"}
	}

	if c.ValidityDays < 0 {
		return &mock.ValidationError{Field: "validityDays", Message: "validityDays must be >= 0"}
	}

	return nil
}
