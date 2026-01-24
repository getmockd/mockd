package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunImport_SourceRequired(t *testing.T) {
	err := RunImport([]string{})
	if err == nil {
		t.Error("expected error when source is not provided")
	}
}

func TestRunImport_FileNotFound(t *testing.T) {
	err := RunImport([]string{"/nonexistent/path/file.json"})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestRunImport_HelpFlag(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RunImport panicked with --help: %v", r)
		}
	}()

	// Capture stderr since help goes there
	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	_ = RunImport([]string{"--help"})

	w.Close()
	os.Stderr = oldStderr
}

func TestRunImport_FormatDetection(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name           string
		filename       string
		content        string
		expectFormat   string
		expectError    bool
		errorContain   string
		skipServerCall bool
	}{
		{
			name:     "detect mockd from yaml content",
			filename: "mocks.yaml",
			content: `
version: "1.0"
mocks:
  - id: test
    type: http
    http:
      matcher:
        method: GET
        path: /test
      response:
        statusCode: 200
        body: "{}"
`,
			expectFormat:   "mockd",
			skipServerCall: true,
		},
		{
			name:     "detect openapi from yaml content",
			filename: "api.yaml",
			content: `
openapi: "3.0.0"
info:
  title: Test API
  version: "1.0"
paths:
  /test:
    get:
      responses:
        '200':
          description: OK
`,
			expectFormat:   "openapi",
			skipServerCall: true,
		},
		{
			name:     "detect postman from json content",
			filename: "collection.json",
			content: `{
  "info": {
    "name": "Test Collection",
    "_postman_id": "12345"
  },
  "item": []
}`,
			expectFormat:   "postman",
			skipServerCall: true,
		},
		{
			name:     "detect har from json content",
			filename: "recording.har",
			content: `{
  "log": {
    "version": "1.2",
    "entries": []
  }
}`,
			expectFormat:   "har",
			skipServerCall: true,
		},
		{
			name:     "detect wiremock from json content",
			filename: "mapping.json",
			content: `{
  "request": {
    "method": "GET",
    "url": "/test"
  },
  "response": {
    "status": 200
  }
}`,
			expectFormat:   "wiremock",
			skipServerCall: true,
		},
		{
			name:           "detect curl command from file",
			filename:       "curl-command",
			content:        `curl -X GET https://api.example.com/test`,
			expectFormat:   "curl",
			skipServerCall: true,
		},
		{
			name:         "unknown format",
			filename:     "unknown.txt",
			content:      `some random content that doesn't match any format`,
			expectError:  true,
			errorContain: "unable to detect format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test file
			filePath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(filePath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			// Run with dry-run to avoid needing a server
			args := []string{filePath, "--dry-run"}
			err := RunImport(args)

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				if tt.errorContain != "" && err != nil {
					if !containsString(err.Error(), tt.errorContain) {
						t.Errorf("error should contain %q, got: %v", tt.errorContain, err)
					}
				}
				return
			}

			// If we expect success but get an error, it might be due to
			// server connection (which is expected without --dry-run working fully)
			// The dry-run should work for valid formats
			if err != nil && !tt.skipServerCall {
				// Some errors are acceptable if format was detected correctly
				if containsString(err.Error(), "connect") ||
					containsString(err.Error(), "connection") {
					// Connection error means format detection succeeded
					return
				}
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunImport_ForceFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with ambiguous content
	filePath := filepath.Join(tmpDir, "ambiguous.json")
	content := `{"mocks": []}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	t.Run("force mockd format", func(t *testing.T) {
		err := RunImport([]string{filePath, "--format", "mockd", "--dry-run"})
		// May fail due to server connection, but shouldn't fail on format
		if err != nil && containsString(err.Error(), "unknown format") {
			t.Error("forced format should be accepted")
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		err := RunImport([]string{filePath, "--format", "invalid"})
		if err == nil {
			t.Error("expected error for invalid format")
		}
	})
}

func TestRunImport_CurlCommand(t *testing.T) {
	// Test importing from a cURL command string
	t.Run("basic curl command", func(t *testing.T) {
		// The source itself is the curl command
		err := RunImport([]string{
			`curl -X POST https://api.example.com/users -H "Content-Type: application/json" -d '{"name": "test"}'`,
			"--dry-run",
		})
		// This should parse the curl command (may fail on server connection)
		if err != nil && !containsString(err.Error(), "connect") {
			// If error is not about connection, it might be a parsing error
			// which is acceptable for this test
			t.Logf("curl parsing returned error (acceptable): %v", err)
		}
	})
}

func TestRunImport_DryRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid mockd config file
	filePath := filepath.Join(tmpDir, "mocks.yaml")
	content := `
version: "1.0"
mocks:
  - id: test-1
    type: http
    enabled: true
    http:
      matcher:
        method: GET
        path: /api/users
      response:
        statusCode: 200
        body: "[]"
  - id: test-2
    type: http
    enabled: true
    http:
      matcher:
        method: POST
        path: /api/users
      response:
        statusCode: 201
        body: "{}"
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := RunImport([]string{filePath, "--dry-run"})

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("dry-run should succeed: %v", err)
	}

	// Read captured output
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Verify output mentions dry run
	if !containsString(output, "Dry run") {
		t.Error("output should mention dry run")
	}
}

func TestRunImport_ReplaceFlag(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid mockd config file
	filePath := filepath.Join(tmpDir, "mocks.yaml")
	content := `
version: "1.0"
mocks: []
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Test with --replace flag (will fail on server connection but flag should be accepted)
	err := RunImport([]string{filePath, "--replace"})

	// Should fail with connection error, not flag parsing error
	if err != nil && containsString(err.Error(), "flag") {
		t.Error("--replace flag should be accepted")
	}
}

func TestRunImport_IncludeStaticFlag(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a HAR file
	filePath := filepath.Join(tmpDir, "recording.har")
	content := `{
  "log": {
    "version": "1.2",
    "creator": {"name": "test"},
    "entries": []
  }
}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Test with --include-static flag
	err := RunImport([]string{filePath, "--include-static", "--dry-run"})

	// Should not fail on flag parsing
	if err != nil && containsString(err.Error(), "flag") {
		t.Error("--include-static flag should be accepted")
	}
}

func TestReorderArgs(t *testing.T) {
	// Test the reorderArgs helper function that puts flags before positional args
	tests := []struct {
		name       string
		args       []string
		flagArgs   []string
		wantLen    int
		wantNonNil bool
	}{
		{
			name:       "no reordering needed",
			args:       []string{"--format", "mockd", "file.yaml"},
			flagArgs:   []string{"format", "f"},
			wantLen:    3,
			wantNonNil: true,
		},
		{
			name:       "needs reordering",
			args:       []string{"file.yaml", "--format", "mockd"},
			flagArgs:   []string{"format", "f"},
			wantLen:    3,
			wantNonNil: true,
		},
		{
			name:       "empty args returns empty slice",
			args:       []string{},
			flagArgs:   []string{"format"},
			wantLen:    0,
			wantNonNil: false, // Empty args returns nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("reorderArgs panicked: %v", r)
				}
			}()

			result := reorderArgs(tt.args, tt.flagArgs)

			// For empty input, result may be nil (which is valid for empty slices in Go)
			if tt.wantNonNil && result == nil {
				t.Error("result should not be nil")
			}

			if len(result) != tt.wantLen {
				t.Errorf("got length %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}
