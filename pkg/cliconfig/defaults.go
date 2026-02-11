package cliconfig

import "strconv"

// DefaultPort is the default HTTP server port for mock traffic.
const DefaultPort = 4280

// DefaultAdminPort is the default admin API port.
const DefaultAdminPort = 4290

// DefaultHTTPSPort is the default HTTPS port (0 = disabled).
const DefaultHTTPSPort = 0

// DefaultReadTimeout is the default read timeout in seconds.
const DefaultReadTimeout = 30

// DefaultWriteTimeout is the default write timeout in seconds.
const DefaultWriteTimeout = 30

// DefaultMaxLogEntries is the default maximum request log entries.
const DefaultMaxLogEntries = 1000

// DefaultAutoCert is whether to auto-generate TLS certificates.
const DefaultAutoCert = true

// DefaultAdminURL returns the default admin API URL based on the admin port.
func DefaultAdminURL(adminPort int) string {
	if adminPort == 0 {
		adminPort = DefaultAdminPort
	}
	return "http://localhost:" + strconv.Itoa(adminPort)
}

// NewDefault creates a new CLIConfig with default values.
func NewDefault() *CLIConfig {
	cfg := &CLIConfig{
		Port:          DefaultPort,
		AdminPort:     DefaultAdminPort,
		HTTPSPort:     DefaultHTTPSPort,
		ReadTimeout:   DefaultReadTimeout,
		WriteTimeout:  DefaultWriteTimeout,
		MaxLogEntries: DefaultMaxLogEntries,
		AutoCert:      DefaultAutoCert,
		Sources:       make(map[string]string),
	}
	cfg.AdminURL = DefaultAdminURL(cfg.AdminPort)

	// Mark all as default source
	cfg.Sources["port"] = SourceDefault
	cfg.Sources["adminPort"] = SourceDefault
	cfg.Sources["httpsPort"] = SourceDefault
	cfg.Sources["readTimeout"] = SourceDefault
	cfg.Sources["writeTimeout"] = SourceDefault
	cfg.Sources["maxLogEntries"] = SourceDefault
	cfg.Sources["autoCert"] = SourceDefault
	cfg.Sources["adminUrl"] = SourceDefault

	return cfg
}
