package cli

import (
	"fmt"
	"os"

	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/i18n"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	// Persistent flags available to all subcommands
	adminURL      string
	jsonOutput    bool
	workspaceFlag string
	langFlag      string

	// Version is injected during build
	Version = "dev"
	// Commit is injected during build
	Commit = "none"
	// BuildDate is injected during build
	BuildDate = "unknown"
)

// resolvedWorkspace returns the active workspace ID, resolved from flag > env > context.
func resolvedWorkspace() string {
	return cliconfig.ResolveWorkspace(workspaceFlag)
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mockd",
	Short: i18n.T("mockd is a flexible multi-protocol mocking server"),
	Long: i18n.T(`mockd enables zero-code mocking, validation, and testing of backend endpoints locally.
It supports HTTP, REST, GraphQL, gRPC, MQTT, WebSocket, and SOAP protocols.

Configuration can be provided via flags, environment variables, or a configuration file.
By default, mockd looks for a configuration file at ~/.mockd/config.yaml.`),
	// No Run function here means 'mockd' with no args will print help text by default.
	SilenceUsage:  true,
	SilenceErrors: true, // We handle errors in Execute()
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Parse flags before Execute so --lang is available for help output too.
	_ = rootCmd.ParseFlags(os.Args[1:])
	// Detect language from --lang flag first, then MOCKD_LANG env var.
	if langFlag != "" {
		i18n.SetLang(langFlag)
	} else {
		i18n.DetectFromEnv()
	}
	// Commands are constructed in init() before the language is known;
	// re-translate Short/Long/flag usage text now that lang is resolved.
	refreshCommandText(rootCmd)
	// Localize cobra's built-in usage/help template labels.
	rootCmd.SetUsageTemplate(i18n.UsageTemplate())
	rootCmd.SetHelpTemplate(i18n.UsageTemplate())
	// Cobra's built-in help command is registered during Execute(); localize it too.
	if helpCmd, _, err := rootCmd.Find([]string{"help"}); err == nil && helpCmd != nil {
		helpCmd.Short = i18n.T("Show help for topics or commands")
	}
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// refreshCommandText recursively re-applies i18n translation to a command's
// user-visible help text. Source strings are English; T() falls back to the
// source when no translation exists, so re-running is safe.
func refreshCommandText(cmd *cobra.Command) {
	cmd.Short = i18n.T(cmd.Short)
	if cmd.Long != "" {
		cmd.Long = i18n.T(cmd.Long)
	}
	if cmd.Example != "" {
		cmd.Example = i18n.T(cmd.Example)
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) { f.Usage = i18n.T(f.Usage) })
	cmd.PersistentFlags().VisitAll(func(f *pflag.Flag) { f.Usage = i18n.T(f.Usage) })
	for _, c := range cmd.Commands() {
		refreshCommandText(c)
	}
}

func init() {
	// Define persistent flags that apply globally to all mockd commands
	rootCmd.PersistentFlags().StringVar(&adminURL, "admin-url", cliconfig.GetAdminURL(), "Admin API base URL (default: http://localhost:4290)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output command results in JSON format")
	rootCmd.PersistentFlags().StringVar(&workspaceFlag, "workspace", "", "Workspace to operate on (overrides MOCKD_WORKSPACE and context config)")
	rootCmd.PersistentFlags().StringVar(&langFlag, "lang", "", "Output language (en, zh-CN; default: en or MOCKD_LANG)")
}
