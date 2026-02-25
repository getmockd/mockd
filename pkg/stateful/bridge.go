package stateful

import (
	"context"
	"fmt"
	"time"
)

// Action represents the type of operation to perform on a stateful resource.
type Action string

const (
	// ActionGet retrieves a single item by ID.
	ActionGet Action = "get"
	// ActionList retrieves a filtered/paginated collection of items.
	ActionList Action = "list"
	// ActionCreate creates a new item.
	ActionCreate Action = "create"
	// ActionUpdate replaces an existing item (PUT semantics).
	ActionUpdate Action = "update"
	// ActionPatch partially updates an existing item (PATCH semantics).
	ActionPatch Action = "patch"
	// ActionDelete removes an item by ID.
	ActionDelete Action = "delete"
	// ActionCustom executes a multi-step custom operation via the OperationExecutor.
	ActionCustom Action = "custom"
)

// ResultStatus indicates the outcome of a bridge operation.
type ResultStatus int

const (
	// StatusSuccess indicates the operation completed successfully.
	StatusSuccess ResultStatus = iota
	// StatusCreated indicates a new item was created.
	StatusCreated
	// StatusNotFound indicates the requested item/resource was not found.
	StatusNotFound
	// StatusConflict indicates a duplicate ID conflict.
	StatusConflict
	// StatusValidationError indicates invalid input.
	StatusValidationError
	// StatusCapacityExceeded indicates the resource is at maximum capacity.
	StatusCapacityExceeded
	// StatusError indicates an internal or unexpected error.
	StatusError
)

// OperationRequest is a protocol-agnostic request to perform a CRUD operation
// on a stateful resource. Protocol handlers (SOAP, GraphQL, gRPC, etc.)
// translate their wire format into this struct before calling Bridge.Execute().
type OperationRequest struct {
	// Resource is the name of the stateful resource (e.g., "users", "orders").
	Resource string
	// Action is the CRUD action to perform.
	Action Action
	// ResourceID is the item ID for single-item operations (get, update, patch, delete).
	ResourceID string
	// Data is the request payload, already deserialized into a map by the protocol adapter.
	Data map[string]interface{}
	// Params contains protocol-extracted path parameters (for nested resources).
	Params map[string]string
	// Filter contains query/filter/pagination parameters for list operations.
	Filter *QueryFilter
}

// OperationResult is the protocol-agnostic response from a bridge operation.
// Protocol handlers translate this back to their wire format (JSON, XML, protobuf, etc.).
type OperationResult struct {
	// Status is the outcome of the operation.
	Status ResultStatus
	// Item is the result for single-item operations (get, create, update, patch).
	// Nil for list and delete operations.
	Item *ResourceItem
	// List is the result for list operations.
	// Nil for single-item operations.
	List *PaginatedResponse
	// Error is the domain error, if any. Nil on success.
	// Protocol adapters should inspect Error to generate protocol-specific error responses
	// (e.g., HTTP status codes, SOAP faults, gRPC status codes).
	Error error
}

// Bridge is a protocol-agnostic service layer that routes operation requests
// to stateful resources. Any protocol handler (HTTP, SOAP, GraphQL, gRPC, etc.)
// can call Bridge.Execute() to perform CRUD operations on stateful resources.
//
// The Bridge also fires Observer hooks on every operation, making the Observer
// pattern live (previously it was defined but never wired).
type Bridge struct {
	store     *StateStore
	observer  Observer
	executor  *OperationExecutor
	customOps map[string]*CustomOperation // name → custom operation definition
}

// NewBridge creates a new Bridge backed by the given StateStore.
// The Bridge uses the store's observer for metrics/logging hooks.
func NewBridge(store *StateStore) *Bridge {
	if store == nil {
		panic("stateful.NewBridge: store must not be nil")
	}
	return &Bridge{
		store:     store,
		observer:  store.GetObserver(),
		executor:  NewOperationExecutor(store),
		customOps: make(map[string]*CustomOperation),
	}
}

