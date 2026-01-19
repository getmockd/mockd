package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getmockd/mockd/internal/cliconfig"
)

func setupTestContextConfig(t *testing.T) (cleanup func()) {
	t.Helper()

	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "mockd-test")
	if err != nil {
		t.Fatal(err)
	}

	// Override config dir
	originalConfigDir := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", tmpDir)

	return func() {
		os.Setenv("XDG_CONFIG_HOME", originalConfigDir)
		os.RemoveAll(tmpDir)
	}
}

func TestRunContext_HelpFlag(t *testing.T) {
	// Just verify it doesn't error
	err := RunContext([]string{"--help"})
	if err != nil {
		t.Errorf("RunContext --help failed: %v", err)
	}
}

func TestRunContext_Show(t *testing.T) {
	cleanup := setupTestContextConfig(t)
	defer cleanup()

	// Initialize with default config
	cfg := cliconfig.NewDefaultContextConfig()
	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Should not error
	err := RunContext([]string{})
	if err != nil {
		t.Errorf("RunContext show failed: %v", err)
	}

	err = RunContext([]string{"show"})
	if err != nil {
		t.Errorf("RunContext show failed: %v", err)
	}
}

func TestRunContext_List(t *testing.T) {
	cleanup := setupTestContextConfig(t)
	defer cleanup()

	// Initialize with some contexts
	cfg := &cliconfig.ContextConfig{
		Version:        1,
		CurrentContext: "local",
		Contexts: map[string]*cliconfig.Context{
			"local": {
				AdminURL:    "http://localhost:4290",
				Description: "Local server",
			},
			"staging": {
				AdminURL:    "http://staging:4290",
				Description: "Staging server",
			},
		},
	}
	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Should not error
	err := RunContext([]string{"list"})
	if err != nil {
		t.Errorf("RunContext list failed: %v", err)
	}

	// JSON output
	err = RunContext([]string{"list", "--json"})
	if err != nil {
		t.Errorf("RunContext list --json failed: %v", err)
	}
}

func TestRunContext_Use(t *testing.T) {
	cleanup := setupTestContextConfig(t)
	defer cleanup()

	// Initialize with some contexts
	cfg := &cliconfig.ContextConfig{
		Version:        1,
		CurrentContext: "local",
		Contexts: map[string]*cliconfig.Context{
			"local":   {AdminURL: "http://localhost:4290"},
			"staging": {AdminURL: "http://staging:4290"},
		},
	}
	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Switch to staging
	err := RunContext([]string{"use", "staging"})
	if err != nil {
		t.Errorf("RunContext use staging failed: %v", err)
	}

	// Verify switch
	loaded, _ := cliconfig.LoadContextConfig()
	if loaded.CurrentContext != "staging" {
		t.Errorf("CurrentContext = %q, want %q", loaded.CurrentContext, "staging")
	}

	// Try non-existent context
	err = RunContext([]string{"use", "nonexistent"})
	if err == nil {
		t.Error("expected error for non-existent context")
	}

	// Missing argument
	err = RunContext([]string{"use"})
	if err == nil {
		t.Error("expected error for missing argument")
	}
}

func TestRunContext_Add(t *testing.T) {
	cleanup := setupTestContextConfig(t)
	defer cleanup()

	// Initialize with default config
	cfg := cliconfig.NewDefaultContextConfig()
	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Add a new context (flags before positional arg)
	err := RunContext([]string{"add", "--admin-url", "http://staging:4290", "staging"})
	if err != nil {
		t.Errorf("RunContext add failed: %v", err)
	}

	// Verify it was added
	loaded, _ := cliconfig.LoadContextConfig()
	if _, exists := loaded.Contexts["staging"]; !exists {
		t.Error("staging context not added")
	}

	// Try to add duplicate
	err = RunContext([]string{"add", "--admin-url", "http://staging2:4290", "staging"})
	if err == nil {
		t.Error("expected error for duplicate context")
	}

	// Missing name
	err = RunContext([]string{"add"})
	if err == nil {
		t.Error("expected error for missing name")
	}

	// Add with --use flag (unified from --set-current)
	err = RunContext([]string{"add", "-u", "http://prod:4290", "--use", "production"})
	if err != nil {
		t.Errorf("RunContext add --use failed: %v", err)
	}

	// Verify it's now current
	loaded, _ = cliconfig.LoadContextConfig()
	if loaded.CurrentContext != "production" {
		t.Errorf("CurrentContext = %q, want %q", loaded.CurrentContext, "production")
	}

	// Add with auth token
	err = RunContext([]string{"add", "-u", "http://cloud:4290", "-t", "secret-token", "cloud"})
	if err != nil {
		t.Errorf("RunContext add with token failed: %v", err)
	}

	// Test validation: empty name
	err = RunContext([]string{"add", "-u", "http://test:4290", ""})
	if err == nil {
		t.Error("expected error for empty name")
	}

	// Test validation: invalid URL
	err = RunContext([]string{"add", "-u", "not-a-url", "badurl"})
	if err == nil {
		t.Error("expected error for invalid URL")
	}

	// Test validation: whitespace in name
	err = RunContext([]string{"add", "-u", "http://test:4290", "has space"})
	if err == nil {
		t.Error("expected error for whitespace in name")
	}

	loaded, _ = cliconfig.LoadContextConfig()
	cloudCtx := loaded.Contexts["cloud"]
	if cloudCtx == nil {
		t.Fatal("cloud context not found")
	}
	if cloudCtx.AuthToken != "secret-token" {
		t.Errorf("AuthToken = %q, want %q", cloudCtx.AuthToken, "secret-token")
	}
}

func TestRunContext_Remove(t *testing.T) {
	cleanup := setupTestContextConfig(t)
	defer cleanup()

	// Initialize with some contexts
	cfg := &cliconfig.ContextConfig{
		Version:        1,
		CurrentContext: "local",
		Contexts: map[string]*cliconfig.Context{
			"local":   {AdminURL: "http://localhost:4290"},
			"staging": {AdminURL: "http://staging:4290"},
		},
	}
	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Remove staging (force to skip confirmation, flags before positional arg)
	err := RunContext([]string{"remove", "--force", "staging"})
	if err != nil {
		t.Errorf("RunContext remove failed: %v", err)
	}

	// Verify it was removed
	loaded, _ := cliconfig.LoadContextConfig()
	if _, exists := loaded.Contexts["staging"]; exists {
		t.Error("staging context still exists")
	}

	// Try to remove current context
	err = RunContext([]string{"remove", "--force", "local"})
	if err == nil {
		t.Error("expected error when removing current context")
	}

	// Try to remove non-existent context
	err = RunContext([]string{"remove", "--force", "nonexistent"})
	if err == nil {
		t.Error("expected error for non-existent context")
	}
}

func TestRunContext_UnknownSubcommand(t *testing.T) {
	err := RunContext([]string{"unknown"})
	if err == nil {
		t.Error("expected error for unknown subcommand")
	}
}

func TestContextConfigPath(t *testing.T) {
	cleanup := setupTestContextConfig(t)
	defer cleanup()

	path, err := cliconfig.GetContextConfigPath()
	if err != nil {
		t.Fatalf("GetContextConfigPath failed: %v", err)
	}

	// Should contain contexts.json
	if filepath.Base(path) != cliconfig.ContextConfigFileName {
		t.Errorf("path = %q, want to end with %q", path, cliconfig.ContextConfigFileName)
	}
}
