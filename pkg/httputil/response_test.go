package httputil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	t.Run("writes JSON with correct content type", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		data := map[string]string{"foo": "bar"}

		WriteJSON(rec, http.StatusOK, data)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var result map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "bar", result["foo"])
	})

	t.Run("handles nil data", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()

		WriteJSON(rec, http.StatusNoContent, nil)

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("sets custom status codes", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()

		WriteJSON(rec, http.StatusCreated, map[string]string{"id": "123"})

		assert.Equal(t, http.StatusCreated, rec.Code)
	})
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	t.Run("writes error response with correct format", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()

		WriteError(rec, http.StatusBadRequest, "invalid_input", "Name is required")

		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

		var result map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "invalid_input", result["error"])
		assert.Equal(t, "Name is required", result["message"])
	})
}

func TestWriteErrorWithDetails(t *testing.T) {
	t.Parallel()

	t.Run("includes details in response", func(t *testing.T) {
		t.Parallel()
		rec := httptest.NewRecorder()
		details := map[string]string{"field": "email", "reason": "invalid format"}

		WriteErrorWithDetails(rec, http.StatusBadRequest, "validation_error", "Validation failed", details)

		assert.Equal(t, http.StatusBadRequest, rec.Code)

		var result map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "validation_error", result["error"])
		assert.Equal(t, "Validation failed", result["message"])
		assert.NotNil(t, result["details"])
	})
}

func TestWriteNoContent(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteNoContent(rec)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}

func TestWriteCreated(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteCreated(rec, map[string]string{"id": "new-123"})

	assert.Equal(t, http.StatusCreated, rec.Code)

	var result map[string]string
	err := json.Unmarshal(rec.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "new-123", result["id"])
}

func TestWriteOK(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteOK(rec, map[string]string{"status": "ok"})

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWriteBadRequest(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteBadRequest(rec, "bad_request", "Invalid input")

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestWriteNotFound(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteNotFound(rec, "not_found", "Resource not found")

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestWriteInternalError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteInternalError(rec, "internal_error", "Something went wrong")

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestWriteServiceUnavailable(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteServiceUnavailable(rec, "service_unavailable", "Engine offline")

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestWriteConflict(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteConflict(rec, "duplicate", "Resource already exists")

	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestWriteTooManyRequests(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteTooManyRequests(rec, "rate_limited", "Too many requests")

	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}
