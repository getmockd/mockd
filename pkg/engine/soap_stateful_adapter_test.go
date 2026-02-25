package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/soap"
	"github.com/getmockd/mockd/pkg/stateful"
)

func TestSOAPStatefulAdapter_Create_Get_List_Delete(t *testing.T) {
	// Full lifecycle test: create a user via SOAP adapter, get it, list, delete it
	store := stateful.NewStateStore()
	if err := store.Register(&config.StatefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	}); err != nil {
		t.Fatalf("failed to register resource: %v", err)
	}

	bridge := stateful.NewBridge(store)
	adapter := newSOAPStatefulAdapter(bridge)
	ctx := context.Background()

	// CREATE
	createResult := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource: "users",
		Action:   soap.StatefulActionCreate,
		Data: map[string]interface{}{
			"name":  "Alice",
			"email": "alice@example.com",
		},
	})
	if createResult.Error != nil {
		t.Fatalf("create failed: %v", createResult.Error)
	}
	if !createResult.Success {
		t.Fatal("expected create to succeed")
	}
	if createResult.Item == nil {
		t.Fatal("expected item in create result")
	}
	userID, ok := createResult.Item["id"].(string)
	if !ok || userID == "" {
		t.Fatalf("expected non-empty string ID, got %v", createResult.Item["id"])
	}

	// GET
	getResult := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource:   "users",
		Action:     soap.StatefulActionGet,
		ResourceID: userID,
	})
	if getResult.Error != nil {
		t.Fatalf("get failed: %v", getResult.Error)
	}
	if getResult.Item["name"] != "Alice" {
		t.Errorf("expected name=Alice, got %v", getResult.Item["name"])
	}

	// LIST
	listResult := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource: "users",
		Action:   soap.StatefulActionList,
		Filter: &soap.StatefulFilter{
			Limit:  100,
			Offset: 0,
			Sort:   "createdAt",
			Order:  "desc",
		},
	})
	if listResult.Error != nil {
		t.Fatalf("list failed: %v", listResult.Error)
	}
	if len(listResult.Items) != 1 {
		t.Errorf("expected 1 item in list, got %d", len(listResult.Items))
	}
	if listResult.Meta == nil {
		t.Fatal("expected meta in list result")
	}
	if listResult.Meta.Total != 1 {
		t.Errorf("expected total=1, got %d", listResult.Meta.Total)
	}

	// DELETE
	deleteResult := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource:   "users",
		Action:     soap.StatefulActionDelete,
		ResourceID: userID,
	})
	if deleteResult.Error != nil {
		t.Fatalf("delete failed: %v", deleteResult.Error)
	}

	// Verify deleted
	getAfterDelete := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource:   "users",
		Action:     soap.StatefulActionGet,
		ResourceID: userID,
	})
	if getAfterDelete.Error == nil {
		t.Error("expected error after delete, got nil")
	}
	if getAfterDelete.Error.Code != "soap:Client" {
		t.Errorf("expected soap:Client fault, got %q", getAfterDelete.Error.Code)
	}
}

func TestSOAPStatefulAdapter_Update(t *testing.T) {
	store := stateful.NewStateStore()
	if err := store.Register(&config.StatefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	}); err != nil {
		t.Fatalf("failed to register resource: %v", err)
	}

	bridge := stateful.NewBridge(store)
	adapter := newSOAPStatefulAdapter(bridge)
	ctx := context.Background()

	// Create
	createResult := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource: "users",
		Action:   soap.StatefulActionCreate,
		Data: map[string]interface{}{
			"name": "Bob",
		},
	})
	userID := createResult.Item["id"].(string)

	// Update
	updateResult := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource:   "users",
		Action:     soap.StatefulActionUpdate,
		ResourceID: userID,
		Data: map[string]interface{}{
			"name":  "Bob Updated",
			"email": "bob@example.com",
		},
	})
	if updateResult.Error != nil {
		t.Fatalf("update failed: %v", updateResult.Error)
	}
	if updateResult.Item["name"] != "Bob Updated" {
		t.Errorf("expected name='Bob Updated', got %v", updateResult.Item["name"])
	}
}

