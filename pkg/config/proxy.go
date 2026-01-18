// Package config provides proxy configuration types.
package config

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
