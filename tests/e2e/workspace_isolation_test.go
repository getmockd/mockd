package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkspaceIsolation verifies that mocks, stateful resources, request logs,
// custom operations, and config import/export are properly scoped by workspace.
// This prevents data from workspace A bleeding into workspace B.
func TestWorkspaceIsolation(t *testing.T) {
	port := getFreePort(t)
	controlPort := getFreePort(t)
	adminPort := getFreePort(t)

	cfg := &config.ServerConfiguration{
		HTTPPort:       port,
		ManagementPort: controlPort,
	}

	server := engine.NewServer(cfg)
	go func() { _ = server.Start() }()
	defer server.Stop()

	adminURL := "http://localhost:" + strconv.Itoa(adminPort)
	engineURL := "http://localhost:" + strconv.Itoa(controlPort)
	mockTargetURL := "http://localhost:" + strconv.Itoa(port)

	engClient := engineclient.New(engineURL)

	adminAPI := admin.NewAPI(adminPort,
		admin.WithLocalEngine(engineURL),
		admin.WithAPIKeyDisabled(),
		admin.WithDataDir(t.TempDir()),
	)
	adminAPI.SetLocalEngine(engClient)

	go func() { _ = adminAPI.Start() }()
	defer adminAPI.Stop()

	waitForServer(t, adminURL+"/health")
	waitForServer(t, engineURL+"/health")

	client := &http.Client{}

	apiReq := func(method, path string, body []byte) *http.Response {
		urlStr := adminURL + path
		req, err := http.NewRequest(method, urlStr, bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)

		if resp.StatusCode >= 400 {
			b, _ := io.ReadAll(resp.Body)
			t.Logf("API Error %s %s -> %d : %s", method, urlStr, resp.StatusCode, string(b))
			resp.Body = io.NopCloser(bytes.NewBuffer(b))
		}

		return resp
	}

	engineReq := func(method, path string, body []byte) *http.Response {
		req, err := http.NewRequest(method, mockTargetURL+path, bytes.NewBuffer(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		return resp
	}

	// readJSON is a helper to decode JSON response bodies.
	readJSON := func(resp *http.Response, v interface{}) {
		t.Helper()
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(b, v), "body: %s", string(b))
	}

	// Clean slate before tests.
	resp := apiReq("DELETE", "/mocks", nil)
	resp.Body.Close()

	// Create two workspaces.
	var wsA, wsB struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	resp = apiReq("POST", "/workspaces", []byte(`{"name": "workspace-alpha"}`))
	require.Equal(t, 201, resp.StatusCode)
	readJSON(resp, &wsA)
	require.NotEmpty(t, wsA.ID)

	resp = apiReq("POST", "/workspaces", []byte(`{"name": "workspace-beta"}`))
	require.Equal(t, 201, resp.StatusCode)
	readJSON(resp, &wsB)
	require.NotEmpty(t, wsB.ID)

	// Workspace cleanup happens before server shutdown (defers run LIFO).
	defer func() {
		apiReq("DELETE", "/workspaces/"+wsA.ID, nil).Body.Close()
		apiReq("DELETE", "/workspaces/"+wsB.ID, nil).Body.Close()
	}()

	// ─── Mock Isolation ─────────────────────────────────────────
	t.Run("Mock Isolation", func(t *testing.T) {
		// Create a mock in workspace A.
		resp := apiReq("POST", "/mocks?workspaceId="+wsA.ID, []byte(`{
			"type": "http",
			"name": "Alpha Greeting",
			"http": {
				"matcher": {"method": "GET", "path": "/api/greet"},
				"response": {"statusCode": 200, "body": "{\"from\":\"alpha\"}"}
			}
		}`))
		require.Equal(t, 201, resp.StatusCode)
		var mockA struct {
			ID string `json:"id"`
		}
		readJSON(resp, &mockA)

		// Create a mock in workspace B with the same path but different body.
		resp = apiReq("POST", "/mocks?workspaceId="+wsB.ID, []byte(`{
			"type": "http",
			"name": "Beta Greeting",
			"http": {
				"matcher": {"method": "GET", "path": "/api/beta-greet"},
				"response": {"statusCode": 200, "body": "{\"from\":\"beta\"}"}
			}
		}`))
		require.Equal(t, 201, resp.StatusCode)
		var mockB struct {
			ID string `json:"id"`
		}
		readJSON(resp, &mockB)

		// List mocks filtered by workspace A — should see only A's mock.
		var listA struct {
			Mocks []struct {
				ID          string `json:"id"`
				WorkspaceID string `json:"workspaceId"`
				Name        string `json:"name"`
			} `json:"mocks"`
			Total int `json:"total"`
		}
		resp = apiReq("GET", "/mocks?workspaceId="+wsA.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &listA)

		assert.Equal(t, 1, listA.Total, "workspace A should have exactly 1 mock")
		if assert.Len(t, listA.Mocks, 1) {
			assert.Equal(t, mockA.ID, listA.Mocks[0].ID)
			assert.Equal(t, wsA.ID, listA.Mocks[0].WorkspaceID)
		}

		// List mocks filtered by workspace B — should see only B's mock.
		var listB struct {
			Mocks []struct {
				ID          string `json:"id"`
				WorkspaceID string `json:"workspaceId"`
				Name        string `json:"name"`
			} `json:"mocks"`
			Total int `json:"total"`
		}
		resp = apiReq("GET", "/mocks?workspaceId="+wsB.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &listB)

		assert.Equal(t, 1, listB.Total, "workspace B should have exactly 1 mock")
		if assert.Len(t, listB.Mocks, 1) {
			assert.Equal(t, mockB.ID, listB.Mocks[0].ID)
			assert.Equal(t, wsB.ID, listB.Mocks[0].WorkspaceID)
		}

		// Clean up.
		apiReq("DELETE", "/mocks/"+mockA.ID, nil).Body.Close()
		apiReq("DELETE", "/mocks/"+mockB.ID, nil).Body.Close()
	})

	// ─── Stateful Resource Isolation ────────────────────────────
	t.Run("Stateful Resource Isolation", func(t *testing.T) {
		// Register a "products" resource in workspace A.
		resp := apiReq("POST", "/state/resources?workspaceId="+wsA.ID, []byte(`{
			"name": "products",
			"idField": "id"
		}`))
		require.Equal(t, 201, resp.StatusCode)
		resp.Body.Close()

		// Register a "products" resource in workspace B (same name — should be independent).
		resp = apiReq("POST", "/state/resources?workspaceId="+wsB.ID, []byte(`{
			"name": "products",
			"idField": "id"
		}`))
		require.Equal(t, 201, resp.StatusCode)
		resp.Body.Close()

		// Create an item in workspace A's products.
		resp = apiReq("POST", "/state/resources/products/items?workspaceId="+wsA.ID,
			[]byte(`{"id": "prod-a1", "name": "Widget Alpha", "price": 9.99}`))
		require.Equal(t, 201, resp.StatusCode)
		resp.Body.Close()

		// Create an item in workspace B's products.
		resp = apiReq("POST", "/state/resources/products/items?workspaceId="+wsB.ID,
			[]byte(`{"id": "prod-b1", "name": "Widget Beta", "price": 19.99}`))
		require.Equal(t, 201, resp.StatusCode)
		resp.Body.Close()

		// List items in workspace A — should only see Alpha's item.
		var itemsA struct {
			Data []map[string]interface{} `json:"data"`
			Meta struct {
				Total int `json:"total"`
			} `json:"meta"`
		}
		resp = apiReq("GET", "/state/resources/products/items?workspaceId="+wsA.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &itemsA)

		assert.Equal(t, 1, itemsA.Meta.Total, "workspace A should have 1 product")
		if assert.Len(t, itemsA.Data, 1) {
			assert.Equal(t, "prod-a1", itemsA.Data[0]["id"])
			assert.Equal(t, "Widget Alpha", itemsA.Data[0]["name"])
		}

		// List items in workspace B — should only see Beta's item.
		var itemsB struct {
			Data []map[string]interface{} `json:"data"`
			Meta struct {
				Total int `json:"total"`
			} `json:"meta"`
		}
		resp = apiReq("GET", "/state/resources/products/items?workspaceId="+wsB.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &itemsB)

		assert.Equal(t, 1, itemsB.Meta.Total, "workspace B should have 1 product")
		if assert.Len(t, itemsB.Data, 1) {
			assert.Equal(t, "prod-b1", itemsB.Data[0]["id"])
			assert.Equal(t, "Widget Beta", itemsB.Data[0]["name"])
		}

		// Get specific item from workspace A — should not leak B's data.
		resp = apiReq("GET", "/state/resources/products/items/prod-b1?workspaceId="+wsA.ID, nil)
		assert.Equal(t, 404, resp.StatusCode, "workspace A should not see B's item")
		resp.Body.Close()

		// Reset workspace A — workspace B should be unaffected.
		resp = apiReq("POST", "/state/resources/products/reset?workspaceId="+wsA.ID, nil)
		require.True(t, resp.StatusCode == 200 || resp.StatusCode == 204,
			"expected 200 or 204, got %d", resp.StatusCode)
		resp.Body.Close()

		// Verify workspace B still has its item.
		var itemsBAfterReset struct {
			Data []map[string]interface{} `json:"data"`
			Meta struct {
				Total int `json:"total"`
			} `json:"meta"`
		}
		resp = apiReq("GET", "/state/resources/products/items?workspaceId="+wsB.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &itemsBAfterReset)
		assert.Equal(t, 1, itemsBAfterReset.Meta.Total, "workspace B should still have 1 product after A's reset")
	})

	// ─── Request Log Isolation ──────────────────────────────────
	t.Run("Request Log Isolation", func(t *testing.T) {
		// Clear existing logs.
		resp := apiReq("DELETE", "/requests", nil)
		resp.Body.Close()

		// Create mocks in each workspace that we can hit to generate logs.
		// Note: Non-default workspaces get basePath prefixes on the engine.
		// workspace-alpha → basePath /workspace-alpha
		// workspace-beta  → basePath /workspace-beta
		// So the engine-visible paths are: /workspace-alpha/api/log-alpha, etc.
		resp = apiReq("POST", "/mocks?workspaceId="+wsA.ID, []byte(`{
			"type": "http",
			"name": "Log Alpha",
			"http": {
				"matcher": {"method": "GET", "path": "/api/log-alpha"},
				"response": {"statusCode": 200, "body": "{\"ws\":\"alpha\"}"}
			}
		}`))
		require.Equal(t, 201, resp.StatusCode)
		var logMockA struct {
			ID string `json:"id"`
		}
		readJSON(resp, &logMockA)

		resp = apiReq("POST", "/mocks?workspaceId="+wsB.ID, []byte(`{
			"type": "http",
			"name": "Log Beta",
			"http": {
				"matcher": {"method": "GET", "path": "/api/log-beta"},
				"response": {"statusCode": 200, "body": "{\"ws\":\"beta\"}"}
			}
		}`))
		require.Equal(t, 201, resp.StatusCode)
		var logMockB struct {
			ID string `json:"id"`
		}
		readJSON(resp, &logMockB)

		// Hit both mocks on the engine using their basePath-prefixed paths.
		resp = engineReq("GET", "/workspace-alpha/api/log-alpha", nil)
		require.Equal(t, 200, resp.StatusCode, "workspace-alpha mock should be reachable at basePath-prefixed path")
		resp.Body.Close()

		resp = engineReq("GET", "/workspace-beta/api/log-beta", nil)
		require.Equal(t, 200, resp.StatusCode, "workspace-beta mock should be reachable at basePath-prefixed path")
		resp.Body.Close()

		// List logs filtered by workspace A — should only see alpha's request.
		// Both "count" and "total" should be 1 (total is now filtered too).
		var logsA struct {
			Requests []struct {
				Path        string `json:"path"`
				WorkspaceID string `json:"workspaceId"`
			} `json:"requests"`
			Count int `json:"count"`
			Total int `json:"total"`
		}
		resp = apiReq("GET", "/requests?workspaceId="+wsA.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &logsA)

		assert.Equal(t, 1, logsA.Count, "workspace A count should be 1")
		assert.Equal(t, 1, logsA.Total, "workspace A total should be 1 (filtered)")
		if assert.Len(t, logsA.Requests, 1) {
			assert.Contains(t, logsA.Requests[0].Path, "log-alpha")
			assert.Equal(t, wsA.ID, logsA.Requests[0].WorkspaceID)
		}

		// List logs filtered by workspace B — should only see beta's request.
		var logsB struct {
			Requests []struct {
				Path        string `json:"path"`
				WorkspaceID string `json:"workspaceId"`
			} `json:"requests"`
			Count int `json:"count"`
			Total int `json:"total"`
		}
		resp = apiReq("GET", "/requests?workspaceId="+wsB.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &logsB)

		assert.Equal(t, 1, logsB.Count, "workspace B count should be 1")
		assert.Equal(t, 1, logsB.Total, "workspace B total should be 1 (filtered)")
		if assert.Len(t, logsB.Requests, 1) {
			assert.Contains(t, logsB.Requests[0].Path, "log-beta")
			assert.Equal(t, wsB.ID, logsB.Requests[0].WorkspaceID)
		}

		// Clean up mocks.
		apiReq("DELETE", "/mocks/"+logMockA.ID, nil).Body.Close()
		apiReq("DELETE", "/mocks/"+logMockB.ID, nil).Body.Close()
	})

	// ─── Config Import/Export Isolation ─────────────────────────
	t.Run("Config Import Export Isolation", func(t *testing.T) {
		// Clean all mocks first.
		resp := apiReq("DELETE", "/mocks", nil)
		resp.Body.Close()

		// Import a config into workspace A.
		configA := `{
			"mocks": [{
				"type": "http",
				"name": "Imported Alpha",
				"http": {
					"matcher": {"method": "GET", "path": "/api/imported-alpha"},
					"response": {"statusCode": 200, "body": "{\"imported\":\"alpha\"}"}
				}
			}]
		}`
		resp = apiReq("POST", "/config?workspaceId="+wsA.ID, []byte(configA))
		require.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"import into workspace A failed: %d", resp.StatusCode)
		resp.Body.Close()

		// Import a different config into workspace B.
		configB := `{
			"mocks": [{
				"type": "http",
				"name": "Imported Beta",
				"http": {
					"matcher": {"method": "GET", "path": "/api/imported-beta"},
					"response": {"statusCode": 200, "body": "{\"imported\":\"beta\"}"}
				}
			}]
		}`
		resp = apiReq("POST", "/config?workspaceId="+wsB.ID, []byte(configB))
		require.True(t, resp.StatusCode >= 200 && resp.StatusCode < 300,
			"import into workspace B failed: %d", resp.StatusCode)
		resp.Body.Close()

		// Export workspace A — should only contain the alpha mock.
		resp = apiReq("GET", "/config?workspaceId="+wsA.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		var exportA struct {
			Mocks []struct {
				Name        string `json:"name"`
				WorkspaceID string `json:"workspaceId"`
			} `json:"mocks"`
		}
		readJSON(resp, &exportA)

		foundAlpha := false
		foundBeta := false
		for _, m := range exportA.Mocks {
			if m.Name == "Imported Alpha" {
				foundAlpha = true
			}
			if m.Name == "Imported Beta" {
				foundBeta = true
			}
		}
		assert.True(t, foundAlpha, "workspace A export should include 'Imported Alpha'")
		assert.False(t, foundBeta, "workspace A export should NOT include 'Imported Beta'")

		// Export workspace B — should only contain the beta mock.
		resp = apiReq("GET", "/config?workspaceId="+wsB.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		var exportB struct {
			Mocks []struct {
				Name        string `json:"name"`
				WorkspaceID string `json:"workspaceId"`
			} `json:"mocks"`
		}
		readJSON(resp, &exportB)

		foundAlpha = false
		foundBeta = false
		for _, m := range exportB.Mocks {
			if m.Name == "Imported Alpha" {
				foundAlpha = true
			}
			if m.Name == "Imported Beta" {
				foundBeta = true
			}
		}
		assert.False(t, foundAlpha, "workspace B export should NOT include 'Imported Alpha'")
		assert.True(t, foundBeta, "workspace B export should include 'Imported Beta'")
	})

	// ─── Bulk Create Isolation ──────────────────────────────────
	t.Run("Bulk Create Isolation", func(t *testing.T) {
		// Clean all mocks.
		resp := apiReq("DELETE", "/mocks", nil)
		resp.Body.Close()

		// Bulk create into workspace A.
		resp = apiReq("POST", "/mocks/bulk?workspaceId="+wsA.ID, []byte(`[
			{
				"type": "http",
				"name": "Bulk Alpha 1",
				"http": {
					"matcher": {"method": "GET", "path": "/api/bulk-a1"},
					"response": {"statusCode": 200}
				}
			},
			{
				"type": "http",
				"name": "Bulk Alpha 2",
				"http": {
					"matcher": {"method": "GET", "path": "/api/bulk-a2"},
					"response": {"statusCode": 200}
				}
			}
		]`))
		require.True(t, resp.StatusCode == 200 || resp.StatusCode == 201,
			"bulk create A: expected 200|201, got %d", resp.StatusCode)
		resp.Body.Close()

		// Bulk create into workspace B.
		resp = apiReq("POST", "/mocks/bulk?workspaceId="+wsB.ID, []byte(`[
			{
				"type": "http",
				"name": "Bulk Beta 1",
				"http": {
					"matcher": {"method": "GET", "path": "/api/bulk-b1"},
					"response": {"statusCode": 200}
				}
			}
		]`))
		require.True(t, resp.StatusCode == 200 || resp.StatusCode == 201,
			"bulk create B: expected 200|201, got %d", resp.StatusCode)
		resp.Body.Close()

		// Verify workspace A has 2 mocks.
		var listA struct {
			Total int `json:"total"`
		}
		resp = apiReq("GET", "/mocks?workspaceId="+wsA.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &listA)
		assert.Equal(t, 2, listA.Total, "workspace A should have 2 bulk-created mocks")

		// Verify workspace B has 1 mock.
		var listB struct {
			Total int `json:"total"`
		}
		resp = apiReq("GET", "/mocks?workspaceId="+wsB.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &listB)
		assert.Equal(t, 1, listB.Total, "workspace B should have 1 bulk-created mock")
	})

	// ─── Workspace Delete Cascades Mocks ────────────────────────
	t.Run("Workspace Delete Cascades Mocks", func(t *testing.T) {
		// Clean slate.
		resp := apiReq("DELETE", "/mocks", nil)
		resp.Body.Close()

		// Create a temporary workspace.
		var wsTemp struct {
			ID string `json:"id"`
		}
		resp = apiReq("POST", "/workspaces", []byte(`{"name": "temp-cascade"}`))
		require.Equal(t, 201, resp.StatusCode)
		readJSON(resp, &wsTemp)

		// Create a mock in the temp workspace.
		resp = apiReq("POST", "/mocks?workspaceId="+wsTemp.ID, []byte(`{
			"type": "http",
			"name": "Temp Mock",
			"http": {
				"matcher": {"method": "GET", "path": "/api/temp"},
				"response": {"statusCode": 200, "body": "{\"temp\":true}"}
			}
		}`))
		require.Equal(t, 201, resp.StatusCode)
		var tempMock struct {
			ID string `json:"id"`
		}
		readJSON(resp, &tempMock)

		// Verify the mock exists.
		resp = apiReq("GET", "/mocks/"+tempMock.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		resp.Body.Close()

		// Delete the workspace — should cascade-delete its mocks.
		resp = apiReq("DELETE", "/workspaces/"+wsTemp.ID, nil)
		require.Equal(t, 204, resp.StatusCode)
		resp.Body.Close()

		// Verify the mock is gone.
		resp = apiReq("GET", "/mocks/"+tempMock.ID, nil)
		assert.Equal(t, 404, resp.StatusCode, "mock should be deleted after workspace deletion")
		resp.Body.Close()
	})

	// ─── State Overview Isolation ───────────────────────────────
	t.Run("State Overview Isolation", func(t *testing.T) {
		// Register a resource in workspace A.
		resp := apiReq("POST", "/state/resources?workspaceId="+wsA.ID, []byte(`{
			"name": "orders",
			"idField": "id"
		}`))
		require.Equal(t, 201, resp.StatusCode)
		resp.Body.Close()

		// State overview for workspace A should include "orders".
		var overviewA struct {
			ResourceList []string `json:"resourceList"`
			Total        int      `json:"total"`
		}
		resp = apiReq("GET", "/state?workspaceId="+wsA.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &overviewA)
		assert.Contains(t, overviewA.ResourceList, "orders",
			"workspace A state overview should include 'orders'")

		// State overview for workspace B should NOT include "orders" (unless it was
		// independently registered there).
		var overviewB struct {
			ResourceList []string `json:"resourceList"`
			Total        int      `json:"total"`
		}
		resp = apiReq("GET", "/state?workspaceId="+wsB.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &overviewB)

		// "orders" should only appear if workspace B has its own copy from earlier tests.
		// The "products" resource may be there from the earlier subtest.
		// The key assertion: "orders" should NOT be in B's overview since we only
		// registered it in A.
		ordersInB := false
		for _, r := range overviewB.ResourceList {
			if r == "orders" {
				ordersInB = true
			}
		}
		assert.False(t, ordersInB,
			"workspace B state overview should NOT include 'orders' (registered only in A)")
	})

	// ─── Custom Operations Isolation ────────────────────────────
	t.Run("Custom Operations Isolation", func(t *testing.T) {
		// Register a custom operation in workspace A.
		resp := apiReq("POST", "/state/operations?workspaceId="+wsA.ID, []byte(`{
			"name": "confirm-order",
			"steps": [
				{
					"type": "update",
					"resource": "orders",
					"field": "status",
					"value": "confirmed"
				}
			]
		}`))
		require.True(t, resp.StatusCode == 200 || resp.StatusCode == 201,
			"register custom op A: expected 200|201, got %d", resp.StatusCode)
		resp.Body.Close()

		// List custom operations in workspace A — should see "confirm-order".
		var opsA struct {
			Operations []struct {
				Name string `json:"name"`
			} `json:"operations"`
		}
		resp = apiReq("GET", "/state/operations?workspaceId="+wsA.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &opsA)

		foundOp := false
		for _, op := range opsA.Operations {
			if op.Name == "confirm-order" {
				foundOp = true
			}
		}
		assert.True(t, foundOp, "workspace A should have 'confirm-order' operation")

		// List custom operations in workspace B — should NOT see "confirm-order".
		var opsB struct {
			Operations []struct {
				Name string `json:"name"`
			} `json:"operations"`
		}
		resp = apiReq("GET", "/state/operations?workspaceId="+wsB.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &opsB)

		foundOpInB := false
		for _, op := range opsB.Operations {
			if op.Name == "confirm-order" {
				foundOpInB = true
			}
		}
		assert.False(t, foundOpInB,
			"workspace B should NOT have 'confirm-order' (registered only in A)")

		// Clean up.
		apiReq("DELETE", "/state/operations/confirm-order?workspaceId="+wsA.ID, nil).Body.Close()
	})

	// ─── Default Workspace Mocks Stay Isolated ──────────────────
	t.Run("Default Workspace Isolation", func(t *testing.T) {
		// Clean all mocks.
		resp := apiReq("DELETE", "/mocks", nil)
		resp.Body.Close()

		// Create a mock without specifying workspace (defaults to "local").
		resp = apiReq("POST", "/mocks", []byte(`{
			"type": "http",
			"name": "Default Mock",
			"http": {
				"matcher": {"method": "GET", "path": "/api/default"},
				"response": {"statusCode": 200, "body": "{\"ws\":\"default\"}"}
			}
		}`))
		require.Equal(t, 201, resp.StatusCode)
		// The response is {"id": "...", "mock": {<full mock with workspaceId>}, ...}
		var createResp struct {
			ID   string `json:"id"`
			Mock struct {
				WorkspaceID string `json:"workspaceId"`
			} `json:"mock"`
		}
		readJSON(resp, &createResp)
		assert.Equal(t, "local", createResp.Mock.WorkspaceID,
			"mock without workspace should default to 'local'")

		// List with workspace A filter — should NOT see the default mock.
		var listA struct {
			Total int `json:"total"`
		}
		resp = apiReq("GET", "/mocks?workspaceId="+wsA.ID, nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &listA)
		assert.Equal(t, 0, listA.Total, "workspace A should have 0 mocks (default mock is in 'local')")

		// List with "local" filter — should see it.
		var listDefault struct {
			Total int `json:"total"`
		}
		resp = apiReq("GET", "/mocks?workspaceId=local", nil)
		require.Equal(t, 200, resp.StatusCode)
		readJSON(resp, &listDefault)
		assert.Equal(t, 1, listDefault.Total, "default workspace should have 1 mock")
	})
}
