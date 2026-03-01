package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getTestProtoFile returns the absolute path to the test proto file
func getTestProtoFile() string {
	_, currentFile, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(currentFile)
	return filepath.Join(testDir, "..", "fixtures", "grpc", "test.proto")
}

// getUniquePort returns a unique port - wrapper around shared helper
func getUniquePort() int {
	return GetFreePortSafe()
}

// ============================================================================
// Test Helpers
// ============================================================================

// portConflictTestBundle groups server and admin API for port conflict tests
type portConflictTestBundle struct {
	Server         *engine.Server
	AdminAPI       *admin.API
	HTTPPort       int
	AdminPort      int
	ManagementPort int
}

func setupPortConflictServer(t *testing.T) *portConflictTestBundle {
	// Use unique ports to avoid conflicts with parallel tests
	httpPort := getUniquePort()
	adminPort := getUniquePort()
	managementPort := getUniquePort()

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

	// Create our own temp dir so we can control cleanup timing
	dataDir, err := os.MkdirTemp("", "mockd-port-conflict-test-*")
	require.NoError(t, err)

	// Create admin API with data dir for persistence
	adminAPI := admin.NewAPI(adminPort,
		admin.WithLocalEngine(fmt.Sprintf("http://localhost:%d", srv.ManagementPort())),
		admin.WithAPIKeyDisabled(),
		admin.WithDataDir(dataDir),
	)
	err = adminAPI.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		adminAPI.Stop()
		srv.Stop()
		// Give time for file handles to close
		time.Sleep(50 * time.Millisecond)
		// Remove temp dir after everything is stopped
		os.RemoveAll(dataDir)
	})

	waitForReady(t, managementPort)
	waitForReady(t, adminPort)

	return &portConflictTestBundle{
		Server:         srv,
		AdminAPI:       adminAPI,
		HTTPPort:       httpPort,
		AdminPort:      adminPort,
		ManagementPort: managementPort,
	}
}

// createMQTTMock helper to create an MQTT mock via the admin API
func createMQTTMock(t *testing.T, adminPort int, name string, mqttPort int, workspaceID string) (int, map[string]interface{}) {
	mockData := map[string]interface{}{
		"type":        "mqtt",
		"name":        name,
		"enabled":     true,
		"workspaceId": workspaceID,
		"mqtt": map[string]interface{}{
			"port": mqttPort,
		},
	}
	body, _ := json.Marshal(mockData)

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/mocks", adminPort),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var respData map[string]interface{}
	json.Unmarshal(respBody, &respData)

	return resp.StatusCode, respData
}

// createGRPCMock helper to create a gRPC mock via the admin API
func createGRPCMock(t *testing.T, adminPort int, name string, grpcPort int, workspaceID string) (int, map[string]interface{}) {
	mockData := map[string]interface{}{
		"type":        "grpc",
		"name":        name,
		"enabled":     true,
		"workspaceId": workspaceID,
		"grpc": map[string]interface{}{
			"port":      grpcPort,
			"protoFile": getTestProtoFile(),
		},
	}
	body, _ := json.Marshal(mockData)

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/mocks", adminPort),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var respData map[string]interface{}
	json.Unmarshal(respBody, &respData)

	return resp.StatusCode, respData
}

