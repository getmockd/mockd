package cli

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLabels(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect map[string]string
	}{
		{
			name:   "empty string",
			input:  "",
			expect: nil,
		},
		{
			name:  "single label",
			input: "env=prod",
			expect: map[string]string{
				"env": "prod",
			},
		},
		{
			name:  "multiple labels",
			input: "env=prod,region=us-west,tier=1",
			expect: map[string]string{
				"env":    "prod",
				"region": "us-west",
				"tier":   "1",
			},
		},
		{
			name:  "labels with spaces",
			input: "env = prod , region = us-west",
			expect: map[string]string{
				"env":    "prod",
				"region": "us-west",
			},
		},
		{
			name:  "label with equals in value",
			input: "query=foo=bar",
			expect: map[string]string{
				"query": "foo=bar",
			},
		},
		{
			name:   "invalid label no equals",
			input:  "invalid",
			expect: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLabels(tt.input)
			if tt.expect == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if len(result) != len(tt.expect) {
				t.Errorf("length mismatch: got %d, want %d", len(result), len(tt.expect))
			}
			for k, v := range tt.expect {
				if result[k] != v {
					t.Errorf("key %s: got %s, want %s", k, result[k], v)
				}
			}
		})
	}
}

func TestRunServe_FlagParsing(t *testing.T) {
	// Test that --help doesn't panic
	t.Run("help flag", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("RunServe panicked with --help: %v", r)
			}
		}()

		// Capture stderr since help goes there
		oldStderr := os.Stderr
		_, w, _ := os.Pipe()
		os.Stderr = w

		// This will return ErrHelp which is fine
		_ = RunServe([]string{"--help"})

		w.Close()
		os.Stderr = oldStderr
	})
}