func TestSOAPStatefulAdapter_Patch(t *testing.T) {
	store := stateful.NewStateStore()
	if err := store.Register(&config.StatefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	}); err != nil {
		t.Fatalf("failed to register resource: %v", err)
	}

	bridge := stateful.NewBridge(store)
	adapter := newSOAPStatefulAdapter(bridge)
	ctx := context.Background()

	// Create
	createResult := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource: "users",
		Action:   soap.StatefulActionCreate,
		Data: map[string]interface{}{
			"name":  "Charlie",
			"email": "charlie@example.com",
		},
	})
	userID := createResult.Item["id"].(string)

	// Patch (only update email, keep name)
	patchResult := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource:   "users",
		Action:     soap.StatefulActionPatch,
		ResourceID: userID,
		Data: map[string]interface{}{
			"email": "newemail@example.com",
		},
	})
	if patchResult.Error != nil {
		t.Fatalf("patch failed: %v", patchResult.Error)
	}
	if patchResult.Item["name"] != "Charlie" {
		t.Errorf("expected name to be preserved as 'Charlie', got %v", patchResult.Item["name"])
	}
	if patchResult.Item["email"] != "newemail@example.com" {
		t.Errorf("expected patched email, got %v", patchResult.Item["email"])
	}
}

func TestSOAPStatefulAdapter_NotFound_Error(t *testing.T) {
	store := stateful.NewStateStore()
	if err := store.Register(&config.StatefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	}); err != nil {
		t.Fatalf("failed to register resource: %v", err)
	}

	bridge := stateful.NewBridge(store)
	adapter := newSOAPStatefulAdapter(bridge)
	ctx := context.Background()

	result := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource:   "users",
		Action:     soap.StatefulActionGet,
		ResourceID: "nonexistent",
	})

	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if result.Error.Code != "soap:Client" {
		t.Errorf("expected soap:Client fault code, got %q", result.Error.Code)
	}
	if !strings.Contains(result.Error.Message, "not found") {
		t.Errorf("expected 'not found' in message, got %q", result.Error.Message)
	}
}

func TestSOAPStatefulAdapter_ResourceNotRegistered(t *testing.T) {
	store := stateful.NewStateStore()
	bridge := stateful.NewBridge(store)
	adapter := newSOAPStatefulAdapter(bridge)
	ctx := context.Background()

	result := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource:   "nonexistent",
		Action:     soap.StatefulActionGet,
		ResourceID: "1",
	})

	if result.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if result.Error.Code != "soap:Client" {
		t.Errorf("expected soap:Client fault code, got %q", result.Error.Code)
	}
}

func TestSOAPStatefulAdapter_Conflict_Error(t *testing.T) {
	store := stateful.NewStateStore()
	if err := store.Register(&config.StatefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	}); err != nil {
		t.Fatalf("failed to register resource: %v", err)
	}

	bridge := stateful.NewBridge(store)
	adapter := newSOAPStatefulAdapter(bridge)
	ctx := context.Background()

	// Create with explicit ID
	adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource:   "users",
		Action:     soap.StatefulActionCreate,
		ResourceID: "dup-id",
		Data: map[string]interface{}{
			"id":   "dup-id",
			"name": "First",
		},
	})

	// Try to create with same ID
	result := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource:   "users",
		Action:     soap.StatefulActionCreate,
		ResourceID: "dup-id",
		Data: map[string]interface{}{
			"id":   "dup-id",
			"name": "Duplicate",
		},
	})

	if result.Error == nil {
		t.Fatal("expected conflict error, got nil")
	}
	if result.Error.Code != "soap:Client" {
		t.Errorf("expected soap:Client fault code, got %q", result.Error.Code)
	}
	if !strings.Contains(result.Error.Message, "already exists") {
		t.Errorf("expected 'already exists' in message, got %q", result.Error.Message)
	}
}

