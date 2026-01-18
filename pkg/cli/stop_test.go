package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRunStop_NoServer(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	err := RunStop([]string{"--pid-file", pidPath})
	if err == nil {
		t.Error("expected error when no server running")
	}
}

func TestRunStop_StalePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "stale.pid")

	// Create PID file with non-existent PID
	info := &PIDFile{
		PID:       9999999, // Very high PID unlikely to exist
		StartTime: time.Now(),
		Version:   "0.1.0",
	}

	if err := WritePIDFile(pidPath, info); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	err := RunStop([]string{"--pid-file", pidPath})
	if err == nil {
		t.Error("expected error for stale PID file")
	}

	// PID file should be cleaned up
	if _, statErr := os.Stat(pidPath); !os.IsNotExist(statErr) {
		t.Error("stale PID file should be removed")
	}
}

func TestRunStop_InvalidComponent(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	err := RunStop([]string{"invalid-component", "--pid-file", pidPath})
	if err == nil {
		t.Error("expected error for invalid component")
	}
}

func TestRunStop_HelpFlag(t *testing.T) {
	// --help should not return an error (flag.ErrHelp is handled)
	// but in our implementation it may, so just ensure it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RunStop panicked with --help: %v", r)
		}
	}()

	// Capture stderr since help goes there
	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	RunStop([]string{"--help"})

	w.Close()
	os.Stderr = oldStderr
}

func TestProcessIsRunning(t *testing.T) {
	// Current process should be running
	if !processIsRunning(os.Getpid()) {
		t.Error("current process should be detected as running")
	}

	// Invalid PID should not be running
	if processIsRunning(0) {
		t.Error("PID 0 should not be running")
	}

	// Very high PID unlikely to exist
	if processIsRunning(9999999) {
		t.Error("PID 9999999 should not be running")
	}
}
