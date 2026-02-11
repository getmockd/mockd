package config

import (
	"os"
	"testing"
)

func TestExpandEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "no variables",
			input:    "hello world",
			envVars:  nil,
			expected: "hello world",
		},
		{
			name:     "simple variable",
			input:    "port: ${PORT}",
			envVars:  map[string]string{"PORT": "8080"},
			expected: "port: 8080",
		},
		{
			name:     "variable with default - value exists",
			input:    "port: ${PORT:-3000}",
			envVars:  map[string]string{"PORT": "8080"},
			expected: "port: 8080",
		},
		{
			name:     "variable with default - value missing",
			input:    "port: ${PORT:-3000}",
			envVars:  nil,
			expected: "port: 3000",
		},
		{
			name:     "variable with empty default",
			input:    "key: ${API_KEY:-}",
			envVars:  nil,
			expected: "key: ",
		},
		{
			name:     "multiple variables",
			input:    "host: ${HOST}, port: ${PORT}",
			envVars:  map[string]string{"HOST": "localhost", "PORT": "8080"},
			expected: "host: localhost, port: 8080",
		},
		{
			name:     "mixed with and without defaults",
			input:    "url: ${PROTO:-http}://${HOST}:${PORT:-80}",
			envVars:  map[string]string{"HOST": "example.com"},
			expected: "url: http://example.com:80",
		},
		{
			name:     "variable not set without default",
			input:    "key: ${MISSING_VAR}",
			envVars:  nil,
			expected: "key: ",
		},
		{
			name:     "underscore in name",
			input:    "${MY_VAR_NAME}",
			envVars:  map[string]string{"MY_VAR_NAME": "value"},
			expected: "value",
		},
		{
			name:     "number in name",
			input:    "${VAR1}",
			envVars:  map[string]string{"VAR1": "one"},
			expected: "one",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear and set env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			result := ExpandEnvVars(tt.input)
			if result != tt.expected {
				t.Errorf("ExpandEnvVars(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoadProjectConfigFromBytes(t *testing.T) {
	yaml := `
version: "1.0"

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

mocks:
  - id: health
    workspace: default
    type: http
    http:
      matcher:
        path: /health
      response:
        statusCode: 200
        body: '{"status": "ok"}'
`

	cfg, err := LoadProjectConfigFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadProjectConfigFromBytes failed: %v", err)
	}

	if cfg.Version != "1.0" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1.0")
	}

	if len(cfg.Admins) != 1 {
		t.Fatalf("len(Admins) = %d, want 1", len(cfg.Admins))
	}
	if cfg.Admins[0].Name != "local" {
		t.Errorf("Admins[0].Name = %q, want %q", cfg.Admins[0].Name, "local")
	}
	if cfg.Admins[0].Port != 4290 {
		t.Errorf("Admins[0].Port = %d, want %d", cfg.Admins[0].Port, 4290)
	}

	if len(cfg.Engines) != 1 {
		t.Fatalf("len(Engines) = %d, want 1", len(cfg.Engines))
	}
	if cfg.Engines[0].Name != "default" {
		t.Errorf("Engines[0].Name = %q, want %q", cfg.Engines[0].Name, "default")
	}
	if cfg.Engines[0].HTTPPort != 4280 {
		t.Errorf("Engines[0].HTTPPort = %d, want %d", cfg.Engines[0].HTTPPort, 4280)
	}
	if cfg.Engines[0].Admin != "local" {
		t.Errorf("Engines[0].Admin = %q, want %q", cfg.Engines[0].Admin, "local")
	}

	if len(cfg.Mocks) != 1 {
		t.Fatalf("len(Mocks) = %d, want 1", len(cfg.Mocks))
	}
	if cfg.Mocks[0].ID != "health" {
		t.Errorf("Mocks[0].ID = %q, want %q", cfg.Mocks[0].ID, "health")
	}
	if cfg.Mocks[0].Type != "http" {
		t.Errorf("Mocks[0].Type = %q, want %q", cfg.Mocks[0].Type, "http")
	}
}

