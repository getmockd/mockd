// Stateful resource CRUD handlers for the mock engine.

package engine

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/getmockd/mockd/pkg/stateful"
)

// handleStateful handles CRUD operations for stateful resources.
func (h *Handler) handleStateful(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, itemID string, pathParams map[string]string, bodyBytes []byte) int {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		if itemID != "" {
			return h.handleStatefulGet(w, resource, itemID)
		}
		return h.handleStatefulList(w, r, resource, pathParams)

	case http.MethodPost:
		return h.handleStatefulCreate(w, resource, pathParams, bodyBytes)

	case http.MethodPut:
		if itemID == "" {
			return h.writeStatefulError(w, http.StatusBadRequest, "ID required for PUT", resource.Name(), "")
		}
		return h.handleStatefulUpdate(w, resource, itemID, bodyBytes)

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
	json.NewEncoder(w).Encode(item.ToJSON())
	return http.StatusOK
}

// handleStatefulList returns a paginated collection of items.
func (h *Handler) handleStatefulList(w http.ResponseWriter, r *http.Request, resource *stateful.StatefulResource, pathParams map[string]string) int {
	filter := h.parseQueryFilter(r, resource, pathParams)
	result := resource.List(filter)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(result)
	return http.StatusOK
}

// handleStatefulCreate creates a new item.
func (h *Handler) handleStatefulCreate(w http.ResponseWriter, resource *stateful.StatefulResource, pathParams map[string]string, bodyBytes []byte) int {
	if len(bodyBytes) > MaxStatefulBodySize {
		return h.writeStatefulErrorWithHint(w, http.StatusRequestEntityTooLarge, "request body too large", resource.Name(), "", "Reduce request body size to under 1MB")
	}

	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return h.writeStatefulError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), resource.Name(), "")
	}

	item, err := resource.Create(data, pathParams)
	if err != nil {
		if _, ok := err.(*stateful.ConflictError); ok {
			return h.writeStatefulError(w, http.StatusConflict, "resource already exists", resource.Name(), data["id"].(string))
		}
		return h.writeStatefulError(w, http.StatusInternalServerError, err.Error(), resource.Name(), "")
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(item.ToJSON())
	return http.StatusCreated
}

// handleStatefulUpdate updates an existing item.
func (h *Handler) handleStatefulUpdate(w http.ResponseWriter, resource *stateful.StatefulResource, itemID string, bodyBytes []byte) int {
	if len(bodyBytes) > MaxStatefulBodySize {
		return h.writeStatefulErrorWithHint(w, http.StatusRequestEntityTooLarge, "request body too large", resource.Name(), itemID, "Reduce request body size to under 1MB")
	}

	var data map[string]any
	if err := json.Unmarshal(bodyBytes, &data); err != nil {
		return h.writeStatefulError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), resource.Name(), itemID)
	}

	item, err := resource.Update(itemID, data)
	if err != nil {
		if _, ok := err.(*stateful.NotFoundError); ok {
			return h.writeStatefulError(w, http.StatusNotFound, "resource not found", resource.Name(), itemID)
		}
		return h.writeStatefulError(w, http.StatusInternalServerError, err.Error(), resource.Name(), itemID)
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(item.ToJSON())
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
	json.NewEncoder(w).Encode(resp)
	return statusCode
}

// writeStatefulErrorWithHint writes a JSON error response with a resolution hint.
func (h *Handler) writeStatefulErrorWithHint(w http.ResponseWriter, statusCode int, errorMsg, resource, id, hint string) int {
	w.WriteHeader(statusCode)
	resp := stateful.ErrorResponse{Error: errorMsg, Resource: resource, ID: id, StatusCode: statusCode, Hint: hint}
	json.NewEncoder(w).Encode(resp)
	return statusCode
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

// parseIntParam parses an integer parameter.
func parseIntParam(s string, v *int) (int, error) {
	var n int
	_, err := func() (int, error) {
		for _, c := range s {
			if c < '0' || c > '9' {
				return 0, io.EOF
			}
			n = n*10 + int(c-'0')
		}
		*v = n
		return n, nil
	}()
	return n, err
}
