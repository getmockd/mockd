package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"text/tabwriter"

	"github.com/getmockd/mockd/internal/cliconfig"
	"github.com/getmockd/mockd/pkg/requestlog"
)

// RunLogs handles the logs command.
func RunLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)

	protocol := fs.String("protocol", "", "Filter by protocol (http, grpc, mqtt, soap, graphql, websocket, sse)")
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
	follow := fs.Bool("follow", false, "Stream logs in real-time (like tail -f)")
	fs.BoolVar(follow, "f", false, "Stream logs in real-time (shorthand)")
	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd logs [flags]

View request logs.

Flags:
      --protocol    Filter by protocol (http, grpc, mqtt, soap, graphql, websocket, sse)
  -m, --method      Filter by HTTP method
  -p, --path        Filter by path (substring match)
      --matched     Show only matched requests
      --unmatched   Show only unmatched requests
  -n, --limit       Number of entries to show (default: 20)
      --verbose     Show headers and body
      --clear       Clear all logs
  -f, --follow      Stream logs in real-time (like tail -f)
      --admin-url   Admin API base URL (default: http://localhost:4290)
      --json        Output in JSON format

Examples:
  # Show recent logs
  mockd logs

  # Show last 50 entries
  mockd logs -n 50

  # Filter by method
  mockd logs -m POST

  # Filter by protocol
  mockd logs --protocol grpc

  # Show verbose output
  mockd logs --verbose

  # Stream logs in real-time
  mockd logs --follow

  # Clear logs
  mockd logs --clear
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Handle clear command
	if *clear {
		client := NewAdminClient(*adminURL)
		count, err := client.ClearLogs()
		if err != nil {
			return fmt.Errorf("%s", FormatConnectionError(err))
		}
		fmt.Printf("Cleared %d log entries\n", count)
		return nil
	}

	// Handle follow mode (streaming)
	if *follow {
		return streamLogs(*adminURL, *jsonOutput, *verbose)
	}

	client := NewAdminClient(*adminURL)

	// Build filter
	filter := &LogFilter{
		Protocol: *protocol,
		Method:   *method,
		Path:     *path,
		Limit:    *limit,
	}

	// Get logs
	result, err := client.GetLogs(filter)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Filter matched/unmatched locally
	requests := result.Requests
	if *matched || *unmatched {
		filtered := make([]*requestlog.Entry, 0)
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

func printTableLogs(requests []*requestlog.Entry) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIMESTAMP\tPROTOCOL\tMETHOD\tPATH\tMATCHED\tDURATION")

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
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%dms\n",
			timestamp, protocol, req.Method, path, matched, req.DurationMs)
	}

	return w.Flush()
}

// streamLogs connects to the SSE endpoint and streams logs in real-time.
func streamLogs(adminURL string, jsonOutput, verbose bool) error {
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

	// Create HTTP request for SSE endpoint
	streamURL := adminURL + "/requests/stream"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")

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
	defer resp.Body.Close()

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
			if jsonOutput {
				fmt.Println(data)
			} else if verbose {
				printVerboseEntry(&entry)
			} else {
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

	fmt.Printf("[%s] [%s] %s %s → %d (%dms)\n",
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
		timestamp := req.Timestamp.Format("2006-01-02 15:04:05")
		protocol := req.Protocol
		if protocol == "" {
			protocol = "http"
		}
		matched := req.MatchedMockID
		if matched == "" {
			matched = "(none)"
		}

		fmt.Printf("[%s] [%s] %s %s → %d (%dms)\n",
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
	return nil
}
