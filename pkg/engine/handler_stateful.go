// Stateful resource CRUD handlers for the mock engine.

package engine

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/getmockd/mockd/pkg/stateful"
	"github.com/getmockd/mockd/pkg/validation"
)

// handleStateful handles CRUD operations for stateful resources.
func (h *Handler) handleStateful(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, itemID string, pathParams map[string]string, bodyBytes []byte) int {
	w.Header().Set("Content-Type", "application/json")

	// Validate path params for all operations (if validation configured)
	if resource.HasValidation() && len(pathParams) > 0 {
		ctx := r.Context()
		result := resource.ValidatePathParams(ctx, pathParams)
		if !result.Valid {
			status := h.writeValidationError(w, result, resource.GetValidationMode())
			if status != 0 {
				return status
			}
		}
	}

	switch r.Method {
	case http.MethodGet:
		if itemID != "" {
			return h.handleStatefulGet(w, resource, itemID)
		}
		return h.handleStatefulList(w, r, resource, pathParams)

	case http.MethodPost:
		return h.handleStatefulCreate(w, r, resource, pathParams, bodyBytes)

	case http.MethodPut:
		if itemID == "" {
			return h.writeStatefulError(w, http.StatusBadRequest, "ID required for PUT", resource.Name(), "")
		}
		return h.handleStatefulUpdate(w, r, resource, itemID, pathParams, bodyBytes)

	case http.MethodDelete:
		if itemID == "" {
			return h.writeStatefulError(w, http.StatusBadRequest, "ID required for DELETE", resource.Name(), "")
		}
		return h.handleStatefulDelete(w, resource, itemID)

	default:
		return h.writeStatefulError(w, http.StatusMethodNotAllowed, "method not allowed", resource.Name(), "")
	}
}

// handleStatefulGet retrieves a single item by ID.
func (h *Handler) handleStatefulGet(w http.ResponseWriter, resource *stateful.StatefulResource, itemID string) int {
	item := resource.Get(itemID)
	if item == nil {
		return h.writeStatefulError(w, http.StatusNotFound, "resource not found", resource.Name(), itemID)
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(item.ToJSON()); err != nil {
		h.log.Error("failed to encode stateful get response", "error", err)
	}
	return http.StatusOK
}

// handleStatefulList returns a paginated collection of items.
func (h *Handler) handleStatefulList(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, pathParams map[string]string) int {
	filter := h.parseQueryFilter(r, resource, pathParams)
	result := resource.List(filter)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		h.log.Error("failed to encode stateful list response", "error", err)
	}
	return http.StatusOK
}

// handleStatefulCreate creates a new item.
func (h *Handler) handleStatefulCreate(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, pathParams map[string]string, bodyBytes []byte) int {
	if len(bodyBytes) > MaxStatefulBodySize {
		return h.writeStatefulErrorWithHint(w, http.StatusRequestEntityTooLarge, "request body too large", resource.Name(), "", "Reduce request body size to under 1MB")
	}

	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return h.writeStatefulError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), resource.Name(), "")
	}

	// Run validation if configured
	if resource.HasValidation() {
		ctx := r.Context()
		result := resource.ValidateCreate(ctx, data, pathParams)
		if !result.Valid {
			return h.writeValidationError(w, result, resource.GetValidationMode())
		}
	}

	item, err := resource.Create(data, pathParams)
	if err != nil {
		if conflictErr, ok := err.(*stateful.ConflictError); ok {
			return h.writeStatefulError(w, http.StatusConflict, "resource already exists", resource.Name(), conflictErr.ID)
		}
		if capErr, ok := err.(*stateful.CapacityError); ok {
			return h.writeStatefulErrorWithHint(w, capErr.StatusCode(), capErr.Error(), resource.Name(), "", capErr.Hint())
		}
		return h.writeStatefulError(w, http.StatusInternalServerError, err.Error(), resource.Name(), "")
	}

	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(item.ToJSON()); err != nil {
		h.log.Error("failed to encode stateful create response", "error", err)
	}
	return http.StatusCreated
}

