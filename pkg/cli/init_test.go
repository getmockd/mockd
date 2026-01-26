package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"gopkg.in/yaml.v3"
)

func TestRunInit_Defaults(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mockd.yaml")

	// Change to temp directory for the test
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Run init with --defaults
	var stdout, stderr bytes.Buffer
	err := runInitWithIO([]string{"--defaults", "-o", outputPath}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Read and parse the config
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var cfg config.ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Verify defaults
	if cfg.Version != "1" {
		t.Errorf("Expected version '1', got '%s'", cfg.Version)
	}
	if len(cfg.Admins) != 1 {
		t.Errorf("Expected 1 admin, got %d", len(cfg.Admins))
	}
	if cfg.Admins[0].Port != 4290 {
		t.Errorf("Expected admin port 4290, got %d", cfg.Admins[0].Port)
	}
	if len(cfg.Engines) != 1 {
		t.Errorf("Expected 1 engine, got %d", len(cfg.Engines))
	}
	if cfg.Engines[0].HTTPPort != 4280 {
		t.Errorf("Expected engine HTTP port 4280, got %d", cfg.Engines[0].HTTPPort)
	}
	if len(cfg.Mocks) != 1 {
		t.Errorf("Expected 1 mock, got %d", len(cfg.Mocks))
	}
	if cfg.Mocks[0].ID != "health" {
		t.Errorf("Expected mock ID 'health', got '%s'", cfg.Mocks[0].ID)
	}
}

func TestRunInit_TemplateMinimal(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mockd.yaml")

	var stdout, stderr bytes.Buffer
	err := runInitWithIO([]string{"--template", "minimal", "-o", outputPath}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var cfg config.ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Verify minimal template structure
	if cfg.Version != "1" {
		t.Errorf("Expected version '1', got '%s'", cfg.Version)
	}
	if len(cfg.Admins) != 1 {
		t.Errorf("Expected 1 admin, got %d", len(cfg.Admins))
	}
	if len(cfg.Engines) != 1 {
		t.Errorf("Expected 1 engine, got %d", len(cfg.Engines))
	}
	if len(cfg.Workspaces) != 1 {
		t.Errorf("Expected 1 workspace, got %d", len(cfg.Workspaces))
	}
	if len(cfg.Mocks) != 1 {
		t.Errorf("Expected 1 mock (health), got %d", len(cfg.Mocks))
	}
}

func TestRunInit_TemplateFull(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mockd.yaml")

	var stdout, stderr bytes.Buffer
	err := runInitWithIO([]string{"--template", "full", "-o", outputPath}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var cfg config.ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Verify full template has more mocks
	if len(cfg.Mocks) < 3 {
		t.Errorf("Expected at least 3 mocks in full template, got %d", len(cfg.Mocks))
	}

	// Check for expected mocks
	mockIDs := make(map[string]bool)
	for _, m := range cfg.Mocks {
		mockIDs[m.ID] = true
	}
	expectedMocks := []string{"health", "hello", "echo"}
	for _, id := range expectedMocks {
		if !mockIDs[id] {
			t.Errorf("Expected mock '%s' not found in full template", id)
		}
	}
}

func TestRunInit_TemplateAPI(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mockd.yaml")

	var stdout, stderr bytes.Buffer
	err := runInitWithIO([]string{"--template", "api", "-o", outputPath}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var cfg config.ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Verify API template has CRUD mocks
	mockIDs := make(map[string]bool)
	for _, m := range cfg.Mocks {
		mockIDs[m.ID] = true
	}

	expectedMocks := []string{"health", "users-list", "users-get", "users-create", "users-update", "users-delete"}
	for _, id := range expectedMocks {
		if !mockIDs[id] {
			t.Errorf("Expected mock '%s' not found in API template", id)
		}
	}
}

func TestRunInit_InvalidTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mockd.yaml")

	var stdout, stderr bytes.Buffer
	err := runInitWithIO([]string{"--template", "nonexistent", "-o", outputPath}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Expected error for invalid template")
	}
	if !strings.Contains(err.Error(), "unknown template") {
		t.Errorf("Expected 'unknown template' error, got: %v", err)
	}
}

func TestRunInit_FileExists(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mockd.yaml")

	// Create existing file
	if err := os.WriteFile(outputPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runInitWithIO([]string{"--defaults", "-o", outputPath}, strings.NewReader(""), &stdout, &stderr)
	if err == nil {
		t.Fatal("Expected error when file exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got: %v", err)
	}
}

func TestRunInit_ForceOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mockd.yaml")

	// Create existing file
	if err := os.WriteFile(outputPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	var stdout, stderr bytes.Buffer
	err := runInitWithIO([]string{"--defaults", "--force", "-o", outputPath}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunInit with --force failed: %v", err)
	}

	// Verify file was overwritten
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}
	if string(data) == "existing" {
		t.Error("File was not overwritten")
	}
}

func TestRunInit_JSONOutput(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mockd.json")

	var stdout, stderr bytes.Buffer
	err := runInitWithIO([]string{"--defaults", "-o", outputPath}, strings.NewReader(""), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Verify it's valid JSON
	var cfg config.ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse JSON config: %v", err)
	}

	// Check it looks like JSON (starts with {)
	if !strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
		t.Error("Output doesn't look like JSON")
	}
}

func TestRunInit_Interactive(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mockd.yaml")

	// Simulate interactive input: custom ports, no HTTPS, api-key auth
	input := "4291\n4281\nn\napi-key\n"

	var stdout, stderr bytes.Buffer
	err := runInitWithIO([]string{"-o", outputPath}, strings.NewReader(input), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var cfg config.ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Verify custom values
	if cfg.Admins[0].Port != 4291 {
		t.Errorf("Expected admin port 4291, got %d", cfg.Admins[0].Port)
	}
	if cfg.Engines[0].HTTPPort != 4281 {
		t.Errorf("Expected engine HTTP port 4281, got %d", cfg.Engines[0].HTTPPort)
	}
	if cfg.Admins[0].Auth.Type != "api-key" {
		t.Errorf("Expected auth type 'api-key', got '%s'", cfg.Admins[0].Auth.Type)
	}
}

func TestRunInit_InteractiveWithHTTPS(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mockd.yaml")

	// Simulate interactive input with HTTPS enabled
	input := "4290\n4280\ny\n4443\nnone\n"

	var stdout, stderr bytes.Buffer
	err := runInitWithIO([]string{"-o", outputPath}, strings.NewReader(input), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var cfg config.ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Verify HTTPS is enabled
	if cfg.Engines[0].HTTPSPort != 4443 {
		t.Errorf("Expected HTTPS port 4443, got %d", cfg.Engines[0].HTTPSPort)
	}
	if cfg.Engines[0].TLS == nil {
		t.Error("Expected TLS config to be set")
	} else if !cfg.Engines[0].TLS.AutoGenerateCert {
		t.Error("Expected AutoGenerateCert to be true")
	}
}

func TestRunInit_InteractiveDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "mockd.yaml")

	// Simulate interactive input with all defaults (just pressing enter)
	input := "\n\n\n\n"

	var stdout, stderr bytes.Buffer
	err := runInitWithIO([]string{"-o", outputPath}, strings.NewReader(input), &stdout, &stderr)
	if err != nil {
		t.Fatalf("RunInit failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var cfg config.ProjectConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Verify defaults
	if cfg.Admins[0].Port != 4290 {
		t.Errorf("Expected default admin port 4290, got %d", cfg.Admins[0].Port)
	}
	if cfg.Engines[0].HTTPPort != 4280 {
		t.Errorf("Expected default engine HTTP port 4280, got %d", cfg.Engines[0].HTTPPort)
	}
	if cfg.Admins[0].Auth.Type != "none" {
		t.Errorf("Expected default auth type 'none', got '%s'", cfg.Admins[0].Auth.Type)
	}
}

func TestGenerateMinimalProjectConfig(t *testing.T) {
	cfg := defaultInitConfig()
	projectCfg := generateMinimalProjectConfig(cfg)

	if projectCfg.Version != "1" {
		t.Errorf("Expected version '1', got '%s'", projectCfg.Version)
	}
	if len(projectCfg.Admins) != 1 {
		t.Errorf("Expected 1 admin, got %d", len(projectCfg.Admins))
	}
	if projectCfg.Admins[0].Name != "local" {
		t.Errorf("Expected admin name 'local', got '%s'", projectCfg.Admins[0].Name)
	}
	if len(projectCfg.Engines) != 1 {
		t.Errorf("Expected 1 engine, got %d", len(projectCfg.Engines))
	}
	if projectCfg.Engines[0].Name != "default" {
		t.Errorf("Expected engine name 'default', got '%s'", projectCfg.Engines[0].Name)
	}
	if len(projectCfg.Workspaces) != 1 {
		t.Errorf("Expected 1 workspace, got %d", len(projectCfg.Workspaces))
	}
	if len(projectCfg.Mocks) != 1 {
		t.Errorf("Expected 1 mock, got %d", len(projectCfg.Mocks))
	}
}

func TestGetProjectConfigTemplate_InvalidTemplate(t *testing.T) {
	_, err := getProjectConfigTemplate("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent template")
	}
}

func TestGetProjectConfigTemplate_AllTemplates(t *testing.T) {
	templates := []string{"minimal", "full", "api"}
	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			cfg, err := getProjectConfigTemplate(tmpl)
			if err != nil {
				t.Fatalf("Failed to get template %s: %v", tmpl, err)
			}
			if cfg.Version != "1" {
				t.Errorf("Expected version '1' for template %s, got '%s'", tmpl, cfg.Version)
			}
			if len(cfg.Admins) == 0 {
				t.Errorf("Expected at least 1 admin for template %s", tmpl)
			}
			if len(cfg.Engines) == 0 {
				t.Errorf("Expected at least 1 engine for template %s", tmpl)
			}
		})
	}
}
