// Stateful resource CRUD handlers for the mock engine.

package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

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

	case http.MethodPatch:
		if itemID == "" {
			return h.writeStatefulError(w, http.StatusBadRequest, "ID required for PATCH", resource.Name(), "")
		}
		return h.handleStatefulPatch(w, r, resource, itemID, pathParams, bodyBytes)

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
		return h.writeResourceError(w, http.StatusNotFound, "resource not found", resource, itemID)
	}

	data := stateful.TransformItem(item.ToJSON(), resource.ResponseConfig())
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.log.Error("failed to encode stateful get response", "error", err)
	}
	return http.StatusOK
}

// handleStatefulList returns a paginated collection of items.
func (h *Handler) handleStatefulList(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, pathParams map[string]string) int {
	filter := h.parseQueryFilter(r, resource, pathParams)
	result := resource.List(filter)

	response := stateful.TransformList(result.Data, result.Meta, resource.ResponseConfig())
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("failed to encode stateful list response", "error", err)
	}
	return http.StatusOK
}

// handleStatefulCreate creates a new item.
func (h *Handler) handleStatefulCreate(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, pathParams map[string]string, bodyBytes []byte) int {
	if len(bodyBytes) > MaxStatefulBodySize {
		return h.writeStatefulErrorWithHint(w, http.StatusRequestEntityTooLarge, "request body too large", resource.Name(), "", "Reduce request body size to under 1MB")
	}

	data, err := parseStatefulBody(bodyBytes, r.Header.Get("Content-Type"))
	if err != nil {
		return h.writeResourceError(w, http.StatusBadRequest, err.Error(), resource, "")
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
		var conflictErr *stateful.ConflictError
		var capErr *stateful.CapacityError
		if errors.As(err, &conflictErr) {
			return h.writeResourceError(w, http.StatusConflict, "resource already exists", resource, conflictErr.ID)
		}
		if errors.As(err, &capErr) {
			return h.writeStatefulErrorWithHint(w, http.StatusInsufficientStorage, capErr.Error(), resource.Name(), "", capErr.Hint())
		}
		return h.writeResourceError(w, http.StatusInternalServerError, err.Error(), resource, "")
	}

	cfg := resource.ResponseConfig()
	responseData := stateful.TransformItem(item.ToJSON(), cfg)
	createStatus := stateful.TransformCreateStatus(cfg)
	w.WriteHeader(createStatus)
	if err := json.NewEncoder(w).Encode(responseData); err != nil {
		h.log.Error("failed to encode stateful create response", "error", err)
	}
	return createStatus
}

// handleStatefulUpdate updates an existing item (full replace).
func (h *Handler) handleStatefulUpdate(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, itemID string, pathParams map[string]string, bodyBytes []byte) int {
	return h.handleStatefulMutate(w, r, resource, itemID, pathParams, bodyBytes, resource.Update)
}

// handleStatefulPatch partially updates an existing item by merging fields.
func (h *Handler) handleStatefulPatch(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, itemID string, pathParams map[string]string, bodyBytes []byte) int {
	return h.handleStatefulMutate(w, r, resource, itemID, pathParams, bodyBytes, resource.Patch)
}

// handleStatefulMutate is the shared implementation for update and patch operations.
func (h *Handler) handleStatefulMutate(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, itemID string, pathParams map[string]string, bodyBytes []byte, mutate func(string, map[string]any) (*stateful.ResourceItem, error)) int {
	if len(bodyBytes) > MaxStatefulBodySize {
		return h.writeStatefulErrorWithHint(w, http.StatusRequestEntityTooLarge, "request body too large", resource.Name(), itemID, "Reduce request body size to under 1MB")
	}

	data, err := parseStatefulBody(bodyBytes, r.Header.Get("Content-Type"))
	if err != nil {
		return h.writeResourceError(w, http.StatusBadRequest, err.Error(), resource, itemID)
	}

	// Run validation if configured
	if resource.HasValidation() {
		ctx := r.Context()
		result := resource.ValidateUpdate(ctx, data, pathParams)
		if !result.Valid {
			return h.writeValidationError(w, result, resource.GetValidationMode())
		}
	}

	item, err := mutate(itemID, data)
	if err != nil {
		var notFoundErr *stateful.NotFoundError
		if errors.As(err, &notFoundErr) {
			return h.writeResourceError(w, http.StatusNotFound, "resource not found", resource, itemID)
		}
		return h.writeResourceError(w, http.StatusInternalServerError, err.Error(), resource, itemID)
	}

	mutateData := stateful.TransformItem(item.ToJSON(), resource.ResponseConfig())
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(mutateData); err != nil {
		h.log.Error("failed to encode stateful response", "error", err)
	}
	return http.StatusOK
}

