package cli

import (
	"fmt"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/spf13/cobra"
)

// ─── SSE parent command ─────────────────────────────────────────────────────

var sseCmd = &cobra.Command{
	Use:   "sse",
	Short: "Manage SSE (Server-Sent Events) connections",
}

var sseAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new SSE mock endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		addMockType = "sse"
		return runAdd(cmd, args)
	},
}

// ─── SSE connection management ──────────────────────────────────────────────

var sseConnectionsCmd = &cobra.Command{
	Use:   "connections",
	Short: "Manage active SSE connections",
	Long:  `List, inspect, or close active SSE connections.`,
	RunE:  runSSEConnectionsList,
}

var sseConnectionsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List active SSE connections",
	Example: `  mockd sse connections list
  mockd sse connections list --json`,
	Args: cobra.NoArgs,
	RunE: runSSEConnectionsList,
}

func runSSEConnectionsList(cmd *cobra.Command, args []string) error {
	client := NewAdminClientWithAuth(adminURL)
	result, err := client.ListSSEConnections()
	if err != nil {
		return fmt.Errorf("failed to list SSE connections: %s", FormatConnectionError(err))
	}

	printList(result, func() {
		if len(result.Connections) == 0 {
			fmt.Println("No active SSE connections")
			return
		}
		tw := output.Table()
		_, _ = fmt.Fprintf(tw, "ID\tPATH\tMOCK ID\tCLIENT IP\tCONNECTED\tEVENTS\tSTATUS\n")
		for _, c := range result.Connections {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
				c.ID, c.Path, c.MockID, c.ClientIP,
				formatDuration(time.Since(c.ConnectedAt)),
				c.EventsSent, c.Status)
		}
		_ = tw.Flush()
		fmt.Printf("\nTotal: %d connection(s)\n", len(result.Connections))
	})
	return nil
}

var sseConnectionsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get details of an SSE connection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		conn, err := client.GetSSEConnection(args[0])
		if err != nil {
			return fmt.Errorf("failed to get SSE connection: %s", FormatConnectionError(err))
		}
		printResult(conn, func() {
			fmt.Printf("SSE Connection: %s\n", conn.ID)
			fmt.Printf("  Path:        %s\n", conn.Path)
			fmt.Printf("  Mock ID:     %s\n", conn.MockID)
			fmt.Printf("  Client IP:   %s\n", conn.ClientIP)
			if conn.UserAgent != "" {
				fmt.Printf("  User Agent:  %s\n", conn.UserAgent)
			}
			fmt.Printf("  Connected:   %s (%s ago)\n", conn.ConnectedAt.Format(time.RFC3339), formatDuration(time.Since(conn.ConnectedAt)))
			fmt.Printf("  Events Sent: %d\n", conn.EventsSent)
			fmt.Printf("  Bytes Sent:  %d\n", conn.BytesSent)
			fmt.Printf("  Status:      %s\n", conn.Status)
		})
		return nil
	},
}

var sseConnectionsCloseCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Close an SSE connection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		if err := client.CloseSSEConnection(args[0]); err != nil {
			return fmt.Errorf("failed to close SSE connection: %s", FormatConnectionError(err))
		}
		printResult(map[string]interface{}{"id": args[0], "closed": true}, func() {
			fmt.Printf("Closed SSE connection: %s\n", args[0])
		})
		return nil
	},
}

var sseStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show SSE statistics",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		stats, err := client.GetSSEStats()
		if err != nil {
			return fmt.Errorf("failed to get SSE stats: %s", FormatConnectionError(err))
		}

		printResult(stats, func() {
			fmt.Println("SSE Statistics")
			fmt.Printf("  Active Connections: %d\n", stats.ActiveConnections)
			fmt.Printf("  Total Connections:  %d\n", stats.TotalConnections)
			fmt.Printf("  Total Events Sent:  %d\n", stats.TotalEventsSent)
			fmt.Printf("  Total Bytes Sent:   %d\n", stats.TotalBytesSent)
			fmt.Printf("  Connection Errors:  %d\n", stats.ConnectionErrors)
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

// ─── init ───────────────────────────────────────────────────────────────────

func init() {
	rootCmd.AddCommand(sseCmd)

	// sse add
	sseAddCmd.Flags().StringVar(&addPath, "path", "", "SSE endpoint path (e.g., /events)")
	sseAddCmd.Flags().StringVar(&addName, "name", "", "Mock display name")
	sseCmd.AddCommand(sseAddCmd)

	// list/get/delete generic aliases
	sseCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List SSE mocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			listMockType = "sse"
			return runList(cmd, args)
		},
	})
	sseCmd.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Get details of an SSE mock",
		RunE:  runGet,
	})
	sseCmd.AddCommand(&cobra.Command{
		Use:   "delete",
		Short: "Delete an SSE mock",
		RunE:  runDelete,
	})

	// connections subgroup
	sseConnectionsCmd.AddCommand(sseConnectionsListCmd)
	sseConnectionsCmd.AddCommand(sseConnectionsGetCmd)
	sseConnectionsCmd.AddCommand(sseConnectionsCloseCmd)
	sseCmd.AddCommand(sseConnectionsCmd)

	// stats
	sseCmd.AddCommand(sseStatsCmd)
}
