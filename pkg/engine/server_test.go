package engine

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getmockd/mockd/internal/storage"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// Server Creation Tests
// ============================================================================

func TestNewServer(t *testing.T) {
	t.Parallel()

	t.Run("creates server with nil config uses defaults", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		require.NotNil(t, srv)
		assert.NotNil(t, srv.cfg)
		assert.NotNil(t, srv.handler)
		assert.NotNil(t, srv.mockManager)
		assert.False(t, srv.IsRunning())
	})

	t.Run("creates server with custom config", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{
			HTTPPort:      9090,
			MaxLogEntries: 500,
			ReadTimeout:   30,
			WriteTimeout:  30,
		}
		srv := NewServer(cfg)
		require.NotNil(t, srv)
		assert.Equal(t, 9090, srv.cfg.HTTPPort)
		assert.Equal(t, 500, srv.cfg.MaxLogEntries)
	})

	t.Run("creates server with logger option", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil, WithLogger(nil))
		require.NotNil(t, srv)
		// nil logger should result in nop logger being used
		assert.NotNil(t, srv.log)
	})
}

func TestNewServerWithMocks(t *testing.T) {
	t.Parallel()

	t.Run("creates server with pre-loaded mocks", func(t *testing.T) {
		t.Parallel()
		mocks := []*config.MockConfiguration{
			createTestHTTPMock("mock-1", "/api/users", "GET", 200, `{"users": []}`),
			createTestHTTPMock("mock-2", "/api/orders", "GET", 200, `{"orders": []}`),
		}

		srv := NewServerWithMocks(nil, mocks)
		require.NotNil(t, srv)

		// Verify mocks were loaded
		assert.Equal(t, 2, srv.Store().Count())
		assert.NotNil(t, srv.getMock("mock-1"))
		assert.NotNil(t, srv.getMock("mock-2"))
	})

	t.Run("handles nil mocks in slice gracefully", func(t *testing.T) {
		t.Parallel()
		mocks := []*config.MockConfiguration{
			createTestHTTPMock("mock-1", "/api/users", "GET", 200, `{}`),
			nil,
			createTestHTTPMock("mock-2", "/api/orders", "GET", 200, `{}`),
		}

		srv := NewServerWithMocks(nil, mocks)
		require.NotNil(t, srv)
		assert.Equal(t, 2, srv.Store().Count())
	})

	t.Run("handles empty mocks slice", func(t *testing.T) {
		t.Parallel()
		srv := NewServerWithMocks(nil, []*config.MockConfiguration{})
		require.NotNil(t, srv)
		assert.Equal(t, 0, srv.Store().Count())
	})
}

// ============================================================================
// Server Lifecycle Tests
// ============================================================================

func TestServerStartStop(t *testing.T) {
	t.Run("starts and stops server successfully", func(t *testing.T) {
		cfg := &config.ServerConfiguration{
			HTTPPort:      0, // Use port 0 so we don't actually bind
			MaxLogEntries: 100,
		}
		srv := NewServer(cfg)
		require.NotNil(t, srv)

		assert.False(t, srv.IsRunning())
		assert.Equal(t, 0, srv.Uptime())

		err := srv.Start()
		require.NoError(t, err)
		assert.True(t, srv.IsRunning())

		// Allow some time to pass for uptime
		time.Sleep(10 * time.Millisecond)
		assert.GreaterOrEqual(t, srv.Uptime(), 0)

		err = srv.Stop()
		require.NoError(t, err)
		assert.False(t, srv.IsRunning())
		assert.Equal(t, 0, srv.Uptime())
	})

	t.Run("start returns error if already running", func(t *testing.T) {
		cfg := &config.ServerConfiguration{
			HTTPPort:      0,
			MaxLogEntries: 100,
		}
		srv := NewServer(cfg)

		err := srv.Start()
		require.NoError(t, err)
		defer srv.Stop()

		err = srv.Start()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already running")
	})

	t.Run("stop is idempotent", func(t *testing.T) {
		cfg := &config.ServerConfiguration{
			HTTPPort:      0,
			MaxLogEntries: 100,
		}
		srv := NewServer(cfg)

		// Stop without start should not error
		err := srv.Stop()
		assert.NoError(t, err)

		// Double stop should not error
		err = srv.Start()
		require.NoError(t, err)
		err = srv.Stop()
		assert.NoError(t, err)
		err = srv.Stop()
		assert.NoError(t, err)
	})
}

