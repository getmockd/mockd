package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	chaosEnableLatency     string
	chaosEnableErrorRate   float64
	chaosEnableErrorCode   int
	chaosEnablePath        string
	chaosEnableProbability float64
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
		// Validate that at least one chaos option is specified
		if chaosEnableLatency == "" && chaosEnableErrorRate == 0 {
			return errors.New("at least --latency or --error-rate must be specified")
		}

		// Build chaos config
		chaosConfig := map[string]interface{}{
			"enabled": true,
		}

		if chaosEnableLatency != "" {
			min, max := ParseLatencyRange(chaosEnableLatency)
			chaosConfig["latency"] = map[string]interface{}{
				"min":         min,
				"max":         max,
				"probability": chaosEnableProbability,
			}
		}

		if chaosEnableErrorRate > 0 {
			chaosConfig["errorRate"] = map[string]interface{}{
				"probability": chaosEnableErrorRate,
				"defaultCode": chaosEnableErrorCode,
			}
		}

		if chaosEnablePath != "" {
			chaosConfig["rules"] = []map[string]interface{}{
				{
					"pathPattern": chaosEnablePath,
					"probability": chaosEnableProbability,
				},
			}
		}

		// Send request to admin API
		client := NewAdminClientWithAuth(adminURL)
		if err := client.SetChaosConfig(chaosConfig); err != nil {
			return fmt.Errorf("failed to enable chaos: %s", FormatConnectionError(err))
		}

		printResult(map[string]any{"enabled": true, "config": chaosConfig}, func() {
			fmt.Println("Chaos injection enabled")
			if chaosEnableLatency != "" {
				fmt.Printf("  Latency: %s\n", chaosEnableLatency)
			}
			if chaosEnableErrorRate > 0 {
				fmt.Printf("  Error rate: %.1f%% (HTTP %d)\n", chaosEnableErrorRate*100, chaosEnableErrorCode)
			}
			if chaosEnablePath != "" {
				fmt.Printf("  Path pattern: %s\n", chaosEnablePath)
			}
		})
		return nil
	},
}

var chaosDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable chaos injection",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Send request to admin API
		client := NewAdminClientWithAuth(adminURL)
		chaosConfig := map[string]interface{}{
			"enabled": false,
		}

		if err := client.SetChaosConfig(chaosConfig); err != nil {
			return fmt.Errorf("failed to disable chaos: %s", FormatConnectionError(err))
		}

		printResult(map[string]any{"enabled": false}, func() {
			fmt.Println("Chaos injection disabled")
		})
		return nil
	},
}

var chaosStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current chaos configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get chaos config from admin API
		client := NewAdminClientWithAuth(adminURL)
		config, err := client.GetChaosConfig()
		if err != nil {
			return fmt.Errorf("failed to get chaos status: %s", FormatConnectionError(err))
		}

		printResult(config, func() {
			// Pretty print status
			enabled, _ := config["enabled"].(bool)
			if !enabled {
				fmt.Println("Chaos injection: disabled")
				return
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
		})
		return nil
	},
}

var chaosProfilesCmd = &cobra.Command{
	Use:   "profiles",
	Short: "List available chaos profiles",
	Long:  `List all built-in chaos profiles that can be applied by name.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		profiles, err := client.ListChaosProfiles()
		if err != nil {
			return fmt.Errorf("failed to list chaos profiles: %s", FormatConnectionError(err))
		}

		printList(profiles, func() {
			fmt.Println("Available chaos profiles:")
			fmt.Println()
			for _, p := range profiles {
				fmt.Printf("  %-15s %s\n", p.Name, p.Description)
			}
			fmt.Println()
			fmt.Println("Apply a profile with: mockd chaos apply <profile-name>")
		})
		return nil
	},
}

var chaosApplyCmd = &cobra.Command{
	Use:   "apply <profile-name>",
	Short: "Apply a named chaos profile",
	Long: `Apply a built-in chaos profile by name. This sets the chaos configuration
to the profile's preset values and enables chaos injection.

Use "mockd chaos profiles" to see available profiles.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		profileName := args[0]

		client := NewAdminClientWithAuth(adminURL)

		// First get the profile details to show what's being applied
		profile, err := client.GetChaosProfile(profileName)
		if err != nil {
			return fmt.Errorf("failed to get chaos profile: %s", FormatConnectionError(err))
		}

		if err := client.ApplyChaosProfile(profileName); err != nil {
			return fmt.Errorf("failed to apply chaos profile: %s", FormatConnectionError(err))
		}

		printResult(map[string]any{
			"profile": profileName,
			"applied": true,
			"config":  profile.Config,
		}, func() {
			fmt.Printf("Applied chaos profile: %s\n", profileName)
			fmt.Printf("  %s\n", profile.Description)
			fmt.Println()
			printChaosProfileConfig(profile)
			fmt.Println()
			fmt.Println("Disable with: mockd chaos disable")
		})
		return nil
	},
}

// printChaosProfileConfig displays the configuration details of a chaos profile.
func printChaosProfileConfig(p *ChaosProfileInfo) {
	cfg := p.Config
	if latency, ok := cfg["latency"].(map[string]interface{}); ok {
		min, _ := latency["min"].(string)
		max, _ := latency["max"].(string)
		fmt.Printf("  Latency: %s - %s\n", min, max)
	}
	if errorRate, ok := cfg["errorRate"].(map[string]interface{}); ok {
		prob, _ := errorRate["probability"].(float64)
		fmt.Printf("  Error rate: %.0f%%\n", prob*100)
		if codes, ok := errorRate["statusCodes"].([]interface{}); ok && len(codes) > 0 {
			fmt.Printf("  Error codes: ")
			for i, c := range codes {
				if i > 0 {
					fmt.Print(", ")
				}
				if code, ok := c.(float64); ok {
					fmt.Printf("%d", int(code))
				}
			}
			fmt.Println()
		}
	}
	if bandwidth, ok := cfg["bandwidth"].(map[string]interface{}); ok {
		bps, _ := bandwidth["bytesPerSecond"].(float64)
		if bps >= 1024 {
			fmt.Printf("  Bandwidth: %.0f KB/s\n", bps/1024)
		} else {
			fmt.Printf("  Bandwidth: %.0f B/s\n", bps)
		}
	}
}

func init() {
	rootCmd.AddCommand(chaosCmd)

	chaosCmd.AddCommand(chaosEnableCmd)
	chaosEnableCmd.Flags().StringVarP(&chaosEnableLatency, "latency", "l", "", "Add random latency (e.g., \"10ms-100ms\")")
	chaosEnableCmd.Flags().Float64VarP(&chaosEnableErrorRate, "error-rate", "e", 0, "Error rate (0.0-1.0)")
	chaosEnableCmd.Flags().IntVar(&chaosEnableErrorCode, "error-code", 500, "HTTP error code to return")
	chaosEnableCmd.Flags().StringVarP(&chaosEnablePath, "path", "p", "", "Path pattern to apply chaos to (regex)")
	chaosEnableCmd.Flags().Float64Var(&chaosEnableProbability, "probability", 1.0, "Probability of applying chaos (0.0-1.0)")

	chaosCmd.AddCommand(chaosDisableCmd)
	chaosCmd.AddCommand(chaosStatusCmd)
	chaosCmd.AddCommand(chaosProfilesCmd)
	chaosCmd.AddCommand(chaosApplyCmd)
}
