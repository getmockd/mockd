package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validYAML returns a minimal valid mockd YAML config with the given mock ID and path.
func validYAML(id, path string) string {
	return `version: "1.0"
mocks:
  - id: "` + id + `"
    type: http
    http:
      matcher:
        method: GET
        path: ` + path + `
      response:
        statusCode: 200
        body: "ok"
`
}

// writeFile is a test helper that writes content to a file, creating parent dirs as needed.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

// =============================================================================
// TestDirLoader — DirectoryLoader.Load() tests
// =============================================================================

func TestDirLoader_SingleYAMLFile(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "api.yaml"), validYAML("test-1", "/test"))

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.FileCount)
	assert.Len(t, result.Collection.Mocks, 1)
	assert.Contains(t, result.Collection.Mocks[0].ID, "test-1")
	assert.Empty(t, result.Errors)
}

func TestDirLoader_MultipleYAMLFilesMerged(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "a.yaml"), validYAML("mock-a", "/a"))
	writeFile(t, filepath.Join(tmpDir, "b.yaml"), validYAML("mock-b", "/b"))

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.FileCount)
	assert.Len(t, result.Collection.Mocks, 2)
	assert.Empty(t, result.Errors)

	// Both mocks should be present (order depends on filesystem, so just check IDs exist)
	ids := make([]string, len(result.Collection.Mocks))
	for i, m := range result.Collection.Mocks {
		ids[i] = m.ID
	}
	assert.True(t, containsSubstring(ids, "mock-a"), "should contain mock-a")
	assert.True(t, containsSubstring(ids, "mock-b"), "should contain mock-b")
}

func TestDirLoader_DirectoryNotFound(t *testing.T) {
	loader := NewDirectoryLoader("/nonexistent/path/that/does/not/exist")
	result, err := loader.Load()

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "directory not found")
}

func TestDirLoader_PathIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "notadir.yaml")
	writeFile(t, filePath, validYAML("test-1", "/test"))

	loader := NewDirectoryLoader(filePath)
	result, err := loader.Load()

	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not a directory")
}

func TestDirLoader_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.FileCount)
	assert.Empty(t, result.Collection.Mocks)
	assert.Empty(t, result.Errors)
}

func TestDirLoader_RecursiveSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "root.yaml"), validYAML("root-mock", "/root"))
	writeFile(t, filepath.Join(tmpDir, "sub", "nested.yaml"), validYAML("nested-mock", "/nested"))

	loader := NewDirectoryLoader(tmpDir)
	loader.Recursive = true // default, but be explicit
	result, err := loader.Load()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.FileCount)
	assert.Len(t, result.Collection.Mocks, 2)
}

func TestDirLoader_NonRecursiveSkipsSubdirs(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "root.yaml"), validYAML("root-mock", "/root"))
	writeFile(t, filepath.Join(tmpDir, "sub", "nested.yaml"), validYAML("nested-mock", "/nested"))

	loader := NewDirectoryLoader(tmpDir)
	loader.Recursive = false
	result, err := loader.Load()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.FileCount, "non-recursive should only load root-level files")
	assert.Len(t, result.Collection.Mocks, 1)
	assert.Contains(t, result.Collection.Mocks[0].ID, "root-mock")
}

func TestDirLoader_NonYAMLFilesIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "valid.yaml"), validYAML("yaml-mock", "/yaml"))
	writeFile(t, filepath.Join(tmpDir, "readme.md"), "# Not a config file")
	writeFile(t, filepath.Join(tmpDir, "notes.txt"), "some notes")
	writeFile(t, filepath.Join(tmpDir, "data.csv"), "a,b,c\n1,2,3")

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.FileCount, "only YAML/JSON files should be loaded")
	assert.Len(t, result.Collection.Mocks, 1)
}

func TestDirLoader_YMLExtensionSupported(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "config.yml"), validYAML("yml-mock", "/yml"))

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.FileCount)
	assert.Len(t, result.Collection.Mocks, 1)
}