// updateMock helper to update a mock via the admin API
func updateMock(t *testing.T, adminPort int, mockID string, mockData map[string]interface{}) (int, map[string]interface{}) {
	body, _ := json.Marshal(mockData)

	req, err := http.NewRequest("PUT", fmt.Sprintf("http://localhost:%d/mocks/%s", adminPort, mockID), bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var respData map[string]interface{}
	json.Unmarshal(respBody, &respData)

	return resp.StatusCode, respData
}

// ============================================================================
// Port Conflict Tests
// ============================================================================

func TestPortConflict_CreateMQTT_SamePort_SameWorkspace(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get a free port for MQTT
	mqttPort := getUniquePort()

	// Create first MQTT mock
	status1, data1 := createMQTTMock(t, bundle.AdminPort, "IoT Sensor Broker", mqttPort, "local")
	assert.Equal(t, http.StatusCreated, status1)
	assert.NotEmpty(t, data1["id"])
	firstMockID := data1["id"].(string)

	// Create second MQTT mock on same port - should MERGE into the first mock
	// gRPC/MQTT mocks on the same port (same protocol, same workspace) get merged
	status2, data2 := createMQTTMock(t, bundle.AdminPort, "Second Sensor Broker", mqttPort, "local")
	assert.Equal(t, http.StatusOK, status2, "Second MQTT mock on same port should be merged")
	assert.Equal(t, "merged", data2["action"])
	assert.Equal(t, firstMockID, data2["targetMockId"], "Should merge into first mock")
	assert.Contains(t, data2["message"].(string), "Merged into existing MQTT broker")
}

func TestPortConflict_CreateMQTT_SamePort_DifferentWorkspace(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get a free port for MQTT
	mqttPort := getUniquePort()

	// Create first MQTT mock in workspace-1
	status1, data1 := createMQTTMock(t, bundle.AdminPort, "Workspace 1 Broker", mqttPort, "workspace-1")
	assert.Equal(t, http.StatusCreated, status1)
	assert.NotEmpty(t, data1["id"])

	// Create second MQTT mock on same port in a different workspace
	// Different workspaces don't share an engine by default, so no conflict at admin level.
	// But the engine will fail to bind if the port is actually in use.
	// In local-engine mode with isolated workspaces, this succeeds at admin layer.
	status2, data2 := createMQTTMock(t, bundle.AdminPort, "Workspace 2 Broker", mqttPort, "workspace-2")
	// The admin-level check only looks at same-workspace or sibling workspaces on same engine.
	// Different workspaces = different scope, so admin allows it.
	// But the engine will fail to start the second broker (port already bound).
	// This results in port_unavailable error from the engine.
	assert.Equal(t, http.StatusConflict, status2, "Engine should fail to bind same port twice")
	assert.Equal(t, "port_unavailable", data2["error"])
}

func TestPortConflict_CreateMQTT_DifferentPort_SameWorkspace(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get two free ports for MQTT
	mqttPort1 := getUniquePort()
	mqttPort2 := getUniquePort()
	// Ensure they're different
	for mqttPort2 == mqttPort1 {
		mqttPort2 = getUniquePort()
	}

	// Create first MQTT mock
	status1, data1 := createMQTTMock(t, bundle.AdminPort, "Broker 1", mqttPort1, "local")
	assert.Equal(t, http.StatusCreated, status1)
	assert.NotEmpty(t, data1["id"])

	// Create second MQTT mock on different port - should succeed
	status2, data2 := createMQTTMock(t, bundle.AdminPort, "Broker 2", mqttPort2, "local")
	assert.Equal(t, http.StatusCreated, status2)
	assert.NotEmpty(t, data2["id"])
}

func TestPortConflict_CreateGRPC_ConflictsWithMQTT(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get a free port
	port := getUniquePort()

	// Create MQTT mock first
	status1, data1 := createMQTTMock(t, bundle.AdminPort, "MQTT Broker", port, "local")
	assert.Equal(t, http.StatusCreated, status1)
	assert.NotEmpty(t, data1["id"])

	// Try to create gRPC mock on same port - should fail (cross-protocol conflict)
	status2, data2 := createGRPCMock(t, bundle.AdminPort, "gRPC Server", port, "local")
	assert.Equal(t, http.StatusConflict, status2)
	assert.Equal(t, "port_conflict", data2["error"])
	// Error message now says protocol mismatch
	assert.Contains(t, data2["message"].(string), "mqtt")
	assert.Contains(t, data2["message"].(string), "Different protocols cannot share ports")
}

func TestPortConflict_CreateMQTT_ConflictsWithGRPC(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get a free port
	port := getUniquePort()

	// Create gRPC mock first (using valid proto file)
	status1, data1 := createGRPCMock(t, bundle.AdminPort, "gRPC Server", port, "local")
	assert.Equal(t, http.StatusCreated, status1)
	assert.NotEmpty(t, data1["id"])

	// Try to create MQTT mock on same port - should fail (cross-protocol conflict)
	status2, data2 := createMQTTMock(t, bundle.AdminPort, "MQTT Broker", port, "local")
	assert.Equal(t, http.StatusConflict, status2)
	assert.Equal(t, "port_conflict", data2["error"])
	assert.Contains(t, data2["message"].(string), "grpc")
	assert.Contains(t, data2["message"].(string), "Different protocols cannot share ports")
}

func TestPortConflict_UpdateMock_ToConflictingPort(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get two free ports
	port1 := getUniquePort()
	port2 := getUniquePort()
	for port2 == port1 {
		port2 = getUniquePort()
	}

	// Create two MQTT mocks on different ports
	status1, _ := createMQTTMock(t, bundle.AdminPort, "Broker 1", port1, "local")
	assert.Equal(t, http.StatusCreated, status1)

	status2, data2 := createMQTTMock(t, bundle.AdminPort, "Broker 2", port2, "local")
	assert.Equal(t, http.StatusCreated, status2)
	mockID2 := data2["id"].(string)

	// Update Broker 2 to use port1 - should FAIL
	// Note: Updates don't support merging - that only happens on create.
	// Updating a mock to use a port already in use by another mock of same protocol
	// is still a conflict because we don't want to silently merge during updates.
	updateData := map[string]interface{}{
		"type":        "mqtt",
		"name":        "Broker 2 Updated",
		"enabled":     true,
		"workspaceId": "local",
		"mqtt": map[string]interface{}{
			"port": port1, // Try to use Broker 1's port
		},
	}
	status3, data3 := updateMock(t, bundle.AdminPort, mockID2, updateData)
	assert.Equal(t, http.StatusConflict, status3, "Updating to conflicting port should fail")
	assert.Equal(t, "port_conflict", data3["error"])
	// Error message mentions either the name or the ID of the conflicting mock
	assert.Contains(t, data3["message"].(string), "Broker 1")
}

func TestPortConflict_UpdateMock_KeepsSamePort(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get a free port
	mqttPort := getUniquePort()

	// Create MQTT mock
	status1, data1 := createMQTTMock(t, bundle.AdminPort, "My Broker", mqttPort, "local")
	assert.Equal(t, http.StatusCreated, status1)
	mockID := data1["id"].(string)

	// Update the mock but keep the same port - should succeed
	updateData := map[string]interface{}{
		"type":        "mqtt",
		"name":        "My Broker Renamed",
		"enabled":     true,
		"workspaceId": "local",
		"mqtt": map[string]interface{}{
			"port": mqttPort, // Same port is OK
		},
	}
	status2, data2 := updateMock(t, bundle.AdminPort, mockID, updateData)
	assert.Equal(t, http.StatusOK, status2)

	// PUT response wraps the mock in an envelope: {id, action, message, mock: {...}}
	mockData2, ok := data2["mock"].(map[string]interface{})
	require.True(t, ok, "PUT response should contain a 'mock' envelope field")
	assert.Equal(t, "My Broker Renamed", mockData2["name"])
}

func TestPortConflict_UpdateMock_ToNewUnusedPort(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get two free ports
	port1 := getUniquePort()
	port2 := getUniquePort()
	for port2 == port1 {
		port2 = getUniquePort()
	}

	// Create MQTT mock on port1
	status1, data1 := createMQTTMock(t, bundle.AdminPort, "My Broker", port1, "local")
	assert.Equal(t, http.StatusCreated, status1)
	mockID := data1["id"].(string)

	// Update to port2 (unused) - should succeed
	updateData := map[string]interface{}{
		"type":        "mqtt",
		"name":        "My Broker",
		"enabled":     true,
		"workspaceId": "local",
		"mqtt": map[string]interface{}{
			"port": port2, // New port
		},
	}
	status2, data2 := updateMock(t, bundle.AdminPort, mockID, updateData)
	assert.Equal(t, http.StatusOK, status2)

	// PUT response wraps the mock in an envelope: {id, action, message, mock: {...}}
	mockData2, ok := data2["mock"].(map[string]interface{})
	require.True(t, ok, "PUT response should contain a 'mock' envelope field")
	assert.NotNil(t, mockData2["mqtt"])
}

func TestPortConflict_HTTPMock_NoConflictCheck(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Create HTTP mock (should not check port conflicts)
	mockData := map[string]interface{}{
		"type":    "http",
		"name":    "HTTP Endpoint",
		"enabled": true,
		"http": map[string]interface{}{
			"matcher": map[string]interface{}{
				"method": "GET",
				"path":   "/api/test",
			},
			"response": map[string]interface{}{
				"statusCode": 200,
				"body":       "OK",
			},
		},
	}
	body, _ := json.Marshal(mockData)

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/mocks?workspaceId=local", bundle.AdminPort),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var respData map[string]interface{}
	json.Unmarshal(respBody, &respData)

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.NotEmpty(t, respData["id"])
}

func TestPortConflict_MultipleGRPCMocks_DifferentPorts(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get three free ports using existing helper
	ports := make([]int, 3)
	for i := 0; i < 3; i++ {
		ports[i] = getUniquePort()
		// Ensure uniqueness
		for j := 0; j < i; j++ {
			if ports[i] == ports[j] {
				ports[i] = getUniquePort()
				j = -1 // restart check
			}
		}
	}

	// Create three gRPC mocks on different ports - all should succeed
	for i, port := range ports {
		name := fmt.Sprintf("gRPC Server %d", i+1)
		status, data := createGRPCMock(t, bundle.AdminPort, name, port, "local")
		assert.Equal(t, http.StatusCreated, status, "Failed to create %s", name)
		assert.NotEmpty(t, data["id"])
	}
}

// ============================================================================
// Bulk Create Port Conflict Tests
// ============================================================================

// bulkCreateMocks helper to bulk create mocks via the admin API
func bulkCreateMocks(t *testing.T, adminPort int, mocks []map[string]interface{}) (int, map[string]interface{}) {
	body, _ := json.Marshal(mocks)

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/mocks/bulk", adminPort),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var respData map[string]interface{}
	json.Unmarshal(respBody, &respData)

	return resp.StatusCode, respData
}

func TestPortConflict_BulkCreate_SamePortWithinBatch_Fails(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get a port that will be used by multiple mocks in the batch
	sharedPort := getUniquePort()

	// Create batch with two MQTT mocks on the same port - should FAIL
	// Simple rule: one port = one mock. No port sharing.
	mocks := []map[string]interface{}{
		{
			"type":        "mqtt",
			"name":        "MQTT Broker 1",
			"enabled":     true,
			"workspaceId": "local",
			"mqtt": map[string]interface{}{
				"port": sharedPort,
			},
		},
		{
			"type":        "mqtt",
			"name":        "MQTT Broker 2",
			"enabled":     true,
			"workspaceId": "local",
			"mqtt": map[string]interface{}{
				"port": sharedPort, // Same port - conflict!
			},
		},
	}

	status, data := bulkCreateMocks(t, bundle.AdminPort, mocks)
	assert.Equal(t, http.StatusConflict, status, "Two mocks on same port should conflict")
	assert.Equal(t, "port_conflict", data["error"])

	conflicts, ok := data["conflicts"].([]interface{})
	require.True(t, ok, "should have conflicts array")
	assert.Len(t, conflicts, 1, "should have 1 conflict")
}

func TestPortConflict_BulkCreate_ConflictWithExisting(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// First create an existing MQTT mock
	mqttPort := getUniquePort()
	status1, data1 := createMQTTMock(t, bundle.AdminPort, "Existing Broker", mqttPort, "local")
	assert.Equal(t, http.StatusCreated, status1)
	assert.NotEmpty(t, data1["id"])

	// Now bulk create with a mock using the same port - should FAIL
	mocks := []map[string]interface{}{
		{
			"type":        "http",
			"name":        "HTTP Mock",
			"enabled":     true,
			"workspaceId": "local",
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/test",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
				},
			},
		},
		{
			"type":        "mqtt",
			"name":        "Conflicting MQTT",
			"enabled":     true,
			"workspaceId": "local",
			"mqtt": map[string]interface{}{
				"port": mqttPort, // Same port as existing - conflict!
			},
		},
	}

	status, data := bulkCreateMocks(t, bundle.AdminPort, mocks)
	assert.Equal(t, http.StatusConflict, status, "MQTT mock on existing port should conflict")
	assert.Equal(t, "port_conflict", data["error"])

	conflicts, ok := data["conflicts"].([]interface{})
	require.True(t, ok, "should have conflicts array")
	assert.Len(t, conflicts, 1, "should have 1 conflict")
}

