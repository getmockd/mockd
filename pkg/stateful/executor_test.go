package stateful

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupExecutorTest creates a store with "accounts" resource seeded with two accounts
// that have numeric balances, suitable for transfer-fund style tests.
func setupExecutorTest(t *testing.T) (*StateStore, *OperationExecutor) {
	t.Helper()
	store := NewStateStore()
	obs := NewMetricsObserver()
	store.SetObserver(obs)

	err := store.Register(&ResourceConfig{
		Name:     "accounts",
		BasePath: "/api/accounts",
		SeedData: []map[string]interface{}{
			{"id": "acc-1", "name": "Alice", "balance": float64(1000)},
			{"id": "acc-2", "name": "Bob", "balance": float64(500)},
		},
	})
	require.NoError(t, err)

	err = store.Register(&ResourceConfig{
		Name:     "logs",
		BasePath: "/api/logs",
	})
	require.NoError(t, err)

	executor := NewOperationExecutor(store)
	return store, executor
}

// --- CO-09: Simple set step with expr evaluation ---

func TestExecutor_SetStep_SimpleExpression(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "simple-set",
		Steps: []Step{
			{Type: StepSet, Var: "greeting", Value: `"Hello, " + input.name`},
		},
		Response: map[string]string{
			"message": "greeting",
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{"name": "World"},
	})

	require.Equal(t, StatusSuccess, result.Status)
	require.NotNil(t, result.Item)
	assert.Equal(t, "Hello, World", result.Item.Data["message"])
}

func TestExecutor_SetStep_ArithmeticExpression(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "arithmetic",
		Steps: []Step{
			{Type: StepSet, Var: "total", Value: `input.price * input.qty`},
			{Type: StepSet, Var: "withTax", Value: `total * 1.08`},
		},
		Response: map[string]string{
			"total":   "total",
			"withTax": "withTax",
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{"price": float64(100), "qty": float64(3)},
	})

	require.Equal(t, StatusSuccess, result.Status)
	assert.InDelta(t, float64(300), result.Item.Data["total"], 0.01)
	assert.InDelta(t, float64(324), result.Item.Data["withTax"], 0.01)
}

func TestExecutor_SetStep_MissingVar(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "missing-var",
		Steps: []Step{
			{Type: StepSet, Var: "", Value: `"test"`},
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error.Error(), "var name")
}

func TestExecutor_SetStep_MissingValue(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "missing-value",
		Steps: []Step{
			{Type: StepSet, Var: "x", Value: ""},
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error.Error(), "value expression")
}

// --- CO-10: Read step from resource ---

func TestExecutor_ReadStep_Success(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "read-account",
		Steps: []Step{
			{Type: StepRead, Resource: "accounts", ID: `input.accountId`, As: "account"},
		},
		Response: map[string]string{
			"name":    "account.name",
			"balance": "account.balance",
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{"accountId": "acc-1"},
	})

	require.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "Alice", result.Item.Data["name"])
	assert.Equal(t, float64(1000), result.Item.Data["balance"])
}

func TestExecutor_ReadStep_NotFound(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "read-missing",
		Steps: []Step{
			{Type: StepRead, Resource: "accounts", ID: `"nonexistent"`, As: "account"},
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error.Error(), "step 0")
}

func TestExecutor_ReadStep_MissingResource(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "read-no-resource",
		Steps: []Step{
			{Type: StepRead, Resource: "", ID: `"acc-1"`, As: "account"},
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error.Error(), "requires resource")
}

func TestExecutor_ReadStep_MissingAs(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "read-no-as",
		Steps: []Step{
			{Type: StepRead, Resource: "accounts", ID: `"acc-1"`, As: ""},
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error.Error(), "'as' variable name")
}

func TestExecutor_ReadStep_ResourceDoesNotExist(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "read-bad-resource",
		Steps: []Step{
			{Type: StepRead, Resource: "doesnotexist", ID: `"1"`, As: "item"},
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
}

// --- CO-11: Update step with expression ---

func TestExecutor_UpdateStep_Success(t *testing.T) {
	store, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "debit-account",
		Steps: []Step{
			{Type: StepRead, Resource: "accounts", ID: `input.accountId`, As: "account"},
			{Type: StepUpdate, Resource: "accounts", ID: `input.accountId`, Set: map[string]string{
				"balance": "account.balance - input.amount",
			}},
		},
		Response: map[string]string{
			"previousBalance": "account.balance",
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{
			"accountId": "acc-1",
			"amount":    float64(200),
		},
	})

	require.Equal(t, StatusSuccess, result.Status)
	// Response should show the balance BEFORE the update (from the read step)
	assert.Equal(t, float64(1000), result.Item.Data["previousBalance"])

	// Verify the actual resource was updated
	resource := store.Get("accounts")
	item := resource.Get("acc-1")
	require.NotNil(t, item)
	assert.Equal(t, float64(800), item.Data["balance"])
}

