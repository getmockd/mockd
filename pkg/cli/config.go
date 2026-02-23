package cli

import (
	"fmt"

	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage and display configuration",
	Long: `Manage and display mockd effective configuration.

Shows parameters loaded from flags, environment variables, or config files.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load all configuration
		cfg, err := cliconfig.LoadAll()
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		printResult(cfg, func() {
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
		})

		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
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
