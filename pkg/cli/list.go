package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/getmockd/mockd/internal/cliconfig"
)

// RunList handles the list command.
func RunList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)

	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd list [flags]

List all configured mocks.

Flags:
      --admin-url    Admin API base URL (default: http://localhost:9090)
      --json         Output in JSON format

Examples:
  # List all mocks
  mockd list

  # List as JSON
  mockd list --json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Create admin client and list mocks
	client := NewAdminClient(*adminURL)
	mocks, err := client.ListMocks()
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Output result
	if *jsonOutput {
		// Build simplified output
		type mockSummary struct {
			ID         string `json:"id"`
			Method     string `json:"method"`
			Path       string `json:"path"`
			StatusCode int    `json:"statusCode"`
			Enabled    bool   `json:"enabled"`
		}
		output := make([]mockSummary, 0, len(mocks))
		for _, m := range mocks {
			summary := mockSummary{
				ID:      m.ID,
				Enabled: m.Enabled,
			}
			if m.Matcher != nil {
				summary.Method = m.Matcher.Method
				summary.Path = m.Matcher.Path
			}
			if m.Response != nil {
				summary.StatusCode = m.Response.StatusCode
			}
			output = append(output, summary)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	// No mocks message
	if len(mocks) == 0 {
		fmt.Println("No mocks configured")
		return nil
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tMETHOD\tPATH\tSTATUS\tENABLED")

	for _, m := range mocks {
		method := ""
		path := ""
		status := 0
		if m.Matcher != nil {
			method = m.Matcher.Method
			path = m.Matcher.Path
		}
		if m.Response != nil {
			status = m.Response.StatusCode
		}

		// Truncate long paths
		if len(path) > 30 {
			path = path[:27] + "..."
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%t\n",
			m.ID, method, path, status, m.Enabled)
	}

	return w.Flush()
}
