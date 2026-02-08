package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVerifyMock_NoEngine tests that handlers properly return errors when no engine is connected.
func TestVerifyMock_NoEngine(t *testing.T) {
	// Create admin API without engine client
	adminAPI := NewAdminAPI(0)

	t.Run("handleGetMockVerification returns no_engine error", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/mocks/test-id/verify", nil)
		req.SetPathValue("id", "test-id")
		w := httptest.NewRecorder()

		adminAPI.requireEngine(adminAPI.handleGetMockVerification)(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		var resp ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "no_engine", resp.Error)
	})

	t.Run("handleVerifyMock returns no_engine error", func(t *testing.T) {
		body := `{"exactly": 5}`
		req := httptest.NewRequest("POST", "/mocks/test-id/verify", bytes.NewBufferString(body))
		req.SetPathValue("id", "test-id")
		w := httptest.NewRecorder()

		adminAPI.requireEngine(adminAPI.handleVerifyMock)(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		var resp ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "no_engine", resp.Error)
	})

	t.Run("handleListMockInvocations returns no_engine error", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/mocks/test-id/invocations", nil)
		req.SetPathValue("id", "test-id")
		w := httptest.NewRecorder()

		adminAPI.requireEngine(adminAPI.handleListMockInvocations)(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		var resp ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "no_engine", resp.Error)
	})

	t.Run("handleResetMockVerification returns no_engine when no engine connected", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/mocks/test-id/invocations", nil)
		req.SetPathValue("id", "test-id")
		w := httptest.NewRecorder()

		adminAPI.requireEngine(adminAPI.handleResetMockVerification)(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		var resp ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "no_engine", resp.Error)
	})

	t.Run("handleResetAllVerification returns no_engine error", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/verify", nil)
		w := httptest.NewRecorder()

		adminAPI.requireEngine(adminAPI.handleResetAllVerification)(w, req)

		assert.Equal(t, http.StatusServiceUnavailable, w.Code)

		var resp ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.Equal(t, "no_engine", resp.Error)
	})
}

// TestEvaluateVerification tests the verification logic without needing an engine.
func TestEvaluateVerification(t *testing.T) {
	adminAPI := NewAdminAPI(0)

	t.Run("atLeast passes when count meets minimum", func(t *testing.T) {
		minVal := 3
		req := VerifyRequest{AtLeast: &minVal}
		resp := adminAPI.evaluateVerification(req, 5)

		assert.True(t, resp.Passed)
		assert.Equal(t, 5, resp.Actual)
		assert.Contains(t, resp.Expected, "at least 3")
	})

	t.Run("atLeast fails when count is below minimum", func(t *testing.T) {
		minVal := 5
		req := VerifyRequest{AtLeast: &minVal}
		resp := adminAPI.evaluateVerification(req, 2)

		assert.False(t, resp.Passed)
		assert.Equal(t, 2, resp.Actual)
		assert.Contains(t, resp.Message, "at least 5")
	})

	t.Run("atMost passes when count is within limit", func(t *testing.T) {
		maxVal := 5
		req := VerifyRequest{AtMost: &maxVal}
		resp := adminAPI.evaluateVerification(req, 3)

		assert.True(t, resp.Passed)
		assert.Equal(t, 3, resp.Actual)
	})

	t.Run("atMost fails when count exceeds limit", func(t *testing.T) {
		maxVal := 5
		req := VerifyRequest{AtMost: &maxVal}
		resp := adminAPI.evaluateVerification(req, 10)

		assert.False(t, resp.Passed)
		assert.Equal(t, 10, resp.Actual)
		assert.Contains(t, resp.Message, "at most 5")
	})

	t.Run("exactly passes when count matches", func(t *testing.T) {
		exactVal := 5
		req := VerifyRequest{Exactly: &exactVal}
		resp := adminAPI.evaluateVerification(req, 5)

		assert.True(t, resp.Passed)
		assert.Equal(t, 5, resp.Actual)
		assert.Contains(t, resp.Expected, "exactly 5")
	})

	t.Run("exactly fails when count does not match", func(t *testing.T) {
		exactVal := 5
		req := VerifyRequest{Exactly: &exactVal}
		resp := adminAPI.evaluateVerification(req, 3)

		assert.False(t, resp.Passed)
		assert.Equal(t, 3, resp.Actual)
		assert.Contains(t, resp.Message, "exactly 5")
	})

	t.Run("never passes when count is zero", func(t *testing.T) {
		neverVal := true
		req := VerifyRequest{Never: &neverVal}
		resp := adminAPI.evaluateVerification(req, 0)

		assert.True(t, resp.Passed)
		assert.Equal(t, 0, resp.Actual)
		assert.Contains(t, resp.Expected, "never")
	})

	t.Run("never fails when count is not zero", func(t *testing.T) {
		neverVal := true
		req := VerifyRequest{Never: &neverVal}
		resp := adminAPI.evaluateVerification(req, 2)

		assert.False(t, resp.Passed)
		assert.Equal(t, 2, resp.Actual)
		assert.Contains(t, resp.Message, "never be called")
	})

	t.Run("combined atLeast and atMost passes when in range", func(t *testing.T) {
		minVal := 3
		maxVal := 7
		req := VerifyRequest{AtLeast: &minVal, AtMost: &maxVal}
		resp := adminAPI.evaluateVerification(req, 5)

		assert.True(t, resp.Passed)
		assert.Equal(t, 5, resp.Actual)
	})

	t.Run("combined atLeast and atMost fails when below range", func(t *testing.T) {
		minVal := 3
		maxVal := 7
		req := VerifyRequest{AtLeast: &minVal, AtMost: &maxVal}
		resp := adminAPI.evaluateVerification(req, 1)

		assert.False(t, resp.Passed)
	})

	t.Run("combined atLeast and atMost fails when above range", func(t *testing.T) {
		minVal := 3
		maxVal := 7
		req := VerifyRequest{AtLeast: &minVal, AtMost: &maxVal}
		resp := adminAPI.evaluateVerification(req, 10)

		assert.False(t, resp.Passed)
	})
}
