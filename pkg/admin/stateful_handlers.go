package admin

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/store"
)

// handleStateOverview returns information about all stateful resources.
func (a *API) handleStateOverview(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")

	overview, err := engine.GetStateOverview(ctx, workspaceID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "get state overview"))
		return
	}

	writeJSON(w, http.StatusOK, overview)
}

// handleStateReset resets stateful resources to their seed data.
func (a *API) handleStateReset(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")

	// Get optional resource filter from query param
	resourceName := r.URL.Query().Get("resource")

	if err := engine.ResetState(ctx, workspaceID, resourceName); err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Resource not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "reset state"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

// handleResetStateResource resets a specific stateful resource to its seed data.
func (a *API) handleResetStateResource(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	if err := engine.ResetState(ctx, workspaceID, name); err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Resource not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "reset state resource"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "reset", "resource": name})
}

// handleListStateResources returns a list of all registered stateful resources.
func (a *API) handleListStateResources(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")

	overview, err := engine.GetStateOverview(ctx, workspaceID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "list state resources"))
		return
	}

	writeJSON(w, http.StatusOK, overview.Resources)
}

// handleGetStateResource returns details about a specific stateful resource.
func (a *API) handleGetStateResource(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")

	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	resource, err := engine.GetStateResource(ctx, workspaceID, name)
	if err != nil {
		status, code, msg := mapStatefulResourceError(err, a.logger(), "Resource not found", "get state resource")
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusOK, resource)
}

// handleCreateStateResource registers a new stateful resource definition.
// The engine is authoritative: we register there first, then persist to the
// file store so the definition survives restarts.
func (a *API) handleCreateStateResource(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")

	var cfg config.StatefulResourceConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSONDecodeError(w, err, a.logger())
		return
	}

	if cfg.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "resource name is required")
		return
	}

	// Default ID field to "id" if not specified.
	if cfg.IDField == "" {
		cfg.IDField = "id"
	}

	// Register with the engine first — engine is the source of truth for
	// whether a resource is already active. This prevents state disagreement
	// where the file store says "already exists" but the engine never loaded it.
	if err := engine.RegisterStatefulResource(ctx, workspaceID, &cfg); err != nil {
		if errors.Is(err, engineclient.ErrConflict) {
			writeError(w, http.StatusConflict, "conflict", "resource already exists: "+cfg.Name)
			return
		}
		writeError(w, http.StatusBadRequest, "registration_failed",
			sanitizeError(err, a.logger(), "register stateful resource"))
		return
	}

	// Engine accepted the resource — now persist to the file store so it
	// survives restarts. If persistence fails (e.g., stale duplicate in
	// data.json), log a warning but don't error — the engine is authoritative.
	if a.dataStore != nil {
		// Stamp the workspace so the file store records (workspace, name) as
		// identity (issue #12). The engine already registered under workspaceID.
		cfg.Workspace = workspaceID
		resStore := a.dataStore.StatefulResources()
		if err := resStore.Create(ctx, &cfg); err != nil {
			if errors.Is(err, store.ErrAlreadyExists) {
				a.logger().Warn("resource already in file store (stale entry), engine is authoritative",
					"name", cfg.Name)
			} else {
				a.logger().Warn("failed to persist stateful resource", "name", cfg.Name, "error", err)
			}
		}
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"name":    cfg.Name,
		"idField": cfg.IDField,
		"message": "Stateful resource created",
	})
}

// handleDeleteStateResource fully unregisters a stateful resource definition.
// It removes the resource from both the engine (runtime) and the data store (persistence).
func (a *API) handleDeleteStateResource(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")

	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	// Unregister from the engine (runtime state).
	if err := engine.DeleteStatefulResource(ctx, workspaceID, name); err != nil {
		status, code, msg := mapStatefulResourceError(err, a.logger(), "Resource not found", "delete state resource")
		writeError(w, status, code, msg)
		return
	}

	// Remove from the file store so it doesn't reload on restart.
	if a.dataStore != nil {
		resStore := a.dataStore.StatefulResources()
		if err := resStore.Delete(ctx, workspaceID, name); err != nil {
			a.logger().Warn("failed to delete stateful resource from store",
				"name", name, "workspace", workspaceID, "error", err)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"deleted":  true,
		"resource": name,
		"message":  "Stateful resource deleted",
	})
}

// handleClearStateResource clears all items from a specific resource (does not restore seed data).
func (a *API) handleClearStateResource(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")

	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	if err := engine.ClearStateResource(ctx, workspaceID, name); err != nil {
		status, code, msg := mapStatefulResourceError(err, a.logger(), "Resource not found", "clear state resource")
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"cleared": true})
}

// handleListStatefulItems returns paginated items for a stateful resource.
func (a *API) handleListStatefulItems(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	sort := r.URL.Query().Get("sort")
	if sort == "" {
		sort = "createdAt"
	}
	order := r.URL.Query().Get("order")
	if order == "" {
		order = "desc"
	}

	result, err := engine.ListStatefulItems(ctx, workspaceID, name, limit, offset, sort, order)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Resource not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "list stateful items"))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleGetStatefulItem returns a specific item from a stateful resource.
