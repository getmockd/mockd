package cli

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/spf13/cobra"
)

// ─── websocket connection management ────────────────────────────────────────

var wsConnectionsCmd = &cobra.Command{
	Use:   "connections",
	Short: "Manage active WebSocket connections",
	Long:  `List, inspect, close, or send messages to active WebSocket connections.`,
	RunE:  runWSConnectionsList,
}

var wsConnectionsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List active WebSocket connections",
	Example: `  mockd websocket connections list
  mockd websocket connections list --json`,
	Args: cobra.NoArgs,
	RunE: runWSConnectionsList,
}

func runWSConnectionsList(cmd *cobra.Command, args []string) error {
	client := NewAdminClientWithAuth(adminURL)
	result, err := client.ListWebSocketConnections()
	if err != nil {
		return fmt.Errorf("failed to list WebSocket connections: %s", FormatConnectionError(err))
	}

	printList(result, func() {
		if len(result.Connections) == 0 {
			fmt.Println("No active WebSocket connections")
			return
		}
		tw := output.Table()
		fmt.Fprintf(tw, "ID\tPATH\tMOCK ID\tCONNECTED\tMSG SENT\tMSG RECV\tSTATUS\n")
		for _, c := range result.Connections {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\t%d\t%s\n",
				c.ID, c.Path, c.MockID,
				formatDuration(time.Since(c.ConnectedAt)),
				c.MessagesSent, c.MessagesRecv, c.Status)
		}
		_ = tw.Flush()
		fmt.Printf("\nTotal: %d connection(s)\n", len(result.Connections))
	})
	return nil
}

var wsConnectionsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get details of a WebSocket connection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		conn, err := client.GetWebSocketConnection(args[0])
		if err != nil {
			return fmt.Errorf("failed to get WebSocket connection: %s", FormatConnectionError(err))
		}
		printResult(conn, func() {
			fmt.Printf("WebSocket Connection: %s\n", conn.ID)
			fmt.Printf("  Path:          %s\n", conn.Path)
			fmt.Printf("  Mock ID:       %s\n", conn.MockID)
			fmt.Printf("  Connected:     %s (%s ago)\n", conn.ConnectedAt.Format(time.RFC3339), formatDuration(time.Since(conn.ConnectedAt)))
			fmt.Printf("  Messages Sent: %d\n", conn.MessagesSent)
			fmt.Printf("  Messages Recv: %d\n", conn.MessagesRecv)
			fmt.Printf("  Status:        %s\n", conn.Status)
		})
		return nil
	},
}

var wsConnectionsCloseCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Close a WebSocket connection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		if err := client.CloseWebSocketConnection(args[0]); err != nil {
			return fmt.Errorf("failed to close WebSocket connection: %s", FormatConnectionError(err))
		}
		printResult(map[string]interface{}{"id": args[0], "closed": true}, func() {
			fmt.Printf("Closed WebSocket connection: %s\n", args[0])
		})
		return nil
	},
}

var wsConnectionsSendBinary bool

var wsConnectionsSendCmd = &cobra.Command{
	Use:   "send <id> <message>",
	Short: "Send a message to a WebSocket connection",
	Long: `Send a text or binary message to an active WebSocket connection.
Use --binary to send base64-encoded binary data.`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		message := args[1]

		if wsConnectionsSendBinary {
			// Validate base64
			if _, err := base64.StdEncoding.DecodeString(message); err != nil {
				return fmt.Errorf("invalid base64 data: %w", err)
			}
		}

		client := NewAdminClientWithAuth(adminURL)
		if err := client.SendWebSocketMessage(id, message, wsConnectionsSendBinary); err != nil {
			return fmt.Errorf("failed to send message: %s", FormatConnectionError(err))
		}

		msgType := "text"
		if wsConnectionsSendBinary {
			msgType = "binary"
		}
		printResult(map[string]interface{}{"id": id, "sent": true, "type": msgType}, func() {
			fmt.Printf("Sent %s message to connection %s\n", msgType, id)
		})
		return nil
	},
}

var wsStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show WebSocket statistics",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		stats, err := client.GetWebSocketStats()
		if err != nil {
			return fmt.Errorf("failed to get WebSocket stats: %s", FormatConnectionError(err))
		}

		printResult(stats, func() {
			fmt.Println("WebSocket Statistics")
			fmt.Printf("  Active Connections:  %d\n", stats.ActiveConnections)
			fmt.Printf("  Total Connections:   %d\n", stats.TotalConnections)
			fmt.Printf("  Total Messages Sent: %d\n", stats.TotalMessagesSent)
			fmt.Printf("  Total Messages Recv: %d\n", stats.TotalMessagesRecv)
			if len(stats.ConnectionsByMock) > 0 {
				fmt.Println("  Connections by Mock:")
				for mockID, count := range stats.ConnectionsByMock {
					fmt.Printf("    %s: %d\n", mockID, count)
				}
			}
		})
		return nil
	},
}

func init() {
	// connections subgroup
	wsConnectionsCmd.AddCommand(wsConnectionsListCmd)
	wsConnectionsCmd.AddCommand(wsConnectionsGetCmd)
	wsConnectionsCmd.AddCommand(wsConnectionsCloseCmd)

	wsConnectionsSendCmd.Flags().BoolVar(&wsConnectionsSendBinary, "binary", false, "Send base64-encoded binary message")
	wsConnectionsCmd.AddCommand(wsConnectionsSendCmd)

	websocketCmd.AddCommand(wsConnectionsCmd)

	// stats as top-level websocket subcommand
	websocketCmd.AddCommand(wsStatsCmd)
}