func TestExecutor_UpdateStep_WithAs(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "update-with-as",
		Steps: []Step{
			{Type: StepRead, Resource: "accounts", ID: `input.accountId`, As: "before"},
			{Type: StepUpdate, Resource: "accounts", ID: `input.accountId`, As: "after", Set: map[string]string{
				"balance": "before.balance + input.deposit",
			}},
		},
		Response: map[string]string{
			"beforeBalance": "before.balance",
			"afterBalance":  "after.balance",
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{
			"accountId": "acc-2",
			"deposit":   float64(250),
		},
	})

	require.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, float64(500), result.Item.Data["beforeBalance"])
	assert.Equal(t, float64(750), result.Item.Data["afterBalance"])
}

func TestExecutor_UpdateStep_NotFound(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "update-missing",
		Steps: []Step{
			{Type: StepUpdate, Resource: "accounts", ID: `"nonexistent"`, Set: map[string]string{
				"balance": "0",
			}},
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
}

// --- CO-12: Multi-step TransferFunds scenario ---

func TestExecutor_TransferFunds_Success(t *testing.T) {
	store, executor := setupExecutorTest(t)

	// Classic banking transfer: read both accounts, debit source, credit destination
	op := &CustomOperation{
		Name: "TransferFunds",
		Steps: []Step{
			// Step 0: Read source account
			{Type: StepRead, Resource: "accounts", ID: `input.sourceId`, As: "source"},
			// Step 1: Read destination account
			{Type: StepRead, Resource: "accounts", ID: `input.destId`, As: "dest"},
			// Step 2: Calculate new balances
			{Type: StepSet, Var: "newSourceBalance", Value: `source.balance - input.amount`},
			{Type: StepSet, Var: "newDestBalance", Value: `dest.balance + input.amount`},
			// Step 3: Update source (debit)
			{Type: StepUpdate, Resource: "accounts", ID: `input.sourceId`, Set: map[string]string{
				"balance": "newSourceBalance",
			}},
			// Step 4: Update destination (credit)
			{Type: StepUpdate, Resource: "accounts", ID: `input.destId`, Set: map[string]string{
				"balance": "newDestBalance",
			}},
			// Step 5: Create a transfer log entry
			{Type: StepCreate, Resource: "logs", Set: map[string]string{
				"type":   `"transfer"`,
				"from":   "input.sourceId",
				"to":     "input.destId",
				"amount": "input.amount",
			}, As: "logEntry"},
		},
		Response: map[string]string{
			"sourceBalance": "newSourceBalance",
			"destBalance":   "newDestBalance",
			"transferId":    "logEntry.id",
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{
			"sourceId": "acc-1",
			"destId":   "acc-2",
			"amount":   float64(300),
		},
	})

	require.Equal(t, StatusSuccess, result.Status)
	require.NotNil(t, result.Item)

	// Verify response
	assert.Equal(t, float64(700), result.Item.Data["sourceBalance"])
	assert.Equal(t, float64(800), result.Item.Data["destBalance"])
	assert.NotEmpty(t, result.Item.Data["transferId"])

	// Verify actual resource state
	source := store.Get("accounts").Get("acc-1")
	require.NotNil(t, source)
	assert.Equal(t, float64(700), source.Data["balance"])

	dest := store.Get("accounts").Get("acc-2")
	require.NotNil(t, dest)
	assert.Equal(t, float64(800), dest.Data["balance"])

	// Verify the log entry was created
	logs := store.Get("logs")
	logList := logs.List(DefaultQueryFilter())
	assert.Equal(t, 1, logList.Meta.Total)
}

func TestExecutor_TransferFunds_SourceNotFound(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "TransferFunds",
		Steps: []Step{
			{Type: StepRead, Resource: "accounts", ID: `input.sourceId`, As: "source"},
			{Type: StepRead, Resource: "accounts", ID: `input.destId`, As: "dest"},
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{
			"sourceId": "nonexistent",
			"destId":   "acc-2",
		},
	})

	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error.Error(), "step 0")
}

// --- Create step tests ---

func TestExecutor_CreateStep_Success(t *testing.T) {
	store, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "create-account",
		Steps: []Step{
			{Type: StepCreate, Resource: "accounts", As: "newAccount", Set: map[string]string{
				"name":    "input.name",
				"balance": "input.initialDeposit",
			}},
		},
		Response: map[string]string{
			"accountId": "newAccount.id",
			"name":      "newAccount.name",
			"balance":   "newAccount.balance",
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{
			"name":           "Charlie",
			"initialDeposit": float64(2500),
		},
	})

	require.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "Charlie", result.Item.Data["name"])
	assert.Equal(t, float64(2500), result.Item.Data["balance"])
	assert.NotEmpty(t, result.Item.Data["accountId"])

	// Verify in store
	accounts := store.Get("accounts")
	assert.Equal(t, 3, accounts.Count()) // 2 seed + 1 created
}