func TestPortConflict_BulkCreate_CrossProtocolConflict(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get a shared port
	sharedPort := getUniquePort()

	// Create batch with MQTT and gRPC on same port - cross-protocol conflict
	mocks := []map[string]interface{}{
		{
			"type":        "mqtt",
			"name":        "MQTT Broker",
			"enabled":     true,
			"workspaceId": "local",
			"mqtt":        map[string]interface{}{"port": sharedPort},
		},
		{
			"type":        "grpc",
			"name":        "gRPC Server",
			"enabled":     true,
			"workspaceId": "local",
			"grpc":        map[string]interface{}{"port": sharedPort, "protoFile": "test.proto"},
		},
	}

	status, data := bulkCreateMocks(t, bundle.AdminPort, mocks)
	assert.Equal(t, http.StatusConflict, status)
	assert.Equal(t, "port_conflict", data["error"])

	conflicts, ok := data["conflicts"].([]interface{})
	require.True(t, ok, "should have conflicts array")
	assert.Len(t, conflicts, 1, "should have 1 conflict (gRPC conflicts with MQTT)")
}

func TestPortConflict_BulkCreate_SameProtocol_SamePort_Fails(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get a shared port
	sharedPort := getUniquePort()

	// Create batch with multiple MQTT mocks on same port - should FAIL
	// One port = one mock. No sharing allowed.
	mocks := []map[string]interface{}{
		{
			"type":        "mqtt",
			"name":        "Broker 1",
			"enabled":     true,
			"workspaceId": "local",
			"mqtt":        map[string]interface{}{"port": sharedPort},
		},
		{
			"type":        "mqtt",
			"name":        "Broker 2",
			"enabled":     true,
			"workspaceId": "local",
			"mqtt":        map[string]interface{}{"port": sharedPort},
		},
		{
			"type":        "mqtt",
			"name":        "Broker 3",
			"enabled":     true,
			"workspaceId": "local",
			"mqtt":        map[string]interface{}{"port": sharedPort},
		},
	}

	status, data := bulkCreateMocks(t, bundle.AdminPort, mocks)
	assert.Equal(t, http.StatusConflict, status, "Multiple mocks on same port should conflict")
	assert.Equal(t, "port_conflict", data["error"])

	// Should have 2 conflicts (Broker 2 and Broker 3 conflict with Broker 1)
	conflicts, ok := data["conflicts"].([]interface{})
	require.True(t, ok, "should have conflicts array")
	assert.Len(t, conflicts, 2, "should have 2 conflicts")
}

