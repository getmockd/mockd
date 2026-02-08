package cli

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cliconfig"
)

// WorkspaceDTO matches the API response format.
type WorkspaceDTO struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"`
	Description  string `json:"description,omitempty"`
	Path         string `json:"path,omitempty"`
	URL          string `json:"url,omitempty"`
	Branch       string `json:"branch,omitempty"`
	ReadOnly     bool   `json:"readOnly,omitempty"`
	SyncStatus   string `json:"syncStatus,omitempty"`
	LastSyncedAt string `json:"lastSyncedAt,omitempty"`
	AutoSync     bool   `json:"autoSync,omitempty"`
	CreatedAt    string `json:"createdAt,omitempty"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
}

// RunWorkspace handles the workspace command and its subcommands.
func RunWorkspace(args []string) error {
	if len(args) == 0 {
		// No subcommand: show current workspace
		return runWorkspaceShow()
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "use":
		return runWorkspaceUse(subArgs)
	case "list", "ls":
		return runWorkspaceList(subArgs)
	case "create":
		return runWorkspaceCreate(subArgs)
	case "delete", "rm", "remove":
		return runWorkspaceDelete(subArgs)
	case "show":
		return runWorkspaceShow()
	case "clear":
		return runWorkspaceClear(subArgs)
	case "--help", "-h", "help":
		printWorkspaceUsage()
		return nil
	default:
		return fmt.Errorf("unknown workspace subcommand: %s\n\nRun 'mockd workspace --help' for usage", subcommand)
	}
}

func printWorkspaceUsage() {
	fmt.Print(`Usage: mockd workspace [command]

Manage workspaces within the current context.

Commands:
  (no command)  Show current workspace
  show          Show current workspace (same as no command)
  use <id>      Switch to a different workspace
  list          List all workspaces on the server
  create        Create a new workspace
  delete <id>   Delete a workspace
  clear         Clear workspace selection (use default)

Examples:
  # Show current workspace
  mockd workspace

  # List available workspaces
  mockd workspace list

  # Switch to a workspace
  mockd workspace use ws_abc123

  # Create a new workspace
  mockd workspace create --name "My Project"

  # Delete a workspace
  mockd workspace delete ws_abc123

  # Clear workspace selection
  mockd workspace clear

Notes:
  Workspace selection is stored in the current context.
  Use 'mockd context' to manage contexts.
