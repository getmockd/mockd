package config

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// REGRESSION TESTS - Bug Fixes
// =============================================================================

// TestDirectoryLoader_SubdirSameFilename_UniqueIDs verifies that the same filename
// in different subdirectories produces unique IDs (Bug 3.4 fix).
func TestDirectoryLoader_SubdirSameFilename_UniqueIDs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create subdirectories: users/ and products/
	usersDir := filepath.Join(tmpDir, "users")
	productsDir := filepath.Join(tmpDir, "products")
	require.NoError(t, os.MkdirAll(usersDir, 0755))
	require.NoError(t, os.MkdirAll(productsDir, 0755))

	// Create api.yaml in both directories with the same mock ID
	usersYAML := `version: "1.0"
mocks:
  - id: "get-item"
    type: http
    http:
      matcher:
        method: GET
        path: /users
      response:
        statusCode: 200
        body: "users list"
`
	productsYAML := `version: "1.0"
mocks:
  - id: "get-item"
    type: http
    http:
      matcher:
        method: GET
        path: /products
      response:
        statusCode: 200
        body: "products list"
`
	require.NoError(t, os.WriteFile(filepath.Join(usersDir, "api.yaml"), []byte(usersYAML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(productsDir, "api.yaml"), []byte(productsYAML), 0644))

	// Load directory
	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Collection.Mocks, 2, "Should load mocks from both files")

	// Extract IDs
	ids := make(map[string]bool)
	for _, mock := range result.Collection.Mocks {
		ids[mock.ID] = true
	}

	// Verify IDs are unique (no collision)
	assert.Len(t, ids, 2, "IDs should be unique even with same filename in different subdirs")

	// Verify the IDs include path prefix to distinguish them
	var foundUsers, foundProducts bool
	for _, mock := range result.Collection.Mocks {
		if strings.Contains(mock.ID, "users") {
			foundUsers = true
		}
		if strings.Contains(mock.ID, "products") {
			foundProducts = true
		}
	}
	assert.True(t, foundUsers || foundProducts, "IDs should include path-based prefix")
}

// TestWatcher_StartStopStart_Works verifies watcher can be restarted (Bug 2.3 fix).
func TestWatcher_StartStopStart_Works(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")
	content := `version: "1.0"
mocks: []
`
	require.NoError(t, os.WriteFile(configFile, []byte(content), 0644))

	loader := NewDirectoryLoader(tmpDir)
	_, err := loader.Load()
	require.NoError(t, err)

	watcher := NewWatcher(loader)

	// First start
	ch1 := watcher.Start()
	require.NotNil(t, ch1)

	// Stop
	watcher.Stop()

	// Second start - this should work without panic or deadlock
	ch2 := watcher.Start()
	require.NotNil(t, ch2)

	// Clean up
	watcher.Stop()
}

// TestWatcher_ConcurrentStartStop_NoRace tests for race conditions in Start/Stop (Bug 3.5 fix).
func TestWatcher_ConcurrentStartStop_NoRace(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")
	content := `version: "1.0"
mocks: []
`
	require.NoError(t, os.WriteFile(configFile, []byte(content), 0644))

	loader := NewDirectoryLoader(tmpDir)
	_, err := loader.Load()
	require.NoError(t, err)

	watcher := NewWatcher(loader)

	// Run rapid Start/Stop cycles concurrently
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			watcher.Start()
		}()
		go func() {
			defer wg.Done()
			watcher.Stop()
		}()
	}

	// Use a channel with timeout to avoid hanging
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("Test timed out - possible deadlock in concurrent Start/Stop")
	}

	// Final cleanup
	watcher.Stop()
}

// TestValidator_ManagementPort_ConflictsWithHTTP verifies managementPort conflict check (Bug 2.1 fix).
func TestValidator_ManagementPort_ConflictsWithHTTP(t *testing.T) {
	cfg := &ServerConfiguration{
		HTTPPort:       8080,
		AdminPort:      8081,
		ManagementPort: 8080, // Same as HTTPPort
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "managementPort")
	assert.Contains(t, err.Error(), "conflicts")
}

