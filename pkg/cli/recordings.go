package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/recording"
)

// RunRecordings handles the recordings command and its subcommands.
func RunRecordings(args []string) error {
	if len(args) == 0 {
		printRecordingsUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "list":
		return runRecordingsList(subArgs)
	case "convert":
		return runRecordingsConvert(subArgs)
	case "export":
		return runRecordingsExport(subArgs)
	case "import":
		return runRecordingsImport(subArgs)
	case "clear":
		return runRecordingsClear(subArgs)
	case "help", "--help", "-h":
		printRecordingsUsage()
		return nil
	default:
		return fmt.Errorf("unknown recordings subcommand: %s\n\nRun 'mockd recordings --help' for usage", subcommand)
	}
}

func printRecordingsUsage() {
	fmt.Print(`Usage: mockd recordings <subcommand> [flags]

Manage recorded API traffic.

Subcommands:
  list      List all recordings
  convert   Convert recordings to mock definitions
  export    Export recordings to JSON
  import    Import recordings from JSON
  clear     Clear all recordings

Run 'mockd recordings <subcommand> --help' for more information.
`)
}

// runRecordingsList lists all recordings.
func runRecordingsList(args []string) error {
	fs := flag.NewFlagSet("recordings list", flag.ContinueOnError)

	sessionID := fs.String("session", "", "Filter by session ID")
	method := fs.String("method", "", "Filter by HTTP method")
	path := fs.String("path", "", "Filter by request path")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	limit := fs.Int("limit", 0, "Maximum number of recordings to show")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd recordings list [flags]

List all recorded API requests.

Flags:
      --session   Filter by session ID
      --method    Filter by HTTP method
      --path      Filter by request path
      --json      Output as JSON
      --limit     Maximum number of recordings to show

Examples:
  # List all recordings
  mockd recordings list

  # List only GET requests
  mockd recordings list --method GET

  # List as JSON
  mockd recordings list --json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Use in-process store if available, otherwise fall back to admin API.
	var recordings []*recording.Recording
	var total int

	if proxyServer.store != nil {
		filter := recording.RecordingFilter{
			SessionID: *sessionID,
			Method:    *method,
			Path:      *path,
			Limit:     *limit,
		}
		recordings, total = proxyServer.store.ListRecordings(filter)
	} else {
		// Proxy not running in this process — fetch from admin API.
		adminURL := cliconfig.ResolveAdminURL("")
		resp, err := http.Get(adminURL + "/recordings")
		if err != nil {
			return fmt.Errorf("failed to list recordings: %w (is mockd running?)", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to list recordings: HTTP %d", resp.StatusCode)
		}
		var data struct {
			Recordings []*recording.Recording `json:"recordings"`
			Total      int                    `json:"total"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
			return fmt.Errorf("failed to decode recordings: %w", err)
		}
		recordings = data.Recordings
		total = data.Total
	}

	if *jsonOutput {
		return output.JSON(recordings)
	}

	if len(recordings) == 0 {
		fmt.Println("No recordings found")
		return nil
	}

	// Table output
	w := output.Table()
	_, _ = fmt.Fprintln(w, "ID\tMETHOD\tPATH\tSTATUS\tDURATION")
	for _, r := range recordings {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%v\n",
			r.ID[:8], r.Request.Method, r.Request.Path, r.Response.StatusCode, r.Duration)
	}
	_ = w.Flush()

	if len(recordings) < total {
		fmt.Printf("\nShowing %d of %d recordings\n", len(recordings), total)
	}

	return nil
}

// runRecordingsConvert converts recordings to mock definitions.
func runRecordingsConvert(args []string) error {
	fs := flag.NewFlagSet("recordings convert", flag.ContinueOnError)

	sessionID := fs.String("session", "", "Filter by session ID")
	deduplicate := fs.Bool("deduplicate", true, "Remove duplicate request patterns")
	includeHeaders := fs.Bool("include-headers", false, "Include request headers in matchers")
	output := fs.String("output", "", "Output file path (default: stdout)")
	fs.StringVar(output, "o", "", "Output file path (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd recordings convert [flags]

Convert recordings to mock definitions.

Flags:
      --session         Filter by session ID
      --deduplicate     Remove duplicate request patterns (default: true)
      --include-headers Include request headers in matchers
  -o, --output          Output file path (default: stdout)

Examples:
  # Convert all recordings to mocks
  mockd recordings convert

  # Convert specific session with deduplication
  mockd recordings convert --session my-session --deduplicate

  # Save to file
  mockd recordings convert -o mocks.json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if proxyServer.store == nil {
		return ErrProxyNotRunning
	}

	filter := recording.RecordingFilter{
		SessionID: *sessionID,
	}

	recordings, _ := proxyServer.store.ListRecordings(filter)
	if len(recordings) == 0 {
		return errors.New("no recordings to convert")
	}

	opts := recording.ConvertOptions{
		Deduplicate:    *deduplicate,
		IncludeHeaders: *includeHeaders,
	}

	mocks := recording.ToMocks(recordings, opts)

	mockOutput, err := json.MarshalIndent(mocks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mocks: %w", err)
	}

	if *output == "" {
		fmt.Println(string(mockOutput))
	} else {
		if err := os.WriteFile(*output, mockOutput, 0644); err != nil {
			return fmt.Errorf("failed to write mocks: %w", err)
		}
		fmt.Printf("Converted %d recordings to %d mocks\n", len(recordings), len(mocks))
		fmt.Printf("Output written to: %s\n", *output)
	}

	return nil
}

// runRecordingsExport exports recordings to JSON.
func runRecordingsExport(args []string) error {
	fs := flag.NewFlagSet("recordings export", flag.ContinueOnError)

	sessionID := fs.String("session", "", "Export specific session")
	output := fs.String("output", "", "Output file path (default: stdout)")
	fs.StringVar(output, "o", "", "Output file path (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd recordings export [flags]

Export recordings to JSON format.

Flags:
      --session   Export specific session
  -o, --output    Output file path (default: stdout)

Examples:
  # Export all recordings to stdout
  mockd recordings export

  # Export specific session to file
  mockd recordings export --session my-session -o recordings.json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if proxyServer.store == nil {
		return ErrProxyNotRunning
	}

	var jsonOutput []byte
	var err error

	if *sessionID != "" {
		jsonOutput, err = proxyServer.store.ExportSession(*sessionID)
	} else {
		jsonOutput, err = proxyServer.store.ExportRecordings(recording.RecordingFilter{})
	}

	if err != nil {
		return fmt.Errorf("failed to export recordings: %w", err)
	}

	if *output == "" {
		fmt.Println(string(jsonOutput))
	} else {
		if err := os.WriteFile(*output, jsonOutput, 0644); err != nil {
			return fmt.Errorf("failed to write export: %w", err)
		}
		fmt.Printf("Recordings exported to: %s\n", *output)
	}

	return nil
}

// runRecordingsImport imports recordings from JSON.
func runRecordingsImport(args []string) error {
	fs := flag.NewFlagSet("recordings import", flag.ContinueOnError)

	input := fs.String("input", "", "Input file path (required)")
	fs.StringVar(input, "i", "", "Input file path (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd recordings import [flags]

Import recordings from JSON format.

Flags:
  -i, --input   Input file path (required)

Examples:
  mockd recordings import -i recordings.json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *input == "" {
		return errors.New("--input is required")
	}

	if proxyServer.store == nil {
		return ErrProxyNotRunning
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		return fmt.Errorf("failed to read input file: %w", err)
	}

	// Try to parse as recordings array
	var recordings []*recording.Recording
	if err := json.Unmarshal(data, &recordings); err != nil {
		return fmt.Errorf("failed to parse recordings JSON: %w", err)
	}

	// Add recordings to store
	for _, r := range recordings {
		if err := proxyServer.store.AddRecording(r); err != nil {
			return fmt.Errorf("failed to add recording: %w", err)
		}
	}

	fmt.Printf("Imported %d recordings\n", len(recordings))
	return nil
}

// runRecordingsClear clears all recordings.
func runRecordingsClear(args []string) error {
	fs := flag.NewFlagSet("recordings clear", flag.ContinueOnError)

	force := fs.Bool("force", false, "Skip confirmation")
	fs.BoolVar(force, "f", false, "Skip confirmation (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd recordings clear [flags]

Clear all recordings.

Flags:
  -f, --force   Skip confirmation

Examples:
  mockd recordings clear
  mockd recordings clear --force
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if proxyServer.store == nil {
		return ErrProxyNotRunning
	}

	if !*force {
		_, total := proxyServer.store.ListRecordings(recording.RecordingFilter{})
		if total == 0 {
			fmt.Println("No recordings to clear")
			return nil
		}

		fmt.Printf("This will clear %d recordings. Continue? [y/N]: ", total)
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	count := proxyServer.store.Clear()
	fmt.Printf("Cleared %d recordings\n", count)
	return nil
}
