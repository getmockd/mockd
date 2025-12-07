package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/getmockd/mockd/internal/cliconfig"
	"github.com/getmockd/mockd/pkg/config"
)

// RunImport handles the import command.
func RunImport(args []string) error {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)

	replace := fs.Bool("replace", false, "Replace all existing mocks")
	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd import <file> [flags]

Import mocks from a configuration file.

Arguments:
  file    Path to JSON configuration file (required)

Flags:
      --replace      Replace all existing mocks (default: merge)
      --admin-url    Admin API base URL (default: http://localhost:9090)

Examples:
  # Add mocks from file (merge with existing)
  mockd import mocks.json

  # Replace all mocks with file contents
  mockd import mocks.json --replace
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Get file path from positional args
	if fs.NArg() < 1 {
		return fmt.Errorf(`file path is required

Usage: mockd import <file>

Run 'mockd import --help' for more options`)
	}
	filePath := fs.Arg(0)

	// Read and parse file
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf(`file not found: %s

Suggestions:
  • Check the file path is correct
  • Use absolute path if needed`, filePath)
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse JSON with error location
	var collection config.MockCollection
	if err := json.Unmarshal(data, &collection); err != nil {
		// Try to get line/column info from JSON error
		if syntaxErr, ok := err.(*json.SyntaxError); ok {
			line, col := cliconfig.FindLineColumn(data, syntaxErr.Offset)
			return fmt.Errorf(`invalid JSON at line %d, column %d: %s

Suggestions:
  • Check for missing commas or brackets
  • Validate JSON syntax: cat %s | jq .`, line, col, syntaxErr.Error(), filePath)
		}
		return fmt.Errorf(`invalid JSON: %s

Suggestions:
  • Check for missing commas or brackets
  • Validate JSON syntax: cat %s | jq .`, err.Error(), filePath)
	}

	// Create admin client and import config
	client := NewAdminClient(*adminURL)
	result, err := client.ImportConfig(&collection, *replace)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	fmt.Printf("Imported %d mocks from %s\n", result.Imported, filePath)
	if *replace {
		fmt.Printf("Total mocks: %d\n", result.Total)
	}

	return nil
}