`)
}

// runWorkspaceShow displays the current workspace.
func runWorkspaceShow() error {
	cfg, err := cliconfig.LoadContextConfig()
	if err != nil {
		return fmt.Errorf("failed to load context config: %w", err)
	}

	ctx := cfg.GetCurrentContext()
	if ctx == nil {
		fmt.Println("No current context set")
		return nil
	}

	fmt.Printf("Current context: %s\n", cfg.CurrentContext)
	fmt.Printf("  Admin URL: %s\n", ctx.AdminURL)

	if ctx.Workspace == "" {
		fmt.Println("  Workspace: (default)")
		return nil
	}

	fmt.Printf("  Workspace: %s\n", ctx.Workspace)

	// Try to fetch workspace details from server
	client := NewWorkspaceClient(ctx.AdminURL, &WorkspaceClientOptions{
		AuthToken:   ctx.AuthToken,
		TLSInsecure: ctx.TLSInsecure,
	})
	ws, err := client.GetWorkspace(ctx.Workspace)
	if err == nil && ws != nil {
		fmt.Printf("    Name: %s\n", ws.Name)
		if ws.Description != "" {
			fmt.Printf("    Description: %s\n", ws.Description)
		}
	}

	return nil
}

// runWorkspaceUse switches to a different workspace.
func runWorkspaceUse(args []string) error {
	fs := flag.NewFlagSet("workspace use", flag.ContinueOnError)
	adminURL := fs.String("admin-url", "", "Admin API base URL (overrides context)")
	fs.StringVar(adminURL, "u", "", "Admin API base URL (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd workspace use <id>

Switch to a different workspace in the current context.

Flags:
  -u, --admin-url  Admin API base URL (overrides context)

Examples:
  mockd workspace use ws_abc123
  mockd workspace use local
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("workspace ID required")
	}

	workspaceID := fs.Arg(0)

	cfg, err := cliconfig.LoadContextConfig()
	if err != nil {
		return fmt.Errorf("failed to load context config: %w", err)
	}

	ctx := cfg.GetCurrentContext()
	if ctx == nil {
		return fmt.Errorf("no current context set; run 'mockd context add <name>' first")
	}

	// Determine admin URL
	targetURL := cliconfig.ResolveAdminURL(*adminURL)

	// Verify workspace exists on server
	client := NewWorkspaceClient(targetURL, &WorkspaceClientOptions{
		AuthToken:   ctx.AuthToken,
		TLSInsecure: ctx.TLSInsecure,
	})
	ws, err := client.GetWorkspace(workspaceID)
	if err != nil {
		return fmt.Errorf("failed to verify workspace: %w", err)
	}

	// Update context
	ctx.Workspace = workspaceID
	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		return fmt.Errorf("failed to save context config: %w", err)
	}

	fmt.Printf("Switched to workspace %q\n", workspaceID)
	if ws != nil {
		fmt.Printf("  Name: %s\n", ws.Name)
	}

	return nil
}

// runWorkspaceList lists all workspaces.
func runWorkspaceList(args []string) error {
	fs := flag.NewFlagSet("workspace list", flag.ContinueOnError)
	adminURL := fs.String("admin-url", "", "Admin API base URL (overrides context)")
	fs.StringVar(adminURL, "u", "", "Admin API base URL (shorthand)")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd workspace list [flags]

List all workspaces on the server.

Flags:
  -u, --admin-url  Admin API base URL (overrides context)
      --json       Output in JSON format

Examples:
  mockd workspace list
  mockd workspace list --json
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	targetURL := cliconfig.ResolveAdminURL(*adminURL)

	// Get client options from context
	var opts *WorkspaceClientOptions
	if cfg, err := cliconfig.LoadContextConfig(); err == nil {
		if ctx := cfg.GetCurrentContext(); ctx != nil {
			opts = &WorkspaceClientOptions{
				AuthToken:   ctx.AuthToken,
				TLSInsecure: ctx.TLSInsecure,
			}
		}
	}

	client := NewWorkspaceClient(targetURL, opts)
	workspaces, err := client.ListWorkspaces()
	if err != nil {
		return fmt.Errorf("%s", FormatConnectionError(err))
	}

	// Get current workspace for marking
	currentWorkspace := cliconfig.GetWorkspaceFromContext()

	if *jsonOutput {
		result := struct {
			CurrentWorkspace string          `json:"currentWorkspace"`
			Workspaces       []*WorkspaceDTO `json:"workspaces"`
			Count            int             `json:"count"`
		}{
			CurrentWorkspace: currentWorkspace,
			Workspaces:       workspaces,
			Count:            len(workspaces),
		}
		return output.JSON(result)
	}

	if len(workspaces) == 0 {
		fmt.Println("No workspaces found")
		return nil
	}

	w := output.Table()
	_, _ = fmt.Fprintln(w, "CURRENT\tID\tNAME\tTYPE\tDESCRIPTION")

	for _, ws := range workspaces {
		current := ""
		if ws.ID == currentWorkspace {
			current = "*"
		}

		description := ws.Description
		if len(description) > 30 {
			description = description[:27] + "..."
		}
		if description == "" {
			description = "-"
		}

		id := ws.ID
		if len(id) > 20 {
			id = id[:17] + "..."
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", current, id, ws.Name, ws.Type, description)
	}

	return w.Flush()
}

// runWorkspaceCreate creates a new workspace.
func runWorkspaceCreate(args []string) error {
	fs := flag.NewFlagSet("workspace create", flag.ContinueOnError)
	adminURL := fs.String("admin-url", "", "Admin API base URL (overrides context)")
	fs.StringVar(adminURL, "u", "", "Admin API base URL (shorthand)")
	name := fs.String("name", "", "Workspace name (required)")
	fs.StringVar(name, "n", "", "Workspace name (shorthand)")
	description := fs.String("description", "", "Workspace description")
	fs.StringVar(description, "d", "", "Workspace description (shorthand)")
	wsType := fs.String("type", "local", "Workspace type (local)")
	useCurrent := fs.Bool("use", false, "Switch to this workspace after creating")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd workspace create [flags]

Create a new workspace.

Flags:
  -n, --name         Workspace name (required)
  -d, --description  Workspace description
      --type         Workspace type (default: local)
      --use          Switch to this workspace after creating
  -u, --admin-url    Admin API base URL (overrides context)
      --json         Output in JSON format

Examples:
  mockd workspace create --name "My Project"
  mockd workspace create --name "API Tests" --description "Mocks for API testing" --use
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if *name == "" {
		fs.Usage()
		return fmt.Errorf("workspace name required (--name)")
	}

	targetURL := cliconfig.ResolveAdminURL(*adminURL)

	// Get client options from context
	var opts *WorkspaceClientOptions
	if cfg, err := cliconfig.LoadContextConfig(); err == nil {
		if ctx := cfg.GetCurrentContext(); ctx != nil {
			opts = &WorkspaceClientOptions{
				AuthToken:   ctx.AuthToken,
				TLSInsecure: ctx.TLSInsecure,
			}
		}
	}

	client := NewWorkspaceClient(targetURL, opts)
	ws, err := client.CreateWorkspace(*name, *wsType, *description)
	if err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	// Optionally switch to this workspace
	if *useCurrent {
		cfg, err := cliconfig.LoadContextConfig()
		if err == nil {
			ctx := cfg.GetCurrentContext()
			if ctx != nil {
				ctx.Workspace = ws.ID
				_ = cliconfig.SaveContextConfig(cfg)
			}
		}
	}

	if *jsonOutput {
		return output.JSON(ws)
	}

	fmt.Printf("Created workspace %q (ID: %s)\n", ws.Name, ws.ID)
	if *useCurrent {
		fmt.Printf("Switched to workspace %q\n", ws.ID)
	}

	return nil
}

// runWorkspaceDelete deletes a workspace.
func runWorkspaceDelete(args []string) error {
	fs := flag.NewFlagSet("workspace delete", flag.ContinueOnError)
	adminURL := fs.String("admin-url", "", "Admin API base URL (overrides context)")
	fs.StringVar(adminURL, "u", "", "Admin API base URL (shorthand)")
	force := fs.Bool("force", false, "Force deletion without confirmation")
	fs.BoolVar(force, "f", false, "Force deletion (shorthand)")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd workspace delete <id> [flags]

Delete a workspace.

Flags:
  -f, --force      Force deletion without confirmation
  -u, --admin-url  Admin API base URL (overrides context)

Examples:
  mockd workspace delete ws_abc123
  mockd workspace delete ws_abc123 --force
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("workspace ID required")
	}

	workspaceID := fs.Arg(0)
	targetURL := cliconfig.ResolveAdminURL(*adminURL)

	// Get client options from context
	var opts *WorkspaceClientOptions
	if cfg, err := cliconfig.LoadContextConfig(); err == nil {
		if ctx := cfg.GetCurrentContext(); ctx != nil {
			opts = &WorkspaceClientOptions{
				AuthToken:   ctx.AuthToken,
				TLSInsecure: ctx.TLSInsecure,
			}
		}
	}

	client := NewWorkspaceClient(targetURL, opts)

	// Get workspace details first
	ws, err := client.GetWorkspace(workspaceID)
	if err != nil {
		return fmt.Errorf("failed to get workspace: %w", err)
	}

	// Confirm unless forced
	if !*force {
		fmt.Printf("Delete workspace %q?\n", ws.Name)
		fmt.Printf("  ID: %s\n", ws.ID)
		fmt.Print("Type 'yes' to confirm: ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		if strings.TrimSpace(input) != "yes" {
			fmt.Println("Aborted")
			return nil
		}
	}

	if err := client.DeleteWorkspace(workspaceID); err != nil {
		return fmt.Errorf("failed to delete workspace: %w", err)
	}

	// Clear from context if it was the current workspace
	cfg, err := cliconfig.LoadContextConfig()
	if err == nil {
		ctx := cfg.GetCurrentContext()
		if ctx != nil && ctx.Workspace == workspaceID {
			ctx.Workspace = ""
			_ = cliconfig.SaveContextConfig(cfg)
			fmt.Println("Cleared workspace from current context")
		}
	}

	fmt.Printf("Deleted workspace %q\n", workspaceID)
	return nil
}

// runWorkspaceClear clears the workspace selection.
func runWorkspaceClear(args []string) error {
	fs := flag.NewFlagSet("workspace clear", flag.ContinueOnError)

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd workspace clear

Clear workspace selection in the current context.
This will use the default workspace.

Examples:
  mockd workspace clear
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	cfg, err := cliconfig.LoadContextConfig()
	if err != nil {
		return fmt.Errorf("failed to load context config: %w", err)
	}

	ctx := cfg.GetCurrentContext()
	if ctx == nil {
		return fmt.Errorf("no current context set")
	}

	if ctx.Workspace == "" {
		fmt.Println("No workspace was selected")
		return nil
	}

	oldWorkspace := ctx.Workspace
	ctx.Workspace = ""

	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		return fmt.Errorf("failed to save context config: %w", err)
	}

	fmt.Printf("Cleared workspace selection (was: %s)\n", oldWorkspace)
	return nil
}

// WorkspaceClient provides methods for workspace API calls.
type WorkspaceClient struct {
	baseURL    string
	authToken  string
	httpClient *http.Client
}

// WorkspaceClientOptions configures the workspace client.
type WorkspaceClientOptions struct {
	AuthToken   string
	TLSInsecure bool
}

// NewWorkspaceClient creates a new workspace API client.
func NewWorkspaceClient(baseURL string, opts *WorkspaceClientOptions) *WorkspaceClient {
	client := &WorkspaceClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	if opts != nil {
		client.authToken = opts.AuthToken

		if opts.TLSInsecure {
			client.httpClient.Transport = &http.Transport{
				//nolint:gosec // G402: InsecureSkipVerify is intentional when --insecure flag is used
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			}
		}
	}

	return client
}

// NewWorkspaceClientFromContext creates a workspace client using the current context settings.
func NewWorkspaceClientFromContext() (*WorkspaceClient, error) {
	cfg, err := cliconfig.LoadContextConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load context config: %w", err)
	}

	ctx := cfg.GetCurrentContext()
	if ctx == nil {
		// Fall back to default
		return NewWorkspaceClient(cliconfig.DefaultAdminURL(cliconfig.DefaultAdminPort), nil), nil
	}

	return NewWorkspaceClient(ctx.AdminURL, &WorkspaceClientOptions{
		AuthToken:   ctx.AuthToken,
		TLSInsecure: ctx.TLSInsecure,
	}), nil
}

// doRequest performs an HTTP request with auth token if configured.
func (c *WorkspaceClient) doRequest(req *http.Request) (*http.Response, error) {
	if c.authToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.authToken)
	}
	return c.httpClient.Do(req)
}

// ListWorkspaces returns all workspaces.
func (c *WorkspaceClient) ListWorkspaces() ([]*WorkspaceDTO, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/workspaces", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, &APIError{
			StatusCode: 0,
			ErrorCode:  "connection_error",
			Message:    fmt.Sprintf("cannot connect to admin API at %s: %v", c.baseURL, err),
		}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var result struct {
		Workspaces []*WorkspaceDTO `json:"workspaces"`
		Count      int             `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Workspaces, nil
}

