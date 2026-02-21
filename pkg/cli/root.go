package cli

import (
	"fmt"
	"os"

	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/spf13/cobra"
)

var (
	// Persistent flags available to all subcommands
	adminURL   string
	jsonOutput bool

	// Version is injected during build
	Version = "dev"
	// Commit is injected during build
	Commit = "none"
	// BuildDate is injected during build
	BuildDate = "unknown"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mockd",
	Short: "mockd is a flexible multi-protocol mocking server",
	Long: `mockd enables zero-code mocking, validation, and testing of backend endpoints locally.
It supports HTTP, REST, GraphQL, gRPC, MQTT, WebSocket, and SOAP protocols.

Configuration can be provided via flags, environment variables, or a configuration file.
By default, mockd looks for a configuration file at ~/.mockd/config.yaml.`,
	// No Run function here means 'mockd' with no args will print help text by default.
	SilenceUsage:  true,
	SilenceErrors: true, // We handle errors in Execute()
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Define persistent flags that apply globally to all mockd commands
	rootCmd.PersistentFlags().StringVar(&adminURL, "admin-url", cliconfig.GetAdminURL(), "Admin API base URL (default: http://localhost:4290)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output command results in JSON format")

	// Ensure any subcommands we migrate get attached immediately if they exist
	// (they will add themselves in their own init() functions once migrated)
}
