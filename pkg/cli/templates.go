package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/getmockd/mockd/pkg/portability"
	"github.com/spf13/cobra"
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

var (
	templatesListCategory string
	templatesListBaseURL  string

	templatesAddOutput  string
	templatesAddBaseURL string
	templatesAddDryRun  bool
)

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Manage mock templates from the official template library",
	Long: `Manage mock templates from the official template library.

Examples:
  # List all templates
  mockd templates list

  # List templates by category
  mockd templates list --category protocols

  # Add a template to current server
  mockd templates add services/openai/chat-completions

  # Save template to file
  mockd templates add protocols/websocket/chat -o websocket.yaml`,
}

var templatesListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List available templates from the template library",
	RunE: func(cmd *cobra.Command, args []string) error {
		category := &templatesListCategory
		baseURL := &templatesListBaseURL

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
	},
}

var templatesAddCmd = &cobra.Command{
	Use:     "add <template-id>",
	Aliases: []string{"get"},
	Short:   "Download and import a template from the template library",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		templateID := args[0]

		// Fetch the template
		templateURL := fmt.Sprintf("%s/%s/template.yaml", templatesAddBaseURL, templateID)
		data, err := fetchURL(templateURL)
		if err != nil {
			return fmt.Errorf("failed to fetch template %s: %w\n\nRun 'mockd templates list' to see available templates", templateID, err)
		}

		// Dry run - just print the content
		if templatesAddDryRun {
			fmt.Printf("# Template: %s\n", templateID)
			fmt.Printf("# Source: %s\n\n", templateURL)
			fmt.Print(string(data))
			return nil
		}

		// Save to file
		if templatesAddOutput != "" {
			// Ensure parent directory exists
			dir := filepath.Dir(templatesAddOutput)
			if dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}
			}

			if err := os.WriteFile(templatesAddOutput, data, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			fmt.Printf("Template saved to %s\n", templatesAddOutput)
			fmt.Println("\nTo use this template:")
			fmt.Printf("  mockd serve --load %s\n", templatesAddOutput)
			return nil
		}

		// Parse the template
		importer := portability.GetImporter(portability.FormatMockd)
		collection, err := importer.Import(data)
		if err != nil {
			return fmt.Errorf("failed to parse template: %w", err)
		}

		// Import to running server
		client := NewAdminClientWithAuth(adminURL)
		result, err := client.ImportConfig(collection, false)
		if err != nil {
			// Check if server is not running
			if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "cannot connect") {
				return fmt.Errorf(`cannot connect to mockd server at %s

Suggestions:
  • Start the server with: mockd serve
  • Save to file instead: mockd templates add %s -o template.yaml`, adminURL, templateID)
			}
			return fmt.Errorf("failed to import template: %w", err)
		}

		fmt.Printf("Imported template '%s' (%d mocks)\n", templateID, result.Imported)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(templatesCmd)

	templatesCmd.AddCommand(templatesListCmd)
	templatesListCmd.Flags().StringVarP(&templatesListCategory, "category", "c", "", "Filter by category (protocols, services, patterns)")
	templatesListCmd.Flags().StringVar(&templatesListBaseURL, "base-url", defaultTemplatesBaseURL, "Templates repository base URL")

	templatesCmd.AddCommand(templatesAddCmd)
	templatesAddCmd.Flags().StringVarP(&templatesAddOutput, "output", "o", "", "Save to file instead of importing to server")
	templatesAddCmd.Flags().StringVar(&templatesAddBaseURL, "base-url", defaultTemplatesBaseURL, "Templates repository base URL")
	templatesAddCmd.Flags().BoolVar(&templatesAddDryRun, "dry-run", false, "Preview template content without importing")
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
