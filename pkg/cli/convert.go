package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/getmockd/mockd/pkg/recording"
	"github.com/spf13/cobra"
)

var (
	convertSessionName    string
	convertFile           string
	convertRecordingsDir  string
	convertIncludeHosts   string
	convertPathFilter     string
	convertMethod         string
	convertStatus         string
	convertSmartMatch     bool
	convertDuplicates     string
	convertIncludeHeaders bool
	convertCheckSensitive bool
	convertOutput         string
)

var convertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert recorded API traffic to mock definitions",
	Long: `Convert recorded API traffic to mock definitions.
Reads recordings from disk (written by 'mockd proxy start') and produces
mock configuration that can be imported with 'mockd import'.

Examples:
  # Convert latest session
  mockd convert

  # Convert named session with smart matching
  mockd convert --session stripe-api --smart-match

  # Convert only specific hosts and methods
  mockd convert --include-hosts "api.stripe.com" --method GET,POST

  # Convert a specific file
  mockd convert --file ./my-recordings/rec_abc123.json

  # Pipe directly to import
  mockd convert --session my-api --smart-match | mockd import`,
	RunE: func(cmd *cobra.Command, args []string) error {
		file := &convertFile
		sessionName := &convertSessionName
		recordingsDir := &convertRecordingsDir
		includeHosts := &convertIncludeHosts
		statusFilter := &convertStatus
		includeHeaders := &convertIncludeHeaders
		duplicates := &convertDuplicates
		smartMatch := &convertSmartMatch
		pathFilter := &convertPathFilter
		methodFilter := &convertMethod
		checkSensitive := &convertCheckSensitive
		output := &convertOutput

		// Load recordings from disk
		recordings, err := loadRecordingsFromFlags(*file, *sessionName, *recordingsDir)
		if err != nil {
			return err
		}

		if len(recordings) == 0 {
			return errors.New("no recordings found")
		}

		// Filter by host if specified
		if *includeHosts != "" {
			hostPatterns := splitPatterns(*includeHosts)
			recordings = filterByHosts(recordings, hostPatterns)
			if len(recordings) == 0 {
				return errors.New("no recordings match the host filter")
			}
		}

		// Build conversion options
		statusCodes, statusRange := recording.ParseStatusFilter(*statusFilter)

		opts := recording.SessionConvertOptions{
			ConvertOptions: recording.ConvertOptions{
				IncludeHeaders: *includeHeaders,
				Deduplicate:    *duplicates != "all",
				SmartMatch:     *smartMatch,
			},
			Filter: recording.FilterOptions{
				PathPattern: *pathFilter,
				Methods:     recording.ParseMethodFilter(*methodFilter),
				StatusCodes: statusCodes,
				StatusRange: statusRange,
			},
			Duplicates: *duplicates,
		}

		if opts.Duplicates == "" {
			opts.Duplicates = "first"
		}

		// Convert
		result := recording.ConvertRecordingsWithOptions(recordings, opts)

		// Show sensitive data warnings
		if *checkSensitive && len(result.Warnings) > 0 {
			fmt.Fprintf(os.Stderr, "Warning: Found %d potential sensitive data issues:\n", len(result.Warnings))

			warningsByType := make(map[string][]recording.SensitiveDataWarning)
			for _, w := range result.Warnings {
				warningsByType[w.Type] = append(warningsByType[w.Type], w)
			}

			for typ, warnings := range warningsByType {
				fmt.Fprintf(os.Stderr, "  %s (%d):\n", cases.Title(language.English).String(typ), len(warnings))
				shown := 0
				for _, w := range warnings {
					if shown >= 3 {
						fmt.Fprintf(os.Stderr, "    ... and %d more\n", len(warnings)-3)
						break
					}
					fmt.Fprintf(os.Stderr, "    - %s\n", w.Message)
					shown++
				}
			}
			fmt.Fprintln(os.Stderr)
		}

		// Show stats
		fmt.Fprintf(os.Stderr, "Processed %d recordings", result.Total)
		if result.Filtered > 0 {
			fmt.Fprintf(os.Stderr, " (filtered out %d)", result.Filtered)
		}
		fmt.Fprintf(os.Stderr, ", generated %d mocks\n", len(result.Mocks))

		return outputConversionResult(result, *output)
	},
}