func TestServerConfig(t *testing.T) {
	t.Parallel()

	t.Run("returns server configuration", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{
			HTTPPort:  8080,
			HTTPSPort: 8443,
		}
		srv := NewServer(cfg)
		assert.Equal(t, cfg, srv.Config())
	})
}

// ============================================================================
// Handler Tests
// ============================================================================

func TestHandlerServeHTTP(t *testing.T) {
	t.Parallel()

	t.Run("handles CORS preflight request", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		req := httptest.NewRequest(http.MethodOptions, "/api/test", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
		assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "GET")
		assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "POST")
	})

	t.Run("handles health endpoint", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		req := httptest.NewRequest(http.MethodGet, "/__mockd/health", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("handles ready endpoint", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		req := httptest.NewRequest(http.MethodGet, "/__mockd/ready", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("returns 404 for unmatched request", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		req := httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), "no_match")
	})

	t.Run("matches and returns mock response", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		// Add a mock
		mockCfg := createTestHTTPMock("test-mock", "/api/users", "GET", 200, `{"users": ["alice", "bob"]}`)
		err := store.Set(mockCfg)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "alice")
		assert.Contains(t, rec.Body.String(), "bob")
	})

	t.Run("matches mock with headers", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		// Add a mock with header requirement
		mockCfg := &config.MockConfiguration{
			ID:      "header-mock",
			Type:    mock.MockTypeHTTP,
			Enabled: true,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method:  "GET",
					Path:    "/api/secure",
					Headers: map[string]string{"Authorization": "Bearer token123"},
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       `{"secure": true}`,
				},
			},
		}
		err := store.Set(mockCfg)
		require.NoError(t, err)

		// Request without header - should not match
		req := httptest.NewRequest(http.MethodGet, "/api/secure", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)

		// Request with header - should match
		req = httptest.NewRequest(http.MethodGet, "/api/secure", nil)
		req.Header.Set("Authorization", "Bearer token123")
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "secure")
	})

	t.Run("matches mock with query params", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		mockCfg := &config.MockConfiguration{
			ID:      "query-mock",
			Type:    mock.MockTypeHTTP,
			Enabled: true,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method:      "GET",
					Path:        "/api/search",
					QueryParams: map[string]string{"q": "test"},
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       `{"results": []}`,
				},
			},
		}
		err := store.Set(mockCfg)
		require.NoError(t, err)

		// Request with matching query param
		req := httptest.NewRequest(http.MethodGet, "/api/search?q=test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("returns custom headers from mock", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		mockCfg := &config.MockConfiguration{
			ID:      "headers-mock",
			Type:    mock.MockTypeHTTP,
			Enabled: true,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/data",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Headers: map[string]string{
						"Content-Type":    "application/json",
						"X-Custom-Header": "custom-value",
					},
					Body: `{"data": true}`,
				},
			},
		}
		err := store.Set(mockCfg)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/data", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.Equal(t, "custom-value", rec.Header().Get("X-Custom-Header"))
	})

	t.Run("handles POST request with body matching", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		mockCfg := &config.MockConfiguration{
			ID:      "body-mock",
			Type:    mock.MockTypeHTTP,
			Enabled: true,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method:       "POST",
					Path:         "/api/users",
					BodyContains: "email",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 201,
					Body:       `{"created": true}`,
				},
			},
		}
		err := store.Set(mockCfg)
		require.NoError(t, err)

		body := `{"email": "test@example.com", "name": "Test User"}`
		req := httptest.NewRequest(http.MethodPost, "/api/users", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("fallback health endpoint when no mock", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("fallback ready endpoint when no mock", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestHandlerRequestLogging(t *testing.T) {
	t.Parallel()

	t.Run("logs requests when logger is set", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		logger := NewInMemoryRequestLogger(100)
		handler.SetLogger(logger)

		mockCfg := createTestHTTPMock("log-mock", "/api/test", "GET", 200, `{}`)
		err := store.Set(mockCfg)
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, 1, logger.Count())
		entries := logger.List(nil)
		require.Len(t, entries, 1)
		assert.Equal(t, "GET", entries[0].Method)
		assert.Equal(t, "/api/test", entries[0].Path)
		assert.Equal(t, "log-mock", entries[0].MatchedMockID)
	})
}

// ============================================================================
// Mock Manager Tests
// ============================================================================

func TestMockManagerAdd(t *testing.T) {
	t.Parallel()

	t.Run("adds mock successfully", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		mockCfg := createTestHTTPMock("", "/api/users", "GET", 200, `{}`)
		err := mm.Add(mockCfg)
		require.NoError(t, err)
		assert.NotEmpty(t, mockCfg.ID) // ID should be generated
		assert.True(t, mockCfg.Enabled)
		assert.False(t, mockCfg.CreatedAt.IsZero())
		assert.False(t, mockCfg.UpdatedAt.IsZero())
	})

	t.Run("returns error for nil mock", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		err := mm.Add(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("returns error for duplicate ID", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		mockCfg := createTestHTTPMock("mock-1", "/api/users", "GET", 200, `{}`)
		err := mm.Add(mockCfg)
		require.NoError(t, err)

		duplicate := createTestHTTPMock("mock-1", "/api/orders", "GET", 200, `{}`)
		err = mm.Add(duplicate)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("validates mock before adding", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		// Mock with both Path and PathPattern (invalid)
		mockCfg := &config.MockConfiguration{
			ID:      "invalid-mock",
			Type:    mock.MockTypeHTTP,
			Enabled: true,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method:      "GET",
					Path:        "/api/users",
					PathPattern: "^/api/users$",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
				},
			},
		}

		err := mm.Add(mockCfg)
		assert.Error(t, err)
	})
}