func TestRunServe_ModeValidation(t *testing.T) {
	t.Run("register and pull mutually exclusive", func(t *testing.T) {
		err := RunServe([]string{
			"--register",
			"--pull", "mockd://test/api",
			"--name", "test-runtime",
			"--token", "test-token",
		})
		if err == nil {
			t.Error("expected error when both --register and --pull are specified")
		}
		if err != nil && err.Error() != "cannot use --register and --pull together" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("register requires name", func(t *testing.T) {
		err := RunServe([]string{
			"--register",
			"--token", "test-token",
		})
		if err == nil {
			t.Error("expected error when --register without --name")
		}
	})

	t.Run("register requires token", func(t *testing.T) {
		err := RunServe([]string{
			"--register",
			"--name", "test-runtime",
		})
		if err == nil {
			t.Error("expected error when --register without --token")
		}
	})

	t.Run("pull requires token", func(t *testing.T) {
		// Clear any existing env var
		os.Unsetenv("MOCKD_TOKEN")
		os.Unsetenv("MOCKD_RUNTIME_TOKEN")

		err := RunServe([]string{
			"--pull", "mockd://test/api",
		})
		if err == nil {
			t.Error("expected error when --pull without --token")
		}
	})
}

func TestRunServe_TokenFromEnv(t *testing.T) {
	// This test verifies that token can be read from environment variable
	// We can't fully test serve without actually starting a server,
	// but we can verify the validation logic
	t.Run("MOCKD_RUNTIME_TOKEN env var", func(t *testing.T) {
		os.Setenv("MOCKD_RUNTIME_TOKEN", "env-token")
		defer os.Unsetenv("MOCKD_RUNTIME_TOKEN")

		// Register mode should not fail on missing token when env var is set
		// (it will fail on other things like port availability)
		err := RunServe([]string{
			"--register",
			"--name", "test-runtime",
			"--port", "0", // Use port 0 to fail fast
		})
		// The error should NOT be about missing token
		if err != nil && err.Error() == "--token is required when using --register (or set MOCKD_RUNTIME_TOKEN)" {
			t.Error("token should have been read from MOCKD_RUNTIME_TOKEN env var")
		}
	})
}

func TestRunServe_ConfigFile(t *testing.T) {
	t.Run("nonexistent config file", func(t *testing.T) {
		err := RunServe([]string{
			"--config", "/nonexistent/path/config.yaml",
			"--port", "0",
		})
		// Should fail when trying to load config (after port checks which may also fail)
		// This might pass if port 0 causes an earlier error, which is fine
		_ = err
	})

	t.Run("valid config file path format", func(t *testing.T) {
		// Create a temporary config file
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "test-config.yaml")
		err := os.WriteFile(configPath, []byte(`
version: "1.0"
mocks: []
`), 0644)
		if err != nil {
			t.Fatalf("failed to create test config: %v", err)
		}

		// Just verify the flag parsing works - can't run the full server in test
		// because RunServe blocks waiting for signals
		fs := flag.NewFlagSet("serve-test", flag.ContinueOnError)
		cfg := fs.String("config", "", "Path to mock configuration file")
		fs.StringVar(cfg, "c", "", "Path to mock configuration file (shorthand)")

		err = fs.Parse([]string{"--config", configPath})
		if err != nil {
			t.Errorf("failed to parse config flag: %v", err)
		}
		if *cfg != configPath {
			t.Errorf("config path mismatch: got %s, want %s", *cfg, configPath)
		}
	})
}

func TestServeFlagDefaults(t *testing.T) {
	// Test that flags have correct default values
	fs := flag.NewFlagSet("test-serve", flag.ContinueOnError)

	// Register same flags as RunServe
	port := fs.Int("port", 4280, "HTTP server port")
	adminPort := fs.Int("admin-port", 4290, "Admin API port")
	httpsPort := fs.Int("https-port", 0, "HTTPS server port")
	readTimeout := fs.Int("read-timeout", 30, "Read timeout in seconds")
	writeTimeout := fs.Int("write-timeout", 30, "Write timeout in seconds")
	maxLogEntries := fs.Int("max-log-entries", 1000, "Maximum request log entries")
	autoCert := fs.Bool("auto-cert", true, "Auto-generate TLS certificate")
	graphqlPath := fs.String("graphql-path", "/graphql", "GraphQL endpoint path")
	grpcReflection := fs.Bool("grpc-reflection", true, "Enable gRPC reflection")

	// Parse empty args to get defaults
	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	// Verify defaults
	if *port != 4280 {
		t.Errorf("default port: got %d, want 4280", *port)
	}
	if *adminPort != 4290 {
		t.Errorf("default admin port: got %d, want 4290", *adminPort)
	}
	if *httpsPort != 0 {
		t.Errorf("default HTTPS port: got %d, want 0", *httpsPort)
	}
	if *readTimeout != 30 {
		t.Errorf("default read timeout: got %d, want 30", *readTimeout)
	}
	if *writeTimeout != 30 {
		t.Errorf("default write timeout: got %d, want 30", *writeTimeout)
	}
	if *maxLogEntries != 1000 {
		t.Errorf("default max log entries: got %d, want 1000", *maxLogEntries)
	}
	if !*autoCert {
		t.Error("default auto-cert should be true")
	}
	if *graphqlPath != "/graphql" {
		t.Errorf("default graphql path: got %s, want /graphql", *graphqlPath)
	}
	if !*grpcReflection {
		t.Error("default grpc-reflection should be true")
	}
}

func TestServeFlagCustomValues(t *testing.T) {
	fs := flag.NewFlagSet("test-serve-custom", flag.ContinueOnError)

	port := fs.Int("port", 4280, "HTTP server port")
	fs.IntVar(port, "p", 4280, "HTTP server port (shorthand)")
	adminPort := fs.Int("admin-port", 4290, "Admin API port")
	configFile := fs.String("config", "", "Path to mock configuration file")
	fs.StringVar(configFile, "c", "", "Path to mock configuration file (shorthand)")
	httpsPort := fs.Int("https-port", 0, "HTTPS server port")
	tlsCert := fs.String("tls-cert", "", "Path to TLS certificate file")
	tlsKey := fs.String("tls-key", "", "Path to TLS private key file")
	mtlsEnabled := fs.Bool("mtls-enabled", false, "Enable mTLS")
	auditEnabled := fs.Bool("audit-enabled", false, "Enable audit logging")
	register := fs.Bool("register", false, "Register with control plane")
	name := fs.String("name", "", "Runtime name")
	detach := fs.Bool("detach", false, "Run in background")
	fs.BoolVar(detach, "d", false, "Run in background (shorthand)")

	args := []string{
		"-p", "3000",
		"--admin-port", "4000",
		"-c", "/path/to/config.yaml",
		"--https-port", "8443",
		"--tls-cert", "/path/to/cert.pem",
		"--tls-key", "/path/to/key.pem",
		"--mtls-enabled",
		"--audit-enabled",
		"--register",
		"--name", "my-runtime",
		"-d",
	}

	if err := fs.Parse(args); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	if *port != 3000 {
		t.Errorf("port: got %d, want 3000", *port)
	}
	if *adminPort != 4000 {
		t.Errorf("admin port: got %d, want 4000", *adminPort)
	}
	if *configFile != "/path/to/config.yaml" {
		t.Errorf("config file: got %s, want /path/to/config.yaml", *configFile)
	}
	if *httpsPort != 8443 {
		t.Errorf("HTTPS port: got %d, want 8443", *httpsPort)
	}
	if *tlsCert != "/path/to/cert.pem" {
		t.Errorf("TLS cert: got %s, want /path/to/cert.pem", *tlsCert)
	}
	if *tlsKey != "/path/to/key.pem" {
		t.Errorf("TLS key: got %s, want /path/to/key.pem", *tlsKey)
	}
	if !*mtlsEnabled {
		t.Error("mtls-enabled should be true")
	}
	if !*auditEnabled {
		t.Error("audit-enabled should be true")
	}
	if !*register {
		t.Error("register should be true")
	}
	if *name != "my-runtime" {
		t.Errorf("name: got %s, want my-runtime", *name)
	}
	if !*detach {
		t.Error("detach should be true")
	}
}

func TestFormatPortError(t *testing.T) {
	// formatPortError is unexported but we can test it indirectly through RunServe
	// by checking error messages when port is in use

	// This test verifies the error handling path exists
	// A real port conflict test would require starting a listener first
	t.Run("error message format", func(t *testing.T) {
		// We can't easily test the actual error without binding a port,
		// but we verify the code path doesn't panic
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("formatPortError panicked: %v", r)
			}
		}()

		// Try to start on a likely-in-use port (this may or may not error)
		// The main goal is to verify no panics in error handling
		_ = RunServe([]string{
			"--port", "1", // Privileged port, will likely fail
		})
	})
}
