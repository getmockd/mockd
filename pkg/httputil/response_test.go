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

func TestWriteNoContent(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteNoContent(rec)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Empty(t, rec.Body.String())
}
