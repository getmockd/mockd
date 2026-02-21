package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/recording"
	"github.com/spf13/cobra"
)

var recordingsCmd = &cobra.Command{
	Use:   "recordings",
	Short: "Manage recorded API traffic on disk",
}

var (
	recordingsSessionsDir string

	recordingsListSession string
	recordingsListDir     string
	recordingsListMethod  string
	recordingsListHost    string
	recordingsListLimit   int

	recordingsExportSession string
	recordingsExportDir     string
	recordingsExportOutput  string

	recordingsImportInput   string
	recordingsImportSession string
	recordingsImportDir     string

	recordingsClearSession string
	recordingsClearDir     string
	recordingsClearForce   bool
)

var recordingsSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List all recording sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		recordingsDir := &recordingsSessionsDir

		sessions, err := recording.ListSessions(*recordingsDir)
		if err != nil {
			return err
		}

		if len(sessions) == 0 {
			fmt.Println("No recording sessions found")
			fmt.Println("Run 'mockd proxy start' to capture traffic")
			return nil
		}

		if jsonOutput {
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
	},
}

var recordingsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recorded API requests from a session",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := &recordingsListSession
		recordingsDir := &recordingsListDir
		method := &recordingsListMethod
		host := &recordingsListHost
		limit := &recordingsListLimit

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

		if jsonOutput {
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
	},
}

var recordingsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export recordings to JSON format",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := &recordingsExportSession
		recordingsDir := &recordingsExportDir
		outputPath := &recordingsExportOutput

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
	},
}

var recordingsImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import recordings from JSON into local disk storage",
	RunE: func(cmd *cobra.Command, args []string) error {
		input := &recordingsImportInput
		sessionName := &recordingsImportSession
		recordingsDir := &recordingsImportDir

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
	},
}

var recordingsClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear recordings from a session or all sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := &recordingsClearSession
		recordingsDir := &recordingsClearDir
		force := &recordingsClearForce

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
	},
}

func init() {
	rootCmd.AddCommand(recordingsCmd)

	recordingsCmd.AddCommand(recordingsSessionsCmd)
	recordingsSessionsCmd.Flags().StringVar(&recordingsSessionsDir, "recordings-dir", "", "Base recordings directory override")

	recordingsCmd.AddCommand(recordingsListCmd)
	recordingsListCmd.Flags().StringVarP(&recordingsListSession, "session", "s", "", "Session name or directory (default: latest)")
	recordingsListCmd.Flags().StringVar(&recordingsListDir, "recordings-dir", "", "Base recordings directory override")
	recordingsListCmd.Flags().StringVar(&recordingsListMethod, "method", "", "Filter by HTTP method")
	recordingsListCmd.Flags().StringVar(&recordingsListHost, "host", "", "Filter by request host")
	recordingsListCmd.Flags().IntVar(&recordingsListLimit, "limit", 0, "Maximum number of recordings to show")

	recordingsCmd.AddCommand(recordingsExportCmd)
	recordingsExportCmd.Flags().StringVarP(&recordingsExportSession, "session", "s", "", "Session name or directory (default: latest)")
	recordingsExportCmd.Flags().StringVar(&recordingsExportDir, "recordings-dir", "", "Base recordings directory override")
	recordingsExportCmd.Flags().StringVarP(&recordingsExportOutput, "output", "o", "", "Output file path (default: stdout)")

	recordingsCmd.AddCommand(recordingsImportCmd)
	recordingsImportCmd.Flags().StringVarP(&recordingsImportInput, "input", "i", "", "Input file path (required)")
	recordingsImportCmd.Flags().StringVar(&recordingsImportSession, "session", "imported", "Session name for imported recordings")
	recordingsImportCmd.Flags().StringVar(&recordingsImportDir, "recordings-dir", "", "Base recordings directory override")

	recordingsCmd.AddCommand(recordingsClearCmd)
	recordingsClearCmd.Flags().StringVar(&recordingsClearSession, "session", "", "Session to clear (omit for all sessions)")
	recordingsClearCmd.Flags().StringVar(&recordingsClearDir, "recordings-dir", "", "Base recordings directory override")
	recordingsClearCmd.Flags().BoolVarP(&recordingsClearForce, "force", "f", false, "Skip confirmation")
}
