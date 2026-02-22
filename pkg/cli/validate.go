package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/spf13/cobra"
)

var (
	validateConfigFiles  []string
	validateVerbose      bool
	validateShowResolved bool
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate a mockd configuration file without starting any services",
	Long: `Validate a mockd configuration file without starting any services.

This command checks:
  - YAML syntax
  - Schema validation (required fields, valid values)
  - Reference integrity (engines reference valid admins, etc.)
  - Port conflicts between services`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		configFiles := validateConfigFiles
		verbose := &validateVerbose
		showResolved := &validateShowResolved

		// Load config(s)
		var cfg *config.ProjectConfig
		var err error
		var configPath string

		switch {
		case len(configFiles) == 0:
			// Discover config
			configPath, err = config.DiscoverProjectConfig()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return err
			}
			cfg, err = config.LoadProjectConfig(configPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", configPath, err)
				return err
			}
			if *verbose {
				fmt.Printf("Discovered config: %s\n", configPath)
			}
		case len(configFiles) == 1:
			configPath = configFiles[0]
			cfg, err = config.LoadProjectConfig(configPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", configPath, err)
				return err
			}
		default:
			// Multiple configs - load and merge
			cfg, err = config.LoadAndMergeProjectConfigs(configFiles)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error loading configs: %v\n", err)
				return err
			}
			if *verbose {
				fmt.Printf("Merged %d config files\n", len(configFiles))
			}
		}

		// Run validation
		result := config.ValidateProjectConfig(cfg)

		// Also check port conflicts
		portResult := config.ValidatePortConflicts(cfg)

		// Combine results
		allErrors := make([]config.SchemaValidationError, 0, len(result.Errors)+len(portResult.Errors))
		allErrors = append(allErrors, result.Errors...)
		allErrors = append(allErrors, portResult.Errors...)
		hasErrors := len(allErrors) > 0

		if jsonOutput {
			errorStrings := make([]string, len(allErrors))
			for i, e := range allErrors {
				errorStrings[i] = e.Error()
			}
			printResult(map[string]any{
				"valid":      !hasErrors,
				"errors":     errorStrings,
				"errorCount": len(allErrors),
				"summary": map[string]int{
					"admins":     len(cfg.Admins),
					"engines":    len(cfg.Engines),
					"workspaces": len(cfg.Workspaces),
					"mocks":      len(cfg.Mocks),
				},
			}, nil)
			if hasErrors {
				return fmt.Errorf("validation failed with %d error(s)", len(allErrors))
			}
			return nil
		}

		// Text output
		switch {
		case *verbose:
			printVerboseValidation(cfg, allErrors)
		case hasErrors:
			fmt.Println("Validation failed:")
			for _, e := range allErrors {
				fmt.Printf("  - %s\n", e.Error())
			}
		default:
			fmt.Println("Configuration is valid.")
		}

		if *showResolved {
			fmt.Println("\nResolved configuration:")
			printResolvedConfig(cfg)
		}

		if hasErrors {
			return fmt.Errorf("validation failed with %d error(s)", len(allErrors))
		}

		// Print summary if verbose
		if *verbose {
			printConfigSummary(cfg)
		}

		return nil
	},
}

func printVerboseValidation(cfg *config.ProjectConfig, errors []config.SchemaValidationError) {
	fmt.Println("Validation Results")
	fmt.Println("==================")
	fmt.Println()

	// Schema validation
	fmt.Printf("Version: %s\n", cfg.Version)
	fmt.Printf("Admins: %d defined\n", len(cfg.Admins))
	fmt.Printf("Engines: %d defined\n", len(cfg.Engines))
	fmt.Printf("Workspaces: %d defined\n", len(cfg.Workspaces))
	fmt.Printf("Mocks: %d defined\n", len(cfg.Mocks))
	fmt.Printf("Stateful Resources: %d defined\n", len(cfg.StatefulResources))
	fmt.Println()

	if len(errors) > 0 {
		fmt.Printf("Errors (%d):\n", len(errors))
		for _, e := range errors {
			fmt.Printf("  [ERROR] %s\n", e.Error())
		}
	} else {
		fmt.Println("No errors found.")
	}
}

