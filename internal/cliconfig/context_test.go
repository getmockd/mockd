package cliconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewDefaultContextConfig(t *testing.T) {
	cfg := NewDefaultContextConfig()

	if cfg.Version != ContextConfigVersion {
		t.Errorf("Version = %d, want %d", cfg.Version, ContextConfigVersion)
	}

	if cfg.CurrentContext != DefaultContextName {
		t.Errorf("CurrentContext = %q, want %q", cfg.CurrentContext, DefaultContextName)
	}

	ctx, exists := cfg.Contexts[DefaultContextName]
	if !exists {
		t.Fatal("default context not found")
	}

	expectedURL := DefaultAdminURL(DefaultAdminPort)
	if ctx.AdminURL != expectedURL {
		t.Errorf("AdminURL = %q, want %q", ctx.AdminURL, expectedURL)
	}
}

func TestContextConfig_GetCurrentContext(t *testing.T) {
	cfg := &ContextConfig{
		CurrentContext: "test",
		Contexts: map[string]*Context{
			"test": {AdminURL: "http://localhost:4290"},
		},
	}

	ctx := cfg.GetCurrentContext()
	if ctx == nil {
		t.Fatal("GetCurrentContext returned nil")
	}
	if ctx.AdminURL != "http://localhost:4290" {
		t.Errorf("AdminURL = %q, want %q", ctx.AdminURL, "http://localhost:4290")
	}

	// Test with non-existent context
	cfg.CurrentContext = "nonexistent"
	ctx = cfg.GetCurrentContext()
	if ctx != nil {
		t.Error("expected nil for non-existent context")
	}
}

func TestContextConfig_SetCurrentContext(t *testing.T) {
	cfg := &ContextConfig{
		CurrentContext: "local",
		Contexts: map[string]*Context{
			"local":   {AdminURL: "http://localhost:4290"},
			"staging": {AdminURL: "http://staging:4290"},
		},
	}

	// Switch to existing context
	err := cfg.SetCurrentContext("staging")
	if err != nil {
		t.Fatalf("SetCurrentContext failed: %v", err)
	}
	if cfg.CurrentContext != "staging" {
		t.Errorf("CurrentContext = %q, want %q", cfg.CurrentContext, "staging")
	}

	// Try to switch to non-existent context
	err = cfg.SetCurrentContext("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent context")
	}
}

func TestContextConfig_AddContext(t *testing.T) {
	cfg := NewDefaultContextConfig()

	// Add new context
	ctx := &Context{
		AdminURL:    "http://staging:4290",
		Description: "Staging server",
	}
	err := cfg.AddContext("staging", ctx)
	if err != nil {
		t.Fatalf("AddContext failed: %v", err)
	}

	if _, exists := cfg.Contexts["staging"]; !exists {
		t.Error("staging context not added")
	}

	// Try to add duplicate
	err = cfg.AddContext("staging", ctx)
	if err == nil {
		t.Error("expected error when adding duplicate context")
	}
}

func TestContextConfig_RemoveContext(t *testing.T) {
	cfg := &ContextConfig{
		CurrentContext: "local",
		Contexts: map[string]*Context{
			"local":   {AdminURL: "http://localhost:4290"},
			"staging": {AdminURL: "http://staging:4290"},
		},
	}

	// Remove non-current context
	err := cfg.RemoveContext("staging")
	if err != nil {
		t.Fatalf("RemoveContext failed: %v", err)
	}
	if _, exists := cfg.Contexts["staging"]; exists {
		t.Error("staging context still exists after removal")
	}

	// Try to remove current context
	err = cfg.RemoveContext("local")
	if err == nil {
		t.Error("expected error when removing current context")
	}

	// Try to remove non-existent context
	err = cfg.RemoveContext("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent context")
	}
}

