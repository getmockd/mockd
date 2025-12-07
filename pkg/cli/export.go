package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/getmockd/mockd/internal/cliconfig"
)

// RunExport handles the export command.
func RunExport(args []string) error {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)

	output := fs.String("output", "", "Output file (default: stdout)")
	fs.StringVar(output, "o", "", "Output file (shorthand)")
	name := fs.String("name", "exported-config", "Collection name")
	fs.StringVar(name, "n", "exported-config", "Collection name (shorthand)")
	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd export [flags]

Export current mock configuration.

Flags:
  -o, --output       Output file (default: stdout)
  -n, --name         Collection name (default: exported-config)
      --admin-url    Admin API base URL (default: http://localhost:9090)

Examples:
  # Export to stdout
  mockd export

  # Export to file
  mockd export -o mocks.json

  # Export with custom name
  mockd export -n "My API Mocks" -o mocks.json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Create admin client and export config
	client := NewAdminClient(*adminURL)
	collection, err := client.ExportConfig(*name)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Format JSON with indentation
	data, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode JSON: %w", err)
	}

	// Output to file or stdout
	if *output != "" {
		if err := os.WriteFile(*output, append(data, '\n'), 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Printf("Exported %d mocks to %s\n", len(collection.Mocks), *output)
	} else {
		fmt.Println(string(data))
	}

	return nil
}
