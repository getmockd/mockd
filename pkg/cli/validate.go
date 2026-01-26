package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/getmockd/mockd/pkg/config"
)

// RunValidate validates a mockd configuration file without starting any services.
func RunValidate(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)

	var configFiles stringSliceFlag
	fs.Var(&configFiles, "config", "Config file path (can be specified multiple times)")
	fs.Var(&configFiles, "f", "Config file path (shorthand)")

	verbose := fs.Bool("verbose", false, "Show detailed validation information")
	showResolved := fs.Bool("show-resolved", false, "Show resolved config after env var expansion")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd validate [flags]

Validate a mockd configuration file without starting any services.

This command checks:
  - YAML syntax
  - Schema validation (required fields, valid values)
  - Reference integrity (engines reference valid admins, etc.)
  - Port conflicts between services

Flags:
  -f, --config <path>    Config file path (can be specified multiple times)
                         If not specified, discovers mockd.yaml in current directory
  --verbose              Show detailed validation information
  --show-resolved        Show resolved config after env var expansion

Environment Variables:
  MOCKD_CONFIG           Default config file path (if -f not specified)

Examples:
  # Validate config in current directory
  mockd validate

  # Validate a specific config file
  mockd validate -f ./mockd.yaml

  # Validate multiple config files (merged in order)
  mockd validate -f base.yaml -f production.yaml

  # Show resolved config with env vars expanded
  mockd validate --show-resolved

  # Verbose output with all checks
  mockd validate --verbose
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	// Load config(s)
	var cfg *config.ProjectConfig
	var err error
	var configPath string

	if len(configFiles) == 0 {
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
	} else if len(configFiles) == 1 {
		configPath = configFiles[0]
		cfg, err = config.LoadProjectConfig(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading %s: %v\n", configPath, err)
			return err
		}
	} else {
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
	allErrors := append(result.Errors, portResult.Errors...)
	hasErrors := len(allErrors) > 0

	// Print results
	if *verbose {
		printVerboseValidation(cfg, allErrors)
	} else if hasErrors {
		fmt.Println("Validation failed:")
		for _, e := range allErrors {
			fmt.Printf("  - %s\n", e.Error())
		}
	} else {
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
			if m.IsInline() {
				inlineCount++
			} else if m.IsFileRef() {
				fileRefCount++
			} else if m.IsGlob() {
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

// stringSliceFlag implements flag.Value for accumulating multiple string values.
type stringSliceFlag []string

func (s *stringSliceFlag) String() string {
	return strings.Join(*s, ", ")
}

func (s *stringSliceFlag) Set(value string) error {
	*s = append(*s, value)
	return nil
}