func TestLoadProjectConfigFromBytes_WithEnvVars(t *testing.T) {
	os.Setenv("API_KEY", "secret123")
	defer os.Unsetenv("API_KEY")

	yaml := `
version: "1.0"

admins:
  - name: production
    url: https://admin.example.com
    apiKey: ${API_KEY}

engines:
  - name: default
    httpPort: 4280
    admin: production
`

	cfg, err := LoadProjectConfigFromBytes([]byte(yaml))
	if err != nil {
		t.Fatalf("LoadProjectConfigFromBytes failed: %v", err)
	}

	if cfg.Admins[0].APIKey != "secret123" {
		t.Errorf("Admins[0].APIKey = %q, want %q", cfg.Admins[0].APIKey, "secret123")
	}
}

func TestMergeProjectConfigs(t *testing.T) {
	base := &ProjectConfig{
		Version: "1.0",
		Admins: []AdminConfig{
			{Name: "local", Port: 4290},
		},
		Engines: []EngineConfig{
			{Name: "default", HTTPPort: 4280, Admin: "local"},
		},
	}

	overlay := &ProjectConfig{
		Admins: []AdminConfig{
			{Name: "local", Port: 9090}, // Override port
		},
		Engines: []EngineConfig{
			{Name: "default", HTTPPort: 9080}, // Override port
			{Name: "extra", HTTPPort: 9081, Admin: "local"},
		},
	}

	result := MergeProjectConfigs(base, overlay)

	if result.Version != "1.0" {
		t.Errorf("Version = %q, want %q", result.Version, "1.0")
	}

	if len(result.Admins) != 1 {
		t.Fatalf("len(Admins) = %d, want 1", len(result.Admins))
	}
	if result.Admins[0].Port != 9090 {
		t.Errorf("Admins[0].Port = %d, want %d", result.Admins[0].Port, 9090)
	}

	if len(result.Engines) != 2 {
		t.Fatalf("len(Engines) = %d, want 2", len(result.Engines))
	}
	if result.Engines[0].HTTPPort != 9080 {
		t.Errorf("Engines[0].HTTPPort = %d, want %d", result.Engines[0].HTTPPort, 9080)
	}
	if result.Engines[0].Admin != "local" {
		t.Errorf("Engines[0].Admin = %q, want %q", result.Engines[0].Admin, "local")
	}
	if result.Engines[1].Name != "extra" {
		t.Errorf("Engines[1].Name = %q, want %q", result.Engines[1].Name, "extra")
	}
}

func TestValidateProjectConfig_Valid(t *testing.T) {
	cfg := &ProjectConfig{
		Version: "1.0",
		Admins: []AdminConfig{
			{Name: "local", Port: 4290},
		},
		Engines: []EngineConfig{
			{Name: "default", HTTPPort: 4280, Admin: "local"},
		},
		Workspaces: []WorkspaceConfig{
			{Name: "default", Engines: []string{"default"}},
		},
		Mocks: []MockEntry{
			{
				ID:        "health",
				Workspace: "default",
				Type:      "http",
				HTTP: &HTTPMockConfig{
					Matcher:  HTTPMatcher{Path: "/health"},
					Response: HTTPResponse{StatusCode: 200, Body: "ok"},
				},
			},
		},
	}

	result := ValidateProjectConfig(cfg)
	if !result.IsValid() {
		t.Errorf("ValidateProjectConfig returned errors for valid config: %v", result.Errors)
	}
}

func TestValidateProjectConfig_MissingVersion(t *testing.T) {
	cfg := &ProjectConfig{
		Admins: []AdminConfig{
			{Name: "local", Port: 4290},
		},
	}

	result := ValidateProjectConfig(cfg)
	if result.IsValid() {
		t.Error("ValidateProjectConfig should have returned error for missing version")
	}

	found := false
	for _, err := range result.Errors {
		if err.Path == "version" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected error for 'version' path")
	}
}

