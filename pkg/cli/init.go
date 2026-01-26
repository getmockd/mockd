package cli

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/getmockd/mockd/pkg/config"
	"gopkg.in/yaml.v3"
)

// initConfig holds the configuration gathered during init.
type initConfig struct {
	AdminPort   int
	HTTPPort    int
	EnableHTTPS bool
	HTTPSPort   int
	AuthType    string // "none" or "api-key"
}

// defaultInitConfig returns default values for init configuration.
func defaultInitConfig() *initConfig {
	return &initConfig{
		AdminPort:   4290,
		HTTPPort:    4280,
		EnableHTTPS: false,
		HTTPSPort:   4443,
		AuthType:    "none",
	}
}

// RunInit handles the init command for creating a starter config file.
func RunInit(args []string) error {
	return runInitWithIO(args, os.Stdin, os.Stdout, os.Stderr)
}

// runInitWithIO is the testable version of RunInit that accepts custom I/O.
func runInitWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)

	force := fs.Bool("force", false, "Overwrite existing config file")
	output := fs.String("output", "mockd.yaml", "Output filename")
	fs.StringVar(output, "o", "mockd.yaml", "Output filename (shorthand)")
	format := fs.String("format", "", "Output format: yaml or json (default: inferred from filename)")
	defaults := fs.Bool("defaults", false, "Generate minimal config without prompts")
	template := fs.String("template", "", "Use predefined template (minimal, full, api)")
	fs.StringVar(template, "t", "", "Use predefined template (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(stderr, `Usage: mockd init [flags]

Create a starter mockd.yaml configuration file.

Flags:
      --force           Overwrite existing config file
  -o, --output          Output filename (default: mockd.yaml)
      --format          Output format: yaml or json (default: inferred from filename)
      --defaults        Generate minimal config without prompts
  -t, --template        Use predefined template (minimal, full, api)

Templates:
  minimal    Just admin + engine + one health mock
  full       Admin + engine + workspace + sample mocks
  api        Setup for REST API mocking with CRUD examples

Examples:
  # Interactive wizard (default)
  mockd init

  # Generate minimal config without prompts
  mockd init --defaults

  # Use a specific template
  mockd init --template full

  # Custom output file
  mockd init -o my-mocks.yaml

  # Overwrite existing config
  mockd init --force
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	// Determine output format
	outputFormat := strings.ToLower(*format)
	if outputFormat == "" {
		// Infer from filename extension
		ext := strings.ToLower(filepath.Ext(*output))
		if ext == ".json" {
			outputFormat = "json"
		} else {
			outputFormat = "yaml"
		}
	}

	// Validate format
	if outputFormat != "yaml" && outputFormat != "json" {
		return fmt.Errorf("invalid format: %s (must be yaml or json)", outputFormat)
	}

	// Check if file already exists
	if _, err := os.Stat(*output); err == nil {
		if !*force {
			return fmt.Errorf("file already exists: %s\n\nUse --force to overwrite", *output)
		}
	}

	// Build the config based on flags
	var cfg *config.ProjectConfig
	var err error

	if *template != "" {
		// Use predefined template
		cfg, err = getProjectConfigTemplate(*template)
		if err != nil {
			return err
		}
	} else if *defaults {
		// Generate minimal config without prompts
		cfg = generateMinimalProjectConfig(defaultInitConfig())
	} else {
		// Interactive wizard
		fmt.Fprintln(stdout, "Creating new mockd configuration...")
		fmt.Fprintln(stdout)

		initCfg, err := runInteractiveWizard(stdin, stdout)
		if err != nil {
			return err
		}

		cfg = generateMinimalProjectConfig(initCfg)
	}

	// Generate output
	var data []byte
	if outputFormat == "json" {
		data, err = json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to generate JSON: %w", err)
		}
		data = append(data, '\n')
	} else {
		data, err = generateProjectConfigYAML(cfg)
		if err != nil {
			return fmt.Errorf("failed to generate YAML: %w", err)
		}
	}

	// Write the file
	if err := os.WriteFile(*output, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Print success message
	fmt.Fprintf(stdout, "\nWriting %s...\n", *output)
	fmt.Fprintln(stdout, "Done! Run 'mockd up' to start.")

	return nil
}

// runInteractiveWizard prompts the user for configuration options.
func runInteractiveWizard(stdin io.Reader, stdout io.Writer) (*initConfig, error) {
	reader := bufio.NewReader(stdin)
	cfg := defaultInitConfig()

	// Admin port
	fmt.Fprintf(stdout, "Admin port [%d]: ", cfg.AdminPort)
	if val, err := readIntWithDefault(reader, cfg.AdminPort); err == nil {
		cfg.AdminPort = val
	}

	// Engine HTTP port
	fmt.Fprintf(stdout, "Engine HTTP port [%d]: ", cfg.HTTPPort)
	if val, err := readIntWithDefault(reader, cfg.HTTPPort); err == nil {
		cfg.HTTPPort = val
	}

	// Enable HTTPS?
	fmt.Fprint(stdout, "Enable HTTPS? (y/N): ")
	if val, _ := readBoolWithDefault(reader, false); val {
		cfg.EnableHTTPS = true
		fmt.Fprintf(stdout, "Engine HTTPS port [%d]: ", cfg.HTTPSPort)
		if val, err := readIntWithDefault(reader, cfg.HTTPSPort); err == nil {
			cfg.HTTPSPort = val
		}
	}

	// Auth type
	fmt.Fprintf(stdout, "Auth type (none/api-key) [%s]: ", cfg.AuthType)
	if val, _ := readStringWithDefault(reader, cfg.AuthType); val == "api-key" || val == "none" {
		cfg.AuthType = val
	}

	fmt.Fprintln(stdout)

	return cfg, nil
}

