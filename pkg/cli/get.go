package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/getmockd/mockd/pkg/cliconfig"
)

// reorderArgs moves flags before positional arguments to work around
// Go's flag package behavior of stopping at the first non-flag argument.
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

// RunGet handles the get command.
func RunGet(args []string) error {
	fs := flag.NewFlagSet("get", flag.ContinueOnError)

	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd get <mock-id> [flags]

Get details of a specific mock.

Arguments:
  mock-id    ID of the mock to retrieve (required)

Flags:
      --admin-url    Admin API base URL (default: http://localhost:4290)
      --json         Output in JSON format

Examples:
  # Get mock details
  mockd get abc123

  # Get as JSON
  mockd get abc123 --json
`)
	}

	// Reorder args so flags come before positional arguments
	reorderedArgs := reorderArgs(args, []string{"admin-url"})

	if err := fs.Parse(reorderedArgs); err != nil {
		return err
	}

	// Get mock ID from positional args
	if fs.NArg() < 1 {
		return fmt.Errorf(`mock ID is required

Usage: mockd get <mock-id>

Run 'mockd get --help' for more options`)
	}
	mockID := fs.Arg(0)

	// Create admin client and get mock
	client := NewAdminClientWithAuth(*adminURL)
	mock, err := client.GetMock(mockID)
	if err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == 404 {
			return fmt.Errorf("%s", FormatNotFoundError("mock", mockID))
		}
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Output result
	if *jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(mock)
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
	fmt.Printf("  Enabled:  %t\n", mock.Enabled)
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
