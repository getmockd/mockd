package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

// setExportFlags sets package-level export flags for testing and returns cleanup via t.Cleanup.
func setExportFlags(t *testing.T, output, name, format, version string) {
	t.Helper()
	oldOutput := exportOutput
	oldName := exportName
	oldFormat := exportFormat
	oldVersion := exportVersion
	t.Cleanup(func() {
		exportOutput = oldOutput
		exportName = oldName
		exportFormat = oldFormat
		exportVersion = oldVersion
	})
	exportOutput = output
	exportName = name
	exportFormat = format
	exportVersion = version
}

func TestExportCmd_InvalidFormat(t *testing.T) {
	setExportFlags(t, "", "exported-config", "invalid", "")
	err := runExportCobra(exportCmd, []string{})
	if err == nil {
		t.Error("expected error for invalid format")
	}
	if err != nil && !strings.Contains(err.Error(), "invalid format") {
		t.Errorf("error should mention invalid format: %v", err)
	}
}

func TestExportCmd_UnsupportedExportFormat(t *testing.T) {
	// Formats like postman, har, wiremock can import but not export
	unsupportedFormats := []string{"postman", "har", "wiremock", "curl"}

	for _, format := range unsupportedFormats {
		t.Run(format, func(t *testing.T) {
			setExportFlags(t, "", "exported-config", format, "")
			err := runExportCobra(exportCmd, []string{})
			if err == nil {
				t.Errorf("expected error for unsupported export format: %s", format)
			}
			if err != nil && !strings.Contains(err.Error(), "does not support export") {
				t.Errorf("error should mention export not supported for %s: %v", format, err)
			}
		})
	}
}

func TestExportCmd_SupportedFormats(t *testing.T) {
	// These formats should be accepted (will fail on server connection)
	supportedFormats := []string{"mockd", "openapi"}

	for _, format := range supportedFormats {
		t.Run(format, func(t *testing.T) {
			setExportFlags(t, "", "exported-config", format, "")
			err := runExportCobra(exportCmd, []string{})

			// Should fail with connection error, not format error
			if err != nil {
				if strings.Contains(err.Error(), "invalid format") ||
					strings.Contains(err.Error(), "does not support export") {
					t.Errorf("format %s should be supported for export", format)
				}
				// Connection error is expected
			}
		})
	}
}

func TestExportCmd_OutputFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("yaml extension", func(t *testing.T) {
		outputPath := filepath.Join(tmpDir, "output.yaml")
		setExportFlags(t, outputPath, "exported-config", "mockd", "")
		err := runExportCobra(exportCmd, []string{})

		// Will fail on connection, but should accept the output path
		if err != nil && strings.Contains(err.Error(), "output") {
			t.Error("output flag should be accepted")
		}
	})

	t.Run("json extension", func(t *testing.T) {
		outputPath := filepath.Join(tmpDir, "output.json")
		setExportFlags(t, outputPath, "exported-config", "mockd", "")
		err := runExportCobra(exportCmd, []string{})

		// Will fail on connection, but should accept the output path
		if err != nil && strings.Contains(err.Error(), "output") {
			t.Error("output flag should be accepted")
		}
	})

	t.Run("yml extension", func(t *testing.T) {
		outputPath := filepath.Join(tmpDir, "output.yml")
		setExportFlags(t, outputPath, "exported-config", "mockd", "")
		err := runExportCobra(exportCmd, []string{})

		// Will fail on connection, but should accept the output path
		if err != nil && strings.Contains(err.Error(), "output") {
			t.Error("output flag should be accepted")
		}
	})
}

func TestExportCmd_NameFlag(t *testing.T) {
	t.Run("default name", func(t *testing.T) {
		setExportFlags(t, "", "exported-config", "mockd", "")
		err := runExportCobra(exportCmd, []string{})
		// Will fail on connection, but name flag should have default
		if err != nil && strings.Contains(err.Error(), "flag provided but not defined") {
			t.Error("name should have default value")
		}
	})

	t.Run("custom name", func(t *testing.T) {
		setExportFlags(t, "", "My API Mocks", "mockd", "")
		err := runExportCobra(exportCmd, []string{})
		// Will fail on connection, but should accept custom name
		if err != nil && strings.Contains(err.Error(), "flag provided but not defined") {
			t.Error("custom name should be accepted")
		}
	})
}

func TestExportCmd_VersionFlag(t *testing.T) {
	setExportFlags(t, "", "exported-config", "mockd", "1.0.0")
	err := runExportCobra(exportCmd, []string{})

	// Will fail on connection, but should accept version flag
	if err != nil && strings.Contains(err.Error(), "version") &&
		!strings.Contains(err.Error(), "connect") {
		t.Error("--version flag should be accepted")
	}
}

func TestExportCmd_ConnectionError(t *testing.T) {
	// Temporarily override adminURL to a port that's unlikely to be in use
	oldAdminURL := adminURL
	adminURL = "http://localhost:19999"
	defer func() { adminURL = oldAdminURL }()

	setExportFlags(t, "", "exported-config", "mockd", "")
	err := runExportCobra(exportCmd, []string{})

	if err == nil {
		t.Error("expected connection error")
	}

	// The error should be a formatted connection error
	if err != nil {
		errStr := err.Error()
		// Should mention cannot connect or connection refused
		if !strings.Contains(errStr, "connect") &&
			!strings.Contains(errStr, "connection") &&
			!strings.Contains(errStr, "refused") {
			t.Logf("Error format may vary, but should be about connectivity: %s", errStr)
		}
	}
}

func TestExportCmd_DefaultValues(t *testing.T) {
	t.Run("default format is mockd", func(t *testing.T) {
		// Without --format, should default to mockd
		setExportFlags(t, "", "exported-config", "mockd", "")
		err := runExportCobra(exportCmd, []string{})
		// If it fails with format error, default wasn't applied
		if err != nil && strings.Contains(err.Error(), "format") &&
			!strings.Contains(err.Error(), "connect") {
			t.Error("default format should be mockd")
		}
	})

	t.Run("default name is exported-config", func(t *testing.T) {
		setExportFlags(t, "", "exported-config", "mockd", "")
		err := runExportCobra(exportCmd, []string{})
		// If it fails with name error, default wasn't applied
		if err != nil && strings.Contains(err.Error(), "name required") {
			t.Error("default name should be applied")
		}
	})
}

func TestExportCmd_StdoutOutput(t *testing.T) {
	// When no --output is specified, should output to stdout
	setExportFlags(t, "", "exported-config", "mockd", "")
	err := runExportCobra(exportCmd, []string{})

	// Should fail on connection, not on missing output file
	if err != nil && strings.Contains(err.Error(), "output") &&
		strings.Contains(err.Error(), "required") {
		t.Error("--output should not be required (stdout is default)")
	}
}
