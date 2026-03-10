// Stateful table binding handlers for the mock engine.
//
// All stateful routing goes through handleStatefulBinding, which dispatches
// by the explicit action from an extend binding (not HTTP method). The binding
// is resolved at config-load time by processTablesAndExtend.

package engine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/stateful"
)

// handleStatefulBinding handles a request that has been bound to a stateful table
// via an extend binding. The binding specifies the table name and CRUD action.
//
// This method:
//  1. Looks up the table in the StateStore
//  2. Extracts item ID from path params (last param for single-item actions)
//  3. Parses request body (JSON or form-encoded)
//  4. Calls Bridge.Execute() with the configured action
//  5. Applies response transforms (binding override > table default)
//  6. Writes the response
func (h *Handler) handleStatefulBinding(w http.ResponseWriter, r *http.Request, matched *mock.Mock, bodyBytes []byte, pathParams map[string]string) int {
	w.Header().Set("Content-Type", "application/json")

	binding := matched.HTTP.StatefulBinding
	if h.statefulBridge == nil {
		return h.writeStatefulError(w, http.StatusServiceUnavailable, "stateful bridge not configured", binding.Table, "")
	}

	action := stateful.Action(binding.Action)

	// Resolve response transform config: binding override > table default.
	// The binding's Response.Transform was resolved at config-load time.
	var responseCfg *config.ResponseTransform
	if binding.Response != nil {
		if transform, ok := binding.Response.Transform.(*config.ResponseTransform); ok {
			responseCfg = transform
		}
	}
	// Fall back to the table's configured response transform
	if responseCfg == nil {
		responseCfg = h.statefulBridge.GetResponseConfig(binding.Table)
	}

	// Extract item ID from path params. Convention: last path param value
	// is the item ID for single-item operations.
	// Custom actions handle their own ID extraction via the operation steps.
	var itemID string
	if action != stateful.ActionList && action != stateful.ActionCreate && action != stateful.ActionCustom {
		itemID = extractLastPathParam(matched, pathParams)
		if itemID == "" {
			return h.writeStatefulError(w, http.StatusBadRequest, "item ID required for "+binding.Action, binding.Table, "")
		}
	}

	// Dispatch by action
	switch action {
	case stateful.ActionList:
		return h.handleBindingList(w, r, binding.Table, pathParams, responseCfg)
	case stateful.ActionGet:
		return h.handleBindingGet(w, r, binding.Table, itemID, responseCfg)
	case stateful.ActionCreate:
		return h.handleBindingCreate(w, r, binding.Table, pathParams, bodyBytes, responseCfg)
	case stateful.ActionUpdate:
		return h.handleBindingMutate(w, r, binding.Table, itemID, pathParams, bodyBytes, responseCfg, stateful.ActionUpdate)
	case stateful.ActionPatch:
		return h.handleBindingMutate(w, r, binding.Table, itemID, pathParams, bodyBytes, responseCfg, stateful.ActionPatch)
	case stateful.ActionDelete:
		return h.handleBindingDelete(w, r, binding.Table, itemID, responseCfg)
	case stateful.ActionCustom:
		return h.handleBindingCustom(w, r, binding, pathParams, bodyBytes, responseCfg)
	default:
		return h.writeStatefulError(w, http.StatusBadRequest, "unsupported action: "+binding.Action, binding.Table, "")
	}
}