func TestContextConfig_SetWorkspace(t *testing.T) {
	cfg := &ContextConfig{
		CurrentContext: "local",
		Contexts: map[string]*Context{
			"local": {AdminURL: "http://localhost:4290"},
		},
	}

	err := cfg.SetWorkspace("ws-123")
	if err != nil {
		t.Fatalf("SetWorkspace failed: %v", err)
	}

	ctx := cfg.GetCurrentContext()
	if ctx.Workspace != "ws-123" {
		t.Errorf("Workspace = %q, want %q", ctx.Workspace, "ws-123")
	}

	// Test with no current context
	cfg.CurrentContext = ""
	err = cfg.SetWorkspace("ws-456")
	if err == nil {
		t.Error("expected error when no current context")
	}
}

func TestLoadSaveContextConfig(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mockd-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config dir
	originalConfigDir := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalConfigDir)

	// Create config
	cfg := &ContextConfig{
		Version:        1,
		CurrentContext: "test",
		Contexts: map[string]*Context{
			"test": {
				AdminURL:    "http://test:4290",
				Workspace:   "ws-123",
				Description: "Test context",
			},
		},
	}

	// Save
	err = SaveContextConfig(cfg)
	if err != nil {
		t.Fatalf("SaveContextConfig failed: %v", err)
	}

	// Verify file exists
	configPath := filepath.Join(tmpDir, GlobalConfigDir, ContextConfigFileName)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("config file not created at %s", configPath)
	}

	// Load
	loaded, err := LoadContextConfig()
	if err != nil {
		t.Fatalf("LoadContextConfig failed: %v", err)
	}

	if loaded.CurrentContext != cfg.CurrentContext {
		t.Errorf("CurrentContext = %q, want %q", loaded.CurrentContext, cfg.CurrentContext)
	}

	ctx := loaded.Contexts["test"]
	if ctx == nil {
		t.Fatal("test context not found")
	}
	if ctx.AdminURL != "http://test:4290" {
		t.Errorf("AdminURL = %q, want %q", ctx.AdminURL, "http://test:4290")
	}
	if ctx.Workspace != "ws-123" {
		t.Errorf("Workspace = %q, want %q", ctx.Workspace, "ws-123")
	}
}

func TestLoadContextConfig_Default(t *testing.T) {
	// Create temp directory with no config
	tmpDir, err := os.MkdirTemp("", "mockd-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config dir
	originalConfigDir := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalConfigDir)

	// Load should return default config
	cfg, err := LoadContextConfig()
	if err != nil {
		t.Fatalf("LoadContextConfig failed: %v", err)
	}

	if cfg.CurrentContext != DefaultContextName {
		t.Errorf("CurrentContext = %q, want %q", cfg.CurrentContext, DefaultContextName)
	}

	ctx := cfg.Contexts[DefaultContextName]
	if ctx == nil {
		t.Fatal("default context not found")
	}
}

func TestLoadContextConfig_InvalidJSON(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mockd-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config dir
	originalConfigDir := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalConfigDir)

	// Create invalid config file
	configDir := filepath.Join(tmpDir, GlobalConfigDir)
	os.MkdirAll(configDir, 0755)
	configPath := filepath.Join(configDir, ContextConfigFileName)
	os.WriteFile(configPath, []byte("invalid json"), 0644)

	// Load should fail
	_, err = LoadContextConfig()
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestResolveAdminURL(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		want      string
	}{
		{
			name:      "explicit flag value",
			flagValue: "http://override:4290",
			want:      "http://override:4290",
		},
		{
			name:      "empty flag uses context",
			flagValue: "",
			want:      DefaultAdminURL(DefaultAdminPort), // Falls back to default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveAdminURL(tt.flagValue)
			if tt.flagValue != "" && got != tt.want {
				t.Errorf("ResolveAdminURL(%q) = %q, want %q", tt.flagValue, got, tt.want)
			}
		})
	}
}

func TestResolveWorkspace(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
		want      string
	}{
		{
			name:      "explicit flag value",
			flagValue: "ws-override",
			want:      "ws-override",
		},
		{
			name:      "empty flag uses context",
			flagValue: "",
			want:      "", // No workspace set by default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveWorkspace(tt.flagValue)
			if tt.flagValue != "" && got != tt.want {
				t.Errorf("ResolveWorkspace(%q) = %q, want %q", tt.flagValue, got, tt.want)
			}
		})
	}
}

