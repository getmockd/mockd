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
	"sync"
	"syscall"
	"time"

	"github.com/getmockd/mockd/internal/cliconfig"
	"github.com/getmockd/mockd/pkg/cli/internal/flags"
	"github.com/getmockd/mockd/pkg/cli/internal/parse"
	"github.com/gorilla/websocket"
)

// RunWebSocket handles the websocket command and its subcommands.
func RunWebSocket(args []string) error {
	if len(args) == 0 {
		printWebSocketUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "connect":
		return runWebSocketConnect(subArgs)
	case "send":
		return runWebSocketSend(subArgs)
	case "listen":
		return runWebSocketListen(subArgs)
	case "status":
		return runWebSocketStatus(subArgs)
	case "help", "--help", "-h":
		printWebSocketUsage()
		return nil
	default:
		return fmt.Errorf("unknown websocket subcommand: %s\n\nRun 'mockd websocket --help' for usage", subcommand)
	}
}

func printWebSocketUsage() {
	fmt.Print(`Usage: mockd websocket <subcommand> [flags]

Interact with WebSocket endpoints for testing.

Subcommands:
  connect   Interactive WebSocket client (REPL mode)
  send      Send a single message and exit
  listen    Stream incoming messages
  status    Show WebSocket handler status from admin API

Run 'mockd websocket <subcommand> --help' for more information.
`)
}

// runWebSocketConnect starts an interactive WebSocket REPL session.
func runWebSocketConnect(args []string) error {
	fs := flag.NewFlagSet("websocket connect", flag.ContinueOnError)

	var headers flags.StringSlice
	fs.Var(&headers, "header", "Custom headers (key:value), repeatable")
	fs.Var(&headers, "H", "Custom headers (shorthand)")

	subprotocol := fs.String("subprotocol", "", "WebSocket subprotocol")
	timeout := fs.Duration("timeout", 30*time.Second, "Connection timeout")
	fs.DurationVar(timeout, "t", 30*time.Second, "Connection timeout (shorthand)")

	jsonOutput := fs.Bool("json", false, "Output messages in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd websocket connect [flags] <url>

Start an interactive WebSocket client session (REPL mode).
Type messages to send, press Enter to send. Ctrl+C to exit.

Arguments:
  url    WebSocket URL (e.g., ws://localhost:4280/ws)

Flags:
  -H, --header       Custom headers (key:value), repeatable
      --subprotocol  WebSocket subprotocol
  -t, --timeout      Connection timeout (default: 30s)
      --json         Output messages in JSON format

Examples:
  # Connect to a WebSocket endpoint
  mockd websocket connect ws://localhost:4280/ws

  # Connect with custom headers
  mockd websocket connect -H "Authorization:Bearer token" ws://localhost:4280/ws

  # Connect with subprotocol
  mockd websocket connect --subprotocol graphql-ws ws://localhost:4280/graphql
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("url is required")
	}

	url := fs.Arg(0)

	// Setup dialer
	dialer := websocket.Dialer{
		HandshakeTimeout: *timeout,
	}

	// Build request header
	requestHeader := http.Header{}
	for _, h := range headers {
		parts := parse.HeaderParts(h)
		if len(parts) == 2 {
			requestHeader.Add(parts[0], parts[1])
		}
	}

	if *subprotocol != "" {
		dialer.Subprotocols = []string{*subprotocol}
	}

	// Connect
	fmt.Printf("Connecting to %s...\n", url)
	conn, resp, err := dialer.Dial(url, requestHeader)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("connection failed: %v (HTTP %d)", err, resp.StatusCode)
		}
		return fmt.Errorf("connection failed: %v", err)
	}
	defer conn.Close()

	if *subprotocol != "" && resp.Header.Get("Sec-WebSocket-Protocol") != "" {
		fmt.Printf("Connected (subprotocol: %s)\n", resp.Header.Get("Sec-WebSocket-Protocol"))
	} else {
		fmt.Println("Connected. Type messages and press Enter to send. Ctrl+C to exit.")
	}

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nDisconnecting...")
		cancel()
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}()

	// Channel for incoming messages
	msgChan := make(chan wsMessage, 100)
	errChan := make(chan error, 1)

	// Goroutine for reading messages
	go func() {
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					errChan <- err
					return
				}
			}
			msgChan <- wsMessage{Type: messageType, Data: message}
		}
	}()

	// Goroutine for reading user input
	inputChan := make(chan string, 10)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case inputChan <- scanner.Text():
			}
		}
	}()

	// Main loop
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errChan:
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				fmt.Println("Connection closed by server")
				return nil
			}
			return fmt.Errorf("read error: %v", err)
		case msg := <-msgChan:
			if *jsonOutput {
				output := map[string]interface{}{
					"type":      messageTypeString(msg.Type),
					"data":      string(msg.Data),
					"timestamp": time.Now().Format(time.RFC3339),
				}
				enc := json.NewEncoder(os.Stdout)
				if err := enc.Encode(output); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to encode output: %v\n", err)
				}
			} else {
				fmt.Printf("< %s\n", string(msg.Data))
			}
		case input := <-inputChan:
			if input == "" {
				continue
			}
			if err := conn.WriteMessage(websocket.TextMessage, []byte(input)); err != nil {
				return fmt.Errorf("send error: %v", err)
			}
			if *jsonOutput {
				output := map[string]interface{}{
					"direction": "sent",
					"type":      "text",
					"data":      input,
					"timestamp": time.Now().Format(time.RFC3339),
				}
				enc := json.NewEncoder(os.Stdout)
				if err := enc.Encode(output); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to encode output: %v\n", err)
				}
			} else {
				fmt.Printf("> %s\n", input)
			}
		}
	}
}

