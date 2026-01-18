package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/getmockd/mockd/pkg/audit"
	"github.com/getmockd/mockd/pkg/mock"
)

// validClientAuthValues are the allowed mTLS client authentication policies.
var validClientAuthValues = map[string]bool{
	"none":               true,
	"request":            true,
	"require":            true,
	"verify-if-given":    true,
	"require-and-verify": true,
}

// validAuditLevels are the allowed audit log levels.
var validAuditLevels = map[string]bool{
	audit.LevelDebug: true,
	audit.LevelInfo:  true,
	audit.LevelWarn:  true,
	audit.LevelError: true,
}

// validateFilePath checks if a file path is valid and the file exists.
// Returns nil if the path is empty (considered optional) or valid.
func validateFilePath(path, fieldName string) error {
	if path == "" {
		return nil
	}

	// Check if path is absolute or relative
	if !filepath.IsAbs(path) {
		// Relative paths are allowed, but we check if the file exists
		// from the current working directory
	}

	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("file does not exist: %s", path),
			}
		}
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("cannot access file: %s", err.Error()),
		}
	}

	// Ensure it's a file, not a directory
	if info.IsDir() {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("path is a directory, not a file: %s", path),
		}
	}

	return nil
}

// validateParentDirExists checks if the parent directory of a file path exists or can be created.
// Returns nil if the path is empty (considered optional) or the parent exists.
func validateParentDirExists(path, fieldName string) error {
	if path == "" {
		return nil
	}

	parentDir := filepath.Dir(path)
	if parentDir == "" || parentDir == "." {
		// Current directory, assumed to exist
		return nil
	}

	info, err := os.Stat(parentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return &ValidationError{
				Field:   fieldName,
				Message: fmt.Sprintf("parent directory does not exist: %s", parentDir),
			}
		}
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("cannot access parent directory: %s", err.Error()),
		}
	}

	if !info.IsDir() {
		return &ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("parent path is not a directory: %s", parentDir),
		}
	}

	return nil
}

// ValidationError is an alias for mock.ValidationError for backward compatibility.
type ValidationError = mock.ValidationError

// Validate checks if the TLSConfig is valid.
func (t *TLSConfig) Validate() error {
	if t == nil || !t.Enabled {
		return nil
	}

	// If Enabled, either AutoGenerateCert must be true OR both CertFile and KeyFile must be provided
	if !t.AutoGenerateCert {
		if t.CertFile == "" && t.KeyFile == "" {
			return &ValidationError{
				Field:   "tls",
				Message: "when enabled, either autoGenerateCert must be true or both certFile and keyFile must be provided",
			}
		}
		if t.CertFile == "" {
			return &ValidationError{
				Field:   "tls.certFile",
				Message: "certFile is required when autoGenerateCert is false",
			}
		}
		if t.KeyFile == "" {
			return &ValidationError{
				Field:   "tls.keyFile",
				Message: "keyFile is required when autoGenerateCert is false",
			}
		}

		// Validate file paths exist
		if err := validateFilePath(t.CertFile, "tls.certFile"); err != nil {
			return err
		}
		if err := validateFilePath(t.KeyFile, "tls.keyFile"); err != nil {
			return err
		}
	}

	return nil
}

// Validate checks if the MTLSConfig is valid.
func (m *MTLSConfig) Validate() error {
	if m == nil || !m.Enabled {
		return nil
	}

	// At least one CA cert source must be provided
	if m.CACertFile == "" && len(m.CACertFiles) == 0 {
		return &ValidationError{
			Field:   "mtls",
			Message: "when enabled, at least one CA certificate source must be provided (caCertFile or caCertFiles)",
		}
	}

	// Validate ClientAuth if provided
	if m.ClientAuth != "" {
		if !validClientAuthValues[m.ClientAuth] {
			return &ValidationError{
				Field:   "mtls.clientAuth",
				Message: fmt.Sprintf("invalid clientAuth value: %s (must be one of: none, request, require, verify-if-given, require-and-verify)", m.ClientAuth),
			}
		}
	}

	// Validate CACertFile if provided
	if err := validateFilePath(m.CACertFile, "mtls.caCertFile"); err != nil {
		return err
	}

	// Validate each CACertFiles entry
	for i, certFile := range m.CACertFiles {
		if certFile == "" {
			return &ValidationError{
				Field:   fmt.Sprintf("mtls.caCertFiles[%d]", i),
				Message: "CA certificate file path cannot be empty",
			}
		}
		if err := validateFilePath(certFile, fmt.Sprintf("mtls.caCertFiles[%d]", i)); err != nil {
			return err
		}
	}

	return nil
}

// ValidateAuditConfig checks if the AuditConfig is valid.
func ValidateAuditConfig(a *audit.AuditConfig) error {
	if a == nil || !a.Enabled {
		return nil
	}

	// Validate Level
	if a.Level != "" && !validAuditLevels[a.Level] {
		return &ValidationError{
			Field:   "audit.level",
			Message: fmt.Sprintf("invalid audit level: %s (must be one of: debug, info, warn, error)", a.Level),
		}
	}

	// Validate OutputFile parent directory exists if provided
	if err := validateParentDirExists(a.OutputFile, "audit.outputFile"); err != nil {
		return err
	}

	return nil
}

