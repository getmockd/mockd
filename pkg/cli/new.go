package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getmockd/mockd/pkg/portability"
)

// RunNew handles the new command for creating mocks from templates.
func RunNew(args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)

	template := fs.String("template", "blank", "Template: blank, crud, auth, pagination, errors")
	fs.StringVar(template, "t", "blank", "Template (shorthand)")
	name := fs.String("name", "", "Collection name")
	fs.StringVar(name, "n", "", "Collection name (shorthand)")
	output := fs.String("output", "", "Output file")
	fs.StringVar(output, "o", "", "Output file (shorthand)")
	resource := fs.String("resource", "", "Resource name (for crud/pagination templates)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd new [flags]

Create a new mock collection from a template.

Flags:
  -t, --template     Template: blank, crud, auth, pagination, errors (default: blank)
  -n, --name         Collection name
  -o, --output       Output file (default: stdout)
      --resource     Resource name (for crud/pagination templates)

Templates:
  blank       Empty mock collection
  crud        REST CRUD endpoints (GET list, GET one, POST, PUT, DELETE)
  auth        Authentication flow (login, logout, refresh, me)
  pagination  List endpoints with cursor/offset pagination
  errors      Common HTTP error responses (400, 401, 403, 404, 500)

Examples:
  # Create a blank collection
  mockd new -t blank -o mocks.yaml

  # Create CRUD endpoints for users
  mockd new -t crud --resource users -o users.yaml

  # Create auth endpoints
  mockd new -t auth -n "Auth API" -o auth.yaml

  # List available templates
  mockd new --help
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Get the template
	tmpl := portability.GetTemplate(*template)
	if tmpl == nil {
		available := []string{}
		for _, t := range portability.ListTemplates() {
			available = append(available, t.Name)
		}
		return fmt.Errorf(`unknown template: %s

Available templates: %s

Use 'mockd new --help' for more information`, *template, strings.Join(available, ", "))
	}

	// Build parameters
	params := make(map[string]string)
	if *name != "" {
		params["name"] = *name
	}
	if *resource != "" {
		params["resource"] = *resource
	}

	// Generate the collection
	collection, err := tmpl.Generate(params)
	if err != nil {
		return fmt.Errorf("failed to generate from template: %w", err)
	}

	// Determine output format
	asYAML := true
	if *output != "" {
		ext := strings.ToLower(filepath.Ext(*output))
		asYAML = ext == ".yaml" || ext == ".yml"
	}

	// Export the collection
	exporter := &portability.NativeExporter{AsYAML: asYAML}
	data, err := exporter.Export(collection)
	if err != nil {
		return fmt.Errorf("failed to export collection: %w", err)
	}

	// Output
	if *output != "" {
		// Ensure parent directory exists
		dir := filepath.Dir(*output)
		if dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		}

		if err := os.WriteFile(*output, data, 0644); err != nil {
			return fmt.Errorf("failed to write file: %w", err)
		}
		fmt.Printf("Created %s with %d mock(s) using '%s' template\n", *output, len(collection.Mocks), *template)
	} else {
		fmt.Print(string(data))
	}

	return nil
}
