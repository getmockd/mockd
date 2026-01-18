package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/getmockd/mockd/internal/cliconfig"
)

// RunChaos handles the chaos command and its subcommands.
func RunChaos(args []string) error {
	if len(args) == 0 {
		printChaosUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "enable":
		return runChaosEnable(subArgs)
	case "disable":
		return runChaosDisable(subArgs)
	case "status":
		return runChaosStatus(subArgs)
	case "help", "--help", "-h":
		printChaosUsage()
		return nil
	default:
		return fmt.Errorf("unknown chaos subcommand: %s\n\nRun 'mockd chaos --help' for usage", subcommand)
	}
}

func printChaosUsage() {
	fmt.Print(`Usage: mockd chaos <subcommand> [flags]

Manage chaos injection for fault testing.

Subcommands:
  enable   Enable chaos injection with specified parameters
  disable  Disable chaos injection
  status   Show current chaos configuration

Run 'mockd chaos <subcommand> --help' for more information.
`)
}

// runChaosEnable enables chaos injection.
func runChaosEnable(args []string) error {
	fs := flag.NewFlagSet("chaos enable", flag.ContinueOnError)

	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")

	latency := fs.String("latency", "", "Add random latency (e.g., \"10ms-100ms\")")
	fs.StringVar(latency, "l", "", "Add random latency (shorthand)")

	errorRate := fs.Float64("error-rate", 0, "Error rate (0.0-1.0)")
	fs.Float64Var(errorRate, "e", 0, "Error rate (shorthand)")

	errorCode := fs.Int("error-code", 500, "HTTP error code to return")

	path := fs.String("path", "", "Path pattern to apply chaos to (regex)")
	fs.StringVar(path, "p", "", "Path pattern (shorthand)")

	probability := fs.Float64("probability", 1.0, "Probability of applying chaos (0.0-1.0)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd chaos enable [flags]

Enable chaos injection on the running mock server.

Flags:
      --admin-url    Admin API base URL (default: http://localhost:4290)
  -l, --latency      Add random latency (e.g., "10ms-100ms")
  -e, --error-rate   Error rate (0.0-1.0)
      --error-code   HTTP error code to return (default: 500)
  -p, --path         Path pattern to apply chaos to (regex)
      --probability  Probability of applying chaos (default: 1.0)

Examples:
  # Enable random latency
  mockd chaos enable --latency "50ms-200ms"

  # Enable error injection with 10% rate
  mockd chaos enable --error-rate 0.1 --error-code 503

  # Apply chaos only to specific paths
  mockd chaos enable --latency "100ms-500ms" --path "/api/.*"

  # Combine latency and errors
  mockd chaos enable --latency "10ms-50ms" --error-rate 0.05
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Validate that at least one chaos option is specified
	if *latency == "" && *errorRate == 0 {
		return fmt.Errorf("at least --latency or --error-rate must be specified")
	}

	// Build chaos config
	chaosConfig := map[string]interface{}{
		"enabled": true,
	}

	globalRules := map[string]interface{}{}

	if *latency != "" {
		min, max := ParseLatencyRange(*latency)
		globalRules["latency"] = map[string]interface{}{
			"min":         min,
			"max":         max,
			"probability": *probability,
		}
	}

	if *errorRate > 0 {
		globalRules["errorRate"] = map[string]interface{}{
			"probability": *errorRate,
			"defaultCode": *errorCode,
		}
	}

	if len(globalRules) > 0 {
		chaosConfig["global"] = globalRules
	}

	if *path != "" {
		chaosConfig["rules"] = []map[string]interface{}{
			{
				"pathPattern": *path,
				"probability": *probability,
			},
		}
	}

	// Send request to admin API
	client := NewAdminClient(*adminURL)
	if err := client.SetChaosConfig(chaosConfig); err != nil {
		return fmt.Errorf("failed to enable chaos: %s", FormatConnectionError(err))
	}

	fmt.Println("Chaos injection enabled")
	if *latency != "" {
		fmt.Printf("  Latency: %s\n", *latency)
	}
	if *errorRate > 0 {
		fmt.Printf("  Error rate: %.1f%% (HTTP %d)\n", *errorRate*100, *errorCode)
	}
	if *path != "" {
		fmt.Printf("  Path pattern: %s\n", *path)
	}

	return nil
}

// runChaosDisable disables chaos injection.
func runChaosDisable(args []string) error {
	fs := flag.NewFlagSet("chaos disable", flag.ContinueOnError)

	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd chaos disable [flags]

Disable chaos injection on the running mock server.

Flags:
      --admin-url   Admin API base URL (default: http://localhost:4290)

Examples:
  mockd chaos disable
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Send request to admin API
	client := NewAdminClient(*adminURL)
	chaosConfig := map[string]interface{}{
		"enabled": false,
	}

	if err := client.SetChaosConfig(chaosConfig); err != nil {
		return fmt.Errorf("failed to disable chaos: %s", FormatConnectionError(err))
	}

	fmt.Println("Chaos injection disabled")
	return nil
}

// runChaosStatus shows the current chaos configuration.
func runChaosStatus(args []string) error {
	fs := flag.NewFlagSet("chaos status", flag.ContinueOnError)

	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd chaos status [flags]

Show current chaos configuration.

Flags:
      --admin-url   Admin API base URL (default: http://localhost:4290)
      --json        Output in JSON format

Examples:
  mockd chaos status
  mockd chaos status --json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Get chaos config from admin API
	client := NewAdminClient(*adminURL)
	config, err := client.GetChaosConfig()
	if err != nil {
		return fmt.Errorf("failed to get chaos status: %s", FormatConnectionError(err))
	}

	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(config)
	}

	// Pretty print status
	enabled, _ := config["enabled"].(bool)
	if !enabled {
		fmt.Println("Chaos injection: disabled")
		return nil
	}

	fmt.Println("Chaos injection: enabled")

	if global, ok := config["global"].(map[string]interface{}); ok {
		if latency, ok := global["latency"].(map[string]interface{}); ok {
			min, _ := latency["min"].(string)
			max, _ := latency["max"].(string)
			prob, _ := latency["probability"].(float64)
			fmt.Printf("  Latency: %s-%s (%.0f%% probability)\n", min, max, prob*100)
		}
		if errorRate, ok := global["errorRate"].(map[string]interface{}); ok {
			prob, _ := errorRate["probability"].(float64)
			code, _ := errorRate["defaultCode"].(float64)
			fmt.Printf("  Error rate: %.1f%% (HTTP %d)\n", prob*100, int(code))
		}
	}

	if rules, ok := config["rules"].([]interface{}); ok && len(rules) > 0 {
		fmt.Println("  Rules:")
		for _, r := range rules {
			if rule, ok := r.(map[string]interface{}); ok {
				pattern, _ := rule["pathPattern"].(string)
				prob, _ := rule["probability"].(float64)
				fmt.Printf("    - %s (%.0f%% probability)\n", pattern, prob*100)
			}
		}
	}

	return nil
}
