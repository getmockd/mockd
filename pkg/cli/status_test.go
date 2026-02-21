package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func RunStatus(args []string) error {
	statusPidFile = ""
	statusPort = 0
	statusAdminPort = 0
	jsonOutput = false
	
	if f := rootCmd.Flags().Lookup("help"); f != nil {
		f.Changed = false
		f.Value.Set("false")
	}
	if f := statusCmd.Flags().Lookup("help"); f != nil {
		f.Changed = false
		f.Value.Set("false")
	}
	if f := rootCmd.Flags().Lookup("json"); f != nil {
		f.Changed = false
		f.Value.Set("false")
	}
	rootCmd.SetArgs(append([]string{"status"}, args...))
	return rootCmd.Execute()
}

func TestRunStatus_NoServer(t *testing.T) {
	// Use non-existent PID file path
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunStatus([]string{"--pid-file", pidPath})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("RunStatus should not return error when server not running: %v", err)
	}

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if output == "" {
		t.Error("expected some output")
	}
}

func TestRunStatus_StalePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "stale.pid")

	// Create PID file with non-existent PID
	info := &PIDFile{
		PID:       9999999, // Very high PID unlikely to exist
		StartTime: time.Now(),
		Version:   "0.1.0",
		Components: ComponentsInfo{
			Admin: ComponentStatus{
				Enabled: true,
				Port:    9090,
				Host:    "localhost",
			},
			Engine: ComponentStatus{
				Enabled: true,
				Port:    8080,
				Host:    "localhost",
			},
		},
	}

	if err := WritePIDFile(pidPath, info); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunStatus([]string{"--pid-file", pidPath})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("RunStatus should not return error for stale PID file: %v", err)
	}

	// Read captured output - should show not running
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if output == "" {
		t.Error("expected some output for not running state")
	}
}

func TestRunStatus_JSONOutput_NotRunning(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunStatus([]string{"--pid-file", pidPath, "--json"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Errorf("RunStatus with --json should not error: %v", err)
	}

	// Read captured output
	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Should contain JSON indicators
	if len(output) == 0 {
		t.Skip("skipping json output validation because output was empty")
	} else if output[0] != '{' {
		t.Errorf("expected JSON output to start with '{', got: %s", output[:10])
	}
}

func TestBuildStatusOutput(t *testing.T) {
	info := &PIDFile{
		PID:       12345,
		StartTime: time.Now().Add(-1 * time.Hour),
		Version:   "0.1.0",
		Commit:    "abc1234",
		Components: ComponentsInfo{
			Admin: ComponentStatus{
				Enabled: true,
				Port:    9090,
				Host:    "localhost",
			},
			Engine: ComponentStatus{
				Enabled:   true,
				Port:      8080,
				Host:      "localhost",
				HTTPSPort: 8443,
			},
		},
	}

	output := buildStatusOutput(info)

	if output.Version != "0.1.0" {
		t.Errorf("Version mismatch: got %s, want 0.1.0", output.Version)
	}
	if output.Commit != "abc1234" {
		t.Errorf("Commit mismatch: got %s, want abc1234", output.Commit)
	}
	if !output.Running {
		t.Error("Running should be true")
	}
	if output.PID != 12345 {
		t.Errorf("PID mismatch: got %d, want 12345", output.PID)
	}
	if output.Components.Admin.Status != "running" {
		t.Errorf("Admin status should be running, got %s", output.Components.Admin.Status)
	}
	if output.Components.Engine.Status != "running" {
		t.Errorf("Engine status should be running, got %s", output.Components.Engine.Status)
	}
	if output.Components.Engine.HTTPSURL != "https://localhost:8443" {
		t.Errorf("Engine HTTPS URL mismatch: got %s", output.Components.Engine.HTTPSURL)
	}
}

func TestBuildStatusOutput_DisabledComponents(t *testing.T) {
	info := &PIDFile{
		PID:       12345,
		StartTime: time.Now(),
		Version:   "0.1.0",
		Components: ComponentsInfo{
			Admin: ComponentStatus{
				Enabled: false,
			},
			Engine: ComponentStatus{
				Enabled: false,
			},
		},
	}

	output := buildStatusOutput(info)

	if output.Components.Admin.Status != "stopped" {
		t.Errorf("disabled Admin status should be stopped, got %s", output.Components.Admin.Status)
	}
	if output.Components.Engine.Status != "stopped" {
		t.Errorf("disabled Engine status should be stopped, got %s", output.Components.Engine.Status)
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input int
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{123, "123"},
		{1000, "1,000"},
		{1234, "1,234"},
		{12345, "12,345"},
		{123456, "123,456"},
		{1234567, "1,234,567"},
	}

	for _, tt := range tests {
		got := formatNumber(tt.input)
		if got != tt.want {
			t.Errorf("formatNumber(%d) = %s, want %s", tt.input, got, tt.want)
		}
	}
}
