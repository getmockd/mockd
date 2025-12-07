package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
)

// T094: Save config to file
func TestPersistenceSaveConfigToFile(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add some mocks
	srv.AddMock(&config.MockConfiguration{
		ID:       "persist-mock-1",
		Priority: 10,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "users list",
		},
	})

	srv.AddMock(&config.MockConfiguration{
		ID:       "persist-mock-2",
		Priority: 5,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "POST",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 201,
			Body:       "user created",
		},
	})

	// Save to file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mocks.json")

	err := srv.SaveConfig(configPath, "test-config")
	require.NoError(t, err)

	// Verify file exists and contains expected data
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var collection config.MockCollection
	err = json.Unmarshal(data, &collection)
	require.NoError(t, err)

	assert.Equal(t, "1.0", collection.Version)
	assert.Equal(t, "test-config", collection.Name)
	assert.Len(t, collection.Mocks, 2)
}

// T095: Restart server with config file
func TestPersistenceRestartWithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create initial server and add mocks
	httpPort1 := getFreePort()
	adminPort1 := getFreePort()

	cfg1 := &config.ServerConfiguration{
		HTTPPort:     httpPort1,
		AdminPort:    adminPort1,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv1 := engine.NewServer(cfg1)
	srv1.AddMock(&config.MockConfiguration{
		ID:       "restart-mock",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/restart-test",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "restart test response",
		},
	})

	err := srv1.Start()
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)

	// Verify it works
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/restart-test", httpPort1))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "restart test response", string(body))

	// Save config
	err = srv1.SaveConfig(configPath, "restart-test")
	require.NoError(t, err)

	// Stop first server
	srv1.Stop()

	// Create new server and load config
	httpPort2 := getFreePort()
	adminPort2 := getFreePort()

	cfg2 := &config.ServerConfiguration{
		HTTPPort:     httpPort2,
		AdminPort:    adminPort2,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv2 := engine.NewServer(cfg2)
	err = srv2.LoadConfig(configPath, true)
	require.NoError(t, err)

	err = srv2.Start()
	require.NoError(t, err)
	defer srv2.Stop()
	time.Sleep(50 * time.Millisecond)

	// Verify mock was restored
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/restart-test", httpPort2))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "restart test response", string(body))
}

// T096: Import via admin API
func TestPersistenceImportViaAdminAPI(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)
	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	adminAPI := admin.NewAdminAPI(srv, adminPort)
	err = adminAPI.Start()
	require.NoError(t, err)
	defer adminAPI.Stop()

	time.Sleep(50 * time.Millisecond)

	// Prepare import request
	importReq := map[string]interface{}{
		"replace": true,
		"config": map[string]interface{}{
			"version": "1.0",
			"name":    "imported-config",
			"mocks": []map[string]interface{}{
				{
					"id":       "imported-mock-1",
					"priority": 10,
					"enabled":  true,
					"matcher": map[string]string{
						"method": "GET",
						"path":   "/api/imported",
					},
					"response": map[string]interface{}{
						"statusCode": 200,
						"body":       "imported response",
					},
				},
				{
					"id":       "imported-mock-2",
					"priority": 5,
					"enabled":  true,
					"matcher": map[string]string{
						"path": "/api/imported-2",
					},
					"response": map[string]interface{}{
						"statusCode": 201,
						"body":       "imported 2",
					},
				},
			},
		},
	}

	reqBody, _ := json.Marshal(importReq)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/config", adminPort),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "imported")

	// Verify mocks work
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/imported", httpPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "imported response", string(body))
}

// Test export via admin API
func TestPersistenceExportViaAdminAPI(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add a mock
	srv.AddMock(&config.MockConfiguration{
		ID:       "export-mock",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/export-test",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "export test",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	adminAPI := admin.NewAdminAPI(srv, adminPort)
	err = adminAPI.Start()
	require.NoError(t, err)
	defer adminAPI.Stop()

	time.Sleep(50 * time.Millisecond)

	// Export config
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/config?name=my-export", adminPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var collection config.MockCollection
	err = json.Unmarshal(body, &collection)
	require.NoError(t, err)

	assert.Equal(t, "1.0", collection.Version)
	assert.Equal(t, "my-export", collection.Name)
	assert.Len(t, collection.Mocks, 1)
	assert.Equal(t, "export-mock", collection.Mocks[0].ID)
}

// Test import without replace (merge)
func TestPersistenceImportMerge(t *testing.T) {
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Add existing mock
	srv.AddMock(&config.MockConfiguration{
		ID:       "existing-mock",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Path: "/existing",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "existing",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	adminAPI := admin.NewAdminAPI(srv, adminPort)
	err = adminAPI.Start()
	require.NoError(t, err)
	defer adminAPI.Stop()

	time.Sleep(50 * time.Millisecond)

	// Import without replace
	importReq := map[string]interface{}{
		"replace": false,
		"config": map[string]interface{}{
			"version": "1.0",
			"name":    "merge-config",
			"mocks": []map[string]interface{}{
				{
					"id":      "new-mock",
					"enabled": true,
					"matcher": map[string]string{
						"path": "/new",
					},
					"response": map[string]interface{}{
						"statusCode": 200,
						"body":       "new",
					},
				},
			},
		},
	}

	reqBody, _ := json.Marshal(importReq)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/config", adminPort),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify both mocks exist
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/existing", httpPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "existing", string(body))

	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/new", httpPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "new", string(body))
}

// Test loading from file at server creation
func TestNewServerWithMocksFromFile(t *testing.T) {
	// Create a config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "preload.json")

	collection := &config.MockCollection{
		Version: "1.0",
		Name:    "preload-test",
		Mocks: []*config.MockConfiguration{
			{
				ID:       "preload-mock",
				Priority: 0,
				Enabled:  true,
				Matcher: &config.RequestMatcher{
					Method: "GET",
					Path:   "/api/preloaded",
				},
				Response: &config.ResponseDefinition{
					StatusCode: 200,
					Body:       "preloaded response",
				},
			},
		},
	}

	err := config.SaveToFile(configPath, collection)
	require.NoError(t, err)

	// Load mocks from file
	mocks, err := config.LoadMocksFromFile(configPath)
	require.NoError(t, err)

	// Create server with preloaded mocks
	httpPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServerWithMocks(cfg, mocks)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Verify mock works
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/preloaded", httpPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "preloaded response", string(body))
}
