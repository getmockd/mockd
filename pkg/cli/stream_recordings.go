package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/recording"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/spf13/cobra"
)

// streamRecordingStore is the global file store for stream recordings.
var streamRecordingStore *recording.FileStore

// initStreamRecordingStore initializes the global file store if needed.
func initStreamRecordingStore() error {
	if streamRecordingStore != nil {
		return nil
	}

	cfg := recording.StorageConfig{
		DataDir:     store.DefaultRecordingsDir(),
		MaxBytes:    recording.DefaultMaxStorageBytes,
		WarnPercent: recording.DefaultWarnPercent,
	}

	fileStore, err := recording.NewFileStore(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize recording store: %w", err)
	}

	streamRecordingStore = fileStore
	return nil
}

var streamRecordingsCmd = &cobra.Command{
	Use:   "stream-recordings",
	Short: "Manage WebSocket and SSE stream recordings",
	Long:  "Manage WebSocket and SSE stream recordings.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return initStreamRecordingStore()
	},
}

var (
	srListProtocol       string
	srListPath           string
	srListStatus         string
	srListLimit          int
	srListOffset         int
	srListSortBy         string
	srListSortOrder      string
	srListIncludeDeleted bool

	srDeleteForce     bool
	srDeletePermanent bool

	srExportOutput string

	srConvertOutput         string
	srConvertSimplifyTiming bool
	srConvertMinDelay       int
	srConvertMaxDelay       int
	srConvertIncludeClient  bool
	srConvertDeduplicate    bool

	srVacuumForce bool
)

var streamRecordingsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all stream recordings",
	RunE: func(cmd *cobra.Command, args []string) error {
		protocol := &srListProtocol
		path := &srListPath
		status := &srListStatus
		limit := &srListLimit
		offset := &srListOffset
		sortBy := &srListSortBy
		sortOrder := &srListSortOrder
		includeDeleted := &srListIncludeDeleted

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

		if jsonOutput {
			return output.JSON(recordings)
		}

		if len(recordings) == 0 {
			fmt.Println("No stream recordings found")
			return nil
		}

		// Table output
		w := output.Table()
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
	},
}

var streamRecordingsShowCmd = &cobra.Command{
	Use:     "show <id>",
	Aliases: []string{"get"},
	Short:   "Show details of a specific stream recording",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]

		rec, err := streamRecordingStore.Get(id)
		if err != nil {
			return fmt.Errorf("recording not found: %w", err)
		}

		if jsonOutput {
			return output.JSON(rec)
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
		switch rec.Protocol { //nolint:exhaustive // only stream protocols have detailed info
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
	},
}

var streamRecordingsDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm"},
	Short:   "Delete a stream recording",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force := &srDeleteForce
		permanent := &srDeletePermanent
		id := args[0]

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
	},
}

var streamRecordingsExportCmd = &cobra.Command{
	Use:   "export <id>",
	Short: "Export a stream recording to JSON format",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		output := &srExportOutput
		id := args[0]

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
	},
}

