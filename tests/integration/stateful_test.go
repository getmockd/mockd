package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/stateful"
)

// statefulResourceConfig is used in tests to configure resources
type statefulResourceConfig struct {
	Name        string
	BasePath    string
	IDField     string
	ParentField string
	SeedData    []map[string]interface{}
}

// Test helpers

//nolint:unused // kept for future tests
func createTestStore() *stateful.StateStore {
	return stateful.NewStateStore()
}

//nolint:unused // kept for future tests
func createTestResource(store *stateful.StateStore, name, basePath string, seedData []map[string]interface{}) error {
	cfg := &stateful.ResourceConfig{
		Name:     name,
		BasePath: basePath,
		SeedData: seedData,
	}
	return store.Register(cfg)
}

//nolint:unused // kept for future tests
func makeJSONRequest(t *testing.T, handler http.Handler, method, path string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()

	var reqBody io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request body: %v", err)
		}
		reqBody = bytes.NewReader(jsonBytes)
	}

	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

//nolint:unused // kept for future tests
func parseJSONResponse(t *testing.T, rec *httptest.ResponseRecorder, v interface{}) {
	t.Helper()
	if err := json.NewDecoder(rec.Body).Decode(v); err != nil {
		t.Fatalf("failed to decode response: %v, body: %s", err, rec.Body.String())
	}
}

// testServerBundle groups server, admin API, and ports for easy cleanup
type testServerBundle struct {
	Server    *engine.Server
	AdminAPI  *admin.AdminAPI
	HTTPPort  int
	AdminPort int
}

// Stop stops both the server and admin API
func (b *testServerBundle) Stop() {
	if b.AdminAPI != nil {
		b.AdminAPI.Stop()
	}
	if b.Server != nil {
		b.Server.Stop()
	}
}

// createStatefulServer creates a test server with stateful resources
func createStatefulServer(t *testing.T, resources ...*statefulResourceConfig) (*engine.Server, int, int) {
	t.Helper()
	bundle := createStatefulServerWithAdmin(t, resources...)
	// Note: cleanup is already registered in createStatefulServerWithAdmin
	return bundle.Server, bundle.HTTPPort, bundle.AdminPort
}

// createStatefulServerWithAdmin creates a test server with admin API for tests that need it
// Note: The server is NOT started here - tests must call srv.Start() themselves.
// The admin API is created but its engine client connects lazily on first request.
func createStatefulServerWithAdmin(t *testing.T, resources ...*statefulResourceConfig) *testServerBundle {
	t.Helper()

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

	// Convert test resources to config resources
	var statefulResources []*config.StatefulResourceConfig
	for _, res := range resources {
		statefulResources = append(statefulResources, &config.StatefulResourceConfig{
			Name:        res.Name,
			BasePath:    res.BasePath,
			IDField:     res.IDField,
			ParentField: res.ParentField,
			SeedData:    res.SeedData,
		})
	}

	// Import resources via MockCollection
	if len(statefulResources) > 0 {
		collection := &config.MockCollection{
			Version:           "1.0",
			Name:              "stateful-test",
			StatefulResources: statefulResources,
		}
		err := srv.ImportConfig(collection, true)
		require.NoError(t, err, "failed to import stateful resources")
	}

	// Create and start admin API
	// The engine client connects lazily, so it's OK that the server isn't started yet
	tempDir := t.TempDir() // Use temp dir for test isolation
	adminAPI := admin.NewAdminAPI(adminPort,
		admin.WithLocalEngine(fmt.Sprintf("http://localhost:%d", srv.ManagementPort())),
		admin.WithAPIKeyDisabled(),
		admin.WithDataDir(tempDir),
	)
	err := adminAPI.Start()
	require.NoError(t, err, "failed to start admin API")

	t.Cleanup(func() {
		adminAPI.Stop()
		time.Sleep(10 * time.Millisecond) // Allow file handles to release
	})

	return &testServerBundle{
		Server:    srv,
		AdminAPI:  adminAPI,
		HTTPPort:  httpPort,
		AdminPort: adminPort,
	}
}

// =============================================================================
// User Story 1: Basic CRUD Resource Simulation
// =============================================================================