// readIntWithDefault reads an integer from the reader, returning the default if empty.
func readIntWithDefault(reader *bufio.Reader, defaultVal int) (int, error) {
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal, nil
	}
	return strconv.Atoi(input)
}

// readBoolWithDefault reads a boolean (y/n) from the reader, returning the default if empty.
func readBoolWithDefault(reader *bufio.Reader, defaultVal bool) (bool, error) {
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return defaultVal, nil
	}
	return input == "y" || input == "yes", nil
}

// readStringWithDefault reads a string from the reader, returning the default if empty.
func readStringWithDefault(reader *bufio.Reader, defaultVal string) (string, error) {
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	if input == "" {
		return defaultVal, nil
	}
	return input, nil
}

// generateMinimalProjectConfig creates a minimal ProjectConfig from init settings.
func generateMinimalProjectConfig(cfg *initConfig) *config.ProjectConfig {
	projectCfg := &config.ProjectConfig{
		Version: "1",
		Admins: []config.AdminConfig{
			{
				Name: "local",
				Port: cfg.AdminPort,
				Auth: &config.AdminAuthConfig{
					Type: cfg.AuthType,
				},
			},
		},
		Engines: []config.EngineConfig{
			{
				Name:     "default",
				HTTPPort: cfg.HTTPPort,
				Admin:    "local",
			},
		},
		Workspaces: []config.WorkspaceConfig{
			{
				Name:    "default",
				Engines: []string{"default"},
			},
		},
		Mocks: []config.MockEntry{
			{
				ID:        "health",
				Workspace: "default",
				Type:      "http",
				HTTP: &config.HTTPMockConfig{
					Matcher: config.HTTPMatcher{
						Path: "/health",
					},
					Response: config.HTTPResponse{
						StatusCode: 200,
						Body:       `{"status": "ok"}`,
					},
				},
			},
		},
	}

	// Add HTTPS if enabled
	if cfg.EnableHTTPS {
		projectCfg.Engines[0].HTTPSPort = cfg.HTTPSPort
		projectCfg.Engines[0].TLS = &config.TLSConfig{
			Enabled:          true,
			AutoGenerateCert: true,
		}
	}

	return projectCfg
}

// getProjectConfigTemplate returns a ProjectConfig for the given template name.
func getProjectConfigTemplate(name string) (*config.ProjectConfig, error) {
	switch strings.ToLower(name) {
	case "minimal":
		return generateMinimalTemplate(), nil
	case "full":
		return generateFullTemplate(), nil
	case "api":
		return generateAPITemplate(), nil
	default:
		return nil, fmt.Errorf("unknown template: %s\n\nAvailable templates: minimal, full, api", name)
	}
}

// generateMinimalTemplate creates a minimal ProjectConfig template.
func generateMinimalTemplate() *config.ProjectConfig {
	return &config.ProjectConfig{
		Version: "1",
		Admins: []config.AdminConfig{
			{
				Name: "local",
				Port: 4290,
				Auth: &config.AdminAuthConfig{
					Type: "none",
				},
			},
		},
		Engines: []config.EngineConfig{
			{
				Name:     "default",
				HTTPPort: 4280,
				Admin:    "local",
			},
		},
		Workspaces: []config.WorkspaceConfig{
			{
				Name:    "default",
				Engines: []string{"default"},
			},
		},
		Mocks: []config.MockEntry{
			{
				ID:        "health",
				Workspace: "default",
				Type:      "http",
				HTTP: &config.HTTPMockConfig{
					Matcher: config.HTTPMatcher{
						Path: "/health",
					},
					Response: config.HTTPResponse{
						StatusCode: 200,
						Body:       `{"status": "ok"}`,
					},
				},
			},
		},
	}
}

