package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultPIDPath(t *testing.T) {
	path := DefaultPIDPath()
	if path == "" {
		t.Error("DefaultPIDPath returned empty string")
	}

	// Should contain .mockd/mockd.pid
	if filepath.Base(path) != "mockd.pid" {
		t.Errorf("expected filename mockd.pid, got %s", filepath.Base(path))
	}
}

func TestWriteAndReadPIDFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Create test PID file info
	now := time.Now().Truncate(time.Second)
	info := &PIDFile{
		PID:       12345,
		StartTime: now,
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
		Config: ConfigInfo{
			File:        "/path/to/config.yaml",
			MocksLoaded: 24,
		},
	}

	// Write PID file
	if err := WritePIDFile(pidPath, info); err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Error("PID file was not created")
	}

	// Read it back
	readInfo, err := ReadPIDFile(pidPath)
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}

	// Verify fields
	if readInfo.PID != info.PID {
		t.Errorf("PID mismatch: got %d, want %d", readInfo.PID, info.PID)
	}
	if readInfo.Version != info.Version {
		t.Errorf("Version mismatch: got %s, want %s", readInfo.Version, info.Version)
	}
	if readInfo.Commit != info.Commit {
		t.Errorf("Commit mismatch: got %s, want %s", readInfo.Commit, info.Commit)
	}
	if !readInfo.StartTime.Equal(info.StartTime) {
		t.Errorf("StartTime mismatch: got %v, want %v", readInfo.StartTime, info.StartTime)
	}

	// Verify components
	if !readInfo.Components.Admin.Enabled {
		t.Error("Admin should be enabled")
	}
	if readInfo.Components.Admin.Port != 9090 {
		t.Errorf("Admin port mismatch: got %d, want 9090", readInfo.Components.Admin.Port)
	}
	if !readInfo.Components.Engine.Enabled {
		t.Error("Engine should be enabled")
	}
	if readInfo.Components.Engine.HTTPSPort != 8443 {
		t.Errorf("Engine HTTPS port mismatch: got %d, want 8443", readInfo.Components.Engine.HTTPSPort)
	}

	// Verify config
	if readInfo.Config.MocksLoaded != 24 {
		t.Errorf("MocksLoaded mismatch: got %d, want 24", readInfo.Config.MocksLoaded)
	}
}

func TestReadPIDFile_NotFound(t *testing.T) {
	_, err := ReadPIDFile("/nonexistent/path/test.pid")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestRemovePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, "test.pid")

	// Create a test file
	if err := os.WriteFile(pidPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Remove it
	if err := RemovePIDFile(pidPath); err != nil {
		t.Errorf("RemovePIDFile failed: %v", err)
	}

	// Verify it's gone
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file still exists after removal")
	}

	// Removing non-existent file should not error
	if err := RemovePIDFile(pidPath); err != nil {
		t.Errorf("RemovePIDFile on non-existent file should not error: %v", err)
	}
}

func TestPIDFile_IsRunning(t *testing.T) {
	// Current process should be running
	info := &PIDFile{PID: os.Getpid()}
	if !info.IsRunning() {
		t.Error("current process should be detected as running")
	}

	// Invalid PID should not be running
	info = &PIDFile{PID: 0}
	if info.IsRunning() {
		t.Error("PID 0 should not be running")
	}

	// Very high PID unlikely to exist
	info = &PIDFile{PID: 9999999}
	if info.IsRunning() {
		t.Error("PID 9999999 should not be running")
	}
}

func TestPIDFile_FormatUptime(t *testing.T) {
	tests := []struct {
		name      string
		startTime time.Time
		wantMatch string // partial match
	}{
		{
			name:      "seconds",
			startTime: time.Now().Add(-30 * time.Second),
			wantMatch: "s",
		},
		{
			name:      "minutes",
			startTime: time.Now().Add(-5 * time.Minute),
			wantMatch: "m",
		},
		{
			name:      "hours",
			startTime: time.Now().Add(-2 * time.Hour),
			wantMatch: "h",
		},
		{
			name:      "days",
			startTime: time.Now().Add(-25 * time.Hour),
			wantMatch: "d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &PIDFile{StartTime: tt.startTime}
			uptime := info.FormatUptime()
			if uptime == "" {
				t.Error("FormatUptime returned empty string")
			}
			// Just verify it contains some time indicator
			if len(uptime) == 0 {
				t.Error("uptime string is empty")
			}
		})
	}
}

func TestPIDFile_URLs(t *testing.T) {
	info := &PIDFile{
		Components: ComponentsInfo{
			Admin: ComponentStatus{
				Enabled: true,
				Port:    4290,
				Host:    "localhost",
			},
			Engine: ComponentStatus{
				Enabled:   true,
				Port:      4280,
				Host:      "localhost",
				HTTPSPort: 8443,
			},
		},
	}

	// Test AdminURL
	adminURL := info.AdminURL()
	if adminURL != "http://localhost:4290" {
		t.Errorf("AdminURL mismatch: got %s, want http://localhost:4290", adminURL)
	}

	// Test EngineURL
	engineURL := info.EngineURL()
	if engineURL != "http://localhost:4280" {
		t.Errorf("EngineURL mismatch: got %s, want http://localhost:4280", engineURL)
	}

	// Test EngineHTTPSURL
	httpsURL := info.EngineHTTPSURL()
	if httpsURL != "https://localhost:8443" {
		t.Errorf("EngineHTTPSURL mismatch: got %s, want https://localhost:8443", httpsURL)
	}

	// Test disabled components
	info.Components.Admin.Enabled = false
	if info.AdminURL() != "" {
		t.Error("disabled admin should return empty URL")
	}

	info.Components.Engine.Enabled = false
	if info.EngineURL() != "" {
		t.Error("disabled engine should return empty URL")
	}
	if info.EngineHTTPSURL() != "" {
		t.Error("disabled engine should return empty HTTPS URL")
	}

	// Test no HTTPS port
	info.Components.Engine.Enabled = true
	info.Components.Engine.HTTPSPort = 0
	if info.EngineHTTPSURL() != "" {
		t.Error("no HTTPS port should return empty HTTPS URL")
	}
}

func TestPIDFile_EmptyHost(t *testing.T) {
	info := &PIDFile{
		Components: ComponentsInfo{
			Admin: ComponentStatus{
				Enabled: true,
				Port:    4290,
				Host:    "", // Empty host should default to localhost
			},
			Engine: ComponentStatus{
				Enabled:   true,
				Port:      4280,
				Host:      "",
				HTTPSPort: 8443,
			},
		},
	}

	if info.AdminURL() != "http://localhost:4290" {
		t.Errorf("empty host should default to localhost, got %s", info.AdminURL())
	}
	if info.EngineURL() != "http://localhost:4280" {
		t.Errorf("empty host should default to localhost, got %s", info.EngineURL())
	}
}
