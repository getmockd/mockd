package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	verifyExactly int
	verifyAtLeast int
	verifyAtMost  int
	verifyNever   bool
	verifyAll     bool
)

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify mock call counts and inspect invocations",
	Long: `Verify that your mocks were called the expected number of times
and inspect the request details of each invocation. Useful for
integration testing where you need to prove your code makes the
correct API calls.`,
}

var verifyStatusCmd = &cobra.Command{
	Use:   "status <mock-id>",
	Short: "Show call count and last-called time for a mock",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mockID := args[0]
		client := NewAdminClientWithAuth(adminURL)

		data, err := client.GetMockVerification(mockID)
		if err != nil {
			return fmt.Errorf("failed to get verification status: %s", FormatConnectionError(err))
		}

		printResult(data, func() {
			callCount := 0
			if c, ok := data["callCount"].(float64); ok {
				callCount = int(c)
			}
			fmt.Printf("Mock: %s\n", mockID)
			fmt.Printf("  Call count: %d\n", callCount)
			if lastCalled, ok := data["lastCalledAt"].(string); ok && lastCalled != "" {
				if t, err := time.Parse(time.RFC3339Nano, lastCalled); err == nil {
					fmt.Printf("  Last called: %s\n", t.Format("2006-01-02 15:04:05"))
				} else {
					fmt.Printf("  Last called: %s\n", lastCalled)
				}
			} else {
				fmt.Println("  Last called: never")
			}
		})
		return nil
	},
}

var verifyCheckCmd = &cobra.Command{
	Use:   "check <mock-id>",
	Short: "Assert that a mock was called the expected number of times",
	Long: `Assert call count expectations for a mock. Returns exit code 0
on pass and exit code 1 on failure — suitable for CI scripts.

At least one assertion flag is required.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mockID := args[0]

		// Build criteria from flags
		criteria := make(map[string]interface{})
		if verifyNever {
			criteria["never"] = true
		}
		if cmd.Flags().Changed("exactly") {
			criteria["exactly"] = verifyExactly
		}
		if cmd.Flags().Changed("at-least") {
			criteria["atLeast"] = verifyAtLeast
		}
		if cmd.Flags().Changed("at-most") {
			criteria["atMost"] = verifyAtMost
		}

		if len(criteria) == 0 {
			return fmt.Errorf("at least one assertion flag is required (--exactly, --at-least, --at-most, --never)")
		}

		client := NewAdminClientWithAuth(adminURL)

		data, err := client.VerifyMock(mockID, criteria)
		if err != nil {
			return fmt.Errorf("failed to verify mock: %s", FormatConnectionError(err))
		}

		printResult(data, func() {
			passed, _ := data["passed"].(bool)
			actual := 0
			if a, ok := data["actual"].(float64); ok {
				actual = int(a)
			}
			msg, _ := data["message"].(string)

			if passed {
				fmt.Printf("PASS: %s (called %d time(s))\n", msg, actual)
			} else {
				fmt.Printf("FAIL: %s\n", msg)
			}
		})

		// Exit with non-zero code on failure for CI usage
		if passed, _ := data["passed"].(bool); !passed {
			// Return an error so cobra exits with code 1
			return fmt.Errorf("verification failed")
		}
		return nil
	},
}

var verifyInvocationsCmd = &cobra.Command{
	Use:   "invocations <mock-id>",
	Short: "List recorded request details for a mock",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mockID := args[0]
		client := NewAdminClientWithAuth(adminURL)

		data, err := client.ListMockInvocations(mockID)
		if err != nil {
			return fmt.Errorf("failed to list invocations: %s", FormatConnectionError(err))
		}

		printResult(data, func() {
			count := 0
			if c, ok := data["count"].(float64); ok {
				count = int(c)
			}
			fmt.Printf("Mock: %s (%d invocation(s))\n\n", mockID, count)

			invocations, _ := data["invocations"].([]interface{})
			for i, inv := range invocations {
				invMap, ok := inv.(map[string]interface{})
				if !ok {
					continue
				}
				method, _ := invMap["method"].(string)
				path, _ := invMap["path"].(string)
				ts, _ := invMap["timestamp"].(string)

				displayTime := ts
				if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
					displayTime = t.Format("15:04:05.000")
				}

				fmt.Printf("  [%d] %s %s %s at %s\n", i+1, displayTime, method, path, mockID)

				if body, ok := invMap["body"].(string); ok && body != "" {
					if len(body) > 120 {
						body = body[:120] + "..."
					}
					fmt.Printf("      Body: %s\n", body)
				}
			}

			if count == 0 {
				fmt.Println("  No invocations recorded.")
			}
		})
		return nil
	},
}

var verifyResetCmd = &cobra.Command{
	Use:   "reset [mock-id]",
	Short: "Clear verification data (call counts and invocation history)",
	Long: `Clear verification data for a specific mock, or for all mocks
with the --all flag. Use between test runs for isolation.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)

		if verifyAll || len(args) == 0 {
			if err := client.ResetAllVerification(); err != nil {
				return fmt.Errorf("failed to reset verification: %s", FormatConnectionError(err))
			}
			printResult(map[string]string{"message": "All verification data cleared"}, func() {
				fmt.Println("All verification data cleared")
			})
			return nil
		}

		mockID := args[0]
		if err := client.ResetMockVerification(mockID); err != nil {
			return fmt.Errorf("failed to reset verification for %s: %s", mockID, FormatConnectionError(err))
		}
		printResult(map[string]string{"message": "Verification data cleared", "mockId": mockID}, func() {
			fmt.Printf("Verification data cleared for mock: %s\n", mockID)
		})
		return nil
	},
}

func init() {
	rootCmd.AddCommand(verifyCmd)

	verifyCmd.AddCommand(verifyStatusCmd)

	verifyCmd.AddCommand(verifyCheckCmd)
	verifyCheckCmd.Flags().IntVar(&verifyExactly, "exactly", 0, "Assert mock was called exactly N times")
	verifyCheckCmd.Flags().IntVar(&verifyAtLeast, "at-least", 0, "Assert mock was called at least N times")
	verifyCheckCmd.Flags().IntVar(&verifyAtMost, "at-most", 0, "Assert mock was called at most N times")
	verifyCheckCmd.Flags().BoolVar(&verifyNever, "never", false, "Assert mock was never called")

	verifyCmd.AddCommand(verifyInvocationsCmd)

	verifyCmd.AddCommand(verifyResetCmd)
	verifyResetCmd.Flags().BoolVar(&verifyAll, "all", false, "Reset verification data for all mocks")
}
