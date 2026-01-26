package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
)

func TestRunPs_HelpFlag(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RunPs panicked with --help: %v", r)
		}
	}()

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	err := RunPs([]string{"--help"})

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Errorf("RunPs --help returned error: %v", err)
	}
}

func TestRunPs_NoPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunPs([]string{"--pid-file", pidPath})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("RunPs with no PID file returned error: %v", err)
	}

	// Read output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if output != "No running mockd services.\n" {
		t.Errorf("unexpected output: %q", output)
	}
}

func TestRunPs_NoPIDFile_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunPs([]string{"--pid-file", pidPath, "--json"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("RunPs --json with no PID file returned error: %v", err)
	}

	// Read output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	expected := `{"running":false,"services":[]}`
	if output != expected+"\n" {
		t.Errorf("output = %q, want %q", output, expected)
	}
}

func TestRunPs_WithPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Create PID file with current process (so it shows as running)
	pidInfo := &config.PIDFile{
		PID:       os.Getpid(),
		StartedAt: "2024-01-01T00:00:00Z",
		Config:    "/test/mockd.yaml",
		Services: []config.PIDFileService{
			{Name: "local", Type: "admin", Port: 4290, PID: os.Getpid()},
			{Name: "default", Type: "engine", Port: 4280, PID: os.Getpid()},
		},
	}

	if err := writeUpPIDFile(pidPath, pidInfo); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunPs([]string{"--pid-file", pidPath})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("RunPs returned error: %v", err)
	}

	// Read output
	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify output contains expected elements
	if !contains(output, "running") {
		t.Errorf("output should contain 'running': %s", output)
	}
	if !contains(output, "local") {
		t.Errorf("output should contain 'local': %s", output)
	}
	if !contains(output, "default") {
		t.Errorf("output should contain 'default': %s", output)
	}
	if !contains(output, "4290") {
		t.Errorf("output should contain '4290': %s", output)
	}
	if !contains(output, "4280") {
		t.Errorf("output should contain '4280': %s", output)
	}
}

func TestRunPs_WithPIDFile_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	pidInfo := &config.PIDFile{
		PID:       os.Getpid(),
		StartedAt: "2024-01-01T00:00:00Z",
		Config:    "/test/mockd.yaml",
		Services: []config.PIDFileService{
			{Name: "local", Type: "admin", Port: 4290, PID: os.Getpid()},
		},
	}

	if err := writeUpPIDFile(pidPath, pidInfo); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunPs([]string{"--pid-file", pidPath, "--json"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("RunPs --json returned error: %v", err)
	}

	// Read output
	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify JSON structure (with spaces since we use SetIndent)
	if !contains(output, `"running": true`) {
		t.Errorf("output should contain 'running': true: %s", output)
	}
	if !contains(output, `"name": "local"`) {
		t.Errorf("output should contain 'name': 'local': %s", output)
	}
}

func TestRunPs_StalePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "stale.pid")

	// Create PID file with non-existent PID
	pidInfo := &config.PIDFile{
		PID:       9999999,
		StartedAt: "2024-01-01T00:00:00Z",
		Config:    "/test/mockd.yaml",
		Services: []config.PIDFileService{
			{Name: "local", Type: "admin", Port: 4290, PID: 9999999},
		},
	}

	if err := writeUpPIDFile(pidPath, pidInfo); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunPs([]string{"--pid-file", pidPath})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("RunPs returned error: %v", err)
	}

	// Read output
	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Should show as stopped
	if !contains(output, "stopped") {
		t.Errorf("output should contain 'stopped' for stale PID: %s", output)
	}
}

func TestPrintPsTable(t *testing.T) {
	pidInfo := &config.PIDFile{
		PID:       12345,
		StartedAt: "2024-01-01T00:00:00Z",
		Config:    "/test/mockd.yaml",
		Services: []config.PIDFileService{
			{Name: "local", Type: "admin", Port: 4290, PID: 12345},
			{Name: "default", Type: "engine", Port: 4280, PID: 12345},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := printPsTable(pidInfo, false)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("printPsTable returned error: %v", err)
	}

	// Read output
	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify headers
	if !contains(output, "NAME") {
		t.Errorf("output should contain NAME header: %s", output)
	}
	if !contains(output, "TYPE") {
		t.Errorf("output should contain TYPE header: %s", output)
	}
	if !contains(output, "PORT") {
		t.Errorf("output should contain PORT header: %s", output)
	}
	if !contains(output, "STATUS") {
		t.Errorf("output should contain STATUS header: %s", output)
	}
}

func TestPrintPsJSON(t *testing.T) {
	pidInfo := &config.PIDFile{
		PID:       12345,
		StartedAt: "2024-01-01T00:00:00Z",
		Config:    "/test/mockd.yaml",
		Services: []config.PIDFileService{
			{Name: "local", Type: "admin", Port: 4290, PID: 12345},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := printPsJSON(pidInfo, true)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("printPsJSON returned error: %v", err)
	}

	// Read output
	buf := make([]byte, 2048)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify JSON structure
	if !contains(output, `"running":`) {
		t.Errorf("output should contain 'running' field: %s", output)
	}
	if !contains(output, `"services":`) {
		t.Errorf("output should contain 'services' field: %s", output)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
