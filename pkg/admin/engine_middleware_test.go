package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequireEngine(t *testing.T) {
	t.Parallel()

	t.Run("returns 503 when no engine connected", func(t *testing.T) {
		t.Parallel()

		api := &API{
			localEngine: nil, // No engine
		}

		handlerCalled := false
		handler := api.requireEngine(func(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
			handlerCalled = true
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.False(t, handlerCalled, "handler should not be called when no engine")

		var result map[string]string
		err := json.Unmarshal(rec.Body.Bytes(), &result)
		require.NoError(t, err)
		assert.Equal(t, "no_engine", result["error"])
	})

	t.Run("calls handler when engine connected", func(t *testing.T) {
		t.Parallel()

		// Create a minimal mock engine client
		mockEngine := &engineclient.Client{}
		api := &API{
			localEngine: mockEngine,
		}

		handlerCalled := false
		var receivedEngine *engineclient.Client
		handler := api.requireEngine(func(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
			handlerCalled = true
			receivedEngine = engine
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, handlerCalled, "handler should be called when engine connected")
		assert.Equal(t, mockEngine, receivedEngine, "handler should receive the engine client")
	})
}

func TestRequireEngineOr(t *testing.T) {
	t.Parallel()

	t.Run("calls fallback when no engine connected", func(t *testing.T) {
		t.Parallel()

		api := &API{
			localEngine: nil,
		}

		handlerCalled := false
		fallbackCalled := false

		handler := api.requireEngineOr(
			func(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
				handlerCalled = true
			},
			func(w http.ResponseWriter, r *http.Request) {
				fallbackCalled = true
				w.WriteHeader(http.StatusAccepted) // Custom response
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusAccepted, rec.Code)
		assert.False(t, handlerCalled)
		assert.True(t, fallbackCalled)
	})

	t.Run("calls handler when engine connected", func(t *testing.T) {
		t.Parallel()

		mockEngine := &engineclient.Client{}
		api := &API{
			localEngine: mockEngine,
		}

		handlerCalled := false
		fallbackCalled := false

		handler := api.requireEngineOr(
			func(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
				handlerCalled = true
				w.WriteHeader(http.StatusOK)
			},
			func(w http.ResponseWriter, r *http.Request) {
				fallbackCalled = true
			},
		)

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.True(t, handlerCalled)
		assert.False(t, fallbackCalled)
	})
}

func TestHasEngine(t *testing.T) {
	t.Parallel()

	t.Run("returns false when no engine", func(t *testing.T) {
		t.Parallel()
		api := &API{localEngine: nil}
		assert.False(t, api.HasEngine())
	})

	t.Run("returns true when engine connected", func(t *testing.T) {
		t.Parallel()
		api := &API{localEngine: &engineclient.Client{}}
		assert.True(t, api.HasEngine())
	})
}

func TestWithEngine(t *testing.T) {
	t.Parallel()

	t.Run("returns nil and writes error when no engine", func(t *testing.T) {
		t.Parallel()

		api := &API{localEngine: nil}
		rec := httptest.NewRecorder()

		engine := api.withEngine(rec)

		assert.Nil(t, engine)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("returns engine when connected", func(t *testing.T) {
		t.Parallel()

		mockEngine := &engineclient.Client{}
		api := &API{localEngine: mockEngine}
		rec := httptest.NewRecorder()

		engine := api.withEngine(rec)

		assert.Equal(t, mockEngine, engine)
		assert.Equal(t, http.StatusOK, rec.Code) // No error written
	})
}