func TestStateful_US1_PostCreatesResourceReturns201(t *testing.T) {
	// T011: Integration test: POST creates resource and returns 201
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create a user
	body := bytes.NewBufferString(`{"name": "Alice", "email": "alice@example.com"}`)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/users", httpPort),
		"application/json",
		body,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Verify 201 Created
	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	// Verify response contains auto-generated ID
	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.NotEmpty(t, result["id"], "response should contain auto-generated ID")
	assert.Equal(t, "Alice", result["name"])
	assert.Equal(t, "alice@example.com", result["email"])
	assert.NotEmpty(t, result["createdAt"], "response should contain createdAt timestamp")
}

func TestStateful_US1_GetRetrievesSingleResourceByID(t *testing.T) {
	// T012: Integration test: GET retrieves single resource by ID
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "user-123", "name": "Bob", "role": "admin"},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// GET single user by ID
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users/user-123", httpPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "user-123", result["id"])
	assert.Equal(t, "Bob", result["name"])
	assert.Equal(t, "admin", result["role"])
}

func TestStateful_US1_GetCollectionReturnsAllResources(t *testing.T) {
	// T013: Integration test: GET collection returns all resources
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "user-1", "name": "Alice"},
			{"id": "user-2", "name": "Bob"},
			{"id": "user-3", "name": "Charlie"},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// GET collection
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users", httpPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result stateful.PaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Len(t, result.Data, 3, "should return all 3 users")
	assert.Equal(t, 3, result.Meta.Total)
	assert.Equal(t, 3, result.Meta.Count)
}

func TestStateful_US1_PutUpdatesExistingResource(t *testing.T) {
	// T014: Integration test: PUT updates existing resource
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "user-123", "name": "Bob", "role": "user"},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// PUT to update user
	body := bytes.NewBufferString(`{"name": "Robert", "role": "admin"}`)
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://localhost:%d/api/users/user-123", httpPort), body)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "user-123", result["id"])
	assert.Equal(t, "Robert", result["name"])
	assert.Equal(t, "admin", result["role"])
}

func TestStateful_US1_DeleteRemovesResourceReturns204(t *testing.T) {
	// T015: Integration test: DELETE removes resource and returns 204
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "user-123", "name": "ToDelete"},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// DELETE user
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://localhost:%d/api/users/user-123", httpPort), nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	// Verify user is gone
	resp2, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users/user-123", httpPort))
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp2.StatusCode)
}

func TestStateful_US1_GetReturns404ForNonExistent(t *testing.T) {
	// T016: Integration test: GET returns 404 for non-existent resource
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// GET non-existent user
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users/nonexistent-id", httpPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	var result stateful.ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "resource not found", result.Error)
}

func TestStateful_US1_PostReturns409ForDuplicateID(t *testing.T) {
	// T017: Integration test: POST returns 409 for duplicate ID
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "existing-id", "name": "Existing"},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// POST with duplicate ID
	body := bytes.NewBufferString(`{"id": "existing-id", "name": "Duplicate"}`)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/users", httpPort),
		"application/json",
		body,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

// =============================================================================
// User Story 2: State Initialization with Seed Data
// =============================================================================

func TestStateful_US2_SeedDataAvailableAfterStart(t *testing.T) {
	// T027: Integration test: Seed data available immediately after server start
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "products",
		BasePath: "/api/products",
		SeedData: []map[string]interface{}{
			{"id": "prod-1", "name": "Widget", "price": 9.99},
			{"id": "prod-2", "name": "Gadget", "price": 19.99},
			{"id": "prod-3", "name": "Doodad", "price": 4.99},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Verify all seed data is available
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/products", httpPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	var result stateful.PaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, 3, result.Meta.Total, "all seed data should be available")

	// Verify individual items exist
	resp2, err := http.Get(fmt.Sprintf("http://localhost:%d/api/products/prod-1", httpPort))
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var product map[string]interface{}
	err = json.NewDecoder(resp2.Body).Decode(&product)
	require.NoError(t, err)

	assert.Equal(t, "Widget", product["name"])
}

func TestStateful_US2_SeedDataPersistsUntilRestart(t *testing.T) {
	// T028: Integration test: Seed data persists until server restart
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "user-1", "name": "Original"},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Modify the seed data by adding a new user
	body := bytes.NewBufferString(`{"name": "NewUser"}`)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/users", httpPort),
		"application/json",
		body,
	)
	require.NoError(t, err)
	resp.Body.Close()

	// Delete the original seed user
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://localhost:%d/api/users/user-1", httpPort), nil)
	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	resp2.Body.Close()

	// Verify changes persisted (only 1 user, the new one)
	resp3, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users", httpPort))
	require.NoError(t, err)
	defer resp3.Body.Close()

	var result stateful.PaginatedResponse
	err = json.NewDecoder(resp3.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Meta.Total, "should have 1 user (new one, seed deleted)")
}

