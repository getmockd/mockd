package cli

import (
	"os"
	"path/filepath"
	"strings"
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

func TestRunContext_Add_RejectsURLWithCredentials(t *testing.T) {
	cleanup := setupTestContextConfig(t)
	defer cleanup()

	// Initialize with default config
	cfg := cliconfig.NewDefaultContextConfig()
	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Test URLs with embedded credentials - should be rejected
	testCases := []struct {
		name string
		url  string
	}{
		{"user and password", "http://user:pass@example.com:4290"},
		{"user only", "http://user@example.com:4290"},
		{"user and password https", "https://admin:secret@staging.example.com:4290"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := RunContext([]string{"add", "-u", tc.url, "test-creds"})
			if err == nil {
				t.Errorf("expected error for URL with credentials: %s", tc.url)
			}
			if err != nil && !strings.Contains(err.Error(), "embedded credentials") {
				t.Errorf("expected 'embedded credentials' error, got: %v", err)
			}
		})
	}
}

func TestSanitizeContextForJSON(t *testing.T) {
	// Test that auth tokens are not exposed in sanitized output
	ctx := &cliconfig.Context{
		AdminURL:    "http://localhost:4290",
		Workspace:   "test-ws",
		Description: "Test context",
		AuthToken:   "super-secret-token",
		TLSInsecure: true,
	}

	sanitized := sanitizeContextForJSON(ctx)

	// Verify token is not included
	if sanitized.HasToken != true {
		t.Error("HasToken should be true when token is set")
	}

	// Verify other fields are preserved
	if sanitized.AdminURL != ctx.AdminURL {
		t.Errorf("AdminURL = %q, want %q", sanitized.AdminURL, ctx.AdminURL)
	}
	if sanitized.Workspace != ctx.Workspace {
		t.Errorf("Workspace = %q, want %q", sanitized.Workspace, ctx.Workspace)
	}
	if sanitized.Description != ctx.Description {
		t.Errorf("Description = %q, want %q", sanitized.Description, ctx.Description)
	}
	if sanitized.TLSInsecure != ctx.TLSInsecure {
		t.Errorf("TLSInsecure = %v, want %v", sanitized.TLSInsecure, ctx.TLSInsecure)
	}

	// Test with no token
	ctxNoToken := &cliconfig.Context{
		AdminURL: "http://localhost:4290",
	}
	sanitizedNoToken := sanitizeContextForJSON(ctxNoToken)
	if sanitizedNoToken.HasToken != false {
		t.Error("HasToken should be false when no token is set")
	}
}

func TestRunContext_Add_NameValidation(t *testing.T) {
	cleanup := setupTestContextConfig(t)
	defer cleanup()

	// Initialize with default config
	cfg := cliconfig.NewDefaultContextConfig()
	if err := cliconfig.SaveContextConfig(cfg); err != nil {
		t.Fatal(err)
	}

	// Test name length validation
	longName := strings.Repeat("a", 65) // 65 chars, exceeds 64 limit
	err := RunContext([]string{"add", "-u", "http://example.com:4290", longName})
	if err == nil {
		t.Error("expected error for name exceeding 64 characters")
	}
	if err != nil && !strings.Contains(err.Error(), "64 characters") {
		t.Errorf("expected '64 characters' error, got: %v", err)
	}

	// Test that 64 chars is accepted
	exactName := strings.Repeat("a", 64)
	err = RunContext([]string{"add", "-u", "http://example.com:4290", exactName})
	if err != nil {
		t.Errorf("expected 64-char name to be accepted, got: %v", err)
	}
}

func TestSanitizeContextsForJSON(t *testing.T) {
	contexts := map[string]*cliconfig.Context{
		"local": {
			AdminURL:  "http://localhost:4290",
			AuthToken: "",
		},
		"cloud": {
			AdminURL:  "https://api.mockd.io",
			AuthToken: "secret-cloud-token",
		},
	}

	sanitized := sanitizeContextsForJSON(contexts)

	if len(sanitized) != 2 {
		t.Errorf("expected 2 contexts, got %d", len(sanitized))
	}

	if sanitized["local"].HasToken != false {
		t.Error("local context should not have token")
	}
	if sanitized["cloud"].HasToken != true {
		t.Error("cloud context should have token")
	}
}
