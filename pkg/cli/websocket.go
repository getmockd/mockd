package cli

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/flags"
	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cli/internal/parse"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
)

// ─── websocket parent command ────────────────────────────────────────────────

var websocketCmd = &cobra.Command{
	Use:     "websocket",
	Aliases: []string{"ws"},
	Short:   "Manage and test WebSocket endpoints",
}

var wsAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new WebSocket mock endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		addMockType = "websocket"
		return runAdd(cmd, args)
	},
}

// ─── websocket connect ───────────────────────────────────────────────────────

var (
	wsConnectHeaders     flags.StringSlice
	wsConnectSubprotocol string
	wsConnectTimeout     time.Duration
)

var wsConnectCmd = &cobra.Command{
	Use:   "connect <url>",
	Short: "Interactive WebSocket client (REPL mode)",
	Long: `Start an interactive WebSocket client session (REPL mode).
Type messages to send, press Enter to send. Ctrl+C to exit.`,
	Example: `  # Connect to a WebSocket endpoint
  mockd websocket connect ws://localhost:4280/ws

  # Connect with custom headers
  mockd websocket connect -H "Authorization:Bearer token" ws://localhost:4280/ws

  # Connect with subprotocol
  mockd websocket connect --subprotocol graphql-ws ws://localhost:4280/graphql`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		// Setup dialer
		dialer := websocket.Dialer{
			HandshakeTimeout: wsConnectTimeout,
		}

		// Build request header
		requestHeader := http.Header{}
		for _, h := range wsConnectHeaders {
			parts := parse.HeaderParts(h)
			if len(parts) == 2 {
				requestHeader.Add(parts[0], parts[1])
			}
		}

		if wsConnectSubprotocol != "" {
			dialer.Subprotocols = []string{wsConnectSubprotocol}
		}

		// Connect
		fmt.Printf("Connecting to %s...\n", url)
		conn, resp, err := dialer.Dial(url, requestHeader)
		if resp != nil && resp.Body != nil {
			defer func() { _ = resp.Body.Close() }()
		}
		if err != nil {
			if resp != nil {
				return fmt.Errorf("connection failed: %w (HTTP %d)", err, resp.StatusCode)
			}
			return fmt.Errorf("connection failed: %w", err)
		}
		defer func() { _ = conn.Close() }()

		if wsConnectSubprotocol != "" && resp.Header.Get("Sec-WebSocket-Protocol") != "" {
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
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
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
				return fmt.Errorf("read error: %w", err)
			case msg := <-msgChan:
				if jsonOutput {
					m := map[string]interface{}{
						"type":      messageTypeString(msg.Type),
						"data":      string(msg.Data),
						"timestamp": time.Now().Format(time.RFC3339),
					}
					if err := output.JSONCompact(m); err != nil {
						output.Warn("failed to encode output: %v", err)
					}
				} else {
					fmt.Printf("< %s\n", string(msg.Data))
				}
			case input := <-inputChan:
				if input == "" {
					continue
				}
				if err := conn.WriteMessage(websocket.TextMessage, []byte(input)); err != nil {
					return fmt.Errorf("send error: %w", err)
				}
				if jsonOutput {
					sent := map[string]interface{}{
						"direction": "sent",
						"type":      "text",
						"data":      input,
						"timestamp": time.Now().Format(time.RFC3339),
					}
					if err := output.JSONCompact(sent); err != nil {
						output.Warn("failed to encode output: %v", err)
					}
				} else {
					fmt.Printf("> %s\n", input)
				}
			}
		}
	},
}

// ─── websocket send ──────────────────────────────────────────────────────────

var (
	wsSendHeaders     flags.StringSlice
	wsSendSubprotocol string
	wsSendTimeout     time.Duration
)

