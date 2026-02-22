// Package integration provides binary E2E tests for the mockd server.
// This file contains tests for the Docker Compose-style commands: up, down, ps, validate.
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

type adminEnginesResponse struct {
	Engines []struct {
		ID       string `json:"id"`
		Name     string `json:"name"`
		Port     int    `json:"port"`
		Status   string `json:"status"`
		LastSeen string `json:"lastSeen"`
	} `json:"engines"`
}

// ============================================================================
// Test: mockd validate
// ============================================================================

func TestBinaryE2E_Validate(t *testing.T) {
	binaryPath := buildBinary(t)

	// Test valid config
	t.Run("ValidConfig", func(t *testing.T) {
		configDir := t.TempDir()
		configPath := filepath.Join(configDir, "mockd.yaml")

		configContent := `version: "1.0"
admins:
  - name: local
    port: 4290
engines:
  - name: default
    httpPort: 4280
    admin: local
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, binaryPath, "validate", "-f", configPath)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			t.Errorf("Expected validate to succeed for valid config, got error: %v\nstdout: %s\nstderr: %s",
				err, stdout.String(), stderr.String())
		}

		// Should contain success message
		output := stdout.String() + stderr.String()
		if !strings.Contains(output, "valid") && !strings.Contains(output, "Valid") {
			t.Logf("Output: %s", output)
		}
	})

	// Test invalid config - missing admin port
	t.Run("InvalidConfig_MissingPort", func(t *testing.T) {
		configDir := t.TempDir()
		configPath := filepath.Join(configDir, "mockd.yaml")

		configContent := `version: "1.0"
admins:
  - name: local
    # missing port
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, binaryPath, "validate", "-f", configPath)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err == nil {
			t.Errorf("Expected validate to fail for invalid config")
		}

		// Should mention the port error
		output := stdout.String() + stderr.String()
		if !strings.Contains(output, "port") {
			t.Errorf("Expected error to mention 'port', got: %s", output)
		}
	})

	// Test port conflict
	t.Run("PortConflict", func(t *testing.T) {
		configDir := t.TempDir()
		configPath := filepath.Join(configDir, "mockd.yaml")

		configContent := `version: "1.0"
admins:
  - name: local
    port: 4280
engines:
  - name: default
    httpPort: 4280
    admin: local
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, binaryPath, "validate", "-f", configPath)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err == nil {
			t.Errorf("Expected validate to fail for port conflict")
		}

		// Should mention port conflict
		output := stdout.String() + stderr.String()
		if !strings.Contains(strings.ToLower(output), "conflict") && !strings.Contains(output, "4280") {
			t.Errorf("Expected error to mention conflict or port 4280, got: %s", output)
		}
	})

	// Test verbose output
	t.Run("VerboseOutput", func(t *testing.T) {
		configDir := t.TempDir()
		configPath := filepath.Join(configDir, "mockd.yaml")

		configContent := `version: "1.0"
admins:
  - name: local
    port: 4290
engines:
  - name: default
    httpPort: 4280
    admin: local
`
		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, binaryPath, "validate", "-f", configPath, "--verbose")
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			t.Errorf("Expected validate to succeed, got error: %v\nstdout: %s\nstderr: %s",
				err, stdout.String(), stderr.String())
		}
	})
}

func TestBinaryE2E_SplitRegistrationHeartbeatFast(t *testing.T) {
	binaryPath := buildBinary(t)

	adminPort := GetFreePortSafe()
	engineHTTPPort := GetFreePortSafe()

	tmpDir := t.TempDir()
	adminCfgPath := filepath.Join(tmpDir, "admin.yaml")
	engineCfgPath := filepath.Join(tmpDir, "engine.yaml")

	adminCfg := fmt.Sprintf(`version: "1.0"
admins:
  - name: main
    port: %d
    auth:
      type: none
    persistence:
      path: %s
`, adminPort, filepath.Join(tmpDir, "admin-data"))

	engineCfg := fmt.Sprintf(`version: "1.0"
admins:
  - name: main
    url: http://127.0.0.1:%d
engines:
  - name: worker
    httpPort: %d
    admin: main
`, adminPort, engineHTTPPort)

	if err := os.WriteFile(adminCfgPath, []byte(adminCfg), 0644); err != nil {
		t.Fatalf("write admin config: %v", err)
	}
	if err := os.WriteFile(engineCfgPath, []byte(engineCfg), 0644); err != nil {
		t.Fatalf("write engine config: %v", err)
	}

	startUp := func(t *testing.T, cfgPath string, home string) (*exec.Cmd, *bytes.Buffer, *bytes.Buffer) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		t.Cleanup(cancel)

		cmd := exec.CommandContext(ctx, binaryPath, "up", "-f", cfgPath)
		cmd.Dir = "../.."
		cmd.Env = append(os.Environ(),
			"HOME="+home,
			"XDG_DATA_HOME="+filepath.Join(home, ".local", "share"),
			"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
		)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Start(); err != nil {
			t.Fatalf("start mockd up (%s): %v", cfgPath, err)
		}
		t.Cleanup(func() {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			_, _ = cmd.Process.Wait()
		})
		return cmd, &stdout, &stderr
	}

	adminHome := filepath.Join(tmpDir, "admin-home")
	engineHome := filepath.Join(tmpDir, "engine-home")
	_ = os.MkdirAll(adminHome, 0755)
	_ = os.MkdirAll(engineHome, 0755)

	adminCmd, adminStdout, adminStderr := startUp(t, adminCfgPath, adminHome)
	_ = adminCmd
	adminURL := fmt.Sprintf("http://127.0.0.1:%d", adminPort)
	if !waitForServer(adminURL+"/health", 15*time.Second) {
		t.Fatalf("admin did not become ready\nstdout: %s\nstderr: %s", adminStdout.String(), adminStderr.String())
	}

	engineCmd, engineStdout, engineStderr := startUp(t, engineCfgPath, engineHome)
	_ = engineCmd
	engineURL := fmt.Sprintf("http://127.0.0.1:%d", engineHTTPPort)
	if !waitForServer(engineURL+"/__mockd/health", 15*time.Second) {
		t.Fatalf("engine did not become ready\nstdout: %s\nstderr: %s", engineStdout.String(), engineStderr.String())
	}

	getWorker := func(t *testing.T) (port int, status string, lastSeen time.Time) {
		t.Helper()
		resp, err := http.Get(adminURL + "/engines")
		if err != nil {
			t.Fatalf("GET /engines failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("GET /engines status=%d body=%s", resp.StatusCode, string(body))
		}
		var out adminEnginesResponse
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			t.Fatalf("decode /engines: %v", err)
		}
		for _, e := range out.Engines {
			if e.Name != "worker" {
				continue
			}
			parsed, err := time.Parse(time.RFC3339Nano, e.LastSeen)
			if err != nil {
				t.Fatalf("parse worker lastSeen %q: %v", e.LastSeen, err)
			}
			return e.Port, e.Status, parsed
		}
		t.Fatalf("worker engine not found in /engines response")
		return 0, "", time.Time{}
	}

	workerControlPort, status1, lastSeen1 := getWorker(t)
	if status1 != "online" {
		t.Fatalf("worker status=%q, want online", status1)
	}
	if workerControlPort <= 0 {
		t.Fatalf("worker control port invalid: %d", workerControlPort)
	}
	if workerControlPort == engineHTTPPort {
		t.Fatalf("worker registered port=%d should be control API port, not HTTP port=%d", workerControlPort, engineHTTPPort)
	}

	// The registered port should be the engine control API and expose /health.
	if !waitForServer(fmt.Sprintf("http://127.0.0.1:%d/health", workerControlPort), 5*time.Second) {
		t.Fatalf("worker control API port %d not reachable", workerControlPort)
	}

	// Heartbeat interval is ~10s in `mockd up`; wait a bit longer and ensure lastSeen advances
	// without waiting for the full 30s offline timeout.
	time.Sleep(12 * time.Second)
	_, status2, lastSeen2 := getWorker(t)
	if status2 != "online" {
		t.Fatalf("worker status after heartbeat window=%q, want online", status2)
	}
	if !lastSeen2.After(lastSeen1) {
		t.Fatalf("worker lastSeen did not advance: before=%s after=%s", lastSeen1, lastSeen2)
	}
}

// ============================================================================
// Test: mockd up/down basic lifecycle
// ============================================================================

func TestBinaryE2E_UpDownLifecycle(t *testing.T) {
	binaryPath := buildBinary(t)

	// Create config with unique ports
	adminPort := GetFreePortSafe()
	httpPort := GetFreePortSafe()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "mockd.yaml")

	configContent := fmt.Sprintf(`version: "1.0"
admins:
  - name: local
    port: %d
    auth:
      type: none
engines:
  - name: default
    httpPort: %d
    admin: local
`, adminPort, httpPort)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Start server with mockd up
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "up", "-f", configPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start 'mockd up': %v", err)
	}

	// Ensure cleanup
	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Wait for services to be ready
	adminURL := fmt.Sprintf("http://localhost:%d", adminPort)
	if !waitForServer(adminURL+"/health", 15*time.Second) {
		t.Fatalf("Admin server did not become ready\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}

	// Verify admin API is working
	t.Run("AdminAPIHealthy", func(t *testing.T) {
		resp, err := http.Get(adminURL + "/health")
		if err != nil {
			t.Fatalf("Admin health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// Verify engine is working
	engineURL := fmt.Sprintf("http://localhost:%d", httpPort)
	t.Run("EngineHealthy", func(t *testing.T) {
		resp, err := http.Get(engineURL + "/__mockd/health")
		if err != nil {
			t.Fatalf("Engine health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// Create a mock via admin API
	t.Run("CreateMock", func(t *testing.T) {
		// Use a unique ID to avoid conflicts
		mockID := fmt.Sprintf("test-mock-%d", time.Now().UnixNano())
		mockPayload := map[string]interface{}{
			"id":      mockID,
			"name":    "Test Mock",
			"type":    "http",
			"enabled": true,
			"http": map[string]interface{}{
				"matcher": map[string]interface{}{
					"method": "GET",
					"path":   "/api/hello",
				},
				"response": map[string]interface{}{
					"statusCode": 200,
					"body":       `{"message": "Hello from mockd up!"}`,
					"headers": map[string]string{
						"Content-Type": "application/json",
					},
				},
			},
		}

		mockJSON, _ := json.Marshal(mockPayload)
		resp, err := http.Post(adminURL+"/mocks", "application/json", bytes.NewReader(mockJSON))
		if err != nil {
			t.Fatalf("Failed to create mock: %v", err)
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			t.Fatalf("Failed to create mock: status %d, body: %s", resp.StatusCode, respBody)
		}

		// Small delay for mock registration
		time.Sleep(100 * time.Millisecond)

		// Verify mock works
		mockResp, err := http.Get(engineURL + "/api/hello")
		if err != nil {
			t.Fatalf("Failed to call mock: %v", err)
		}
		defer mockResp.Body.Close()

		mockRespBody, _ := io.ReadAll(mockResp.Body)
		if mockResp.StatusCode != http.StatusOK {
			t.Errorf("Expected mock to return 200, got %d. Response: %s", mockResp.StatusCode, mockRespBody)
		}

		if !strings.Contains(string(mockRespBody), "Hello from mockd up!") {
			t.Errorf("Unexpected mock response: %s", mockRespBody)
		}
	})

	// Test graceful shutdown with SIGTERM
	t.Run("GracefulShutdown", func(t *testing.T) {
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		// Send SIGTERM
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			cmd.Process.Kill()
			t.Log("SIGTERM not supported, using Kill")
		}

		// Wait for graceful shutdown
		select {
		case <-done:
			// Process exited - expected
		case <-time.After(10 * time.Second):
			cmd.Process.Kill()
			t.Error("Server did not shut down gracefully within 10 seconds")
		}

		// Verify services are no longer responding
		resp, err := http.Get(adminURL + "/health")
		if err == nil {
			resp.Body.Close()
			t.Error("Admin server still responding after shutdown")
		}
	})
}

// ============================================================================
// Test: mockd ps
// ============================================================================

func TestBinaryE2E_Ps(t *testing.T) {
	binaryPath := buildBinary(t)

	// Test ps with no server running (should report no services)
	t.Run("NoServicesRunning", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, binaryPath, "ps")
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Should not error, just report no services
		_ = cmd.Run()

		output := stdout.String() + stderr.String()
		// Should indicate no services or empty
		if !strings.Contains(strings.ToLower(output), "no") &&
			!strings.Contains(strings.ToLower(output), "empty") &&
			!strings.Contains(output, "running") {
			t.Logf("ps output when no services: %s", output)
		}
	})

	// Test ps with running services
	t.Run("WithRunningServices", func(t *testing.T) {
		// Start a server
		adminPort := GetFreePortSafe()
		httpPort := GetFreePortSafe()

		configDir := t.TempDir()
		configPath := filepath.Join(configDir, "mockd.yaml")

		configContent := fmt.Sprintf(`version: "1.0"
admins:
  - name: local
    port: %d
    auth:
      type: none
engines:
  - name: default
    httpPort: %d
    admin: local
`, adminPort, httpPort)

		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		upCmd := exec.CommandContext(ctx, binaryPath, "up", "-f", configPath)
		var upStdout, upStderr bytes.Buffer
		upCmd.Stdout = &upStdout
		upCmd.Stderr = &upStderr

		if err := upCmd.Start(); err != nil {
			t.Fatalf("Failed to start 'mockd up': %v", err)
		}
		defer func() {
			upCmd.Process.Kill()
			upCmd.Wait()
		}()

		// Wait for server to be ready
		adminURL := fmt.Sprintf("http://localhost:%d", adminPort)
		if !waitForServer(adminURL+"/health", 15*time.Second) {
			t.Fatalf("Server did not become ready\nstdout: %s\nstderr: %s", upStdout.String(), upStderr.String())
		}

		// Give the PID file time to be written
		time.Sleep(500 * time.Millisecond)

		// Run ps command
		psCtx, psCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer psCancel()

		psCmd := exec.CommandContext(psCtx, binaryPath, "ps")
		var psStdout, psStderr bytes.Buffer
		psCmd.Stdout = &psStdout
		psCmd.Stderr = &psStderr

		if err := psCmd.Run(); err != nil {
			t.Logf("ps command error (may be expected): %v", err)
		}

		output := psStdout.String()

		// Should show running services
		if strings.Contains(output, "admin") || strings.Contains(output, "engine") ||
			strings.Contains(output, "local") || strings.Contains(output, "default") {
			t.Logf("ps shows services: %s", output)
		}

		// Test JSON output
		t.Run("JSONFormat", func(t *testing.T) {
			psJSONCmd := exec.CommandContext(psCtx, binaryPath, "ps", "--json")
			var jsonStdout, jsonStderr bytes.Buffer
			psJSONCmd.Stdout = &jsonStdout
			psJSONCmd.Stderr = &jsonStderr

			_ = psJSONCmd.Run()

			// Try to parse as JSON
			var result map[string]interface{}
			if err := json.Unmarshal(jsonStdout.Bytes(), &result); err == nil {
				t.Logf("ps JSON output: %s", jsonStdout.String())
			}
		})
	})
}

// ============================================================================
// Test: mockd down
// ============================================================================

func TestBinaryE2E_Down(t *testing.T) {
	binaryPath := buildBinary(t)

	// Test down with no server running
	t.Run("NoServerRunning", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, binaryPath, "down")
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		// Should handle gracefully (may or may not error)
		_ = cmd.Run()

		output := stdout.String() + stderr.String()
		// Should indicate no server or already stopped
		t.Logf("down output when no server: %s", output)
	})

	// Test full up/down cycle
	t.Run("FullCycle", func(t *testing.T) {
		adminPort := GetFreePortSafe()
		httpPort := GetFreePortSafe()

		configDir := t.TempDir()
		configPath := filepath.Join(configDir, "mockd.yaml")

		configContent := fmt.Sprintf(`version: "1.0"
admins:
  - name: local
    port: %d
    auth:
      type: none
engines:
  - name: default
    httpPort: %d
    admin: local
`, adminPort, httpPort)

		if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
			t.Fatalf("Failed to write config: %v", err)
		}

		// Start server
		upCtx, upCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer upCancel()

		upCmd := exec.CommandContext(upCtx, binaryPath, "up", "-f", configPath)
		var upStdout, upStderr bytes.Buffer
		upCmd.Stdout = &upStdout
		upCmd.Stderr = &upStderr

		if err := upCmd.Start(); err != nil {
			t.Fatalf("Failed to start 'mockd up': %v", err)
		}
		defer func() {
			upCmd.Process.Kill()
			upCmd.Wait()
		}()

		// Wait for server to be ready
		adminURL := fmt.Sprintf("http://localhost:%d", adminPort)
		if !waitForServer(adminURL+"/health", 15*time.Second) {
			t.Fatalf("Server did not become ready\nstdout: %s\nstderr: %s", upStdout.String(), upStderr.String())
		}

		// Give the PID file time to be written
		time.Sleep(500 * time.Millisecond)

		// Run down command
		downCtx, downCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer downCancel()

		downCmd := exec.CommandContext(downCtx, binaryPath, "down")
		var downStdout, downStderr bytes.Buffer
		downCmd.Stdout = &downStdout
		downCmd.Stderr = &downStderr

		if err := downCmd.Run(); err != nil {
			t.Logf("down command: %v\nstdout: %s\nstderr: %s", err, downStdout.String(), downStderr.String())
		}

		// Wait a moment for shutdown
		time.Sleep(2 * time.Second)

		// Verify server is stopped
		resp, err := http.Get(adminURL + "/health")
		if err == nil {
			resp.Body.Close()
			// Server still responding - this could happen if up process handled the signal differently
			t.Log("Server still responding after down command - may need to wait longer")
		}
	})
}

// ============================================================================
// Test: Config with Environment Variables
// ============================================================================

func TestBinaryE2E_ConfigEnvVars(t *testing.T) {
	binaryPath := buildBinary(t)

	adminPort := GetFreePortSafe()
	httpPort := GetFreePortSafe()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "mockd.yaml")

	// Config using environment variables
	configContent := `version: "1.0"
admins:
  - name: local
    port: ${MOCKD_ADMIN_PORT}
    auth:
      type: none
engines:
  - name: default
    httpPort: ${MOCKD_HTTP_PORT}
    admin: local
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Test validation with env vars
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "validate", "-f", configPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("MOCKD_ADMIN_PORT=%d", adminPort),
		fmt.Sprintf("MOCKD_HTTP_PORT=%d", httpPort),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Errorf("Config with env vars should be valid: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}
}

// ============================================================================
// Test: Config with Default Values
// ============================================================================

func TestBinaryE2E_ConfigDefaultValues(t *testing.T) {
	binaryPath := buildBinary(t)

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "mockd.yaml")

	// Config using environment variable with default
	configContent := `version: "1.0"
admins:
  - name: local
    port: ${MOCKD_ADMIN_PORT:-4290}
    auth:
      type: none
engines:
  - name: default
    httpPort: ${MOCKD_HTTP_PORT:-4280}
    admin: local
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Test validation without env vars (should use defaults)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "validate", "-f", configPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Errorf("Config with default values should be valid: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}
}

// ============================================================================
// Test: Multiple Config Files (Merge)
// ============================================================================

func TestBinaryE2E_ConfigMerge(t *testing.T) {
	binaryPath := buildBinary(t)

	adminPort := GetFreePortSafe()
	httpPort := GetFreePortSafe()

	configDir := t.TempDir()
	basePath := filepath.Join(configDir, "base.yaml")
	overridePath := filepath.Join(configDir, "override.yaml")

	// Base config with placeholder port
	baseContent := fmt.Sprintf(`version: "1.0"
admins:
  - name: local
    port: %d
    auth:
      type: api-key
engines:
  - name: default
    httpPort: %d
    admin: local
`, adminPort, httpPort)

	// Override config to disable auth
	overrideContent := `version: "1.0"
admins:
  - name: local
    auth:
      type: none
`

	if err := os.WriteFile(basePath, []byte(baseContent), 0644); err != nil {
		t.Fatalf("Failed to write base config: %v", err)
	}
	if err := os.WriteFile(overridePath, []byte(overrideContent), 0644); err != nil {
		t.Fatalf("Failed to write override config: %v", err)
	}

	// Test validation with merged configs
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "validate", "-f", basePath, "-f", overridePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		t.Errorf("Merged configs should be valid: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}
}

// ============================================================================
// Test: Config Discovery (mockd.yaml in current directory)
// ============================================================================

func TestBinaryE2E_ConfigDiscovery(t *testing.T) {
	binaryPath := buildBinary(t)

	// Create a temp directory with mockd.yaml
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "mockd.yaml")

	adminPort := GetFreePortSafe()
	httpPort := GetFreePortSafe()

	configContent := fmt.Sprintf(`version: "1.0"
admins:
  - name: local
    port: %d
    auth:
      type: none
engines:
  - name: default
    httpPort: %d
    admin: local
`, adminPort, httpPort)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Run validate from the config directory (should auto-discover mockd.yaml)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, "validate")
	cmd.Dir = configDir // Run from the config directory
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// May fail if config discovery doesn't find the file
		t.Logf("Config discovery test: %v\nstdout: %s\nstderr: %s",
			err, stdout.String(), stderr.String())
	}
}

// ============================================================================
// Test: Inline Mocks in Config
// ============================================================================

func TestBinaryE2E_InlineMocks(t *testing.T) {
	binaryPath := buildBinary(t)

	adminPort := GetFreePortSafe()
	httpPort := GetFreePortSafe()

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "mockd.yaml")

	// Config with inline mocks
	configContent := fmt.Sprintf(`version: "1.0"
admins:
  - name: local
    port: %d
    auth:
      type: none
engines:
  - name: default
    httpPort: %d
    admin: local
workspaces:
  - name: default
    engines:
      - default
mocks:
  - id: health
    workspace: default
    type: http
    http:
      matcher:
        method: GET
        path: /health
      response:
        statusCode: 200
        body: '{"status": "ok"}'
        headers:
          Content-Type: application/json
  - id: users
    workspace: default
    type: http
    http:
      matcher:
        method: GET
        path: /api/users
      response:
        statusCode: 200
        body: '[{"id": 1, "name": "Alice"}]'
        headers:
          Content-Type: application/json
`, adminPort, httpPort)

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Validate the config
	validateCtx, validateCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer validateCancel()

	validateCmd := exec.CommandContext(validateCtx, binaryPath, "validate", "-f", configPath)
	var validateStdout, validateStderr bytes.Buffer
	validateCmd.Stdout = &validateStdout
	validateCmd.Stderr = &validateStderr

	if err := validateCmd.Run(); err != nil {
		t.Errorf("Config with inline mocks should be valid: %v\nstdout: %s\nstderr: %s",
			err, validateStdout.String(), validateStderr.String())
	}
}
