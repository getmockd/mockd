package cli

import (
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:                "init [flags]",
	Short:              "Create a starter mockd.yaml configuration file",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunInit(args)
	},
}

var importCmd = &cobra.Command{
	Use:                "import <source> [flags]",
	Short:              "Import mocks from various sources and formats",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunImport(args)
	},
}

var exportCmd = &cobra.Command{
	Use:                "export [flags]",
	Short:              "Export current mock configuration to various formats",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return RunExport(args)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(importCmd)
	rootCmd.AddCommand(exportCmd)
}
