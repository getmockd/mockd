package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cliconfig"
)

// RunConfig handles the config command and its subcommands.
func RunConfig(args []string) error {
	// Check for subcommands first
	if handled, err := runConfigSubcommand(args); handled {
		return err
	}

	fs := flag.NewFlagSet("config", flag.ContinueOnError)

	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd config [command] [flags]

Manage and display configuration.

Commands:
  show        Show resolved project config (env vars expanded)

Flags:
      --json         Output in JSON format

Examples:
  # Show CLI effective config
  mockd config
  mockd config --json

  # Show resolved project config (mockd.yaml)
  mockd config show
  mockd config show --service default
  mockd config show --json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load all configuration
	cfg, err := cliconfig.LoadAll()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	if *jsonOutput {
		return output.JSON(cfg)
	}

	// Human-readable output with source annotations
	fmt.Println("Effective Configuration:")
	fmt.Println()

	printConfigValue("port", cfg.Port, cfg.Sources["port"])
	printConfigValue("adminPort", cfg.AdminPort, cfg.Sources["adminPort"])
	printConfigValue("adminUrl", cfg.AdminURL, cfg.Sources["adminUrl"])
	printConfigValue("httpsPort", cfg.HTTPSPort, cfg.Sources["httpsPort"])
	if cfg.ConfigFile != "" {
		printConfigValue("configFile", cfg.ConfigFile, cfg.Sources["configFile"])
	}
	printConfigValue("readTimeout", cfg.ReadTimeout, cfg.Sources["readTimeout"])
	printConfigValue("writeTimeout", cfg.WriteTimeout, cfg.Sources["writeTimeout"])
	printConfigValue("maxLogEntries", cfg.MaxLogEntries, cfg.Sources["maxLogEntries"])
	printConfigValue("autoGenerateCert", cfg.AutoCert, cfg.Sources["autoCert"])
	printConfigValue("verbose", cfg.Verbose, cfg.Sources["verbose"])

	// Show loaded sources
	fmt.Println()
	globalPath, _ := cliconfig.FindGlobalConfig()
	localPath, _ := cliconfig.FindLocalConfig()

	if globalPath != "" || localPath != "" {
		fmt.Println("Sources loaded:")
		if globalPath != "" {
			fmt.Printf("  • %s (global)\n", globalPath)
		}
		if localPath != "" {
			fmt.Printf("  • %s (local)\n", localPath)
		}
	} else {
		fmt.Println("Sources loaded: (none)")
	}

	// Show searched locations
	fmt.Println()
	fmt.Println("Searched:")
	for _, p := range cliconfig.GetGlobalConfigSearchPaths() {
		if p == globalPath {
			fmt.Printf("  ✓ %s\n", p)
		} else {
			fmt.Printf("  ✗ %s\n", p)
		}
	}
	for _, p := range cliconfig.GetLocalConfigSearchPaths() {
		if p == localPath {
			fmt.Printf("  ✓ %s\n", p)
		} else {
			fmt.Printf("  ✗ %s\n", p)
		}
	}

	return nil
}

// printConfigValue prints a config value with source annotation.
func printConfigValue(name string, value interface{}, source string) {
	if source == "" {
		source = cliconfig.SourceDefault
	}
	sourceLabel := formatSource(source)
	fmt.Printf("  %-18s %v%s\n", name+":", value, sourceLabel)
}

// formatSource formats a source type for display.
func formatSource(source string) string {
	switch source {
	case cliconfig.SourceDefault:
		return "  (default)"
	case cliconfig.SourceEnv:
		return "  (env)"
	case cliconfig.SourceGlobal:
		return "  (global config)"
	case cliconfig.SourceLocal:
		return "  (local config)"
	case cliconfig.SourceFlag:
		return "  (flag)"
	default:
		return ""
	}
}