// TestValidator_ManagementPort_ConflictsWithHTTPS verifies managementPort conflict with HTTPS.
func TestValidator_ManagementPort_ConflictsWithHTTPS(t *testing.T) {
	cfg := &ServerConfiguration{
		HTTPPort:       8080,
		HTTPSPort:      8443,
		AdminPort:      8081,
		ManagementPort: 8443, // Same as HTTPSPort
		TLS: &TLSConfig{
			Enabled:          true,
			AutoGenerateCert: true,
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "managementPort")
	assert.Contains(t, err.Error(), "conflicts")
}

// TestValidator_ManagementPort_ConflictsWithAdmin verifies managementPort conflict with admin.
func TestValidator_ManagementPort_ConflictsWithAdmin(t *testing.T) {
	cfg := &ServerConfiguration{
		HTTPPort:       8080,
		AdminPort:      8081,
		ManagementPort: 8081, // Same as AdminPort
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "managementPort")
	assert.Contains(t, err.Error(), "conflicts")
}

// TestValidator_HTTPSPort_RequiresTLS verifies HTTPS port requires TLS enabled (Bug 2.2 fix).
func TestValidator_HTTPSPort_RequiresTLS(t *testing.T) {
	cfg := &ServerConfiguration{
		HTTPSPort: 8443,
		AdminPort: 8081,
		TLS:       nil, // No TLS config
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TLS")
}

// TestValidator_HTTPSPort_RequiresCertWhenNotAutoGenerate verifies cert requirement.
func TestValidator_HTTPSPort_RequiresCertWhenNotAutoGenerate(t *testing.T) {
	cfg := &ServerConfiguration{
		HTTPSPort: 8443,
		AdminPort: 8081,
		TLS: &TLSConfig{
			Enabled:          true,
			AutoGenerateCert: false,
			// Missing CertFile and KeyFile
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "certFile")
}

// =============================================================================
// DIRECTORY LOADER TESTS
// =============================================================================

func TestDirectoryLoader_LoadsYAMLFiles(t *testing.T) {
	tmpDir := t.TempDir()
	content := `version: "1.0"
mocks:
  - id: "yaml-mock"
    type: http
    http:
      matcher:
        method: GET
        path: /yaml
      response:
        statusCode: 200
        body: "yaml response"
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.yaml"), []byte(content), 0644))

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.FileCount)
	assert.Len(t, result.Collection.Mocks, 1)
}

func TestDirectoryLoader_LoadsJSONFiles(t *testing.T) {
	tmpDir := t.TempDir()
	content := `{
	"version": "1.0",
	"mocks": [
		{
			"id": "json-mock",
			"enabled": true,
			"matcher": {
				"method": "GET",
				"path": "/json"
			},
			"response": {
				"statusCode": 200,
				"body": "json response"
			}
		}
	]
}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "test.json"), []byte(content), 0644))

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.FileCount)
	assert.Len(t, result.Collection.Mocks, 1)
}

func TestDirectoryLoader_MergesMultipleFiles(t *testing.T) {
	tmpDir := t.TempDir()

	yaml1 := `version: "1.0"
mocks:
  - id: "mock-1"
    type: http
    http:
      matcher:
        path: /one
      response:
        statusCode: 200
`
	yaml2 := `version: "1.0"
mocks:
  - id: "mock-2"
    type: http
    http:
      matcher:
        path: /two
      response:
        statusCode: 201
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "a.yaml"), []byte(yaml1), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "b.yaml"), []byte(yaml2), 0644))

	loader := NewDirectoryLoader(tmpDir)
	result, err := loader.Load()
	require.NoError(t, err)
	assert.Equal(t, 2, result.FileCount)
	assert.Len(t, result.Collection.Mocks, 2)
}

func TestDirectoryLoader_HasChanges_DetectsModification(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")
	content := `version: "1.0"
mocks: []
`
	require.NoError(t, os.WriteFile(configFile, []byte(content), 0644))

	loader := NewDirectoryLoader(tmpDir)
	_, err := loader.Load()
	require.NoError(t, err)

	// Initially no changes
	changed, err := loader.HasChanges()
	require.NoError(t, err)
	assert.Empty(t, changed)

	// Wait a bit and modify the file
	time.Sleep(10 * time.Millisecond)
	newContent := `version: "1.0"
mocks:
  - id: "new-mock"
    type: http
    http:
      matcher:
        path: /new
      response:
        statusCode: 200
`
	require.NoError(t, os.WriteFile(configFile, []byte(newContent), 0644))

	// Now should detect changes
	changed, err = loader.HasChanges()
	require.NoError(t, err)
	assert.NotEmpty(t, changed)
	assert.Contains(t, changed, configFile)
}

func TestDirectoryLoader_NotFound(t *testing.T) {
	loader := NewDirectoryLoader("/nonexistent/path/that/does/not/exist")
	result, err := loader.Load()
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "directory not found")
}

func TestDirectoryLoader_NonRecursive(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	rootYAML := `version: "1.0"
mocks:
  - id: "root-mock"
    type: http
    http:
      matcher:
        path: /root
      response:
        statusCode: 200
`
	subYAML := `version: "1.0"
mocks:
  - id: "sub-mock"
    type: http
    http:
      matcher:
        path: /sub
      response:
        statusCode: 200
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "root.yaml"), []byte(rootYAML), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "sub.yaml"), []byte(subYAML), 0644))

	loader := NewDirectoryLoader(tmpDir)
	loader.Recursive = false

	result, err := loader.Load()
	require.NoError(t, err)
	assert.Equal(t, 1, result.FileCount, "Should only load root level files")
}