// generateFullTemplate creates a full ProjectConfig template with sample mocks.
func generateFullTemplate() *config.ProjectConfig {
	return &config.ProjectConfig{
		Version: "1",
		Admins: []config.AdminConfig{
			{
				Name: "local",
				Port: 4290,
				Auth: &config.AdminAuthConfig{
					Type: "none",
				},
			},
		},
		Engines: []config.EngineConfig{
			{
				Name:     "default",
				HTTPPort: 4280,
				Admin:    "local",
			},
		},
		Workspaces: []config.WorkspaceConfig{
			{
				Name:        "default",
				Description: "Default workspace for development",
				Engines:     []string{"default"},
			},
		},
		Mocks: []config.MockEntry{
			{
				ID:        "health",
				Workspace: "default",
				Type:      "http",
				HTTP: &config.HTTPMockConfig{
					Matcher: config.HTTPMatcher{
						Path: "/health",
					},
					Response: config.HTTPResponse{
						StatusCode: 200,
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Body: `{"status": "ok"}`,
					},
				},
			},
			{
				ID:        "hello",
				Workspace: "default",
				Type:      "http",
				HTTP: &config.HTTPMockConfig{
					Matcher: config.HTTPMatcher{
						Method: "GET",
						Path:   "/hello",
					},
					Response: config.HTTPResponse{
						StatusCode: 200,
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Body: `{"message": "Hello from mockd!"}`,
					},
				},
			},
			{
				ID:        "echo",
				Workspace: "default",
				Type:      "http",
				HTTP: &config.HTTPMockConfig{
					Matcher: config.HTTPMatcher{
						Method: "POST",
						Path:   "/echo",
					},
					Response: config.HTTPResponse{
						StatusCode: 200,
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Body: `{"received": {{request.body}}}`,
					},
				},
			},
		},
	}
}

// generateAPITemplate creates a ProjectConfig template for REST API mocking with CRUD examples.
func generateAPITemplate() *config.ProjectConfig {
	return &config.ProjectConfig{
		Version: "1",
		Admins: []config.AdminConfig{
			{
				Name: "local",
				Port: 4290,
				Auth: &config.AdminAuthConfig{
					Type: "none",
				},
			},
		},
		Engines: []config.EngineConfig{
			{
				Name:     "default",
				HTTPPort: 4280,
				Admin:    "local",
			},
		},
		Workspaces: []config.WorkspaceConfig{
			{
				Name:        "default",
				Description: "REST API mocking workspace",
				Engines:     []string{"default"},
			},
		},
		Mocks: []config.MockEntry{
			{
				ID:        "health",
				Workspace: "default",
				Type:      "http",
				HTTP: &config.HTTPMockConfig{
					Matcher: config.HTTPMatcher{
						Path: "/health",
					},
					Response: config.HTTPResponse{
						StatusCode: 200,
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Body: `{"status": "ok"}`,
					},
				},
			},
			{
				ID:        "users-list",
				Workspace: "default",
				Type:      "http",
				HTTP: &config.HTTPMockConfig{
					Matcher: config.HTTPMatcher{
						Method: "GET",
						Path:   "/api/users",
					},
					Response: config.HTTPResponse{
						StatusCode: 200,
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Body: `{"users": [{"id": 1, "name": "Alice", "email": "alice@example.com"}, {"id": 2, "name": "Bob", "email": "bob@example.com"}]}`,
					},
				},
			},
			{
				ID:        "users-get",
				Workspace: "default",
				Type:      "http",
				HTTP: &config.HTTPMockConfig{
					Matcher: config.HTTPMatcher{
						Method: "GET",
						Path:   "/api/users/{id}",
					},
					Response: config.HTTPResponse{
						StatusCode: 200,
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Body: `{"id": "{{request.pathParam.id}}", "name": "Alice", "email": "alice@example.com"}`,
					},
				},
			},
			{
				ID:        "users-create",
				Workspace: "default",
				Type:      "http",
				HTTP: &config.HTTPMockConfig{
					Matcher: config.HTTPMatcher{
						Method: "POST",
						Path:   "/api/users",
					},
					Response: config.HTTPResponse{
						StatusCode: 201,
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Body: `{"id": 3, "name": "{{request.body.name}}", "email": "{{request.body.email}}"}`,
					},
				},
			},
			{
				ID:        "users-update",
				Workspace: "default",
				Type:      "http",
				HTTP: &config.HTTPMockConfig{
					Matcher: config.HTTPMatcher{
						Method: "PUT",
						Path:   "/api/users/{id}",
					},
					Response: config.HTTPResponse{
						StatusCode: 200,
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Body: `{"id": "{{request.pathParam.id}}", "name": "{{request.body.name}}", "email": "{{request.body.email}}"}`,
					},
				},
			},
			{
				ID:        "users-delete",
				Workspace: "default",
				Type:      "http",
				HTTP: &config.HTTPMockConfig{
					Matcher: config.HTTPMatcher{
						Method: "DELETE",
						Path:   "/api/users/{id}",
					},
					Response: config.HTTPResponse{
						StatusCode: 204,
					},
				},
			},
		},
	}
}

// generateProjectConfigYAML generates YAML output with header comments.
func generateProjectConfigYAML(cfg *config.ProjectConfig) ([]byte, error) {
	// Generate the YAML content
	yamlData, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	// Add header comments
	header := `# mockd.yaml
# Generated by: mockd init
# Documentation: https://mockd.io/docs
#
# Start services: mockd up
# Test endpoint:  curl http://localhost:4280/health

`
	return append([]byte(header), yamlData...), nil
}