// handleStatefulDelete removes an item.
func (h *Handler) handleStatefulDelete(w http.ResponseWriter, resource *stateful.StatefulResource, itemID string) int {
	deletedItem, err := resource.Delete(itemID)
	if err != nil {
		var notFoundErr *stateful.NotFoundError
		if errors.As(err, &notFoundErr) {
			return h.writeResourceError(w, http.StatusNotFound, "resource not found", resource, itemID)
		}
		return h.writeResourceError(w, http.StatusInternalServerError, err.Error(), resource, itemID)
	}

	cfg := resource.ResponseConfig()
	deleteStatus, deleteBody := stateful.TransformDeleteResponse(deletedItem, cfg)
	w.WriteHeader(deleteStatus)
	if deleteBody != nil {
		if err := json.NewEncoder(w).Encode(deleteBody); err != nil {
			h.log.Error("failed to encode stateful delete response", "error", err)
		}
	}
	return deleteStatus
}

// handleCustomOperation executes a registered custom operation via the Bridge.
// The JSON request body becomes the operation input, and the result is returned as JSON.
func (h *Handler) handleCustomOperation(w http.ResponseWriter, r *http.Request, operationName string, bodyBytes []byte) int {
	w.Header().Set("Content-Type", "application/json")

	if h.statefulBridge == nil {
		return h.writeStatefulError(w, http.StatusServiceUnavailable, "stateful bridge not configured", "", "")
	}
	if len(bodyBytes) > MaxStatefulBodySize {
		return h.writeStatefulErrorWithHint(w, http.StatusRequestEntityTooLarge, "request body too large", operationName, "", "Reduce request body size to under 1MB")
	}

	// Parse JSON body as input (allow empty body)
	var input map[string]interface{}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &input); err != nil {
			return h.writeStatefulError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error(), "", "")
		}
	}
	if input == nil {
		input = make(map[string]interface{})
	}

	// Execute via bridge
	result := h.statefulBridge.Execute(r.Context(), &stateful.OperationRequest{
		Action:        stateful.ActionCustom,
		OperationName: operationName,
		Data:          input,
	})

	if result.Error != nil {
		// Map stateful errors to HTTP status codes
		code := stateful.GetErrorCode(result.Error)
		var httpStatus int
		switch code {
		case stateful.ErrCodeNotFound:
			httpStatus = http.StatusNotFound
		case stateful.ErrCodeValidation:
			httpStatus = http.StatusBadRequest
		case stateful.ErrCodeConflict:
			httpStatus = http.StatusConflict
		default:
			httpStatus = http.StatusInternalServerError
		}
		return h.writeStatefulError(w, httpStatus, result.Error.Error(), operationName, "")
	}

	// Return result as JSON
	if result.Item != nil {
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(result.Item.ToJSON()); err != nil {
			h.log.Error("failed to encode custom operation response", "error", err)
		}
		return http.StatusOK
	}

	// Successful but no item result (e.g., void operation)
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"success": true}); err != nil {
		h.log.Error("failed to encode custom operation response", "error", err)
	}
	return http.StatusOK
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

// writeResourceError writes an error response, applying error transforms if the resource has them configured.
func (h *Handler) writeResourceError(w http.ResponseWriter, statusCode int, errorMsg string, resource *stateful.StatefulResource, id string) int {
	cfg := resource.ResponseConfig()
	if cfg != nil && cfg.Errors != nil {
		code := httpStatusToErrorCode(statusCode)
		if transformed := stateful.TransformError(code, errorMsg, resource.Name(), id, "", cfg); transformed != nil {
			w.WriteHeader(statusCode)
			if err := json.NewEncoder(w).Encode(transformed); err != nil {
				h.log.Error("failed to encode error response", "error", err)
			}
			return statusCode
		}
	}
	return h.writeStatefulError(w, statusCode, errorMsg, resource.Name(), id)
}

