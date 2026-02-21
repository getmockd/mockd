package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/spf13/cobra"
)

// reorderArgs moves flags before positional arguments to work around
// Go's flag package behavior of stopping at the first non-flag argument.
// Keep it in case other files use it still, but mockd get no longer needs it for cobra.
func reorderArgs(args []string, knownFlags []string) []string {
	var flags, positional []string

	i := 0
	for i < len(args) {
		arg := args[i]

		// Check if it's a known flag
		isFlag := false
		for _, f := range knownFlags {
			if arg == "--"+f || arg == "-"+f {
				// Flag with separate value
				flags = append(flags, arg)
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					flags = append(flags, args[i])
				}
				isFlag = true
				break
			}
			if strings.HasPrefix(arg, "--"+f+"=") || strings.HasPrefix(arg, "-"+f+"=") {
				// Flag with = value
				flags = append(flags, arg)
				isFlag = true
				break
			}
		}

		// Check for boolean flags (no value)
		if !isFlag && (arg == "--json" || arg == "-json") {
			flags = append(flags, arg)
			isFlag = true
		}

		if !isFlag {
			if strings.HasPrefix(arg, "-") && arg != "-" {
				// Unknown flag, still treat as flag
				flags = append(flags, arg)
			} else {
				positional = append(positional, arg)
			}
		}
		i++
	}

	return append(flags, positional...)
}

var getCmd = &cobra.Command{
	Use:   "get <mock-id>",
	Short: "Get details of a specific mock",
	Long:  `Get details of a specific mock.`,
	Example: `  # Get mock details
  mockd get abc123

  # Get as JSON
  mockd get abc123 --json`,
	RunE: runGet,
}

func init() {
	rootCmd.AddCommand(getCmd)
}

//nolint:gocyclo
func runGet(cmd *cobra.Command, args []string) error {

	// Get mock ID from positional args
	if len(args) < 1 {
		return errors.New(`mock ID is required

Usage: mockd get <mock-id>

Run 'mockd get --help' for more options`)
	}
	mockID := args[0]

	// Create admin client and get mock
	client := NewAdminClientWithAuth(adminURL)
	mock, err := client.GetMock(mockID)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == 404 {
			return fmt.Errorf("%s", FormatNotFoundError("mock", mockID))
		}
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Output result
	if jsonOutput {
		return output.JSON(mock)
	}

	// Human-readable output
	fmt.Printf("Mock: %s\n", mock.ID)
	if mock.Name != "" {
		fmt.Printf("  Name:     %s\n", mock.Name)
	}
	if mock.HTTP != nil && mock.HTTP.Matcher != nil {
		fmt.Printf("  Method:   %s\n", mock.HTTP.Matcher.Method)
		fmt.Printf("  Path:     %s\n", mock.HTTP.Matcher.Path)
	}
	if mock.HTTP != nil && mock.HTTP.Response != nil {
		fmt.Printf("  Status:   %d\n", mock.HTTP.Response.StatusCode)
	}
	fmt.Printf("  Enabled:  %t\n", mock.Enabled == nil || *mock.Enabled)
	priority := 0
	if mock.HTTP != nil {
		priority = mock.HTTP.Priority
	}
	fmt.Printf("  Priority: %d\n", priority)

	// Response headers
	if mock.HTTP != nil && mock.HTTP.Response != nil && len(mock.HTTP.Response.Headers) > 0 {
		fmt.Println("  Headers:")
		for key, value := range mock.HTTP.Response.Headers {
			fmt.Printf("    %s: %s\n", key, value)
		}
	}

	// Match headers
	if mock.HTTP != nil && mock.HTTP.Matcher != nil && len(mock.HTTP.Matcher.Headers) > 0 {
		fmt.Println("  Match Headers:")
		for key, value := range mock.HTTP.Matcher.Headers {
			fmt.Printf("    %s: %s\n", key, value)
		}
	}

	// Match query params
	if mock.HTTP != nil && mock.HTTP.Matcher != nil && len(mock.HTTP.Matcher.QueryParams) > 0 {
		fmt.Println("  Match Query:")
		for key, value := range mock.HTTP.Matcher.QueryParams {
			fmt.Printf("    %s: %s\n", key, value)
		}
	}

	// Response body
	if mock.HTTP != nil && mock.HTTP.Response != nil && mock.HTTP.Response.Body != "" {
		fmt.Println("  Body:")
		// Indent body content
		body := mock.HTTP.Response.Body
		if len(body) > 500 {
			body = body[:500] + "...(truncated)"
		}
		fmt.Printf("    %s\n", body)
	}

	// Delay
	if mock.HTTP != nil && mock.HTTP.Response != nil && mock.HTTP.Response.DelayMs > 0 {
		fmt.Printf("  Delay:    %dms\n", mock.HTTP.Response.DelayMs)
	}

	return nil
}