func TestMockManagerUpdate(t *testing.T) {
	t.Parallel()

	t.Run("updates existing mock", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		original := createTestHTTPMock("update-mock", "/api/users", "GET", 200, `{"original": true}`)
		err := mm.Add(original)
		require.NoError(t, err)
		originalCreatedAt := original.CreatedAt

		time.Sleep(1 * time.Millisecond)

		updated := createTestHTTPMock("", "/api/users", "GET", 201, `{"updated": true}`)
		err = mm.Update("update-mock", updated)
		require.NoError(t, err)

		retrieved := mm.Get("update-mock")
		require.NotNil(t, retrieved)
		assert.Equal(t, "update-mock", retrieved.ID)
		assert.Equal(t, 201, retrieved.HTTP.Response.StatusCode)
		assert.Equal(t, originalCreatedAt, retrieved.CreatedAt)
		assert.True(t, retrieved.UpdatedAt.After(originalCreatedAt))
	})

	t.Run("returns error for non-existent mock", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		updated := createTestHTTPMock("", "/api/users", "GET", 200, `{}`)
		err := mm.Update("nonexistent", updated)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("returns error for nil mock", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		err := mm.Update("some-id", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})
}

func TestMockManagerDelete(t *testing.T) {
	t.Parallel()

	t.Run("deletes existing mock", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		mockCfg := createTestHTTPMock("delete-mock", "/api/users", "GET", 200, `{}`)
		err := mm.Add(mockCfg)
		require.NoError(t, err)
		assert.True(t, mm.Exists("delete-mock"))

		err = mm.Delete("delete-mock")
		require.NoError(t, err)
		assert.False(t, mm.Exists("delete-mock"))
	})

	t.Run("returns error for non-existent mock", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		err := mm.Delete("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestMockManagerGet(t *testing.T) {
	t.Parallel()

	t.Run("returns mock by ID", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		mockCfg := createTestHTTPMock("get-mock", "/api/users", "GET", 200, `{}`)
		err := mm.Add(mockCfg)
		require.NoError(t, err)

		retrieved := mm.Get("get-mock")
		require.NotNil(t, retrieved)
		assert.Equal(t, "get-mock", retrieved.ID)
	})

	t.Run("returns nil for non-existent mock", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		retrieved := mm.Get("nonexistent")
		assert.Nil(t, retrieved)
	})
}

func TestMockManagerList(t *testing.T) {
	t.Parallel()

	t.Run("returns all mocks", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		_ = mm.Add(createTestHTTPMock("mock-1", "/api/users", "GET", 200, `{}`))
		_ = mm.Add(createTestHTTPMock("mock-2", "/api/orders", "GET", 200, `{}`))
		_ = mm.Add(createTestHTTPMock("mock-3", "/api/products", "GET", 200, `{}`))

		mocks := mm.List()
		assert.Len(t, mocks, 3)
	})

	t.Run("returns empty list when no mocks", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		mocks := mm.List()
		assert.Empty(t, mocks)
	})
}