func TestSOAPStatefulAdapter_NilBridge(t *testing.T) {
	adapter := newSOAPStatefulAdapter(nil)
	ctx := context.Background()

	result := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource: "users",
		Action:   soap.StatefulActionGet,
	})

	if result.Error == nil {
		t.Fatal("expected error for nil bridge, got nil")
	}
	if result.Error.Code != "soap:Server" {
		t.Errorf("expected soap:Server fault code, got %q", result.Error.Code)
	}
}

func TestSOAPStatefulAdapter_NilRequest(t *testing.T) {
	store := stateful.NewStateStore()
	bridge := stateful.NewBridge(store)
	adapter := newSOAPStatefulAdapter(bridge)
	ctx := context.Background()

	result := adapter.ExecuteStateful(ctx, nil)

	if result.Error == nil {
		t.Fatal("expected error for nil request, got nil")
	}
}

func TestSOAPStatefulAdapter_CustomOperation_FullStack(t *testing.T) {
	// CO-14: End-to-end test: SOAP request → adapter → Bridge → executor → response
	// Proves that a custom multi-step operation works through the full adapter stack.
	store := stateful.NewStateStore()
	if err := store.Register(&config.StatefulResourceConfig{
		Name:     "accounts",
		BasePath: "/api/accounts",
		SeedData: []map[string]interface{}{
			{"id": "acc-1", "name": "Alice", "balance": float64(1000)},
			{"id": "acc-2", "name": "Bob", "balance": float64(500)},
		},
	}); err != nil {
		t.Fatalf("failed to register resource: %v", err)
	}

	bridge := stateful.NewBridge(store)

	// Register the TransferFunds custom operation
	bridge.RegisterCustomOperation("TransferFunds", &stateful.CustomOperation{
		Name: "TransferFunds",
		Steps: []stateful.Step{
			{Type: stateful.StepRead, Resource: "accounts", ID: `input.sourceId`, As: "source"},
			{Type: stateful.StepRead, Resource: "accounts", ID: `input.destId`, As: "dest"},
			{Type: stateful.StepSet, Var: "newSourceBal", Value: `source.balance - input.amount`},
			{Type: stateful.StepSet, Var: "newDestBal", Value: `dest.balance + input.amount`},
			{Type: stateful.StepUpdate, Resource: "accounts", ID: `input.sourceId`, Set: map[string]string{
				"balance": "newSourceBal",
			}},
			{Type: stateful.StepUpdate, Resource: "accounts", ID: `input.destId`, Set: map[string]string{
				"balance": "newDestBal",
			}},
		},
		Response: map[string]string{
			"sourceBalance": "newSourceBal",
			"destBalance":   "newDestBal",
			"status":        `"completed"`,
		},
	})

	adapter := newSOAPStatefulAdapter(bridge)
	ctx := context.Background()

	// Execute custom operation via SOAP adapter
	result := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource: "TransferFunds",
		Action:   soap.StatefulActionCustom,
		Data: map[string]interface{}{
			"sourceId": "acc-1",
			"destId":   "acc-2",
			"amount":   float64(300),
		},
	})

	if result.Error != nil {
		t.Fatalf("custom operation failed: %v", result.Error)
	}
	if !result.Success {
		t.Fatal("expected success")
	}
	if result.Item == nil {
		t.Fatal("expected item in result")
	}

	// Verify response data
	if result.Item["sourceBalance"] != float64(700) {
		t.Errorf("expected sourceBalance=700, got %v", result.Item["sourceBalance"])
	}
	if result.Item["destBalance"] != float64(800) {
		t.Errorf("expected destBalance=800, got %v", result.Item["destBalance"])
	}
	if result.Item["status"] != "completed" {
		t.Errorf("expected status=completed, got %v", result.Item["status"])
	}

	// Verify actual resource state via direct store access
	source := store.Get("accounts").Get("acc-1")
	if source == nil {
		t.Fatal("source account not found")
	}
	if source.Data["balance"] != float64(700) {
		t.Errorf("expected source balance=700 in store, got %v", source.Data["balance"])
	}

	dest := store.Get("accounts").Get("acc-2")
	if dest == nil {
		t.Fatal("dest account not found")
	}
	if dest.Data["balance"] != float64(800) {
		t.Errorf("expected dest balance=800 in store, got %v", dest.Data["balance"])
	}
}