func TestValidateProjectConfig_InvalidVersion(t *testing.T) {
	cfg := &ProjectConfig{
		Version: "2",
	}

	result := ValidateProjectConfig(cfg)
	if result.IsValid() {
		t.Error("ValidateProjectConfig should have returned error for unsupported version")
	}
}

func TestValidateProjectConfig_DuplicateAdminName(t *testing.T) {
	cfg := &ProjectConfig{
		Version: "1.0",
		Admins: []AdminConfig{
			{Name: "local", Port: 4290},
			{Name: "local", Port: 4291}, // Duplicate
		},
	}

	result := ValidateProjectConfig(cfg)
	if result.IsValid() {
		t.Error("ValidateProjectConfig should have returned error for duplicate admin name")
	}
}

func TestValidateProjectConfig_LocalAdminMissingPort(t *testing.T) {
	cfg := &ProjectConfig{
		Version: "1.0",
		Admins: []AdminConfig{
			{Name: "local"}, // No port, no URL
		},
	}

	result := ValidateProjectConfig(cfg)
	if result.IsValid() {
		t.Error("ValidateProjectConfig should have returned error for local admin missing port")
	}
}

func TestValidateProjectConfig_RemoteAdmin(t *testing.T) {
	cfg := &ProjectConfig{
		Version: "1.0",
		Admins: []AdminConfig{
			{Name: "remote", URL: "https://admin.example.com", APIKey: "secret"},
		},
		Engines: []EngineConfig{
			{Name: "default", HTTPPort: 4280, Admin: "remote"},
		},
	}

	result := ValidateProjectConfig(cfg)
	if !result.IsValid() {
		t.Errorf("ValidateProjectConfig returned errors for valid remote admin config: %v", result.Errors)
	}
}

func TestValidateProjectConfig_EngineReferencesUnknownAdmin(t *testing.T) {
	cfg := &ProjectConfig{
		Version: "1.0",
		Admins: []AdminConfig{
			{Name: "local", Port: 4290},
		},
		Engines: []EngineConfig{
			{Name: "default", HTTPPort: 4280, Admin: "nonexistent"},
		},
	}

	result := ValidateProjectConfig(cfg)
	if result.IsValid() {
		t.Error("ValidateProjectConfig should have returned error for unknown admin reference")
	}
}

func TestValidateProjectConfig_EngineMissingPorts(t *testing.T) {
	cfg := &ProjectConfig{
		Version: "1.0",
		Admins: []AdminConfig{
			{Name: "local", Port: 4290},
		},
		Engines: []EngineConfig{
			{Name: "default", Admin: "local"}, // No ports
		},
	}

	result := ValidateProjectConfig(cfg)
	if result.IsValid() {
		t.Error("ValidateProjectConfig should have returned error for engine with no ports")
	}
}

func TestValidateProjectConfig_MockFileRef(t *testing.T) {
	cfg := &ProjectConfig{
		Version: "1.0",
		Admins: []AdminConfig{
			{Name: "local", Port: 4290},
		},
		Engines: []EngineConfig{
			{Name: "default", HTTPPort: 4280, Admin: "local"},
		},
		Mocks: []MockEntry{
			{File: "./mocks/users.yaml"},
		},
	}

	result := ValidateProjectConfig(cfg)
	if !result.IsValid() {
		t.Errorf("ValidateProjectConfig returned errors for file ref mock: %v", result.Errors)
	}
}

func TestValidateProjectConfig_MockGlob(t *testing.T) {
	cfg := &ProjectConfig{
		Version: "1.0",
		Admins: []AdminConfig{
			{Name: "local", Port: 4290},
		},
		Engines: []EngineConfig{
			{Name: "default", HTTPPort: 4280, Admin: "local"},
		},
		Mocks: []MockEntry{
			{Files: "./mocks/**/*.yaml"},
		},
	}

	result := ValidateProjectConfig(cfg)
	if !result.IsValid() {
		t.Errorf("ValidateProjectConfig returned errors for glob mock: %v", result.Errors)
	}
}