func (a *API) handleGetStatefulItem(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Item ID is required")
		return
	}

	item, err := engine.GetStatefulItem(ctx, workspaceID, name, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Item not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "get stateful item"))
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// handleCreateStatefulItem creates a new item in a stateful resource.
func (a *API) handleCreateStatefulItem(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Resource name is required")
		return
	}

	var data map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeJSONDecodeError(w, err, a.logger())
		return
	}

	item, err := engine.CreateStatefulItem(ctx, workspaceID, name, data)
	if err != nil {
		status, code, msg := mapCreateStatefulItemError(err, a.logger())
		writeError(w, status, code, msg)
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

func mapCreateStatefulItemError(err error, log *slog.Logger) (int, string, string) {
	if errors.Is(err, engineclient.ErrNotFound) {
		return http.StatusNotFound, "not_found", "Resource not found"
	}
	if errors.Is(err, engineclient.ErrConflict) {
		return http.StatusConflict, "conflict", "Item already exists"
	}
	if errors.Is(err, engineclient.ErrCapacity) {
		return http.StatusInsufficientStorage, "capacity_exceeded", "Resource capacity exceeded"
	}
	return http.StatusBadRequest, "create_failed", sanitizeError(err, log, "create stateful item")
}

func mapStatefulResourceError(err error, log *slog.Logger, notFoundMsg, operation string) (int, string, string) { //nolint:unparam // notFoundMsg is always "Resource not found" today but kept as a parameter for caller clarity and future flexibility
	if errors.Is(err, engineclient.ErrNotFound) {
		return http.StatusNotFound, "not_found", notFoundMsg
	}
	return http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, log, operation)
}

// --- Table Reset Handlers (top-level /reset endpoints) ---

// handleResetTables resets all stateful tables to their seed data.
// POST /reset
func (a *API) handleResetTables(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")

	resp, err := engine.ResetStateWithResponse(ctx, workspaceID, "")
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "reset tables"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "reset",
		"tables": len(resp.Resources),
	})
}

// handleResetTable resets a single stateful table to its seed data.
// POST /reset/{table}
func (a *API) handleResetTable(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")

	table := r.PathValue("table")
	if table == "" {
		writeError(w, http.StatusBadRequest, "missing_table", "Table name is required")
		return
	}

	if err := engine.ResetState(ctx, workspaceID, table); err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Table not found: "+table)
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "reset table"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "reset",
		"table":  table,
	})
}

// --- Custom Operation Handlers ---

// handleListCustomOperations returns all registered custom operations.
func (a *API) handleListCustomOperations(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")

	ops, err := engine.ListCustomOperations(ctx, workspaceID)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "list custom operations"))
		return
	}

	if ops == nil {
		ops = []engineclient.CustomOperationInfo{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"operations": ops,
		"count":      len(ops),
	})
}

// handleGetCustomOperation returns a specific custom operation.
func (a *API) handleGetCustomOperation(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Operation name is required")
		return
	}

	op, err := engine.GetCustomOperation(ctx, workspaceID, name)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Custom operation not found: "+name)
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "get custom operation"))
		return
	}

	writeJSON(w, http.StatusOK, op)
}

// handleRegisterCustomOperation registers a new custom operation.
func (a *API) handleRegisterCustomOperation(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")

	var cfg map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSONDecodeError(w, err, a.logger())
		return
	}

	name, _ := cfg["name"].(string)
	if name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "Operation name is required")
		return
	}

	if err := engine.RegisterCustomOperation(ctx, workspaceID, cfg); err != nil {
		writeError(w, http.StatusBadRequest, "registration_failed", sanitizeError(err, a.logger(), "register custom operation"))
		return
	}

	// Persist to file store so custom operations survive restarts.
	if a.dataStore != nil {
		opStore := a.dataStore.CustomOperations()
		// Re-marshal the map into a CustomOperationConfig for typed persistence.
		rawBytes, marshalErr := json.Marshal(cfg)
		if marshalErr == nil {
			var opCfg config.CustomOperationConfig
			if unmarshalErr := json.Unmarshal(rawBytes, &opCfg); unmarshalErr == nil && opCfg.Name != "" {
				// Stamp the workspace so the file store records (workspace, name) as
				// identity (issue #12).
				opCfg.Workspace = workspaceID
				if createErr := opStore.Create(ctx, &opCfg); createErr != nil {
					if errors.Is(createErr, store.ErrAlreadyExists) {
						// Update: delete then re-create within this workspace.
						_ = opStore.Delete(ctx, workspaceID, opCfg.Name)
						_ = opStore.Create(ctx, &opCfg)
					} else {
						a.logger().Warn("failed to persist custom operation",
							"name", name, "workspace", workspaceID, "error", createErr)
					}
				}
			}
		}
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"name":    name,
		"message": "Custom operation registered",
	})
}

// handleDeleteCustomOperation deletes a custom operation.
func (a *API) handleDeleteCustomOperation(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Operation name is required")
		return
	}

	if err := engine.DeleteCustomOperation(ctx, workspaceID, name); err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Custom operation not found: "+name)
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "delete custom operation"))
		return
	}

	// Remove from file store.
	if a.dataStore != nil {
		if delErr := a.dataStore.CustomOperations().Delete(ctx, workspaceID, name); delErr != nil {
			if !errors.Is(delErr, store.ErrNotFound) {
				a.logger().Warn("failed to remove custom operation from file store",
					"name", name, "workspace", workspaceID, "error", delErr)
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":    name,
		"message": "Custom operation deleted",
	})
}

// handleExecuteCustomOperation executes a custom operation with the given input.
func (a *API) handleExecuteCustomOperation(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	workspaceID := r.URL.Query().Get("workspaceId")
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Operation name is required")
		return
	}

	var input map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		// Allow empty body (no input)
		input = make(map[string]interface{})
	}

	result, err := engine.ExecuteCustomOperation(ctx, workspaceID, name, input)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Custom operation not found: "+name)
			return
		}
		writeError(w, http.StatusBadRequest, "execution_failed", sanitizeError(err, a.logger(), "execute custom operation"))
		return
	}

	writeJSON(w, http.StatusOK, result)
}
