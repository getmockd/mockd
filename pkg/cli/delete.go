package cli

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/getmockd/mockd/pkg/mock"
)

// RunDelete handles the delete command.
func RunDelete(args []string) error {
	fs := flag.NewFlagSet("delete", flag.ContinueOnError)

	adminURL := fs.String("admin-url", cliconfig.GetAdminURL(), "Admin API base URL")
	path := fs.String("path", "", "Delete mocks matching this URL path")
	method := fs.String("method", "", "HTTP method to match (used with --path, default: all methods)")
	yes := fs.Bool("yes", false, "Skip confirmation prompt")
	fs.BoolVar(yes, "y", false, "Skip confirmation prompt (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd delete [<mock-id>] [flags]

Delete mocks by ID, ID prefix, or path.

Arguments:
  mock-id    Full or partial mock ID (prefix match)

Flags:
      --path       Delete mocks matching this URL path
      --method     HTTP method filter (used with --path, default: all methods)
  -y, --yes        Skip confirmation prompt
      --admin-url  Admin API base URL (default: http://localhost:4290)

Examples:
  # Delete by full ID
  mockd delete http_abc123def456

  # Delete by ID prefix (must be unambiguous)
  mockd delete http_abc

  # Delete by path
  mockd delete --path /api/hello

  # Delete by path and method
  mockd delete --path /api/hello --method POST

  # Skip confirmation when multiple mocks match
  mockd delete --path /api/hello --yes
`)
	}

	// Reorder args so flags come before positional arguments
	reorderedArgs := reorderArgs(args, []string{"admin-url", "path", "method"})

	if err := fs.Parse(reorderedArgs); err != nil {
		return err
	}

	client := NewAdminClientWithAuth(*adminURL)

	// Determine mode: path-based or ID-based
	if *path != "" {
		return deleteByPath(client, *path, *method, *yes)
	}

	// ID-based delete (positional arg)
	if fs.NArg() < 1 {
		return fmt.Errorf(`mock ID or --path is required

Usage: mockd delete <mock-id>
       mockd delete --path /api/hello

Run 'mockd delete --help' for more options`)
	}

	return deleteByID(client, fs.Arg(0))
}

// deleteByID deletes a mock by full ID or prefix match.
func deleteByID(client AdminClient, idPrefix string) error {
	// First try exact match
	if err := client.DeleteMock(idPrefix); err != nil {
		if apiErr, ok := err.(*APIError); ok && apiErr.StatusCode == 404 {
			// Try prefix match
			return deleteByPrefix(client, idPrefix)
		}
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	fmt.Printf("Deleted mock: %s\n", idPrefix)
	return nil
}

// deleteByPrefix finds mocks matching a partial ID and deletes if unambiguous.
func deleteByPrefix(client AdminClient, prefix string) error {
	mocks, err := client.ListMocks()
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	var matches []*mock.Mock
	for _, m := range mocks {
		if strings.HasPrefix(m.ID, prefix) {
			matches = append(matches, m)
		}
	}

	switch len(matches) {
	case 0:
		return fmt.Errorf("%s", FormatNotFoundError("mock", prefix))
	case 1:
		if err := client.DeleteMock(matches[0].ID); err != nil {
			return fmt.Errorf("failed to delete mock: %s", FormatConnectionError(err))
		}
		path, method, status := extractMockDetails(matches[0])
		fmt.Printf("Deleted mock: %s (%s %s → %d)\n", matches[0].ID, method, path, status)
		return nil
	default:
		fmt.Fprintf(os.Stderr, "Ambiguous ID prefix '%s' matches %d mocks:\n\n", prefix, len(matches))
		for _, m := range matches {
			path, method, status := extractMockDetails(m)
			if status > 0 {
				fmt.Fprintf(os.Stderr, "  %s  %s %s → %d\n", m.ID, method, path, status)
			} else {
				fmt.Fprintf(os.Stderr, "  %s  %s %s\n", m.ID, method, path)
			}
		}
		fmt.Fprintln(os.Stderr, "\nProvide a longer prefix to narrow down the match.")
		return fmt.Errorf("ambiguous ID prefix: %s", prefix)
	}
}

// deleteByPath finds and deletes mocks matching a path and optional method.
func deleteByPath(client AdminClient, path, method string, skipConfirm bool) error {
	mocks, err := client.ListMocks()
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	method = strings.ToUpper(method)

	var matches []*mock.Mock
	for _, m := range mocks {
		mockPath, mockMethod, _ := extractMockDetails(m)
		if mockPath != path {
			continue
		}
		if method != "" && mockMethod != method {
			continue
		}
		matches = append(matches, m)
	}

	if len(matches) == 0 {
		if method != "" {
			return fmt.Errorf("no mocks found matching %s %s", method, path)
		}
		return fmt.Errorf("no mocks found matching path: %s", path)
	}

	// Single match — just delete it
	if len(matches) == 1 {
		if err := client.DeleteMock(matches[0].ID); err != nil {
			return fmt.Errorf("failed to delete mock: %s", FormatConnectionError(err))
		}
		_, mockMethod, mockStatus := extractMockDetails(matches[0])
		fmt.Printf("Deleted mock: %s (%s %s → %d)\n", matches[0].ID, mockMethod, path, mockStatus)
		return nil
	}

	// Multiple matches — confirm
	fmt.Printf("Found %d mocks matching %s:\n\n", len(matches), path)
	for _, m := range matches {
		_, mockMethod, mockStatus := extractMockDetails(m)
		if mockStatus > 0 {
			fmt.Printf("  %s  %s %s → %d\n", m.ID, mockMethod, path, mockStatus)
		} else {
			fmt.Printf("  %s  %s %s\n", m.ID, mockMethod, path)
		}
	}
	fmt.Println()

	if !skipConfirm {
		fmt.Printf("Delete all %d mocks? [y/N]: ", len(matches))
		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled")
			return nil
		}
	}

	deleted := 0
	for _, m := range matches {
		if err := client.DeleteMock(m.ID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to delete %s: %v\n", m.ID, err)
			continue
		}
		deleted++
	}

	fmt.Printf("Deleted %d mock(s)\n", deleted)
	return nil
}
