package cli

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/cliconfig"
	"github.com/spf13/cobra"
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

var workspaceCmd = &cobra.Command{
	Use:   "workspace",
	Short: "Manage workspaces within the current context",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWorkspaceShow()
	},
}

func init() {
	rootCmd.AddCommand(workspaceCmd)
}

var workspaceShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWorkspaceShow()
	},
}

func init() {
	workspaceCmd.AddCommand(workspaceShowCmd)
	// Add other workspace subcommands here
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

var workspaceUseAdminURL string

var workspaceUseCmd = &cobra.Command{
	Use:   "use <id>",
	Short: "Switch to a different workspace in the current context",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workspaceID := args[0]

		cfg, err := cliconfig.LoadContextConfig()
		if err != nil {
			return fmt.Errorf("failed to load context config: %w", err)
		}

		ctx := cfg.GetCurrentContext()
		if ctx == nil {
			return errors.New("no current context set; run 'mockd context add <name>' first")
		}

		// Determine admin URL
		targetURL := cliconfig.ResolveAdminURL(workspaceUseAdminURL)

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
	},
}

func init() {
	workspaceUseCmd.Flags().StringVarP(&workspaceUseAdminURL, "admin-url", "u", "", "Admin API base URL (overrides context)")
	workspaceCmd.AddCommand(workspaceUseCmd)
}

var workspaceListAdminURL string

var workspaceListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all workspaces on the server",
	RunE: func(cmd *cobra.Command, args []string) error {
		targetURL := cliconfig.ResolveAdminURL(workspaceListAdminURL)

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

		if jsonOutput {
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
	},
}

func init() {
	workspaceListCmd.Flags().StringVarP(&workspaceListAdminURL, "admin-url", "u", "", "Admin API base URL (overrides context)")
	workspaceCmd.AddCommand(workspaceListCmd)
}

var (
	workspaceCreateAdminURL   string
	workspaceCreateName       string
	workspaceCreateDesc       string
	workspaceCreateType       string
	workspaceCreateUseCurrent bool
)

var workspaceCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new workspace",
	RunE: func(cmd *cobra.Command, args []string) error {
		if workspaceCreateName == "" {
			return errors.New("workspace name required (--name)")
		}

		targetURL := cliconfig.ResolveAdminURL(workspaceCreateAdminURL)

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
		ws, err := client.CreateWorkspace(workspaceCreateName, workspaceCreateType, workspaceCreateDesc)
		if err != nil {
			return fmt.Errorf("failed to create workspace: %w", err)
		}

		// Optionally switch to this workspace
		if workspaceCreateUseCurrent {
			cfg, err := cliconfig.LoadContextConfig()
			if err == nil {
				ctx := cfg.GetCurrentContext()
				if ctx != nil {
					ctx.Workspace = ws.ID
					_ = cliconfig.SaveContextConfig(cfg)
				}
			}
		}

		if jsonOutput {
			return output.JSON(ws)
		}

		fmt.Printf("Created workspace %q (ID: %s)\n", ws.Name, ws.ID)
		if workspaceCreateUseCurrent {
			fmt.Printf("Switched to workspace %q\n", ws.ID)
		}

		return nil
	},
}

func init() {
	workspaceCreateCmd.Flags().StringVarP(&workspaceCreateAdminURL, "admin-url", "u", "", "Admin API base URL (overrides context)")
	workspaceCreateCmd.Flags().StringVarP(&workspaceCreateName, "name", "n", "", "Workspace name (required)")
	workspaceCreateCmd.Flags().StringVarP(&workspaceCreateDesc, "description", "d", "", "Workspace description")
	workspaceCreateCmd.Flags().StringVar(&workspaceCreateType, "type", "local", "Workspace type")
	workspaceCreateCmd.Flags().BoolVar(&workspaceCreateUseCurrent, "use", false, "Switch to this workspace after creating")
	workspaceCmd.AddCommand(workspaceCreateCmd)
}

var (
	workspaceDeleteAdminURL string
	workspaceDeleteForce    bool
)

var workspaceDeleteCmd = &cobra.Command{
	Use:     "delete <id>",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a workspace",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		workspaceID := args[0]
		targetURL := cliconfig.ResolveAdminURL(workspaceDeleteAdminURL)

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
		if !workspaceDeleteForce {
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
	},
}

func init() {
	workspaceDeleteCmd.Flags().StringVarP(&workspaceDeleteAdminURL, "admin-url", "u", "", "Admin API base URL (overrides context)")
	workspaceDeleteCmd.Flags().BoolVarP(&workspaceDeleteForce, "force", "f", false, "Force deletion without confirmation")
	workspaceCmd.AddCommand(workspaceDeleteCmd)
}

var workspaceClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear workspace selection (use default)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := cliconfig.LoadContextConfig()
		if err != nil {
			return fmt.Errorf("failed to load context config: %w", err)
		}

		ctx := cfg.GetCurrentContext()
		if ctx == nil {
			return errors.New("no current context set")
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
	},
}

func init() {
	workspaceCmd.AddCommand(workspaceClearCmd)
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
			Message:    "workspace not found: " + id,
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
			Message:    "workspace not found: " + id,
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