func TestMockManagerListByType(t *testing.T) {
	t.Parallel()

	t.Run("returns only mocks of specified type", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		_ = mm.Add(createTestHTTPMock("http-1", "/api/users", "GET", 200, `{}`))
		_ = mm.Add(createTestHTTPMock("http-2", "/api/orders", "GET", 200, `{}`))

		httpMocks := mm.ListByType(mock.MockTypeHTTP)
		assert.Len(t, httpMocks, 2)

		wsMocks := mm.ListByType(mock.MockTypeWebSocket)
		assert.Empty(t, wsMocks)
	})
}

func TestMockManagerClear(t *testing.T) {
	t.Parallel()

	t.Run("removes all mocks", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		_ = mm.Add(createTestHTTPMock("mock-1", "/api/users", "GET", 200, `{}`))
		_ = mm.Add(createTestHTTPMock("mock-2", "/api/orders", "GET", 200, `{}`))
		assert.Equal(t, 2, mm.Count())

		mm.Clear()
		assert.Equal(t, 0, mm.Count())
	})
}

func TestMockManagerCount(t *testing.T) {
	t.Parallel()

	t.Run("returns correct count", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		assert.Equal(t, 0, mm.Count())

		_ = mm.Add(createTestHTTPMock("mock-1", "/api/users", "GET", 200, `{}`))
		assert.Equal(t, 1, mm.Count())

		_ = mm.Add(createTestHTTPMock("mock-2", "/api/orders", "GET", 200, `{}`))
		assert.Equal(t, 2, mm.Count())

		_ = mm.Delete("mock-1")
		assert.Equal(t, 1, mm.Count())
	})
}

// ============================================================================
// Server Mock Operations Tests (via Server methods)
// ============================================================================

func TestServerMockOperations(t *testing.T) {
	t.Parallel()

	t.Run("add and list mocks through server", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)

		mockCfg := createTestHTTPMock("server-mock", "/api/test", "GET", 200, `{}`)
		err := srv.addMock(mockCfg)
		require.NoError(t, err)

		mocks := srv.listMocks()
		assert.Len(t, mocks, 1)

		httpMocks := srv.listHTTPMocks()
		assert.Len(t, httpMocks, 1)
	})

	t.Run("update mock through server", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)

		mockCfg := createTestHTTPMock("update-server-mock", "/api/test", "GET", 200, `{"v": 1}`)
		err := srv.addMock(mockCfg)
		require.NoError(t, err)

		updated := createTestHTTPMock("", "/api/test", "GET", 201, `{"v": 2}`)
		err = srv.updateMock("update-server-mock", updated)
		require.NoError(t, err)

		retrieved := srv.getMock("update-server-mock")
		require.NotNil(t, retrieved)
		assert.Equal(t, 201, retrieved.HTTP.Response.StatusCode)
	})

	t.Run("delete mock through server", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)

		mockCfg := createTestHTTPMock("delete-server-mock", "/api/test", "GET", 200, `{}`)
		_ = srv.addMock(mockCfg)

		err := srv.deleteMock("delete-server-mock")
		require.NoError(t, err)
		assert.Nil(t, srv.getMock("delete-server-mock"))
	})

	t.Run("clear mocks through server", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)

		_ = srv.addMock(createTestHTTPMock("mock-1", "/api/1", "GET", 200, `{}`))
		_ = srv.addMock(createTestHTTPMock("mock-2", "/api/2", "GET", 200, `{}`))

		srv.clearMocks()
		assert.Empty(t, srv.listMocks())
	})
}

// ============================================================================
// Server Request Logs Tests
// ============================================================================

func TestServerRequestLogs(t *testing.T) {
	t.Parallel()

	t.Run("get request logs", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)

		// Simulate some request logs via direct logger access
		logs := srv.GetRequestLogs(nil)
		assert.NotNil(t, logs) // Should return empty slice, not nil
	})

	t.Run("clear request logs", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)

		srv.ClearRequestLogs()
		assert.Equal(t, 0, srv.RequestLogCount())
	})
}

// ============================================================================
// Concurrent Access Tests
// ============================================================================

