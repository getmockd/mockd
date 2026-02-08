package cli

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// RunList handles the list command.
func RunList(args []string) error {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)

	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	configFile := fs.String("config", "", "List mocks from config file (no server needed)")
	fs.StringVar(configFile, "c", "", "List mocks from config file (shorthand)")
	mockType := fs.String("type", "", "Filter by type: http, websocket, graphql, grpc, mqtt, soap")
	fs.StringVar(mockType, "t", "", "Filter by type (shorthand)")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd list [flags]

List all configured mocks.

Flags:
  -c, --config     List mocks from config file (no server needed)
  -t, --type       Filter by type: http, websocket, graphql, grpc, mqtt, soap
      --admin-url  Admin API base URL (default: http://localhost:4290)
      --json       Output in JSON format

Examples:
  # List all mocks from running server
  mockd list

  # List mocks from config file (no server needed)
  mockd list --config mockd.yaml

  # List only WebSocket mocks
  mockd list --type websocket

  # List as JSON
  mockd list --json

  # List from remote server
  mockd list --admin-url http://remote:4290
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	var mocks []*mock.Mock

	// Load mocks from config file or admin API
	if *configFile != "" {
		// Load from config file directly (no server needed)
		collection, err := config.LoadFromFile(*configFile)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
		mocks = collection.Mocks
	} else {
		// Query running server via admin API
		client := NewAdminClientWithAuth(*adminURL)
		var err error
		mocks, err = client.ListMocks()
		if err != nil {
			return fmt.Errorf("%s", FormatConnectionError(err))
		}
	}

	// Filter by type if specified
	if *mockType != "" {
		filterType := mock.Type(strings.ToLower(*mockType))
		filtered := make([]*mock.Mock, 0)
		for _, m := range mocks {
			if m.Type == filterType {
				filtered = append(filtered, m)
			}
		}
		mocks = filtered
	}

	// Output result
	if *jsonOutput {
		return outputMocksJSON(mocks)
	}

	return outputMocksTable(mocks)
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
func outputMocksTable(mocks []*mock.Mock) error {
	if len(mocks) == 0 {
		fmt.Println("No mocks configured")
		return nil
	}

	w := output.Table()
	_, _ = fmt.Fprintln(w, "ID\tTYPE\tPATH\tMETHOD\tSTATUS\tENABLED")

	for _, m := range mocks {
		path, method, status := extractMockDetails(m)

		// Truncate long IDs and paths
		id := m.ID
		if len(id) > 20 {
			id = id[:17] + "..."
		}
		if len(path) > 25 {
			path = path[:22] + "..."
		}

		// Format status - show "-" for non-HTTP
		statusStr := "-"
		if status > 0 {
			statusStr = fmt.Sprintf("%d", status)
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
	switch m.Type {
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