// =============================================================================
// User Story 3: Auto-Generated IDs and Timestamps
// =============================================================================

func TestStateful_US3_PostWithoutIDGeneratesUniqueID(t *testing.T) {
	// T038: Integration test: POST without ID field generates unique ID
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create multiple items without IDs
	ids := make(map[string]bool)
	for i := 0; i < 5; i++ {
		body := bytes.NewBufferString(fmt.Sprintf(`{"name": "Item %d"}`, i))
		resp, err := http.Post(
			fmt.Sprintf("http://localhost:%d/api/items", httpPort),
			"application/json",
			body,
		)
		require.NoError(t, err)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()
		require.NoError(t, err)

		id := result["id"].(string)
		assert.NotEmpty(t, id, "should have auto-generated ID")
		assert.False(t, ids[id], "ID should be unique")
		ids[id] = true
	}

	assert.Len(t, ids, 5, "should have 5 unique IDs")
}

func TestStateful_US3_CreatedResourceHasCreatedAtTimestamp(t *testing.T) {
	// T039: Integration test: Created resource has createdAt timestamp
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	beforeCreate := time.Now().Add(-1 * time.Second)

	body := bytes.NewBufferString(`{"name": "Test Item"}`)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/items", httpPort),
		"application/json",
		body,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	createdAtStr, ok := result["createdAt"].(string)
	require.True(t, ok, "createdAt should be a string")

	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	require.NoError(t, err, "createdAt should be valid RFC3339")

	assert.True(t, createdAt.After(beforeCreate), "createdAt should be recent")
}

func TestStateful_US3_UpdatedResourceHasNewUpdatedAtTimestamp(t *testing.T) {
	// T040: Integration test: Updated resource has new updatedAt timestamp
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "item-1", "name": "Original"},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Get original to compare timestamps
	resp1, err := http.Get(fmt.Sprintf("http://localhost:%d/api/items/item-1", httpPort))
	require.NoError(t, err)
	var original map[string]interface{}
	json.NewDecoder(resp1.Body).Decode(&original)
	resp1.Body.Close()

	originalUpdatedAt, _ := time.Parse(time.RFC3339, original["updatedAt"].(string))

	// Wait longer to ensure timestamp difference (RFC3339 has second precision)
	time.Sleep(1100 * time.Millisecond)

	// Update the item
	body := bytes.NewBufferString(`{"name": "Updated"}`)
	req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("http://localhost:%d/api/items/item-1", httpPort), body)
	req.Header.Set("Content-Type", "application/json")

	resp2, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()

	var updated map[string]interface{}
	err = json.NewDecoder(resp2.Body).Decode(&updated)
	require.NoError(t, err)

	updatedUpdatedAt, err := time.Parse(time.RFC3339, updated["updatedAt"].(string))
	require.NoError(t, err)

	// Use !Before instead of After to handle equal times (>= instead of >)
	assert.True(t, !updatedUpdatedAt.Before(originalUpdatedAt), "updatedAt should be newer or equal after update")
	// Also verify the timestamps are different (we waited 1+ second)
	assert.NotEqual(t, originalUpdatedAt.Unix(), updatedUpdatedAt.Unix(), "updatedAt should have changed")
}

// =============================================================================
// User Story 4: State Reset and Management
// =============================================================================

