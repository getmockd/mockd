package logging

import (
	"io"
	"log/slog"
	"os"
)

// Level represents a log level.
type Level = slog.Level

// Log levels.
const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

// Format represents the log output format.
type Format string

// Output formats.
const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// Config holds logging configuration.
type Config struct {
	// Level is the minimum log level to output.
	Level Level

	// Format is the output format (text or json).
	Format Format

	// Output is the writer to send logs to. Defaults to os.Stderr.
	Output io.Writer

	// AddSource adds source file and line to log entries.
	AddSource bool
}

// DefaultConfig returns sensible defaults for logging.
func DefaultConfig() Config {
	return Config{
		Level:     LevelInfo,
		Format:    FormatText,
		Output:    os.Stderr,
		AddSource: false,
	}
}

// New creates a new slog.Logger with the given configuration.
func New(cfg Config) *slog.Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stderr
	}

	opts := &slog.HandlerOptions{
		Level:     cfg.Level,
		AddSource: cfg.AddSource,
	}

	var handler slog.Handler
	switch cfg.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(cfg.Output, opts)
	default:
		handler = slog.NewTextHandler(cfg.Output, opts)
	}

	return slog.New(handler)
}

// NewWithLevel creates a logger with the specified level using text format.
func NewWithLevel(level Level) *slog.Logger {
	return New(Config{
		Level:  level,
		Format: FormatText,
		Output: os.Stderr,
	})
}

// Nop returns a no-op logger that discards all output.
// Use this when a logger is required but logging is disabled.
func Nop() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ParseLevel parses a log level string.
// Valid values: "debug", "info", "warn", "error".
// Returns LevelInfo if the string is not recognized.
func ParseLevel(s string) Level {
	switch s {
	case "debug", "DEBUG":
		return LevelDebug
	case "info", "INFO", "":
		return LevelInfo
	case "warn", "WARN", "warning", "WARNING":
		return LevelWarn
	case "error", "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

// ParseFormat parses a log format string.
// Valid values: "text", "json".
// Returns FormatText if the string is not recognized.
func ParseFormat(s string) Format {
	switch s {
	case "json", "JSON":
		return FormatJSON
	default:
		return FormatText
	}
}
