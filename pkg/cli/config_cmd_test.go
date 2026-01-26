package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
)

func TestRunConfigShow_HelpFlag(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := RunConfigShow([]string{"--help"})

	w.Close()
	os.Stderr = oldStderr

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// --help returns nil after printing usage
	if err != nil {
		t.Errorf("expected nil error for --help, got: %v", err)
	}

	// Check that help text was printed
	if !strings.Contains(output, "Display resolved configuration") {
		t.Errorf("expected help text to contain 'Display resolved configuration', got: %s", output)
	}
}

func TestRunConfigShow_WithConfigFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	configContent := `version: "1"
admins:
  - name: local
    port: 4290
engines:
  - name: default
    httpPort: 4280
    admin: local
workspaces:
  - name: default
    engines:
      - default
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunConfigShow([]string{"-f", configPath})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("RunConfigShow returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check output contains expected content
	if !strings.Contains(output, "version:") {
		t.Error("expected output to contain 'version:'")
	}
	if !strings.Contains(output, "admins:") {
		t.Error("expected output to contain 'admins:'")
	}
	if !strings.Contains(output, "name: local") {
		t.Error("expected output to contain 'name: local'")
	}
	if !strings.Contains(output, "port: 4290") {
		t.Error("expected output to contain 'port: 4290'")
	}
	if !strings.Contains(output, "# Resolved configuration from") {
		t.Error("expected output to contain header comment")
	}
}

func TestRunConfigShow_JSON(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	configContent := `version: "1"
admins:
  - name: local
    port: 4290