var streamRecordingsConvertCmd = &cobra.Command{
	Use:   "convert <id>",
	Short: "Convert a stream recording to a mock configuration",
	Long: `Convert a stream recording to a mock configuration.

For WebSocket recordings, creates a scenario config.
For SSE recordings, creates an SSE mock config.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		output := &srConvertOutput
		simplifyTiming := &srConvertSimplifyTiming
		minDelay := &srConvertMinDelay
		maxDelay := &srConvertMaxDelay
		includeClient := &srConvertIncludeClient
		deduplicate := &srConvertDeduplicate

		id := args[0]

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
	},
}

var streamRecordingsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show storage statistics for stream recordings",
	RunE: func(cmd *cobra.Command, args []string) error {
		stats, err := streamRecordingStore.GetStats()
		if err != nil {
			return fmt.Errorf("failed to get stats: %w", err)
		}

		if jsonOutput {
			return output.JSON(stats)
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
	},
}

var streamRecordingsVacuumCmd = &cobra.Command{
	Use:   "vacuum",
	Short: "Permanently remove soft-deleted recordings to free up disk space",
	RunE: func(cmd *cobra.Command, args []string) error {
		force := &srVacuumForce

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
	},
}

var streamRecordingsSessionsCmd = &cobra.Command{
	Use:   "sessions",
	Short: "List active recording sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		sessions := streamRecordingStore.GetActiveSessions()

		if jsonOutput {
			return output.JSON(sessions)
		}

		if len(sessions) == 0 {
			fmt.Println("No active recording sessions")
			return nil
		}

		w := output.Table()
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
	},
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

func init() {
	rootCmd.AddCommand(streamRecordingsCmd)

	streamRecordingsCmd.AddCommand(streamRecordingsListCmd)
	streamRecordingsListCmd.Flags().StringVar(&srListProtocol, "protocol", "", "Filter by protocol (websocket, sse)")
	streamRecordingsListCmd.Flags().StringVar(&srListPath, "path", "", "Filter by path prefix")
	streamRecordingsListCmd.Flags().StringVar(&srListStatus, "status", "", "Filter by status (complete, incomplete, recording)")
	streamRecordingsListCmd.Flags().IntVar(&srListLimit, "limit", 20, "Maximum number of recordings to show (default: 20)")
	streamRecordingsListCmd.Flags().IntVar(&srListOffset, "offset", 0, "Offset for pagination")
	streamRecordingsListCmd.Flags().StringVar(&srListSortBy, "sort", "startTime", "Sort by field: startTime, name, size (default: startTime)")
	streamRecordingsListCmd.Flags().StringVar(&srListSortOrder, "order", "desc", "Sort order: asc, desc (default: desc)")
	streamRecordingsListCmd.Flags().BoolVar(&srListIncludeDeleted, "include-deleted", false, "Include soft-deleted recordings")

	streamRecordingsCmd.AddCommand(streamRecordingsShowCmd)

	streamRecordingsCmd.AddCommand(streamRecordingsDeleteCmd)
	streamRecordingsDeleteCmd.Flags().BoolVarP(&srDeleteForce, "force", "f", false, "Skip confirmation")
	streamRecordingsDeleteCmd.Flags().BoolVar(&srDeletePermanent, "permanent", false, "Permanently delete (not soft-delete)")

	streamRecordingsCmd.AddCommand(streamRecordingsExportCmd)
	streamRecordingsExportCmd.Flags().StringVarP(&srExportOutput, "output", "o", "", "Output file path (default: stdout)")

	streamRecordingsCmd.AddCommand(streamRecordingsConvertCmd)
	streamRecordingsConvertCmd.Flags().StringVarP(&srConvertOutput, "output", "o", "", "Output file path (default: stdout)")
	streamRecordingsConvertCmd.Flags().BoolVar(&srConvertSimplifyTiming, "simplify-timing", false, "Normalize timing to reduce noise")
	streamRecordingsConvertCmd.Flags().IntVar(&srConvertMinDelay, "min-delay", 10, "Minimum delay to preserve in ms (default: 10)")
	streamRecordingsConvertCmd.Flags().IntVar(&srConvertMaxDelay, "max-delay", 5000, "Maximum delay in ms (default: 5000)")
	streamRecordingsConvertCmd.Flags().BoolVar(&srConvertIncludeClient, "include-client", true, "Include client messages as expect steps (default: true)")
	streamRecordingsConvertCmd.Flags().BoolVar(&srConvertDeduplicate, "deduplicate", false, "Remove consecutive duplicate messages")

	streamRecordingsCmd.AddCommand(streamRecordingsStatsCmd)
	streamRecordingsCmd.AddCommand(streamRecordingsVacuumCmd)
	streamRecordingsVacuumCmd.Flags().BoolVarP(&srVacuumForce, "force", "f", false, "Skip confirmation")

	streamRecordingsCmd.AddCommand(streamRecordingsSessionsCmd)
}
