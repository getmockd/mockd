package cli

import (
	"fmt"

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
			return printServiceConfig(cfg, configShowService, configPath)
		}

		// Print full config
		return printFullConfig(cfg, configPath)
	},
}

func init() {
	configShowCmd.Flags().StringSliceVarP(&configShowFiles, "config", "f", []string{}, "Config file path (can be specified multiple times)")
	configShowCmd.Flags().StringVar(&configShowService, "service", "", "Show config for a specific service (admin or engine name)")
	configCmd.AddCommand(configShowCmd)
}

// printFullConfig outputs the full resolved configuration.
func printFullConfig(cfg *config.ProjectConfig, configPath string) error {
	printResult(cfg, func() { _ = printConfigAsYAML(cfg, configPath) })
	return nil
}

// printServiceConfig outputs configuration for a specific service.
func printServiceConfig(cfg *config.ProjectConfig, serviceName, configPath string) error {
	// Try to find the service in admins
	for _, admin := range cfg.Admins {
		if admin.Name == serviceName {
			printResult(admin, func() { _ = printServiceAsYAML("admin", admin, configPath) })
			return nil
		}
	}

	// Try to find the service in engines
	for _, engine := range cfg.Engines {
		if engine.Name == serviceName {
			printResult(engine, func() { _ = printServiceAsYAML("engine", engine, configPath) })
			return nil
		}
	}

	return fmt.Errorf("service '%s' not found in admins or engines", serviceName)
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
