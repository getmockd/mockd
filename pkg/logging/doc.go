// Package logging provides structured logging configuration for mockd.
//
// This package wraps log/slog to provide consistent logging across all mockd
// components. It supports configurable log levels and output formats.
//
// # Usage
//
// Create a logger with desired configuration:
//
//	logger := logging.New(logging.Config{
//	    Level:  logging.LevelInfo,
//	    Format: logging.FormatText,
//	})
//
//	logger.Info("server started", "port", 4280)
//	logger.Error("failed to connect", "error", err)
//
// # Log Levels
//
// Four log levels are supported:
//   - Debug: Detailed information for debugging
//   - Info: General operational information
//   - Warn: Warning conditions that should be addressed
//   - Error: Error conditions that need attention
//
// # Output Formats
//
//   - Text: Human-readable format for development
//   - JSON: Structured format for log aggregation systems
//
// # Integration
//
// Components should accept a *slog.Logger in their constructor or via a setter.
// If no logger is provided, use logging.Nop() for a no-op logger.
package logging
