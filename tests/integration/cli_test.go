package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestCLIStartCommand tests the mockd start command lifecycle.
func TestCLIStartCommand(t *testing.T) {
	// Find available ports
	httpPort := findAvailablePort(t, 18080)
	adminPort := findAvailablePort(t, 19090)

	// Create isolated data directory for this test
	dataDir := t.TempDir()

	// Build the CLI binary
	buildCmd := exec.Command("go", "build", "-o", "mockd_test", "./cmd/mockd")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\n%s", err, out)
	}
	defer exec.Command("rm", "-f", "../../mockd_test").Run()

	// Start the server in background with isolated data directory
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "./mockd_test", "start",
		"--port", fmt.Sprintf("%d", httpPort),
		"--admin-port", fmt.Sprintf("%d", adminPort),
		"--no-auth",
		"--data-dir", dataDir,
	)
	cmd.Dir = "../.."

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Wait for server to be ready
	adminURL := fmt.Sprintf("http://localhost:%d", adminPort)
	if !waitForServer(adminURL+"/health", 10*time.Second) {
		cmd.Process.Kill()
		t.Fatalf("Server did not become ready in time\nstdout: %s\nstderr: %s", stdout.String(), stderr.String())
	}

	// Verify admin API responds
	resp, err := http.Get(adminURL + "/health")
	if err != nil {
		cmd.Process.Kill()
		t.Fatalf("Failed to reach admin API: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cmd.Process.Kill()
		t.Fatalf("Expected 200 from health endpoint, got %d", resp.StatusCode)
	}

	// Verify mock server responds
	mockURL := fmt.Sprintf("http://localhost:%d", httpPort)
	resp, err = http.Get(mockURL + "/")
	if err != nil {
		cmd.Process.Kill()
		t.Fatalf("Failed to reach mock server: %v", err)
	}
	resp.Body.Close()
	// 404 is expected since no mocks configured
	if resp.StatusCode != http.StatusNotFound {
		cmd.Process.Kill()
		t.Fatalf("Expected 404 from mock server (no mocks), got %d", resp.StatusCode)
	}

	// Kill the server
	cmd.Process.Kill()
	cmd.Wait()

	t.Log("CLI start command test passed")
}

// TestCLIMockCRUDCommands tests add, list, get, delete commands.
func TestCLIMockCRUDCommands(t *testing.T) {
	// Find available ports
	httpPort := findAvailablePort(t, 28080)
	adminPort := findAvailablePort(t, 29090)

	// Create isolated data directory for this test
	dataDir := t.TempDir()

	// Build the CLI binary
	buildCmd := exec.Command("go", "build", "-o", "mockd_test", "./cmd/mockd")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\n%s", err, out)
	}
	defer exec.Command("rm", "-f", "../../mockd_test").Run()

	// Start the server in background with isolated data directory
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serverCmd := exec.CommandContext(ctx, "./mockd_test", "start",
		"--port", fmt.Sprintf("%d", httpPort),
		"--admin-port", fmt.Sprintf("%d", adminPort),
		"--no-auth",
		"--data-dir", dataDir,
	)
	serverCmd.Dir = "../.."
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer serverCmd.Process.Kill()

	// Wait for server to be ready
	adminURL := fmt.Sprintf("http://localhost:%d", adminPort)
	if !waitForServer(adminURL+"/health", 10*time.Second) {
		t.Fatal("Server did not become ready in time")
	}

	// Test add command
	t.Run("add", func(t *testing.T) {
		addCmd := exec.Command("./mockd_test", "add",
			"--path", "/api/test",
			"--status", "201",
			"--body", `{"message": "created"}`,
			"-H", "Content-Type:application/json",
			"--admin-url", adminURL,
		)
		addCmd.Dir = "../.."
		out, err := addCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Add command failed: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "Created mock:") {
			t.Fatalf("Expected 'Created mock:' in output, got: %s", out)
		}
	})

	// Test list command
	var mockID string
	t.Run("list", func(t *testing.T) {
		listCmd := exec.Command("./mockd_test", "list", "--json", "--admin-url", adminURL)
		listCmd.Dir = "../.."
		out, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("List command failed: %v\n%s", err, out)
		}

		var mocks []struct {
			ID   string `json:"id"`
			Path string `json:"path"`
		}
		if err := json.Unmarshal(out, &mocks); err != nil {
			t.Fatalf("Failed to parse list output: %v\n%s", err, out)
		}
		if len(mocks) == 0 {
			t.Fatal("Expected at least one mock in list")
		}
		mockID = mocks[0].ID
	})

	// Test get command
	t.Run("get", func(t *testing.T) {
		if mockID == "" {
			t.Skip("No mock ID from list test")
		}
		getCmd := exec.Command("./mockd_test", "get", mockID, "--admin-url", adminURL)
		getCmd.Dir = "../.."
		out, err := getCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Get command failed: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "/api/test") {
			t.Fatalf("Expected '/api/test' in output, got: %s", out)
		}
	})

	// Test delete command
	t.Run("delete", func(t *testing.T) {
		if mockID == "" {
			t.Skip("No mock ID from list test")
		}
		delCmd := exec.Command("./mockd_test", "delete", mockID, "--admin-url", adminURL)
		delCmd.Dir = "../.."
		out, err := delCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Delete command failed: %v\n%s", err, out)
		}
		if !strings.Contains(string(out), "Deleted mock:") {
			t.Fatalf("Expected 'Deleted mock:' in output, got: %s", out)
		}
	})

	// Verify mock was deleted
	t.Run("verify_deleted", func(t *testing.T) {
		if mockID == "" {
			t.Skip("No mock ID from list test")
		}

		listCmd := exec.Command("./mockd_test", "list", "--json", "--admin-url", adminURL)
		listCmd.Dir = "../.."
		out, err := listCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("List command failed: %v\n%s", err, out)
		}

		var mocks []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(out, &mocks); err != nil {
			t.Fatalf("Failed to parse list output: %v\n%s", err, out)
		}

		// Verify the specific mock we deleted is no longer in the list
		for _, m := range mocks {
			if m.ID == mockID {
				t.Fatalf("Mock %s should have been deleted but still exists", mockID)
			}
		}
	})
}

