package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/getmockd/mockd/pkg/cliconfig"
)

// RunDelete handles the delete command.
func RunDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)

	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd delete <mock-id>

Delete a mock by ID.

Arguments:
  mock-id    ID of the mock to delete (required)

Flags:
      --admin-url    Admin API base URL (default: http://localhost:4290)

Examples:
  mockd delete abc123
`)
	}

	// Reorder args so flags come before positional arguments
	reorderedArgs := reorderArgs(args, []string{"admin-url"})

	if err := fs.Parse(reorderedArgs); err != nil {
		return err
	}

	// Get mock ID from positional args
	if fs.NArg() < 1 {
		return errors.New(`mock ID is required

Usage: mockd delete <mock-id>

Run 'mockd delete --help' for more options`)
	}
	mockID := fs.Arg(0)

	// Create admin client and delete mock
	client := NewAdminClientWithAuth(*adminURL)
	if err := client.DeleteMock(mockID); err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == 404 {
			return fmt.Errorf("%s", FormatNotFoundError("mock", mockID))
		}
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	fmt.Printf("Deleted mock: %s\n", mockID)
	return nil
}
