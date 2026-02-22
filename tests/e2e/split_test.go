package e2e_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSplitArchitecture verifies the distributed admin→engine architecture:
//   - Admin API starts with no local engine
//   - Engine starts separately with its own control API
//   - Engine registers with admin via POST /engines/register
//   - Mocks created via admin are served by the engine
//   - Heartbeat keeps engine status "online"
//
// This is the in-process equivalent of docker-compose.split.yml.
func TestSplitArchitecture(t *testing.T) {
	// Allocate ports for all four listeners
	engineHTTPPort := getFreePort(t)
	engineControlPort := getFreePort(t)
	adminPort := getFreePort(t)

	adminURL := "http://localhost:" + strconv.Itoa(adminPort)
	engineControlURL := "http://localhost:" + strconv.Itoa(engineControlPort)
	engineMockURL := "http://localhost:" + strconv.Itoa(engineHTTPPort)

	// ── Start Engine (data plane) ──────────────────────────────────────────
	engineCfg := &config.ServerConfiguration{
		HTTPPort:       engineHTTPPort,
		ManagementPort: engineControlPort,
	}
	eng := engine.NewServer(engineCfg)
	go func() { _ = eng.Start() }()
	defer eng.Stop()

	// ── Start Admin API (control plane, no local engine) ───────────────────
	adminAPI := admin.NewAPI(adminPort,
		admin.WithAPIKeyDisabled(),
		admin.WithDataDir(t.TempDir()),
	)
	go func() { _ = adminAPI.Start() }()
	defer adminAPI.Stop()

	// Wait for both to be healthy
	waitForServer(t, adminURL+"/health")
	waitForServer(t, engineControlURL+"/health")

	client := &http.Client{Timeout: 5 * time.Second}

	// ── Step 1: Register engine with admin ─────────────────────────────────
	t.Run("EngineRegistration", func(t *testing.T) {
		regBody, _ := json.Marshal(map[string]any{
			"name": "test-engine",
			"host": "localhost",
			"port": engineControlPort,
		})
		resp, err := client.Post(adminURL+"/engines/register", "application/json", bytes.NewReader(regBody))
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode, "engine registration should succeed")

		var regResp map[string]any
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&regResp))
		assert.NotEmpty(t, regResp["id"], "registration should return an engine ID")
	})

	// ── Step 2: Verify engine appears in admin's engine list ───────────────
	var engineID string
	t.Run("EngineVisible", func(t *testing.T) {
		resp, err := client.Get(adminURL + "/engines")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var listResp struct {
			Engines []map[string]any `json:"engines"`
			Total   int              `json:"total"`
		}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&listResp))
		require.NotEmpty(t, listResp.Engines, "admin should list at least one engine")

		// Find our engine
		for _, e := range listResp.Engines {
			if name, _ := e["name"].(string); name == "test-engine" {
				engineID, _ = e["id"].(string)
				break
			}
		}
		assert.NotEmpty(t, engineID, "our test-engine should appear in the list")
	})

	// ── Step 3: Send a heartbeat and verify status ─────────────────────────
	t.Run("Heartbeat", func(t *testing.T) {
		if engineID == "" {
			t.Skip("engine ID not available")
		}

		hbBody, _ := json.Marshal(map[string]any{
			"status": "online",
		})
		req, _ := http.NewRequest("POST", adminURL+"/engines/"+engineID+"/heartbeat", bytes.NewReader(hbBody))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode, "heartbeat should succeed")

		// Verify engine status is online
		resp2, err := client.Get(adminURL + "/engines/" + engineID)
		require.NoError(t, err)
		defer resp2.Body.Close()
		require.Equal(t, http.StatusOK, resp2.StatusCode)

		var detail map[string]any
		require.NoError(t, json.NewDecoder(resp2.Body).Decode(&detail))
		assert.Equal(t, "online", detail["status"], "engine status should be online after heartbeat")
	})

	// ── Step 4: Create a mock via admin, served by engine ──────────────────
	t.Run("MockCreationAndServing", func(t *testing.T) {
		mockBody, _ := json.Marshal(map[string]any{
			"type": "http",
			"http": map[string]any{
				"matcher": map[string]any{
					"method": "GET",
					"path":   "/api/split-test",
				},
				"response": map[string]any{
					"statusCode": 200,
					"body":       `{"source":"split-architecture"}`,
					"headers": map[string]string{
						"Content-Type": "application/json",
					},
				},
			},
		})
		resp, err := client.Post(adminURL+"/mocks", "application/json", bytes.NewReader(mockBody))
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode, "mock creation via admin should succeed: %s", string(body))

		var created map[string]any
		require.NoError(t, json.Unmarshal(body, &created))
		mockID, _ := created["id"].(string)
		assert.NotEmpty(t, mockID, "created mock should have an ID")

		// Hit the mock on the engine's HTTP port
		resp2, err := client.Get(engineMockURL + "/api/split-test")
		require.NoError(t, err)
		defer resp2.Body.Close()
		assert.Equal(t, http.StatusOK, resp2.StatusCode, "engine should serve the mock")

		respBody, _ := io.ReadAll(resp2.Body)
		assert.Contains(t, string(respBody), "split-architecture", "response should contain expected body")

		// Clean up
		if mockID != "" {
			req, _ := http.NewRequest("DELETE", adminURL+"/mocks/"+mockID, nil)
			delResp, err := client.Do(req)
			if err == nil {
				delResp.Body.Close()
			}
		}
	})
}