func TestContextConfig_JSON_Serialization(t *testing.T) {
	cfg := &ContextConfig{
		Version:        1,
		CurrentContext: "production",
		Contexts: map[string]*Context{
			"local": {
				AdminURL:    "http://localhost:4290",
				Description: "Local development",
			},
			"production": {
				AdminURL:    "https://api.example.com:4290",
				Workspace:   "ws-prod",
				Description: "Production server",
				AuthToken:   "secret-token",
				TLSInsecure: true,
			},
		},
	}

	// Serialize
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	// Deserialize
	var loaded ContextConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	// Verify
	if loaded.Version != cfg.Version {
		t.Errorf("Version mismatch: got %d, want %d", loaded.Version, cfg.Version)
	}
	if loaded.CurrentContext != cfg.CurrentContext {
		t.Errorf("CurrentContext mismatch: got %q, want %q", loaded.CurrentContext, cfg.CurrentContext)
	}
	if len(loaded.Contexts) != len(cfg.Contexts) {
		t.Errorf("Contexts count mismatch: got %d, want %d", len(loaded.Contexts), len(cfg.Contexts))
	}

	// Verify new fields
	prodCtx := loaded.Contexts["production"]
	if prodCtx.AuthToken != "secret-token" {
		t.Errorf("AuthToken mismatch: got %q, want %q", prodCtx.AuthToken, "secret-token")
	}
	if !prodCtx.TLSInsecure {
		t.Error("TLSInsecure should be true")
	}
}

func TestGetWorkspace(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mockd-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Override config dir
	originalConfigDir := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)
	defer os.Setenv("XDG_CONFIG_HOME", originalConfigDir)

	// Clear any env var
	originalEnvWs := os.Getenv(EnvWorkspace)
	os.Unsetenv(EnvWorkspace)
	defer func() {
		if originalEnvWs != "" {
			os.Setenv(EnvWorkspace, originalEnvWs)
		}
	}()

	// Create config with workspace
	cfg := &ContextConfig{
		Version:        1,
		CurrentContext: "test",
		Contexts: map[string]*Context{
			"test": {
				AdminURL:  "http://test:4290",
				Workspace: "ws-from-context",
			},
		},
	}
	if err := SaveContextConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Should get workspace from context
	ws := GetWorkspace()
	if ws != "ws-from-context" {
		t.Errorf("GetWorkspace() = %q, want %q", ws, "ws-from-context")
	}

	// Env var should take precedence
	os.Setenv(EnvWorkspace, "ws-from-env")
	ws = GetWorkspace()
	if ws != "ws-from-env" {
		t.Errorf("GetWorkspace() with env = %q, want %q", ws, "ws-from-env")
	}
}

func TestResolveContext(t *testing.T) {
	// Clear env
	originalEnv := os.Getenv(EnvContext)
	os.Unsetenv(EnvContext)
	defer func() {
		if originalEnv != "" {
			os.Setenv(EnvContext, originalEnv)
		}
	}()

	// Flag takes precedence
	got := ResolveContext("flag-context")
	if got != "flag-context" {
		t.Errorf("ResolveContext with flag = %q, want %q", got, "flag-context")
	}

	// Env var next
	os.Setenv(EnvContext, "env-context")
	got = ResolveContext("")
	if got != "env-context" {
		t.Errorf("ResolveContext with env = %q, want %q", got, "env-context")
	}
}

func TestContext_AuthTokenAndTLS(t *testing.T) {
	ctx := &Context{
		AdminURL:    "https://api.example.com:4290",
		AuthToken:   "my-secret-token",
		TLSInsecure: true,
	}

	if ctx.AuthToken != "my-secret-token" {
		t.Errorf("AuthToken = %q, want %q", ctx.AuthToken, "my-secret-token")
	}
	if !ctx.TLSInsecure {
		t.Error("TLSInsecure should be true")
	}
}
