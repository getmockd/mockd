package mcp

import (
	"github.com/getmockd/mockd/pkg/cli"
	"github.com/getmockd/mockd/pkg/cliconfig"
)

// =============================================================================
// Context / Workspace Handlers
// =============================================================================

// handleGetCurrentContext shows the active context and all available contexts.
func handleGetCurrentContext(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	// Load all configured contexts (read-only)
	ctxConfig, _ := cliconfig.LoadContextConfig()

	result := ContextResult{
		Current:  session.GetContextName(),
		AdminURL: session.GetAdminURL(),
	}
	if ws := session.GetWorkspace(); ws != "" {
		result.Workspace = ws
	}

	// Build context list — AuthToken intentionally omitted for security.
	if ctxConfig != nil {
		for name, ctx := range ctxConfig.Contexts {
			result.Contexts = append(result.Contexts, ContextInfo{
				Name:        name,
				AdminURL:    ctx.AdminURL,
				Workspace:   ctx.Workspace,
				Description: ctx.Description,
			})
		}
	}

	// If no contexts configured, show at least the current one
	if len(result.Contexts) == 0 {
		result.Contexts = []ContextInfo{
			{
				Name:        result.Current,
				AdminURL:    result.AdminURL,
				Workspace:   result.Workspace,
				Description: "Current session",
			},
		}
	}

	return ToolResultJSON(result)
}

// handleSwitchContext switches to a different context (session-scoped, not persisted).
func handleSwitchContext(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	name := getString(args, "name", "")
	if name == "" {
		return ToolResultError("name is required"), nil
	}

	// Load context store (read-only)
	ctxConfig, err := cliconfig.LoadContextConfig()
	if err != nil {
		return ToolResultError("failed to load contexts: " + err.Error()), nil
	}
	if ctxConfig == nil {
		return ToolResultError("no contexts configured in ~/.config/mockd/contexts.yaml"), nil
	}

	ctx, ok := ctxConfig.Contexts[name]
	if !ok {
		return ToolResultError("context not found: " + name), nil
	}

	// Create new admin client for the target context.
	// Use the client factory if available, otherwise fall back to basic client.
	var newClient cli.AdminClient
	if server.clientFactory != nil {
		newClient = server.clientFactory(ctx.AdminURL)
	} else {
		newClient = cli.NewAdminClient(ctx.AdminURL)
	}

	// Update session atomically
	session.SetContext(name, ctx.AdminURL, ctx.Workspace, newClient)

	// Return updated context info (no auth token)
	result := ContextResult{
		Current:   name,
		AdminURL:  ctx.AdminURL,
		Workspace: ctx.Workspace,
	}

	return ToolResultJSON(result)
}

// handleListWorkspaces lists workspaces on the current admin server.
func handleListWorkspaces(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	client := session.GetAdminClient()
	if client == nil {
		return ToolResultError("admin client not available"), nil
	}

	workspaces, err := client.ListWorkspaces()
	if err != nil {
		//nolint:nilerr // MCP spec: tool errors are returned in result content, not as JSON-RPC errors
		return ToolResultError("failed to list workspaces: " + err.Error()), nil
	}

	currentWS := session.GetWorkspace()

	type workspaceSummary struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Type        string `json:"type,omitempty"`
		Description string `json:"description,omitempty"`
		Active      bool   `json:"active"`
	}

	items := make([]workspaceSummary, 0, len(workspaces))
	for _, ws := range workspaces {
		items = append(items, workspaceSummary{
			ID:          ws.ID,
			Name:        ws.Name,
			Type:        ws.Type,
			Description: ws.Description,
			Active:      ws.ID == currentWS,
		})
	}

	result := map[string]interface{}{
		"currentWorkspace": currentWS,
		"workspaces":       items,
	}

	return ToolResultJSON(result)
}

// handleSwitchWorkspace switches the active workspace (session-scoped, not persisted).
func handleSwitchWorkspace(args map[string]interface{}, session *MCPSession, server *Server) (*ToolResult, error) {
	id := getString(args, "id", "")
	if id == "" {
		return ToolResultError("id is required"), nil
	}

	session.SetWorkspace(id)

	result := map[string]interface{}{
		"switched":  true,
		"workspace": id,
	}

	return ToolResultJSON(result)
}
