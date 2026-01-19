// Package cliconfig provides configuration types and loading for the mockd CLI.
//
// It implements a layered configuration system with the following precedence
// (highest to lowest):
//
//  1. Command-line flags
//  2. Environment variables (MOCKD_* prefix)
//  3. Local config file (.mockdrc.json in current directory)
//  4. Global config file (~/.config/mockd/config.json)
//  5. Default values
//
// The package handles configuration discovery, loading, merging, and validation.
// It tracks the source of each configuration value for debugging purposes.
//
// Key types:
//
//   - CLIConfig: Complete configuration structure for the CLI
//   - ConfigSource: Constants identifying where config values originated
//
// Key functions:
//
//   - Load: Loads and merges configuration from all sources
//   - FindLocalConfig: Locates .mockdrc.json in the current directory
//   - FindGlobalConfig: Locates global config file
//   - ApplyEnv: Applies environment variable overrides
package cliconfig