// GetWorkspace returns a specific workspace.
func (c *WorkspaceClient) GetWorkspace(id string) (*WorkspaceDTO, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/workspaces/"+url.PathEscape(id), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, &APIError{
			StatusCode: 0,
			ErrorCode:  "connection_error",
			Message:    fmt.Sprintf("cannot connect to admin API at %s: %v", c.baseURL, err),
		}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			ErrorCode:  "not_found",
			Message:    fmt.Sprintf("workspace not found: %s", id),
		}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp)
	}

	var ws WorkspaceDTO
	if err := json.NewDecoder(resp.Body).Decode(&ws); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &ws, nil
}

// CreateWorkspace creates a new workspace.
func (c *WorkspaceClient) CreateWorkspace(name, wsType, description string) (*WorkspaceDTO, error) {
	body := map[string]interface{}{
		"name": name,
	}
	if wsType != "" {
		body["type"] = wsType
	}
	if description != "" {
		body["description"] = description
	}

	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.baseURL+"/workspaces", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, &APIError{
			StatusCode: 0,
			ErrorCode:  "connection_error",
			Message:    fmt.Sprintf("cannot connect to admin API at %s: %v", c.baseURL, err),
		}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		return nil, parseAPIError(resp)
	}

	var ws WorkspaceDTO
	if err := json.NewDecoder(resp.Body).Decode(&ws); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &ws, nil
}

// DeleteWorkspace deletes a workspace.
func (c *WorkspaceClient) DeleteWorkspace(id string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/workspaces/"+url.PathEscape(id), nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.doRequest(req)
	if err != nil {
		return &APIError{
			StatusCode: 0,
			ErrorCode:  "connection_error",
			Message:    fmt.Sprintf("cannot connect to admin API at %s: %v", c.baseURL, err),
		}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return &APIError{
			StatusCode: resp.StatusCode,
			ErrorCode:  "not_found",
			Message:    fmt.Sprintf("workspace not found: %s", id),
		}
	}
	if resp.StatusCode != http.StatusNoContent {
		return parseAPIError(resp)
	}

	return nil
}

// parseAPIError parses an error response.
func parseAPIError(resp *http.Response) error {
	var errResp struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err == nil && errResp.Message != "" {
		return &APIError{
			StatusCode: resp.StatusCode,
			ErrorCode:  errResp.Error,
			Message:    errResp.Message,
		}
	}
	return &APIError{
		StatusCode: resp.StatusCode,
		ErrorCode:  "unknown_error",
		Message:    fmt.Sprintf("server returned status %d", resp.StatusCode),
	}
}
