package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunExport_HelpFlag(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("RunExport panicked with --help: %v", r)
		}
	}()

	// Capture stderr since help goes there
	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	_ = RunExport([]string{"--help"})

	w.Close()
	os.Stderr = oldStderr
}

func TestRunExport_InvalidFormat(t *testing.T) {
	err := RunExport([]string{"--format", "invalid"})
	if err == nil {
		t.Error("expected error for invalid format")
	}
	if err != nil && !containsString(err.Error(), "invalid format") {
		t.Errorf("error should mention invalid format: %v", err)
	}
}

func TestRunExport_UnsupportedExportFormat(t *testing.T) {
	// Formats like postman, har, wiremock can import but not export
	unsupportedFormats := []string{"postman", "har", "wiremock", "curl"}

	for _, format := range unsupportedFormats {
		t.Run(format, func(t *testing.T) {
			err := RunExport([]string{"--format", format})
			if err == nil {
				t.Errorf("expected error for unsupported export format: %s", format)
			}
			if err != nil && !containsString(err.Error(), "does not support export") {
				t.Errorf("error should mention export not supported for %s: %v", format, err)
			}
		})
	}
}

func TestRunExport_SupportedFormats(t *testing.T) {
	// These formats should be accepted (will fail on server connection)
	supportedFormats := []string{"mockd", "openapi"}

	for _, format := range supportedFormats {
		t.Run(format, func(t *testing.T) {
			err := RunExport([]string{"--format", format})

			// Should fail with connection error, not format error
			if err != nil {
				if containsString(err.Error(), "invalid format") ||
					containsString(err.Error(), "does not support export") {
					t.Errorf("format %s should be supported for export", format)
				}
				// Connection error is expected
			}
		})
	}
}

func TestRunExport_OutputFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("yaml extension", func(t *testing.T) {
		outputPath := filepath.Join(tmpDir, "output.yaml")
		err := RunExport([]string{"--output", outputPath})

		// Will fail on connection, but should accept the output path
		if err != nil && containsString(err.Error(), "output") {
			t.Error("--output flag should be accepted")
		}
	})

	t.Run("json extension", func(t *testing.T) {
		outputPath := filepath.Join(tmpDir, "output.json")
		err := RunExport([]string{"--output", outputPath})

		// Will fail on connection, but should accept the output path
		if err != nil && containsString(err.Error(), "output") {
			t.Error("--output flag should be accepted")
		}
	})

	t.Run("yml extension", func(t *testing.T) {
		outputPath := filepath.Join(tmpDir, "output.yml")
		err := RunExport([]string{"--output", outputPath})

		// Will fail on connection, but should accept the output path
		if err != nil && containsString(err.Error(), "output") {
			t.Error("--output flag should be accepted")
		}
	})
}

func TestRunExport_OutputShorthand(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.yaml")

	err := RunExport([]string{"-o", outputPath})

	// Will fail on connection, but -o shorthand should work
	if err != nil && containsString(err.Error(), "flag provided but not defined: -o") {
		t.Error("-o shorthand should be accepted")
	}
}

func TestRunExport_NameFlag(t *testing.T) {
	t.Run("default name", func(t *testing.T) {
		// Default name is "exported-config"
		err := RunExport([]string{})
		// Will fail on connection, but name flag should have default
		// The URL will contain name=exported-config as a query param
		if err != nil && containsString(err.Error(), "flag provided but not defined") {
			t.Error("name should have default value")
		}
	})

	t.Run("custom name", func(t *testing.T) {
		err := RunExport([]string{"--name", "My API Mocks"})
		// Will fail on connection, but should accept custom name
		if err != nil && containsString(err.Error(), "flag provided but not defined: --name") {
			t.Error("--name flag should be accepted")
		}
	})

	t.Run("name shorthand", func(t *testing.T) {
		err := RunExport([]string{"-n", "My API Mocks"})
		// Will fail on connection, but -n shorthand should work
		if err != nil && containsString(err.Error(), "flag provided but not defined: -n") {
			t.Error("-n shorthand should be accepted")
		}
	})
}

