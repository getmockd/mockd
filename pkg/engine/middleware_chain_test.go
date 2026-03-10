package engine

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/chaos"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// NewMiddlewareChain Tests
// ============================================================================

func TestMiddlewareChain_New(t *testing.T) {
	t.Parallel()

	t.Run("empty config creates chain with no middleware", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)
		require.NotNil(t, mc)
		assert.Nil(t, mc.validator, "validator should be nil with empty config")
		assert.Nil(t, mc.auditLogger, "auditLogger should be nil with empty config")
		assert.Nil(t, mc.tracer, "tracer should be nil with empty config")
		assert.Nil(t, mc.chaosInjector.Load(), "chaosInjector should be nil with empty config")
	})

	t.Run("with chain tracer option", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg, WithChainTracer(nil))
		require.NoError(t, err)
		require.NotNil(t, mc)
		assert.Nil(t, mc.tracer, "nil tracer option should leave tracer nil")
	})
}

// ============================================================================
// Wrap Tests
// ============================================================================

func TestMiddlewareChain_Wrap(t *testing.T) {
	t.Parallel()

	t.Run("passes request through to inner handler", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		called := false
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})

		wrapped := mc.Wrap(inner)
		require.NotNil(t, wrapped)

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		wrapped.ServeHTTP(rec, req)

		assert.True(t, called, "inner handler should be called")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "ok", rec.Body.String())
	})

	t.Run("nil middleware still passes through", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		// Confirm all middleware are nil/unset
		assert.Nil(t, mc.validator)
		assert.Nil(t, mc.auditLogger)
		assert.Nil(t, mc.tracer)
		assert.Nil(t, mc.chaosInjector.Load())

		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTeapot)
		})

		wrapped := mc.Wrap(inner)
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/resource", nil)
		wrapped.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusTeapot, rec.Code)
	})

	t.Run("wrapped handler serves full HTTP request", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Custom", "value")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"123"}`))
		})

		wrapped := mc.Wrap(inner)

		// Use httptest.Server for a more realistic HTTP test
		srv := httptest.NewServer(wrapped)
		defer srv.Close()

		resp, err := http.Get(srv.URL + "/api/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.Equal(t, "value", resp.Header.Get("X-Custom"))
		assert.Equal(t, `{"id":"123"}`, string(body))
	})
}

// ============================================================================
// Chaos Injector Tests
// ============================================================================

func TestMiddlewareChain_ChaosEnabled(t *testing.T) {
	t.Parallel()

	t.Run("returns false when no injector set", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		assert.False(t, mc.ChaosEnabled())
	})

	t.Run("returns true after SetChaosInjector with enabled injector", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		injector, err := chaos.NewInjector(&chaos.ChaosConfig{Enabled: true})
		require.NoError(t, err)

		mc.SetChaosInjector(injector)

		assert.True(t, mc.ChaosEnabled())
		assert.Same(t, injector, mc.ChaosInjector())
	})

	t.Run("returns false after SetChaosInjector with nil", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		// First set an enabled injector
		injector, err := chaos.NewInjector(&chaos.ChaosConfig{Enabled: true})
		require.NoError(t, err)
		mc.SetChaosInjector(injector)
		require.True(t, mc.ChaosEnabled())

		// Then set nil
		mc.SetChaosInjector(nil)
		assert.False(t, mc.ChaosEnabled())
		assert.Nil(t, mc.ChaosInjector())
	})

	t.Run("returns false with disabled injector", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		injector, err := chaos.NewInjector(&chaos.ChaosConfig{Enabled: false})
		require.NoError(t, err)

		mc.SetChaosInjector(injector)
		assert.False(t, mc.ChaosEnabled())
	})
}

// ============================================================================
// Validation Tests
// ============================================================================

func TestMiddlewareChain_ValidationEnabled(t *testing.T) {
	t.Parallel()

	t.Run("returns false when no validator", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		assert.False(t, mc.ValidationEnabled())
		assert.Nil(t, mc.Validator())
	})
}

// ============================================================================
// Tracer Tests
// ============================================================================

func TestMiddlewareChain_Tracer(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when no tracer set", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		assert.Nil(t, mc.Tracer())
	})

	t.Run("SetTracer updates tracer", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		mc.SetTracer(nil)
		assert.Nil(t, mc.Tracer())
	})
}

// ============================================================================
// AuditLogger Tests
// ============================================================================

func TestMiddlewareChain_AuditLogger(t *testing.T) {
	t.Parallel()

	t.Run("returns nil when no audit logger", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		assert.Nil(t, mc.AuditLogger())
	})
}

// ============================================================================
// Close Tests
// ============================================================================

func TestMiddlewareChain_Close(t *testing.T) {
	t.Parallel()

	t.Run("nil audit logger returns no error", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		err = mc.Close()
		assert.NoError(t, err)
	})
}

// ============================================================================
// dynamicChaosHandler Tests
// ============================================================================

func TestDynamicChaosHandler(t *testing.T) {
	t.Parallel()

	t.Run("passes through when chaos injector is nil", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		called := false
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})

		handler := &dynamicChaosHandler{chain: mc, handler: inner}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		handler.ServeHTTP(rec, req)

		assert.True(t, called, "inner handler should be called when chaos is nil")
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("passes through when chaos injector is disabled", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		// Set a disabled injector
		injector, err := chaos.NewInjector(&chaos.ChaosConfig{Enabled: false})
		require.NoError(t, err)
		mc.SetChaosInjector(injector)

		called := false
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusAccepted)
		})

		handler := &dynamicChaosHandler{chain: mc, handler: inner}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		handler.ServeHTTP(rec, req)

		assert.True(t, called, "inner handler should be called when chaos is disabled")
		assert.Equal(t, http.StatusAccepted, rec.Code)
	})

	t.Run("invokes chaos middleware when chaos is enabled", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{}
		mc, err := NewMiddlewareChain(cfg)
		require.NoError(t, err)

		// Enable chaos with no rules/faults — the chaos middleware wraps but
		// with no faults configured, requests still pass through to the inner handler.
		injector, err := chaos.NewInjector(&chaos.ChaosConfig{Enabled: true})
		require.NoError(t, err)
		mc.SetChaosInjector(injector)

		called := false
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("chaos-pass"))
		})

		handler := &dynamicChaosHandler{chain: mc, handler: inner}

		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		handler.ServeHTTP(rec, req)

		// With no faults, the chaos middleware passes through to the inner handler
		assert.True(t, called, "inner handler should be called when chaos has no faults")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "chaos-pass", rec.Body.String())
	})
}