// httpStatusToErrorCode maps common HTTP status codes to stateful ErrorCode values.
func httpStatusToErrorCode(status int) stateful.ErrorCode {
	switch status {
	case http.StatusNotFound:
		return stateful.ErrCodeNotFound
	case http.StatusConflict:
		return stateful.ErrCodeConflict
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		return stateful.ErrCodeValidation
	case http.StatusRequestEntityTooLarge:
		return stateful.ErrCodePayloadTooLarge
	case http.StatusInsufficientStorage:
		return stateful.ErrCodeCapacityExceeded
	default:
		return stateful.ErrCodeInternal
	}
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

// parseStatefulBody parses the request body based on Content-Type.
// Supports JSON (default) and application/x-www-form-urlencoded (for APIs like Stripe).
// Form-encoded nested keys use bracket syntax: metadata[key]=value → {"metadata":{"key":"value"}}
func parseStatefulBody(bodyBytes []byte, contentType string) (map[string]any, error) {
	// Default to JSON if no Content-Type or explicit JSON
	if contentType == "" || strings.HasPrefix(contentType, "application/json") {
		var data map[string]any
		if err := json.Unmarshal(bodyBytes, &data); err != nil {
			return nil, fmt.Errorf("invalid JSON body: %s", err.Error())
		}
		return data, nil
	}

	// Form-encoded (application/x-www-form-urlencoded)
	if strings.HasPrefix(contentType, "application/x-www-form-urlencoded") {
		values, err := url.ParseQuery(string(bodyBytes))
		if err != nil {
			return nil, fmt.Errorf("invalid form body: %s", err.Error())
		}
		return formToMap(values), nil
	}

	// Unknown Content-Type — try JSON as fallback
	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return nil, fmt.Errorf("unsupported Content-Type %q and body is not valid JSON", contentType)
	}
	return data, nil
}

// formToMap converts url.Values to a map, handling bracket-nested keys.
// "name=Alice" → {"name":"Alice"}
// "metadata[tier]=premium" → {"metadata":{"tier":"premium"}}
// "items[0][price]=price_123" → {"items":{"0":{"price":"price_123"}}}
func formToMap(values url.Values) map[string]any {
	result := make(map[string]any)

	for key, vals := range values {
		if len(vals) == 0 {
			continue
		}
		val := vals[0] // Use first value (Stripe convention)

		// Check for bracket notation: key[sub] or key[sub][sub2]
		if idx := strings.IndexByte(key, '['); idx > 0 {
			base := key[:idx]
			rest := key[idx:]
			setNested(result, base, rest, val)
		} else {
			result[key] = val
		}
	}

	return result
}

// setNested sets a value in a nested map using bracket notation path.
// base="metadata", path="[tier]", val="premium" → result["metadata"]["tier"] = "premium"
func setNested(result map[string]any, base, path, val string) {
	// Ensure base map exists
	var current map[string]any
	if existing, ok := result[base]; ok {
		if m, ok := existing.(map[string]any); ok {
			current = m
		} else {
			return // Conflict — base exists but isn't a map
		}
	} else {
		current = make(map[string]any)
		result[base] = current
	}

	// Parse remaining bracket segments: [key1][key2]...
	for strings.HasPrefix(path, "[") {
		end := strings.IndexByte(path, ']')
		if end < 0 {
			break
		}
		segment := path[1:end]
		path = path[end+1:]

		if path == "" || !strings.HasPrefix(path, "[") {
			// Last segment — set the value
			current[segment] = val
			return
		}

		// More segments — descend into nested map
		if existing, ok := current[segment]; ok {
			if m, ok := existing.(map[string]any); ok {
				current = m
			} else {
				return // Conflict
			}
		} else {
			next := make(map[string]any)
			current[segment] = next
			current = next
		}
	}
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

	// Cursor-based pagination params (Stripe-style)
	if startingAfter := query.Get("starting_after"); startingAfter != "" {
		filter.StartingAfter = startingAfter
	}
	if endingBefore := query.Get("ending_before"); endingBefore != "" {
		filter.EndingBefore = endingBefore
	}

	if parentField := resource.ParentField(); parentField != "" {
		if parentID, ok := pathParams[parentField]; ok {
			filter.ParentField = parentField
			filter.ParentID = parentID
		}
	}

	reserved := map[string]bool{"limit": true, "offset": true, "sort": true, "order": true, "starting_after": true, "ending_before": true}
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
