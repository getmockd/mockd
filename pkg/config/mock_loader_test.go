package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMocksFromEntry_Inline(t *testing.T) {
	entry := MockEntry{
		ID:        "health-check",
		Workspace: "default",
		Type:      "http",
		HTTP: &HTTPMockConfig{
			Matcher: HTTPMatcher{
				Path: "/health",
			},
			Response: HTTPResponse{
				StatusCode: 200,
				Body:       `{"status": "ok"}`,
			},
		},
	}

	mocks, err := LoadMocksFromEntry(entry, "/tmp")
	if err != nil {
		t.Fatalf("LoadMocksFromEntry failed: %v", err)
	}

	if len(mocks) != 1 {
		t.Fatalf("expected 1 mock, got %d", len(mocks))
	}

	if mocks[0].ID != "health-check" {
		t.Errorf("expected ID 'health-check', got %q", mocks[0].ID)
	}
	if mocks[0].Workspace != "default" {
		t.Errorf("expected Workspace 'default', got %q", mocks[0].Workspace)
	}
	if mocks[0].Type != "http" {
		t.Errorf("expected Type 'http', got %q", mocks[0].Type)
	}
}

func TestLoadMocksFromEntry_FileRef_SingleMock(t *testing.T) {
	// Create a temp directory
	tmpDir := t.TempDir()

	// Create a mock file with a single mock
	mockContent := `id: get-user
workspace: default
type: http
http:
  matcher:
    path: /api/user
  response:
    statusCode: 200
    body: '{"id": 1, "name": "John"}'
`
	mockFile := filepath.Join(tmpDir, "user.yaml")
	if err := os.WriteFile(mockFile, []byte(mockContent), 0644); err != nil {
		t.Fatalf("failed to write mock file: %v", err)
	}

	entry := MockEntry{File: "./user.yaml"}
	mocks, err := LoadMocksFromEntry(entry, tmpDir)
	if err != nil {
		t.Fatalf("LoadMocksFromEntry failed: %v", err)
	}

	if len(mocks) != 1 {
		t.Fatalf("expected 1 mock, got %d", len(mocks))
	}

	if mocks[0].ID != "get-user" {
		t.Errorf("expected ID 'get-user', got %q", mocks[0].ID)
	}
	if mocks[0].HTTP == nil {
		t.Fatal("expected HTTP config to be set")
	}
	if mocks[0].HTTP.Matcher.Path != "/api/user" {
		t.Errorf("expected path '/api/user', got %q", mocks[0].HTTP.Matcher.Path)
	}
}

func TestLoadMocksFromEntry_FileRef_ArrayOfMocks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mock file with an array of mocks
	mockContent := `- id: list-users
  workspace: default
  type: http
  http:
    matcher:
      path: /api/users
      method: GET
    response:
      statusCode: 200
      body: '[]'

- id: create-user
  workspace: default
  type: http
  http:
    matcher:
      path: /api/users
      method: POST
    response:
      statusCode: 201
      body: '{"id": 1}'
`
	mockFile := filepath.Join(tmpDir, "users.yaml")
	if err := os.WriteFile(mockFile, []byte(mockContent), 0644); err != nil {
		t.Fatalf("failed to write mock file: %v", err)
	}

	entry := MockEntry{File: "users.yaml"}
	mocks, err := LoadMocksFromEntry(entry, tmpDir)
	if err != nil {
		t.Fatalf("LoadMocksFromEntry failed: %v", err)
	}

	if len(mocks) != 2 {
		t.Fatalf("expected 2 mocks, got %d", len(mocks))
	}

	if mocks[0].ID != "list-users" {
		t.Errorf("expected first mock ID 'list-users', got %q", mocks[0].ID)
	}
	if mocks[1].ID != "create-user" {
		t.Errorf("expected second mock ID 'create-user', got %q", mocks[1].ID)
	}
}

