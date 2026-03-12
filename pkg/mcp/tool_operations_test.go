package mcp

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/getmockd/mockd/pkg/cli"
)

// =============================================================================
// handleManageCustomOperation Tests
// =============================================================================

func TestHandleManageCustomOperation_List(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listCustomOperationsFn: func(_ string) ([]cli.CustomOperationInfo, error) {
			return []cli.CustomOperationInfo{
				{Name: "TransferFunds", StepCount: 3, Consistency: "atomic"},
				{Name: "CancelOrder", StepCount: 2},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	count, ok := parsed["count"].(float64)
	if !ok || count != 2 {
		t.Errorf("count = %v, want 2", parsed["count"])
	}

	ops, ok := parsed["operations"].([]interface{})
	if !ok {
		t.Fatal("expected 'operations' array in result")
	}
	if len(ops) != 2 {
		t.Fatalf("expected 2 operations, got %d", len(ops))
	}
}

func TestHandleManageCustomOperation_ListEmpty(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listCustomOperationsFn: func(_ string) ([]cli.CustomOperationInfo, error) {
			return []cli.CustomOperationInfo{}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	count, ok := parsed["count"].(float64)
	if !ok || count != 0 {
		t.Errorf("count = %v, want 0", parsed["count"])
	}
}

func TestHandleManageCustomOperation_ListFailure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		listCustomOperationsFn: func(_ string) ([]cli.CustomOperationInfo, error) {
			return nil, fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "list"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for list failure")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleManageCustomOperation_Get(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getCustomOperationFn: func(_ string, name string) (*cli.CustomOperationDetail, error) {
			if name == "TransferFunds" {
				return &cli.CustomOperationDetail{
					Name:        "TransferFunds",
					Consistency: "atomic",
					Steps: []cli.CustomOperationStep{
						{Type: "lookup", Resource: "accounts", As: "src"},
						{Type: "lookup", Resource: "accounts", As: "dst"},
						{Type: "set", Resource: "accounts", Set: map[string]string{"balance": "{{src.balance - input.amount}}"}},
					},
				}, nil
			}
			return nil, &cli.APIError{StatusCode: 404, Message: "operation not found: " + name}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "get", "name": "TransferFunds"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var detail cli.CustomOperationDetail
	resultJSON(t, result, &detail)

	if detail.Name != "TransferFunds" {
		t.Errorf("Name = %s, want TransferFunds", detail.Name)
	}
	if len(detail.Steps) != 3 {
		t.Errorf("Steps count = %d, want 3", len(detail.Steps))
	}
}

func TestHandleManageCustomOperation_GetMissingName(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "get"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing name")
	}

	text := resultText(t, result)
	if text != "name is required for action=get" {
		t.Errorf("error text = %q, want %q", text, "name is required for action=get")
	}
}

func TestHandleManageCustomOperation_GetNotFound(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getCustomOperationFn: func(_ string, name string) (*cli.CustomOperationDetail, error) {
			return nil, &cli.APIError{StatusCode: 404, Message: "operation not found: " + name}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "get", "name": "Nonexistent"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for not-found operation")
	}

	text := resultText(t, result)
	if text != "operation not found: Nonexistent" {
		t.Errorf("error text = %q, want %q", text, "operation not found: Nonexistent")
	}
}

func TestHandleManageCustomOperation_Register(t *testing.T) {
	t.Parallel()

	var registeredDef map[string]interface{}
	client := &mockAdminClient{
		registerCustomOpFn: func(_ string, definition map[string]interface{}) error {
			registeredDef = definition
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	definition := map[string]interface{}{
		"name": "TransferFunds",
		"steps": []interface{}{
			map[string]interface{}{"type": "lookup", "resource": "accounts"},
		},
	}

	args := map[string]interface{}{"action": "register", "definition": definition}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["registered"] != true {
		t.Errorf("registered = %v, want true", parsed["registered"])
	}

	if registeredDef == nil {
		t.Fatal("expected definition to be passed to RegisterCustomOperation")
	}
	if registeredDef["name"] != "TransferFunds" {
		t.Errorf("registered name = %v, want TransferFunds", registeredDef["name"])
	}
}

func TestHandleManageCustomOperation_RegisterMissingDefinition(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "register"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing definition")
	}

	text := resultText(t, result)
	if text != "definition is required for action=register" {
		t.Errorf("error text = %q, want %q", text, "definition is required for action=register")
	}
}

func TestHandleManageCustomOperation_RegisterFailure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		registerCustomOpFn: func(_ string, definition map[string]interface{}) error {
			return fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	definition := map[string]interface{}{
		"name": "BadOp",
	}

	args := map[string]interface{}{"action": "register", "definition": definition}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for register failure")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleManageCustomOperation_Delete(t *testing.T) {
	t.Parallel()

	deletedName := ""
	client := &mockAdminClient{
		deleteCustomOpFn: func(_ string, name string) error {
			deletedName = name
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "delete", "name": "TransferFunds"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["deleted"] != true {
		t.Errorf("deleted = %v, want true", parsed["deleted"])
	}
	if parsed["name"] != "TransferFunds" {
		t.Errorf("name = %v, want TransferFunds", parsed["name"])
	}
	if deletedName != "TransferFunds" {
		t.Errorf("deletedName = %s, want TransferFunds", deletedName)
	}
}

func TestHandleManageCustomOperation_DeleteMissingName(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "delete"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing name")
	}

	text := resultText(t, result)
	if text != "name is required for action=delete" {
		t.Errorf("error text = %q, want %q", text, "name is required for action=delete")
	}
}

func TestHandleManageCustomOperation_DeleteNotFound(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		deleteCustomOpFn: func(_ string, name string) error {
			return &cli.APIError{StatusCode: 404, Message: "operation not found: " + name}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "delete", "name": "Nonexistent"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for delete not-found")
	}

	text := resultText(t, result)
	if text != "operation not found: Nonexistent" {
		t.Errorf("error text = %q, want %q", text, "operation not found: Nonexistent")
	}
}

func TestHandleManageCustomOperation_Execute(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		executeCustomOpFn: func(_ string, name string, input map[string]interface{}) (map[string]interface{}, error) {
			return map[string]interface{}{
				"success":       true,
				"operationName": name,
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "execute", "name": "TransferFunds"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["success"] != true {
		t.Errorf("success = %v, want true", parsed["success"])
	}
	if parsed["operationName"] != "TransferFunds" {
		t.Errorf("operationName = %v, want TransferFunds", parsed["operationName"])
	}
}

func TestHandleManageCustomOperation_ExecuteWithInput(t *testing.T) {
	t.Parallel()

	var capturedInput map[string]interface{}
	client := &mockAdminClient{
		executeCustomOpFn: func(_ string, name string, input map[string]interface{}) (map[string]interface{}, error) {
			capturedInput = input
			return map[string]interface{}{
				"success": true,
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	input := map[string]interface{}{
		"fromAccount": "acc-1",
		"toAccount":   "acc-2",
		"amount":      float64(100),
	}

	args := map[string]interface{}{"action": "execute", "name": "TransferFunds", "input": input}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if capturedInput == nil {
		t.Fatal("expected input to be passed to ExecuteCustomOperation")
	}
	if capturedInput["fromAccount"] != "acc-1" {
		t.Errorf("input.fromAccount = %v, want acc-1", capturedInput["fromAccount"])
	}
	if capturedInput["amount"] != float64(100) {
		t.Errorf("input.amount = %v, want 100", capturedInput["amount"])
	}
}

func TestHandleManageCustomOperation_ExecuteMissingName(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "execute"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing name")
	}

	text := resultText(t, result)
	if text != "name is required for action=execute" {
		t.Errorf("error text = %q, want %q", text, "name is required for action=execute")
	}
}

func TestHandleManageCustomOperation_ExecuteNotFound(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		executeCustomOpFn: func(_ string, name string, input map[string]interface{}) (map[string]interface{}, error) {
			return nil, &cli.APIError{StatusCode: 404, Message: "operation not found: " + name}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "execute", "name": "Nonexistent"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for execute not-found")
	}

	text := resultText(t, result)
	if text != "operation not found: Nonexistent" {
		t.Errorf("error text = %q, want %q", text, "operation not found: Nonexistent")
	}
}

func TestHandleManageCustomOperation_MissingAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing action")
	}

	text := resultText(t, result)
	if text != "action is required (list, get, register, delete, execute)" {
		t.Errorf("error text = %q, want %q", text, "action is required (list, get, register, delete, execute)")
	}
}

func TestHandleManageCustomOperation_UnknownAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "explode"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for unknown action")
	}

	text := resultText(t, result)
	if text != "unknown action: explode. Use list, get, register, delete, or execute" {
		t.Errorf("error text = %q, want %q", text, "unknown action: explode. Use list, get, register, delete, or execute")
	}
}

func TestHandleManageCustomOperation_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{"action": "list"}
	result, err := handleManageCustomOperation(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCustomOperation() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

// Ensure imports are used.
var _ = json.Unmarshal
var _ = fmt.Sprintf