// wsMessage represents a WebSocket message.
type wsMessage struct {
	Type int
	Data []byte
}

// messageTypeString returns a human-readable message type.
func messageTypeString(t int) string {
	switch t {
	case websocket.TextMessage:
		return "text"
	case websocket.BinaryMessage:
		return "binary"
	case websocket.CloseMessage:
		return "close"
	case websocket.PingMessage:
		return "ping"
	case websocket.PongMessage:
		return "pong"
	default:
		return "unknown"
	}
}

// runWebSocketSend sends a single message to a WebSocket endpoint.
func runWebSocketSend(args []string) error {
	fs := flag.NewFlagSet("websocket send", flag.ContinueOnError)

	var headers flags.StringSlice
	fs.Var(&headers, "header", "Custom headers (key:value), repeatable")
	fs.Var(&headers, "H", "Custom headers (shorthand)")

	subprotocol := fs.String("subprotocol", "", "WebSocket subprotocol")
	timeout := fs.Duration("timeout", 30*time.Second, "Connection timeout")
	fs.DurationVar(timeout, "t", 30*time.Second, "Connection timeout (shorthand)")

	jsonOutput := fs.Bool("json", false, "Output result in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd websocket send [flags] <url> <message>

Send a single message to a WebSocket endpoint and exit.

Arguments:
  url      WebSocket URL (e.g., ws://localhost:4280/ws)
  message  Message to send (or @filename for file content)

Flags:
  -H, --header       Custom headers (key:value), repeatable
      --subprotocol  WebSocket subprotocol
  -t, --timeout      Connection timeout (default: 30s)
      --json         Output result in JSON format

Examples:
  # Send a simple message
  mockd websocket send ws://localhost:4280/ws "hello"

  # Send JSON message
  mockd websocket send ws://localhost:4280/ws '{"action":"ping"}'

  # Send with custom headers
  mockd websocket send -H "Authorization:Bearer token" ws://localhost:4280/ws "hello"

  # Send message from file
  mockd websocket send ws://localhost:4280/ws @message.json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("url and message are required")
	}

	url := fs.Arg(0)
	message := fs.Arg(1)

	// Load message from file if prefixed with @
	if len(message) > 0 && message[0] == '@' {
		msgBytes, err := os.ReadFile(message[1:])
		if err != nil {
			return fmt.Errorf("failed to read message file: %w", err)
		}
		message = string(msgBytes)
	}

	// Setup dialer
	dialer := websocket.Dialer{
		HandshakeTimeout: *timeout,
	}

	// Build request header
	requestHeader := http.Header{}
	for _, h := range headers {
		parts := parse.HeaderParts(h)
		if len(parts) == 2 {
			requestHeader.Add(parts[0], parts[1])
		}
	}

	if *subprotocol != "" {
		dialer.Subprotocols = []string{*subprotocol}
	}

	// Connect
	conn, resp, err := dialer.Dial(url, requestHeader)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("connection failed: %v (HTTP %d)", err, resp.StatusCode)
		}
		return fmt.Errorf("connection failed: %v", err)
	}
	defer conn.Close()

	// Send message
	if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
		return fmt.Errorf("send error: %v", err)
	}

	// Close gracefully
	conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

	if *jsonOutput {
		output := map[string]interface{}{
			"success":   true,
			"url":       url,
			"message":   message,
			"timestamp": time.Now().Format(time.RFC3339),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(output)
	}

	fmt.Printf("Sent to %s: %s\n", url, message)
	return nil
}