// RegisterCustomOperation registers a named custom operation.
// Operations are referenced by name in OperationRequest.Resource when Action is "custom".
func (b *Bridge) RegisterCustomOperation(name string, op *CustomOperation) {
	b.customOps[name] = op
}

// GetCustomOperation returns a registered custom operation by name.
func (b *Bridge) GetCustomOperation(name string) *CustomOperation {
	return b.customOps[name]
}

// ListCustomOperations returns all registered custom operations as a name→operation map.
// Used by Export to serialize custom operation definitions back to config format.
func (b *Bridge) ListCustomOperations() map[string]*CustomOperation {
	if len(b.customOps) == 0 {
		return nil
	}
	// Return a copy to prevent external mutation
	result := make(map[string]*CustomOperation, len(b.customOps))
	for name, op := range b.customOps {
		result[name] = op
	}
	return result
}

// Execute performs a CRUD operation on a stateful resource.
// This is the single entry point for all protocol adapters.
//
// The method:
//  1. Resolves the resource by name from the store
//  2. Dispatches to the appropriate CRUD method based on Action
//  3. Fires Observer hooks with operation timing
//  4. Returns a protocol-agnostic OperationResult
func (b *Bridge) Execute(ctx context.Context, req *OperationRequest) *OperationResult {
	if req == nil {
		return &OperationResult{
			Status: StatusError,
			Error:  fmt.Errorf("operation request must not be nil"),
		}
	}

	// Custom operations are dispatched before resource lookup because
	// req.Resource contains the operation name, not a resource name.
	// The operation's individual steps reference resources by name internally.
	if req.Action == ActionCustom {
		return b.executeCustom(ctx, req)
	}

	resource := b.store.Get(req.Resource)
	if resource == nil {
		err := &NotFoundError{Resource: req.Resource}
		b.observer.OnError(req.Resource, string(req.Action), err)
		return &OperationResult{
			Status: StatusNotFound,
			Error:  err,
		}
	}

	switch req.Action {
	case ActionGet:
		return b.executeGet(resource, req)
	case ActionList:
		return b.executeList(resource, req)
	case ActionCreate:
		return b.executeCreate(resource, req)
	case ActionUpdate:
		return b.executeUpdate(resource, req)
	case ActionPatch:
		return b.executePatch(resource, req)
	case ActionDelete:
		return b.executeDelete(resource, req)
	default:
		err := fmt.Errorf("unsupported action: %s", req.Action)
		b.observer.OnError(req.Resource, string(req.Action), err)
		return &OperationResult{
			Status: StatusError,
			Error:  err,
		}
	}
}

func (b *Bridge) executeGet(resource *StatefulResource, req *OperationRequest) *OperationResult {
	start := time.Now()

	if req.ResourceID == "" {
		err := &ValidationError{Message: "resource ID is required for get operations"}
		b.observer.OnError(resource.Name(), "get", err)
		return &OperationResult{Status: StatusValidationError, Error: err}
	}

	item := resource.Get(req.ResourceID)
	if item == nil {
		err := &NotFoundError{Resource: resource.Name(), ID: req.ResourceID}
		b.observer.OnError(resource.Name(), "get", err)
		return &OperationResult{Status: StatusNotFound, Error: err}
	}

	b.observer.OnRead(resource.Name(), req.ResourceID, time.Since(start))
	return &OperationResult{Status: StatusSuccess, Item: item}
}

func (b *Bridge) executeList(resource *StatefulResource, req *OperationRequest) *OperationResult {
	start := time.Now()

	filter := req.Filter
	if filter == nil {
		filter = DefaultQueryFilter()
	}

	// Inject parent params from the request if applicable
	if resource.ParentField() != "" && req.Params != nil {
		if parentID, ok := req.Params[resource.ParentField()]; ok {
			filter.ParentID = parentID
			filter.ParentField = resource.ParentField()
		}
	}

	result := resource.List(filter)

	b.observer.OnList(resource.Name(), result.Meta.Count, time.Since(start))
	return &OperationResult{Status: StatusSuccess, List: result}
}

