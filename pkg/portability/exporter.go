package portability

import (
	"github.com/getmockd/mockd/pkg/config"
)

// Exporter defines the interface for exporting mock configurations to external formats.
type Exporter interface {
	// Export converts a MockCollection to the exporter's format.
	// Returns the raw bytes suitable for writing to a file.
	Export(collection *config.MockCollection) ([]byte, error)

	// Format returns the format this exporter produces.
	Format() Format
}

// ExportOptions provides configuration for the export process.
type ExportOptions struct {
	// Format is the output format (defaults to FormatMockd)
	Format Format

	// AsYAML if true and format is Mockd, outputs YAML instead of JSON
	AsYAML bool

	// Pretty if true, formats output with indentation
	Pretty bool
}

// ExportResult contains the result of an export operation.
type ExportResult struct {
	// Data is the exported bytes
	Data []byte

	// Format is the format that was used
	Format Format

	// Statistics about what was exported
	EndpointCount int
	ScenarioCount int
	StatefulCount int
}

// Export is a convenience function that exports to a specified format.
func Export(collection *config.MockCollection, opts *ExportOptions) (*ExportResult, error) {
	if opts == nil {
		opts = &ExportOptions{Format: FormatMockd}
	}

	format := opts.Format
	if format == FormatUnknown {
		format = FormatMockd
	}

	if !format.CanExport() {
		return nil, &ExportError{
			Format:  format,
			Message: "format does not support export",
		}
	}

	exporter := GetExporter(format)
	if exporter == nil {
		return nil, &ExportError{
			Format:  format,
			Message: "no exporter available for format",
		}
	}

	data, err := exporter.Export(collection)
	if err != nil {
		return nil, err
	}

	result := &ExportResult{
		Data:          data,
		Format:        format,
		EndpointCount: len(collection.Mocks),
		ScenarioCount: 0,
		StatefulCount: len(collection.StatefulResources),
	}

	return result, nil
}

// ExportError represents an error during export.
type ExportError struct {
	Format  Format
	Message string
	Cause   error
}

func (e *ExportError) Error() string {
	msg := e.Message
	if e.Format != FormatUnknown {
		msg = string(e.Format) + ": " + msg
	}
	if e.Cause != nil {
		msg = msg + ": " + e.Cause.Error()
	}
	return msg
}

func (e *ExportError) Unwrap() error {
	return e.Cause
}
