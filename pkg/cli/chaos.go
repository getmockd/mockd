package cli

import (
	"errors"
	"fmt"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/spf13/cobra"
)

var (
	chaosEnableAdminURL    string
	chaosEnableLatency     string
	chaosEnableErrorRate   float64
	chaosEnableErrorCode   int
	chaosEnablePath        string
	chaosEnableProbability float64

	chaosDisableAdminURL string

	chaosStatusAdminURL string
)

var chaosCmd = &cobra.Command{
	Use:   "chaos",
	Short: "Manage chaos injection for fault testing",
	Long:  `Manage chaos injection for fault testing.`,
}

var chaosEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Enable chaos injection with specified parameters",
	RunE: func(cmd *cobra.Command, args []string) error {
		adminURL := &chaosEnableAdminURL
		latency := &chaosEnableLatency
		errorRate := &chaosEnableErrorRate
		errorCode := &chaosEnableErrorCode
		path := &chaosEnablePath
		probability := &chaosEnableProbability

		// Validate that at least one chaos option is specified
		if *latency == "" && *errorRate == 0 {
			return errors.New("at least --latency or --error-rate must be specified")
		}

		// Build chaos config
		chaosConfig := map[string]interface{}{
			"enabled": true,
		}

		if *latency != "" {
			min, max := ParseLatencyRange(*latency)
			chaosConfig["latency"] = map[string]interface{}{
				"min":         min,
				"max":         max,
				"probability": *probability,
			}
		}

		if *errorRate > 0 {
			chaosConfig["errorRate"] = map[string]interface{}{
				"probability": *errorRate,
				"defaultCode": *errorCode,
			}
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
		client := NewAdminClientWithAuth(*adminURL)
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
	},
}

var chaosDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable chaos injection",
	RunE: func(cmd *cobra.Command, args []string) error {
		adminURL := &chaosDisableAdminURL

		// Send request to admin API
		client := NewAdminClientWithAuth(*adminURL)
		chaosConfig := map[string]interface{}{
			"enabled": false,
		}

		if err := client.SetChaosConfig(chaosConfig); err != nil {
			return fmt.Errorf("failed to disable chaos: %s", FormatConnectionError(err))
		}

		fmt.Println("Chaos injection disabled")
		return nil
	},
}

var chaosStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current chaos configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		adminURL := &chaosStatusAdminURL

		// Get chaos config from admin API
		client := NewAdminClientWithAuth(*adminURL)
		config, err := client.GetChaosConfig()
		if err != nil {
			return fmt.Errorf("failed to get chaos status: %s", FormatConnectionError(err))
		}

		if jsonOutput {
			return output.JSON(config)
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
	},
}

func init() {
	rootCmd.AddCommand(chaosCmd)

	chaosCmd.AddCommand(chaosEnableCmd)
	chaosEnableCmd.Flags().StringVar(&chaosEnableAdminURL, "admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	chaosEnableCmd.Flags().StringVarP(&chaosEnableLatency, "latency", "l", "", "Add random latency (e.g., \"10ms-100ms\")")
	chaosEnableCmd.Flags().Float64VarP(&chaosEnableErrorRate, "error-rate", "e", 0, "Error rate (0.0-1.0)")
	chaosEnableCmd.Flags().IntVar(&chaosEnableErrorCode, "error-code", 500, "HTTP error code to return")
	chaosEnableCmd.Flags().StringVarP(&chaosEnablePath, "path", "p", "", "Path pattern to apply chaos to (regex)")
	chaosEnableCmd.Flags().Float64Var(&chaosEnableProbability, "probability", 1.0, "Probability of applying chaos (0.0-1.0)")

	chaosCmd.AddCommand(chaosDisableCmd)
	chaosDisableCmd.Flags().StringVar(&chaosDisableAdminURL, "admin-url", cliconfig.GetAdminURL(), "Admin API base URL")

	chaosCmd.AddCommand(chaosStatusCmd)
	chaosStatusCmd.Flags().StringVar(&chaosStatusAdminURL, "admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
}