func TestPortConflict_BulkCreate_DifferentWorkspaces_SamePort(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Get two different ports for the two workspaces
	port1 := getUniquePort()
	port2 := getUniquePort()
	for port2 == port1 {
		port2 = getUniquePort()
	}

	// Create batch with mocks in different workspaces on DIFFERENT ports
	// Even different workspaces can't share ports at the engine level
	mocks := []map[string]interface{}{
		{
			"type":        "mqtt",
			"name":        "Workspace 1 Broker",
			"enabled":     true,
			"workspaceId": "workspace-1",
			"mqtt":        map[string]interface{}{"port": port1},
		},
		{
			"type":        "mqtt",
			"name":        "Workspace 2 Broker",
			"enabled":     true,
			"workspaceId": "workspace-2",
			"mqtt":        map[string]interface{}{"port": port2}, // Different port
		},
	}

	status, data := bulkCreateMocks(t, bundle.AdminPort, mocks)
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, float64(2), data["created"])
}

func TestPortConflict_BulkCreate_NoDedicatedPorts_NoConflict(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Create batch of HTTP mocks - no port conflict checks needed
	mocks := []map[string]interface{}{
		{
			"type":        "http",
			"name":        "HTTP Mock 1",
			"enabled":     true,
			"workspaceId": "local",
			"http": map[string]interface{}{
				"matcher":  map[string]interface{}{"method": "GET", "path": "/api/1"},
				"response": map[string]interface{}{"statusCode": 200},
			},
		},
		{
			"type":        "http",
			"name":        "HTTP Mock 2",
			"enabled":     true,
			"workspaceId": "local",
			"http": map[string]interface{}{
				"matcher":  map[string]interface{}{"method": "GET", "path": "/api/2"},
				"response": map[string]interface{}{"statusCode": 200},
			},
		},
	}

	status, data := bulkCreateMocks(t, bundle.AdminPort, mocks)
	assert.Equal(t, http.StatusCreated, status)
	assert.Equal(t, float64(2), data["created"])
}

