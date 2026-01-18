package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/getmockd/mockd/internal/cliconfig"
	"github.com/getmockd/mockd/pkg/portability"
)

// RunImport handles the import command.
func RunImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)

	format := fs.String("format", "", "Force format (auto-detected if omitted)")
	fs.StringVar(format, "f", "", "Force format (shorthand)")
	replace := fs.Bool("replace", false, "Replace all existing mocks")
	dryRun := fs.Bool("dry-run", false, "Preview import without saving")
	includeStatic := fs.Bool("include-static", false, "Include static assets (for HAR imports)")
	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd import <source> [flags]

Import mocks from various sources and formats.

Arguments:
  source    Path to file, or cURL command (in quotes)

Flags:
  -f, --format         Force format (auto-detected if omitted)
      --replace        Replace all existing mocks (default: merge)
      --dry-run        Preview import without saving
      --include-static Include static assets (for HAR imports)
      --admin-url      Admin API base URL (default: http://localhost:4290)

Supported Formats:
  mockd      Mockd native format (YAML/JSON)
  openapi    OpenAPI 3.x or Swagger 2.0
  postman    Postman Collection v2.x
  har        HTTP Archive (browser recordings)
  wiremock   WireMock JSON mappings
  curl       cURL command

Examples:
  # Import from OpenAPI spec (auto-detected)
  mockd import openapi.yaml

  # Import from Postman collection
  mockd import collection.json -f postman

  # Import from HAR file including static assets
  mockd import recording.har --include-static

  # Import from cURL command
  mockd import "curl -X POST https://api.example.com/users -H 'Content-Type: application/json' -d '{\"name\": \"test\"}'"

  # Preview import without saving
  mockd import openapi.yaml --dry-run

  # Replace all mocks with imported ones
  mockd import mocks.yaml --replace
`)
	}

	// Reorder args so flags come before positional arguments
	reorderedArgs := reorderArgs(args, []string{"admin-url", "format", "f"})

	if err := fs.Parse(reorderedArgs); err != nil {
		return err
	}

	// Get source from positional args
	if fs.NArg() < 1 {
		return fmt.Errorf(`source is required

Usage: mockd import <source>

Run 'mockd import --help' for more options`)
	}
	source := fs.Arg(0)

	// Check if source is a cURL command
	var data []byte
	var filename string

	if len(source) > 5 && source[:4] == "curl" {
		data = []byte(source)
		filename = "curl-command"
	} else {
		// Read file
		var err error
		data, err = os.ReadFile(source)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf(`file not found: %s

Suggestions:
  • Check the file path is correct
  • Use absolute path if needed`, source)
			}
			return fmt.Errorf("failed to read file: %w", err)
		}
		filename = filepath.Base(source)
	}

	// Detect or parse format
	var importFormat portability.Format
	if *format != "" {
		importFormat = portability.ParseFormat(*format)
		if importFormat == portability.FormatUnknown {
			return fmt.Errorf(`unknown format: %s

Supported formats: mockd, openapi, postman, har, wiremock, curl`, *format)
		}
	} else {
		importFormat = portability.DetectFormat(data, filename)
		if importFormat == portability.FormatUnknown {
			return fmt.Errorf(`unable to detect format from file content

Suggestions:
  • Specify format explicitly with -f/--format
  • Supported formats: mockd, openapi, postman, har, wiremock, curl`)
		}
	}

	// Get importer
	importer := portability.GetImporter(importFormat)
	if importer == nil {
		return fmt.Errorf(`no importer available for format: %s`, importFormat)
	}

	// Handle HAR-specific options
	if importFormat == portability.FormatHAR {
		if harImporter, ok := importer.(*portability.HARImporter); ok {
			harImporter.IncludeStatic = *includeStatic
		}
	}

	// Import
	collection, err := importer.Import(data)
	if err != nil {
		return fmt.Errorf("import failed: %w", err)
	}

	fmt.Printf("Parsed %d mocks from %s (format: %s)\n", len(collection.Mocks), source, importFormat)

	// Dry run - just show what would be imported
	if *dryRun {
		fmt.Println("\nDry run - mocks would be imported:")
		for _, mock := range collection.Mocks {
			method := "???"
			path := "???"
			if mock.HTTP != nil && mock.HTTP.Matcher != nil {
				method = mock.HTTP.Matcher.Method
				path = mock.HTTP.Matcher.Path
			}
			name := mock.Name
			if name == "" {
				name = mock.ID
			}
			fmt.Printf("  • %s %s (%s)\n", method, path, name)
		}
		return nil
	}

	// Create admin client and import config
	client := NewAdminClient(*adminURL)
	result, err := client.ImportConfig(collection, *replace)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	fmt.Printf("Imported %d mocks to server\n", result.Imported)
	if *replace {
		fmt.Printf("Total mocks: %d\n", result.Total)
	}

	return nil
}
