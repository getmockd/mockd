package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
)

func TestRunDown_HelpFlag(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RunDown panicked with --help: %v", r)
		}
	}()

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	err := RunDown([]string{"--help"})

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Errorf("RunDown --help returned error: %v", err)
	}
}

func TestRunDown_NoPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nonexistent.pid")

	// Capture stdout
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := RunDown([]string{"--pid-file", pidPath})

	w.Close()
	os.Stdout = oldStdout

	// Should not return error, just print "no running services"
	if err != nil {
		t.Errorf("RunDown with no PID file returned error: %v", err)
	}
}

func TestRunDown_StalePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "stale.pid")

	// Create PID file with non-existent PID
	pidInfo := &config.PIDFile{
		PID:       9999999, // Very high PID unlikely to exist
		StartedAt: "2024-01-01T00:00:00Z",
		Config:    "/test/mockd.yaml",
		Services:  []config.PIDFileService{},
	}

	if err := writeUpPIDFile(pidPath, pidInfo); err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	err := RunDown([]string{"--pid-file", pidPath})

	w.Close()
	os.Stdout = oldStdout

	// Should not return error for stale PID
	if err != nil {
		t.Errorf("RunDown with stale PID returned error: %v", err)
	}

	// PID file should be cleaned up
	if _, statErr := os.Stat(pidPath); !os.IsNotExist(statErr) {
		t.Error("stale PID file should be removed")
	}
}

func TestReadUpPIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Write a valid PID file
	pidInfo := &config.PIDFile{
		PID:       12345,
		StartedAt: "2024-01-01T00:00:00Z",
		Config:    "/test/mockd.yaml",
		Services: []config.PIDFileService{
			{Name: "local", Type: "admin", Port: 4290, PID: 12345},
			{Name: "default", Type: "engine", Port: 4280, PID: 12345},
		},
	}

	if err := writeUpPIDFile(pidPath, pidInfo); err != nil {
		t.Fatalf("writeUpPIDFile failed: %v", err)
	}

	// Read it back
	readInfo, err := readUpPIDFile(pidPath)
	if err != nil {
		t.Fatalf("readUpPIDFile failed: %v", err)
	}

	if readInfo.PID != 12345 {
		t.Errorf("PID = %d, want 12345", readInfo.PID)
	}

	if readInfo.Config != "/test/mockd.yaml" {
		t.Errorf("Config = %q, want %q", readInfo.Config, "/test/mockd.yaml")
	}

	if len(readInfo.Services) != 2 {
		t.Fatalf("len(Services) = %d, want 2", len(readInfo.Services))
	}
}

func TestReadUpPIDFile_NotFound(t *testing.T) {
	_, err := readUpPIDFile("/nonexistent/path/mockd.pid")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected IsNotExist error, got: %v", err)
	}
}

func TestReadUpPIDFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "invalid.pid")

	// Write invalid JSON
	os.WriteFile(pidPath, []byte("not valid json"), 0644)

	_, err := readUpPIDFile(pidPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestWriteUpPIDFile_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "nested", "dir", "mockd.pid")

	pidInfo := &config.PIDFile{
		PID:       12345,
		StartedAt: "2024-01-01T00:00:00Z",
		Config:    "/test/mockd.yaml",
		Services:  []config.PIDFileService{},
	}

	err := writeUpPIDFile(pidPath, pidInfo)
	if err != nil {
		t.Fatalf("writeUpPIDFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Error("PID file was not created")
	}
}

func TestProcessExists(t *testing.T) {
	// Current process should exist
	if !processExists(os.Getpid()) {
		t.Error("current process should exist")
	}

	// Very high PID unlikely to exist
	if processExists(9999999) {
		t.Error("PID 9999999 should not exist")
	}

	// PID 0 should not exist (or at least not be killable)
	// Note: On some systems PID 0 is the scheduler
	// We check signal 0 which requires permission
}

func TestDefaultUpPIDPath(t *testing.T) {
	path := defaultUpPIDPath()
	if path == "" {
		t.Error("defaultUpPIDPath returned empty string")
	}

	// Should contain .mockd
	if filepath.Base(filepath.Dir(path)) != ".mockd" {
		// Unless home dir failed, then it's /tmp
		if filepath.Dir(path) != "/tmp" {
			t.Errorf("unexpected path: %s", path)
		}
	}
}
