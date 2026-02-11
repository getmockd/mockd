package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/portability"
)

const (
	defaultTemplatesBaseURL = "https://raw.githubusercontent.com/getmockd/mockd-templates/main"
	templatesIndexURL       = defaultTemplatesBaseURL + "/templates.json"
)

// TemplatesIndex represents the templates.json structure.
type TemplatesIndex struct {
	Version    string             `json:"version"`
	Updated    string             `json:"updated"`
	Templates  []TemplateInfo     `json:"templates"`
	ComingSoon []TemplateInfoBase `json:"coming_soon"`
}

// TemplateInfo represents a template entry.
type TemplateInfo struct {
	TemplateInfoBase
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
}

// TemplateInfoBase represents minimal template info.
type TemplateInfoBase struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// RunTemplates handles the templates command.
func RunTemplates(args []string) error {
	if len(args) == 0 {
		return templatesUsage()
	}

	switch args[0] {
	case "list", "ls":
		return templatesListCmd(args[1:])
	case "add", "get":
		return templatesAddCmd(args[1:])
	case "help", "--help", "-h":
		return templatesUsage()
	default:
		return fmt.Errorf("unknown templates subcommand: %s\n\nRun 'mockd templates --help' for usage", args[0])
	}
}

func templatesUsage() error {
	fmt.Print(`Usage: mockd templates <command> [flags]

Manage mock templates from the official template library.

Commands:
  list    List available templates
  add     Download and import a template

Examples:
  # List all templates
  mockd templates list

  # List templates by category
  mockd templates list --category protocols

  # Add a template to current server
  mockd templates add services/openai/chat-completions

  # Save template to file
  mockd templates add protocols/websocket/chat -o websocket.yaml

Run 'mockd templates <command> --help' for command details.
`)
	return nil
}

func templatesListCmd(args []string) error {
	fs := flag.NewFlagSet("templates list", flag.ContinueOnError)
	category := fs.String("category", "", "Filter by category (protocols, services, patterns)")
	fs.StringVar(category, "c", "", "Filter by category (shorthand)")
	baseURL := fs.String("base-url", defaultTemplatesBaseURL, "Templates repository base URL")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd templates list [flags]

List available templates from the template library.

Flags:
  -c, --category  Filter by category: protocols, services, patterns
      --base-url  Custom templates repository URL

Examples:
  mockd templates list
  mockd templates list -c protocols
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Fetch index
	index, err := fetchTemplatesIndex(*baseURL)
	if err != nil {
		return fmt.Errorf("failed to fetch templates index: %w", err)
	}

	// Filter by category if specified
	templates := index.Templates
	if *category != "" {
		filtered := make([]TemplateInfo, 0)
		for _, t := range templates {
			if strings.EqualFold(t.Category, *category) {
				filtered = append(filtered, t)
			}
		}
		templates = filtered
	}

	if len(templates) == 0 {
		if *category != "" {
			fmt.Printf("No templates found in category: %s\n", *category)
		} else {
			fmt.Println("No templates available")
		}
		return nil
	}

	// Group by category
	byCategory := make(map[string][]TemplateInfo)
	for _, t := range templates {
		byCategory[t.Category] = append(byCategory[t.Category], t)
	}

	fmt.Printf("Available templates (updated: %s)\n\n", index.Updated)

	// Print in order: protocols, services, patterns
	order := []string{"protocols", "services", "patterns"}
	for _, cat := range order {
		if tmps, ok := byCategory[cat]; ok {
			fmt.Printf("%s:\n", cases.Title(language.English).String(cat))
			for _, t := range tmps {
				fmt.Printf("  %-40s  %s\n", t.ID, t.Description)
			}
			fmt.Println()
		}
	}

	// Print any other categories
	for cat, tmps := range byCategory {
		found := false
		for _, o := range order {
			if o == cat {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("%s:\n", cases.Title(language.English).String(cat))
			for _, t := range tmps {
				fmt.Printf("  %-40s  %s\n", t.ID, t.Description)
			}
			fmt.Println()
		}
	}

	if len(index.ComingSoon) > 0 {
		fmt.Println("Coming soon:")
		for _, t := range index.ComingSoon {
			fmt.Printf("  %-40s  %s\n", t.ID, t.Description)
		}
		fmt.Println()
	}

	fmt.Println("Use 'mockd templates add <template-id>' to download a template")

	return nil
}

func templatesAddCmd(args []string) error {
	fs := flag.NewFlagSet("templates add", flag.ContinueOnError)
	output := fs.String("output", "", "Save to file instead of importing to server")
	fs.StringVar(output, "o", "", "Save to file (shorthand)")
	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	baseURL := fs.String("base-url", defaultTemplatesBaseURL, "Templates repository base URL")
	dryRun := fs.Bool("dry-run", false, "Preview template content without importing")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd templates add <template-id> [flags]

Download and import a template from the template library.

Arguments:
  template-id   Template identifier (e.g., services/openai/chat-completions)

Flags:
  -o, --output     Save to file instead of importing to running server
      --dry-run    Preview template content without importing
      --admin-url  Admin API base URL (default: http://localhost:4290)
      --base-url   Custom templates repository URL

Examples:
  # Import template to running server
  mockd templates add services/openai/chat-completions

  # Save template to file
  mockd templates add protocols/websocket/chat -o websocket.yaml

  # Preview template
  mockd templates add services/stripe/webhooks --dry-run
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return errors.New(`template ID is required

Usage: mockd templates add <template-id>

Run 'mockd templates list' to see available templates`)
	}

	templateID := fs.Arg(0)

	// Fetch the template
	templateURL := fmt.Sprintf("%s/%s/template.yaml", *baseURL, templateID)
	data, err := fetchURL(templateURL)
	if err != nil {
		return fmt.Errorf("failed to fetch template %s: %w\n\nRun 'mockd templates list' to see available templates", templateID, err)
	}

	// Dry run - just print the content
	if *dryRun {
		fmt.Printf("# Template: %s\n", templateID)
		fmt.Printf("# Source: %s\n\n", templateURL)
		fmt.Print(string(data))
		return nil
	}

	// Save to file
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
		fmt.Printf("Template saved to %s\n", *output)
		fmt.Println("\nTo use this template:")
		fmt.Printf("  mockd serve --load %s\n", *output)
		return nil
	}

	// Parse the template
	importer := portability.GetImporter(portability.FormatMockd)
	collection, err := importer.Import(data)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Import to running server
	client := NewAdminClientWithAuth(*adminURL)
	result, err := client.ImportConfig(collection, false)
	if err != nil {
		// Check if server is not running
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "cannot connect") {
			return fmt.Errorf(`cannot connect to mockd server at %s

Suggestions:
  • Start the server with: mockd serve
  • Save to file instead: mockd templates add %s -o template.yaml`, *adminURL, templateID)
		}
		return fmt.Errorf("failed to import template: %w", err)
	}

	fmt.Printf("Imported template '%s' (%d mocks)\n", templateID, result.Imported)

	return nil
}

// fetchTemplatesIndex fetches and parses the templates index.
func fetchTemplatesIndex(baseURL string) (*TemplatesIndex, error) {
	indexURL := baseURL + "/templates.json"
	data, err := fetchURL(indexURL)
	if err != nil {
		return nil, err
	}

	var index TemplatesIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse templates index: %w", err)
	}

	return &index, nil
}

// fetchURL fetches content from a URL.
func fetchURL(url string) ([]byte, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == 404 {
		return nil, errors.New("not found")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}