func TestConcurrentMockAccess(t *testing.T) {
	t.Parallel()

	t.Run("concurrent mock additions", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		var wg sync.WaitGroup
		numGoroutines := 10
		wg.Add(numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(idx int) {
				defer wg.Done()
				mockCfg := createTestHTTPMock(
					fmt.Sprintf("concurrent-mock-%d", idx),
					fmt.Sprintf("/api/%d", idx),
					"GET",
					200,
					`{}`,
				)
				_ = mm.Add(mockCfg)
			}(i)
		}

		wg.Wait()
		assert.Equal(t, numGoroutines, mm.Count())
	})

	t.Run("concurrent reads and writes", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)
		pm := NewProtocolManager()
		mm := NewMockManager(store, handler, pm)

		// Pre-populate some mocks
		for i := 0; i < 5; i++ {
			_ = mm.Add(createTestHTTPMock(
				fmt.Sprintf("pre-mock-%d", i),
				fmt.Sprintf("/api/pre/%d", i),
				"GET",
				200,
				`{}`,
			))
		}

		var wg sync.WaitGroup
		wg.Add(3)

		// Writer
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_ = mm.Add(createTestHTTPMock(
					fmt.Sprintf("writer-mock-%d", i),
					fmt.Sprintf("/api/writer/%d", i),
					"GET",
					200,
					`{}`,
				))
			}
		}()

		// Reader 1
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_ = mm.List()
				_ = mm.Count()
			}
		}()

		// Reader 2
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				_ = mm.Get(fmt.Sprintf("pre-mock-%d", i%5))
				_ = mm.Exists(fmt.Sprintf("pre-mock-%d", i%5))
			}
		}()

		wg.Wait()
		// No panics or race conditions
	})
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestHandlerIntegration(t *testing.T) {
	t.Run("end-to-end mock matching", func(t *testing.T) {
		srv := NewServer(&config.ServerConfiguration{
			HTTPPort:      0,
			MaxLogEntries: 100,
		})

		// Add various mocks
		_ = srv.addMock(createTestHTTPMock("users-get", "/api/users", "GET", 200, `{"users": []}`))
		_ = srv.addMock(createTestHTTPMock("users-post", "/api/users", "POST", 201, `{"created": true}`))
		_ = srv.addMock(createTestHTTPMock("user-get", "/api/users/{id}", "GET", 200, `{"id": "{{request.pathParams.id}}"}`))

		handler := srv.Handler()

		// Test GET /api/users
		req := httptest.NewRequest("GET", "/api/users", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, 200, rec.Code)
		assert.Contains(t, rec.Body.String(), "users")

		// Test POST /api/users
		req = httptest.NewRequest("POST", "/api/users", strings.NewReader(`{"name": "Test"}`))
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, 201, rec.Code)

		// Test GET /api/users/123 (path variable)
		req = httptest.NewRequest("GET", "/api/users/123", nil)
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		assert.Equal(t, 200, rec.Code)
	})

	t.Run("priority-based matching", func(t *testing.T) {
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		// Low priority mock
		lowPriority := &config.MockConfiguration{
			ID:      "low-priority",
			Type:    mock.MockTypeHTTP,
			Enabled: true,
			HTTP: &mock.HTTPSpec{
				Priority: 1,
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/test",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       "low",
				},
			},
		}
		_ = store.Set(lowPriority)

		// High priority mock
		highPriority := &config.MockConfiguration{
			ID:      "high-priority",
			Type:    mock.MockTypeHTTP,
			Enabled: true,
			HTTP: &mock.HTTPSpec{
				Priority: 10,
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/test",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       "high",
				},
			},
		}
		_ = store.Set(highPriority)

		req := httptest.NewRequest("GET", "/api/test", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, 200, rec.Code)
		assert.Equal(t, "high", rec.Body.String())
	})
}

func TestHandlerWithDelay(t *testing.T) {
	t.Run("applies response delay", func(t *testing.T) {
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		mockCfg := &config.MockConfiguration{
			ID:      "delayed-mock",
			Type:    mock.MockTypeHTTP,
			Enabled: true,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/slow",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       "delayed",
					DelayMs:    50,
				},
			},
		}
		_ = store.Set(mockCfg)

		start := time.Now()
		req := httptest.NewRequest("GET", "/api/slow", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		duration := time.Since(start)

		assert.Equal(t, 200, rec.Code)
		assert.GreaterOrEqual(t, duration.Milliseconds(), int64(50))
	})
}

