package stateful

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/getmockd/mockd/pkg/tracing"
)

// CustomOperation defines a multi-step operation composed of reads, writes,
// and expression-evaluated transforms against stateful resources.
// This enables complex mock scenarios like "TransferFunds" that read two accounts,
// validate a condition, and update both balances atomically.
type CustomOperation struct {
	// Name is a human-readable identifier for the operation.
	Name string `json:"name" yaml:"name"`

	// Consistency controls execution semantics: "best_effort" (default) or "atomic".
	// Atomic mode rolls back prior mutations in this operation if a later step fails.
	Consistency ConsistencyMode `json:"consistency,omitempty" yaml:"consistency,omitempty"`

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

// ConsistencyMode controls rollback behavior for multi-step custom operations.
type ConsistencyMode string

const (
	// ConsistencyBestEffort applies step mutations immediately; failures do not roll back prior steps.
	ConsistencyBestEffort ConsistencyMode = "best_effort"
	// ConsistencyAtomic rolls back prior mutations in the custom operation when a later step fails.
	ConsistencyAtomic ConsistencyMode = "atomic"
)

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
	store        *StateStore
	tracer       *tracing.Tracer
	programMu    sync.RWMutex
	programCache map[string]*vm.Program
}

// NewOperationExecutor creates a new executor backed by the given store.
func NewOperationExecutor(store *StateStore) *OperationExecutor {
	return &OperationExecutor{
		store:        store,
		programCache: make(map[string]*vm.Program),
	}
}