// TestCLIImportExportCommands tests import and export commands.
func TestCLIImportExportCommands(t *testing.T) {
	// Find available ports
	httpPort := findAvailablePort(t, 38080)
	adminPort := findAvailablePort(t, 39090)

	// Create isolated data directory for this test
	dataDir := t.TempDir()

	// Build the CLI binary
	buildCmd := exec.Command("go", "build", "-o", "mockd_test", "./cmd/mockd")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\n%s", err, out)
	}
	defer exec.Command("rm", "-f", "../../mockd_test").Run()

	// Start the server with isolated data directory
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serverCmd := exec.CommandContext(ctx, "./mockd_test", "start",
		"--port", fmt.Sprintf("%d", httpPort),
		"--admin-port", fmt.Sprintf("%d", adminPort),
		"--no-auth",
		"--data-dir", dataDir,
	)
	serverCmd.Dir = "../.."
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer serverCmd.Process.Kill()

	adminURL := fmt.Sprintf("http://localhost:%d", adminPort)
	if !waitForServer(adminURL+"/health", 10*time.Second) {
		t.Fatal("Server did not become ready in time")
	}

	// Add a mock first
	addCmd := exec.Command("./mockd_test", "add",
		"--path", "/api/export-test",
		"--status", "200",
		"--admin-url", adminURL,
	)
	addCmd.Dir = "../.."
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("Add command failed: %v\n%s", err, out)
	}

	// Test export command
	t.Run("export", func(t *testing.T) {
		// Export to JSON file (format inferred from extension)
		exportFile := filepath.Join(t.TempDir(), "export.json")
		exportCmd := exec.Command("./mockd_test", "export", "--admin-url", adminURL, "-o", exportFile)
		exportCmd.Dir = "../.."
		out, err := exportCmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Export command failed: %v\n%s", err, out)
		}

		// Read the exported file
		exportData, err := os.ReadFile(exportFile)
		if err != nil {
			t.Fatalf("Failed to read export file: %v", err)
		}

		var exported struct {
			Version   string        `json:"version"`
			Endpoints []interface{} `json:"endpoints"`
		}
		if err := json.Unmarshal(exportData, &exported); err != nil {
			t.Fatalf("Failed to parse export output: %v\n%s", err, exportData)
		}
		if len(exported.Endpoints) == 0 {
			t.Fatal("Expected at least one endpoint in export")
		}
	})
}

// TestCLIVersionCommand tests the version command.
func TestCLIVersionCommand(t *testing.T) {
	// Build the CLI binary
	buildCmd := exec.Command("go", "build", "-o", "mockd_test", "./cmd/mockd")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\n%s", err, out)
	}
	defer exec.Command("rm", "-f", "../../mockd_test").Run()

	t.Run("text_output", func(t *testing.T) {
		cmd := exec.Command("./mockd_test", "version")
		cmd.Dir = "../.."
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Version command failed: %v\n%s", err, out)
		}
		// Output format is "mockd v{version} ({commit}, {date})"
		if !strings.HasPrefix(string(out), "mockd v") {
			t.Fatalf("Expected 'mockd v' prefix, got: %s", out)
		}
	})

	t.Run("json_output", func(t *testing.T) {
		cmd := exec.Command("./mockd_test", "version", "--json")
		cmd.Dir = "../.."
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Version --json command failed: %v\n%s", err, out)
		}

		var version struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(out, &version); err != nil {
			t.Fatalf("Failed to parse version JSON: %v\n%s", err, out)
		}
		if version.Version == "" {
			t.Fatal("Expected non-empty version field")
		}
	})
}