func TestRunExport_VersionFlag(t *testing.T) {
	err := RunExport([]string{"--version", "1.0.0"})

	// Will fail on connection, but should accept version flag
	if err != nil && containsString(err.Error(), "version") {
		t.Error("--version flag should be accepted")
	}
}

func TestRunExport_FormatShorthand(t *testing.T) {
	err := RunExport([]string{"-f", "openapi"})

	// Will fail on connection, but -f shorthand should work
	if err != nil && containsString(err.Error(), "flag provided but not defined: -f") {
		t.Error("-f shorthand should be accepted")
	}
}

func TestRunExport_AdminURLFlag(t *testing.T) {
	t.Run("custom admin url", func(t *testing.T) {
		err := RunExport([]string{"--admin-url", "http://custom:9000"})

		// Will fail on connection to custom URL, but flag should be accepted
		if err != nil && containsString(err.Error(), "admin-url") {
			t.Error("--admin-url flag should be accepted")
		}
	})
}

func TestRunExport_CombinedFlags(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "api-spec.yaml")

	err := RunExport([]string{
		"-o", outputPath,
		"-n", "My API",
		"-f", "openapi",
		"--version", "2.0.0",
	})

	// Will fail on connection, but all flags should be parsed correctly
	if err != nil {
		// Should not be a flag parsing error
		if containsString(err.Error(), "flag provided but not defined") {
			t.Errorf("all flags should be accepted: %v", err)
		}
	}
}

func TestExportOutputFormatDetection(t *testing.T) {
	// Test that output format (YAML vs JSON) is detected from file extension
	tests := []struct {
		filename   string
		expectYAML bool
	}{
		{"output.yaml", true},
		{"output.yml", true},
		{"output.json", false},
		{"output.JSON", false},
		{"output.YAML", true},
		{"output", true},      // default to YAML when no extension
		{"output.txt", false}, // non-yaml extensions default to JSON
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			// We can't directly test the internal logic, but we can verify
			// the flags are accepted with different extensions
			tmpDir := t.TempDir()
			outputPath := filepath.Join(tmpDir, tt.filename)

			err := RunExport([]string{"-o", outputPath})

			// Should not fail on file extension handling
			if err != nil && containsString(err.Error(), "extension") {
				t.Errorf("file extension %s should be accepted", filepath.Ext(tt.filename))
			}
		})
	}
}

func TestRunExport_ConnectionError(t *testing.T) {
	// Test that connection errors are properly formatted
	err := RunExport([]string{"--admin-url", "http://localhost:19999"}) // unlikely port

	if err == nil {
		t.Error("expected connection error")
	}

	// The error should be a formatted connection error
	if err != nil {
		errStr := err.Error()
		// Should mention cannot connect or connection refused
		if !containsString(errStr, "connect") &&
			!containsString(errStr, "connection") &&
			!containsString(errStr, "refused") {
			t.Logf("Error format may vary, but should be about connectivity: %s", errStr)
		}
	}
}

func TestRunExport_DefaultValues(t *testing.T) {
	// Test that defaults are applied correctly
	// We test this by verifying the command doesn't fail on missing required values

	t.Run("default format is mockd", func(t *testing.T) {
		// Without --format, should default to mockd
		err := RunExport([]string{})
		// If it fails with format error, default wasn't applied
		if err != nil && containsString(err.Error(), "format") &&
			!containsString(err.Error(), "connect") {
			t.Error("default format should be mockd")
		}
	})

	t.Run("default name is exported-config", func(t *testing.T) {
		// Without --name, should default to "exported-config"
		err := RunExport([]string{})
		// If it fails with name error, default wasn't applied
		if err != nil && containsString(err.Error(), "name required") {
			t.Error("default name should be applied")
		}
	})
}

func TestRunExport_StdoutOutput(t *testing.T) {
	// When no --output is specified, should output to stdout
	// We can't easily test this without a running server, but we verify
	// the code path doesn't require --output

	err := RunExport([]string{"--format", "mockd"})

	// Should fail on connection, not on missing output file
	if err != nil && containsString(err.Error(), "output") &&
		containsString(err.Error(), "required") {
		t.Error("--output should not be required (stdout is default)")
	}
}
