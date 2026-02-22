package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/portability"
	"github.com/spf13/cobra"
)

// ─── init command ────────────────────────────────────────────────────────────

var (
	initForce       bool
	initOutput      string
	initFormat      string
	initDefaults    bool
	initInteractive bool
	initTemplate    string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a starter mockd.yaml configuration file",
	Long: `Create a starter mockd.yaml configuration file.

Built-in Templates:
  minimal          Just admin + engine + one health mock
  full             Admin + engine + workspace + sample mocks
  api              Setup for REST API mocking with CRUD examples

Protocol Templates:
  default          Basic HTTP mocks (hello, echo, health)
  crud             Full REST CRUD API for resources
  websocket-chat   Chat room WebSocket endpoint with echo
  graphql-api      GraphQL API with User CRUD resolvers
  grpc-service     gRPC Greeter service with reflection
  mqtt-iot         MQTT broker with IoT sensor topics`,
	Example: `  # Interactive wizard (default)
  mockd init

  # Generate minimal config without prompts
  mockd init --defaults

  # Use a built-in template
  mockd init --template full

  # Use a protocol template
  mockd init --template graphql-api

  # List all available templates
  mockd init --template list

  # Custom output file
  mockd init -o my-mocks.yaml

  # Overwrite existing config
  mockd init --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunInit([]string{}) // RunInit reads from package-level vars now
	},
}

// ─── import command ──────────────────────────────────────────────────────────

var (
	importFormat        string
	importReplace       bool
	importDryRun        bool
	importIncludeStatic bool
)

var importCmd = &cobra.Command{
	Use:   "import <source>",
	Short: "Import mocks from various sources and formats",
	Long: `Import mocks from various sources and formats.

Supported Formats:
  mockd      Mockd native format (YAML/JSON)
  openapi    OpenAPI 3.x or Swagger 2.0
  postman    Postman Collection v2.x
  har        HTTP Archive (browser recordings)
  wiremock   WireMock JSON mappings
  curl       cURL command`,
	Example: `  # Import from OpenAPI spec (auto-detected)
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
  mockd import mocks.yaml --replace`,
	Args: cobra.ExactArgs(1),
	RunE: runImportCobra,
}

// ─── export command ──────────────────────────────────────────────────────────

var (
	exportOutput  string
	exportName    string
	exportFormat  string
	exportVersion string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export current mock configuration to various formats",
	Long: `Export current mock configuration to various formats.

Formats:
  mockd    Mockd native format (YAML/JSON) - recommended for portability
  openapi  OpenAPI 3.x specification - for API documentation`,
	Example: `  # Export to stdout as YAML
  mockd export

  # Export to JSON file
  mockd export -o mocks.json

  # Export to YAML file
  mockd export -o mocks.yaml

  # Export as OpenAPI specification
  mockd export -f openapi -o api.yaml

  # Export with custom name
  mockd export -n "My API Mocks" -o mocks.yaml`,
	Args: cobra.NoArgs,
	RunE: runExportCobra,
}

func init() {
	// init command flags
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite existing config file")
	initCmd.Flags().StringVarP(&initOutput, "output", "o", "mockd.yaml", "Output filename")
	initCmd.Flags().StringVar(&initFormat, "format", "", "Output format: yaml or json (default: inferred from filename)")
	initCmd.Flags().BoolVar(&initDefaults, "defaults", false, "Generate minimal config without prompts")
	initCmd.Flags().BoolVarP(&initInteractive, "interactive", "i", false, "Interactive mode - prompts for configuration")
	initCmd.Flags().StringVarP(&initTemplate, "template", "t", "", "Use predefined template")
	rootCmd.AddCommand(initCmd)

	// import command flags
	importCmd.Flags().StringVarP(&importFormat, "format", "f", "", "Force format (auto-detected if omitted)")
	importCmd.Flags().BoolVar(&importReplace, "replace", false, "Replace all existing mocks")
	importCmd.Flags().BoolVar(&importDryRun, "dry-run", false, "Preview import without saving")
	importCmd.Flags().BoolVar(&importIncludeStatic, "include-static", false, "Include static assets (for HAR imports)")
	rootCmd.AddCommand(importCmd)

	// export command flags
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "", "Output file (default: stdout)")
	exportCmd.Flags().StringVarP(&exportName, "name", "n", "exported-config", "Collection name")
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "mockd", "Output format: mockd, openapi")
	exportCmd.Flags().StringVar(&exportVersion, "version", "", "Version tag for the export")
	rootCmd.AddCommand(exportCmd)
}

// runImportCobra is the Cobra RunE for the import command.
func runImportCobra(cmd *cobra.Command, args []string) error {
	source := args[0]

	// Check if source is a cURL command
	var data []byte
	var filename string

	if len(source) > 4 && source[:4] == "curl" {
		data = []byte(source)
		filename = "curl-command"
	} else {
		// Read file
		var err error
		data, err = os.ReadFile(source)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("file not found: %s\n\nSuggestions:\n  • Check the file path is correct\n  • Use absolute path if needed", source)
			}
			return fmt.Errorf("failed to read file: %w", err)
		}
		filename = filepath.Base(source)
	}

	// Detect or parse format
	var impFormat portability.Format
	if importFormat != "" {
		impFormat = portability.ParseFormat(importFormat)
		if impFormat == portability.FormatUnknown {
			return fmt.Errorf("unknown format: %s\n\nSupported formats: mockd, openapi, postman, har, wiremock, curl", importFormat)
		}
	} else {
		impFormat = portability.DetectFormat(data, filename)
		if impFormat == portability.FormatUnknown {
			return errors.New("unable to detect format from file content\n\nSuggestions:\n  • Specify format explicitly with -f/--format\n  • Supported formats: mockd, openapi, postman, har, wiremock, curl")
		}
	}

	// Get importer
	importer := portability.GetImporter(impFormat)
	if importer == nil {
		return fmt.Errorf("no importer available for format: %s", impFormat)
	}

	// Handle HAR-specific options
	if impFormat == portability.FormatHAR {
		if harImporter, ok := importer.(*portability.HARImporter); ok {
			harImporter.IncludeStatic = importIncludeStatic
		}
	}

	// Import
	collection, err := importer.Import(data)
	if err != nil {
		return formatImportError(err, impFormat, source)
	}

	if !jsonOutput {
		fmt.Printf("Parsed %d mocks from %s (format: %s)\n", len(collection.Mocks), source, impFormat)
	}

	// Dry run - just show what would be imported
	if importDryRun {
		type dryRunMock struct {
			ID     string `json:"id,omitempty"`
			Name   string `json:"name,omitempty"`
			Method string `json:"method"`
			Path   string `json:"path"`
		}
		mocks := make([]dryRunMock, 0, len(collection.Mocks))
		for _, m := range collection.Mocks {
			method, path := "???", "???"
			if m.HTTP != nil && m.HTTP.Matcher != nil {
				method = m.HTTP.Matcher.Method
				path = m.HTTP.Matcher.Path
			}
			name := m.Name
			if name == "" {
				name = m.ID
			}
			mocks = append(mocks, dryRunMock{ID: m.ID, Name: name, Method: method, Path: path})
		}
		printResult(map[string]any{"dryRun": true, "format": impFormat.String(), "count": len(mocks), "mocks": mocks}, func() {
			fmt.Println("\nDry run - mocks would be imported:")
			for _, m := range mocks {
				fmt.Printf("  • %s %s (%s)\n", m.Method, m.Path, m.Name)
			}
		})
		return nil
	}

	// Create admin client and import config — uses root persistent adminURL
	client := NewAdminClientWithAuth(adminURL)
	result, err := client.ImportConfig(collection, importReplace)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	printResult(map[string]any{"imported": result.Imported, "total": result.Total, "format": impFormat.String(), "replace": importReplace}, func() {
		fmt.Printf("Imported %d mocks to server\n", result.Imported)
		if importReplace {
			fmt.Printf("Total mocks: %d\n", result.Total)
		}
	})

	return nil
}

// runExportCobra is the Cobra RunE for the export command.
func runExportCobra(cmd *cobra.Command, args []string) error {
	// Parse format
	expFormat := portability.ParseFormat(exportFormat)
	if expFormat == portability.FormatUnknown {
		return fmt.Errorf("invalid format: %s\n\nSupported formats: mockd, openapi", exportFormat)
	}

	if !expFormat.CanExport() {
		return fmt.Errorf("format '%s' does not support export\n\nSupported export formats: mockd, openapi", exportFormat)
	}

	// Create admin client and export config — uses root persistent adminURL
	client := NewAdminClientWithAuth(adminURL)
	collection, err := client.ExportConfig(exportName)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Apply version tag if specified
	if exportVersion != "" {
		collection.Version = exportVersion
	}

	// Determine output format (YAML vs JSON)
	formatLower := strings.ToLower(strings.TrimSpace(exportFormat))
	asYAML := true
	switch {
	case formatLower == "json":
		asYAML = false
	case formatLower == "yaml" || formatLower == "yml":
		asYAML = true
	case exportOutput != "":
		ext := strings.ToLower(filepath.Ext(exportOutput))
		asYAML = ext == ".yaml" || ext == ".yml"
	}

	// Get the appropriate exporter
	var data []byte
	switch expFormat {
	case portability.FormatMockd:
		if asYAML {
			data, err = config.ToYAML(collection)
		} else {
			data, err = config.ToJSON(collection)
		}
	case portability.FormatOpenAPI:
		exporter := &portability.OpenAPIExporter{AsYAML: asYAML}
		data, err = exporter.Export(collection)
	default:
		return fmt.Errorf("unsupported export format: %s", expFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to export: %w", err)
	}

	// Output to file or stdout
	if exportOutput != "" {
		if err := os.WriteFile(exportOutput, data, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		printResult(map[string]any{
			"exported": len(collection.Mocks),
			"format":   expFormat.String(),
			"path":     exportOutput,
		}, func() {
			fmt.Printf("Exported %d mocks to %s (format: %s)\n", len(collection.Mocks), exportOutput, expFormat)
		})
	} else {
		// Stdout: in JSON mode, wrap the raw config in a result envelope;
		// in text mode, print the raw YAML/JSON data directly.
		if jsonOutput {
			printResult(map[string]any{
				"exported": len(collection.Mocks),
				"format":   expFormat.String(),
				"data":     string(data),
			}, nil)
		} else {
			fmt.Print(string(data))
		}
	}

	return nil
}