// Validate checks if the ServerConfiguration is valid.
func (s *ServerConfiguration) Validate() error {
	// At least one of HTTPPort or HTTPSPort must be > 0
	if s.HTTPPort <= 0 && s.HTTPSPort <= 0 {
		return &ValidationError{
			Field:   "serverConfig",
			Message: "at least one of httpPort or httpsPort must be > 0",
		}
	}

	// AdminPort must be > 0 and < 65536
	if s.AdminPort <= 0 || s.AdminPort >= 65536 {
		return &ValidationError{
			Field:   "serverConfig.adminPort",
			Message: "adminPort must be between 1 and 65535",
		}
	}

	// Validate port values
	if s.HTTPPort < 0 || s.HTTPPort >= 65536 {
		return &ValidationError{
			Field:   "serverConfig.httpPort",
			Message: "httpPort must be between 0 and 65535",
		}
	}

	if s.HTTPSPort < 0 || s.HTTPSPort >= 65536 {
		return &ValidationError{
			Field:   "serverConfig.httpsPort",
			Message: "httpsPort must be between 0 and 65535",
		}
	}

	// Ports must not conflict (all different if > 0)
	ports := make(map[int]string)
	if s.HTTPPort > 0 {
		ports[s.HTTPPort] = "httpPort"
	}
	if s.HTTPSPort > 0 {
		if name, exists := ports[s.HTTPSPort]; exists {
			return &ValidationError{
				Field:   "serverConfig",
				Message: fmt.Sprintf("httpsPort conflicts with %s (both are %d)", name, s.HTTPSPort),
			}
		}
		ports[s.HTTPSPort] = "httpsPort"
	}
	if s.AdminPort > 0 {
		if name, exists := ports[s.AdminPort]; exists {
			return &ValidationError{
				Field:   "serverConfig",
				Message: fmt.Sprintf("adminPort conflicts with %s (both are %d)", name, s.AdminPort),
			}
		}
	}

	// If HTTPSPort > 0, TLS must be configured
	if s.HTTPSPort > 0 && s.TLS != nil && s.TLS.Enabled && !s.TLS.AutoGenerateCert {
		if s.TLS.CertFile == "" {
			return &ValidationError{
				Field:   "serverConfig.tls.certFile",
				Message: "certFile is required when httpsPort is set and autoGenerateCert is false",
			}
		}
		if s.TLS.KeyFile == "" {
			return &ValidationError{
				Field:   "serverConfig.tls.keyFile",
				Message: "keyFile is required when httpsPort is set and autoGenerateCert is false",
			}
		}
	}

	// MaxBodySize must be > 0 and <= 100MB
	if s.MaxBodySize < 0 {
		return &ValidationError{Field: "serverConfig.maxBodySize", Message: "maxBodySize must be >= 0"}
	}
	if s.MaxBodySize > 100*1024*1024 {
		return &ValidationError{
			Field:   "serverConfig.maxBodySize",
			Message: "maxBodySize must be <= 104857600 (100MB)",
		}
	}

	// MaxLogEntries must be >= 0
	if s.MaxLogEntries < 0 {
		return &ValidationError{Field: "serverConfig.maxLogEntries", Message: "maxLogEntries must be >= 0"}
	}

	// Timeouts must be >= 0
	if s.ReadTimeout < 0 {
		return &ValidationError{Field: "serverConfig.readTimeout", Message: "readTimeout must be >= 0"}
	}
	if s.WriteTimeout < 0 {
		return &ValidationError{Field: "serverConfig.writeTimeout", Message: "writeTimeout must be >= 0"}
	}

	// Validate TLS config if present
	if s.TLS != nil {
		if err := s.TLS.Validate(); err != nil {
			return err
		}
	}

	// Validate MTLS config if present
	if s.MTLS != nil {
		if err := s.MTLS.Validate(); err != nil {
			return err
		}
	}

	// Validate Audit config if present
	if s.Audit != nil {
		if err := ValidateAuditConfig(s.Audit); err != nil {
			return err
		}
	}

	return nil
}

// Validate checks if the MockCollection is valid.
func (c *MockCollection) Validate() error {
	// Version must be "1.0" (only supported version initially)
	if c.Version != "1.0" {
		return &ValidationError{
			Field:   "version",
			Message: fmt.Sprintf("unsupported version: %s (expected 1.0)", c.Version),
		}
	}

	// Check for duplicate IDs
	ids := make(map[string]bool)
	for i, mock := range c.Mocks {
		if mock == nil {
			return &ValidationError{
				Field:   fmt.Sprintf("mocks[%d]", i),
				Message: "mock cannot be null",
			}
		}
		if err := mock.Validate(); err != nil {
			return &ValidationError{
				Field:   fmt.Sprintf("mocks[%d]", i),
				Message: err.Error(),
			}
		}
		if ids[mock.ID] {
			return &ValidationError{
				Field:   fmt.Sprintf("mocks[%d].id", i),
				Message: fmt.Sprintf("duplicate mock ID: %s", mock.ID),
			}
		}
		ids[mock.ID] = true
	}

	// Validate ServerConfig if present
	if c.ServerConfig != nil {
		if err := c.ServerConfig.Validate(); err != nil {
			return err
		}
	}

	return nil
}
