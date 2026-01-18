package performance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// TestServer represents a mockd server started via CLI for realistic benchmarks.
// This ensures benchmarks reflect actual real-world usage patterns.
type TestServer struct {
	HTTPPort   int
	AdminPort  int
	cmd        *exec.Cmd
	binaryPath string
	dataDir    string // Isolated data directory for test isolation
}

var (
	buildMu    sync.Mutex
	binaryPath string
)

// ensureBinary builds the mockd binary, rebuilding if it doesn't exist.
func ensureBinary() (string, error) {
	buildMu.Lock()
	defer buildMu.Unlock()

	// Find project root
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	// Navigate to project root (from tests/performance)
	projectRoot := filepath.Join(wd, "..", "..")
	binaryPath = filepath.Join(projectRoot, "mockd_bench")

	// Check if binary exists
	if _, err := os.Stat(binaryPath); err == nil {
		return binaryPath, nil
	}

	// Build the binary
	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/mockd")
	cmd.Dir = projectRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to build CLI: %w\n%s", err, out)
	}

	return binaryPath, nil
}

// StartTestServer starts a mockd server via CLI and returns when ready.
// The server is started with the specified HTTP and Admin ports.
func StartTestServer(httpPort, adminPort int) (*TestServer, error) {
	binary, err := ensureBinary()
	if err != nil {
		return nil, err
	}

	// Create isolated data directory for test isolation
	dataDir, err := os.MkdirTemp("", "mockd-perf-test-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp data dir: %w", err)
	}

	ts := &TestServer{
		HTTPPort:   httpPort,
		AdminPort:  adminPort,
		binaryPath: binary,
		dataDir:    dataDir,
	}

	// Start the server with isolated data directory
	ts.cmd = exec.Command(binary, "start",
		"--port", fmt.Sprintf("%d", httpPort),
		"--admin-port", fmt.Sprintf("%d", adminPort),
		"--no-auth",
		"--data-dir", dataDir,
	)

	// Capture output for debugging
	ts.cmd.Stdout = io.Discard
	ts.cmd.Stderr = io.Discard

	if err := ts.cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}

	// Wait for server to be ready
	if err := ts.waitForReady(5 * time.Second); err != nil {
		ts.Stop()
		return nil, err
	}

	return ts, nil
}

// waitForReady polls the health endpoint until the server is ready.
func (ts *TestServer) waitForReady(timeout time.Duration) error {
	client := &http.Client{Timeout: 1 * time.Second}
	healthURL := fmt.Sprintf("http://localhost:%d/health", ts.AdminPort)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(healthURL)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("server not ready after %v", timeout)
}

// Stop gracefully stops the server and cleans up resources.
func (ts *TestServer) Stop() error {
	// Clean up data directory
	if ts.dataDir != "" {
		defer os.RemoveAll(ts.dataDir)
	}

	if ts.cmd == nil || ts.cmd.Process == nil {
		return nil
	}

	// Send SIGTERM for graceful shutdown
	if err := ts.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		// If SIGTERM fails, try SIGKILL
		ts.cmd.Process.Kill()
	}

	// Wait for process to exit
	done := make(chan error, 1)
	go func() {
		done <- ts.cmd.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		ts.cmd.Process.Kill()
		return fmt.Errorf("server did not stop gracefully")
	}
}

// CreateMock creates a mock via the Admin API.
func (ts *TestServer) CreateMock(mockConfig interface{}) error {
	body, err := json.Marshal(mockConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal mock config: %w", err)
	}

	url := fmt.Sprintf("http://localhost:%d/mocks", ts.AdminPort)
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create mock: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create mock failed with status %d: %s", resp.StatusCode, body)
	}

	return nil
}

// ResetState resets the server state via the Admin API.
func (ts *TestServer) ResetState() error {
	url := fmt.Sprintf("http://localhost:%d/state/reset", ts.AdminPort)
	resp, err := http.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to reset state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reset state failed with status %d", resp.StatusCode)
	}

	return nil
}

// AdminURL returns the base URL for the Admin API.
func (ts *TestServer) AdminURL() string {
	return fmt.Sprintf("http://localhost:%d", ts.AdminPort)
}

// HTTPURL returns the base URL for the mock HTTP server.
func (ts *TestServer) HTTPURL() string {
	return fmt.Sprintf("http://localhost:%d", ts.HTTPPort)
}
