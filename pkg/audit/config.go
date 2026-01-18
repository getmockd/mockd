package audit

// Level constants define the audit logging levels.
const (
	LevelDebug = "debug"
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// AuditConfig defines the configuration for audit logging.
type AuditConfig struct {
	// Enabled determines whether audit logging is active.
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Level controls the minimum severity of events to log.
	// Valid values: "debug", "info", "warn", "error".
	// Default: "info".
	Level string `json:"level,omitempty" yaml:"level,omitempty"`

	// OutputFile is the path to the audit log file.
	// If empty and Enabled is true, logs are written to stdout.
	OutputFile string `json:"outputFile,omitempty" yaml:"outputFile,omitempty"`

	// MaxBodyPreviewSize limits the size of body previews in bytes.
	// Default: 1024. Set to 0 to disable body previews.
	MaxBodyPreviewSize int `json:"maxBodyPreviewSize,omitempty" yaml:"maxBodyPreviewSize,omitempty"`

	// IncludeHeaders determines whether to include request/response headers.
	// Default: true.
	IncludeHeaders bool `json:"includeHeaders,omitempty" yaml:"includeHeaders,omitempty"`

	// Extensions provides a generic configuration map for enterprise extensions.
	// Enterprise packages can register custom writers and redactors via the registry.
	Extensions map[string]interface{} `json:"extensions,omitempty" yaml:"extensions,omitempty"`
}

// DefaultAuditConfig returns an AuditConfig with sensible defaults.
func DefaultAuditConfig() *AuditConfig {
	return &AuditConfig{
		Enabled:            false,
		Level:              LevelInfo,
		MaxBodyPreviewSize: 1024,
		IncludeHeaders:     true,
	}
}

// Validate checks that the configuration is valid.
func (c *AuditConfig) Validate() error {
	if !c.Enabled {
		return nil
	}

	switch c.Level {
	case LevelDebug, LevelInfo, LevelWarn, LevelError, "":
		// Valid levels
	default:
		return &ConfigError{Field: "level", Message: "must be one of: debug, info, warn, error"}
	}

	return nil
}

// ShouldLog returns true if the given level should be logged
// based on the configured minimum level.
func (c *AuditConfig) ShouldLog(level string) bool {
	if !c.Enabled {
		return false
	}

	configLevel := c.levelPriority(c.Level)
	eventLevel := c.levelPriority(level)

	return eventLevel >= configLevel
}

// levelPriority returns a numeric priority for a log level.
func (c *AuditConfig) levelPriority(level string) int {
	switch level {
	case LevelDebug:
		return 0
	case LevelInfo:
		return 1
	case LevelWarn:
		return 2
	case LevelError:
		return 3
	default:
		return 1 // Default to info
	}
}

// ConfigError represents a configuration validation error.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return "audit config: " + e.Field + ": " + e.Message
}
