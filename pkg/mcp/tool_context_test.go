package mcp

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/cli"
)

// =============================================================================
// handleManageContext Tests
// =============================================================================

func TestHandleManageContext_GetAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "get"}
	result, err := handleManageContext(args, session, server)
	if err != nil {
		t.Fatalf("handleManageContext() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var ctxResult ContextResult
	resultJSON(t, result, &ctxResult)

	if ctxResult.Current != "test-ctx" {
		t.Errorf("Current = %s, want test-ctx", ctxResult.Current)
	}
	if ctxResult.AdminURL != "http://localhost:4290" {
		t.Errorf("AdminURL = %s, want http://localhost:4290", ctxResult.AdminURL)
	}
}

func TestHandleManageContext_GetNoContextsConfigured(t *testing.T) {
	t.Parallel()

	// handleGetCurrentContext reads from cliconfig.LoadContextConfig(). If no
	// contexts.yaml exists on disk it falls back to showing the session's own
	// context. If one does exist the file contexts are returned. Either way the
	// result must be valid: Current matches the session, and Contexts is non-empty.
	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "get"}
	result, err := handleManageContext(args, session, server)
	if err != nil {
		t.Fatalf("handleManageContext() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var ctxResult ContextResult
	resultJSON(t, result, &ctxResult)

	// Current should always reflect the session's context name
	if ctxResult.Current != "test-ctx" {
		t.Errorf("Current = %s, want test-ctx", ctxResult.Current)
	}

	// AdminURL should always reflect the session's admin URL
	if ctxResult.AdminURL != "http://localhost:4290" {
		t.Errorf("AdminURL = %s, want http://localhost:4290", ctxResult.AdminURL)
	}

	// Should have at least one context entry (either from file or fallback)
	if len(ctxResult.Contexts) == 0 {
		t.Fatal("expected at least one context in result")
	}
}

func TestHandleManageContext_MissingAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	// Empty string action hits the default case
	args := map[string]interface{}{}
	result, err := handleManageContext(args, session, server)
	if err != nil {
		t.Fatalf("handleManageContext() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing action")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleManageContext_UnknownAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "destroy"}
	result, err := handleManageContext(args, session, server)
	if err != nil {
		t.Fatalf("handleManageContext() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for unknown action")
	}

	text := resultText(t, result)
	if text != "invalid action: destroy. Use: get, switch" {
		t.Errorf("error text = %q, want %q", text, "invalid action: destroy. Use: get, switch")
	}
}

func TestHandleManageContext_SwitchMissingName(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "switch"}
	result, err := handleManageContext(args, session, server)
	if err != nil {
		t.Fatalf("handleManageContext() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for switch without name")
	}

	text := resultText(t, result)
	if text != "name is required" {
		t.Errorf("error text = %q, want %q", text, "name is required")
	}
}

func TestHandleManageContext_SwitchConfigNotFound(t *testing.T) {
	t.Parallel()

	// handleSwitchContext reads from cliconfig.LoadContextConfig() which
	// will fail or return nil when no contexts.yaml exists. We test the
	// error path — the exact message depends on whether LoadContextConfig
	// returns an error or nil config.
	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "switch", "name": "staging"}
	result, err := handleManageContext(args, session, server)
	if err != nil {
		t.Fatalf("handleManageContext() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when contexts config is missing")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
	// Should mention either "no contexts configured" or "failed to load contexts"
	// or "context not found" depending on filesystem state.
}

// =============================================================================
// handleManageWorkspace Tests
// =============================================================================

func TestHandleManageWorkspace_ListAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listWorkspacesFn: func() ([]*cli.WorkspaceDTO, error) {
			return []*cli.WorkspaceDTO{
				{ID: "ws-1", Name: "default", Type: "local"},
				{ID: "ws-2", Name: "staging", Type: "remote"},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	workspaces, ok := parsed["workspaces"].([]interface{})
	if !ok {
		t.Fatal("expected 'workspaces' array in result")
	}
	if len(workspaces) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(workspaces))
	}
}

func TestHandleManageWorkspace_ListWithActive(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listWorkspacesFn: func() ([]*cli.WorkspaceDTO, error) {
			return []*cli.WorkspaceDTO{
				{ID: "ws-1", Name: "default", Type: "local"},
				{ID: "ws-2", Name: "staging", Type: "remote"},
			}, nil
		},
	}

	session := newTestSession(client)
	session.SetWorkspace("ws-2")
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	workspaces, ok := parsed["workspaces"].([]interface{})
	if !ok {
		t.Fatal("expected 'workspaces' array in result")
	}

	// Check that ws-2 is marked active
	for _, ws := range workspaces {
		wsMap, ok := ws.(map[string]interface{})
		if !ok {
			continue
		}
		if wsMap["id"] == "ws-2" {
			if wsMap["active"] != true {
				t.Errorf("workspace ws-2 should be active, got active=%v", wsMap["active"])
			}
		} else if wsMap["id"] == "ws-1" {
			if wsMap["active"] == true {
				t.Errorf("workspace ws-1 should not be active")
			}
		}
	}
}

