package stateful

import (
	"context"
	"fmt"
	"time"

	"github.com/expr-lang/expr"
)

// CustomOperation defines a multi-step operation composed of reads, writes,
// and expression-evaluated transforms against stateful resources.
// This enables complex mock scenarios like "TransferFunds" that read two accounts,
// validate a condition, and update both balances atomically.
type CustomOperation struct {
	// Name is a human-readable identifier for the operation.
	Name string `json:"name" yaml:"name"`

	// Steps is the ordered sequence of steps to execute.
	// Steps run sequentially; each step can reference variables set by prior steps.
	Steps []Step `json:"steps" yaml:"steps"`

	// Response is a map of expressions that build the final response.
	// Each key becomes a field in the result map; each value is an expr expression
	// evaluated against the accumulated context (input + step variables).
	// Example: {"newBalance": "source.balance - input.amount"}
	Response map[string]string `json:"response,omitempty" yaml:"response,omitempty"`
}

// StepType identifies what kind of step to execute.
type StepType string

const (
	// StepRead reads a single item from a resource and stores it in a named variable.
	StepRead StepType = "read"
	// StepUpdate updates a resource item using expression-evaluated field values.
	StepUpdate StepType = "update"
	// StepDelete deletes a resource item.
	StepDelete StepType = "delete"
	// StepCreate creates a new resource item using expression-evaluated field values.
	StepCreate StepType = "create"
	// StepSet sets a context variable to the result of an expression.
	StepSet StepType = "set"
)

// Step is a single step in a custom operation pipeline.
type Step struct {
	// Type is the kind of step (read, update, delete, create, set).
	Type StepType `json:"type" yaml:"type"`

	// Resource is the stateful resource name (for read/update/delete/create steps).
	Resource string `json:"resource,omitempty" yaml:"resource,omitempty"`

	// ID is an expr expression that resolves to the item ID (for read/update/delete).
	// Example: "input.sourceAccountId"
	ID string `json:"id,omitempty" yaml:"id,omitempty"`

	// As is the variable name to store the result under (for read and create steps).
	// Example: "source" — makes the item accessible as `source.balance` in later expressions.
	As string `json:"as,omitempty" yaml:"as,omitempty"`

	// Set is a map of field name → expr expression for update/create steps.
	// Example: {"balance": "source.balance - input.amount"}
	Set map[string]string `json:"set,omitempty" yaml:"set,omitempty"`

	// Var is the variable name (for set steps).
	Var string `json:"var,omitempty" yaml:"var,omitempty"`

	// Value is an expr expression (for set steps).
	// Example: "source.balance - input.amount"
	Value string `json:"value,omitempty" yaml:"value,omitempty"`
}

// OperationExecutor executes custom multi-step operations against stateful resources.
// It maintains an expression context that accumulates variables from each step,
// allowing later steps to reference results from earlier ones.
type OperationExecutor struct {
	store *StateStore
}

// NewOperationExecutor creates a new executor backed by the given store.
func NewOperationExecutor(store *StateStore) *OperationExecutor {
	return &OperationExecutor{store: store}
}

