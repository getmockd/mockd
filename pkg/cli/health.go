package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Check if the mockd server is healthy and reachable",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)

		type healthResult struct {
			Status   string `json:"status"`
			AdminURL string `json:"adminUrl"`
			Error    string `json:"error,omitempty"`
		}

		err := client.Health()
		if err != nil {
			result := healthResult{
				Status:   "unhealthy",
				AdminURL: adminURL,
				Error:    err.Error(),
			}
			if jsonOutput {
				data, _ := json.MarshalIndent(result, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Fprintf(os.Stderr, "unhealthy: %s\n", FormatConnectionError(err))
			}
			return errors.New("server is not healthy")
		}

		result := healthResult{
			Status:   "healthy",
			AdminURL: adminURL,
		}
		if jsonOutput {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Println("healthy")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