func init() {
	rootCmd.AddCommand(convertCmd)

	convertCmd.Flags().StringVarP(&convertSessionName, "session", "s", "", "Session name or directory (default: latest)")
	convertCmd.Flags().StringVarP(&convertFile, "file", "f", "", "Path to a recording file or directory")
	convertCmd.Flags().StringVar(&convertRecordingsDir, "recordings-dir", "", "Base recordings directory override")
	convertCmd.Flags().StringVar(&convertIncludeHosts, "include-hosts", "", "Comma-separated host patterns to include")

	convertCmd.Flags().StringVar(&convertPathFilter, "path-filter", "", "Glob pattern to filter paths (e.g., /api/*)")
	convertCmd.Flags().StringVar(&convertMethod, "method", "", "Comma-separated HTTP methods (e.g., GET,POST)")
	convertCmd.Flags().StringVar(&convertStatus, "status", "", "Status code filter (e.g., 2xx, 200,201)")
	convertCmd.Flags().BoolVar(&convertSmartMatch, "smart-match", false, "Convert dynamic path segments like /users/123 to /users/{id}")
	convertCmd.Flags().StringVar(&convertDuplicates, "duplicates", "first", "Duplicate handling strategy: first, last, all")
	convertCmd.Flags().BoolVar(&convertIncludeHeaders, "include-headers", false, "Include request headers in mock matchers")
	convertCmd.Flags().BoolVar(&convertCheckSensitive, "check-sensitive", true, "Check for sensitive data and show warnings")

	convertCmd.Flags().StringVarP(&convertOutput, "output", "o", "", "Output file path (default: stdout)")
}

// loadRecordingsFromFlags resolves the recording source from CLI flags.
func loadRecordingsFromFlags(file, sessionName, recordingsDir string) ([]*recording.Recording, error) {
	// --file takes precedence: load from explicit path
	if file != "" {
		info, err := os.Stat(file)
		if err != nil {
			return nil, fmt.Errorf("cannot access %s: %w", file, err)
		}
		if info.IsDir() {
			return recording.LoadFromDir(file)
		}
		return recording.LoadFromFile(file)
	}

	// --session or default to latest
	sessionDir, err := recording.ResolveSessionDir(recordingsDir, sessionName)
	if err != nil {
		if sessionName == "" || sessionName == "latest" {
			return nil, errors.New("no recordings found. Run 'mockd proxy start' to capture traffic first")
		}
		return nil, err
	}

	return recording.LoadFromDir(sessionDir)
}

// filterByHosts filters recordings to only include requests matching host patterns.
func filterByHosts(recordings []*recording.Recording, patterns []string) []*recording.Recording {
	var filtered []*recording.Recording
	for _, r := range recordings {
		for _, pattern := range patterns {
			if matchHost(r.Request.Host, pattern) {
				filtered = append(filtered, r)
				break
			}
		}
	}
	return filtered
}

// matchHost checks if a host matches a glob pattern (supports * wildcards).
func matchHost(host, pattern string) bool {
	matched, _ := filepath.Match(pattern, host)
	return matched
}

// configEnvelope wraps mock configurations in the format accepted by
// mockd's config import endpoint (POST /config) and the 'mockd import -f mockd' command.
type configEnvelope struct {
	Version string      `json:"version"`
	Mocks   interface{} `json:"mocks"`
}

func outputConversionResult(result *recording.ConversionResult, output string) error {
	envelope := configEnvelope{
		Version: "1.0",
		Mocks:   result.Mocks,
	}

	mockOutput, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal mocks: %w", err)
	}

	if output == "" {
		fmt.Println(string(mockOutput))
	} else {
		if err := os.WriteFile(output, mockOutput, 0644); err != nil {
			return fmt.Errorf("failed to write mocks: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Output written to: %s\n", output)
	}

	return nil
}