// runWebSocketListen listens for incoming WebSocket messages.
func runWebSocketListen(args []string) error {
	fs := flag.NewFlagSet("websocket listen", flag.ContinueOnError)

	var headers flags.StringSlice
	fs.Var(&headers, "header", "Custom headers (key:value), repeatable")
	fs.Var(&headers, "H", "Custom headers (shorthand)")

	subprotocol := fs.String("subprotocol", "", "WebSocket subprotocol")
	timeout := fs.Duration("timeout", 30*time.Second, "Connection timeout")
	fs.DurationVar(timeout, "t", 30*time.Second, "Connection timeout (shorthand)")

	count := fs.Int("count", 0, "Number of messages to receive (0 = unlimited)")
	fs.IntVar(count, "n", 0, "Number of messages (shorthand)")

	jsonOutput := fs.Bool("json", false, "Output messages in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd websocket listen [flags] <url>

Listen for incoming WebSocket messages and print them.

Arguments:
  url    WebSocket URL (e.g., ws://localhost:4280/ws)

Flags:
  -H, --header       Custom headers (key:value), repeatable
      --subprotocol  WebSocket subprotocol
  -t, --timeout      Connection timeout (default: 30s)
  -n, --count        Number of messages to receive (0 = unlimited)
      --json         Output messages in JSON format

Examples:
  # Listen to all messages
  mockd websocket listen ws://localhost:4280/ws

  # Listen for 10 messages then exit
  mockd websocket listen -n 10 ws://localhost:4280/ws

  # Listen with JSON output
  mockd websocket listen --json ws://localhost:4280/ws

  # Listen with custom headers
  mockd websocket listen -H "Authorization:Bearer token" ws://localhost:4280/ws
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("url is required")
	}

	url := fs.Arg(0)

	// Setup dialer
	dialer := websocket.Dialer{
		HandshakeTimeout: *timeout,
	}

	// Build request header
	requestHeader := http.Header{}
	for _, h := range headers {
		parts := parse.HeaderParts(h)
		if len(parts) == 2 {
			requestHeader.Add(parts[0], parts[1])
		}
	}

	if *subprotocol != "" {
		dialer.Subprotocols = []string{*subprotocol}
	}

	// Connect
	fmt.Fprintf(os.Stderr, "Connecting to %s...\n", url)
	conn, resp, err := dialer.Dial(url, requestHeader)
	if err != nil {
		if resp != nil {
			return fmt.Errorf("connection failed: %v (HTTP %d)", err, resp.StatusCode)
		}
		return fmt.Errorf("connection failed: %v", err)
	}
	defer conn.Close()

	if *count > 0 {
		fmt.Fprintf(os.Stderr, "Listening for %d messages (Ctrl+C to stop)\n", *count)
	} else {
		fmt.Fprintf(os.Stderr, "Listening for messages (Ctrl+C to stop)\n")
	}

	// Setup context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Use WaitGroup for clean shutdown
	var wg sync.WaitGroup

	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nDisconnecting...")
		cancel()
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	}()

	received := 0
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			messageType, message, err := conn.ReadMessage()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						fmt.Fprintln(os.Stderr, "Connection closed by server")
						return
					}
					fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
					return
				}
			}

			if *jsonOutput {
				output := map[string]interface{}{
					"type":      messageTypeString(messageType),
					"data":      string(message),
					"timestamp": time.Now().Format(time.RFC3339),
					"index":     received,
				}
				enc := json.NewEncoder(os.Stdout)
				if err := enc.Encode(output); err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to encode output: %v\n", err)
				}
			} else {
				fmt.Println(string(message))
			}

			received++
			if *count > 0 && received >= *count {
				fmt.Fprintf(os.Stderr, "Received %d messages\n", received)
				cancel()
				return
			}
		}
	}()

	wg.Wait()
	return nil
}

// runWebSocketStatus shows WebSocket mock status from admin API.
func runWebSocketStatus(args []string) error {
	fs := flag.NewFlagSet("websocket status", flag.ContinueOnError)

	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd websocket status [flags]

Show WebSocket mock status from the admin API.

Flags:
      --admin-url   Admin API base URL (default: http://localhost:4290)
      --json        Output in JSON format

Examples:
  mockd websocket status
  mockd websocket status --json
  mockd websocket status --admin-url http://localhost:9091
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Get WebSocket mocks from admin API
	client := NewAdminClient(*adminURL)
	mocks, err := client.ListMocksByType("websocket")
	if err != nil {
		return fmt.Errorf("failed to get WebSocket status: %s", FormatConnectionError(err))
	}

	if *jsonOutput {
		result := map[string]interface{}{
			"enabled":   len(mocks) > 0,
			"endpoints": mocks,
			"count":     len(mocks),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	// Pretty print status
	if len(mocks) == 0 {
		fmt.Println("WebSocket: no mocks configured")
		return nil
	}

	fmt.Printf("WebSocket: %d mock(s) configured\n", len(mocks))
	for _, m := range mocks {
		path := ""
		if m.WebSocket != nil {
			path = m.WebSocket.Path
		}
		status := "enabled"
		if !m.Enabled {
			status = "disabled"
		}
		fmt.Printf("  - %s (%s) [%s]\n", path, m.ID, status)
	}

	return nil
}