func TestStateful_US4_PostResetResetsAllResources(t *testing.T) {
	// T045: Integration test: POST /admin/state/reset resets all resources
	srv, httpPort, adminPort := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "user-1", "name": "Original"},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Add a new user
	body := bytes.NewBufferString(`{"name": "NewUser"}`)
	resp1, _ := http.Post(fmt.Sprintf("http://localhost:%d/api/users", httpPort), "application/json", body)
	resp1.Body.Close()

	// Delete the seed user
	req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("http://localhost:%d/api/users/user-1", httpPort), nil)
	resp2, _ := http.DefaultClient.Do(req)
	resp2.Body.Close()

	// Reset state via admin API
	resp3, err := http.Post(fmt.Sprintf("http://localhost:%d/state/reset", adminPort), "application/json", nil)
	require.NoError(t, err)
	defer resp3.Body.Close()

	assert.Equal(t, http.StatusOK, resp3.StatusCode)

	// Verify state is reset - original user should be back
	resp4, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users/user-1", httpPort))
	require.NoError(t, err)
	defer resp4.Body.Close()

	assert.Equal(t, http.StatusOK, resp4.StatusCode, "seed user should be restored")

	// Verify collection only has seed data
	resp5, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users", httpPort))
	require.NoError(t, err)
	defer resp5.Body.Close()

	var result stateful.PaginatedResponse
	json.NewDecoder(resp5.Body).Decode(&result)

	assert.Equal(t, 1, result.Meta.Total, "should only have original seed user")
}

func TestStateful_US4_ResetWithResourceParamResetsOnlyThat(t *testing.T) {
	// T046: Integration test: Reset with resource param resets only that resource
	srv, httpPort, adminPort := createStatefulServer(t,
		&statefulResourceConfig{
			Name:     "users",
			BasePath: "/api/users",
			SeedData: []map[string]interface{}{
				{"id": "user-1", "name": "User1"},
			},
		},
		&statefulResourceConfig{
			Name:     "products",
			BasePath: "/api/products",
			SeedData: []map[string]interface{}{
				{"id": "prod-1", "name": "Product1"},
			},
		},
	)

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Add items to both resources
	body1 := bytes.NewBufferString(`{"name": "NewUser"}`)
	resp1, _ := http.Post(fmt.Sprintf("http://localhost:%d/api/users", httpPort), "application/json", body1)
	resp1.Body.Close()

	body2 := bytes.NewBufferString(`{"name": "NewProduct"}`)
	resp2, _ := http.Post(fmt.Sprintf("http://localhost:%d/api/products", httpPort), "application/json", body2)
	resp2.Body.Close()

	// Reset only users resource
	resp3, err := http.Post(fmt.Sprintf("http://localhost:%d/state/reset?resource=users", adminPort), "application/json", nil)
	require.NoError(t, err)
	resp3.Body.Close()

	// Verify users is reset (only 1 item)
	resp4, _ := http.Get(fmt.Sprintf("http://localhost:%d/api/users", httpPort))
	var usersResult stateful.PaginatedResponse
	json.NewDecoder(resp4.Body).Decode(&usersResult)
	resp4.Body.Close()

	assert.Equal(t, 1, usersResult.Meta.Total, "users should be reset to seed data only")

	// Verify products still has 2 items (not reset)
	resp5, _ := http.Get(fmt.Sprintf("http://localhost:%d/api/products", httpPort))
	var productsResult stateful.PaginatedResponse
	json.NewDecoder(resp5.Body).Decode(&productsResult)
	resp5.Body.Close()

	assert.Equal(t, 2, productsResult.Meta.Total, "products should NOT be reset")
}

