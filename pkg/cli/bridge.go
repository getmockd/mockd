package cli

import (
	"errors"
	"fmt"
	"io"
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
  mockoon    Mockoon environment JSON
  curl       cURL command
  wsdl       WSDL 1.1 service definition (generates SOAP mocks)`,
	Example: `  # Import from OpenAPI spec (auto-detected)
  mockd import openapi.yaml

  # Import from Postman collection
  mockd import collection.json -f postman

  # Import from HAR file including static assets
  mockd import recording.har --include-static

  # Import from Mockoon environment
  mockd import environment.json -f mockoon

  # Import from cURL command
  mockd import "curl -X POST https://api.example.com/users -H 'Content-Type: application/json' -d '{\"name\": \"test\"}'"

  # Preview import without saving
  mockd import openapi.yaml --dry-run

  # Replace all mocks with imported ones
  mockd import mocks.yaml --replace`,
	Args: cobra.MaximumNArgs(1),
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
	// Handle no-argument case: read from stdin
	if len(args) == 0 {
		return runImportFromStdin()
	}

	source := args[0]

	// Check if source is a directory
	info, err := os.Stat(source)
	if err == nil && info.IsDir() {
		return runImportFromDirectory(source)
	}

	// Check if source is a cURL command
	var data []byte
	var filename string

	if len(source) > 4 && source[:4] == "curl" {
		data = []byte(source)
		filename = "curl-command"
	} else {
		// Read file
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

	return importData(data, source, impFormat)
}

// runImportFromStdin reads mock data from stdin when no arguments are provided.
func runImportFromStdin() error {
	// Check if stdin is a pipe
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return errors.New(`no input provided

Usage:
  mockd import <file>           Import from a file
  mockd import <directory>      Import all files from a directory
  cat file.yaml | mockd import  Import from stdin

Run 'mockd import --help' for more options`)
	}

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}

	if len(data) == 0 {
		return errors.New("stdin is empty — nothing to import")
	}

	// Detect or use explicit format
	var impFormat portability.Format
	if importFormat != "" {
		impFormat = portability.ParseFormat(importFormat)
		if impFormat == portability.FormatUnknown {
			return fmt.Errorf("unknown format: %s\n\nSupported formats: mockd, openapi, postman, har, wiremock, curl", importFormat)
		}
	} else {
		impFormat = portability.DetectFormat(data, "stdin")
		if impFormat == portability.FormatUnknown {
			return errors.New("unable to detect format from stdin content\n\nSuggestions:\n  • Specify format explicitly with -f/--format\n  • Supported formats: mockd, openapi, postman, har, wiremock, curl")
		}
	}

	return importData(data, "stdin", impFormat)
}

// runImportFromDirectory imports all .json, .yaml, .yml files from a directory.
func runImportFromDirectory(dir string) error {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".json" || ext == ".yaml" || ext == ".yml" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to walk directory: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no .json, .yaml, or .yml files found in %s", dir)
	}

	totalImported := 0
	totalFiles := 0
	var importErrors []string

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			importErrors = append(importErrors, fmt.Sprintf("%s: %v", file, err))
			continue
		}

		filename := filepath.Base(file)
		impFormat := portability.DetectFormat(data, filename)
		if impFormat == portability.FormatUnknown {
			importErrors = append(importErrors, file+": unable to detect format")
			continue
		}

		importer := portability.GetImporter(impFormat)
		if importer == nil {
			importErrors = append(importErrors, fmt.Sprintf("%s: no importer for format %s", file, impFormat))
			continue
		}

		collection, err := importer.Import(data)
		if err != nil {
			importErrors = append(importErrors, fmt.Sprintf("%s: %v", file, err))
			continue
		}

		client := NewAdminClientWithAuth(adminURL)
		result, err := client.ImportConfig(collection, false) // never replace when importing directories
		if err != nil {
			importErrors = append(importErrors, fmt.Sprintf("%s: %v", file, FormatConnectionError(err)))
			continue
		}

		totalImported += result.Imported
		totalFiles++
	}

	printResult(map[string]any{
		"imported": totalImported,
		"files":    totalFiles,
		"errors":   len(importErrors),
		"source":   dir,
	}, func() {
		fmt.Printf("Imported %d mocks from %d files in %s\n", totalImported, totalFiles, dir)
		if len(importErrors) > 0 {
			fmt.Printf("\n%d files had errors:\n", len(importErrors))
			for _, e := range importErrors {
				fmt.Printf("  • %s\n", e)
			}
		}
	})

	return nil
}

// importData imports data with a known format and sends to the admin server.
func importData(data []byte, source string, impFormat portability.Format) error {
	importer := portability.GetImporter(impFormat)
	if importer == nil {
		return fmt.Errorf("no importer available for format: %s", impFormat)
	}

	if impFormat == portability.FormatHAR {
		if harImporter, ok := importer.(*portability.HARImporter); ok {
			harImporter.IncludeStatic = importIncludeStatic
		}
	}

	collection, err := importer.Import(data)
	if err != nil {
		return formatImportError(err, impFormat, source)
	}

	if !jsonOutput {
		fmt.Printf("Parsed %d mocks from %s (format: %s)\n", len(collection.Mocks), source, impFormat)
	}

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
