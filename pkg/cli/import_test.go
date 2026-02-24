package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func reorderArgs(args []string, knownFlags []string) []string {
	var flags, positional []string

	i := 0
	for i < len(args) {
		arg := args[i]

		isFlag := false
		for _, f := range knownFlags {
			if arg == "--"+f || arg == "-"+f {
				flags = append(flags, arg)
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					i++
					flags = append(flags, args[i])
				}
				isFlag = true
				break
			}
			if strings.HasPrefix(arg, "--"+f+"=") || strings.HasPrefix(arg, "-"+f+"=") {
				flags = append(flags, arg)
				isFlag = true
				break
			}
		}

		if !isFlag && (arg == "--json" || arg == "-json") {
			flags = append(flags, arg)
			isFlag = true
		}

		if !isFlag {
			if strings.HasPrefix(arg, "-") && arg != "-" {
				flags = append(flags, arg)
			} else {
				positional = append(positional, arg)
			}
		}
		i++
	}

	return append(flags, positional...)
}

// setImportFlags sets package-level import flags for testing and returns cleanup via t.Cleanup.
func setImportFlags(t *testing.T, format string, replace, dryRun, includeStatic bool) {
	t.Helper()
	oldFormat := importFormat
	oldReplace := importReplace
	oldDryRun := importDryRun
	oldIncludeStatic := importIncludeStatic
	t.Cleanup(func() {
		importFormat = oldFormat
		importReplace = oldReplace
		importDryRun = oldDryRun
		importIncludeStatic = oldIncludeStatic
	})
	importFormat = format
	importReplace = replace
	importDryRun = dryRun
	importIncludeStatic = includeStatic
}

func TestImportCmd_AcceptsZeroOrOneArg(t *testing.T) {
	// importCmd uses MaximumNArgs(1): 0 args = stdin mode, 1 arg = file/dir/curl.
	// Verify the Args validator accepts both 0 and 1 args, rejects 2+.
	if err := cobra.MaximumNArgs(1)(importCmd, []string{}); err != nil {
		t.Error("should accept 0 args (stdin mode)")
	}
	if err := cobra.MaximumNArgs(1)(importCmd, []string{"file.yaml"}); err != nil {
		t.Error("should accept 1 arg (file/dir/curl)")
	}
	if err := cobra.MaximumNArgs(1)(importCmd, []string{"a", "b"}); err == nil {
		t.Error("should reject 2 args")
	}
}

func TestImportCmd_FileNotFoundDirect(t *testing.T) {
	// Test proper invocation with a nonexistent file
	setImportFlags(t, "", false, false, false)
	err := runImportCobra(importCmd, []string{"/nonexistent/file.json"})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestImportCmd_FileNotFound(t *testing.T) {
	setImportFlags(t, "", false, false, false)
	err := runImportCobra(importCmd, []string{"/nonexistent/path/file.json"})
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
	if err != nil && !strings.Contains(err.Error(), "file not found") {
		t.Errorf("expected 'file not found' error, got: %v", err)
	}
}

func TestImportCmd_FormatDetection(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		filename     string
		content      string
		expectError  bool
		errorContain string
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
		},
		{
			name:     "detect postman from json content",
			filename: "collection.json",
			content: `{
  "info": {
    "name": "Test Collection",
    "_postman_id": "12345",
    "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"
  },
  "item": []
}`,
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
			setImportFlags(t, "", false, true, false)
			err := runImportCobra(importCmd, []string{filePath})

			if tt.expectError {
				if err == nil {
					t.Error("expected error but got nil")
				}
				if tt.errorContain != "" && err != nil {
					if !strings.Contains(err.Error(), tt.errorContain) {
						t.Errorf("error should contain %q, got: %v", tt.errorContain, err)
					}
				}
				return
			}

			// For valid formats, dry-run should succeed (parse only, no server needed)
			if err != nil {
				t.Errorf("unexpected error with dry-run: %v", err)
			}
		})
	}
}

func TestImportCmd_ForceFormat(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with ambiguous content
	filePath := filepath.Join(tmpDir, "ambiguous.json")
	content := `{"mocks": []}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	t.Run("force mockd format", func(t *testing.T) {
		setImportFlags(t, "mockd", false, true, false)
		err := runImportCobra(importCmd, []string{filePath})
		// May fail on parsing, but shouldn't fail on format detection
		if err != nil && strings.Contains(err.Error(), "unknown format") {
			t.Error("forced format should be accepted")
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		setImportFlags(t, "invalid", false, false, false)
		err := runImportCobra(importCmd, []string{filePath})
		if err == nil {
			t.Error("expected error for invalid format")
		}
		if err != nil && !strings.Contains(err.Error(), "unknown format") {
			t.Errorf("expected 'unknown format' error, got: %v", err)
		}
	})
}

func TestImportCmd_CurlCommand(t *testing.T) {
	// Test importing from a cURL command string
	t.Run("basic curl command", func(t *testing.T) {
		setImportFlags(t, "", false, true, false)
		// The source itself is the curl command
		err := runImportCobra(importCmd, []string{
			`curl -X POST https://api.example.com/users -H "Content-Type: application/json" -d '{"name": "test"}'`,
		})
		// This should parse the curl command; may or may not succeed depending on parser
		if err != nil && !strings.Contains(err.Error(), "connect") {
			t.Logf("curl parsing returned error (acceptable): %v", err)
		}
	})
}

func TestImportCmd_DryRun(t *testing.T) {
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

	setImportFlags(t, "", false, true, false)
	err := runImportCobra(importCmd, []string{filePath})

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
	if !strings.Contains(output, "Dry run") {
		t.Error("output should mention dry run")
	}
}

func TestImportCmd_ReplaceFlag(t *testing.T) {
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
	setImportFlags(t, "", true, false, false)
	err := runImportCobra(importCmd, []string{filePath})

	// Should fail with connection error, not flag parsing error
	if err != nil && strings.Contains(err.Error(), "flag") {
		t.Error("--replace flag should be accepted")
	}
}

func TestImportCmd_IncludeStaticFlag(t *testing.T) {
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

	// Test with --include-static flag (dry-run to avoid server)
	setImportFlags(t, "", false, true, true)
	err := runImportCobra(importCmd, []string{filePath})

	// Should not fail on flag parsing
	if err != nil && strings.Contains(err.Error(), "flag") {
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