// --- Delete step tests ---

func TestExecutor_DeleteStep_Success(t *testing.T) {
	store, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "delete-account",
		Steps: []Step{
			{Type: StepRead, Resource: "accounts", ID: `input.accountId`, As: "account"},
			{Type: StepDelete, Resource: "accounts", ID: `input.accountId`},
		},
		Response: map[string]string{
			"deletedName": "account.name",
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{"accountId": "acc-2"},
	})

	require.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "Bob", result.Item.Data["deletedName"])

	// Verify deleted
	accounts := store.Get("accounts")
	assert.Equal(t, 1, accounts.Count()) // only acc-1 remains
	assert.Nil(t, accounts.Get("acc-2"))
}

func TestExecutor_DeleteStep_NotFound(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "delete-missing",
		Steps: []Step{
			{Type: StepDelete, Resource: "accounts", ID: `"nonexistent"`},
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
}

// --- CO-13: Error handling ---

func TestExecutor_NilOperation(t *testing.T) {
	_, executor := setupExecutorTest(t)

	result := executor.Execute(context.Background(), nil, &OperationRequest{
		Data: map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error.Error(), "must not be nil")
}

func TestExecutor_BadExpression(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "bad-expr",
		Steps: []Step{
			{Type: StepSet, Var: "x", Value: `completely.undefined.variable`},
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error.Error(), "step 0")
}

func TestExecutor_BadResponseExpression(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "bad-response-expr",
		Steps: []Step{
			{Type: StepSet, Var: "x", Value: `42`},
		},
		Response: map[string]string{
			"bad": "nonexistent.field",
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error.Error(), "response expression")
}

func TestExecutor_UnknownStepType(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "unknown-step",
		Steps: []Step{
			{Type: "bogus", Var: "x"},
		},
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
	assert.Contains(t, result.Error.Error(), "unknown step type")
}

func TestExecutor_NoResponseTemplate_ReturnsContext(t *testing.T) {
	_, executor := setupExecutorTest(t)

	op := &CustomOperation{
		Name: "no-response-template",
		Steps: []Step{
			{Type: StepSet, Var: "x", Value: `42`},
			{Type: StepSet, Var: "y", Value: `"hello"`},
		},
		// No Response template — should return full context minus input
	}

	result := executor.Execute(context.Background(), op, &OperationRequest{
		Data: map[string]interface{}{},
	})

	require.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, 42, result.Item.Data["x"])
	assert.Equal(t, "hello", result.Item.Data["y"])
	// Should NOT include "input" in the response
	_, hasInput := result.Item.Data["input"]
	assert.False(t, hasInput)
}

// --- Bridge.Execute with ActionCustom ---

func TestBridge_ExecuteCustom_Success(t *testing.T) {
	bridge, _ := setupBridgeTest(t)

	// Register a custom operation on the bridge
	bridge.RegisterCustomOperation("double-name", &CustomOperation{
		Name: "double-name",
		Steps: []Step{
			{Type: StepRead, Resource: "users", ID: `input.userId`, As: "user"},
			{Type: StepSet, Var: "doubled", Value: `user.name + " " + user.name`},
		},
		Response: map[string]string{
			"result": "doubled",
		},
	})

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "double-name", // operation name, not resource name
		Action:   ActionCustom,
		Data:     map[string]interface{}{"userId": "u1"},
	})

	require.Equal(t, StatusSuccess, result.Status)
	assert.Equal(t, "Alice Alice", result.Item.Data["result"])
}

func TestBridge_ExecuteCustom_OperationNotRegistered(t *testing.T) {
	bridge, obs := setupBridgeTest(t)

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "nonexistent-op",
		Action:   ActionCustom,
		Data:     map[string]interface{}{},
	})

	assert.Equal(t, StatusNotFound, result.Status)
	assert.Contains(t, result.Error.Error(), "not found")
	assert.Equal(t, int64(1), obs.Snapshot().ErrorCount)
}

func TestBridge_ExecuteCustom_StepFailure(t *testing.T) {
	bridge, obs := setupBridgeTest(t)

	bridge.RegisterCustomOperation("fail-op", &CustomOperation{
		Name: "fail-op",
		Steps: []Step{
			{Type: StepRead, Resource: "users", ID: `"nonexistent"`, As: "user"},
		},
	})

	result := bridge.Execute(context.Background(), &OperationRequest{
		Resource: "fail-op",
		Action:   ActionCustom,
		Data:     map[string]interface{}{},
	})

	assert.Equal(t, StatusError, result.Status)
	assert.Error(t, result.Error)
	assert.Equal(t, int64(1), obs.Snapshot().ErrorCount)
}