func TestValidatePortConflicts(t *testing.T) {
	cfg := &ProjectConfig{
		Version: "1.0",
		Admins: []AdminConfig{
			{Name: "local", Port: 4280}, // Same as engine HTTP
		},
		Engines: []EngineConfig{
			{Name: "default", HTTPPort: 4280, Admin: "local"},
		},
	}

	result := ValidatePortConflicts(cfg)
	if result.IsValid() {
		t.Error("ValidatePortConflicts should have detected port conflict")
	}
}

func TestValidatePortConflicts_NoConflict(t *testing.T) {
	cfg := &ProjectConfig{
		Version: "1.0",
		Admins: []AdminConfig{
			{Name: "local", Port: 4290},
		},
		Engines: []EngineConfig{
			{Name: "default", HTTPPort: 4280, HTTPSPort: 4443, Admin: "local"},
		},
	}

	result := ValidatePortConflicts(cfg)
	if !result.IsValid() {
		t.Errorf("ValidatePortConflicts returned errors for valid config: %v", result.Errors)
	}
}

func TestAdminConfig_IsLocal(t *testing.T) {
	local := AdminConfig{Name: "local", Port: 4290}
	if !local.IsLocal() {
		t.Error("AdminConfig without URL should be local")
	}

	remote := AdminConfig{Name: "remote", URL: "https://admin.example.com"}
	if remote.IsLocal() {
		t.Error("AdminConfig with URL should not be local")
	}
}

func TestMockEntry_TypeChecks(t *testing.T) {
	inline := MockEntry{ID: "test", Type: "http"}
	if !inline.IsInline() {
		t.Error("MockEntry with ID should be inline")
	}
	if inline.IsFileRef() || inline.IsGlob() {
		t.Error("Inline mock should not be file ref or glob")
	}

	fileRef := MockEntry{File: "./mocks.yaml"}
	if !fileRef.IsFileRef() {
		t.Error("MockEntry with File should be file ref")
	}
	if fileRef.IsInline() || fileRef.IsGlob() {
		t.Error("File ref mock should not be inline or glob")
	}

	glob := MockEntry{Files: "./**/*.yaml"}
	if !glob.IsGlob() {
		t.Error("MockEntry with Files should be glob")
	}
	if glob.IsInline() || glob.IsFileRef() {
		t.Error("Glob mock should not be inline or file ref")
	}
}

func TestDefaultProjectConfig(t *testing.T) {
	cfg := DefaultProjectConfig()

	if cfg.Version != "1.0" {
		t.Errorf("Version = %q, want %q", cfg.Version, "1.0")
	}

	if len(cfg.Admins) != 1 {
		t.Fatalf("len(Admins) = %d, want 1", len(cfg.Admins))
	}
	if cfg.Admins[0].Name != "local" {
		t.Errorf("Admins[0].Name = %q, want %q", cfg.Admins[0].Name, "local")
	}
	if cfg.Admins[0].Port != 4290 {
		t.Errorf("Admins[0].Port = %d, want %d", cfg.Admins[0].Port, 4290)
	}

	if len(cfg.Engines) != 1 {
		t.Fatalf("len(Engines) = %d, want 1", len(cfg.Engines))
	}
	if cfg.Engines[0].Name != "default" {
		t.Errorf("Engines[0].Name = %q, want %q", cfg.Engines[0].Name, "default")
	}
	if cfg.Engines[0].HTTPPort != 4280 {
		t.Errorf("Engines[0].HTTPPort = %d, want %d", cfg.Engines[0].HTTPPort, 4280)
	}

	if len(cfg.Workspaces) != 1 {
		t.Fatalf("len(Workspaces) = %d, want 1", len(cfg.Workspaces))
	}
	if cfg.Workspaces[0].Name != "default" {
		t.Errorf("Workspaces[0].Name = %q, want %q", cfg.Workspaces[0].Name, "default")
	}
}
