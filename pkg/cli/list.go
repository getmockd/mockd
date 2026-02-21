package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/spf13/cobra"
)

var (
	listConfigFile string
	listMockType   string
	listNoTruncate bool
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configured mocks",
	Long:  `List all configured mocks.`,
	Example: `  # List all mocks from running server
  mockd list

  # List with full IDs (useful for copy-paste into delete)
  mockd list -w

  # List mocks from config file (no server needed)
  mockd list --config mockd.yaml

  # List only WebSocket mocks
  mockd list --type websocket

  # List as JSON
  mockd list --json

  # List from remote server
  mockd list --admin-url http://remote:4290`,
	RunE: runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.Flags().StringVarP(&listConfigFile, "config", "c", "", "List mocks from config file (no server needed)")
	listCmd.Flags().StringVarP(&listMockType, "type", "t", "", "Filter by type: http, websocket, graphql, grpc, mqtt, soap")
	listCmd.Flags().BoolVarP(&listNoTruncate, "no-truncate", "w", false, "Show full IDs and paths without truncation")
}

func runList(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected arguments: %v", args)
	}

	configFile := listConfigFile
	mockType := listMockType
	noTruncate := listNoTruncate

	var mocks []*mock.Mock

	// Load mocks from config file or admin API
	if configFile != "" {
		// Load from config file directly (no server needed)
		collection, err := config.LoadFromFile(configFile)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
		mocks = collection.Mocks
	} else {
		// Query running server via admin API
		client := NewAdminClientWithAuth(adminURL)
		var err error
		mocks, err = client.ListMocks()
		if err != nil {
			return fmt.Errorf("%s", FormatConnectionError(err))
		}
	}

	// Filter by type if specified
	if mockType != "" {
		filterType := mock.Type(strings.ToLower(mockType))
		filtered := make([]*mock.Mock, 0)
		for _, m := range mocks {
			if m.Type == filterType {
				filtered = append(filtered, m)
			}
		}
		mocks = filtered
	}

	// Output result
	if jsonOutput {
		return outputMocksJSON(mocks)
	}

	return outputMocksTable(mocks, noTruncate)
}

// outputMocksJSON outputs mocks in JSON format.
func outputMocksJSON(mocks []*mock.Mock) error {
	type mockSummary struct {
		ID      string `json:"id"`
		Name    string `json:"name,omitempty"`
		Type    string `json:"type"`
		Path    string `json:"path,omitempty"`
		Method  string `json:"method,omitempty"`
		Status  int    `json:"status,omitempty"`
		Enabled bool   `json:"enabled"`
	}
	summaries := make([]mockSummary, 0, len(mocks))
	for _, m := range mocks {
		summary := mockSummary{
			ID:      m.ID,
			Name:    m.Name,
			Type:    string(m.Type),
			Enabled: m.Enabled == nil || *m.Enabled,
		}
		// Extract path/method/status based on type
		path, method, status := extractMockDetails(m)
		summary.Path = path
		summary.Method = method
		summary.Status = status
		summaries = append(summaries, summary)
	}
	return output.JSON(summaries)
}

// outputMocksTable outputs mocks in table format.
func outputMocksTable(mocks []*mock.Mock, noTruncate bool) error {
	if len(mocks) == 0 {
		fmt.Println("No mocks configured")
		return nil
	}

	w := output.Table()
	_, _ = fmt.Fprintln(w, "ID\tTYPE\tPATH\tMETHOD\tSTATUS\tENABLED")

	for _, m := range mocks {
		path, method, status := extractMockDetails(m)

		// Truncate long IDs and paths unless --no-truncate is set
		id := m.ID
		if !noTruncate {
			if len(id) > 20 {
				id = id[:17] + "..."
			}
			if len(path) > 25 {
				path = path[:22] + "..."
			}
		}

		// Format status - show "-" for non-HTTP
		statusStr := "-"
		if status > 0 {
			statusStr = strconv.Itoa(status)
		}

		// Format type
		mockType := string(m.Type)
		if mockType == "" {
			mockType = "http"
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%t\n",
			id, mockType, path, method, statusStr, m.Enabled == nil || *m.Enabled)
	}

	return w.Flush()
}

// extractMockDetails extracts path, method, and status from a mock based on its type.
func extractMockDetails(m *mock.Mock) (path, method string, status int) {
	switch m.Type { //nolint:exhaustive // OAuth mocks don't have path/method/status details
	case mock.TypeHTTP, "":
		if m.HTTP != nil {
			if m.HTTP.Matcher != nil {
				path = m.HTTP.Matcher.Path
				if path == "" {
					path = m.HTTP.Matcher.PathPattern
				}
				method = m.HTTP.Matcher.Method
			}
			if m.HTTP.Response != nil {
				status = m.HTTP.Response.StatusCode
			}
		}
	case mock.TypeWebSocket:
		if m.WebSocket != nil {
			path = m.WebSocket.Path
			method = "WS"
		}
	case mock.TypeGraphQL:
		if m.GraphQL != nil {
			path = m.GraphQL.Path
			method = "GQL"
		}
	case mock.TypeGRPC:
		if m.GRPC != nil {
			if m.GRPC.Port > 0 {
				path = fmt.Sprintf(":%d", m.GRPC.Port)
			}
			method = "gRPC"
		}
	case mock.TypeMQTT:
		if m.MQTT != nil {
			path = fmt.Sprintf(":%d", m.MQTT.Port)
			method = "MQTT"
		}
	case mock.TypeSOAP:
		if m.SOAP != nil {
			path = m.SOAP.Path
			method = "SOAP"
		}
	}
	return
}
