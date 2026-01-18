package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromFile_ValidJSON(t *testing.T) {
	// Create a temp file with valid JSON
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "valid.json")

	content := `{
		"version": "1.0",
		"name": "test",
		"mocks": [
			{
				"id": "test-mock",
				"priority": 0,
				"enabled": true,
				"matcher": {
					"method": "GET",
					"path": "/test"
				},
				"response": {
					"statusCode": 200,
					"body": "hello"
				}
			}
		]
	}`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	collection, err := LoadFromFile(path)
	require.NoError(t, err)
	assert.NotNil(t, collection)
	assert.Equal(t, "1.0", collection.Version)
	assert.Equal(t, "test", collection.Name)
	assert.Len(t, collection.Mocks, 1)
	assert.Equal(t, "test-mock", collection.Mocks[0].ID)
}

func TestLoadFromFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid.json")

	content := `{ invalid json }`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	collection, err := LoadFromFile(path)
	assert.Error(t, err)
	assert.Nil(t, collection)
	assert.ErrorIs(t, err, ErrInvalidJSON)
}

func TestLoadFromFile_FileNotFound(t *testing.T) {
	collection, err := LoadFromFile("/nonexistent/path/file.json")
	assert.Error(t, err)
	assert.Nil(t, collection)
	assert.ErrorIs(t, err, ErrFileNotFound)
}

func TestLoadFromFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.json")

	err := os.WriteFile(path, []byte(""), 0644)
	require.NoError(t, err)

	collection, err := LoadFromFile(path)
	assert.Error(t, err)
	assert.Nil(t, collection)
	assert.ErrorIs(t, err, ErrEmptyFile)
}

func TestLoadFromFile_ValidationError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "invalid-version.json")

	// Valid JSON but invalid version
	content := `{
		"version": "2.0",
		"name": "test",
		"mocks": []
	}`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	collection, err := LoadFromFile(path)
	assert.Error(t, err)
	assert.Nil(t, collection)
	assert.Contains(t, err.Error(), "validation")
}

func TestSaveToFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "output.json")

	collection := &MockCollection{
		Version: "1.0",
		Name:    "test-save",
		Mocks: []*MockConfiguration{
			{
				ID:      "save-mock",
				Enabled: true,
				Type:    mock.MockTypeHTTP,
				HTTP: &mock.HTTPSpec{
					Priority: 5,
					Matcher: &mock.HTTPMatcher{
						Method: "POST",
						Path:   "/save",
					},
					Response: &mock.HTTPResponse{
						StatusCode: 201,
						Body:       "created",
					},
				},
			},
		},
	}

	err := SaveToFile(path, collection)
	require.NoError(t, err)

	// Verify file exists and contains valid JSON
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "save-mock")
	assert.Contains(t, string(data), "1.0")
}

func TestSaveToFile_CreateDir(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "subdir", "nested", "output.json")

	collection := &MockCollection{
		Version: "1.0",
		Name:    "test",
		Mocks:   []*MockConfiguration{},
	}

	err := SaveToFile(path, collection)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestSaveToFile_NilCollection(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nil.json")

	err := SaveToFile(path, nil)
	assert.Error(t, err)
}

func TestSaveLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "roundtrip.json")

	original := &MockCollection{
		Version: "1.0",
		Name:    "roundtrip-test",
		Mocks: []*MockConfiguration{
			{
				ID:      "mock-1",
				Enabled: true,
				Type:    mock.MockTypeHTTP,
				HTTP: &mock.HTTPSpec{
					Priority: 10,
					Matcher: &mock.HTTPMatcher{
						Method: "GET",
						Path:   "/api/users",
						Headers: map[string]string{
							"Authorization": "Bearer token",
						},
					},
					Response: &mock.HTTPResponse{
						StatusCode: 200,
						Headers: map[string]string{
							"Content-Type": "application/json",
						},
						Body: `{"users": []}`,
					},
				},
			},
			{
				ID:      "mock-2",
				Enabled: false,
				Type:    mock.MockTypeHTTP,
				HTTP: &mock.HTTPSpec{
					Priority: 5,
					Matcher: &mock.HTTPMatcher{
						Method: "POST",
						Path:   "/api/users",
					},
					Response: &mock.HTTPResponse{
						StatusCode: 201,
						Body:       "created",
					},
				},
			},
		},
	}

	// Save
	err := SaveToFile(path, original)
	require.NoError(t, err)

	// Load
	loaded, err := LoadFromFile(path)
	require.NoError(t, err)

	// Compare
	assert.Equal(t, original.Version, loaded.Version)
	assert.Equal(t, original.Name, loaded.Name)
	assert.Len(t, loaded.Mocks, 2)
	assert.Equal(t, "mock-1", loaded.Mocks[0].ID)
	assert.Equal(t, 10, loaded.Mocks[0].HTTP.Priority)
	assert.Equal(t, "mock-2", loaded.Mocks[1].ID)
}

func TestParseJSON_Valid(t *testing.T) {
	data := []byte(`{
		"version": "1.0",
		"name": "parse-test",
		"mocks": []
	}`)

	collection, err := ParseJSON(data)
	require.NoError(t, err)
	assert.Equal(t, "1.0", collection.Version)
	assert.Equal(t, "parse-test", collection.Name)
}

func TestParseJSON_Invalid(t *testing.T) {
	data := []byte(`{ invalid }`)

	collection, err := ParseJSON(data)
	assert.Error(t, err)
	assert.Nil(t, collection)
}

func TestToJSON(t *testing.T) {
	collection := &MockCollection{
		Version: "1.0",
		Name:    "to-json",
		Mocks:   []*MockConfiguration{},
	}

	data, err := ToJSON(collection)
	require.NoError(t, err)
	assert.Contains(t, string(data), "1.0")
	assert.Contains(t, string(data), "to-json")
}

func TestToJSON_Nil(t *testing.T) {
	data, err := ToJSON(nil)
	assert.Error(t, err)
	assert.Nil(t, data)
}

func TestLoadMocksFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "mocks.json")

	content := `{
		"version": "1.0",
		"name": "test",
		"mocks": [
			{
				"id": "m1",
				"matcher": {"path": "/a"},
				"response": {"statusCode": 200}
			},
			{
				"id": "m2",
				"matcher": {"path": "/b"},
				"response": {"statusCode": 201}
			}
		]
	}`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	mocks, err := LoadMocksFromFile(path)
	require.NoError(t, err)
	assert.Len(t, mocks, 2)
}

func TestSaveMocksToFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "save-mocks.json")

	mocks := []*MockConfiguration{
		{
			ID:      "sm1",
			Enabled: true,
			Type:    mock.MockTypeHTTP,
			HTTP: &mock.HTTPSpec{
				Matcher: &mock.HTTPMatcher{
					Path: "/test",
				},
				Response: &mock.HTTPResponse{
					StatusCode: 200,
				},
			},
		},
	}

	err := SaveMocksToFile(path, mocks, "save-mocks-test")
	require.NoError(t, err)

	// Verify
	loaded, err := LoadFromFile(path)
	require.NoError(t, err)
	assert.Equal(t, "save-mocks-test", loaded.Name)
	assert.Len(t, loaded.Mocks, 1)
}
