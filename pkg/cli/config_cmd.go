package cli

import (
	"fmt"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	configShowFiles   []string
	configShowService string
)

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show resolved project config (env vars expanded)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		cfg, configPath, err := loadProjectConfig(configShowFiles)
		if err != nil {
			return err
		}

		// If --service is specified, filter to just that service
		if configShowService != "" {
			return printServiceConfig(cfg, configShowService, configPath, jsonOutput)
		}

		// Print full config
		return printFullConfig(cfg, configPath, jsonOutput)
	},
}

func init() {
	configShowCmd.Flags().StringSliceVarP(&configShowFiles, "config", "f", []string{}, "Config file path (can be specified multiple times)")
	configShowCmd.Flags().StringVar(&configShowService, "service", "", "Show config for a specific service (admin or engine name)")
	configCmd.AddCommand(configShowCmd)
}

// printFullConfig outputs the full resolved configuration.
func printFullConfig(cfg *config.ProjectConfig, configPath string, isJSON bool) error {
	if isJSON {
		return printConfigAsJSON(cfg)
	}
	return printConfigAsYAML(cfg, configPath)
}

// printServiceConfig outputs configuration for a specific service.
func printServiceConfig(cfg *config.ProjectConfig, serviceName, configPath string, isJSON bool) error {
	// Try to find the service in admins
	for _, admin := range cfg.Admins {
		if admin.Name == serviceName {
			if isJSON {
				return printAsJSON(admin)
			}
			return printServiceAsYAML("admin", admin, configPath)
		}
	}

	// Try to find the service in engines
	for _, engine := range cfg.Engines {
		if engine.Name == serviceName {
			if isJSON {
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
