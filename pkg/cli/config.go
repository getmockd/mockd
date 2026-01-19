package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/getmockd/mockd/pkg/cliconfig"
)

// RunConfig handles the config command.
func RunConfig(args []string) error {
	fs := flag.NewFlagSet("config", flag.ContinueOnError)

	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd config [flags]

Show effective configuration with source annotations.

Flags:
      --json         Output in JSON format

Examples:
  mockd config
  mockd config --json
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
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cfg)
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
	fmt.Println("Sources loaded:")

	if globalPath, err := cliconfig.FindGlobalConfig(); err == nil && globalPath != "" {
		fmt.Printf("  • %s (global)\n", globalPath)
	}
	if localPath, err := cliconfig.FindLocalConfig(); err == nil && localPath != "" {
		fmt.Printf("  • %s (local)\n", localPath)
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