func (b *Bridge) executeCreate(resource *StatefulResource, req *OperationRequest) *OperationResult {
	start := time.Now()

	if req.Data == nil {
		req.Data = make(map[string]interface{})
	}

	item, err := resource.Create(req.Data, req.Params)
	if err != nil {
		b.observer.OnError(resource.Name(), "create", err)
		return errorToResult(err)
	}

	b.observer.OnCreate(resource.Name(), item.ID, time.Since(start))
	return &OperationResult{Status: StatusCreated, Item: item}
}

func (b *Bridge) executeUpdate(resource *StatefulResource, req *OperationRequest) *OperationResult {
	start := time.Now()

	if req.ResourceID == "" {
		err := &ValidationError{Message: "resource ID is required for update operations"}
		b.observer.OnError(resource.Name(), "update", err)
		return &OperationResult{Status: StatusValidationError, Error: err}
	}

	if req.Data == nil {
		req.Data = make(map[string]interface{})
	}

	item, err := resource.Update(req.ResourceID, req.Data)
	if err != nil {
		b.observer.OnError(resource.Name(), "update", err)
		return errorToResult(err)
	}

	b.observer.OnUpdate(resource.Name(), item.ID, time.Since(start))
	return &OperationResult{Status: StatusSuccess, Item: item}
}

func (b *Bridge) executePatch(resource *StatefulResource, req *OperationRequest) *OperationResult {
	start := time.Now()

	if req.ResourceID == "" {
		err := &ValidationError{Message: "resource ID is required for patch operations"}
		b.observer.OnError(resource.Name(), "patch", err)
		return &OperationResult{Status: StatusValidationError, Error: err}
	}

	if req.Data == nil {
		req.Data = make(map[string]interface{})
	}

	item, err := resource.Patch(req.ResourceID, req.Data)
	if err != nil {
		b.observer.OnError(resource.Name(), "patch", err)
		return errorToResult(err)
	}

	b.observer.OnUpdate(resource.Name(), item.ID, time.Since(start))
	return &OperationResult{Status: StatusSuccess, Item: item}
}

func (b *Bridge) executeDelete(resource *StatefulResource, req *OperationRequest) *OperationResult {
	start := time.Now()

	if req.ResourceID == "" {
		err := &ValidationError{Message: "resource ID is required for delete operations"}
		b.observer.OnError(resource.Name(), "delete", err)
		return &OperationResult{Status: StatusValidationError, Error: err}
	}

	err := resource.Delete(req.ResourceID)
	if err != nil {
		b.observer.OnError(resource.Name(), "delete", err)
		return errorToResult(err)
	}

	b.observer.OnDelete(resource.Name(), req.ResourceID, time.Since(start))
	return &OperationResult{Status: StatusSuccess}
}

func (b *Bridge) executeCustom(ctx context.Context, req *OperationRequest) *OperationResult {
	start := time.Now()

	opName := req.Resource // for custom actions, Resource holds the operation name
	op := b.customOps[opName]
	if op == nil {
		err := &NotFoundError{Resource: "custom operation: " + opName}
		b.observer.OnError(opName, "custom", err)
		return &OperationResult{
			Status: StatusNotFound,
			Error:  err,
		}
	}

	result := b.executor.Execute(ctx, op, req)
	if result.Error != nil {
		b.observer.OnError(opName, "custom", result.Error)
	} else {
		b.observer.OnRead(opName, "custom", time.Since(start))
	}

	return result
}

// errorToResult converts a domain error to an OperationResult with the appropriate status.
func errorToResult(err error) *OperationResult {
	switch err.(type) {
	case *NotFoundError:
		return &OperationResult{Status: StatusNotFound, Error: err}
	case *ConflictError:
		return &OperationResult{Status: StatusConflict, Error: err}
	case *ValidationError:
		return &OperationResult{Status: StatusValidationError, Error: err}
	case *CapacityError:
		return &OperationResult{Status: StatusCapacityExceeded, Error: err}
	default:
		return &OperationResult{Status: StatusError, Error: err}
	}
}

// Store returns the underlying StateStore.
func (b *Bridge) Store() *StateStore {
	return b.store
}