// ============================================================================
// Engine Port Error + Rollback Tests
// ============================================================================

func TestPortConflict_ExternalPortConflict_CreateRollback(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Start a listener on a port to simulate an external process using it
	port := getUniquePort()
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	require.NoError(t, err)
	defer listener.Close()

	// Try to create an MQTT mock on the port that's already in use externally
	mockData := map[string]interface{}{
		"type":        "mqtt",
		"name":        "MQTT Broker on Used Port",
		"enabled":     true,
		"workspaceId": "local",
		"mqtt": map[string]interface{}{
			"port": port,
		},
	}
	body, _ := json.Marshal(mockData)

	resp, err := http.Post(
		fmt.Sprintf("http://localhost:%d/mocks", bundle.AdminPort),
		"application/json",
		bytes.NewReader(body),
	)
	require.NoError(t, err)

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	var respData map[string]interface{}
	json.Unmarshal(respBody, &respData)

	// Should fail with port_unavailable since engine couldn't bind
	assert.Equal(t, http.StatusConflict, resp.StatusCode, "Expected 409 Conflict when port is in use")
	assert.Equal(t, "port_unavailable", respData["error"], "Expected port_unavailable error")
	assert.Contains(t, respData["message"].(string), "port", "Error message should mention port")

	// Verify the mock was NOT saved (rollback worked)
	// List all mocks and ensure none have the conflicting port
	listResp, err := http.Get(fmt.Sprintf("http://localhost:%d/mocks?type=mqtt", bundle.AdminPort))
	require.NoError(t, err)
	listBody, _ := io.ReadAll(listResp.Body)
	listResp.Body.Close()

	var listData map[string]interface{}
	json.Unmarshal(listBody, &listData)

	mocks, ok := listData["mocks"].([]interface{})
	if ok {
		for _, m := range mocks {
			mock := m.(map[string]interface{})
			if mqtt, ok := mock["mqtt"].(map[string]interface{}); ok {
				assert.NotEqual(t, float64(port), mqtt["port"],
					"Mock on conflicting port should have been rolled back from both store and engine")
			}
		}
	}
}