func TestLoadMocksFromEntry_Glob(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a mocks subdirectory
	mocksDir := filepath.Join(tmpDir, "mocks")
	if err := os.MkdirAll(mocksDir, 0755); err != nil {
		t.Fatalf("failed to create mocks dir: %v", err)
	}

	// Create mock files
	mock1 := `id: mock-1
type: http
http:
  matcher:
    path: /one
  response:
    statusCode: 200
`
	mock2 := `id: mock-2
type: http
http:
  matcher:
    path: /two
  response:
    statusCode: 200
`
	if err := os.WriteFile(filepath.Join(mocksDir, "one.yaml"), []byte(mock1), 0644); err != nil {
		t.Fatalf("failed to write mock1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(mocksDir, "two.yaml"), []byte(mock2), 0644); err != nil {
		t.Fatalf("failed to write mock2: %v", err)
	}

	entry := MockEntry{Files: "./mocks/*.yaml"}
	mocks, err := LoadMocksFromEntry(entry, tmpDir)
	if err != nil {
		t.Fatalf("LoadMocksFromEntry failed: %v", err)
	}

	if len(mocks) != 2 {
		t.Fatalf("expected 2 mocks, got %d", len(mocks))
	}

	// Mocks should be sorted by filename
	ids := []string{mocks[0].ID, mocks[1].ID}
	if ids[0] != "mock-1" || ids[1] != "mock-2" {
		t.Errorf("expected IDs ['mock-1', 'mock-2'], got %v", ids)
	}
}

func TestLoadMocksFromEntry_Glob_Recursive(t *testing.T) {
	tmpDir := t.TempDir()

	// Create nested directory structure
	subDir := filepath.Join(tmpDir, "mocks", "api", "users")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create nested dirs: %v", err)
	}

	// Create mock files at different levels
	rootMock := `id: root-mock
type: http
http:
  matcher:
    path: /root
  response:
    statusCode: 200
`
	nestedMock := `id: nested-mock
type: http
http:
  matcher:
    path: /nested
  response:
    statusCode: 200
`
	if err := os.WriteFile(filepath.Join(tmpDir, "mocks", "root.yaml"), []byte(rootMock), 0644); err != nil {
		t.Fatalf("failed to write root mock: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "users.yaml"), []byte(nestedMock), 0644); err != nil {
		t.Fatalf("failed to write nested mock: %v", err)
	}

	entry := MockEntry{Files: "./mocks/**/*.yaml"}
	mocks, err := LoadMocksFromEntry(entry, tmpDir)
	if err != nil {
		t.Fatalf("LoadMocksFromEntry failed: %v", err)
	}

	if len(mocks) != 2 {
		t.Fatalf("expected 2 mocks, got %d", len(mocks))
	}

	// Check both mocks were loaded
	ids := make(map[string]bool)
	for _, m := range mocks {
		ids[m.ID] = true
	}
	if !ids["root-mock"] {
		t.Error("expected to find 'root-mock'")
	}
	if !ids["nested-mock"] {
		t.Error("expected to find 'nested-mock'")
	}
}

func TestLoadMocksFromEntry_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	entry := MockEntry{File: "./nonexistent.yaml"}
	_, err := LoadMocksFromEntry(entry, tmpDir)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadMocksFromEntry_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid YAML file (unclosed bracket is truly invalid)
	invalidYAML := `invalid: [unclosed bracket
id: test`
	mockFile := filepath.Join(tmpDir, "invalid.yaml")
	if err := os.WriteFile(mockFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write invalid file: %v", err)
	}

	entry := MockEntry{File: "./invalid.yaml"}
	_, err := LoadMocksFromEntry(entry, tmpDir)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadMocksFromEntry_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an empty file
	mockFile := filepath.Join(tmpDir, "empty.yaml")
	if err := os.WriteFile(mockFile, []byte(""), 0644); err != nil {
		t.Fatalf("failed to write empty file: %v", err)
	}

	entry := MockEntry{File: "./empty.yaml"}
	_, err := LoadMocksFromEntry(entry, tmpDir)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}

func TestLoadMocksFromEntry_Glob_NoMatches(t *testing.T) {
	tmpDir := t.TempDir()

	entry := MockEntry{Files: "./mocks/*.yaml"}
	mocks, err := LoadMocksFromEntry(entry, tmpDir)
	if err != nil {
		t.Fatalf("LoadMocksFromEntry failed: %v", err)
	}

	// No matches should return empty slice, not error
	if len(mocks) != 0 {
		t.Errorf("expected 0 mocks, got %d", len(mocks))
	}
}

func TestLoadMocksFromEntry_InvalidEntry(t *testing.T) {
	entry := MockEntry{} // Empty entry - no ID, no File, no Files
	_, err := LoadMocksFromEntry(entry, "/tmp")
	if err == nil {
		t.Fatal("expected error for invalid entry")
	}
}