// handleStatefulUpdate updates an existing item.
func (h *Handler) handleStatefulUpdate(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, itemID string, pathParams map[string]string, bodyBytes []byte) int {
	if len(bodyBytes) > MaxStatefulBodySize {
		return h.writeStatefulErrorWithHint(w, http.StatusRequestEntityTooLarge, "request body too large", resource.Name(), itemID, "Reduce request body size to under 1MB")
	}

	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return h.writeStatefulError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), resource.Name(), itemID)
	}

	// Run validation if configured
	if resource.HasValidation() {
		ctx := r.Context()
		result := resource.ValidateUpdate(ctx, data, pathParams)
		if !result.Valid {
			return h.writeValidationError(w, result, resource.GetValidationMode())
		}
	}

	item, err := resource.Update(itemID, data)
	if err != nil {
		if _, ok := err.(*stateful.NotFoundError); ok {
			return h.writeStatefulError(w, http.StatusNotFound, "resource not found", resource.Name(), itemID)
		}
		return h.writeStatefulError(w, http.StatusInternalServerError, err.Error(), resource.Name(), itemID)
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(item.ToJSON()); err != nil {
		h.log.Error("failed to encode stateful update response", "error", err)
	}
	return http.StatusOK
}

// handleStatefulDelete removes an item.
func (h *Handler) handleStatefulDelete(w http.ResponseWriter, resource *stateful.StatefulResource, itemID string) int {
	err := resource.Delete(itemID)
	if err != nil {
		if _, ok := err.(*stateful.NotFoundError); ok {
			return h.writeStatefulError(w, http.StatusNotFound, "resource not found", resource.Name(), itemID)
		}
		return h.writeStatefulError(w, http.StatusInternalServerError, err.Error(), resource.Name(), itemID)
	}

	w.WriteHeader(http.StatusNoContent)
	return http.StatusNoContent
}

// writeStatefulError writes a JSON error response.
func (h *Handler) writeStatefulError(w http.ResponseWriter, statusCode int, errorMsg, resource, id string) int {
	w.WriteHeader(statusCode)
	resp := stateful.ErrorResponse{Error: errorMsg, Resource: resource, ID: id, StatusCode: statusCode}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.log.Error("failed to encode stateful error response", "error", err)
	}
	return statusCode
}

// writeStatefulErrorWithHint writes a JSON error response with a resolution hint.
func (h *Handler) writeStatefulErrorWithHint(w http.ResponseWriter, statusCode int, errorMsg, resource, id, hint string) int {
	w.WriteHeader(statusCode)
	resp := stateful.ErrorResponse{Error: errorMsg, Resource: resource, ID: id, StatusCode: statusCode, Hint: hint}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.log.Error("failed to encode stateful error response", "error", err)
	}
	return statusCode
}

// writeValidationError writes a validation error response.
func (h *Handler) writeValidationError(w http.ResponseWriter, result *validation.Result, mode string) int {
	// In warn mode, log but don't fail
	if mode == validation.ModeWarn {
		for _, err := range result.Errors {
			h.log.Warn("validation warning", "field", err.Field, "message", err.Message)
		}
		return 0 // Continue processing
	}

	// In permissive mode, only fail on required field errors
	if mode == validation.ModePermissive {
		hasRequired := false
		for _, err := range result.Errors {
			if err.Code == validation.ErrCodeRequired {
				hasRequired = true
				break
			}
		}
		if !hasRequired {
			for _, err := range result.Errors {
				h.log.Warn("validation warning (permissive)", "field", err.Field, "message", err.Message)
			}
			return 0 // Continue processing
		}
	}

	// Strict mode (default) - return error response
	resp := validation.NewErrorResponse(result, http.StatusBadRequest)
	resp.WriteResponse(w)
	return http.StatusBadRequest
}

// parseQueryFilter extracts filter parameters from query string.
func (h *Handler) parseQueryFilter(r *http.Request, resource *stateful.StatefulResource, pathParams map[string]string) *stateful.QueryFilter {
	filter := stateful.DefaultQueryFilter()
	query := r.URL.Query()

	if limit := query.Get("limit"); limit != "" {
		var l int
		if _, err := parseIntParam(limit, &l); err == nil && l > 0 {
			filter.Limit = l
		}
	}

	if offset := query.Get("offset"); offset != "" {
		var o int
		if _, err := parseIntParam(offset, &o); err == nil && o >= 0 {
			filter.Offset = o
		}
	}

	if sort := query.Get("sort"); sort != "" {
		filter.Sort = sort
	}

	if order := query.Get("order"); order != "" {
		filter.Order = order
	}

	if parentField := resource.ParentField(); parentField != "" {
		if parentID, ok := pathParams[parentField]; ok {
			filter.ParentField = parentField
			filter.ParentID = parentID
		}
	}

	reserved := map[string]bool{"limit": true, "offset": true, "sort": true, "order": true}
	for key, values := range query {
		if !reserved[key] && len(values) > 0 {
			filter.Filters[key] = values[0]
		}
	}

	return filter
}

// parseIntParam parses an integer parameter safely using strconv.
// It validates that the string contains a valid non-negative integer.
func parseIntParam(s string, v *int) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	*v = n
	return n, nil
}
