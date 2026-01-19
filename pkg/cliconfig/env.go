package cliconfig

import (
	"os"
	"strconv"
)

// Environment variable names
const (
	EnvPort          = "MOCKD_PORT"
	EnvAdminPort     = "MOCKD_ADMIN_PORT"
	EnvAdminURL      = "MOCKD_ADMIN_URL"
	EnvWorkspace     = "MOCKD_WORKSPACE"
	EnvContext       = "MOCKD_CONTEXT"
	EnvConfig        = "MOCKD_CONFIG"
	EnvHTTPSPort     = "MOCKD_HTTPS_PORT"
	EnvAutoCert      = "MOCKD_AUTO_CERT"
	EnvReadTimeout   = "MOCKD_READ_TIMEOUT"
	EnvWriteTimeout  = "MOCKD_WRITE_TIMEOUT"
	EnvMaxLogEntries = "MOCKD_MAX_LOG_ENTRIES"
	EnvVerbose       = "MOCKD_VERBOSE"
)

// LoadEnvConfig loads configuration from environment variables.
// It only sets values that are present in the environment.
func LoadEnvConfig(cfg *CLIConfig) {
	if cfg.Sources == nil {
		cfg.Sources = make(map[string]string)
	}

	// MOCKD_PORT
	if v := os.Getenv(EnvPort); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.Port = port
			cfg.Sources["port"] = SourceEnv
		}
	}

	// MOCKD_ADMIN_PORT
	if v := os.Getenv(EnvAdminPort); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.AdminPort = port
			cfg.Sources["adminPort"] = SourceEnv
		}
	}

	// MOCKD_ADMIN_URL
	if v := os.Getenv(EnvAdminURL); v != "" {
		cfg.AdminURL = v
		cfg.Sources["adminUrl"] = SourceEnv
	}

	// MOCKD_CONFIG
	if v := os.Getenv(EnvConfig); v != "" {
		cfg.ConfigFile = v
		cfg.Sources["configFile"] = SourceEnv
	}

	// MOCKD_HTTPS_PORT
	if v := os.Getenv(EnvHTTPSPort); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			cfg.HTTPSPort = port
			cfg.Sources["httpsPort"] = SourceEnv
		}
	}

	// MOCKD_AUTO_CERT
	if v := os.Getenv(EnvAutoCert); v != "" {
		cfg.AutoCert = v == "true" || v == "1" || v == "yes"
		cfg.Sources["autoCert"] = SourceEnv
	}

	// MOCKD_READ_TIMEOUT
	if v := os.Getenv(EnvReadTimeout); v != "" {
		if timeout, err := strconv.Atoi(v); err == nil {
			cfg.ReadTimeout = timeout
			cfg.Sources["readTimeout"] = SourceEnv
		}
	}

	// MOCKD_WRITE_TIMEOUT
	if v := os.Getenv(EnvWriteTimeout); v != "" {
		if timeout, err := strconv.Atoi(v); err == nil {
			cfg.WriteTimeout = timeout
			cfg.Sources["writeTimeout"] = SourceEnv
		}
	}

	// MOCKD_MAX_LOG_ENTRIES
	if v := os.Getenv(EnvMaxLogEntries); v != "" {
		if max, err := strconv.Atoi(v); err == nil {
			cfg.MaxLogEntries = max
			cfg.Sources["maxLogEntries"] = SourceEnv
		}
	}

	// MOCKD_VERBOSE
	if v := os.Getenv(EnvVerbose); v != "" {
		cfg.Verbose = v == "true" || v == "1" || v == "yes"
		cfg.Sources["verbose"] = SourceEnv
	}
}

// GetAdminURLFromEnv returns the admin URL from environment variable.
// Returns empty string if not set.
func GetAdminURLFromEnv() string {
	return os.Getenv(EnvAdminURL)
}

// GetWorkspaceFromEnv returns the workspace from environment variable.
// Returns empty string if not set.
func GetWorkspaceFromEnv() string {
	return os.Getenv(EnvWorkspace)
}

// GetContextFromEnv returns the context name from environment variable.
// Returns empty string if not set.
func GetContextFromEnv() string {
	return os.Getenv(EnvContext)
}