// SetTracer configures an optional tracer for custom-operation spans.
func (e *OperationExecutor) SetTracer(t *tracing.Tracer) {
	e.tracer = t
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
// In "best_effort" mode (default), partial state changes persist.
// In "atomic" mode, prior mutations are rolled back on failure.
//
// Note: Atomic provides rollback-on-failure within a single operation.
// It does NOT provide isolation across concurrent requests — other
// requests may observe intermediate state during execution.
func (e *OperationExecutor) Execute(ctx context.Context, op *CustomOperation, req *OperationRequest) *OperationResult {
	if op == nil {
		return &OperationResult{
			Status: StatusError,
			Error:  errors.New("custom operation must not be nil"),
		}
	}

	consistency, err := normalizeConsistencyMode(op.Consistency)
	if err != nil {
		return &OperationResult{
			Status: StatusValidationError,
			Error:  &ValidationError{Field: "consistency", Message: err.Error()},
		}
	}

	var execSpan *tracing.Span
	if e.tracer != nil {
		ctx, execSpan = e.tracer.Start(ctx, "stateful.custom_operation.execute")
		execSpan.SetKind(tracing.SpanKindInternal)
		execSpan.SetAttribute("stateful.custom_operation.name", op.Name)
		execSpan.SetAttribute("stateful.custom_operation.consistency", string(consistency))
		execSpan.SetAttribute("stateful.custom_operation.step_count", strconv.Itoa(len(op.Steps)))
		defer execSpan.End()
	}

	// Build initial expression context
	exprCtx := map[string]interface{}{
		"input": req.Data,
	}

	var tx *rollbackJournal
	if consistency == ConsistencyAtomic {
		tx = newRollbackJournal()
		if execSpan != nil {
			execSpan.AddEvent("atomic_mode_enabled")
		}
	}

	// Execute each step
	for i, step := range op.Steps {
		if err := e.executeStep(ctx, i, step, exprCtx, tx); err != nil {
			if tx != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					err = fmt.Errorf("%w (rollback failed: %v)", err, rbErr)
					if execSpan != nil {
						execSpan.AddEvent("rollback_failed", "error", rbErr.Error())
					}
				} else if execSpan != nil {
					execSpan.AddEvent("rolled_back")
				}
			}
			if execSpan != nil {
				execSpan.SetStatus(tracing.StatusError, err.Error())
			}
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
			val, err := e.evalExpr(exprStr, exprCtx)
			if err != nil {
				if execSpan != nil {
					execSpan.SetStatus(tracing.StatusError, err.Error())
				}
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

	if execSpan != nil {
		execSpan.SetStatus(tracing.StatusOK, "")
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
func (e *OperationExecutor) executeStep(ctx context.Context, stepIndex int, step Step, exprCtx map[string]interface{}, tx *rollbackJournal) error {
	var stepSpan *tracing.Span
	if e.tracer != nil {
		_, stepSpan = e.tracer.Start(ctx, "stateful.custom_operation.step")
		stepSpan.SetKind(tracing.SpanKindInternal)
		stepSpan.SetAttribute("stateful.custom_operation.step.index", strconv.Itoa(stepIndex))
		stepSpan.SetAttribute("stateful.custom_operation.step.type", string(step.Type))
		if step.Resource != "" {
			stepSpan.SetAttribute("stateful.custom_operation.step.resource", step.Resource)
		}
		defer stepSpan.End()
	}

	var err error
	switch step.Type {
	case StepRead:
		err = e.stepRead(step, exprCtx)
	case StepUpdate:
		err = e.stepUpdate(step, exprCtx, tx)
	case StepDelete:
		err = e.stepDelete(step, exprCtx, tx)
	case StepCreate:
		err = e.stepCreate(step, exprCtx, tx)
	case StepSet:
		err = e.stepSet(step, exprCtx)
	default:
		err = fmt.Errorf("unknown step type: %s", step.Type)
	}

	if stepSpan != nil {
		if err != nil {
			stepSpan.SetStatus(tracing.StatusError, err.Error())
			stepSpan.AddEvent("step_failed", "error", err.Error())
		} else {
			stepSpan.SetStatus(tracing.StatusOK, "")
		}
	}
	return err
}

// stepRead reads an item from a resource and stores it in the context.
func (e *OperationExecutor) stepRead(step Step, exprCtx map[string]interface{}) error {
	if step.Resource == "" {
		return errors.New("read step requires resource")
	}
	if step.ID == "" {
		return errors.New("read step requires id expression")
	}
	if step.As == "" {
		return errors.New("read step requires 'as' variable name")
	}

	// Evaluate ID expression
	idVal, err := e.evalExpr(step.ID, exprCtx)
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
func (e *OperationExecutor) stepUpdate(step Step, exprCtx map[string]interface{}, tx *rollbackJournal) error {
	if step.Resource == "" {
		return errors.New("update step requires resource")
	}
	if step.ID == "" {
		return errors.New("update step requires id expression")
	}

	// Evaluate ID expression
	idVal, err := e.evalExpr(step.ID, exprCtx)
	if err != nil {
		return fmt.Errorf("id expression failed: %w", err)
	}
	itemID := fmt.Sprintf("%v", idVal)

	// Look up resource
	resource := e.store.Get(step.Resource)
	if resource == nil {
		return &NotFoundError{Resource: step.Resource}
	}
	if tx != nil {
		tx.RecordBefore(resource, itemID)
	}

	// Evaluate each field expression in Set
	updateData := make(map[string]interface{})
	for field, exprStr := range step.Set {
		val, err := e.evalExpr(exprStr, exprCtx)
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
func (e *OperationExecutor) stepDelete(step Step, exprCtx map[string]interface{}, tx *rollbackJournal) error {
	if step.Resource == "" {
		return errors.New("delete step requires resource")
	}
	if step.ID == "" {
		return errors.New("delete step requires id expression")
	}

	// Evaluate ID expression
	idVal, err := e.evalExpr(step.ID, exprCtx)
	if err != nil {
		return fmt.Errorf("id expression failed: %w", err)
	}
	itemID := fmt.Sprintf("%v", idVal)

	// Look up resource
	resource := e.store.Get(step.Resource)
	if resource == nil {
		return &NotFoundError{Resource: step.Resource}
	}
	if tx != nil {
		tx.RecordBefore(resource, itemID)
	}

	return resource.Delete(itemID)
}

// stepCreate creates a new resource item with expression-evaluated field values.
func (e *OperationExecutor) stepCreate(step Step, exprCtx map[string]interface{}, tx *rollbackJournal) error {
	if step.Resource == "" {
		return errors.New("create step requires resource")
	}

	// Look up resource
	resource := e.store.Get(step.Resource)
	if resource == nil {
		return &NotFoundError{Resource: step.Resource}
	}

	// Evaluate each field expression in Set
	createData := make(map[string]interface{})
	for field, exprStr := range step.Set {
		val, err := e.evalExpr(exprStr, exprCtx)
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
	if tx != nil {
		tx.RecordCreate(resource, item.ID)
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
		return errors.New("set step requires var name")
	}
	if step.Value == "" {
		return errors.New("set step requires value expression")
	}

	val, err := e.evalExpr(step.Value, exprCtx)
	if err != nil {
		return fmt.Errorf("value expression failed: %w", err)
	}

	exprCtx[step.Var] = val
	return nil
}

// evalExpr evaluates an expr-lang expression against the given context, using a compile cache.
func (e *OperationExecutor) evalExpr(expression string, env map[string]interface{}) (interface{}, error) {
	program, err := e.compileExpr(expression, env)
	if err != nil {
		return nil, fmt.Errorf("compile %q: %w", expression, err)
	}

	result, err := expr.Run(program, env)
	if err != nil {
		return nil, fmt.Errorf("eval %q: %w", expression, err)
	}

	return result, nil
}

func (e *OperationExecutor) compileExpr(expression string, env map[string]interface{}) (*vm.Program, error) {
	cacheKey := expression + "\x00" + exprEnvSignature(env)

	e.programMu.RLock()
	if program, ok := e.programCache[cacheKey]; ok {
		e.programMu.RUnlock()
		return program, nil
	}
	e.programMu.RUnlock()

	program, err := expr.Compile(expression, expr.Env(env))
	if err != nil {
		return nil, err
	}

	e.programMu.Lock()
	// Double-check in case another goroutine compiled the same expression/signature.
	if existing, ok := e.programCache[cacheKey]; ok {
		e.programMu.Unlock()
		return existing, nil
	}
	e.programCache[cacheKey] = program
	e.programMu.Unlock()

	return program, nil
}

func exprEnvSignature(env map[string]interface{}) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+":"+fmt.Sprintf("%T", env[k]))
	}
	return strings.Join(parts, ",")
}

func normalizeConsistencyMode(mode ConsistencyMode) (ConsistencyMode, error) {
	switch ConsistencyMode(strings.TrimSpace(string(mode))) {
	case "":
		return ConsistencyBestEffort, nil
	case ConsistencyBestEffort:
		return ConsistencyBestEffort, nil
	case ConsistencyAtomic:
		return ConsistencyAtomic, nil
	default:
		return "", fmt.Errorf("unsupported consistency mode %q (valid: %s, %s)", mode, ConsistencyBestEffort, ConsistencyAtomic)
	}
}

// NormalizeCustomOperation validates and normalizes custom-operation settings in place.
// Currently this normalizes Consistency to the canonical default/value.
func NormalizeCustomOperation(op *CustomOperation) (ConsistencyMode, error) {
	if op == nil {
		return "", errors.New("custom operation must not be nil")
	}
	mode, err := normalizeConsistencyMode(op.Consistency)
	if err != nil {
		return "", err
	}
	op.Consistency = mode
	return mode, nil
}

// CustomOperationConsistency resolves the effective consistency mode without mutating the operation.
func CustomOperationConsistency(op *CustomOperation) (ConsistencyMode, error) {
	if op == nil {
		return "", errors.New("custom operation must not be nil")
	}
	return normalizeConsistencyMode(op.Consistency)
}

type rollbackJournal struct {
	entries map[string]rollbackEntry
	order   []string
}

type rollbackEntry struct {
	resource *StatefulResource
	id       string
	before   *ResourceItem
}

func newRollbackJournal() *rollbackJournal {
	return &rollbackJournal{
		entries: make(map[string]rollbackEntry),
		order:   make([]string, 0),
	}
}

func (j *rollbackJournal) key(resource *StatefulResource, id string) string {
	return resource.Name() + "\x00" + id
}

// RecordBefore snapshots the original item state before the first mutation of that item in the operation.
func (j *rollbackJournal) RecordBefore(resource *StatefulResource, id string) {
	if resource == nil || id == "" {
		return
	}
	key := j.key(resource, id)
	if _, exists := j.entries[key]; exists {
		return
	}
	j.entries[key] = rollbackEntry{
		resource: resource,
		id:       id,
		before:   cloneResourceItem(resource.Get(id)),
	}
	j.order = append(j.order, key)
}

// RecordCreate records that an item did not exist before this operation and should be deleted on rollback.
func (j *rollbackJournal) RecordCreate(resource *StatefulResource, id string) {
	if resource == nil || id == "" {
		return
	}
	key := j.key(resource, id)
	if _, exists := j.entries[key]; exists {
		return
	}
	j.entries[key] = rollbackEntry{
		resource: resource,
		id:       id,
		before:   nil,
	}
	j.order = append(j.order, key)
}

func (j *rollbackJournal) Rollback() error {
	for i := len(j.order) - 1; i >= 0; i-- {
		entry := j.entries[j.order[i]]
		if err := restoreResourceItem(entry.resource, entry.id, entry.before); err != nil {
			return err
		}
	}
	return nil
}

func restoreResourceItem(resource *StatefulResource, id string, before *ResourceItem) error {
	if resource == nil || id == "" {
		return nil
	}
	resource.mu.Lock()
	defer resource.mu.Unlock()

	if before == nil {
		delete(resource.items, id)
		return nil
	}
	resource.items[id] = cloneResourceItem(before)
	return nil
}

func cloneResourceItem(item *ResourceItem) *ResourceItem {
	if item == nil {
		return nil
	}
	clone := &ResourceItem{
		ID:        item.ID,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
	if item.Data != nil {
		clone.Data = deepCopyMap(item.Data)
	} else {
		clone.Data = make(map[string]interface{})
	}
	return clone
}

func deepCopyMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	b, err := json.Marshal(src)
	if err != nil {
		dst := make(map[string]interface{}, len(src))
		for k, v := range src {
			dst[k] = v
		}
		return dst
	}
	var dst map[string]interface{}
	if err := json.Unmarshal(b, &dst); err != nil {
		dst = make(map[string]interface{}, len(src))
		for k, v := range src {
			dst[k] = v
		}
	}
	return dst
}
