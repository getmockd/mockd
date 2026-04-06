package cli

import (
	"fmt"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/spf13/cobra"
)

// ─── MQTT connection management ─────────────────────────────────────────────

var mqttConnectionsCmd = &cobra.Command{
	Use:   "connections",
	Short: "Manage active MQTT client connections",
	Long:  `List, inspect, or disconnect active MQTT client connections.`,
	RunE:  runMQTTConnectionsList,
}

var mqttConnectionsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List active MQTT client connections",
	Example: `  mockd mqtt connections list
  mockd mqtt connections list --json`,
	Args: cobra.NoArgs,
	RunE: runMQTTConnectionsList,
}

func runMQTTConnectionsList(cmd *cobra.Command, args []string) error {
	client := NewAdminClientWithAuth(adminURL)
	result, err := client.ListMQTTConnections()
	if err != nil {
		return fmt.Errorf("failed to list MQTT connections: %s", FormatConnectionError(err))
	}

	printList(result, func() {
		if len(result.Connections) == 0 {
			fmt.Println("No active MQTT connections")
			return
		}
		tw := output.Table()
		fmt.Fprintf(tw, "ID\tREMOTE ADDR\tUSERNAME\tSUBSCRIPTIONS\tCONNECTED\tSTATUS\n")
		for _, c := range result.Connections {
			subs := fmt.Sprintf("%d topic(s)", len(c.Subscriptions))
			username := c.Username
			if username == "" {
				username = "-"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
				c.ID, c.RemoteAddr, username, subs,
				formatDuration(time.Since(c.ConnectedAt)),
				c.Status)
		}
		_ = tw.Flush()
		fmt.Printf("\nTotal: %d connection(s)\n", result.Count)
	})
	return nil
}

var mqttConnectionsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get details of an MQTT client connection",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		conn, err := client.GetMQTTConnection(args[0])
		if err != nil {
			return fmt.Errorf("failed to get MQTT connection: %s", FormatConnectionError(err))
		}
		printResult(conn, func() {
			fmt.Printf("MQTT Connection: %s\n", conn.ID)
			fmt.Printf("  Broker ID:        %s\n", conn.BrokerID)
			fmt.Printf("  Remote Addr:      %s\n", conn.RemoteAddr)
			if conn.Username != "" {
				fmt.Printf("  Username:         %s\n", conn.Username)
			}
			fmt.Printf("  Protocol Version: %d\n", conn.ProtocolVersion)
			fmt.Printf("  Connected:        %s (%s ago)\n", conn.ConnectedAt.Format(time.RFC3339), formatDuration(time.Since(conn.ConnectedAt)))
			fmt.Printf("  Subscriptions:    %s\n", formatStringSlice(conn.Subscriptions))
			fmt.Printf("  Status:           %s\n", conn.Status)
		})
		return nil
	},
}

var mqttConnectionsCloseCmd = &cobra.Command{
	Use:   "close <id>",
	Short: "Disconnect an MQTT client",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		if err := client.CloseMQTTConnection(args[0]); err != nil {
			return fmt.Errorf("failed to close MQTT connection: %s", FormatConnectionError(err))
		}
		printResult(map[string]interface{}{"id": args[0], "closed": true}, func() {
			fmt.Printf("Disconnected MQTT client: %s\n", args[0])
		})
		return nil
	},
}

var mqttStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show MQTT broker statistics",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		client := NewAdminClientWithAuth(adminURL)
		stats, err := client.GetMQTTStats()
		if err != nil {
			return fmt.Errorf("failed to get MQTT stats: %s", FormatConnectionError(err))
		}

		printResult(stats, func() {
			fmt.Println("MQTT Broker Statistics")
			fmt.Printf("  Connected Clients:   %d\n", stats.ConnectedClients)
			fmt.Printf("  Total Subscriptions: %d\n", stats.TotalSubscriptions)
			fmt.Printf("  Topic Count:         %d\n", stats.TopicCount)
			fmt.Printf("  Port:                %d\n", stats.Port)
			fmt.Printf("  TLS Enabled:         %v\n", stats.TLSEnabled)
			fmt.Printf("  Auth Enabled:        %v\n", stats.AuthEnabled)
			if len(stats.SubscriptionsByClient) > 0 {
				fmt.Println("  Subscriptions by Client:")
				for clientID, count := range stats.SubscriptionsByClient {
					fmt.Printf("    %s: %d\n", clientID, count)
				}
			}
		})
		return nil
	},
}

func init() {
	// connections subgroup
	mqttConnectionsCmd.AddCommand(mqttConnectionsListCmd)
	mqttConnectionsCmd.AddCommand(mqttConnectionsGetCmd)
	mqttConnectionsCmd.AddCommand(mqttConnectionsCloseCmd)
	mqttCmd.AddCommand(mqttConnectionsCmd)

	// stats
	mqttCmd.AddCommand(mqttStatsCmd)
}