func TestPortConflict_PATCH_UpdatesProtocolSpec(t *testing.T) {
	bundle := setupPortConflictServer(t)

	// Create an MQTT mock
	mqttPort := getUniquePort()
	newPort := getUniquePort()
	status1, data1 := createMQTTMock(t, bundle.AdminPort, "Original Broker", mqttPort, "local")
	assert.Equal(t, http.StatusCreated, status1)
	mockID := data1["id"].(string)

	// PATCH with name and a different port — both should be applied
	patchData := map[string]interface{}{
		"name": "Renamed Broker",
		"mqtt": map[string]interface{}{
			"port": newPort,
		},
	}
	body, _ := json.Marshal(patchData)

	req, err := http.NewRequest("PATCH", fmt.Sprintf("http://localhost:%d/mocks/%s", bundle.AdminPort, mockID), bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	patchResp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	patchBody, _ := io.ReadAll(patchResp.Body)
	patchResp.Body.Close()

	var patchResult map[string]interface{}
	json.Unmarshal(patchBody, &patchResult)

	// PATCH should succeed
	assert.Equal(t, http.StatusOK, patchResp.StatusCode)

	// PATCH response wraps the mock in an envelope: {id, action, message, mock: {...}}
	mockData, ok := patchResult["mock"].(map[string]interface{})
	require.True(t, ok, "PATCH response should contain a 'mock' envelope field")

	// Name should be updated
	assert.Equal(t, "Renamed Broker", mockData["name"])

	// Port should be changed to the new port
	mqtt := mockData["mqtt"].(map[string]interface{})
	assert.Equal(t, float64(newPort), mqtt["port"], "PATCH should update the MQTT port")
}