// =============================================================================
// VALIDATOR TESTS - Port Ranges and Conflicts
// =============================================================================

func TestValidator_PortRange_Valid(t *testing.T) {
	tests := []struct {
		name string
		cfg  *ServerConfiguration
	}{
		{
			name: "minimum valid ports",
			cfg: &ServerConfiguration{
				HTTPPort:  1,
				AdminPort: 2,
			},
		},
		{
			name: "maximum valid ports",
			cfg: &ServerConfiguration{
				HTTPPort:  65535,
				AdminPort: 65534,
			},
		},
		{
			name: "typical ports",
			cfg: &ServerConfiguration{
				HTTPPort:  8080,
				HTTPSPort: 8443,
				AdminPort: 9090,
				TLS: &TLSConfig{
					Enabled:          true,
					AutoGenerateCert: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestValidator_PortRange_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		cfg    *ServerConfiguration
		errMsg string
	}{
		{
			name: "negative HTTP port",
			cfg: &ServerConfiguration{
				HTTPPort:  -1,
				AdminPort: 8081,
			},
			errMsg: "httpPort",
		},
		{
			name: "HTTP port too high",
			cfg: &ServerConfiguration{
				HTTPPort:  65536,
				AdminPort: 8081,
			},
			errMsg: "httpPort",
		},
		{
			name: "negative admin port",
			cfg: &ServerConfiguration{
				HTTPPort:  8080,
				AdminPort: -1,
			},
			errMsg: "adminPort",
		},
		{
			name: "admin port too high",
			cfg: &ServerConfiguration{
				HTTPPort:  8080,
				AdminPort: 65536,
			},
			errMsg: "adminPort",
		},
		{
			name: "zero admin port",
			cfg: &ServerConfiguration{
				HTTPPort:  8080,
				AdminPort: 0,
			},
			errMsg: "adminPort",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestValidator_PortConflict_HTTPAndHTTPS(t *testing.T) {
	cfg := &ServerConfiguration{
		HTTPPort:  8080,
		HTTPSPort: 8080, // Same as HTTP
		AdminPort: 8081,
		TLS: &TLSConfig{
			Enabled:          true,
			AutoGenerateCert: true,
		},
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "httpsPort")
	assert.Contains(t, err.Error(), "conflicts")
}

func TestValidator_PortConflict_HTTPAndAdmin(t *testing.T) {
	cfg := &ServerConfiguration{
		HTTPPort:  8080,
		AdminPort: 8080, // Same as HTTP
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "adminPort")
	assert.Contains(t, err.Error(), "conflicts")
}

func TestValidator_PortConflict_AllPorts(t *testing.T) {
	tests := []struct {
		name      string
		http      int
		https     int
		admin     int
		mgmt      int
		expectErr bool
	}{
		{"all different", 8080, 8443, 8081, 8082, false},
		{"http=https", 8080, 8080, 8081, 8082, true},
		{"http=admin", 8080, 8443, 8080, 8082, true},
		{"http=mgmt", 8080, 8443, 8081, 8080, true},
		{"https=admin", 8080, 8443, 8443, 8082, true},
		{"https=mgmt", 8080, 8443, 8081, 8443, true},
		{"admin=mgmt", 8080, 8443, 8081, 8081, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ServerConfiguration{
				HTTPPort:       tt.http,
				HTTPSPort:      tt.https,
				AdminPort:      tt.admin,
				ManagementPort: tt.mgmt,
				TLS: &TLSConfig{
					Enabled:          true,
					AutoGenerateCert: true,
				},
			}

			err := cfg.Validate()
			if tt.expectErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "conflicts")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// =============================================================================
// VALIDATOR TESTS - TLS Configuration
// =============================================================================

func TestValidator_TLS_AutoGenerate_NoCertRequired(t *testing.T) {
	cfg := &ServerConfiguration{
		HTTPSPort: 8443,
		AdminPort: 8081,
		TLS: &TLSConfig{
			Enabled:          true,
			AutoGenerateCert: true,
			// No CertFile or KeyFile needed
		},
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestValidator_TLS_Manual_RequiresCertAndKey(t *testing.T) {
	tests := []struct {
		name     string
		certFile string
		keyFile  string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "missing both",
			certFile: "",
			keyFile:  "",
			wantErr:  true,
			errMsg:   "certFile",
		},
		{
			name:     "missing keyFile",
			certFile: "/path/to/cert.pem",
			keyFile:  "",
			wantErr:  true,
			errMsg:   "keyFile",
		},
		{
			name:     "missing certFile",
			certFile: "",
			keyFile:  "/path/to/key.pem",
			wantErr:  true,
			errMsg:   "certFile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ServerConfiguration{
				HTTPSPort: 8443,
				AdminPort: 8081,
				TLS: &TLSConfig{
					Enabled:          true,
					AutoGenerateCert: false,
					CertFile:         tt.certFile,
					KeyFile:          tt.keyFile,
				},
			}

			err := cfg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_TLS_Disabled_HTTPSPortAllowed(t *testing.T) {
	cfg := &ServerConfiguration{
		HTTPPort:  8080,
		HTTPSPort: 8443,
		AdminPort: 8081,
		TLS: &TLSConfig{
			Enabled: false, // TLS disabled
		},
	}

	err := cfg.Validate()
	require.Error(t, err, "HTTPS port without enabled TLS should fail")
	assert.Contains(t, err.Error(), "TLS")
}

// =============================================================================
// WATCHER TESTS
// =============================================================================

func TestWatcher_EmitsEventsOnChange(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")
	content := `version: "1.0"
mocks: []
`
	require.NoError(t, os.WriteFile(configFile, []byte(content), 0644))

	loader := NewDirectoryLoader(tmpDir)
	_, err := loader.Load()
	require.NoError(t, err)

	watcher := &Watcher{
		loader:   loader,
		interval: 50 * time.Millisecond, // Short interval for testing
		stopCh:   make(chan struct{}),
		eventCh:  make(chan WatchEvent, 10),
	}

	ch := watcher.Start()
	require.NotNil(t, ch)

	// Modify file
	time.Sleep(20 * time.Millisecond)
	newContent := `version: "1.0"
mocks:
  - id: "changed"
    type: http
    http:
      matcher:
        path: /changed
      response:
        statusCode: 200
`
	require.NoError(t, os.WriteFile(configFile, []byte(newContent), 0644))

	// Wait for event
	select {
	case event := <-ch:
		assert.Equal(t, "modified", event.Type)
		assert.Contains(t, event.Path, "test.yaml")
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Timed out waiting for watch event")
	}

	watcher.Stop()
}

func TestWatcher_Stop_StopsWatching(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")
	content := `version: "1.0"
mocks: []
`
	require.NoError(t, os.WriteFile(configFile, []byte(content), 0644))

	loader := NewDirectoryLoader(tmpDir)
	_, err := loader.Load()
	require.NoError(t, err)

	watcher := NewWatcher(loader)
	watcher.Start()

	// Stop should complete without blocking
	done := make(chan struct{})
	go func() {
		watcher.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Stop() blocked - possible deadlock")
	}
}

func TestWatcher_MultipleStops_Safe(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "test.yaml")
	content := `version: "1.0"
mocks: []
`
	require.NoError(t, os.WriteFile(configFile, []byte(content), 0644))

	loader := NewDirectoryLoader(tmpDir)
	_, err := loader.Load()
	require.NoError(t, err)

	watcher := NewWatcher(loader)
	watcher.Start()

	// Multiple stops should not panic
	assert.NotPanics(t, func() {
		watcher.Stop()
		watcher.Stop()
		watcher.Stop()
	})
}

func TestWatcher_StopWithoutStart_Safe(t *testing.T) {
	tmpDir := t.TempDir()
	loader := NewDirectoryLoader(tmpDir)
	watcher := NewWatcher(loader)

	// Stop without Start should not panic
	assert.NotPanics(t, func() {
		watcher.Stop()
	})
}

// =============================================================================
// ADDITIONAL VALIDATION TESTS
// =============================================================================

func TestValidator_NoPortsConfigured(t *testing.T) {
	cfg := &ServerConfiguration{
		HTTPPort:  0,
		HTTPSPort: 0,
		AdminPort: 8081,
	}

	err := cfg.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one of httpPort or httpsPort")
}

func TestValidator_MaxBodySize(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		wantErr  bool
		errField string
	}{
		{"zero is valid", 0, false, ""},
		{"positive is valid", 1024, false, ""},
		{"max valid", 100 * 1024 * 1024, false, ""},
		{"negative is invalid", -1, true, "maxBodySize"},
		{"too large", 100*1024*1024 + 1, true, "maxBodySize"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ServerConfiguration{
				HTTPPort:    8080,
				AdminPort:   8081,
				MaxBodySize: tt.size,
			}

			err := cfg.Validate()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errField)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidator_Timeouts(t *testing.T) {
	tests := []struct {
		name         string
		readTimeout  int
		writeTimeout int
		wantErr      bool
	}{
		{"zero timeouts valid", 0, 0, false},
		{"positive timeouts valid", 30, 30, false},
		{"negative read timeout invalid", -1, 30, true},
		{"negative write timeout invalid", 30, -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ServerConfiguration{
				HTTPPort:     8080,
				AdminPort:    8081,
				ReadTimeout:  tt.readTimeout,
				WriteTimeout: tt.writeTimeout,
			}

			err := cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMockCollection_Validate_DuplicateIDs(t *testing.T) {
	collection := &MockCollection{
		Version: "1.0",
		Mocks: []*MockConfiguration{
			{
				ID:      "duplicate-id",
				Enabled: true,
				Type:    "http",
				HTTP: &mock.HTTPSpec{
					Matcher:  &mock.HTTPMatcher{Path: "/test1"},
					Response: &mock.HTTPResponse{StatusCode: 200},
				},
			},
			{
				ID:      "duplicate-id",
				Enabled: true,
				Type:    "http",
				HTTP: &mock.HTTPSpec{
					Matcher:  &mock.HTTPMatcher{Path: "/test2"},
					Response: &mock.HTTPResponse{StatusCode: 200},
				},
			},
		},
	}

	err := collection.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestMockCollection_Validate_InvalidVersion(t *testing.T) {
	collection := &MockCollection{
		Version: "2.0",
		Mocks:   []*MockConfiguration{},
	}

	err := collection.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported version")
}

// =============================================================================
// LOAD ERROR TESTS
// =============================================================================

func TestLoadError_Error(t *testing.T) {
	err := &LoadError{
		Path:    "/path/to/file.yaml",
		Message: "failed to load",
		Err:     os.ErrNotExist,
	}

	errStr := err.Error()
	assert.Contains(t, errStr, "/path/to/file.yaml")
	assert.Contains(t, errStr, "failed to load")
	assert.Contains(t, errStr, "file does not exist")
}

func TestLoadError_Error_NoWrappedErr(t *testing.T) {
	err := &LoadError{
		Path:    "/path/to/file.yaml",
		Message: "validation failed",
		Err:     nil,
	}

	errStr := err.Error()
	assert.Contains(t, errStr, "/path/to/file.yaml")
	assert.Contains(t, errStr, "validation failed")
}
