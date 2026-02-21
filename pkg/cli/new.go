package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/portability"
	"github.com/spf13/cobra"
)

var (
	newTemplate string
	newName     string
	newOutput   string
	newResource string
)

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new mock collection from a template",
	Long: `Create a new mock collection from a template.

Templates:
  blank       Empty mock collection
  crud        REST CRUD endpoints (GET list, GET one, POST, PUT, DELETE)
  auth        Authentication flow (login, logout, refresh, me)
  pagination  List endpoints with cursor/offset pagination
  errors      Common HTTP error responses (400, 401, 403, 404, 500)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get the template
		tmpl := portability.GetTemplate(newTemplate)
		if tmpl == nil {
			templates := portability.ListTemplates()
			available := make([]string, 0, len(templates))
			for _, t := range templates {
				available = append(available, t.Name)
			}
			return fmt.Errorf("unknown template: %s\n\nAvailable templates: %s\n\nUse 'mockd new --help' for more information", newTemplate, strings.Join(available, ", "))
		}

		// Build parameters
		params := make(map[string]string)
		if newName != "" {
			params["name"] = newName
		}
		if newResource != "" {
			params["resource"] = newResource
		}

		// Generate the collection
		collection, err := tmpl.Generate(params)
		if err != nil {
			return fmt.Errorf("failed to generate from template: %w", err)
		}

		var data []byte
		if newOutput != "" {
			ext := strings.ToLower(filepath.Ext(newOutput))
			if ext == ".yaml" || ext == ".yml" {
				data, err = config.ToYAML(collection)
			} else {
				data, err = config.ToJSON(collection)
			}
		} else {
			// Default to YAML for stdout
			data, err = config.ToYAML(collection)
		}
		if err != nil {
			return fmt.Errorf("failed to marshal collection: %w", err)
		}

		// Output
		if newOutput != "" {
			dir := filepath.Dir(newOutput)
			if dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}
			}

			if err := os.WriteFile(newOutput, data, 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			fmt.Printf("Created %s with %d mock(s) using '%s' template\n", newOutput, len(collection.Mocks), newTemplate)
		} else {
			fmt.Print(string(data))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
	newCmd.Flags().StringVarP(&newTemplate, "template", "t", "blank", "Template: blank, crud, auth, pagination, errors")
	newCmd.Flags().StringVarP(&newName, "name", "n", "", "Collection name")
	newCmd.Flags().StringVarP(&newOutput, "output", "o", "", "Output file")
	newCmd.Flags().StringVar(&newResource, "resource", "", "Resource name (for crud/pagination templates)")
}