func TestStateful_US4_GetStateReturnsOverview(t *testing.T) {
	// T047: Integration test: GET /admin/state returns state overview
	srv, httpPort, adminPort := createStatefulServer(t,
		&statefulResourceConfig{
			Name:     "users",
			BasePath: "/api/users",
			SeedData: []map[string]interface{}{
				{"id": "user-1", "name": "User1"},
			},
		},
		&statefulResourceConfig{
			Name:     "products",
			BasePath: "/api/products",
		},
	)

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Add some products
	body := bytes.NewBufferString(`{"name": "Product1"}`)
	resp1, _ := http.Post(fmt.Sprintf("http://localhost:%d/api/products", httpPort), "application/json", body)
	resp1.Body.Close()

	// Get state overview
	resp2, err := http.Get(fmt.Sprintf("http://localhost:%d/state", adminPort))
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	// Use anonymous struct matching the API response format (api.StateOverview)
	var overview struct {
		Resources []struct {
			Name      string `json:"name"`
			ItemCount int    `json:"itemCount"`
		} `json:"resources"`
		Total        int      `json:"total"`
		TotalItems   int      `json:"totalItems"`
		ResourceList []string `json:"resourceList"`
	}
	err = json.NewDecoder(resp2.Body).Decode(&overview)
	require.NoError(t, err)

	assert.Equal(t, 2, len(overview.Resources), "should have 2 resources")
	assert.Equal(t, 2, overview.TotalItems, "should have 2 total items (1 user + 1 product)")
	assert.Contains(t, overview.ResourceList, "users")
	assert.Contains(t, overview.ResourceList, "products")
}

func TestStateful_US4_GetResourceDetailsReturnsInfo(t *testing.T) {
	// T048: Integration test: GET /admin/state/resources/{name} returns resource details
	srv, _, adminPort := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "user-1", "name": "User1"},
			{"id": "user-2", "name": "User2"},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Get resource details - the endpoint is /state/resources/{name}
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/state/resources/users", adminPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Use anonymous struct matching the API response format
	var info struct {
		Name      string `json:"name"`
		BasePath  string `json:"basePath"`
		ItemCount int    `json:"itemCount"`
		SeedCount int    `json:"seedCount"`
		IDField   string `json:"idField"`
	}
	err = json.NewDecoder(resp.Body).Decode(&info)
	require.NoError(t, err)

	assert.Equal(t, "users", info.Name)
	assert.Equal(t, "/api/users", info.BasePath)
	assert.Equal(t, 2, info.ItemCount)
	assert.Equal(t, 2, info.SeedCount)
	assert.Equal(t, "id", info.IDField)
}

// =============================================================================
// User Story 5: Query Filtering and Pagination
// =============================================================================

func TestStateful_US5_LimitOffsetReturnsCorrectPage(t *testing.T) {
	// T058: Integration test: ?limit=10&offset=20 returns correct page
	seedData := make([]map[string]interface{}, 50)
	for i := 0; i < 50; i++ {
		seedData[i] = map[string]interface{}{
			"id":   fmt.Sprintf("item-%02d", i),
			"name": fmt.Sprintf("Item %d", i),
		}
	}

	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: seedData,
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Get page with limit=10, offset=20
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/items?limit=10&offset=20", httpPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	var result stateful.PaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, 50, result.Meta.Total, "total should be 50")
	assert.Equal(t, 10, result.Meta.Limit)
	assert.Equal(t, 20, result.Meta.Offset)
	assert.Equal(t, 10, result.Meta.Count, "should return 10 items")
	assert.Len(t, result.Data, 10)
}

func TestStateful_US5_FilterByFieldValue(t *testing.T) {
	// T059: Integration test: ?status=active filters by field value
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "user-1", "name": "Alice", "status": "active"},
			{"id": "user-2", "name": "Bob", "status": "inactive"},
			{"id": "user-3", "name": "Charlie", "status": "active"},
			{"id": "user-4", "name": "Diana", "status": "pending"},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Filter by status=active
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users?status=active", httpPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	var result stateful.PaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, 2, result.Meta.Total, "should find 2 active users")
	for _, item := range result.Data {
		assert.Equal(t, "active", item["status"], "all returned items should have status=active")
	}
}