engines:
  - name: default
    httpPort: 4280
    admin: local
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunConfigShow([]string{"-f", configPath, "--json"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("RunConfigShow returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Parse JSON output
	var cfg config.ProjectConfig
	if err := json.Unmarshal([]byte(output), &cfg); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput was: %s", err, output)
	}

	// Verify parsed content
	if cfg.Version != "1" {
		t.Errorf("expected version '1', got '%s'", cfg.Version)
	}
	if len(cfg.Admins) != 1 {
		t.Fatalf("expected 1 admin, got %d", len(cfg.Admins))
	}
	if cfg.Admins[0].Name != "local" {
		t.Errorf("expected admin name 'local', got '%s'", cfg.Admins[0].Name)
	}
	if cfg.Admins[0].Port != 4290 {
		t.Errorf("expected admin port 4290, got %d", cfg.Admins[0].Port)
	}
}

func TestRunConfigShow_ServiceFilter_Admin(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	configContent := `version: "1"
admins:
  - name: local
    port: 4290
  - name: staging
    port: 4291
engines:
  - name: default
    httpPort: 4280
    admin: local
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunConfigShow([]string{"-f", configPath, "--service", "local"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("RunConfigShow returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check output contains the specific admin
	if !strings.Contains(output, "name: local") {
		t.Error("expected output to contain 'name: local'")
	}
	if !strings.Contains(output, "port: 4290") {
		t.Error("expected output to contain 'port: 4290'")
	}
	// Should NOT contain the other admin
	if strings.Contains(output, "staging") {
		t.Error("expected output to NOT contain 'staging'")
	}
	if !strings.Contains(output, "# Resolved admin configuration") {
		t.Error("expected output to contain admin type in header")
	}
}

func TestRunConfigShow_ServiceFilter_Engine(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	configContent := `version: "1"
admins:
  - name: local
    port: 4290
engines:
  - name: default
    httpPort: 4280
    admin: local
  - name: grpc-engine
    grpcPort: 4281
    admin: local
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunConfigShow([]string{"-f", configPath, "--service", "default"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("RunConfigShow returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check output contains the specific engine
	if !strings.Contains(output, "name: default") {
		t.Error("expected output to contain 'name: default'")
	}
	if !strings.Contains(output, "httpPort: 4280") {
		t.Error("expected output to contain 'httpPort: 4280'")
	}
	// Should NOT contain the other engine
	if strings.Contains(output, "grpc-engine") {
		t.Error("expected output to NOT contain 'grpc-engine'")
	}
	if !strings.Contains(output, "# Resolved engine configuration") {
		t.Error("expected output to contain engine type in header")
	}
}

func TestRunConfigShow_ServiceFilter_JSON(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	configContent := `version: "1"
admins:
  - name: local
    port: 4290
engines:
  - name: default
    httpPort: 4280
    admin: local
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunConfigShow([]string{"-f", configPath, "--service", "default", "--json"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("RunConfigShow returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Parse JSON output
	var engine config.EngineConfig
	if err := json.Unmarshal([]byte(output), &engine); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nOutput was: %s", err, output)
	}

	// Verify parsed content
	if engine.Name != "default" {
		t.Errorf("expected engine name 'default', got '%s'", engine.Name)
	}
	if engine.HTTPPort != 4280 {
		t.Errorf("expected httpPort 4280, got %d", engine.HTTPPort)
	}
	if engine.Admin != "local" {
		t.Errorf("expected admin 'local', got '%s'", engine.Admin)
	}
}

func TestRunConfigShow_ServiceNotFound(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	configContent := `version: "1"
admins:
  - name: local
    port: 4290
engines:
  - name: default
    httpPort: 4280
    admin: local
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	err := RunConfigShow([]string{"-f", configPath, "--service", "nonexistent"})

	if err == nil {
		t.Fatal("expected error for nonexistent service")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to contain 'not found', got: %v", err)
	}
}

func TestRunConfigShow_EnvVarExpansion(t *testing.T) {
	// Set an environment variable
	os.Setenv("TEST_MOCKD_PORT", "9999")
	defer os.Unsetenv("TEST_MOCKD_PORT")

	// Create a temporary config file with env var
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	configContent := `version: "1"
admins:
  - name: local
    port: ${TEST_MOCKD_PORT}
engines:
  - name: default
    httpPort: 4280
    admin: local
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunConfigShow([]string{"-f", configPath})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("RunConfigShow returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check that the env var was expanded
	if !strings.Contains(output, "port: 9999") {
		t.Errorf("expected output to contain expanded port 'port: 9999', got: %s", output)
	}
	// Should NOT contain the env var syntax
	if strings.Contains(output, "${TEST_MOCKD_PORT}") {
		t.Error("expected env var to be expanded, but found raw syntax")
	}
}

func TestRunConfigShow_EnvVarWithDefault(t *testing.T) {
	// Ensure the env var is NOT set
	os.Unsetenv("TEST_MOCKD_UNSET_PORT")

	// Create a temporary config file with env var with default
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	configContent := `version: "1"
admins:
  - name: local
    port: ${TEST_MOCKD_UNSET_PORT:-7777}
engines:
  - name: default
    httpPort: 4280
    admin: local
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunConfigShow([]string{"-f", configPath})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("RunConfigShow returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check that the default value was used
	if !strings.Contains(output, "port: 7777") {
		t.Errorf("expected output to contain default port 'port: 7777', got: %s", output)
	}
}

func TestRunConfigShow_NoConfigFile(t *testing.T) {
	// Create a temp dir without a config file and change to it
	tmpDir := t.TempDir()
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Unset MOCKD_CONFIG to ensure discovery is used
	oldEnv := os.Getenv("MOCKD_CONFIG")
	os.Unsetenv("MOCKD_CONFIG")
	defer func() {
		if oldEnv != "" {
			os.Setenv("MOCKD_CONFIG", oldEnv)
		}
	}()

	err := RunConfigShow([]string{})

	if err == nil {
		t.Fatal("expected error when no config file exists")
	}
	if !strings.Contains(err.Error(), "no config found") {
		t.Errorf("expected 'no config found' error, got: %v", err)
	}
}

func TestPrintFullConfig_YAML(t *testing.T) {
	cfg := &config.ProjectConfig{
		Version: "1",
		Admins: []config.AdminConfig{
			{Name: "local", Port: 4290},
		},
		Engines: []config.EngineConfig{
			{Name: "default", HTTPPort: 4280, Admin: "local"},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := printFullConfig(cfg, "./mockd.yaml", false)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("printFullConfig returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check YAML output
	if !strings.Contains(output, "version:") {
		t.Error("expected YAML output to contain 'version:'")
	}
	if !strings.Contains(output, "# Resolved configuration") {
		t.Error("expected header comment")
	}
}

func TestPrintFullConfig_JSON(t *testing.T) {
	cfg := &config.ProjectConfig{
		Version: "1",
		Admins: []config.AdminConfig{
			{Name: "local", Port: 4290},
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := printFullConfig(cfg, "./mockd.yaml", true)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("printFullConfig returned error: %v", err)
	}

	// Read captured output
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Should be valid JSON
	var parsed config.ProjectConfig
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("expected valid JSON output, got error: %v", err)
	}
}

func TestRunConfigSubcommand_Show(t *testing.T) {
	// Test that "show" is recognized as a subcommand
	// We can't fully test it without a config file, but we can verify routing

	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mockd.yaml")

	configContent := `version: "1"
admins:
  - name: local
    port: 4290
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Test that the subcommand is handled
	handled, err := runConfigSubcommand([]string{"show", "-f", configPath})
	if !handled {
		t.Error("expected 'show' to be handled as subcommand")
	}
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunConfigSubcommand_Unknown(t *testing.T) {
	// Test that unknown subcommands are not handled
	handled, err := runConfigSubcommand([]string{"unknown"})
	if handled {
		t.Error("expected 'unknown' to not be handled")
	}
	if err != nil {
		t.Errorf("unexpected error for unhandled subcommand: %v", err)
	}
}

func TestRunConfigSubcommand_Empty(t *testing.T) {
	// Test that empty args are not handled (falls through to base config command)
	handled, err := runConfigSubcommand([]string{})
	if handled {
		t.Error("expected empty args to not be handled")
	}
	if err != nil {
		t.Errorf("unexpected error for empty args: %v", err)
	}
}
