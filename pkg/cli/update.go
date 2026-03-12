package cli

import (
	"fmt"
	"os"

	"github.com/getmockd/mockd/pkg/cli/internal/flags"
	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cli/internal/parse"
	"github.com/spf13/cobra"
)

var (
	updateBody      string
	updateBodyFile  string
	updateStatus    int
	updateHeaders   flags.StringSlice
	updateDelay     int
	updateTable     string
	updateBind      string
	updateOperation string
	updateName      string
	updateEnabled   string // "true" or "false"
)

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update an existing mock endpoint",
	Long: `Update an existing mock endpoint by ID.

Only specified fields are modified — unspecified fields retain their current values.

Examples:
  mockd update http_abc123 --status 201
  mockd update http_abc123 --body '{"updated": true}'
  mockd update http_abc123 --table users --bind list
  mockd update http_abc123 --delay 500
  mockd update http_abc123 --enabled false`,
	Args: cobra.ExactArgs(1),
	RunE: runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)

	updateCmd.Flags().StringVarP(&updateBody, "body", "b", "", "Response body")
	updateCmd.Flags().StringVar(&updateBodyFile, "body-file", "", "Read response body from file")
	updateCmd.Flags().IntVarP(&updateStatus, "status", "s", 0, "Response status code")
	updateCmd.Flags().VarP(&updateHeaders, "header", "H", "Response header (key:value), repeatable")
	updateCmd.Flags().IntVar(&updateDelay, "delay", 0, "Response delay in milliseconds")
	updateCmd.Flags().StringVar(&updateTable, "table", "", "Bind to a stateful resource table")
	updateCmd.Flags().StringVar(&updateBind, "bind", "", "Stateful action: list, get, create, update, delete, custom")
	updateCmd.Flags().StringVar(&updateOperation, "operation", "", "Custom operation name (for --bind custom)")
	updateCmd.Flags().StringVarP(&updateName, "name", "n", "", "Mock display name")
	updateCmd.Flags().StringVar(&updateEnabled, "enabled", "", "Enable or disable the mock (true/false)")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	id := args[0]

	// Build patch object with only specified fields
	patch := make(map[string]interface{})

	if updateName != "" {
		patch["name"] = updateName
	}

	if updateEnabled != "" {
		switch updateEnabled {
		case "true":
			patch["enabled"] = true
		case "false":
			patch["enabled"] = false
		default:
			return fmt.Errorf("--enabled must be 'true' or 'false', got %q", updateEnabled)
		}
	}

	// Build HTTP spec patch
	httpPatch := make(map[string]interface{})
	hasHTTPPatch := false

	// Handle --table/--bind for stateful binding
	if updateTable != "" || updateBind != "" {
		if (updateTable != "") != (updateBind != "") {
			return fmt.Errorf("--table and --bind must be used together")
		}
		validActions := map[string]bool{
			"list": true, "get": true, "create": true,
			"update": true, "delete": true, "custom": true, "patch": true,
		}
		if !validActions[updateBind] {
			return fmt.Errorf("invalid --bind value %q: must be one of list, get, create, update, delete, custom, patch", updateBind)
		}
		binding := map[string]interface{}{
			"table":  updateTable,
			"action": updateBind,
		}
		if updateOperation != "" {
			binding["operation"] = updateOperation
		}
		httpPatch["statefulBinding"] = binding
		// Clear conflicting response types
		httpPatch["response"] = nil
		httpPatch["sse"] = nil
		httpPatch["statefulOperation"] = nil
		hasHTTPPatch = true
	}

	// Handle response fields
	responsePatch := make(map[string]interface{})
	hasResponsePatch := false

	responseBody := updateBody
	if updateBodyFile != "" {
		data, err := os.ReadFile(updateBodyFile)
		if err != nil {
			return fmt.Errorf("failed to read body file: %w", err)
		}
		responseBody = string(data)
	}
	if responseBody != "" {
		responsePatch["body"] = responseBody
		hasResponsePatch = true
	}
	if cmd.Flags().Changed("status") && updateStatus > 0 {
		responsePatch["statusCode"] = updateStatus
		hasResponsePatch = true
	}
	if cmd.Flags().Changed("delay") {
		responsePatch["delayMs"] = updateDelay
		hasResponsePatch = true
	}
	if len(updateHeaders) > 0 {
		headers := make(map[string]string)
		for _, h := range updateHeaders {
			key, value, ok := parse.KeyValue(h, ':')
			if !ok {
				return fmt.Errorf("invalid header format: %s (expected key:value)", h)
			}
			headers[key] = value
		}
		responsePatch["headers"] = headers
		hasResponsePatch = true
	}

	if hasResponsePatch {
		httpPatch["response"] = responsePatch
		hasHTTPPatch = true
	}

	if hasHTTPPatch {
		patch["http"] = httpPatch
	}

	if len(patch) == 0 {
		return fmt.Errorf("no update flags specified\n\nUsage: mockd update <id> [flags]\nRun 'mockd update --help' for available flags")
	}

	client := NewAdminClientWithAuth(adminURL)
	updated, err := client.PatchMock(id, patch)
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	if jsonOutput {
		return output.JSON(struct {
			ID     string `json:"id"`
			Action string `json:"action"`
		}{
			ID:     updated.ID,
			Action: "updated",
		})
	}

	fmt.Printf("Updated mock: %s\n", updated.ID)
	if updated.HTTP != nil && updated.HTTP.Matcher != nil {
		fmt.Printf("  %s %s\n", updated.HTTP.Matcher.Method, updated.HTTP.Matcher.Path)
	}
	if updated.HTTP != nil && updated.HTTP.StatefulBinding != nil {
		fmt.Printf("  Table: %s\n", updated.HTTP.StatefulBinding.Table)
		fmt.Printf("  Bind:  %s\n", updated.HTTP.StatefulBinding.Action)
	} else if updated.HTTP != nil && updated.HTTP.Response != nil {
		fmt.Printf("  Status: %d\n", updated.HTTP.Response.StatusCode)
	}

	return nil
}