// handleBindingGet retrieves a single item via Bridge.
func (h *Handler) handleBindingGet(w http.ResponseWriter, r *http.Request, table, itemID string, responseCfg *config.ResponseTransform) int {
	result := h.statefulBridge.Execute(r.Context(), &stateful.OperationRequest{
		Resource:   table,
		Action:     stateful.ActionGet,
		ResourceID: itemID,
	})
	if result.Error != nil {
		return h.writeBindingError(w, result, table, itemID, responseCfg)
	}

	data := stateful.TransformItem(result.Item.ToJSON(), responseCfg)

	// Apply ?expand[] if requested
	expandFields := parseExpandFields(r)
	if len(expandFields) > 0 {
		resource := h.statefulBridge.Store().Get(table)
		if resource != nil {
			if rels := resource.Relationships(); len(rels) > 0 {
				resolver := func(tableName string) *stateful.StatefulResource {
					return h.statefulBridge.Store().Get(tableName)
				}
				data = stateful.ExpandRelationships(data, expandFields, rels, resolver)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(data)
	return http.StatusOK
}

// handleBindingList returns a paginated collection via Bridge.
func (h *Handler) handleBindingList(w http.ResponseWriter, r *http.Request, table string, pathParams map[string]string, responseCfg *config.ResponseTransform) int {
	// Get the resource to reuse parseQueryFilter (it needs the resource for parentField)
	resource := h.statefulBridge.Store().Get(table)
	if resource == nil {
		return h.writeStatefulError(w, http.StatusNotFound, "table not found", table, "")
	}

	filter := h.parseQueryFilter(r, resource, pathParams)
	result := h.statefulBridge.Execute(r.Context(), &stateful.OperationRequest{
		Resource: table,
		Action:   stateful.ActionList,
		Filter:   filter,
	})
	if result.Error != nil {
		return h.writeBindingError(w, result, table, "", responseCfg)
	}

	// Apply ?expand[] to each item in the list if requested
	expandFields := parseExpandFields(r)
	if len(expandFields) > 0 {
		if rels := resource.Relationships(); len(rels) > 0 {
			resolver := func(tableName string) *stateful.StatefulResource {
				return h.statefulBridge.Store().Get(tableName)
			}
			for i, item := range result.List.Data {
				result.List.Data[i] = stateful.ExpandRelationships(item, expandFields, rels, resolver)
			}
		}
	}

	response := stateful.TransformList(result.List.Data, result.List.Meta, responseCfg)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
	return http.StatusOK
}

// handleBindingCreate creates a new item via Bridge.
func (h *Handler) handleBindingCreate(w http.ResponseWriter, r *http.Request, table string, pathParams map[string]string, bodyBytes []byte, responseCfg *config.ResponseTransform) int {
	if len(bodyBytes) > MaxStatefulBodySize {
		return h.writeStatefulErrorWithHint(w, http.StatusRequestEntityTooLarge, "request body too large", table, "", "Reduce request body size to under 1MB")
	}

	data, err := parseStatefulBody(bodyBytes, r.Header.Get("Content-Type"))
	if err != nil {
		return h.writeStatefulError(w, http.StatusBadRequest, err.Error(), table, "")
	}

	result := h.statefulBridge.Execute(r.Context(), &stateful.OperationRequest{
		Resource: table,
		Action:   stateful.ActionCreate,
		Data:     data,
		Params:   pathParams,
	})
	if result.Error != nil {
		return h.writeBindingError(w, result, table, "", responseCfg)
	}

	responseData := stateful.TransformItem(result.Item.ToJSON(), responseCfg)
	createStatus := stateful.TransformCreateStatus(responseCfg)
	w.WriteHeader(createStatus)
	_ = json.NewEncoder(w).Encode(responseData)
	return createStatus
}

// handleBindingMutate handles update/patch via Bridge.
func (h *Handler) handleBindingMutate(w http.ResponseWriter, r *http.Request, table, itemID string, pathParams map[string]string, bodyBytes []byte, responseCfg *config.ResponseTransform, action stateful.Action) int {
	if len(bodyBytes) > MaxStatefulBodySize {
		return h.writeStatefulErrorWithHint(w, http.StatusRequestEntityTooLarge, "request body too large", table, itemID, "Reduce request body size to under 1MB")
	}

	data, err := parseStatefulBody(bodyBytes, r.Header.Get("Content-Type"))
	if err != nil {
		return h.writeStatefulError(w, http.StatusBadRequest, err.Error(), table, itemID)
	}

	result := h.statefulBridge.Execute(r.Context(), &stateful.OperationRequest{
		Resource:   table,
		Action:     action,
		ResourceID: itemID,
		Data:       data,
		Params:     pathParams,
	})
	if result.Error != nil {
		return h.writeBindingError(w, result, table, itemID, responseCfg)
	}

	responseData := stateful.TransformItem(result.Item.ToJSON(), responseCfg)
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(responseData)
	return http.StatusOK
}

// handleBindingDelete removes an item via Bridge.
// If the delete override has Preserve set, the item is read but not removed (soft-delete).
func (h *Handler) handleBindingDelete(w http.ResponseWriter, r *http.Request, table, itemID string, responseCfg *config.ResponseTransform) int {
	// If preserve mode, read the item instead of deleting it
	var result *stateful.OperationResult
	if responseCfg != nil && responseCfg.Delete != nil && responseCfg.Delete.Preserve {
		result = h.statefulBridge.Execute(r.Context(), &stateful.OperationRequest{
			Resource:   table,
			Action:     stateful.ActionGet,
			ResourceID: itemID,
		})
	} else {
		result = h.statefulBridge.Execute(r.Context(), &stateful.OperationRequest{
			Resource:   table,
			Action:     stateful.ActionDelete,
			ResourceID: itemID,
		})
	}
	if result.Error != nil {
		return h.writeBindingError(w, result, table, itemID, responseCfg)
	}

	deleteStatus, deleteBody := stateful.TransformDeleteResponse(result.Item, responseCfg)
	w.WriteHeader(deleteStatus)
	if deleteBody != nil {
		_ = json.NewEncoder(w).Encode(deleteBody)
	}
	return deleteStatus
}

// writeBindingError maps a Bridge operation error to an HTTP error response,
// applying error transforms if configured.
func (h *Handler) writeBindingError(w http.ResponseWriter, result *stateful.OperationResult, table, itemID string, responseCfg *config.ResponseTransform) int {
	statusCode := bridgeStatusToHTTP(result.Status)

	if responseCfg != nil && responseCfg.Errors != nil {
		code := httpStatusToErrorCode(statusCode)
		if transformed := stateful.TransformError(code, result.Error.Error(), table, itemID, "", responseCfg); transformed != nil {
			w.WriteHeader(statusCode)
			_ = json.NewEncoder(w).Encode(transformed)
			return statusCode
		}
	}

	return h.writeStatefulError(w, statusCode, result.Error.Error(), table, itemID)
}

// bridgeStatusToHTTP maps Bridge result status to HTTP status codes.
func bridgeStatusToHTTP(status stateful.ResultStatus) int {
	switch status {
	case stateful.StatusNotFound:
		return http.StatusNotFound
	case stateful.StatusConflict:
		return http.StatusConflict
	case stateful.StatusValidationError:
		return http.StatusBadRequest
	case stateful.StatusCapacityExceeded:
		return http.StatusInsufficientStorage
	default:
		return http.StatusInternalServerError
	}
}

// extractLastPathParam returns the value of the last path parameter from the
// mock's matched path pattern. For "/v1/customers/{customer}", returns the
// value of "customer" from pathParams.
func extractLastPathParam(m *mock.Mock, pathParams map[string]string) string {
	if m.HTTP == nil || m.HTTP.Matcher == nil || len(pathParams) == 0 {
		return ""
	}

	path := m.HTTP.Matcher.Path
	if path == "" {
		return ""
	}

	// Walk path segments backward to find the last {param}.
	// Handles both pure params like {id} and params with literal
	// suffixes like {Sid}.json (common in Twilio-style APIs).
	segments := strings.Split(path, "/")
	for i := len(segments) - 1; i >= 0; i-- {
		seg := segments[i]
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			// Pure param: {id}
			paramName := seg[1 : len(seg)-1]
			return pathParams[paramName]
		}
		if strings.Contains(seg, "{") && strings.Contains(seg, "}") {
			// Param with literal prefix/suffix: {Sid}.json, v{version}, etc.
			openIdx := strings.Index(seg, "{")
			closeIdx := strings.Index(seg, "}")
			if openIdx < closeIdx {
				paramName := seg[openIdx+1 : closeIdx]
				return pathParams[paramName]
			}
		}
	}

	return ""
}

// handleBindingCustom handles an extend binding with action: custom.
// It delegates to Bridge.Execute() with ActionCustom, passing the operation name
// from the binding and the parsed request body as input. Path params are merged
// into the input so custom operations can access resource IDs.
func (h *Handler) handleBindingCustom(w http.ResponseWriter, r *http.Request, binding *mock.StatefulBinding, pathParams map[string]string, bodyBytes []byte, responseCfg *config.ResponseTransform) int {
	if binding.Operation == "" {
		return h.writeStatefulError(w, http.StatusBadRequest, "extend binding with action: custom requires an operation name", binding.Table, "")
	}

	// Parse body (JSON or form-encoded). Allow empty body for action endpoints.
	var input map[string]interface{}
	if len(bodyBytes) > 0 {
		var err error
		input, err = parseStatefulBody(bodyBytes, r.Header.Get("Content-Type"))
		if err != nil {
			return h.writeStatefulError(w, http.StatusBadRequest, "invalid request body: "+err.Error(), binding.Table, "")
		}
	}
	if input == nil {
		input = make(map[string]interface{})
	}

	// Merge path params into input so custom operations can reference resource IDs
	for k, v := range pathParams {
		input[k] = v
	}

	result := h.statefulBridge.Execute(r.Context(), &stateful.OperationRequest{
		Action:        stateful.ActionCustom,
		OperationName: binding.Operation,
		Data:          input,
	})

	if result.Error != nil {
		return h.writeBindingError(w, result, binding.Table, "", responseCfg)
	}

	// Apply response transform if the custom operation returned an item
	if result.Item != nil {
		data := stateful.TransformItem(result.Item.ToJSON(), responseCfg)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(data)
		return http.StatusOK
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	return http.StatusOK
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
// "items[0][price]=price_123" → {"items":[{"price":"price_123"}]}
// "payment_method_types[0]=card" → {"payment_method_types":["card"]}
//
// Maps whose keys are consecutive numeric strings ("0","1","2",...) are
// automatically converted to slices so the result matches the JSON arrays
// that Stripe SDKs expect.
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
			result[key] = coerceFormValue(val)
		}
	}

	convertNumericKeysToArrays(result)
	return result
}

// convertNumericKeysToArrays recursively converts maps with consecutive numeric
// string keys ("0", "1", "2", ...) into slices. This handles form-encoded arrays
// like items[0][price]=X&items[1][price]=Y which url.ParseQuery produces as
// nested maps with "0" and "1" keys instead of Go slices.
func convertNumericKeysToArrays(m map[string]any) {
	for key, val := range m {
		if sub, ok := val.(map[string]any); ok {
			// Recurse first so nested maps are converted bottom-up.
			convertNumericKeysToArrays(sub)
			// Check if all keys are consecutive integers starting from 0.
			if isNumericKeyedMap(sub) {
				m[key] = numericMapToSlice(sub)
				// Recurse into the resulting slice elements too.
				if arr, ok := m[key].([]any); ok {
					for _, elem := range arr {
						if elemMap, ok := elem.(map[string]any); ok {
							convertNumericKeysToArrays(elemMap)
						}
					}
				}
			}
		}
	}
}

func isNumericKeyedMap(m map[string]any) bool {
	if len(m) == 0 {
		return false
	}
	for i := 0; i < len(m); i++ {
		if _, ok := m[strconv.Itoa(i)]; !ok {
			return false
		}
	}
	return true
}

func numericMapToSlice(m map[string]any) []any {
	result := make([]any, len(m))
	for i := 0; i < len(m); i++ {
		result[i] = m[strconv.Itoa(i)]
	}
	return result
}

// coerceFormValue attempts to convert a form-encoded string value to its
// natural Go type. Form data arrives as strings, but downstream consumers
// (e.g. the Stripe Go SDK) expect numeric and boolean JSON types.
func coerceFormValue(s string) any {
	// Null sentinels — form encoding has no null literal, so certain
	// string values conventionally represent null/nil. The most common
	// is "inf" (e.g., Stripe SDKs send tiers[N][up_to]=inf meaning
	// "unlimited", and the real API returns null in JSON responses).
	if s == "inf" {
		return nil
	}

	// Boolean
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Integer (no decimal point, no leading +/- sign that could be a phone number prefix)
	// We only coerce values that look purely numeric. Leading '+' is valid for
	// strconv.ParseInt but common in phone numbers (+15551234567), so reject it.
	if !strings.Contains(s, ".") && !strings.HasPrefix(s, "+") {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n
		}
	}

	// Float (has decimal point, no leading +)
	if strings.Contains(s, ".") && !strings.HasPrefix(s, "+") {
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
	}

	return s
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
			current[segment] = coerceFormValue(val)
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

// reservedQueryParams are query parameters that should NOT be treated as field filters.
// These are common pagination, sorting, expansion, and control params used across APIs.
var reservedQueryParams = map[string]bool{
	// Pagination
	"limit": true, "offset": true, "page": true, "per_page": true,
	"starting_after": true, "ending_before": true,
	"cursor": true, "page_size": true, "page_token": true,
	// Sorting
	"sort": true, "order": true, "sort_by": true, "order_by": true,
	// Expansion / field selection
	"expand": true, "expand[]": true, "fields": true, "include": true,
	"exclude": true, "select": true,
	// Other common non-filter params
	"format": true, "pretty": true, "api_version": true,
	"idempotency_key": true, "request_id": true,
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

	for key, values := range query {
		if !reservedQueryParams[key] && len(values) > 0 {
			filter.Filters[key] = values[0]
		}
	}

	return filter
}

// parseExpandFields extracts ?expand[] and ?expand query params from the request.
func parseExpandFields(r *http.Request) []string {
	expandFields := stateful.ParseExpandParam(r.URL.Query()["expand[]"])
	if len(expandFields) == 0 {
		expandFields = stateful.ParseExpandParam(r.URL.Query()["expand"])
	}
	return expandFields
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
