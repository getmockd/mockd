package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/getmockd/mockd/pkg/cli/templates"
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
// It reads from the package-level initForce, initOutput, initFormat, etc. vars
// set by Cobra flags, or can be called from tests via runInitWithIO.
func RunInit(args []string) error {
	return runInitWithIO(os.Stdin, os.Stdout, os.Stderr)
}

// runInitWithIO is the testable version of RunInit that accepts custom I/O.
func runInitWithIO(stdin io.Reader, stdout, _ io.Writer) error {
	// Determine output format
	outputFormat := strings.ToLower(initFormat)
	if outputFormat == "" {
		// Infer from filename extension
		ext := strings.ToLower(filepath.Ext(initOutput))
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
	if _, err := os.Stat(initOutput); err == nil {
		if !initForce {
			return fmt.Errorf("file already exists: %s\n\nUse --force to overwrite", initOutput)
		}
	}

	// Handle --template list
	if initTemplate == "list" {
		_, _ = fmt.Fprint(stdout, templates.FormatList())
		_, _ = fmt.Fprintln(stdout)
		_, _ = fmt.Fprintln(stdout, "Built-in templates:")
		_, _ = fmt.Fprintf(stdout, "  %-16s  %s\n", "minimal", "Just admin + engine + one health mock")
		_, _ = fmt.Fprintf(stdout, "  %-16s  %s\n", "full", "Admin + engine + workspace + sample mocks")
		_, _ = fmt.Fprintf(stdout, "  %-16s  %s\n", "api", "REST API mocking with CRUD examples")
		return nil
	}

	// Build the config based on flags
	var cfg *config.ProjectConfig
	var rawYAML []byte // Used for protocol templates that are raw YAML files
	var err error

	switch {
	case initTemplate != "":
		// Check protocol templates first (raw YAML files from pkg/cli/templates)
		if templates.Exists(initTemplate) {
			rawYAML, err = templates.Get(initTemplate)
			if err != nil {
				return fmt.Errorf("failed to load template %q: %w", initTemplate, err)
			}
			// Protocol templates are always YAML, override format
			if outputFormat == "json" {
				return errors.New("protocol templates are YAML-only; use --format yaml or omit --format")
			}
		} else {
			// Try built-in Go struct templates (minimal, full, api)
			cfg, err = getProjectConfigTemplate(initTemplate)
			if err != nil {
				return err
			}
		}
	case initDefaults:
		// Generate minimal config without prompts
		cfg = generateMinimalProjectConfig(defaultInitConfig())
	default:
		// Interactive wizard (default, or explicit -i/--interactive)
		_, _ = fmt.Fprintln(stdout, "Creating new mockd configuration...")
		_, _ = fmt.Fprintln(stdout)

		initCfg, err := runInteractiveWizard(stdin, stdout)
		if err != nil {
			return err
		}

		cfg = generateMinimalProjectConfig(initCfg)
	}

	// Generate output
	var data []byte
	switch {
	case rawYAML != nil:
		// Protocol template — already formatted YAML with comments
		data = rawYAML
	case outputFormat == "json":
		data, err = json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to generate JSON: %w", err)
		}
		data = append(data, '\n')
	default:
		data, err = generateProjectConfigYAML(cfg)
		if err != nil {
			return fmt.Errorf("failed to generate YAML: %w", err)
		}
	}

	// Write the file
	if err := os.WriteFile(initOutput, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Print success message
	_, _ = fmt.Fprintf(stdout, "\nWriting %s...\n", initOutput)
	_, _ = fmt.Fprintln(stdout, "Done! Run 'mockd up' to start.")

	return nil
}

// runInteractiveWizard prompts the user for configuration options.
//
//nolint:unparam // error return reserved for future validation
func runInteractiveWizard(stdin io.Reader, stdout io.Writer) (*initConfig, error) {
	reader := bufio.NewReader(stdin)
	cfg := defaultInitConfig()

	// Admin port
	_, _ = fmt.Fprintf(stdout, "Admin port [%d]: ", cfg.AdminPort)
	if val, err := readIntWithDefault(reader, cfg.AdminPort); err == nil {
		cfg.AdminPort = val
	}

	// Engine HTTP port
	_, _ = fmt.Fprintf(stdout, "Engine HTTP port [%d]: ", cfg.HTTPPort)
	if val, err := readIntWithDefault(reader, cfg.HTTPPort); err == nil {
		cfg.HTTPPort = val
	}

	// Enable HTTPS?
	_, _ = fmt.Fprint(stdout, "Enable HTTPS? (y/N): ")
	if val, _ := readBoolWithDefault(reader, false); val {
		cfg.EnableHTTPS = true
		_, _ = fmt.Fprintf(stdout, "Engine HTTPS port [%d]: ", cfg.HTTPSPort)
		if val, err := readIntWithDefault(reader, cfg.HTTPSPort); err == nil {
			cfg.HTTPSPort = val
		}
	}

	// Auth type
	_, _ = fmt.Fprintf(stdout, "Auth type (none/api-key) [%s]: ", cfg.AuthType)
	if val, _ := readStringWithDefault(reader, cfg.AuthType); val == "api-key" || val == "none" {
		cfg.AuthType = val
	}

	_, _ = fmt.Fprintln(stdout)

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
//
//nolint:unparam // error return reserved for future validation
func readBoolWithDefault(reader *bufio.Reader, defaultVal bool) (bool, error) {
	input, _ := reader.ReadString('\n')
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" {
		return defaultVal, nil
	}
	return input == "y" || input == "yes", nil
}

// readStringWithDefault reads a string from the reader, returning the default if empty.
//
//nolint:unparam // error return reserved for future validation
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
		Version: "1.0",
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
		return nil, fmt.Errorf("unknown template: %s\n\nRun 'mockd init --template list' to see all available templates", name)
	}
}

// generateMinimalTemplate creates a minimal ProjectConfig template.
func generateMinimalTemplate() *config.ProjectConfig {
	return &config.ProjectConfig{
		Version: "1.0",
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
		Version: "1.0",
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
		Version: "1.0",
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