func TestLoadAllMocks(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mock files
	mockFile := `id: file-mock
type: http
http:
  matcher:
    path: /file
  response:
    statusCode: 200
`
	if err := os.WriteFile(filepath.Join(tmpDir, "file.yaml"), []byte(mockFile), 0644); err != nil {
		t.Fatalf("failed to write mock file: %v", err)
	}

	entries := []MockEntry{
		{
			ID:   "inline-mock",
			Type: "http",
			HTTP: &HTTPMockConfig{
				Matcher:  HTTPMatcher{Path: "/inline"},
				Response: HTTPResponse{StatusCode: 200},
			},
		},
		{File: "./file.yaml"},
	}

	mocks, err := LoadAllMocks(entries, tmpDir)
	if err != nil {
		t.Fatalf("LoadAllMocks failed: %v", err)
	}

	if len(mocks) != 2 {
		t.Fatalf("expected 2 mocks, got %d", len(mocks))
	}

	// Check both mocks are present
	ids := make(map[string]bool)
	for _, m := range mocks {
		ids[m.ID] = true
	}
	if !ids["inline-mock"] {
		t.Error("expected to find 'inline-mock'")
	}
	if !ids["file-mock"] {
		t.Error("expected to find 'file-mock'")
	}
}

func TestLoadAllMocks_FileError(t *testing.T) {
	tmpDir := t.TempDir()

	entries := []MockEntry{
		{ID: "inline-mock", Type: "http"},
		{File: "./nonexistent.yaml"},
	}

	_, err := LoadAllMocks(entries, tmpDir)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}

	// Error message should include the entry index and file path
	errStr := err.Error()
	if errStr == "" {
		t.Error("expected non-empty error message")
	}
}

func TestLoadMocksFromEntry_EnvVarExpansion(t *testing.T) {
	tmpDir := t.TempDir()

	// Set environment variable
	os.Setenv("TEST_STATUS_CODE", "201")
	defer os.Unsetenv("TEST_STATUS_CODE")

	mockContent := `id: env-mock
type: http
http:
  matcher:
    path: /env
  response:
    statusCode: ${TEST_STATUS_CODE}
    body: '{"created": true}'
`
	mockFile := filepath.Join(tmpDir, "env.yaml")
	if err := os.WriteFile(mockFile, []byte(mockContent), 0644); err != nil {
		t.Fatalf("failed to write mock file: %v", err)
	}

	entry := MockEntry{File: "./env.yaml"}
	mocks, err := LoadMocksFromEntry(entry, tmpDir)
	if err != nil {
		t.Fatalf("LoadMocksFromEntry failed: %v", err)
	}

	if len(mocks) != 1 {
		t.Fatalf("expected 1 mock, got %d", len(mocks))
	}

	if mocks[0].HTTP.Response.StatusCode != 201 {
		t.Errorf("expected status code 201, got %d", mocks[0].HTTP.Response.StatusCode)
	}
}

func TestGetMockFileBaseDir(t *testing.T) {
	tests := []struct {
		name       string
		configPath string
		expected   string
	}{
		{
			name:       "with config path",
			configPath: "/home/user/project/mockd.yaml",
			expected:   "/home/user/project",
		},
		{
			name:       "with nested config path",
			configPath: "/home/user/project/config/mockd.yaml",
			expected:   "/home/user/project/config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetMockFileBaseDir(tt.configPath)
			if result != tt.expected {
				t.Errorf("GetMockFileBaseDir(%q) = %q, want %q", tt.configPath, result, tt.expected)
			}
		})
	}
}

func TestGetMockFileBaseDir_EmptyPath(t *testing.T) {
	result := GetMockFileBaseDir("")
	// Should return current working directory or "."
	if result == "" {
		t.Error("expected non-empty result for empty config path")
	}
}

func TestMockFileContent_IsInline(t *testing.T) {
	tests := []struct {
		name     string
		entry    MockEntry
		isInline bool
		isFile   bool
		isGlob   bool
	}{
		{
			name:     "inline with ID",
			entry:    MockEntry{ID: "test"},
			isInline: true,
		},
		{
			name:     "inline with Type",
			entry:    MockEntry{Type: "http"},
			isInline: true,
		},
		{
			name:   "file reference",
			entry:  MockEntry{File: "./mocks.yaml"},
			isFile: true,
		},
		{
			name:   "glob pattern",
			entry:  MockEntry{Files: "./**/*.yaml"},
			isGlob: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.entry.IsInline(); got != tt.isInline {
				t.Errorf("IsInline() = %v, want %v", got, tt.isInline)
			}
			if got := tt.entry.IsFileRef(); got != tt.isFile {
				t.Errorf("IsFileRef() = %v, want %v", got, tt.isFile)
			}
			if got := tt.entry.IsGlob(); got != tt.isGlob {
				t.Errorf("IsGlob() = %v, want %v", got, tt.isGlob)
			}
		})
	}
}