var wsSendCmd = &cobra.Command{
	Use:   "send <url> <message>",
	Short: "Send a single message and exit",
	Long: `Send a single message to a WebSocket endpoint and exit.

If the message starts with @, the content is read from the named file.`,
	Example: `  # Send a simple message
  mockd websocket send ws://localhost:4280/ws "hello"

  # Send JSON message
  mockd websocket send ws://localhost:4280/ws '{"action":"ping"}'

  # Send with custom headers
  mockd websocket send -H "Authorization:Bearer token" ws://localhost:4280/ws "hello"

  # Send message from file
  mockd websocket send ws://localhost:4280/ws @message.json`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]
		message := args[1]

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
			HandshakeTimeout: wsSendTimeout,
		}

		// Build request header
		requestHeader := http.Header{}
		for _, h := range wsSendHeaders {
			parts := parse.HeaderParts(h)
			if len(parts) == 2 {
				requestHeader.Add(parts[0], parts[1])
			}
		}

		if wsSendSubprotocol != "" {
			dialer.Subprotocols = []string{wsSendSubprotocol}
		}

		// Connect
		conn, resp, err := dialer.Dial(url, requestHeader)
		if resp != nil && resp.Body != nil {
			defer func() { _ = resp.Body.Close() }()
		}
		if err != nil {
			if resp != nil {
				return fmt.Errorf("connection failed: %w (HTTP %d)", err, resp.StatusCode)
			}
			return fmt.Errorf("connection failed: %w", err)
		}
		defer func() { _ = conn.Close() }()

		// Send message
		if err := conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
			return fmt.Errorf("send error: %w", err)
		}

		// Close gracefully
		_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

		if jsonOutput {
			result := map[string]interface{}{
				"success":   true,
				"url":       url,
				"message":   message,
				"timestamp": time.Now().Format(time.RFC3339),
			}
			return output.JSON(result)
		}

		fmt.Printf("Sent to %s: %s\n", url, message)
		return nil
	},
}

// ─── websocket listen ────────────────────────────────────────────────────────

var (
	wsListenHeaders     flags.StringSlice
	wsListenSubprotocol string
	wsListenTimeout     time.Duration
	wsListenCount       int
)

var wsListenCmd = &cobra.Command{
	Use:   "listen <url>",
	Short: "Stream incoming messages",
	Long: `Listen for incoming WebSocket messages and print them.

Messages are printed to stdout (one per line). Use --json for structured output.
Use --count to limit the number of messages received.`,
	Example: `  # Listen to all messages
  mockd websocket listen ws://localhost:4280/ws

  # Listen for 10 messages then exit
  mockd websocket listen -n 10 ws://localhost:4280/ws

  # Listen with JSON output
  mockd websocket listen --json ws://localhost:4280/ws

  # Listen with custom headers
  mockd websocket listen -H "Authorization:Bearer token" ws://localhost:4280/ws`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]

		// Setup dialer
		dialer := websocket.Dialer{
			HandshakeTimeout: wsListenTimeout,
		}

		// Build request header
		requestHeader := http.Header{}
		for _, h := range wsListenHeaders {
			parts := parse.HeaderParts(h)
			if len(parts) == 2 {
				requestHeader.Add(parts[0], parts[1])
			}
		}

		if wsListenSubprotocol != "" {
			dialer.Subprotocols = []string{wsListenSubprotocol}
		}

		// Connect
		fmt.Fprintf(os.Stderr, "Connecting to %s...\n", url)
		conn, resp, err := dialer.Dial(url, requestHeader)
		if resp != nil && resp.Body != nil {
			defer func() { _ = resp.Body.Close() }()
		}
		if err != nil {
			if resp != nil {
				return fmt.Errorf("connection failed: %w (HTTP %d)", err, resp.StatusCode)
			}
			return fmt.Errorf("connection failed: %w", err)
		}
		defer func() { _ = conn.Close() }()

		if wsListenCount > 0 {
			fmt.Fprintf(os.Stderr, "Listening for %d messages (Ctrl+C to stop)\n", wsListenCount)
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
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
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

				if jsonOutput {
					m := map[string]interface{}{
						"type":      messageTypeString(messageType),
						"data":      string(message),
						"timestamp": time.Now().Format(time.RFC3339),
						"index":     received,
					}
					if err := output.JSONCompact(m); err != nil {
						output.Warn("failed to encode output: %v", err)
					}
				} else {
					fmt.Println(string(message))
				}

				received++
				if wsListenCount > 0 && received >= wsListenCount {
					fmt.Fprintf(os.Stderr, "Received %d messages\n", received)
					cancel()
					return
				}
			}
		}()

		wg.Wait()
		return nil
	},
}