func TestSOAPStatefulAdapter_CustomOperation_NotRegistered(t *testing.T) {
	store := stateful.NewStateStore()
	bridge := stateful.NewBridge(store)
	adapter := newSOAPStatefulAdapter(bridge)
	ctx := context.Background()

	result := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource: "NonexistentOp",
		Action:   soap.StatefulActionCustom,
		Data:     map[string]interface{}{},
	})

	if result.Error == nil {
		t.Fatal("expected error for unregistered operation, got nil")
	}
	// The Bridge returns NotFound for unregistered custom ops → soap:Client fault
	if result.Error.Code != "soap:Client" {
		t.Errorf("expected soap:Client fault, got %q", result.Error.Code)
	}
	if !strings.Contains(result.Error.Message, "not found") {
		t.Errorf("expected 'not found' in message, got %q", result.Error.Message)
	}
}

func TestSOAPStatefulAdapter_SharedState_HTTP_And_SOAP(t *testing.T) {
	// SS-14: HTTP stateful + SOAP stateful see the same data (shared state)
	store := stateful.NewStateStore()
	if err := store.Register(&config.StatefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	}); err != nil {
		t.Fatalf("failed to register resource: %v", err)
	}

	// Get the raw resource to simulate HTTP handler behavior
	resource := store.Get("users")
	bridge := stateful.NewBridge(store)
	adapter := newSOAPStatefulAdapter(bridge)
	ctx := context.Background()

	// Create via "HTTP" (direct resource access, like handler_stateful.go does)
	item, err := resource.Create(map[string]interface{}{
		"name":  "CreatedViaHTTP",
		"email": "http@example.com",
	}, nil)
	if err != nil {
		t.Fatalf("HTTP create failed: %v", err)
	}

	// Read via SOAP adapter — should see the same item
	soapResult := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource:   "users",
		Action:     soap.StatefulActionGet,
		ResourceID: item.ID,
	})

	if soapResult.Error != nil {
		t.Fatalf("SOAP get failed: %v", soapResult.Error)
	}
	if soapResult.Item["name"] != "CreatedViaHTTP" {
		t.Errorf("expected SOAP to see HTTP-created item, got name=%v", soapResult.Item["name"])
	}

	// Create via SOAP
	soapCreate := adapter.ExecuteStateful(ctx, &soap.StatefulRequest{
		Resource: "users",
		Action:   soap.StatefulActionCreate,
		Data: map[string]interface{}{
			"name":  "CreatedViaSOAP",
			"email": "soap@example.com",
		},
	})
	soapUserID := soapCreate.Item["id"].(string)

	// Read via "HTTP" (direct resource access)
	httpItem := resource.Get(soapUserID)
	if httpItem == nil {
		t.Fatal("HTTP should see SOAP-created item")
	}
	if httpItem.Data["name"] != "CreatedViaSOAP" {
		t.Errorf("expected HTTP to see SOAP-created item name, got %v", httpItem.Data["name"])
	}

	// List should show both
	filter := stateful.DefaultQueryFilter()
	list := resource.List(filter)
	if list.Meta.Total != 2 {
		t.Errorf("expected 2 items total (1 HTTP + 1 SOAP), got %d", list.Meta.Total)
	}
}