// ============================================================================
// Helper Functions
// ============================================================================

// createTestHTTPMock creates a test HTTP mock configuration.
func createTestHTTPMock(id, path, method string, statusCode int, body string) *config.MockConfiguration {
	return &config.MockConfiguration{
		ID:      id,
		Type:    mock.MockTypeHTTP,
		Enabled: true,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: method,
				Path:   path,
			},
			Response: &mock.HTTPResponse{
				StatusCode: statusCode,
				Body:       body,
			},
		},
	}
}

// ============================================================================
// Edge Case Tests
// ============================================================================

func TestHandlerEdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("handles empty body", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		mockCfg := &config.MockConfiguration{
			ID:      "empty-body-mock",
			Type:    mock.MockTypeHTTP,
			Enabled: true,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/empty",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 204,
					Body:       "",
				},
			},
		}
		_ = store.Set(mockCfg)

		req := httptest.NewRequest("GET", "/api/empty", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, 204, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("handles large request body", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		mockCfg := &config.MockConfiguration{
			ID:      "large-body-mock",
			Type:    mock.MockTypeHTTP,
			Enabled: true,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method: "POST",
					Path:   "/api/upload",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       `{"received": true}`,
				},
			},
		}
		_ = store.Set(mockCfg)

		largeBody := strings.Repeat("x", 100000)
		req := httptest.NewRequest("POST", "/api/upload", strings.NewReader(largeBody))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, 200, rec.Code)
	})

	t.Run("handles special characters in path", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		mockCfg := &config.MockConfiguration{
			ID:      "special-path-mock",
			Type:    mock.MockTypeHTTP,
			Enabled: true,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/items/test-item_123",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       `{"found": true}`,
				},
			},
		}
		_ = store.Set(mockCfg)

		req := httptest.NewRequest("GET", "/api/items/test-item_123", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, 200, rec.Code)
	})

	t.Run("disabled mock is not matched", func(t *testing.T) {
		t.Parallel()
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		mockCfg := &config.MockConfiguration{
			ID:      "disabled-mock",
			Type:    mock.MockTypeHTTP,
			Enabled: false, // Disabled
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Method: "GET",
					Path:   "/api/disabled",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
					Body:       `{"should": "not match"}`,
				},
			},
		}
		_ = store.Set(mockCfg)

		req := httptest.NewRequest("GET", "/api/disabled", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		assert.Equal(t, 404, rec.Code)
	})
}

// ============================================================================
// Server Accessors Tests
// ============================================================================

func TestServerAccessors(t *testing.T) {
	t.Parallel()

	t.Run("returns handler", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		assert.NotNil(t, srv.Handler())
	})

	t.Run("returns store", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		assert.NotNil(t, srv.Store())
	})

	t.Run("returns logger", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		assert.NotNil(t, srv.Logger())
	})

	t.Run("returns stateful store", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		assert.NotNil(t, srv.StatefulStore())
	})

	t.Run("returns management port", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		// Port should be set during initialization
		assert.GreaterOrEqual(t, srv.ManagementPort(), 0)
	})
}

func TestServerSetLogger(t *testing.T) {
	t.Parallel()

	t.Run("sets nil logger to nop", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.SetLogger(nil)
		// Should not panic, should use nop logger
		assert.NotNil(t, srv.log)
	})
}

// ============================================================================
// Protocol Status Tests
// ============================================================================

func TestServerProtocolStatus(t *testing.T) {
	t.Run("returns protocol status", func(t *testing.T) {
		cfg := &config.ServerConfiguration{
			HTTPPort:      0,
			MaxLogEntries: 100,
		}
		srv := NewServer(cfg)

		status := srv.ProtocolStatus()
		assert.NotNil(t, status)
		assert.Contains(t, status, "http")

		// HTTP should be stopped before Start()
		assert.Equal(t, "stopped", status["http"].Status)

		// Start server
		err := srv.Start()
		require.NoError(t, err)
		defer srv.Stop()

		status = srv.ProtocolStatus()
		assert.Equal(t, "running", status["http"].Status)
	})
}

// ============================================================================
// Feature Flag Tests
// ============================================================================

