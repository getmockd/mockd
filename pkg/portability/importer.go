package portability

import (
	"strconv"

	"github.com/getmockd/mockd/pkg/config"
)

// Importer defines the interface for importing mock configurations from external formats.
type Importer interface {
	// Import parses data in the importer's format and returns a MockCollection.
	// The data should be the raw bytes of the source file or content.
	Import(data []byte) (*config.MockCollection, error)

	// Format returns the format this importer handles.
	Format() Format
}

// ImportOptions provides configuration for the import process.
type ImportOptions struct {
	// Name is the collection name to use (overrides any in the source)
	Name string

	// Merge if true, merges with existing collection instead of replacing
	Merge bool

	// DryRun if true, parses and validates but doesn't save
	DryRun bool

	// IncludeStatic for HAR imports, includes static assets
	IncludeStatic bool
}

// ImportResult contains the result of an import operation.
type ImportResult struct {
	// Collection is the imported mock collection
	Collection *config.MockCollection

	// Warnings are non-fatal issues encountered during import
	Warnings []string

	// Statistics about what was imported
	EndpointCount int
	ScenarioCount int
	StatefulCount int
}

// Import is a convenience function that auto-detects format and imports.
func Import(data []byte, filename string, opts *ImportOptions) (*ImportResult, error) {
	format := DetectFormat(data, filename)
	if format == FormatUnknown {
		return nil, &ImportError{
			Format:  format,
			Message: "unable to detect format from file content",
		}
	}

	importer := GetImporter(format)
	if importer == nil {
		return nil, &ImportError{
			Format:  format,
			Message: "no importer available for format",
		}
	}

	collection, err := importer.Import(data)
	if err != nil {
		return nil, err
	}

	// Apply options
	if opts != nil {
		if opts.Name != "" {
			collection.Name = opts.Name
		}

		// DryRun: return parsed mocks without committing (caller should not persist)
		if opts.DryRun {
			return &ImportResult{
				Collection:    collection,
				EndpointCount: len(collection.Mocks),
				ScenarioCount: 0,
				StatefulCount: len(collection.StatefulResources),
			}, nil
		}

		// TODO: Merge is not yet wired — when true, the caller should merge the
		// returned collection with an existing one instead of replacing it.

		// TODO: IncludeStatic is not yet wired at this layer — it is respected
		// directly by HARImporter.IncludeStatic. Callers should set that field
		// on the HARImporter before invoking Import when this option is true.
	}

	result := &ImportResult{
		Collection:    collection,
		EndpointCount: len(collection.Mocks),
		ScenarioCount: 0,
		StatefulCount: len(collection.StatefulResources),
	}

	return result, nil
}

// ImportError represents an error during import.
type ImportError struct {
	Format  Format
	Line    int
	Column  int
	Message string
	Cause   error
}

func (e *ImportError) Error() string {
	msg := e.Message
	if e.Format != FormatUnknown {
		msg = string(e.Format) + ": " + msg
	}
	if e.Line > 0 {
		if e.Column > 0 {
			msg = msg + " (line " + strconv.Itoa(e.Line) + ", column " + strconv.Itoa(e.Column) + ")"
		} else {
			msg = msg + " (line " + strconv.Itoa(e.Line) + ")"
		}
	}
	if e.Cause != nil {
		msg = msg + ": " + e.Cause.Error()
	}
	return msg
}

func (e *ImportError) Unwrap() error {
	return e.Cause
}
