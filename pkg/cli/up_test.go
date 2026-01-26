package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
)

func TestRunUp_HelpFlag(t *testing.T) {
	// --help should not panic or return error
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RunUp panicked with --help: %v", r)
		}
	}()

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	err := RunUp([]string{"--help"})

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Errorf("RunUp --help returned error: %v", err)
	}
}

func TestRunUp_NoConfig(t *testing.T) {
	// Create a temp directory with no config file
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	err := RunUp([]string{})
	if err == nil {
		t.Error("expected error when no config file exists")
	}
}

func TestRunUp_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	// Write invalid config (missing required fields)
	invalidConfig := `
version: "1"
admins:
  - name: ""
`
	os.WriteFile(configPath, []byte(invalidConfig), 0644)

	err := RunUp([]string{"-f", configPath})
	if err == nil {
		t.Error("expected error for invalid config")
	}
}

func TestRunUp_PortConflict(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	// Config with port conflict
	conflictConfig := `
version: "1"
admins:
  - name: local
    port: 4280
engines:
  - name: default
    httpPort: 4280
    admin: local
`
	os.WriteFile(configPath, []byte(conflictConfig), 0644)

	err := RunUp([]string{"-f", configPath})
	if err == nil {
		t.Error("expected error for port conflict")
	}
}

func TestLoadProjectConfig_SingleFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	validConfig := `
version: "1"
admins:
  - name: local
    port: 4290
engines:
  - name: default
    httpPort: 4280
    admin: local
`
	os.WriteFile(configPath, []byte(validConfig), 0644)

	cfg, path, err := loadProjectConfig([]string{configPath})
	if err != nil {
		t.Fatalf("loadProjectConfig failed: %v", err)
	}

	if path != configPath {
		t.Errorf("path = %q, want %q", path, configPath)
	}

	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}

	if len(cfg.Admins) != 1 {
		t.Fatalf("len(Admins) = %d, want 1", len(cfg.Admins))
	}

	if cfg.Admins[0].Name != "local" {
		t.Errorf("Admins[0].Name = %q, want %q", cfg.Admins[0].Name, "local")
	}
}

func TestLoadProjectConfig_MultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	basePath := filepath.Join(tmpDir, "base.yaml")
	baseConfig := `
version: "1"
admins:
  - name: local
    port: 4290
engines:
  - name: default
    httpPort: 4280
    admin: local
`
	os.WriteFile(basePath, []byte(baseConfig), 0644)

	overlayPath := filepath.Join(tmpDir, "overlay.yaml")
	overlayConfig := `
admins:
  - name: local
    port: 9090
`
	os.WriteFile(overlayPath, []byte(overlayConfig), 0644)

	cfg, _, err := loadProjectConfig([]string{basePath, overlayPath})
	if err != nil {
		t.Fatalf("loadProjectConfig failed: %v", err)
	}

	// Port should be overridden
	if cfg.Admins[0].Port != 9090 {
		t.Errorf("Admins[0].Port = %d, want 9090 (overridden)", cfg.Admins[0].Port)
	}
}

func TestLoadProjectConfig_Discovery(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	validConfig := `
version: "1"
admins:
  - name: local
    port: 4290
engines:
  - name: default
    httpPort: 4280
    admin: local
`
	os.WriteFile(configPath, []byte(validConfig), 0644)

	// Change to tmpDir so discovery finds the config
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	cfg, path, err := loadProjectConfig([]string{})
	if err != nil {
		t.Fatalf("loadProjectConfig discovery failed: %v", err)
	}

	if path != configPath {
		t.Errorf("discovered path = %q, want %q", path, configPath)
	}

	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}
}

func TestLoadProjectConfig_EnvVarDiscovery(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom.yaml")

	validConfig := `
version: "1"
admins:
  - name: local
    port: 4290
engines:
  - name: default
    httpPort: 4280
    admin: local
`
	os.WriteFile(configPath, []byte(validConfig), 0644)

	// Set MOCKD_CONFIG env var
	os.Setenv("MOCKD_CONFIG", configPath)
	defer os.Unsetenv("MOCKD_CONFIG")

	cfg, path, err := loadProjectConfig([]string{})
	if err != nil {
		t.Fatalf("loadProjectConfig with MOCKD_CONFIG failed: %v", err)
	}

	if path != configPath {
		t.Errorf("path = %q, want %q", path, configPath)
	}

	if cfg.Version != "1" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1")
	}
}