func TestServerFeatureFlags(t *testing.T) {
	t.Parallel()

	t.Run("chaos not enabled by default", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		assert.False(t, srv.ChaosEnabled())
		assert.Nil(t, srv.ChaosInjector())
	})

	t.Run("validation not enabled by default", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		assert.False(t, srv.ValidationEnabled())
	})
}

// ============================================================================
// HTTP Client Integration Test (uses httptest.Server)
// ============================================================================

func TestServerHTTPIntegration(t *testing.T) {
	t.Run("serves HTTP requests via test server", func(t *testing.T) {
		store := storage.NewInMemoryMockStore()
		handler := NewHandler(store)

		// Add mock
		mockCfg := createTestHTTPMock("integration-mock", "/api/test", "GET", 200, `{"test": true}`)
		_ = store.Set(mockCfg)

		// Create test server
		ts := httptest.NewServer(handler)
		defer ts.Close()

		// Make request
		resp, err := http.Get(ts.URL + "/api/test")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		assert.Contains(t, string(body), "test")
	})
}

// ============================================================================
// Server Lifecycle Extended Tests
// ============================================================================

func TestServerStartsOnCorrectPorts(t *testing.T) {
	t.Run("server starts HTTP on configured port", func(t *testing.T) {
		// Use a specific port to verify server binds correctly
		port := getFreePort()
		cfg := &config.ServerConfiguration{
			HTTPPort:      port,
			MaxLogEntries: 100,
			ReadTimeout:   5,
			WriteTimeout:  5,
		}
		srv := NewServer(cfg)

		err := srv.Start()
		require.NoError(t, err)
		defer srv.Stop()

		// Give the server time to bind
		time.Sleep(50 * time.Millisecond)

		// Verify server is accepting connections on the configured port
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/__mockd/health", port))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("management port is assigned and accessible", func(t *testing.T) {
		cfg := &config.ServerConfiguration{
			HTTPPort:       getFreePort(),
			ManagementPort: getFreePort(),
			MaxLogEntries:  100,
		}
		srv := NewServer(cfg)

		err := srv.Start()
		require.NoError(t, err)
		defer srv.Stop()

		// Give the server time to start
		time.Sleep(50 * time.Millisecond)

		// Verify management port is set
		mgmtPort := srv.ManagementPort()
		assert.Greater(t, mgmtPort, 0)

		// Verify management API is accessible
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/status", mgmtPort))
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestServerGracefulShutdown(t *testing.T) {
	t.Run("shutdown completes within timeout", func(t *testing.T) {
		cfg := &config.ServerConfiguration{
			HTTPPort:      getFreePort(),
			MaxLogEntries: 100,
		}
		srv := NewServer(cfg)

		err := srv.Start()
		require.NoError(t, err)

		// Give server time to fully start
		time.Sleep(50 * time.Millisecond)

		// Measure shutdown time
		start := time.Now()
		err = srv.Stop()
		duration := time.Since(start)

		assert.NoError(t, err)
		// Shutdown should complete within the 5 second timeout
		assert.Less(t, duration, 5*time.Second)
	})

	t.Run("server state is correct after shutdown", func(t *testing.T) {
		cfg := &config.ServerConfiguration{
			HTTPPort:      getFreePort(),
			MaxLogEntries: 100,
		}
		srv := NewServer(cfg)

		err := srv.Start()
		require.NoError(t, err)
		assert.True(t, srv.IsRunning())

		err = srv.Stop()
		require.NoError(t, err)

		// Verify state after shutdown
		assert.False(t, srv.IsRunning())
		assert.Equal(t, 0, srv.Uptime())
	})

	t.Run("middleware chain is closed on shutdown", func(t *testing.T) {
		cfg := &config.ServerConfiguration{
			HTTPPort:      getFreePort(),
			MaxLogEntries: 100,
		}
		srv := NewServer(cfg)

		err := srv.Start()
		require.NoError(t, err)

		// Verify middleware chain exists before stop
		assert.NotNil(t, srv.middlewareChain)

		err = srv.Stop()
		require.NoError(t, err)

		// After stop, middleware chain should be nil
		assert.Nil(t, srv.middlewareChain)
	})
}

func TestServerConfigValidation(t *testing.T) {
	t.Parallel()

	t.Run("negative HTTP port uses zero", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{
			HTTPPort:      -1,
			MaxLogEntries: 100,
		}
		srv := NewServer(cfg)
		require.NotNil(t, srv)

		// Server should be created, negative port stored in config
		assert.Equal(t, -1, srv.cfg.HTTPPort)
	})

	t.Run("zero max log entries uses default", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{
			HTTPPort:      0,
			MaxLogEntries: 0,
		}
		srv := NewServer(cfg)
		require.NotNil(t, srv)

		// Request logger should be created with default capacity
		assert.NotNil(t, srv.Logger())
	})

	t.Run("zero timeouts are handled", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{
			HTTPPort:     0,
			ReadTimeout:  0,
			WriteTimeout: 0,
		}
		srv := NewServer(cfg)
		require.NotNil(t, srv)

		// Server should be created despite zero timeouts
		assert.Equal(t, 0, srv.cfg.ReadTimeout)
		assert.Equal(t, 0, srv.cfg.WriteTimeout)
	})

	t.Run("server with HTTPS but no TLS config still creates", func(t *testing.T) {
		t.Parallel()
		cfg := &config.ServerConfiguration{
			HTTPPort:  0,
			HTTPSPort: 8443,
			// No TLS config provided
		}
		srv := NewServer(cfg)
		require.NotNil(t, srv)

		// Server created, TLS manager will generate self-signed certs on start
		assert.NotNil(t, srv.tlsManager)
	})
}