// Execute runs a custom operation against the store.
//
// The execution model:
//  1. Start with context: {"input": req.Data} (the incoming request payload)
//  2. For each step, execute it and add its result to the context
//  3. After all steps, evaluate the Response expressions against the final context
//  4. Return the evaluated response as the operation result
//
// If any step fails, execution stops and the error is returned.
// There is no automatic rollback — partial state changes persist.
// This is intentional for a mock server where partial state is acceptable.
func (e *OperationExecutor) Execute(ctx context.Context, op *CustomOperation, req *OperationRequest) *OperationResult {
	if op == nil {
		return &OperationResult{
			Status: StatusError,
			Error:  fmt.Errorf("custom operation must not be nil"),
		}
	}

	// Build initial expression context
	exprCtx := map[string]interface{}{
		"input": req.Data,
	}

	// Execute each step
	for i, step := range op.Steps {
		if err := e.executeStep(ctx, step, exprCtx); err != nil {
			return &OperationResult{
				Status: StatusError,
				Error:  fmt.Errorf("step %d (%s) failed: %w", i, step.Type, err),
			}
		}
	}

	// Build response by evaluating response expressions
	responseData := make(map[string]interface{})

	if len(op.Response) > 0 {
		for key, exprStr := range op.Response {
			val, err := evalExpr(exprStr, exprCtx)
			if err != nil {
				return &OperationResult{
					Status: StatusError,
					Error:  fmt.Errorf("response expression %q failed: %w", key, err),
				}
			}
			responseData[key] = val
		}
	} else {
		// No response template — return the full context (minus input)
		for k, v := range exprCtx {
			if k != "input" {
				responseData[k] = v
			}
		}
	}

	return &OperationResult{
		Status: StatusSuccess,
		Item: &ResourceItem{
			ID:        "custom",
			Data:      responseData,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

// executeStep executes a single step, updating the expression context.
func (e *OperationExecutor) executeStep(_ context.Context, step Step, exprCtx map[string]interface{}) error {
	switch step.Type {
	case StepRead:
		return e.stepRead(step, exprCtx)
	case StepUpdate:
		return e.stepUpdate(step, exprCtx)
	case StepDelete:
		return e.stepDelete(step, exprCtx)
	case StepCreate:
		return e.stepCreate(step, exprCtx)
	case StepSet:
		return e.stepSet(step, exprCtx)
	default:
		return fmt.Errorf("unknown step type: %s", step.Type)
	}
}

// stepRead reads an item from a resource and stores it in the context.
func (e *OperationExecutor) stepRead(step Step, exprCtx map[string]interface{}) error {
	if step.Resource == "" {
		return fmt.Errorf("read step requires resource")
	}
	if step.ID == "" {
		return fmt.Errorf("read step requires id expression")
	}
	if step.As == "" {
		return fmt.Errorf("read step requires 'as' variable name")
	}

	// Evaluate ID expression
	idVal, err := evalExpr(step.ID, exprCtx)
	if err != nil {
		return fmt.Errorf("id expression failed: %w", err)
	}
	itemID := fmt.Sprintf("%v", idVal)

	// Look up resource
	resource := e.store.Get(step.Resource)
	if resource == nil {
		return &NotFoundError{Resource: step.Resource}
	}

	// Get item
	item := resource.Get(itemID)
	if item == nil {
		return &NotFoundError{Resource: step.Resource, ID: itemID}
	}

	// Store in context as a flat map (id + data fields)
	exprCtx[step.As] = item.ToJSON()
	return nil
}

// stepUpdate updates a resource item with expression-evaluated field values.
func (e *OperationExecutor) stepUpdate(step Step, exprCtx map[string]interface{}) error {
	if step.Resource == "" {
		return fmt.Errorf("update step requires resource")
	}
	if step.ID == "" {
		return fmt.Errorf("update step requires id expression")
	}

	// Evaluate ID expression
	idVal, err := evalExpr(step.ID, exprCtx)
	if err != nil {
		return fmt.Errorf("id expression failed: %w", err)
	}
	itemID := fmt.Sprintf("%v", idVal)

	// Look up resource
	resource := e.store.Get(step.Resource)
	if resource == nil {
		return &NotFoundError{Resource: step.Resource}
	}

	// Evaluate each field expression in Set
	updateData := make(map[string]interface{})
	for field, exprStr := range step.Set {
		val, err := evalExpr(exprStr, exprCtx)
		if err != nil {
			return fmt.Errorf("field %q expression failed: %w", field, err)
		}
		updateData[field] = val
	}

	// Patch the item (partial update — preserves existing fields)
	_, err = resource.Patch(itemID, updateData)
	if err != nil {
		return err
	}

	// If an 'as' variable is specified, re-read and store the updated item
	if step.As != "" {
		updated := resource.Get(itemID)
		if updated != nil {
			exprCtx[step.As] = updated.ToJSON()
		}
	}

	return nil
}

// stepDelete deletes a resource item.
func (e *OperationExecutor) stepDelete(step Step, exprCtx map[string]interface{}) error {
	if step.Resource == "" {
		return fmt.Errorf("delete step requires resource")
	}
	if step.ID == "" {
		return fmt.Errorf("delete step requires id expression")
	}

	// Evaluate ID expression
	idVal, err := evalExpr(step.ID, exprCtx)
	if err != nil {
		return fmt.Errorf("id expression failed: %w", err)
	}
	itemID := fmt.Sprintf("%v", idVal)

	// Look up resource
	resource := e.store.Get(step.Resource)
	if resource == nil {
		return &NotFoundError{Resource: step.Resource}
	}

	return resource.Delete(itemID)
}

// stepCreate creates a new resource item with expression-evaluated field values.
func (e *OperationExecutor) stepCreate(step Step, exprCtx map[string]interface{}) error {
	if step.Resource == "" {
		return fmt.Errorf("create step requires resource")
	}

	// Look up resource
	resource := e.store.Get(step.Resource)
	if resource == nil {
		return &NotFoundError{Resource: step.Resource}
	}

	// Evaluate each field expression in Set
	createData := make(map[string]interface{})
	for field, exprStr := range step.Set {
		val, err := evalExpr(exprStr, exprCtx)
		if err != nil {
			return fmt.Errorf("field %q expression failed: %w", field, err)
		}
		createData[field] = val
	}

	// Create the item
	item, err := resource.Create(createData, nil)
	if err != nil {
		return err
	}

	// If an 'as' variable is specified, store the created item
	if step.As != "" {
		exprCtx[step.As] = item.ToJSON()
	}

	return nil
}

// stepSet evaluates an expression and stores the result in a context variable.
func (e *OperationExecutor) stepSet(step Step, exprCtx map[string]interface{}) error {
	if step.Var == "" {
		return fmt.Errorf("set step requires var name")
	}
	if step.Value == "" {
		return fmt.Errorf("set step requires value expression")
	}

	val, err := evalExpr(step.Value, exprCtx)
	if err != nil {
		return fmt.Errorf("value expression failed: %w", err)
	}

	exprCtx[step.Var] = val
	return nil
}

// evalExpr evaluates an expr-lang expression against the given context.
func evalExpr(expression string, env map[string]interface{}) (interface{}, error) {
	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, fmt.Errorf("compile %q: %w", expression, err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("eval %q: %w", expression, err)
	}

	return result, nil
}
