package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
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
	case "sessions":
		return runRecordingsSessions(subArgs)
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

Manage recorded API traffic on disk.

Subcommands:
  list      List recordings from a session
  sessions  List all recording sessions
  export    Export recordings to JSON
  import    Import recordings from JSON into local storage
  clear     Clear recordings from a session

Run 'mockd recordings <subcommand> --help' for more information.
`)
}

// runRecordingsSessions lists all recording sessions.
func runRecordingsSessions(args []string) error {
	fs := flag.NewFlagSet("recordings sessions", flag.ContinueOnError)

	recordingsDir := fs.String("recordings-dir", "", "Base recordings directory")
	jsonOutput := fs.Bool("json", false, "Output as JSON")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd recordings sessions [flags]

List all recording sessions.

Flags:
      --recordings-dir  Base recordings directory override
      --json            Output as JSON

Examples:
  mockd recordings sessions
  mockd recordings sessions --json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	sessions, err := recording.ListSessions(*recordingsDir)
	if err != nil {
		return err
	}

	if len(sessions) == 0 {
		fmt.Println("No recording sessions found")
		fmt.Println("Run 'mockd proxy start' to capture traffic")
		return nil
	}

	if *jsonOutput {
		return output.JSON(sessions)
	}

	w := output.Table()
	_, _ = fmt.Fprintln(w, "SESSION\tNAME\tSTART\tRECORDINGS\tHOSTS")
	for _, s := range sessions {
		startTime := s.Meta.StartTime
		if len(startTime) > 19 {
			startTime = startTime[:19] // Trim timezone for display
		}
		hosts := ""
		if len(s.Meta.Hosts) > 0 {
			hosts = fmt.Sprintf("%d hosts", len(s.Meta.Hosts))
		}
		count := s.Meta.RecordingCount
		if count == 0 {
			// Count from disk if meta doesn't have it
			count = recording.CountRecordingsInDir(s.Path)
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n",
			s.DirName, s.Meta.Name, startTime, count, hosts)
	}
	_ = w.Flush()

	return nil
}

// runRecordingsList lists recordings from a session.
func runRecordingsList(args []string) error {
	fs := flag.NewFlagSet("recordings list", flag.ContinueOnError)

	sessionName := fs.String("session", "", "Session name or directory (default: latest)")
	fs.StringVar(sessionName, "s", "", "Session name (shorthand)")
	recordingsDir := fs.String("recordings-dir", "", "Base recordings directory")
	method := fs.String("method", "", "Filter by HTTP method")
	host := fs.String("host", "", "Filter by request host")
	jsonOutput := fs.Bool("json", false, "Output as JSON")
	limit := fs.Int("limit", 0, "Maximum number of recordings to show")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd recordings list [flags]

List recorded API requests from a session.

Flags:
  -s, --session         Session name or directory (default: latest)
      --recordings-dir  Base recordings directory override
      --method          Filter by HTTP method
      --host            Filter by request host
      --json            Output as JSON
      --limit           Maximum number of recordings to show

Examples:
  # List recordings from latest session
  mockd recordings list

  # List recordings from named session
  mockd recordings list --session stripe-api

  # List only POST requests
  mockd recordings list --method POST

  # List as JSON
  mockd recordings list --json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Load recordings from disk
	recordings, err := loadRecordingsFromFlags("", *sessionName, *recordingsDir)
	if err != nil {
		return err
	}

	// Apply filters
	if *method != "" {
		var filtered []*recording.Recording
		for _, r := range recordings {
			if r.Request.Method == *method {
				filtered = append(filtered, r)
			}
		}
		recordings = filtered
	}
	if *host != "" {
		recordings = filterByHosts(recordings, []string{*host})
	}

	total := len(recordings)

	// Apply limit
	if *limit > 0 && len(recordings) > *limit {
		recordings = recordings[:*limit]
	}

	if *jsonOutput {
		return output.JSON(recordings)
	}

	if len(recordings) == 0 {
		fmt.Println("No recordings found")
		return nil
	}

	w := output.Table()
	_, _ = fmt.Fprintln(w, "ID\tMETHOD\tHOST\tPATH\tSTATUS\tDURATION")
	for _, r := range recordings {
		idShort := r.ID
		if len(idShort) > 8 {
			idShort = idShort[:8]
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%v\n",
			idShort, r.Request.Method, r.Request.Host, r.Request.Path,
			r.Response.StatusCode, r.Duration)
	}
	_ = w.Flush()

	if len(recordings) < total {
		fmt.Printf("\nShowing %d of %d recordings\n", len(recordings), total)
	}

	return nil
}

// runRecordingsExport exports recordings to a JSON file.
func runRecordingsExport(args []string) error {
	fs := flag.NewFlagSet("recordings export", flag.ContinueOnError)

	sessionName := fs.String("session", "", "Session name or directory (default: latest)")
	fs.StringVar(sessionName, "s", "", "Session name (shorthand)")
	recordingsDir := fs.String("recordings-dir", "", "Base recordings directory")
	outputPath := fs.String("output", "", "Output file path (default: stdout)")
	fs.StringVar(outputPath, "o", "", "Output file path (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd recordings export [flags]

Export recordings to JSON format.

Flags:
  -s, --session         Session name or directory (default: latest)
      --recordings-dir  Base recordings directory override
  -o, --output          Output file path (default: stdout)

Examples:
  # Export latest session to stdout
  mockd recordings export

  # Export named session to file
  mockd recordings export --session stripe-api -o recordings.json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	recordings, err := loadRecordingsFromFlags("", *sessionName, *recordingsDir)
	if err != nil {
		return err
	}

	if len(recordings) == 0 {
		return errors.New("no recordings to export")
	}

	data, err := json.MarshalIndent(recordings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal recordings: %w", err)
	}

	if *outputPath == "" {
		fmt.Println(string(data))
	} else {
		if err := os.WriteFile(*outputPath, data, 0644); err != nil {
			return fmt.Errorf("failed to write export: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Exported %d recordings to %s\n", len(recordings), *outputPath)
	}

	return nil
}

// runRecordingsImport imports recordings from a JSON file into local storage.
func runRecordingsImport(args []string) error {
	fs := flag.NewFlagSet("recordings import", flag.ContinueOnError)

	input := fs.String("input", "", "Input file path (required)")
	fs.StringVar(input, "i", "", "Input file path (shorthand)")
	sessionName := fs.String("session", "imported", "Session name for imported recordings")
	recordingsDir := fs.String("recordings-dir", "", "Base recordings directory")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd recordings import [flags]

Import recordings from JSON into local disk storage.

Flags:
  -i, --input           Input file path (required)
      --session         Session name for imported recordings (default: imported)
      --recordings-dir  Base recordings directory override

Examples:
  mockd recordings import -i recordings.json
  mockd recordings import -i recordings.json --session from-colleague
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *input == "" {
		return errors.New("--input is required")
	}

	// Load recordings from file
	recordings, err := recording.LoadFromFile(*input)
	if err != nil {
		return fmt.Errorf("failed to read recordings: %w", err)
	}

	if len(recordings) == 0 {
		return errors.New("no recordings found in file")
	}

	// Determine target directory
	baseDir := *recordingsDir
	if baseDir == "" {
		baseDir = recording.DefaultRecordingsBaseDir()
	}

	// Create session directory
	timestamp := "imported"
	sessionDir := filepath.Join(baseDir, *sessionName+"-"+timestamp)
	if err := os.MkdirAll(sessionDir, 0700); err != nil {
		return fmt.Errorf("failed to create session directory: %w", err)
	}

	// Write each recording to disk organized by host
	for _, rec := range recordings {
		host := rec.Request.Host
		if host == "" {
			host = "_unknown"
		}
		hostDir := filepath.Join(sessionDir, host)
		if err := os.MkdirAll(hostDir, 0700); err != nil {
			return fmt.Errorf("failed to create host directory: %w", err)
		}

		data, err := json.MarshalIndent(rec, "", "  ")
		if err != nil {
			continue
		}
		filename := filepath.Join(hostDir, "rec_"+rec.ID+".json")
		if err := os.WriteFile(filename, data, 0600); err != nil {
			return fmt.Errorf("failed to write recording: %w", err)
		}
	}

	fmt.Printf("Imported %d recordings to %s\n", len(recordings), sessionDir)
	return nil
}

// runRecordingsClear clears recordings from a session or all sessions.
func runRecordingsClear(args []string) error {
	fs := flag.NewFlagSet("recordings clear", flag.ContinueOnError)

	sessionName := fs.String("session", "", "Session to clear (default: all sessions)")
	recordingsDir := fs.String("recordings-dir", "", "Base recordings directory")
	force := fs.Bool("force", false, "Skip confirmation")
	fs.BoolVar(force, "f", false, "Skip confirmation (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd recordings clear [flags]

Clear recordings from a session or all sessions.

Flags:
      --session         Session to clear (omit for all sessions)
      --recordings-dir  Base recordings directory override
  -f, --force           Skip confirmation

Examples:
  mockd recordings clear --session my-session --force
  mockd recordings clear --force
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	baseDir := *recordingsDir
	if baseDir == "" {
		baseDir = recording.DefaultRecordingsBaseDir()
	}

	if *sessionName != "" {
		// Clear specific session
		sessionDir, err := recording.ResolveSessionDir(baseDir, *sessionName)
		if err != nil {
			return err
		}

		count := recording.CountRecordingsInDir(sessionDir)
		if count == 0 {
			fmt.Println("No recordings to clear")
			return nil
		}

		if !*force {
			fmt.Printf("This will delete %d recordings from %s. Continue? [y/N]: ", count, filepath.Base(sessionDir))
			var response string
			_, _ = fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Cancelled")
				return nil
			}
		}

		if err := recording.DeleteSession(sessionDir); err != nil {
			return fmt.Errorf("failed to clear session: %w", err)
		}
		fmt.Printf("Cleared %d recordings from %s\n", count, filepath.Base(sessionDir))
	} else {
		// Clear all sessions
		sessions, err := recording.ListSessions(baseDir)
		if err != nil {
			return err
		}

		if len(sessions) == 0 {
			fmt.Println("No recording sessions found")
			return nil
		}

		totalCount := 0
		for _, s := range sessions {
			totalCount += recording.CountRecordingsInDir(s.Path)
		}

		if !*force {
			fmt.Printf("This will delete %d sessions with %d total recordings. Continue? [y/N]: ",
				len(sessions), totalCount)
			var response string
			_, _ = fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Println("Cancelled")
				return nil
			}
		}

		for _, s := range sessions {
			_ = recording.DeleteSession(s.Path)
		}
		// Also remove the latest symlink
		_ = os.Remove(filepath.Join(baseDir, "latest"))

		fmt.Printf("Cleared %d sessions with %d recordings\n", len(sessions), totalCount)
	}

	return nil
}