// ─── websocket status ────────────────────────────────────────────────────────

var wsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show WebSocket handler status from admin API",
	Example: `  mockd websocket status
  mockd websocket status --json`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Uses root persistent adminURL
		client := NewAdminClientWithAuth(adminURL)
		mocks, err := client.ListMocksByType("websocket")
		if err != nil {
			return fmt.Errorf("failed to get WebSocket status: %s", FormatConnectionError(err))
		}

		if jsonOutput {
			result := map[string]interface{}{
				"enabled":   len(mocks) > 0,
				"endpoints": mocks,
				"count":     len(mocks),
			}
			return output.JSON(result)
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
			if m.Enabled != nil && !*m.Enabled {
				status = "disabled"
			}
			fmt.Printf("  - %s (%s) [%s]\n", path, m.ID, status)
		}

		return nil
	},
}

// ─── init ────────────────────────────────────────────────────────────────────

func init() {
	rootCmd.AddCommand(websocketCmd)

	// websocket add flags
	wsAddCmd.Flags().StringVar(&addPath, "path", "", "WebSocket endpoint path (e.g., /ws)")
	wsAddCmd.Flags().StringVar(&addMessage, "message", "", "Default response message (JSON)")
	wsAddCmd.Flags().BoolVar(&addEcho, "echo", false, "Enable echo mode")
	wsAddCmd.Flags().StringVar(&addName, "name", "", "Mock display name")
	websocketCmd.AddCommand(wsAddCmd)

	// list/get/delete generic aliases
	websocketCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List WebSocket mocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			listMockType = "websocket"
			return runList(cmd, args)
		},
	})
	websocketCmd.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Get details of a WebSocket mock",
		RunE:  runGet,
	})
	websocketCmd.AddCommand(&cobra.Command{
		Use:   "delete",
		Short: "Delete a WebSocket mock",
		RunE:  runDelete,
	})

	// websocket connect flags
	wsConnectCmd.Flags().VarP(&wsConnectHeaders, "header", "H", "Custom headers (key:value), repeatable")
	wsConnectCmd.Flags().StringVar(&wsConnectSubprotocol, "subprotocol", "", "WebSocket subprotocol")
	wsConnectCmd.Flags().DurationVarP(&wsConnectTimeout, "timeout", "t", 30*time.Second, "Connection timeout")
	websocketCmd.AddCommand(wsConnectCmd)

	// websocket send flags
	wsSendCmd.Flags().VarP(&wsSendHeaders, "header", "H", "Custom headers (key:value), repeatable")
	wsSendCmd.Flags().StringVar(&wsSendSubprotocol, "subprotocol", "", "WebSocket subprotocol")
	wsSendCmd.Flags().DurationVarP(&wsSendTimeout, "timeout", "t", 30*time.Second, "Connection timeout")
	websocketCmd.AddCommand(wsSendCmd)

	// websocket listen flags
	wsListenCmd.Flags().VarP(&wsListenHeaders, "header", "H", "Custom headers (key:value), repeatable")
	wsListenCmd.Flags().StringVar(&wsListenSubprotocol, "subprotocol", "", "WebSocket subprotocol")
	wsListenCmd.Flags().DurationVarP(&wsListenTimeout, "timeout", "t", 30*time.Second, "Connection timeout")
	wsListenCmd.Flags().IntVarP(&wsListenCount, "count", "n", 0, "Number of messages to receive (0 = unlimited)")
	websocketCmd.AddCommand(wsListenCmd)

	// websocket status
	websocketCmd.AddCommand(wsStatusCmd)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

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
