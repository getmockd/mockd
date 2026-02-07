package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/portability"
)

// RunExport handles the export command.
func RunExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)

	output := fs.String("output", "", "Output file (default: stdout)")
	fs.StringVar(output, "o", "", "Output file (shorthand)")
	name := fs.String("name", "exported-config", "Collection name")
	fs.StringVar(name, "n", "exported-config", "Collection name (shorthand)")
	format := fs.String("format", "mockd", "Output format: mockd, openapi")
	fs.StringVar(format, "f", "mockd", "Output format (shorthand)")
	version := fs.String("version", "", "Version tag for the export")
	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd export [flags]

Export current mock configuration to various formats.

Flags:
  -o, --output       Output file (default: stdout)
  -n, --name         Collection name (default: exported-config)
  -f, --format       Output format: mockd, openapi (default: mockd)
      --version      Version tag for the export
      --admin-url    Admin API base URL (default: http://localhost:4290)

Formats:
  mockd    Mockd native format (YAML/JSON) - recommended for portability
  openapi  OpenAPI 3.x specification - for API documentation

Examples:
  # Export to stdout as YAML
  mockd export

  # Export to JSON file
  mockd export -o mocks.json

  # Export to YAML file
  mockd export -o mocks.yaml

  # Export as OpenAPI specification
  mockd export -f openapi -o api.yaml

  # Export with custom name
  mockd export -n "My API Mocks" -o mocks.yaml
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Parse format
	exportFormat := portability.ParseFormat(*format)
	if exportFormat == portability.FormatUnknown {
		return fmt.Errorf(`invalid format: %s

Supported formats: mockd, openapi`, *format)
	}

	if !exportFormat.CanExport() {
		return fmt.Errorf(`format '%s' does not support export

Supported export formats: mockd, openapi`, *format)
	}

	// Create admin client and export config
	client := NewAdminClientWithAuth(*adminURL)
	collection, err := client.ExportConfig(*name)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Apply version tag if specified
	if *version != "" {
		collection.Version = *version
	}

	// Determine output format (YAML vs JSON) from file extension
	asYAML := true
	if *output != "" {
		ext := strings.ToLower(filepath.Ext(*output))
		asYAML = ext == ".yaml" || ext == ".yml"
	}

	// Get the appropriate exporter
	var data []byte
	switch exportFormat {
	case portability.FormatMockd:
		// Output MockCollection format directly so the exported file can be
		// loaded back by `mockd serve --config` without conversion.
		if asYAML {
			data, err = config.ToYAML(collection)
		} else {
			data, err = config.ToJSON(collection)
		}
	case portability.FormatOpenAPI:
		exporter := &portability.OpenAPIExporter{AsYAML: asYAML}
		data, err = exporter.Export(collection)
	default:
		return fmt.Errorf("unsupported export format: %s", exportFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to export: %w", err)
	}

	// Output to file or stdout
	if *output != "" {
		if err := os.WriteFile(*output, data, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Printf("Exported %d mocks to %s (format: %s)\n", len(collection.Mocks), *output, exportFormat)
	} else {
		fmt.Print(string(data))
	}

	return nil
}
