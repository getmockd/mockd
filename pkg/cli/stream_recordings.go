package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// streamRecordingStore is the global file store for stream recordings.
var streamRecordingStore *recording.FileStore

// initStreamRecordingStore initializes the global file store if needed.
func initStreamRecordingStore() error {
	if streamRecordingStore != nil {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	cfg := recording.StorageConfig{
		DataDir:     filepath.Join(homeDir, ".config", "mockd", "recordings"),
		MaxBytes:    recording.DefaultMaxStorageBytes,
		WarnPercent: recording.DefaultWarnPercent,
	}

	store, err := recording.NewFileStore(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize recording store: %w", err)
	}

	streamRecordingStore = store
	return nil
}

// RunStreamRecordings handles the stream-recordings command and its subcommands.
func RunStreamRecordings(args []string) error {
	if len(args) == 0 {
		printStreamRecordingsUsage()
		return nil
	}

	if err := initStreamRecordingStore(); err != nil {
		return err
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "list", "ls":
		return runStreamRecordingsList(subArgs)
	case "show", "get":
		return runStreamRecordingsShow(subArgs)
	case "delete", "rm":
		return runStreamRecordingsDelete(subArgs)
	case "export":
		return runStreamRecordingsExport(subArgs)
	case "convert":
		return runStreamRecordingsConvert(subArgs)
	case "stats":
		return runStreamRecordingsStats(subArgs)
	case "vacuum":
		return runStreamRecordingsVacuum(subArgs)
	case "sessions":
		return runStreamRecordingsSessions(subArgs)
	case "help", "--help", "-h":
		printStreamRecordingsUsage()
		return nil
	default:
		return fmt.Errorf("unknown stream-recordings subcommand: %s\n\nRun 'mockd stream-recordings --help' for usage", subcommand)
	}
}

func printStreamRecordingsUsage() {
	fmt.Print(`Usage: mockd stream-recordings <subcommand> [flags]

Manage WebSocket and SSE stream recordings.

Subcommands:
  list, ls      List all stream recordings
  show, get     Show details of a specific recording
  delete, rm    Delete a recording
  export        Export a recording to JSON
  convert       Convert a recording to mock config
  stats         Show storage statistics
  vacuum        Remove soft-deleted recordings
  sessions      List active recording sessions

Run 'mockd stream-recordings <subcommand> --help' for more information.
`)
}

// runStreamRecordingsList lists all stream recordings.
func runStreamRecordingsList(args []string) error {
	fs := flag.NewFlagSet("stream-recordings list", flag.ContinueOnError)

	protocol := fs.String("protocol", "", "Filter by protocol (websocket, sse)")
	path := fs.String("path", "", "Filter by path prefix")
	status := fs.String("status", "", "Filter by status (complete, incomplete, recording)")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	limit := fs.Int("limit", 20, "Maximum number of recordings to show")
	offset := fs.Int("offset", 0, "Offset for pagination")
	sortBy := fs.String("sort", "startTime", "Sort by field (startTime, name, size)")
	sortOrder := fs.String("order", "desc", "Sort order (asc, desc)")
	includeDeleted := fs.Bool("include-deleted", false, "Include soft-deleted recordings")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd stream-recordings list [flags]

List all WebSocket and SSE stream recordings.

Flags:
      --protocol        Filter by protocol (websocket, sse)
      --path            Filter by path prefix
      --status          Filter by status (complete, incomplete, recording)
      --json            Output as JSON
      --limit           Maximum number of recordings to show (default: 20)
      --offset          Offset for pagination
      --sort            Sort by field: startTime, name, size (default: startTime)
      --order           Sort order: asc, desc (default: desc)
      --include-deleted Include soft-deleted recordings

Examples:
  # List all recordings
  mockd stream-recordings list

  # List only WebSocket recordings
  mockd stream-recordings list --protocol websocket

  # List as JSON
  mockd stream-recordings list --json

  # Paginate results
  mockd stream-recordings list --limit 10 --offset 20
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	filter := recording.StreamRecordingFilter{
		Protocol:       recording.Protocol(*protocol),
		Path:           *path,
		Status:         *status,
		Limit:          *limit,
		Offset:         *offset,
		SortBy:         *sortBy,
		SortOrder:      *sortOrder,
		IncludeDeleted: *includeDeleted,
	}

	recordings, total, err := streamRecordingStore.List(filter)
	if err != nil {
		return fmt.Errorf("failed to list recordings: %w", err)
	}

	if *jsonOutput {
		output, err := json.MarshalIndent(recordings, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal recordings: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	if len(recordings) == 0 {
		fmt.Println("No stream recordings found")
		return nil
	}

	// Table output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tPROTOCOL\tPATH\tSTATUS\tFRAMES\tDURATION\tSIZE")
	for _, r := range recordings {
		duration := formatDurationMs(r.Duration)
		size := formatBytes(r.FileSize)
		idShort := r.ID
		if len(idShort) > 12 {
			idShort = idShort[:12]
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\t%s\n",
			idShort, r.Protocol, truncatePath(r.Path, 30), r.Status, r.FrameCount, duration, size)
	}
	_ = w.Flush()

	if len(recordings) < total {
		fmt.Printf("\nShowing %d of %d recordings (use --offset for more)\n", len(recordings), total)
	}

	return nil
}

// runStreamRecordingsShow shows details of a specific recording.
func runStreamRecordingsShow(args []string) error {
	fs := flag.NewFlagSet("stream-recordings show", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd stream-recordings show <id> [flags]

Show details of a specific stream recording.

Flags:
      --json    Output as JSON

Examples:
  mockd stream-recordings show 01ABCDEF123456
  mockd stream-recordings show 01ABCDEF123456 --json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("recording ID is required")
	}

	id := fs.Arg(0)
	rec, err := streamRecordingStore.Get(id)
	if err != nil {
		return fmt.Errorf("recording not found: %w", err)
	}

	if *jsonOutput {
		output, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal recording: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	// Human-readable output
	fmt.Printf("ID:          %s\n", rec.ID)
	fmt.Printf("Name:        %s\n", rec.Name)
	fmt.Printf("Protocol:    %s\n", rec.Protocol)
	fmt.Printf("Path:        %s\n", rec.Metadata.Path)
	fmt.Printf("Status:      %s\n", rec.Status)
	fmt.Printf("Started:     %s\n", rec.StartTime.Format(time.RFC3339))
	if rec.EndTime != nil {
		fmt.Printf("Ended:       %s\n", rec.EndTime.Format(time.RFC3339))
	}
	fmt.Printf("Duration:    %s\n", formatDurationMs(rec.Duration))
	fmt.Printf("Frames:      %d\n", rec.Stats.FrameCount)
	fmt.Printf("File Size:   %s\n", formatBytes(rec.Stats.FileSizeBytes))

	if rec.Description != "" {
		fmt.Printf("Description: %s\n", rec.Description)
	}
	if len(rec.Tags) > 0 {
		fmt.Printf("Tags:        %v\n", rec.Tags)
	}

	// Protocol-specific info
	switch rec.Protocol {
	case recording.ProtocolWebSocket:
		if rec.WebSocket != nil {
			fmt.Printf("\nWebSocket Details:\n")
			fmt.Printf("  Text Frames:   %d\n", rec.Stats.TextFrames)
			fmt.Printf("  Binary Frames: %d\n", rec.Stats.BinaryFrames)
			fmt.Printf("  Ping/Pong:     %d\n", rec.Stats.PingPongs)
			if rec.WebSocket.CloseCode != nil {
				fmt.Printf("  Close Code:    %d\n", *rec.WebSocket.CloseCode)
			}
		}
	case recording.ProtocolSSE:
		if rec.SSE != nil {
			fmt.Printf("\nSSE Details:\n")
			fmt.Printf("  Events: %d\n", rec.Stats.EventCount)
		}
	}

	return nil
}

// runStreamRecordingsDelete deletes a recording.
func runStreamRecordingsDelete(args []string) error {
	fs := flag.NewFlagSet("stream-recordings delete", flag.ContinueOnError)
	force := fs.Bool("force", false, "Skip confirmation")
	fs.BoolVar(force, "f", false, "Skip confirmation (shorthand)")
	permanent := fs.Bool("permanent", false, "Permanently delete (not soft-delete)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd stream-recordings delete <id> [flags]

Delete a stream recording.

Flags:
  -f, --force      Skip confirmation
      --permanent  Permanently delete (not soft-delete)

Examples:
  mockd stream-recordings delete 01ABCDEF123456
  mockd stream-recordings delete 01ABCDEF123456 --force
  mockd stream-recordings delete 01ABCDEF123456 --permanent
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("recording ID is required")
	}

	id := fs.Arg(0)

	// Verify it exists
	rec, err := streamRecordingStore.Get(id)
	if err != nil {
		return fmt.Errorf("recording not found: %w", err)
	}

	if !*force {
		fmt.Printf("Delete recording '%s' (%s %s)? [y/N]: ", rec.Name, rec.Protocol, rec.Metadata.Path)
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	if *permanent {
		if err := streamRecordingStore.Purge(id); err != nil {
			return fmt.Errorf("failed to permanently delete recording: %w", err)
		}
		fmt.Printf("Permanently deleted recording: %s\n", id)
	} else {
		if err := streamRecordingStore.Delete(id); err != nil {
			return fmt.Errorf("failed to delete recording: %w", err)
		}
		fmt.Printf("Deleted recording: %s (use 'vacuum' to permanently remove)\n", id)
	}

	return nil
}

// runStreamRecordingsExport exports a recording to JSON.
func runStreamRecordingsExport(args []string) error {
	fs := flag.NewFlagSet("stream-recordings export", flag.ContinueOnError)
	output := fs.String("output", "", "Output file path (default: stdout)")
	fs.StringVar(output, "o", "", "Output file path (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd stream-recordings export <id> [flags]

Export a stream recording to JSON format.

Flags:
  -o, --output    Output file path (default: stdout)

Examples:
  mockd stream-recordings export 01ABCDEF123456
  mockd stream-recordings export 01ABCDEF123456 -o recording.json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("recording ID is required")
	}

	id := fs.Arg(0)
	data, err := streamRecordingStore.Export(id, recording.ExportFormatJSON)
	if err != nil {
		return fmt.Errorf("failed to export recording: %w", err)
	}

	if *output == "" {
		fmt.Println(string(data))
	} else {
		if err := os.WriteFile(*output, data, 0644); err != nil {
			return fmt.Errorf("failed to write export: %w", err)
		}
		fmt.Printf("Recording exported to: %s\n", *output)
	}

	return nil
}

// runStreamRecordingsConvert converts a recording to mock config.
func runStreamRecordingsConvert(args []string) error {
	fs := flag.NewFlagSet("stream-recordings convert", flag.ContinueOnError)
	output := fs.String("output", "", "Output file path (default: stdout)")
	fs.StringVar(output, "o", "", "Output file path (shorthand)")
	simplifyTiming := fs.Bool("simplify-timing", false, "Normalize timing to reduce noise")
	minDelay := fs.Int("min-delay", 10, "Minimum delay to preserve (ms)")
	maxDelay := fs.Int("max-delay", 5000, "Maximum delay (ms)")
	includeClient := fs.Bool("include-client", true, "Include client messages as expect steps")
	deduplicate := fs.Bool("deduplicate", false, "Remove consecutive duplicate messages")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd stream-recordings convert <id> [flags]

Convert a stream recording to a mock configuration.

For WebSocket recordings, creates a scenario config.
For SSE recordings, creates an SSE mock config.

Flags:
  -o, --output          Output file path (default: stdout)
      --simplify-timing Normalize timing to reduce noise
      --min-delay       Minimum delay to preserve in ms (default: 10)
      --max-delay       Maximum delay in ms (default: 5000)
      --include-client  Include client messages as expect steps (default: true)
      --deduplicate     Remove consecutive duplicate messages

Examples:
  mockd stream-recordings convert 01ABCDEF123456
  mockd stream-recordings convert 01ABCDEF123456 --simplify-timing
  mockd stream-recordings convert 01ABCDEF123456 -o scenario.json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		return fmt.Errorf("recording ID is required")
	}

	id := fs.Arg(0)
	rec, err := streamRecordingStore.Get(id)
	if err != nil {
		return fmt.Errorf("recording not found: %w", err)
	}

	opts := recording.StreamConvertOptions{
		SimplifyTiming:        *simplifyTiming,
		MinDelay:              *minDelay,
		MaxDelay:              *maxDelay,
		IncludeClientMessages: *includeClient,
		DeduplicateMessages:   *deduplicate,
		Format:                "json",
	}

	result, err := recording.ConvertStreamRecording(rec, opts)
	if err != nil {
		return fmt.Errorf("failed to convert recording: %w", err)
	}

	if *output == "" {
		fmt.Println(string(result.ConfigJSON))
	} else {
		if err := os.WriteFile(*output, result.ConfigJSON, 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Printf("Converted %s recording to mock config: %s\n", result.Protocol, *output)
	}

	return nil
}

// runStreamRecordingsStats shows storage statistics.
func runStreamRecordingsStats(args []string) error {
	fs := flag.NewFlagSet("stream-recordings stats", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd stream-recordings stats [flags]

Show storage statistics for stream recordings.

Flags:
      --json    Output as JSON
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	stats, err := streamRecordingStore.GetStats()
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	if *jsonOutput {
		output, err := json.MarshalIndent(stats, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal stats: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	fmt.Printf("Storage Statistics:\n")
	fmt.Printf("  Used:        %s / %s (%.1f%%)\n",
		formatBytes(stats.UsedBytes), formatBytes(stats.MaxBytes), stats.UsedPercent)
	fmt.Printf("  Recordings:  %d total\n", stats.RecordingCount)
	fmt.Printf("    HTTP:      %d\n", stats.HTTPCount)
	fmt.Printf("    WebSocket: %d\n", stats.WebSocketCount)
	fmt.Printf("    SSE:       %d\n", stats.SSECount)

	if stats.OldestDate != nil {
		fmt.Printf("  Oldest:      %s (%s)\n", stats.OldestRecording, stats.OldestDate.Format(time.RFC3339))
	}
	if stats.NewestDate != nil {
		fmt.Printf("  Newest:      %s (%s)\n", stats.NewestRecording, stats.NewestDate.Format(time.RFC3339))
	}

	return nil
}

// runStreamRecordingsVacuum removes soft-deleted recordings.
func runStreamRecordingsVacuum(args []string) error {
	fs := flag.NewFlagSet("stream-recordings vacuum", flag.ContinueOnError)
	force := fs.Bool("force", false, "Skip confirmation")
	fs.BoolVar(force, "f", false, "Skip confirmation (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd stream-recordings vacuum [flags]

Permanently remove soft-deleted recordings to free up disk space.

Flags:
  -f, --force    Skip confirmation

Examples:
  mockd stream-recordings vacuum
  mockd stream-recordings vacuum --force
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*force {
		fmt.Print("This will permanently remove all soft-deleted recordings. Continue? [y/N]: ")
		var response string
		_, _ = fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	removed, freedBytes, err := streamRecordingStore.Vacuum()
	if err != nil {
		return fmt.Errorf("vacuum failed: %w", err)
	}

	if removed == 0 {
		fmt.Println("No soft-deleted recordings to remove")
	} else {
		fmt.Printf("Removed %d recordings, freed %s\n", removed, formatBytes(freedBytes))
	}

	return nil
}

// runStreamRecordingsSessions lists active recording sessions.
func runStreamRecordingsSessions(args []string) error {
	fs := flag.NewFlagSet("stream-recordings sessions", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd stream-recordings sessions [flags]

List active recording sessions.

Flags:
      --json    Output as JSON
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	sessions := streamRecordingStore.GetActiveSessions()

	if *jsonOutput {
		output, err := json.MarshalIndent(sessions, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal sessions: %w", err)
		}
		fmt.Println(string(output))
		return nil
	}

	if len(sessions) == 0 {
		fmt.Println("No active recording sessions")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tPROTOCOL\tPATH\tDURATION\tFRAMES")
	for _, s := range sessions {
		idShort := s.ID
		if len(idShort) > 12 {
			idShort = idShort[:12]
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\n",
			idShort, s.Protocol, truncatePath(s.Path, 30), s.Duration, s.FrameCount)
	}
	_ = w.Flush()

	return nil
}

// Helper functions

func formatDurationMs(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("%dms", ms)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return d.Round(time.Second).String()
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}
