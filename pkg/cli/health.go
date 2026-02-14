package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/getmockd/mockd/pkg/cliconfig"
)

// RunHealth checks if a mockd server is healthy and reachable.
func RunHealth(args []string) error {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)

	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	jsonOutput := fs.Bool("json", false, "Output result as JSON")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd health [flags]

Check if the mockd server is healthy and reachable.

Flags:
      --admin-url    Admin API base URL (default: http://localhost:4290)
      --json         Output result as JSON

Examples:
  # Check default server health
  mockd health

  # Check a specific server
  mockd health --admin-url http://localhost:4290

  # JSON output for scripting
  mockd health --json
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	client := NewAdminClientWithAuth(*adminURL)

	type healthResult struct {
		Status   string `json:"status"`
		AdminURL string `json:"adminUrl"`
		Error    string `json:"error,omitempty"`
	}

	err := client.Health()
	if err != nil {
		result := healthResult{
			Status:   "unhealthy",
			AdminURL: *adminURL,
			Error:    err.Error(),
		}
		if *jsonOutput {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		} else {
			fmt.Fprintf(os.Stderr, "unhealthy: %s\n", FormatConnectionError(err))
		}
		return fmt.Errorf("server is not healthy")
	}

	result := healthResult{
		Status:   "healthy",
		AdminURL: *adminURL,
	}
	if *jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Println("healthy")
	}
	return nil
}