func TestDirLoader_JSONFileSupported(t *testing.T) {
	tmpDir := t.TempDir()
	content := `{
	"version": "1.0",
	"mocks": [
		{
			"id": "json-mock",
			"type": "http",
			"http": {
				"matcher": {"method": "GET", "path": "/json"},
				"response": {"statusCode": 200, "body": "ok"}
			}
		}
	]
}`
	writeFile(t, filepath.Join(tmpDir, "api.json"), content)

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.FileCount)
	assert.Len(t, result.Collection.Mocks, 1)
}

func TestDirLoader_SyntaxErrorReportsFilename(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "valid.yaml"), validYAML("valid-mock", "/valid"))
	writeFile(t, filepath.Join(tmpDir, "broken.yaml"), "version: \"1.0\"\nmocks:\n  - {invalid yaml content ::::")

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	// Load should succeed (non-fatal per-file errors), but report the broken file
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.FileCount, "only valid file should count")
	assert.Len(t, result.Collection.Mocks, 1)

	// The broken file should appear in Errors
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Path, "broken.yaml")
	assert.Contains(t, result.Errors[0].Message, "failed to load")
	assert.NotNil(t, result.Errors[0].Err)
}

func TestDirLoader_AllFilesInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "bad1.yaml"), "not: valid: yaml: [")
	writeFile(t, filepath.Join(tmpDir, "bad2.yaml"), "{also broken")

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err, "should not return fatal error when individual files fail")
	require.NotNil(t, result)
	assert.Equal(t, 0, result.FileCount)
	assert.Empty(t, result.Collection.Mocks)
	assert.Len(t, result.Errors, 2, "both bad files should be reported")
}

func TestDirLoader_MockIDsPrefixedWithRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "api.yaml"), validYAML("get-item", "/item"))

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	require.Len(t, result.Collection.Mocks, 1)

	// The ID should be prefixed with the filename (minus extension)
	mockID := result.Collection.Mocks[0].ID
	assert.Contains(t, mockID, "api-get-item",
		"mock ID should be prefixed with relative path: got %q", mockID)
}

func TestDirLoader_SubdirIDsUseRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "users", "api.yaml"), validYAML("list", "/users"))
	writeFile(t, filepath.Join(tmpDir, "products", "api.yaml"), validYAML("list", "/products"))

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	require.Len(t, result.Collection.Mocks, 2)

	ids := make(map[string]bool)
	for _, m := range result.Collection.Mocks {
		ids[m.ID] = true
	}
	assert.Len(t, ids, 2, "IDs must be unique even when original IDs are the same across subdirs")

	// Each ID should include its subdirectory in the prefix
	var hasUsers, hasProducts bool
	for _, m := range result.Collection.Mocks {
		if strings.Contains(m.ID, "users") {
			hasUsers = true
		}
		if strings.Contains(m.ID, "products") {
			hasProducts = true
		}
	}
	assert.True(t, hasUsers, "one mock ID should contain 'users' path component")
	assert.True(t, hasProducts, "one mock ID should contain 'products' path component")
}

func TestDirLoader_MockWithoutExplicitID(t *testing.T) {
	tmpDir := t.TempDir()
	// YAML with no id field — auto-generated ID should still get path prefix
	content := `version: "1.0"
mocks:
  - type: http
    http:
      matcher:
        method: GET
        path: /auto
      response:
        statusCode: 200
        body: "auto"
`
	writeFile(t, filepath.Join(tmpDir, "routes.yaml"), content)

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	require.Len(t, result.Collection.Mocks, 1)

	// The auto-generated ID (from fillMockDefaults) gets prefixed with the filename.
	// fillMockDefaults produces "http_<hash>", so the directory loader prefixes it
	// to "routes-http_<hash>".
	mockID := result.Collection.Mocks[0].ID
	assert.True(t, strings.HasPrefix(mockID, "routes-"),
		"auto-generated ID should include filename prefix: got %q", mockID)
}

// =============================================================================
// TestDirLoader — Validate() tests
// =============================================================================

