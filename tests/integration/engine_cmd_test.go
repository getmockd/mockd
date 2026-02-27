package integration

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEngineCmd_E2E_PortZeroAndTraffic(t *testing.T) {
	binaryPath := buildBinary(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mocks.yaml")
	configContent := `version: "1.0"
name: e2e-engine-test
mocks:
  - id: e2e-hello
    name: E2E Hello
    type: http
    enabled: true
    http:
      matcher:
        method: GET
        path: /api/ping
      response:
        statusCode: 200
        body: '{"pong": true}'
        headers:
          Content-Type: application/json
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Start the engine binary with --print-url to capture the URL
	cmd := exec.Command(binaryPath, "engine",
		"--config", configPath,
		"--port", "0",
		"--print-url",
		"--log-level", "error",
	)

	// Capture stdout for the URL
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start engine binary: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Read the first line from stdout (the URL)
	scanner := bufio.NewScanner(stdoutPipe)
	baseURL := ""
	urlCh := make(chan string, 1)
	go func() {
		if scanner.Scan() {
			urlCh <- strings.TrimSpace(scanner.Text())
		}
	}()

	select {
	case url := <-urlCh:
		baseURL = url
	case <-time.After(10 * time.Second):
		t.Fatalf("engine did not print URL within 10s\nstderr: %s", stderr.String())
	}

	if baseURL == "" {
		t.Fatalf("engine printed empty URL\nstderr: %s", stderr.String())
	}

	t.Logf("Engine started at %s", baseURL)

	// Wait for server to be ready
	if !waitForServer(baseURL+"/__mockd/health", 10*time.Second) {
		t.Fatalf("engine did not become healthy at %s\nstderr: %s", baseURL, stderr.String())
	}

	// Test the mock endpoint
	t.Run("MockEndpoint", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/ping")
		if err != nil {
			t.Fatalf("failed to call mock: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if result["pong"] != true {
			t.Errorf("expected pong=true, got %v", result)
		}
	})

	// Test health endpoint
	t.Run("HealthEndpoint", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/__mockd/health")
		if err != nil {
			t.Fatalf("health check failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Errorf("expected health 200, got %d", resp.StatusCode)
		}
	})

	// Clean shutdown
	t.Run("GracefulShutdown", func(t *testing.T) {
		_ = cmd.Process.Signal(os.Interrupt)

		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		select {
		case <-done:
			// Exited cleanly
		case <-time.After(5 * time.Second):
			t.Error("engine did not shut down gracefully within 5 seconds")
			_ = cmd.Process.Kill()
		}
	})
}

func TestEngineCmd_E2E_MissingConfigErrors(t *testing.T) {
	binaryPath := buildBinary(t)

	cmd := exec.Command(binaryPath, "engine", "--config", "/nonexistent/file.yaml")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for missing config file")
	}

	if !strings.Contains(string(output), "config file not found") &&
		!strings.Contains(string(output), "not found") {
		t.Errorf("expected 'not found' error, got: %s", output)
	}
}

func TestEngineCmd_E2E_RequiresConfigFlag(t *testing.T) {
	binaryPath := buildBinary(t)

	cmd := exec.Command(binaryPath, "engine")
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error when --config is not provided")
	}

	if !strings.Contains(string(output), "required") {
		t.Errorf("expected 'required' error for --config, got: %s", output)
	}
}

func TestEngineCmd_E2E_InvalidConfigErrors(t *testing.T) {
	binaryPath := buildBinary(t)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "bad.yaml")
	if err := os.WriteFile(configPath, []byte("{{invalid yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binaryPath, "engine", "--config", configPath)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected error for invalid config")
	}

	_ = fmt.Sprintf("output: %s", output) // Use the output for debugging if needed
}
