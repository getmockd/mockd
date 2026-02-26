package integration

import (
	"bytes"
	"context"
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
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
)

// persistenceTestBundle groups server and client for persistence tests
type persistenceTestBundle struct {
	Server         *engine.Server
	Client         *engineclient.Client
	AdminAPI       *admin.API
	HTTPPort       int
	AdminPort      int
	ManagementPort int
}

func setupPersistenceServer(t *testing.T) *persistenceTestBundle {
	httpPort := getFreePort()
	adminPort := getFreePort()
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		AdminPort:      adminPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServer(cfg)
	err := srv.Start()
	require.NoError(t, err)

	tempDir := t.TempDir() // Use temp dir for test isolation
	adminAPI := admin.NewAPI(adminPort,
		admin.WithLocalEngine(fmt.Sprintf("http://localhost:%d", srv.ManagementPort())),
		admin.WithAPIKeyDisabled(),
		admin.WithDataDir(tempDir),
	)
	err = adminAPI.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		adminAPI.Stop()
		srv.Stop()
		time.Sleep(10 * time.Millisecond) // Allow file handles to release
	})

	waitForReady(t, srv.ManagementPort())

	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	return &persistenceTestBundle{
		Server:         srv,
		Client:         client,
		AdminAPI:       adminAPI,
		HTTPPort:       httpPort,
		AdminPort:      adminPort,
		ManagementPort: managementPort,
	}
}

// T094: Save config to file
func TestPersistenceSaveConfigToFile(t *testing.T) {
	bundle := setupPersistenceServer(t)

	// Add some mocks
	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "persist-mock-1",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 10,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "users list",
			},
		},
	})
	require.NoError(t, err)

	_, err = bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "persist-mock-2",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 5,
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 201,
				Body:       "user created",
			},
		},
	})
	require.NoError(t, err)

	// Export config via client
	collection, err := bundle.Client.ExportConfig(context.Background(), "test-config")
	require.NoError(t, err)

	// Verify collection
	assert.Equal(t, "1.0", collection.Version)
	assert.Equal(t, "test-config", collection.Name)
	assert.Len(t, collection.Mocks, 2)

	// Save to file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mocks.json")

	data, err := json.MarshalIndent(collection, "", "  ")
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0644)
	require.NoError(t, err)

	// Verify file exists and contains expected data
	fileData, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var loadedCollection config.MockCollection
	err = json.Unmarshal(fileData, &loadedCollection)
	require.NoError(t, err)

	assert.Equal(t, "1.0", loadedCollection.Version)
	assert.Equal(t, "test-config", loadedCollection.Name)
	assert.Len(t, loadedCollection.Mocks, 2)
}

// T095: Restart server with config file
func TestPersistenceRestartWithConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create initial server and add mocks
	httpPort1 := getFreePort()
	managementPort1 := getFreePort()

	cfg1 := &config.ServerConfiguration{
		HTTPPort:       httpPort1,
		ManagementPort: managementPort1,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv1 := engine.NewServer(cfg1)
	err := srv1.Start()
	require.NoError(t, err)

	waitForReady(t, srv1.ManagementPort())

	client1 := engineclient.New(fmt.Sprintf("http://localhost:%d", srv1.ManagementPort()))

	_, err = client1.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "restart-mock",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/restart-test",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "restart test response",
			},
		},
	})
	require.NoError(t, err)

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
	managementPort2 := getFreePort()

	cfg2 := &config.ServerConfiguration{
		HTTPPort:       httpPort2,
		ManagementPort: managementPort2,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv2 := engine.NewServer(cfg2)
	err = srv2.LoadConfig(configPath, true)
	require.NoError(t, err)

	err = srv2.Start()
	require.NoError(t, err)
	defer srv2.Stop()
	waitForReady(t, srv2.ManagementPort())

	// Verify mock was restored
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/restart-test", httpPort2))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "restart test response", string(body))
}

// T096: Import via admin API
func TestPersistenceImportViaAdminAPI(t *testing.T) {
	bundle := setupPersistenceServer(t)

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
		fmt.Sprintf("http://localhost:%d/config", bundle.AdminPort),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(body), "imported")

	// Verify mocks work
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/api/imported", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "imported response", string(body))
}

// Test export via admin API
func TestPersistenceExportViaAdminAPI(t *testing.T) {
	bundle := setupPersistenceServer(t)

	// Add a mock through admin API (not engine directly) so it lands in
	// the admin store — the single source of truth for config export.
	mockJSON, _ := json.Marshal(&config.MockConfiguration{
		ID:      "export-mock",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/export-test",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "export test",
			},
		},
	})
	createResp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/mocks", bundle.AdminPort),
		"application/json",
		bytes.NewReader(mockJSON),
	)
	require.NoError(t, err)
	createResp.Body.Close()
	require.Equal(t, http.StatusCreated, createResp.StatusCode)

	// Export config
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/config?name=my-export", bundle.AdminPort))
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
	bundle := setupPersistenceServer(t)

	// Add existing mock
	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "existing-mock",
		Enabled: boolPtr(true),
		Type:    mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Path: "/existing",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "existing",
			},
		},
	})
	require.NoError(t, err)

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
		fmt.Sprintf("http://localhost:%d/config", bundle.AdminPort),
		"application/json",
		bytes.NewReader(reqBody),
	)
	require.NoError(t, err)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Verify both mocks exist
	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/existing", bundle.HTTPPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "existing", string(body))

	resp, err = http.Get(fmt.Sprintf("http://localhost:%d/new", bundle.HTTPPort))
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
				ID:      "preload-mock",
				Enabled: boolPtr(true),
				Type:    mock.TypeHTTP,
				HTTP: &mock.HTTPSpec{
					Priority: 0,
					Matcher: &mock.HTTPMatcher{
						Method: "GET",
						Path:   "/api/preloaded",
					},
					Response: &mock.HTTPResponse{
						StatusCode: 200,
						Body:       "preloaded response",
					},
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
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		ManagementPort: managementPort,
		ReadTimeout:    30,
		WriteTimeout:   30,
	}

	srv := engine.NewServerWithMocks(cfg, mocks)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	waitForReady(t, srv.ManagementPort())

	// Verify mock works
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/preloaded", httpPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "preloaded response", string(body))
}
