package cli

import (
	"fmt"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/spf13/cobra"
)

// ─── gRPC connection management ─────────────────────────────────────────────

var grpcConnectionsCmd = &cobra.Command{
	Use:   "connections",
	Short: "Manage active gRPC streaming connections",
	Long:  `List, inspect, or cancel active gRPC streaming RPC connections.`,
	RunE:  runGRPCConnectionsList,
}

var grpcConnectionsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List active gRPC streams",
	Example: `  mockd grpc connections list
  mockd grpc connections list --json`,
	Args: cobra.NoArgs,
	RunE: runGRPCConnectionsList,
}

func runGRPCConnectionsList(cmd *cobra.Command, args []string) error {
	client := NewAdminClientWithAuth(adminURL)
	result, err := client.ListGRPCStreams()
	if err != nil {
		return fmt.Errorf("failed to list gRPC streams: %s", FormatConnectionError(err))
	}

	printList(result, func() {
		if len(result.Streams) == 0 {
			fmt.Println("No active gRPC streams")
			return
		}
		tw := output.Table()
		_, _ = fmt.Fprintf(tw, "ID\tMETHOD\tSTREAM TYPE\tCLIENT\tCONNECTED\tMSG SENT\tMSG RECV\n")
		for _, s := range result.Streams {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
				s.ID, s.Method, s.StreamType, s.ClientAddr,
				formatDuration(time.Since(s.ConnectedAt)),
				s.MessagesSent, s.MessagesRecv)
		}
		_ = tw.Flush()
		fmt.Printf("\nTotal: %d stream(s)\n", len(result.Streams))
	})
	return nil
}

var grpcConnectionsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get details of a gRPC stream",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		stream, err := client.GetGRPCStream(args[0])
		if err != nil {
			return fmt.Errorf("failed to get gRPC stream: %s", FormatConnectionError(err))
		}
		printResult(stream, func() {
			fmt.Printf("gRPC Stream: %s\n", stream.ID)
			fmt.Printf("  Method:        %s\n", stream.Method)
			fmt.Printf("  Stream Type:   %s\n", stream.StreamType)
			fmt.Printf("  Client Addr:   %s\n", stream.ClientAddr)
			fmt.Printf("  Connected:     %s (%s ago)\n", stream.ConnectedAt.Format(time.RFC3339), formatDuration(time.Since(stream.ConnectedAt)))
			fmt.Printf("  Messages Sent: %d\n", stream.MessagesSent)
			fmt.Printf("  Messages Recv: %d\n", stream.MessagesRecv)
		})
		return nil
	},
}

var grpcConnectionsCloseCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Cancel a gRPC stream",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		if err := client.CloseGRPCStream(args[0]); err != nil {
			return fmt.Errorf("failed to cancel gRPC stream: %s", FormatConnectionError(err))
		}
		printResult(map[string]interface{}{"id": args[0], "cancelled": true}, func() {
			fmt.Printf("Cancelled gRPC stream: %s\n", args[0])
		})
		return nil
	},
}

//nolint:dupl // intentionally parallel structure with other protocol stats commands
var grpcStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show gRPC statistics",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		stats, err := client.GetGRPCStats()
		if err != nil {
			return fmt.Errorf("failed to get gRPC stats: %s", FormatConnectionError(err))
		}

		printResult(stats, func() {
			fmt.Println("gRPC Statistics")
			fmt.Printf("  Active Streams:      %d\n", stats.ActiveStreams)
			fmt.Printf("  Total Streams:       %d\n", stats.TotalStreams)
			fmt.Printf("  Total RPCs:          %d\n", stats.TotalRPCs)
			fmt.Printf("  Total Messages Sent: %d\n", stats.TotalMessagesSent)
			fmt.Printf("  Total Messages Recv: %d\n", stats.TotalMessagesRecv)
			if len(stats.StreamsByMethod) > 0 {
				fmt.Println("  Streams by Method:")
				for method, count := range stats.StreamsByMethod {
					fmt.Printf("    %s: %d\n", method, count)
				}
			}
		})
		return nil
	},
}

func init() {
	// connections subgroup
	grpcConnectionsCmd.AddCommand(grpcConnectionsListCmd)
	grpcConnectionsCmd.AddCommand(grpcConnectionsGetCmd)
	grpcConnectionsCmd.AddCommand(grpcConnectionsCloseCmd)
	grpcCmd.AddCommand(grpcConnectionsCmd)

	// stats
	grpcCmd.AddCommand(grpcStatsCmd)
}
