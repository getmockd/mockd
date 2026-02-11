package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/config"
	"gopkg.in/yaml.v3"
)

// RunConfigShow displays the resolved project configuration.
func RunConfigShow(args []string) error {
	fs := flag.NewFlagSet("config show", flag.ContinueOnError)

	var configFiles stringSliceFlag
	fs.Var(&configFiles, "config", "Config file path (can be specified multiple times)")
	fs.Var(&configFiles, "f", "Config file path (shorthand)")

	serviceName := fs.String("service", "", "Show config for a specific service (admin or engine name)")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd config show [flags]

Display resolved configuration with environment variables expanded.

This command loads the project configuration (mockd.yaml) and displays
the effective configuration after all environment variable substitutions
have been applied.

Flags:
  -f, --config <path>    Config file path (can be specified multiple times)
      --service <name>   Show config for a specific service (admin or engine)
      --json             Output in JSON format (default: YAML)

Examples:
  # Show full resolved config
  mockd config show

  # Show config for a specific engine
  mockd config show --service default

  # Output as JSON
  mockd config show --json

  # Show config from specific file
  mockd config show -f ./production.yaml
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	// Load config
	cfg, configPath, err := loadProjectConfig(configFiles)
	if err != nil {
		return err
	}

	// If --service is specified, filter to just that service
	if *serviceName != "" {
		return printServiceConfig(cfg, *serviceName, configPath, *jsonOutput)
	}

	// Print full config
	return printFullConfig(cfg, configPath, *jsonOutput)
}

// printFullConfig outputs the full resolved configuration.
func printFullConfig(cfg *config.ProjectConfig, configPath string, jsonOutput bool) error {
	if jsonOutput {
		return printConfigAsJSON(cfg)
	}
	return printConfigAsYAML(cfg, configPath)
}

// printServiceConfig outputs configuration for a specific service.
func printServiceConfig(cfg *config.ProjectConfig, serviceName, configPath string, jsonOutput bool) error {
	// Try to find the service in admins
	for _, admin := range cfg.Admins {
		if admin.Name == serviceName {
			if jsonOutput {
				return printAsJSON(admin)
			}
			return printServiceAsYAML("admin", admin, configPath)
		}
	}

	// Try to find the service in engines
	for _, engine := range cfg.Engines {
		if engine.Name == serviceName {
			if jsonOutput {
				return printAsJSON(engine)
			}
			return printServiceAsYAML("engine", engine, configPath)
		}
	}

	return fmt.Errorf("service '%s' not found in admins or engines", serviceName)
}

// printConfigAsJSON outputs the config as JSON.
func printConfigAsJSON(cfg *config.ProjectConfig) error {
	return output.JSON(cfg)
}

// printAsJSON outputs any value as JSON.
func printAsJSON(v interface{}) error {
	return output.JSON(v)
}

// printConfigAsYAML outputs the config as YAML with a header comment.
func printConfigAsYAML(cfg *config.ProjectConfig, configPath string) error {
	// Print header comment
	fmt.Printf("# Resolved configuration from %s\n", configPath)
	fmt.Println("# Environment variables have been expanded")
	fmt.Println()

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	fmt.Print(string(data))
	return nil
}

// printServiceAsYAML outputs a single service config as YAML.
func printServiceAsYAML(serviceType string, service interface{}, configPath string) error {
	// Print header comment
	fmt.Printf("# Resolved %s configuration from %s\n", serviceType, configPath)
	fmt.Println("# Environment variables have been expanded")
	fmt.Println()

	// Marshal to YAML
	data, err := yaml.Marshal(service)
	if err != nil {
		return fmt.Errorf("marshaling %s config: %w", serviceType, err)
	}

	fmt.Print(string(data))
	return nil
}

// configSubcommands maps config subcommand names to their handlers.
var configSubcommands = map[string]func([]string) error{
	"show": RunConfigShow,
}

// runConfigSubcommand routes to the appropriate config subcommand.
func runConfigSubcommand(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	subCmd := strings.ToLower(args[0])
	if handler, ok := configSubcommands[subCmd]; ok {
		return true, handler(args[1:])
	}

	return false, nil
}
