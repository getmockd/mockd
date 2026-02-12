package admin

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

// handleGetMockVerification handles GET /mocks/{id}/verify.
// Returns call count and last called time for a specific mock.
func (a *API) handleGetMockVerification(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	// Check if mock exists
	_, err := engine.GetMock(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Mock not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "get mock for verification"))
		return
	}

	// Get invocations for this mock from request logs
	// Note: We need to filter by matched mock ID - the engine client ListRequests
	// doesn't currently support this filter, so we get all and filter client-side
	result, err := engine.ListRequests(ctx, nil)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "list requests for verification"))
		return
	}

	// Filter to get only requests that matched this mock
	callCount := 0
	var lastCalledAt *string
	for _, req := range result.Requests {
		if req.MatchedMockID == id {
			callCount++
			if lastCalledAt == nil {
				ts := req.Timestamp.String()
				lastCalledAt = &ts
			}
		}
	}

	verification := MockVerification{
		MockID:    id,
		CallCount: callCount,
	}

	// Note: lastCalledAt is intentionally unused here.
	// The timestamp is already captured in the verification struct via the query results.

	writeJSON(w, http.StatusOK, verification)
}

// handleVerifyMock handles POST /mocks/{id}/verify.
// Checks if mock was called according to specified criteria.
func (a *API) handleVerifyMock(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	// Check if mock exists
	_, err := engine.GetMock(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Mock not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "get mock for verify"))
		return
	}

	var req VerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", sanitizeJSONError(err, a.logger()))
		return
	}

	// Validate that at least one criterion is specified
	if req.AtLeast == nil && req.AtMost == nil && req.Exactly == nil && req.Never == nil {
		writeError(w, http.StatusBadRequest, "missing_criteria", "At least one verification criterion is required (atLeast, atMost, exactly, or never)")
		return
	}

	// Get call count for this mock
	result, err := engine.ListRequests(ctx, nil)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "list requests for verify"))
		return
	}

	// Count requests that matched this mock
	actualCount := 0
	for _, req := range result.Requests {
		if req.MatchedMockID == id {
			actualCount++
		}
	}

	response := a.evaluateVerification(req, actualCount)
	writeJSON(w, http.StatusOK, response)
}

// evaluateVerification evaluates the verification criteria against the actual call count.
func (a *API) evaluateVerification(req VerifyRequest, actualCount int) VerifyResponse {
	response := VerifyResponse{
		Passed: true,
		Actual: actualCount,
	}

	// Check "never" first as it's a special case
	if req.Never != nil && *req.Never {
		if actualCount != 0 {
			response.Passed = false
			response.Expected = "never (0 times)"
			response.Message = fmt.Sprintf("Expected mock to never be called, but it was called %d time(s)", actualCount)
			return response
		}
		response.Expected = "never (0 times)"
		response.Message = "Mock was never called as expected"
		return response
	}

	// Check "exactly"
	if req.Exactly != nil {
		expected := *req.Exactly
		if actualCount != expected {
			response.Passed = false
			response.Expected = fmt.Sprintf("exactly %d time(s)", expected)
			response.Message = fmt.Sprintf("Expected mock to be called exactly %d time(s), but it was called %d time(s)", expected, actualCount)
			return response
		}
		response.Expected = fmt.Sprintf("exactly %d time(s)", expected)
		response.Message = fmt.Sprintf("Mock was called exactly %d time(s) as expected", expected)
		return response
	}

	// Check "atLeast" and "atMost" (can be combined)
	var expectations []string
	var failures []string

	if req.AtLeast != nil {
		expected := *req.AtLeast
		expectations = append(expectations, fmt.Sprintf("at least %d time(s)", expected))
		if actualCount < expected {
			response.Passed = false
			failures = append(failures, fmt.Sprintf("expected at least %d call(s) but got %d", expected, actualCount))
		}
	}

	if req.AtMost != nil {
		expected := *req.AtMost
		expectations = append(expectations, fmt.Sprintf("at most %d time(s)", expected))
		if actualCount > expected {
			response.Passed = false
			failures = append(failures, fmt.Sprintf("expected at most %d call(s) but got %d", expected, actualCount))
		}
	}

	if len(expectations) > 0 {
		response.Expected = joinStrings(expectations, " and ")
	}

	if !response.Passed && len(failures) > 0 {
		response.Message = "Verification failed: " + joinStrings(failures, "; ")
	} else if response.Passed {
		response.Message = fmt.Sprintf("Mock was called %d time(s), matching expectations", actualCount)
	}

	return response
}

