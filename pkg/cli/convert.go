package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/recording"
)

// RunConvert handles the convert command and its subcommands.
func RunConvert(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)

	// Recording conversion flags
	recordingID := fs.String("recording", "", "Convert a single recording by ID")
	sessionID := fs.String("session", "", "Convert all recordings from a session")
	pathFilter := fs.String("path-filter", "", "Glob pattern to filter paths (e.g., /api/*)")
	methodFilter := fs.String("method", "", "Comma-separated HTTP methods (e.g., GET,POST)")
	statusFilter := fs.String("status", "", "Status code filter (e.g., 2xx, 200,201)")
	smartMatch := fs.Bool("smart-match", false, "Convert dynamic path segments to parameters")
	duplicates := fs.String("duplicates", "first", "Duplicate handling: first, last, all")
	includeHeaders := fs.Bool("include-headers", false, "Include request headers in matchers")
	output := fs.String("output", "", "Output file path (default: stdout)")
	fs.StringVar(output, "o", "", "Output file path (shorthand)")
	checkSensitive := fs.Bool("check-sensitive", true, "Check for sensitive data and warn")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd convert [flags]

Convert recorded API traffic to mock definitions.

Flags:
      --recording       Convert a single recording by ID
      --session         Convert all recordings from a session (use "latest" for most recent)
      --path-filter     Glob pattern to filter paths (e.g., /api/*)
      --method          Comma-separated HTTP methods (e.g., GET,POST)
      --status          Status code filter (e.g., 2xx, 200,201)
      --smart-match     Convert dynamic path segments like /users/123 to /users/{id}
      --duplicates      Duplicate handling strategy: first, last, all (default: first)
      --include-headers Include request headers in mock matchers
      --check-sensitive Check for sensitive data and show warnings (default: true)
  -o, --output          Output file path (default: stdout)

Examples:
  # Convert a single recording
  mockd convert --recording abc123

  # Convert latest session with smart matching
  mockd convert --session latest --smart-match

  # Convert session filtering only GET requests to /api/*
  mockd convert --session my-session --path-filter '/api/*' --method GET

  # Convert and save to file
  mockd convert --session latest -o mocks.json

  # Convert only successful responses
  mockd convert --session latest --status 2xx
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Check if proxy is running with recordings
	if proxyServer.store == nil {
		return errors.New("no recordings available (proxy not running)")
	}

	// Determine what to convert
	if *recordingID != "" {
		return convertSingleRecording(*recordingID, *smartMatch, *includeHeaders, *checkSensitive, *output)
	}

	if *sessionID != "" {
		return convertSessionRecordings(*sessionID, convertSessionFlags{
			pathFilter:     *pathFilter,
			methodFilter:   *methodFilter,
			statusFilter:   *statusFilter,
			smartMatch:     *smartMatch,
			duplicates:     *duplicates,
			includeHeaders: *includeHeaders,
			checkSensitive: *checkSensitive,
			output:         *output,
		})
	}

	// Default: convert all recordings
	return convertAllRecordings(convertSessionFlags{
		pathFilter:     *pathFilter,
		methodFilter:   *methodFilter,
		statusFilter:   *statusFilter,
		smartMatch:     *smartMatch,
		duplicates:     *duplicates,
		includeHeaders: *includeHeaders,
		checkSensitive: *checkSensitive,
		output:         *output,
	})
}

type convertSessionFlags struct {
	pathFilter     string
	methodFilter   string
	statusFilter   string
	smartMatch     bool
	duplicates     string
	includeHeaders bool
	checkSensitive bool
	output         string
}

func convertSingleRecording(id string, smartMatch, includeHeaders, checkSensitive bool, output string) error {
	rec := proxyServer.store.GetRecording(id)
	if rec == nil {
		return fmt.Errorf("recording not found: %s", id)
	}

	opts := recording.ConvertOptions{
		IncludeHeaders: includeHeaders,
		SmartMatch:     smartMatch,
	}

	// Check for sensitive data
	if checkSensitive {
		warnings := recording.CheckSensitiveData(rec)
		if len(warnings) > 0 {
			fmt.Fprintf(os.Stderr, "Warning: Found %d potential sensitive data issues:\n", len(warnings))
			for _, w := range warnings {
				fmt.Fprintf(os.Stderr, "  - [%s] %s: %s\n", w.Location, w.Type, w.Message)
			}
			fmt.Fprintln(os.Stderr)
		}
	}

	mock := recording.ToMock(rec, opts)

	// Apply smart matching
	if smartMatch && mock.HTTP != nil && mock.HTTP.Matcher != nil {
		originalPath := mock.HTTP.Matcher.Path
		mock.HTTP.Matcher.Path = recording.SmartPathMatcher(mock.HTTP.Matcher.Path)
		if originalPath != mock.HTTP.Matcher.Path {
			fmt.Fprintf(os.Stderr, "Smart match: %s -> %s\n", originalPath, mock.HTTP.Matcher.Path)
		}
	}

	return outputMockConfigs([]*config.MockConfiguration{mock}, output)
}

func convertSessionRecordings(sessionID string, flags convertSessionFlags) error {
	var session *recording.Session

	if sessionID == "latest" {
		session = proxyServer.store.ActiveSession()
		if session == nil {
			sessions := proxyServer.store.ListSessions()
			if len(sessions) > 0 {
				session = sessions[len(sessions)-1]
			}
		}
	} else {
		session = proxyServer.store.GetSession(sessionID)
	}

	if session == nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	return convertRecordingsWithFlags(session.Recordings(), flags)
}

func convertAllRecordings(flags convertSessionFlags) error {
	recordings, _ := proxyServer.store.ListRecordings(recording.RecordingFilter{})
	if len(recordings) == 0 {
		return errors.New("no recordings to convert")
	}

	return convertRecordingsWithFlags(recordings, flags)
}

func convertRecordingsWithFlags(recordings []*recording.Recording, flags convertSessionFlags) error {
	statusCodes, statusRange := recording.ParseStatusFilter(flags.statusFilter)

	opts := recording.SessionConvertOptions{
		ConvertOptions: recording.ConvertOptions{
			IncludeHeaders: flags.includeHeaders,
			Deduplicate:    flags.duplicates != "all",
			SmartMatch:     flags.smartMatch,
		},
		Filter: recording.FilterOptions{
			PathPattern: flags.pathFilter,
			Methods:     recording.ParseMethodFilter(flags.methodFilter),
			StatusCodes: statusCodes,
			StatusRange: statusRange,
		},
		Duplicates: flags.duplicates,
	}

	if opts.Duplicates == "" {
		opts.Duplicates = "first"
	}

	result := recording.ConvertRecordingsWithOptions(recordings, opts)

	// Show warnings
	if flags.checkSensitive && len(result.Warnings) > 0 {
		fmt.Fprintf(os.Stderr, "Warning: Found %d potential sensitive data issues:\n", len(result.Warnings))

		// Group warnings by type
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

	return outputConversionResult(result, flags.output)
}

func outputMockConfigs(mocks []*config.MockConfiguration, output string) error {
	mockOutput, err := json.MarshalIndent(mocks, "", "  ")
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

func outputConversionResult(result *recording.ConversionResult, output string) error {
	mockOutput, err := json.MarshalIndent(result.Mocks, "", "  ")
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