func TestCheckProjectPorts_Available(t *testing.T) {
	cfg := &config.ProjectConfig{
		Version: "1",
		Admins: []config.AdminConfig{
			{Name: "local", Port: 0}, // Port 0 means any available
		},
		Engines: []config.EngineConfig{
			{Name: "default", HTTPPort: 0, Admin: "local"},
		},
	}

	// Port 0 should always pass (OS assigns available port)
	err := checkProjectPorts(cfg)
	if err != nil {
		t.Errorf("checkProjectPorts failed for port 0: %v", err)
	}
}

func TestPrintUpSummary(t *testing.T) {
	cfg := &config.ProjectConfig{
		Version: "1",
		Admins: []config.AdminConfig{
			{Name: "local", Port: 4290},
			{Name: "remote", URL: "https://example.com"},
		},
		Engines: []config.EngineConfig{
			{Name: "default", HTTPPort: 4280, Admin: "local"},
		},
		Workspaces: []config.WorkspaceConfig{
			{Name: "default"},
		},
		Mocks: []config.MockEntry{
			{ID: "test", Type: "http"},
			{File: "./mocks.yaml"},
		},
	}

	// Just verify it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printUpSummary panicked: %v", r)
		}
	}()

	// Capture stdout
	oldStdout := os.Stdout
	_, w, _ := os.Pipe()
	os.Stdout = w

	printUpSummary(cfg)

	w.Close()
	os.Stdout = oldStdout
}

func TestUpContext_WritePIDFile(t *testing.T) {
	tmpDir := t.TempDir()
	pidPath := filepath.Join(tmpDir, ".mockd", "mockd.pid")

	cfg := &config.ProjectConfig{
		Version: "1",
		Admins: []config.AdminConfig{
			{Name: "local", Port: 4290},
		},
		Engines: []config.EngineConfig{
			{Name: "default", HTTPPort: 4280, Admin: "local"},
		},
	}

	uctx := &upContext{
		cfg:        cfg,
		configPath: "/test/mockd.yaml",
	}

	err := uctx.writePIDFile(pidPath)
	if err != nil {
		t.Fatalf("writePIDFile failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(pidPath); os.IsNotExist(err) {
		t.Error("PID file was not created")
	}

	// Read and verify contents
	pidInfo, err := readUpPIDFile(pidPath)
	if err != nil {
		t.Fatalf("readUpPIDFile failed: %v", err)
	}

	if pidInfo.PID != os.Getpid() {
		t.Errorf("PID = %d, want %d", pidInfo.PID, os.Getpid())
	}

	if pidInfo.Config != "/test/mockd.yaml" {
		t.Errorf("Config = %q, want %q", pidInfo.Config, "/test/mockd.yaml")
	}

	if len(pidInfo.Services) != 2 {
		t.Fatalf("len(Services) = %d, want 2", len(pidInfo.Services))
	}

	// Check admin service
	if pidInfo.Services[0].Name != "local" {
		t.Errorf("Services[0].Name = %q, want %q", pidInfo.Services[0].Name, "local")
	}
	if pidInfo.Services[0].Type != "admin" {
		t.Errorf("Services[0].Type = %q, want %q", pidInfo.Services[0].Type, "admin")
	}
	if pidInfo.Services[0].Port != 4290 {
		t.Errorf("Services[0].Port = %d, want %d", pidInfo.Services[0].Port, 4290)
	}

	// Check engine service
	if pidInfo.Services[1].Name != "default" {
		t.Errorf("Services[1].Name = %q, want %q", pidInfo.Services[1].Name, "default")
	}
	if pidInfo.Services[1].Type != "engine" {
		t.Errorf("Services[1].Type = %q, want %q", pidInfo.Services[1].Type, "engine")
	}
	if pidInfo.Services[1].Port != 4280 {
		t.Errorf("Services[1].Port = %d, want %d", pidInfo.Services[1].Port, 4280)
	}
}