func TestStateful_US5_SortByFieldWithOrder(t *testing.T) {
	// T060: Integration test: ?sort=name&order=asc sorts correctly
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "users",
		BasePath: "/api/users",
		SeedData: []map[string]interface{}{
			{"id": "user-1", "name": "Charlie"},
			{"id": "user-2", "name": "Alice"},
			{"id": "user-3", "name": "Bob"},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Sort by name ascending
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users?sort=name&order=asc", httpPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	var result stateful.PaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	require.Len(t, result.Data, 3)
	assert.Equal(t, "Alice", result.Data[0]["name"])
	assert.Equal(t, "Bob", result.Data[1]["name"])
	assert.Equal(t, "Charlie", result.Data[2]["name"])
}

func TestStateful_US5_ResponseIncludesPaginationMeta(t *testing.T) {
	// T061: Integration test: Response includes meta with total/limit/offset/count
	srv, httpPort, _ := createStatefulServer(t, &statefulResourceConfig{
		Name:     "items",
		BasePath: "/api/items",
		SeedData: []map[string]interface{}{
			{"id": "item-1", "name": "Item1"},
			{"id": "item-2", "name": "Item2"},
			{"id": "item-3", "name": "Item3"},
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/items?limit=2", httpPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	var result stateful.PaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, 3, result.Meta.Total, "total should reflect all items")
	assert.Equal(t, 2, result.Meta.Limit, "limit should be 2")
	assert.Equal(t, 0, result.Meta.Offset, "offset should be 0")
	assert.Equal(t, 2, result.Meta.Count, "count should be 2 (items returned)")
}

// =============================================================================
// User Story 6: Relationship and Nested Resources
// =============================================================================

func TestStateful_US6_GetNestedResourcesFiltersByParent(t *testing.T) {
	// T069: Integration test: GET /users/123/orders returns only orders for user 123
	srv, httpPort, _ := createStatefulServer(t,
		&statefulResourceConfig{
			Name:     "users",
			BasePath: "/api/users",
			SeedData: []map[string]interface{}{
				{"id": "user-1", "name": "Alice"},
				{"id": "user-2", "name": "Bob"},
			},
		},
		&statefulResourceConfig{
			Name:        "orders",
			BasePath:    "/api/users/:userId/orders",
			ParentField: "userId",
			SeedData: []map[string]interface{}{
				{"id": "order-1", "userId": "user-1", "total": 100},
				{"id": "order-2", "userId": "user-1", "total": 200},
				{"id": "order-3", "userId": "user-2", "total": 300},
			},
		},
	)

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Get orders for user-1
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/users/user-1/orders", httpPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result stateful.PaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, 2, result.Meta.Total, "should find 2 orders for user-1")
	for _, order := range result.Data {
		assert.Equal(t, "user-1", order["userId"], "all orders should belong to user-1")
	}
}

func TestStateful_US6_PostNestedResourceAutoSetsParentID(t *testing.T) {
	// T070: Integration test: POST /users/123/orders auto-sets userId field
	srv, httpPort, _ := createStatefulServer(t,
		&statefulResourceConfig{
			Name:     "users",
			BasePath: "/api/users",
			SeedData: []map[string]interface{}{
				{"id": "user-1", "name": "Alice"},
			},
		},
		&statefulResourceConfig{
			Name:        "orders",
			BasePath:    "/api/users/:userId/orders",
			ParentField: "userId",
		},
	)

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create order for user-1 (without explicitly setting userId)
	body := bytes.NewBufferString(`{"total": 150, "item": "Widget"}`)
	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/api/users/user-1/orders", httpPort),
		"application/json",
		body,
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, "user-1", result["userId"], "userId should be auto-set from path")
}

func TestStateful_US6_PathParameterExtractedFromNestedPath(t *testing.T) {
	// T071: Integration test: Path parameter extracted from nested path
	srv, httpPort, _ := createStatefulServer(t,
		&statefulResourceConfig{
			Name:        "comments",
			BasePath:    "/api/posts/:postId/comments",
			ParentField: "postId",
			SeedData: []map[string]interface{}{
				{"id": "comment-1", "postId": "post-A", "text": "Comment on A"},
				{"id": "comment-2", "postId": "post-B", "text": "Comment on B"},
			},
		},
	)

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Get comments for post-A
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/posts/post-A/comments", httpPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	var result stateful.PaginatedResponse
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)

	assert.Equal(t, 1, result.Meta.Total, "should find 1 comment for post-A")
	assert.Equal(t, "Comment on A", result.Data[0]["text"])

	// Get single comment by ID
	resp2, err := http.Get(fmt.Sprintf("http://localhost:%d/api/posts/post-A/comments/comment-1", httpPort))
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)
}