// handleListMockInvocations handles GET /mocks/{id}/invocations.
// Returns request history for a specific mock with pagination.
func (a *API) handleListMockInvocations(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	// Check if mock exists
	_, err := engine.GetMock(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Mock not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "get mock for invocations"))
		return
	}

	// Parse pagination parameters
	limit, offset := parsePaginationParams(r.URL.Query())

	// Get all requests and filter by mock ID
	result, err := engine.ListRequests(ctx, nil)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "list requests for invocations"))
		return
	}

	// Filter to get only requests that matched this mock
	var matchedRequests []*engineclient.RequestLogEntry
	for _, req := range result.Requests {
		if req.MatchedMockID == id {
			matchedRequests = append(matchedRequests, req)
		}
	}
	total := len(matchedRequests)

	// Apply pagination
	if offset > 0 && offset < len(matchedRequests) {
		matchedRequests = matchedRequests[offset:]
	} else if offset >= len(matchedRequests) {
		matchedRequests = nil
	}
	if limit > 0 && limit < len(matchedRequests) {
		matchedRequests = matchedRequests[:limit]
	}

	// Convert to MockInvocation format
	invocations := make([]MockInvocation, 0, len(matchedRequests))
	for _, req := range matchedRequests {
		inv := MockInvocation{
			ID:        req.ID,
			Timestamp: req.Timestamp,
			Method:    req.Method,
			Path:      req.Path,
			Body:      req.Body,
		}

		// Flatten multi-value headers to single-value for MockInvocation
		if len(req.Headers) > 0 {
			inv.Headers = make(map[string]string, len(req.Headers))
			for k, v := range req.Headers {
				if len(v) > 0 {
					inv.Headers[k] = v[0]
				}
			}
		}

		invocations = append(invocations, inv)
	}

	writeJSON(w, http.StatusOK, MockInvocationListResponse{
		Invocations: invocations,
		Count:       len(invocations),
		Total:       total,
	})
}

// handleResetMockVerification handles DELETE /mocks/{id}/invocations.
// Clears invocation history for a specific mock.
func (a *API) handleResetMockVerification(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing_id", "Mock ID is required")
		return
	}

	// Verify mock exists
	_, err := engine.GetMock(ctx, id)
	if err != nil {
		if errors.Is(err, engineclient.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "Mock not found")
			return
		}
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "get mock for reset verification"))
		return
	}

	count, err := engine.ClearRequestsByMockID(ctx, id)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "clear requests by mock ID"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Invocations cleared",
		"mockId":  id,
		"cleared": count,
	})
}

// handleResetAllVerification handles DELETE /verify.
// Clears all invocation history (same as clearing all request logs).
func (a *API) handleResetAllVerification(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	ctx := r.Context()

	count, err := engine.ClearRequests(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "engine_error", sanitizeEngineError(err, a.logger(), "clear all requests"))
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "All verification data cleared",
		"cleared": count,
	})
}

// MockInvocationClearer is an interface for loggers that support clearing by mock ID.
type MockInvocationClearer interface {
	ClearByMockID(mockID string)
}

// parsePaginationParams extracts limit and offset from query parameters.
func parsePaginationParams(query url.Values) (limit, offset int) {
	if limitStr := query.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr := query.Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}
	return
}

// joinStrings joins strings with a separator.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	if len(strs) == 1 {
		return strs[0]
	}
	var b strings.Builder
	b.WriteString(strs[0])
	for i := 1; i < len(strs); i++ {
		b.WriteString(sep)
		b.WriteString(strs[i])
	}
	return b.String()
}
