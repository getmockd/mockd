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

	enabled := true
	collection := &MockCollection{
		Version: "1.0",
		Name:    "test-save",
		Mocks: []*MockConfiguration{
			{
				ID:      "save-mock",
				Enabled: &enabled,
				Type:    mock.TypeHTTP,
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

func boolPtr(b bool) *bool { return &b }

func TestSaveLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "roundtrip.json")

	original := &MockCollection{
		Version: "1.0",
		Name:    "roundtrip-test",
		Mocks: []*MockConfiguration{
			{
				ID:      "mock-1",
				Enabled: boolPtr(true),
				Type:    mock.TypeHTTP,
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
				Enabled: boolPtr(false),
				Type:    mock.TypeHTTP,
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

// ============================================================================
// DX-1: Auto-generate IDs and default types in config files
// ============================================================================

func TestParseYAML_AutoGeneratesID(t *testing.T) {
	// Config with no id field — should auto-generate
	data := []byte(`
version: "1.0"
mocks:
  - type: http
    http:
      matcher:
        method: GET
        path: /test
      response:
        statusCode: 200
        body: hello
`)
	collection, err := ParseYAML(data)
	require.NoError(t, err)
	require.Len(t, collection.Mocks, 1)

	// ID should be auto-generated with "http_" prefix
	assert.NotEmpty(t, collection.Mocks[0].ID)
	assert.True(t, len(collection.Mocks[0].ID) > 5, "auto-generated ID should be longer than prefix")
	assert.Contains(t, collection.Mocks[0].ID, "http_")
}

func TestParseJSON_AutoGeneratesID(t *testing.T) {
	data := []byte(`{
		"version": "1.0",
		"mocks": [
			{
				"type": "http",
				"http": {
					"matcher": {"method": "GET", "path": "/test"},
					"response": {"statusCode": 200, "body": "hello"}
				}
			}
		]
	}`)
	collection, err := ParseJSON(data)
	require.NoError(t, err)
	require.Len(t, collection.Mocks, 1)

	assert.NotEmpty(t, collection.Mocks[0].ID)
	assert.Contains(t, collection.Mocks[0].ID, "http_")
}

func TestParseYAML_DefaultsTypeToHTTP(t *testing.T) {
	// Config with no type field but HTTP spec present — should default to "http"
	data := []byte(`
version: "1.0"
mocks:
  - http:
      matcher:
        method: GET
        path: /no-type
      response:
        statusCode: 200
`)
	collection, err := ParseYAML(data)
	require.NoError(t, err)
	require.Len(t, collection.Mocks, 1)

	assert.Equal(t, mock.TypeHTTP, collection.Mocks[0].Type)
	assert.NotEmpty(t, collection.Mocks[0].ID)
	assert.Contains(t, collection.Mocks[0].ID, "http_")
}

func TestParseYAML_InfersTypeFromSpec(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		expectedType mock.Type
		idPrefix     string
	}{
		{
			name: "graphql",
			yaml: `
version: "1.0"
mocks:
  - graphql:
      path: /graphql
      schema: "type Query { hello: String }"
`,
			expectedType: mock.TypeGraphQL,
			idPrefix:     "gql_",
		},
		{
			name: "websocket",
			yaml: `
version: "1.0"
mocks:
  - websocket:
      path: /ws
`,
			expectedType: mock.TypeWebSocket,
			idPrefix:     "ws_",
		},
		{
			name: "grpc",
			yaml: `
version: "1.0"
mocks:
  - grpc:
      port: 50051
      protoFile: service.proto
      services:
        test.Svc:
          methods:
            Get:
              response: {}
`,
			expectedType: mock.TypeGRPC,
			idPrefix:     "grpc_",
		},
		{
			name: "soap",
			yaml: `
version: "1.0"
mocks:
  - soap:
      path: /soap
      operations:
        GetWeather:
          response: "<Temp>72</Temp>"
`,
			expectedType: mock.TypeSOAP,
			idPrefix:     "soap_",
		},
		{
			name: "mqtt",
			yaml: `
version: "1.0"
mocks:
  - mqtt:
      port: 1883
      topics:
        - topic: sensors/temp
          messages:
            - payload: '{"temp": 72}'
`,
			expectedType: mock.TypeMQTT,
			idPrefix:     "mqtt_",
		},
		{
			name: "oauth",
			yaml: `
version: "1.0"
mocks:
  - oauth:
      issuer: http://localhost:9999
      clients:
        - clientId: app
          clientSecret: secret
`,
			expectedType: mock.TypeOAuth,
			idPrefix:     "oauth_",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collection, err := ParseYAML([]byte(tt.yaml))
			require.NoError(t, err)
			require.Len(t, collection.Mocks, 1)

			assert.Equal(t, tt.expectedType, collection.Mocks[0].Type, "type should be inferred from spec")
			assert.NotEmpty(t, collection.Mocks[0].ID)
			assert.Contains(t, collection.Mocks[0].ID, tt.idPrefix, "ID should have correct prefix")
		})
	}
}

func TestParseYAML_PreservesExplicitID(t *testing.T) {
	// If id is provided, it should NOT be overwritten
	data := []byte(`
version: "1.0"
mocks:
  - id: my-custom-id
    type: http
    http:
      matcher:
        method: GET
        path: /keep-id
      response:
        statusCode: 200
`)
	collection, err := ParseYAML(data)
	require.NoError(t, err)
	require.Len(t, collection.Mocks, 1)

	assert.Equal(t, "my-custom-id", collection.Mocks[0].ID)
}

func TestParseYAML_PreservesExplicitType(t *testing.T) {
	// If type is provided, it should NOT be overwritten
	data := []byte(`
version: "1.0"
mocks:
  - type: http
    http:
      matcher:
        method: GET
        path: /keep-type
      response:
        statusCode: 200
`)
	collection, err := ParseYAML(data)
	require.NoError(t, err)
	require.Len(t, collection.Mocks, 1)

	assert.Equal(t, mock.TypeHTTP, collection.Mocks[0].Type)
}

func TestParseYAML_MultipleMocksGetUniqueIDs(t *testing.T) {
	data := []byte(`
version: "1.0"
mocks:
  - type: http
    http:
      matcher:
        method: GET
        path: /one
      response:
        statusCode: 200
  - type: http
    http:
      matcher:
        method: GET
        path: /two
      response:
        statusCode: 200
`)
	collection, err := ParseYAML(data)
	require.NoError(t, err)
	require.Len(t, collection.Mocks, 2)

	// Both should have IDs
	assert.NotEmpty(t, collection.Mocks[0].ID)
	assert.NotEmpty(t, collection.Mocks[1].ID)

	// IDs should be different
	assert.NotEqual(t, collection.Mocks[0].ID, collection.Mocks[1].ID,
		"auto-generated IDs for different mocks should be unique")
}

func TestParseYAML_MixedExplicitAndAutoID(t *testing.T) {
	data := []byte(`
version: "1.0"
mocks:
  - id: explicit-1
    type: http
    http:
      matcher:
        method: GET
        path: /explicit
      response:
        statusCode: 200
  - type: http
    http:
      matcher:
        method: GET
        path: /auto
      response:
        statusCode: 200
`)
	collection, err := ParseYAML(data)
	require.NoError(t, err)
	require.Len(t, collection.Mocks, 2)

	assert.Equal(t, "explicit-1", collection.Mocks[0].ID)
	assert.NotEmpty(t, collection.Mocks[1].ID)
	assert.NotEqual(t, "explicit-1", collection.Mocks[1].ID)
}

func TestLoadFromFile_YAML_AutoGeneratesID(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "no-id.yaml")

	content := `
version: "1.0"
mocks:
  - type: http
    http:
      matcher:
        method: GET
        path: /file-test
      response:
        statusCode: 200
        body: works
`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	collection, err := LoadFromFile(path)
	require.NoError(t, err)
	require.Len(t, collection.Mocks, 1)

	assert.NotEmpty(t, collection.Mocks[0].ID)
	assert.Equal(t, mock.TypeHTTP, collection.Mocks[0].Type)
}

func TestFillMockDefaults_NilMock(t *testing.T) {
	// Should not panic on nil mocks
	collection := &MockCollection{
		Version: "1.0",
		Mocks:   []*MockConfiguration{nil},
	}
	// fillMockDefaults should skip nil entries gracefully
	fillMockDefaults(collection)
	// nil entry should remain nil
	assert.Nil(t, collection.Mocks[0])
}

func TestInferMockType_NoSpec(t *testing.T) {
	// When no spec field is set, default to HTTP
	m := &MockConfiguration{}
	result := inferMockType(m)
	assert.Equal(t, mock.TypeHTTP, result)
}

func TestSaveMocksToFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "save-mocks.json")

	mocks := []*MockConfiguration{
		{
			ID:      "sm1",
			Enabled: boolPtr(true),
			Type:    mock.TypeHTTP,
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