func TestDirLoader_ValidateReportsErrors(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "good.yaml"), validYAML("ok-mock", "/ok"))
	writeFile(t, filepath.Join(tmpDir, "bad.yaml"), "not valid yaml [[[")

	loader := NewDirectoryLoader(tmpDir)
	errors, err := loader.Validate()

	require.NoError(t, err)
	require.Len(t, errors, 1)
	assert.Contains(t, errors[0].Path, "bad.yaml")
}

func TestDirLoader_ValidateAllValid(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "a.yaml"), validYAML("mock-a", "/a"))
	writeFile(t, filepath.Join(tmpDir, "b.yaml"), validYAML("mock-b", "/b"))

	loader := NewDirectoryLoader(tmpDir)
	errors, err := loader.Validate()

	require.NoError(t, err)
	assert.Empty(t, errors)
}

func TestDirLoader_ValidateEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	loader := NewDirectoryLoader(tmpDir)
	errors, err := loader.Validate()

	require.NoError(t, err)
	assert.Empty(t, errors)
}

// =============================================================================
// TestDirLoader — findConfigFiles() tests
// =============================================================================

func TestDirLoader_FindConfigFilesExtensions(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "a.yaml"), "")
	writeFile(t, filepath.Join(tmpDir, "b.yml"), "")
	writeFile(t, filepath.Join(tmpDir, "c.json"), "")
	writeFile(t, filepath.Join(tmpDir, "d.txt"), "")
	writeFile(t, filepath.Join(tmpDir, "e.md"), "")
	writeFile(t, filepath.Join(tmpDir, "f.YAML"), "") // uppercase

	loader := NewDirectoryLoader(tmpDir)
	files, err := loader.findConfigFiles()

	require.NoError(t, err)
	// .yaml, .yml, .json, and .YAML (case-insensitive check)
	assert.Len(t, files, 4, "should find .yaml, .yml, .json, and .YAML files")
}

// =============================================================================
// TestDirLoader — HasChanges() tests
// =============================================================================

func TestDirLoader_HasChangesDetectsModification(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	writeFile(t, cfgPath, validYAML("test-1", "/test"))

	loader := NewDirectoryLoader(tmpDir)
	_, err := loader.Load()
	require.NoError(t, err)

	// No changes initially
	changed, err := loader.HasChanges()
	require.NoError(t, err)
	assert.Empty(t, changed)

	// Modify the file with a small delay to ensure different mtime
	time.Sleep(10 * time.Millisecond)
	writeFile(t, cfgPath, validYAML("test-updated", "/updated"))

	changed, err = loader.HasChanges()
	require.NoError(t, err)
	assert.NotEmpty(t, changed)
	assert.Contains(t, changed, cfgPath)
}

func TestDirLoader_HasChangesDetectsDeleted(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	writeFile(t, cfgPath, validYAML("test-1", "/test"))

	loader := NewDirectoryLoader(tmpDir)
	_, err := loader.Load()
	require.NoError(t, err)

	// Delete the file
	require.NoError(t, os.Remove(cfgPath))

	changed, err := loader.HasChanges()
	require.NoError(t, err)
	assert.NotEmpty(t, changed)
	assert.Contains(t, changed, cfgPath)
}

// =============================================================================
// TestDirLoader — ReloadFile() tests
// =============================================================================

func TestDirLoader_ReloadFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")
	writeFile(t, cfgPath, validYAML("original", "/original"))

	loader := NewDirectoryLoader(tmpDir)
	_, err := loader.Load()
	require.NoError(t, err)

	// Modify file
	writeFile(t, cfgPath, validYAML("updated", "/updated"))

	collection, err := loader.ReloadFile(cfgPath)
	require.NoError(t, err)
	require.NotNil(t, collection)
	require.Len(t, collection.Mocks, 1)
	assert.Equal(t, "updated", collection.Mocks[0].ID)
}

func TestDirLoader_ReloadFileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewDirectoryLoader(tmpDir)

	_, err := loader.ReloadFile(filepath.Join(tmpDir, "nonexistent.yaml"))
	require.Error(t, err)
}

// =============================================================================
// TestDirLoader — NewDirectoryLoader defaults
// =============================================================================

