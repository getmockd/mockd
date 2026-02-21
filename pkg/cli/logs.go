package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/requestlog"
	"github.com/spf13/cobra"
)

var (
	logsFollow    bool
	logsLines     int
	logsLogDir    string
	logsRequests  bool
	logsProtocol  string
	logsMethod    string
	logsPath      string
	logsMatched   bool
	logsUnmatched bool
	logsVerbose   bool
	logsClear     bool
)

var logsCmd = &cobra.Command{
	Use:   "logs [service-name]",
	Short: "View logs from mockd services running in detached mode",
	Long: `View logs from mockd services running in detached mode.

By default, shows daemon logs from ~/.mockd/logs/. Use --requests to show
request logs from the admin API instead.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := ""
		if len(args) > 0 {
			serviceName = args[0]
		}

		if logsRequests {
			return runRequestLogs(runRequestLogsOptions{
				protocol:   logsProtocol,
				method:     logsMethod,
				path:       logsPath,
				matched:    logsMatched,
				unmatched:  logsUnmatched,
				limit:      logsLines,
				verbose:    logsVerbose,
				clear:      logsClear,
				follow:     logsFollow,
				adminURL:   cliconfig.ResolveAdminURL(adminURL),
				jsonOutput: jsonOutput,
			})
		}

		return runDaemonLogs(runDaemonLogsOptions{
			logDir:      logsLogDir,
			serviceName: serviceName,
			lines:       logsLines,
			follow:      logsFollow,
			jsonOutput:  jsonOutput,
		})
	},
}

func init() {
	// Daemon logs flags
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "Follow log output in real-time (like tail -f)")
	logsCmd.Flags().IntVarP(&logsLines, "lines", "n", 100, "Number of lines to show")
	logsCmd.Flags().StringVar(&logsLogDir, "log-dir", defaultLogsPath(), "Path to logs directory")

	// Request logs flags
	logsCmd.Flags().BoolVar(&logsRequests, "requests", false, "Show request logs from admin API instead of daemon logs")
	logsCmd.Flags().StringVar(&logsProtocol, "protocol", "", "Filter by protocol (http, grpc, mqtt, soap, graphql, websocket, sse) [requests mode]")
	logsCmd.Flags().StringVarP(&logsMethod, "method", "m", "", "Filter by HTTP method [requests mode]")
	logsCmd.Flags().StringVarP(&logsPath, "path", "p", "", "Filter by path (substring match) [requests mode]")
	logsCmd.Flags().BoolVar(&logsMatched, "matched", false, "Show only matched requests [requests mode]")
	logsCmd.Flags().BoolVar(&logsUnmatched, "unmatched", false, "Show only unmatched requests [requests mode]")
	logsCmd.Flags().BoolVar(&logsVerbose, "verbose", false, "Show headers and body [requests mode]")
	logsCmd.Flags().BoolVar(&logsClear, "clear", false, "Clear all logs [requests mode]")

	rootCmd.AddCommand(logsCmd)
}

// defaultLogsPath returns the default path for daemon logs.
func defaultLogsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/mockd/logs"
	}
	return filepath.Join(home, ".mockd", "logs")
}

// runDaemonLogsOptions contains options for viewing daemon logs.
type runDaemonLogsOptions struct {
	logDir      string
	serviceName string
	lines       int
	follow      bool
	jsonOutput  bool
}

// runDaemonLogs reads and displays logs from the daemon log files.
func runDaemonLogs(opts runDaemonLogsOptions) error {
	// Check if log directory exists
	if _, err := os.Stat(opts.logDir); os.IsNotExist(err) {
		fmt.Printf("No logs found at %s\n", opts.logDir)
		fmt.Println("Logs are created when running 'mockd up -d' in detached mode.")
		return nil
	}

	// Find log files
	logFiles, err := findLogFiles(opts.logDir, opts.serviceName)
	if err != nil {
		return fmt.Errorf("finding log files: %w", err)
	}

	if len(logFiles) == 0 {
		if opts.serviceName != "" {
			fmt.Printf("No logs found for service '%s' in %s\n", opts.serviceName, opts.logDir)
		} else {
			fmt.Printf("No log files found in %s\n", opts.logDir)
		}
		return nil
	}

	// Follow mode
	if opts.follow {
		return followLogs(logFiles, opts.jsonOutput)
	}

	// Read and display logs
	return displayLogs(logFiles, opts.lines, opts.jsonOutput)
}

// findLogFiles finds log files in the directory, optionally filtered by service name.
func findLogFiles(logDir, serviceName string) ([]string, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".log") {
			continue
		}

		// Filter by service name if provided
		if serviceName != "" {
			// Normalize service name: admin/local -> admin-local, engine/default -> engine-default
			normalizedService := strings.ReplaceAll(serviceName, "/", "-")
			baseName := strings.TrimSuffix(name, ".log")

			// Check for exact match or if file is mockd.log (combined log)
			if baseName != normalizedService && baseName != "mockd" {
				continue
			}
		}

		files = append(files, filepath.Join(logDir, name))
	}

	// Sort by modification time (newest first for display, but we'll read oldest first for chronological order)
	sort.Slice(files, func(i, j int) bool {
		infoI, _ := os.Stat(files[i])
		infoJ, _ := os.Stat(files[j])
		if infoI == nil || infoJ == nil {
			return false
		}
		return infoI.ModTime().Before(infoJ.ModTime())
	})

	return files, nil
}

// displayLogs reads and displays the last N lines from log files.
func displayLogs(logFiles []string, numLines int, jsonOutput bool) error {
	// Collect all log lines with their timestamps
	type logLine struct {
		timestamp time.Time
		line      string
		file      string
	}

	var allLines []logLine

	for _, file := range logFiles {
		lines, err := readLastLines(file, numLines)
		if err != nil {
			output.Warn("could not read %s: %v", file, err)
			continue
		}

		for _, line := range lines {
			ts := parseLogTimestamp(line)
			allLines = append(allLines, logLine{
				timestamp: ts,
				line:      line,
				file:      filepath.Base(file),
			})
		}
	}

	// Sort by timestamp
	sort.Slice(allLines, func(i, j int) bool {
		return allLines[i].timestamp.Before(allLines[j].timestamp)
	})

	// Take last N lines
	if len(allLines) > numLines {
		allLines = allLines[len(allLines)-numLines:]
	}

	// Output
	if jsonOutput {
		type jsonLine struct {
			Timestamp string `json:"timestamp"`
			File      string `json:"file"`
			Message   string `json:"message"`
		}
		lines := make([]jsonLine, 0, len(allLines))
		for _, l := range allLines {
			lines = append(lines, jsonLine{
				Timestamp: l.timestamp.Format(time.RFC3339),
				File:      l.file,
				Message:   l.line,
			})
		}
		return output.JSON(lines)
	}

	// Print lines
	for _, l := range allLines {
		fmt.Println(l.line)
	}

	return nil
}

// readLastLines reads the last N lines from a file.
func readLastLines(filePath string, n int) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	// Read all lines (for simplicity; could optimize with reverse reading for large files)
	var lines []string
	scanner := bufio.NewScanner(file)
	// Increase buffer size for long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Return last N lines
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	return lines, nil
}

// parseLogTimestamp attempts to parse a timestamp from the beginning of a log line.
func parseLogTimestamp(line string) time.Time {
	// Common log formats:
	// 2024-01-15T10:30:00Z ...
	// 2024-01-15 10:30:00 ...
	// [2024-01-15T10:30:00Z] ...

	line = strings.TrimPrefix(line, "[")

	// Try RFC3339
	if len(line) >= 20 {
		if t, err := time.Parse(time.RFC3339, line[:20]); err == nil {
			return t
		}
		// Try with longer RFC3339 (with milliseconds)
		if len(line) >= 24 {
			if t, err := time.Parse(time.RFC3339Nano, line[:min(30, len(line))]); err == nil {
				return t
			}
		}
	}

	// Try common format: 2024-01-15 10:30:00
	if len(line) >= 19 {
		if t, err := time.Parse("2006-01-02 15:04:05", line[:19]); err == nil {
			return t
		}
	}

	// Return zero time if parsing fails
	return time.Time{}
}

// followLogs follows log files in real-time (like tail -f).
func followLogs(logFiles []string, jsonOutput bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nStopping log stream...")
		cancel()
	}()

	fmt.Println("Following logs (press Ctrl+C to stop)...")
	fmt.Println()

	// Track file positions
	type fileTracker struct {
		path   string
		offset int64
	}

	trackers := make([]*fileTracker, len(logFiles))
	for i, file := range logFiles {
		info, err := os.Stat(file)
		if err != nil {
			trackers[i] = &fileTracker{path: file, offset: 0}
		} else {
			trackers[i] = &fileTracker{path: file, offset: info.Size()}
		}
	}

	// Poll for new content
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for _, tracker := range trackers {
				newLines, newOffset, err := readNewLines(tracker.path, tracker.offset)
				if err != nil {
					continue
				}

				tracker.offset = newOffset

				for _, line := range newLines {
					if jsonOutput {
						jsonLine := struct {
							Timestamp string `json:"timestamp"`
							File      string `json:"file"`
							Message   string `json:"message"`
						}{
							Timestamp: time.Now().Format(time.RFC3339),
							File:      filepath.Base(tracker.path),
							Message:   line,
						}
						data, _ := json.Marshal(jsonLine)
						fmt.Println(string(data))
					} else {
						fmt.Println(line)
					}
				}
			}
		}
	}
}

// readNewLines reads new lines from a file starting at the given offset.
func readNewLines(filePath string, offset int64) ([]string, int64, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, offset, err
	}
	defer func() { _ = file.Close() }()

	// Get current file size
	info, err := file.Stat()
	if err != nil {
		return nil, offset, err
	}

	// If file was truncated, start from beginning
	if info.Size() < offset {
		offset = 0
	}

	// No new content
	if info.Size() == offset {
		return nil, offset, nil
	}

	// Seek to offset
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}

	// Read new content
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, info.Size(), scanner.Err()
}

// runRequestLogsOptions contains options for viewing request logs.
type runRequestLogsOptions struct {
	protocol   string
	method     string
	path       string
	matched    bool
	unmatched  bool
	limit      int
	verbose    bool
	clear      bool
	follow     bool
	adminURL   string
	jsonOutput bool
}

// runRequestLogs shows request logs from the admin API.
func runRequestLogs(opts runRequestLogsOptions) error {
	// Handle clear command
	if opts.clear {
		client := NewAdminClientWithAuth(opts.adminURL)
		count, err := client.ClearLogs()
		if err != nil {
			return fmt.Errorf("%s", FormatConnectionError(err))
		}
		fmt.Printf("Cleared %d log entries\n", count)
		return nil
	}

	// Handle follow mode (streaming)
	if opts.follow {
		return streamRequestLogs(opts.adminURL, opts.jsonOutput, opts.verbose)
	}

	client := NewAdminClientWithAuth(opts.adminURL)

	// Build filter
	filter := &LogFilter{
		Protocol: opts.protocol,
		Method:   opts.method,
		Path:     opts.path,
		Limit:    opts.limit,
	}

	// Get logs
	result, err := client.GetLogs(filter)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Filter matched/unmatched locally
	requests := result.Requests
	if opts.matched || opts.unmatched {
		filtered := make([]*requestlog.Entry, 0)
		for _, req := range result.Requests {
			hasMatch := req.MatchedMockID != ""
			if (opts.matched && hasMatch) || (opts.unmatched && !hasMatch) {
				filtered = append(filtered, req)
			}
		}
		requests = filtered
	}

	// JSON output
	if opts.jsonOutput {
		return output.JSON(requests)
	}

	// No logs message
	if len(requests) == 0 {
		fmt.Println("No request logs")
		return nil
	}

	// Verbose output
	if opts.verbose {
		return printVerboseLogs(requests)
	}

	// Table output
	return printTableLogs(requests)
}

func printTableLogs(requests []*requestlog.Entry) error {
	w := output.Table()
	_, _ = fmt.Fprintln(w, "TIMESTAMP\tPROTOCOL\tMETHOD\tPATH\tMATCHED\tDURATION")

	for _, req := range requests {
		timestamp := req.Timestamp.Format("2006-01-02 15:04:05")
		protocol := req.Protocol
		if protocol == "" {
			protocol = "http"
		}
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
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%dms\n",
			timestamp, protocol, req.Method, path, matched, req.DurationMs)
	}

	return w.Flush()
}

// streamRequestLogs connects to the SSE endpoint and streams logs in real-time.
func streamRequestLogs(adminURL string, jsonOutput, verbose bool) error {
	// Set up context with cancellation for Ctrl+C
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nStopping log stream...")
		cancel()
	}()

	// Load API key for authentication
	apiKey, _ := LoadAPIKeyFromFile()

	// Create HTTP request for SSE endpoint
	streamURL := adminURL + "/requests/stream"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	if apiKey != "" {
		req.Header.Set(APIKeyHeader, apiKey)
	}

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil // Context cancelled, clean exit
		}
		return fmt.Errorf("%s", FormatConnectionError(&APIError{
			StatusCode: 0,
			ErrorCode:  "connection_error",
			Message:    fmt.Sprintf("cannot connect to admin API at %s: %v", adminURL, err),
		}))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	fmt.Println("Streaming logs (press Ctrl+C to stop)...")
	fmt.Println()

	// Read SSE events
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and event type lines
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}

		// Parse data lines
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Skip connection message
			if strings.Contains(data, "Connected to request stream") {
				continue
			}

			// Parse the log entry
			var entry requestlog.Entry
			if err := json.Unmarshal([]byte(data), &entry); err != nil {
				continue // Skip invalid entries
			}

			// Output based on format
			switch {
			case jsonOutput:
				fmt.Println(data)
			case verbose:
				printVerboseEntry(&entry)
			default:
				printTableEntry(&entry)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			return nil // Context cancelled, clean exit
		}
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}

// printTableEntry prints a single log entry in table format.
func printTableEntry(req *requestlog.Entry) {
	timestamp := req.Timestamp.Format("2006-01-02 15:04:05")
	protocol := req.Protocol
	if protocol == "" {
		protocol = "http"
	}
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
	fmt.Printf("%s  %s  %-6s  %-25s  %-15s  %dms\n",
		timestamp, protocol, req.Method, path, matched, req.DurationMs)
}

// printVerboseEntry prints a single log entry in verbose format.
func printVerboseEntry(req *requestlog.Entry) {
	timestamp := req.Timestamp.Format("2006-01-02 15:04:05")
	protocol := req.Protocol
	if protocol == "" {
		protocol = "http"
	}
	matched := req.MatchedMockID
	if matched == "" {
		matched = "(none)"
	}

	fmt.Printf("[%s] [%s] %s %s -> %d (%dms)\n",
		timestamp, protocol, req.Method, req.Path, req.ResponseStatus, req.DurationMs)
	fmt.Printf("  Matched: %s\n", matched)

	// Show protocol-specific metadata
	switch protocol {
	case "grpc":
		if req.GRPC != nil {
			fmt.Printf("  gRPC Service: %s, Method: %s, Status: %s\n",
				req.GRPC.Service, req.GRPC.MethodName, req.GRPC.StatusCode)
		}
	case "mqtt":
		if req.MQTT != nil {
			fmt.Printf("  MQTT Topic: %s, ClientID: %s, QoS: %d\n",
				req.MQTT.Topic, req.MQTT.ClientID, req.MQTT.QoS)
		}
	case "soap":
		if req.SOAP != nil {
			fmt.Printf("  SOAP Operation: %s, Version: %s\n",
				req.SOAP.Operation, req.SOAP.SOAPVersion)
		}
	case "graphql":
		if req.GraphQL != nil {
			fmt.Printf("  GraphQL Type: %s, Operation: %s\n",
				req.GraphQL.OperationType, req.GraphQL.OperationName)
		}
	case "websocket":
		if req.WebSocket != nil {
			fmt.Printf("  WebSocket Connection: %s, Direction: %s\n",
				req.WebSocket.ConnectionID, req.WebSocket.Direction)
		}
	case "sse":
		if req.SSE != nil {
			fmt.Printf("  SSE Connection: %s, EventType: %s\n",
				req.SSE.ConnectionID, req.SSE.EventType)
		}
	}

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

func printVerboseLogs(requests []*requestlog.Entry) error {
	for _, req := range requests {
		printVerboseEntry(req)
	}
	return nil
}