func printConfigSummary(cfg *config.ProjectConfig) {
	fmt.Println()
	fmt.Println("Configuration Summary")
	fmt.Println("---------------------")

	if len(cfg.Admins) > 0 {
		fmt.Println("Admins:")
		for _, a := range cfg.Admins {
			if a.IsLocal() {
				fmt.Printf("  - %s (local, port %d)\n", a.Name, a.Port)
			} else {
				fmt.Printf("  - %s (remote, %s)\n", a.Name, a.URL)
			}
		}
	}

	if len(cfg.Engines) > 0 {
		fmt.Println("Engines:")
		for _, e := range cfg.Engines {
			ports := []string{}
			if e.HTTPPort > 0 {
				ports = append(ports, fmt.Sprintf("http:%d", e.HTTPPort))
			}
			if e.HTTPSPort > 0 {
				ports = append(ports, fmt.Sprintf("https:%d", e.HTTPSPort))
			}
			if e.GRPCPort > 0 {
				ports = append(ports, fmt.Sprintf("grpc:%d", e.GRPCPort))
			}
			fmt.Printf("  - %s -> %s (%s)\n", e.Name, e.Admin, strings.Join(ports, ", "))
		}
	}

	if len(cfg.Workspaces) > 0 {
		fmt.Println("Workspaces:")
		for _, w := range cfg.Workspaces {
			if len(w.Engines) > 0 {
				fmt.Printf("  - %s -> [%s]\n", w.Name, strings.Join(w.Engines, ", "))
			} else {
				fmt.Printf("  - %s (no engines assigned)\n", w.Name)
			}
		}
	}

	if len(cfg.Mocks) > 0 {
		fmt.Println("Mocks:")
		inlineCount := 0
		fileRefCount := 0
		globCount := 0
		for _, m := range cfg.Mocks {
			switch {
			case m.IsInline():
				inlineCount++
			case m.IsFileRef():
				fileRefCount++
			case m.IsGlob():
				globCount++
			}
		}
		if inlineCount > 0 {
			fmt.Printf("  - %d inline mocks\n", inlineCount)
		}
		if fileRefCount > 0 {
			fmt.Printf("  - %d file references\n", fileRefCount)
		}
		if globCount > 0 {
			fmt.Printf("  - %d glob patterns\n", globCount)
		}
	}
}

func printResolvedConfig(cfg *config.ProjectConfig) {
	// Print a summary of the resolved config
	fmt.Printf("version: %q\n", cfg.Version)

	if len(cfg.Admins) > 0 {
		fmt.Println("admins:")
		for _, a := range cfg.Admins {
			fmt.Printf("  - name: %s\n", a.Name)
			if a.Port > 0 {
				fmt.Printf("    port: %d\n", a.Port)
			}
			if a.URL != "" {
				fmt.Printf("    url: %s\n", a.URL)
			}
			if a.APIKey != "" {
				// Mask the API key
				fmt.Printf("    apiKey: %s***\n", a.APIKey[:min(4, len(a.APIKey))])
			}
		}
	}

	if len(cfg.Engines) > 0 {
		fmt.Println("engines:")
		for _, e := range cfg.Engines {
			fmt.Printf("  - name: %s\n", e.Name)
			fmt.Printf("    admin: %s\n", e.Admin)
			if e.HTTPPort > 0 {
				fmt.Printf("    httpPort: %d\n", e.HTTPPort)
			}
			if e.HTTPSPort > 0 {
				fmt.Printf("    httpsPort: %d\n", e.HTTPSPort)
			}
			if e.GRPCPort > 0 {
				fmt.Printf("    grpcPort: %d\n", e.GRPCPort)
			}
		}
	}
}

func init() {
	validateCmd.Flags().StringSliceVarP(&validateConfigFiles, "config", "f", nil, "Config file path (can be specified multiple times)")
	validateCmd.Flags().BoolVar(&validateVerbose, "verbose", false, "Show detailed validation information")
	validateCmd.Flags().BoolVar(&validateShowResolved, "show-resolved", false, "Show resolved config after env var expansion")
	rootCmd.AddCommand(validateCmd)
}