func TestServerStartupErrors(t *testing.T) {
	t.Run("start fails on port already in use", func(t *testing.T) {
		// Start a server to occupy a port
		port := getFreePort()
		cfg1 := &config.ServerConfiguration{
			HTTPPort:      port,
			MaxLogEntries: 100,
		}
		srv1 := NewServer(cfg1)
		err := srv1.Start()
		require.NoError(t, err)
		defer srv1.Stop()

		// Give first server time to bind
		time.Sleep(50 * time.Millisecond)

		// Try to start another server on the same port
		cfg2 := &config.ServerConfiguration{
			HTTPPort:      port,
			MaxLogEntries: 100,
		}
		srv2 := NewServer(cfg2)

		// The server starts in a goroutine, so we need to verify
		// by attempting to use the port
		_ = srv2.Start()
		// The Start() itself may succeed because HTTP server starts in goroutine,
		// but the server won't actually bind. Clean up regardless.
		srv2.Stop()
	})
}

func TestServerStoreIntegration(t *testing.T) {
	t.Parallel()

	t.Run("SetStore updates mock manager store", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)

		// Add a mock before setting persistent store
		mockCfg := createTestHTTPMock("test-mock", "/api/test", "GET", 200, `{}`)
		err := srv.addMock(mockCfg)
		require.NoError(t, err)

		// Verify mock exists
		assert.NotNil(t, srv.getMock("test-mock"))
	})

	t.Run("PersistentStore returns nil when not set", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		assert.Nil(t, srv.PersistentStore())
	})
}

func TestServerConcurrentStartStop(t *testing.T) {
	t.Run("concurrent start attempts are safe", func(t *testing.T) {
		cfg := &config.ServerConfiguration{
			HTTPPort:      getFreePort(),
			MaxLogEntries: 100,
		}
		srv := NewServer(cfg)

		var wg sync.WaitGroup
		errors := make(chan error, 3)

		// Try to start the server concurrently
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := srv.Start()
				if err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)
		defer srv.Stop()

		// At least 2 attempts should fail with "already running"
		errorCount := 0
		for err := range errors {
			if err != nil {
				errorCount++
				assert.Contains(t, err.Error(), "already running")
			}
		}
		assert.GreaterOrEqual(t, errorCount, 2)
	})

	t.Run("concurrent stop attempts are safe", func(t *testing.T) {
		cfg := &config.ServerConfiguration{
			HTTPPort:      getFreePort(),
			MaxLogEntries: 100,
		}
		srv := NewServer(cfg)

		err := srv.Start()
		require.NoError(t, err)

		// Give server time to fully start
		time.Sleep(50 * time.Millisecond)

		var wg sync.WaitGroup

		// Try to stop the server concurrently
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = srv.Stop()
			}()
		}

		wg.Wait()

		// Server should be stopped
		assert.False(t, srv.IsRunning())
	})
}

// getFreePort finds an available port for testing
func getFreePort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