func TestDirLoader_NewDirectoryLoaderDefaults(t *testing.T) {
	loader := NewDirectoryLoader("/some/path")

	assert.Equal(t, "/some/path", loader.Path)
	assert.True(t, loader.Recursive, "Recursive should default to true")
	assert.False(t, loader.ValidateOnly)
	assert.NotNil(t, loader.files)
}

// =============================================================================
// TestDirLoader — LoadResult metadata
// =============================================================================

func TestDirLoader_LoadResultCollectionName(t *testing.T) {
	tmpDir := t.TempDir()
	writeFile(t, filepath.Join(tmpDir, "api.yaml"), validYAML("test-1", "/test"))

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	assert.Contains(t, result.Collection.Name, tmpDir,
		"collection name should reference the loaded directory")
	assert.Equal(t, "1.0", result.Collection.Version)
}

func TestDirLoader_MergesServerConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// First file has server config
	content1 := `version: "1.0"
serverConfig:
  httpPort: 9090
  adminPort: 9091
mocks:
  - id: "mock-1"
    type: http
    http:
      matcher:
        method: GET
        path: /one
      response:
        statusCode: 200
`
	// Second file also has server config — should be ignored (first wins)
	content2 := `version: "1.0"
serverConfig:
  httpPort: 7070
  adminPort: 7071
mocks:
  - id: "mock-2"
    type: http
    http:
      matcher:
        method: GET
        path: /two
      response:
        statusCode: 200
`
	writeFile(t, filepath.Join(tmpDir, "a.yaml"), content1)
	writeFile(t, filepath.Join(tmpDir, "b.yaml"), content2)

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	require.NotNil(t, result.Collection.ServerConfig, "server config from first file should be used")
	// The first server config found should win (files are walked alphabetically)
	assert.Equal(t, 9090, result.Collection.ServerConfig.HTTPPort)
}

func TestDirLoader_MergesStatefulResources(t *testing.T) {
	tmpDir := t.TempDir()

	content1 := `version: "1.0"
mocks: []
statefulResources:
  - name: users
    idField: id
`
	content2 := `version: "1.0"
mocks: []
statefulResources:
  - name: products
    idField: id
`
	writeFile(t, filepath.Join(tmpDir, "a.yaml"), content1)
	writeFile(t, filepath.Join(tmpDir, "b.yaml"), content2)

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	assert.Len(t, result.Collection.StatefulResources, 2,
		"stateful resources from both files should be merged")
}

func TestDirLoader_MergesWebSocketEndpoints(t *testing.T) {
	tmpDir := t.TempDir()

	content1 := `version: "1.0"
mocks: []
websocketEndpoints:
  - path: /ws/chat
`
	content2 := `version: "1.0"
mocks: []
websocketEndpoints:
  - path: /ws/events
`
	writeFile(t, filepath.Join(tmpDir, "a.yaml"), content1)
	writeFile(t, filepath.Join(tmpDir, "b.yaml"), content2)

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()

	require.NoError(t, err)
	assert.Len(t, result.Collection.WebSocketEndpoints, 2,
		"websocket endpoints from both files should be merged")
}

// =============================================================================
// TestDirLoader — LoadError type tests
// =============================================================================

func TestDirLoader_LoadErrorWithWrappedErr(t *testing.T) {
	err := &LoadError{
		Path:    "/tmp/broken.yaml",
		Message: "failed to load",
		Err:     os.ErrNotExist,
	}

	s := err.Error()
	assert.Contains(t, s, "/tmp/broken.yaml")
	assert.Contains(t, s, "failed to load")
	assert.Contains(t, s, "file does not exist")
}

func TestDirLoader_LoadErrorWithoutWrappedErr(t *testing.T) {
	err := &LoadError{
		Path:    "/tmp/bad.yaml",
		Message: "validation failed",
	}

	s := err.Error()
	assert.Contains(t, s, "/tmp/bad.yaml")
	assert.Contains(t, s, "validation failed")
}

// =============================================================================
// Test helpers
// =============================================================================

// containsSubstring returns true if any element in items contains substr.
func containsSubstring(items []string, substr string) bool {
	for _, item := range items {
		if strings.Contains(item, substr) {
			return true
		}
	}
	return false
}