// TestCLIInitCommand tests the mockd init command.
func TestCLIInitCommand(t *testing.T) {
	// Build the CLI binary
	buildCmd := exec.Command("go", "build", "-o", "mockd_test", "./cmd/mockd")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\n%s", err, out)
	}
	defer exec.Command("rm", "-f", "../../mockd_test").Run()

	t.Run("create_default_yaml", func(t *testing.T) {
		outputFile := filepath.Join(t.TempDir(), "mockd.yaml")

		cmd := exec.Command("./mockd_test", "init", "-o", outputFile)
		cmd.Dir = "../.."
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Init command failed: %v\n%s", err, out)
		}

		// Check success message
		if !strings.Contains(string(out), "Created") {
			t.Errorf("Expected 'Created' in output, got: %s", out)
		}
		if !strings.Contains(string(out), "Next steps:") {
			t.Errorf("Expected 'Next steps:' in output, got: %s", out)
		}

		// Check file exists and contains expected content
		data, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		content := string(data)
		if !strings.Contains(content, "# mockd.yaml") {
			t.Error("Expected YAML header comment")
		}
		if !strings.Contains(content, "Hello World") {
			t.Error("Expected Hello World mock")
		}
		if !strings.Contains(content, "/hello") {
			t.Error("Expected /hello path")
		}
		if !strings.Contains(content, "version:") {
			t.Error("Expected version field")
		}
		if !strings.Contains(content, "type: http") {
			t.Error("Expected type: http in mocks")
		}
	})

	t.Run("create_json_format", func(t *testing.T) {
		outputFile := filepath.Join(t.TempDir(), "mocks.json")

		cmd := exec.Command("./mockd_test", "init", "--format", "json", "-o", outputFile)
		cmd.Dir = "../.."
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Init command failed: %v\n%s", err, out)
		}

		// Check file exists and is valid JSON
		data, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("Invalid JSON output: %v", err)
		}

		// Check expected structure - MockCollection format
		if _, ok := config["version"]; !ok {
			t.Error("Expected 'version' key in JSON")
		}
		if _, ok := config["mocks"]; !ok {
			t.Error("Expected 'mocks' key in JSON")
		}
	})

	t.Run("error_file_exists", func(t *testing.T) {
		outputFile := filepath.Join(t.TempDir(), "mockd.yaml")

		// Create existing file
		if err := os.WriteFile(outputFile, []byte("existing"), 0644); err != nil {
			t.Fatalf("Failed to create existing file: %v", err)
		}

		cmd := exec.Command("./mockd_test", "init", "-o", outputFile)
		cmd.Dir = "../.."
		out, err := cmd.CombinedOutput()

		// Should fail
		if err == nil {
			t.Fatal("Expected init to fail when file exists")
		}
		if !strings.Contains(string(out), "file already exists") {
			t.Errorf("Expected 'file already exists' error, got: %s", out)
		}
	})

	t.Run("force_overwrite", func(t *testing.T) {
		outputFile := filepath.Join(t.TempDir(), "mockd.yaml")

		// Create existing file
		if err := os.WriteFile(outputFile, []byte("existing"), 0644); err != nil {
			t.Fatalf("Failed to create existing file: %v", err)
		}

		cmd := exec.Command("./mockd_test", "init", "--force", "-o", outputFile)
		cmd.Dir = "../.."
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Init --force command failed: %v\n%s", err, out)
		}

		// Check file was overwritten
		data, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}
		if !strings.Contains(string(data), "Hello World") {
			t.Error("Expected file to be overwritten with new content")
		}
	})

	t.Run("infer_json_from_extension", func(t *testing.T) {
		outputFile := filepath.Join(t.TempDir(), "config.json")

		cmd := exec.Command("./mockd_test", "init", "-o", outputFile)
		cmd.Dir = "../.."
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Init command failed: %v\n%s", err, out)
		}

		// Check file is valid JSON
		data, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("Failed to read output file: %v", err)
		}

		var config map[string]interface{}
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("Expected JSON format inferred from .json extension: %v", err)
		}
	})
}

// TestCLIHelpOutput tests that all commands have help.
func TestCLIHelpOutput(t *testing.T) {
	// Build the CLI binary
	buildCmd := exec.Command("go", "build", "-o", "mockd_test", "./cmd/mockd")
	buildCmd.Dir = "../.."
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build CLI: %v\n%s", err, out)
	}
	defer exec.Command("rm", "-f", "../../mockd_test").Run()

	commands := []string{"init", "start", "add", "list", "get", "delete", "import", "export", "logs", "config", "completion", "version"}

	for _, cmd := range commands {
		t.Run(cmd, func(t *testing.T) {
			helpCmd := exec.Command("./mockd_test", cmd, "--help")
			helpCmd.Dir = "../.."
			out, _ := helpCmd.CombinedOutput()
			// --help returns exit code 1 with flag package, so we don't check error
			if !strings.Contains(string(out), "Usage:") {
				t.Errorf("Expected 'Usage:' in help output for %s, got: %s", cmd, out)
			}
		})
	}
}

// findAvailablePort finds an available TCP port starting from the given port.
func findAvailablePort(t *testing.T, start int) int {
	for port := start; port < start+100; port++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d", port))
		if err != nil {
			// Port is likely available
			return port
		}
		resp.Body.Close()
	}
	t.Fatalf("Could not find available port starting from %d", start)
	return 0
}

// waitForServer waits for a server to become available.
func waitForServer(url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