func TestHandleManageWorkspace_ListFailure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listWorkspacesFn: func() ([]*cli.WorkspaceDTO, error) {
			return nil, fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for list failure")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "failed to list workspaces") {
		t.Errorf("error text = %q, want containing 'failed to list workspaces'", text)
	}
}

func TestHandleManageWorkspace_ListNoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{"action": "list"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

func TestHandleManageWorkspace_MissingAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing action")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleManageWorkspace_UnknownAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "nuke"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for unknown action")
	}

	text := resultText(t, result)
	if text != "invalid action: nuke. Use: list, switch, create" {
		t.Errorf("error text = %q, want %q", text, "invalid action: nuke. Use: list, switch, create")
	}
}

func TestHandleManageWorkspace_SwitchSuccess(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listWorkspacesFn: func() ([]*cli.WorkspaceDTO, error) {
			return []*cli.WorkspaceDTO{
				{ID: "ws-1", Name: "default"},
				{ID: "ws-2", Name: "staging"},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "switch", "id": "ws-2"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["switched"] != true {
		t.Errorf("switched = %v, want true", parsed["switched"])
	}
	if parsed["workspace"] != "ws-2" {
		t.Errorf("workspace = %v, want ws-2", parsed["workspace"])
	}

	// Verify session workspace was updated
	if session.GetWorkspace() != "ws-2" {
		t.Errorf("session workspace = %s, want ws-2", session.GetWorkspace())
	}
}

func TestHandleManageWorkspace_SwitchNotFound(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listWorkspacesFn: func() ([]*cli.WorkspaceDTO, error) {
			return []*cli.WorkspaceDTO{
				{ID: "ws-1", Name: "default"},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "switch", "id": "ws-nonexistent"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for workspace not found")
	}

	text := resultText(t, result)
	if text != "workspace not found: ws-nonexistent" {
		t.Errorf("error text = %q, want %q", text, "workspace not found: ws-nonexistent")
	}
}

func TestHandleManageWorkspace_SwitchMissingID(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "switch"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing id")
	}

	text := resultText(t, result)
	if text != "id is required" {
		t.Errorf("error text = %q, want %q", text, "id is required")
	}
}

func TestHandleManageWorkspace_SwitchListFails(t *testing.T) {
	t.Parallel()

	// When ListWorkspaces fails, switch should still work (graceful degradation)
	client := &mockAdminClient{
		listWorkspacesFn: func() ([]*cli.WorkspaceDTO, error) {
			return nil, fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "switch", "id": "ws-3"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success (graceful degradation), got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["switched"] != true {
		t.Errorf("switched = %v, want true", parsed["switched"])
	}
	if parsed["workspace"] != "ws-3" {
		t.Errorf("workspace = %v, want ws-3", parsed["workspace"])
	}

	// Verify session workspace was updated despite list failure
	if session.GetWorkspace() != "ws-3" {
		t.Errorf("session workspace = %s, want ws-3", session.GetWorkspace())
	}
}

// =============================================================================
// handleCreateWorkspace Tests
// =============================================================================

func TestHandleManageWorkspace_CreateSuccess(t *testing.T) {
	t.Parallel()

	createdName := ""
	client := &mockAdminClient{
		createWorkspaceFn: func(name string) (*cli.WorkspaceResult, error) {
			createdName = name
			return &cli.WorkspaceResult{
				ID:   "ws-new-123",
				Name: name,
				Type: "local",
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "create", "name": "my-workspace"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if createdName != "my-workspace" {
		t.Errorf("created name = %s, want my-workspace", createdName)
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["created"] != true {
		t.Errorf("created = %v, want true", parsed["created"])
	}
	if parsed["id"] != "ws-new-123" {
		t.Errorf("id = %v, want ws-new-123", parsed["id"])
	}
	if parsed["name"] != "my-workspace" {
		t.Errorf("name = %v, want my-workspace", parsed["name"])
	}
	if parsed["type"] != "local" {
		t.Errorf("type = %v, want local", parsed["type"])
	}
}

func TestHandleManageWorkspace_CreateMissingName(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "create"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for create without name")
	}

	text := resultText(t, result)
	if text != "name is required" {
		t.Errorf("error text = %q, want %q", text, "name is required")
	}
}

func TestHandleManageWorkspace_CreateFailure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		createWorkspaceFn: func(name string) (*cli.WorkspaceResult, error) {
			return nil, fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "create", "name": "my-workspace"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for create failure")
	}

	text := resultText(t, result)
	if !strings.Contains(text, "failed to create workspace") {
		t.Errorf("error text = %q, want containing 'failed to create workspace'", text)
	}
}

func TestHandleManageWorkspace_CreateNoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{"action": "create", "name": "test"}
	result, err := handleManageWorkspace(args, session, server)
	if err != nil {
		t.Fatalf("handleManageWorkspace() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

// Ensure json import is used (by ContextResult unmarshaling above).
var _ = json.Unmarshal
