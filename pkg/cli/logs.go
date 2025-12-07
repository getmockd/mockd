package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/getmockd/mockd/internal/cliconfig"
	"github.com/getmockd/mockd/pkg/config"
)

// RunLogs handles the logs command.
func RunLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)

	method := fs.String("method", "", "Filter by HTTP method")
	fs.StringVar(method, "m", "", "Filter by HTTP method (shorthand)")
	path := fs.String("path", "", "Filter by path (substring match)")
	fs.StringVar(path, "p", "", "Filter by path (shorthand)")
	matched := fs.Bool("matched", false, "Show only matched requests")
	unmatched := fs.Bool("unmatched", false, "Show only unmatched requests")
	limit := fs.Int("limit", 20, "Number of entries to show")
	fs.IntVar(limit, "n", 20, "Number of entries to show (shorthand)")
	verbose := fs.Bool("verbose", false, "Show headers and body")
	clear := fs.Bool("clear", false, "Clear all logs")
	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd logs [flags]

View request logs.

Flags:
  -m, --method      Filter by HTTP method
  -p, --path        Filter by path (substring match)
      --matched     Show only matched requests
      --unmatched   Show only unmatched requests
  -n, --limit       Number of entries to show (default: 20)
      --verbose     Show headers and body
      --clear       Clear all logs
      --admin-url   Admin API base URL (default: http://localhost:9090)
      --json        Output in JSON format

Examples:
  # Show recent logs
  mockd logs

  # Show last 50 entries
  mockd logs -n 50

  # Filter by method
  mockd logs -m POST

  # Show verbose output
  mockd logs --verbose

  # Clear logs
  mockd logs --clear
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	client := NewAdminClient(*adminURL)

	// Handle clear command
	if *clear {
		count, err := client.ClearLogs()
		if err != nil {
			return fmt.Errorf("%s", FormatConnectionError(err))
		}
		fmt.Printf("Cleared %d log entries\n", count)
		return nil
	}

	// Build filter
	filter := &LogFilter{
		Method: *method,
		Path:   *path,
		Limit:  *limit,
	}

	// Get logs
	result, err := client.GetLogs(filter)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Filter matched/unmatched locally
	requests := result.Requests
	if *matched || *unmatched {
		filtered := make([]*config.RequestLogEntry, 0)
		for _, req := range result.Requests {
			hasMatch := req.MatchedMockID != ""
			if (*matched && hasMatch) || (*unmatched && !hasMatch) {
				filtered = append(filtered, req)
			}
		}
		requests = filtered
	}

	// JSON output
	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(requests)
	}

	// No logs message
	if len(requests) == 0 {
		fmt.Println("No request logs")
		return nil
	}

	// Verbose output
	if *verbose {
		return printVerboseLogs(requests)
	}

	// Table output
	return printTableLogs(requests)
}

func printTableLogs(requests []*config.RequestLogEntry) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIMESTAMP\tMETHOD\tPATH\tMATCHED\tDURATION")

	for _, req := range requests {
		timestamp := req.Timestamp.Format("2006-01-02 15:04:05")
		path := req.Path
		if len(path) > 25 {
			path = path[:22] + "..."
		}
		matched := req.MatchedMockID
		if matched == "" {
			matched = "(none)"
		} else if len(matched) > 12 {
			matched = matched[:12] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%dms\n",
			timestamp, req.Method, path, matched, req.DurationMs)
	}

	return w.Flush()
}

func printVerboseLogs(requests []*config.RequestLogEntry) error {
	for _, req := range requests {
		timestamp := req.Timestamp.Format("2006-01-02 15:04:05")
		matched := req.MatchedMockID
		if matched == "" {
			matched = "(none)"
		}

		fmt.Printf("[%s] %s %s → %d (%dms)\n",
			timestamp, req.Method, req.Path, req.ResponseStatus, req.DurationMs)
		fmt.Printf("  Matched: %s\n", matched)

		if len(req.Headers) > 0 {
			fmt.Println("  Headers:")
			for key, values := range req.Headers {
				for _, value := range values {
					fmt.Printf("    %s: %s\n", key, value)
				}
			}
		}

		if req.Body != "" {
			body := req.Body
			if len(body) > 200 {
				body = body[:200] + "...(truncated)"
			}
			fmt.Printf("  Body: %s\n", body)
		} else {
			fmt.Println("  Body: (empty)")
		}
		fmt.Println()
	}
	return nil
}
