package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/spf13/cobra"
)

var (
	deletePath   string
	deleteMethod string
	deleteYes    bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete [<mock-id>]",
	Short: "Delete mocks by ID, ID prefix, or path",
	Long:  `Delete mocks by ID, ID prefix, or path.`,
	Example: `  # Delete by full ID
  mockd delete http_abc123def456

  # Delete by ID prefix (must be unambiguous)
  mockd delete http_abc

  # Delete by path
  mockd delete --path /api/hello

  # Delete by path and method
  mockd delete --path /api/hello --method POST

  # Skip confirmation when multiple mocks match
  mockd delete --path /api/hello --yes`,
	RunE: runDelete,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
	deleteCmd.Flags().StringVar(&deletePath, "path", "", "Delete mocks matching this URL path")
	deleteCmd.Flags().StringVar(&deleteMethod, "method", "", "HTTP method to match (used with --path, default: all methods)")
	deleteCmd.Flags().BoolVarP(&deleteYes, "yes", "y", false, "Skip confirmation prompt")
}

func runDelete(cmd *cobra.Command, args []string) error {
	path := deletePath
	method := deleteMethod
	yes := deleteYes

	client := NewAdminClientWithAuth(adminURL)

	// Determine mode: path-based or ID-based
	if path != "" {
		return deleteByPath(client, path, method, yes)
	}

	// ID-based delete (positional arg)
	if len(args) < 1 {
		return errors.New("mock ID or --path is required\n\nUsage: mockd delete <mock-id>\n       mockd delete --path /api/hello\n\nRun 'mockd delete --help' for more options")
	}

	return deleteByID(client, args[0])
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

	printResult(map[string]any{"id": idPrefix, "deleted": true}, func() {
		fmt.Printf("Deleted mock: %s\n", idPrefix)
	})
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
		printResult(map[string]any{"id": matches[0].ID, "method": method, "path": path, "statusCode": status, "deleted": true}, func() {
			fmt.Printf("Deleted mock: %s (%s %s → %d)\n", matches[0].ID, method, path, status)
		})
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
		printResult(map[string]any{"id": matches[0].ID, "method": mockMethod, "path": path, "statusCode": mockStatus, "deleted": true}, func() {
			fmt.Printf("Deleted mock: %s (%s %s → %d)\n", matches[0].ID, mockMethod, path, mockStatus)
		})
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
	deletedIDs := make([]string, 0, len(matches))
	for _, m := range matches {
		if err := client.DeleteMock(m.ID); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to delete %s: %v\n", m.ID, err)
			continue
		}
		deleted++
		deletedIDs = append(deletedIDs, m.ID)
	}

	printResult(map[string]any{"deleted": true, "count": deleted, "ids": deletedIDs}, func() {
		fmt.Printf("Deleted %d mock(s)\n", deleted)
	})
	return nil
}
