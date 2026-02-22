package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
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

func TestValidateServeFlags(t *testing.T) {
	t.Run("register and pull mutually exclusive", func(t *testing.T) {
		f := &serveFlags{
			register: true,
			pull:     "mockd://test/api",
			name:     "test-runtime",
			token:    "test-token",
		}
		err := validateServeFlags(f)
		if err == nil {
			t.Error("expected error when both register and pull are specified")
		}
		if err != nil && err.Error() != "cannot use --register and --pull together" {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("register requires name", func(t *testing.T) {
		f := &serveFlags{
			register: true,
			token:    "test-token",
		}
		err := validateServeFlags(f)
		if err == nil {
			t.Error("expected error when register without name")
		}
	})

	t.Run("register requires token", func(t *testing.T) {
		f := &serveFlags{
			register: true,
			name:     "test-runtime",
		}
		err := validateServeFlags(f)
		if err == nil {
			t.Error("expected error when register without token")
		}
	})

	t.Run("pull requires token", func(t *testing.T) {
		os.Unsetenv("MOCKD_TOKEN")
		os.Unsetenv("MOCKD_RUNTIME_TOKEN")

		f := &serveFlags{
			pull: "mockd://test/api",
		}
		err := validateServeFlags(f)
		if err == nil {
			t.Error("expected error when pull without token")
		}
	})

	t.Run("valid port ranges", func(t *testing.T) {
		f := &serveFlags{port: 4280, adminPort: 4290}
		err := validateServeFlags(f)
		if err != nil {
			t.Errorf("unexpected error for valid ports: %v", err)
		}
	})

	t.Run("invalid port range", func(t *testing.T) {
		f := &serveFlags{port: 99999, adminPort: 4290}
		err := validateServeFlags(f)
		if err == nil {
			t.Error("expected error for invalid port")
		}
	})
}

func TestBuildServerConfiguration(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		f := &serveFlags{
			port:      4280,
			adminPort: 4290,
		}
		cfg, err := buildServerConfiguration(f)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.HTTPPort != 4280 {
			t.Errorf("port: got %d, want 4280", cfg.HTTPPort)
		}
		if cfg.AdminPort != 4290 {
			t.Errorf("admin port: got %d, want 4290", cfg.AdminPort)
		}
	})

	t.Run("request timeout overrides both", func(t *testing.T) {
		f := &serveFlags{
			readTimeout:    10,
			writeTimeout:   10,
			requestTimeout: 60,
		}
		cfg, _ := buildServerConfiguration(f)
		if cfg.ReadTimeout != 60 {
			t.Errorf("read timeout: got %d, want 60", cfg.ReadTimeout)
		}
		if cfg.WriteTimeout != 60 {
			t.Errorf("write timeout: got %d, want 60", cfg.WriteTimeout)
		}
	})

	t.Run("TLS config from flags", func(t *testing.T) {
		f := &serveFlags{
			tlsCert:   "/path/to/cert.pem",
			tlsKey:    "/path/to/key.pem",
			httpsPort: 8443,
		}
		cfg, _ := buildServerConfiguration(f)
		if cfg.TLS == nil {
			t.Fatal("expected TLS config")
		}
		if cfg.TLS.CertFile != "/path/to/cert.pem" {
			t.Errorf("cert file: got %s", cfg.TLS.CertFile)
		}
	})

	t.Run("mTLS config from flags", func(t *testing.T) {
		f := &serveFlags{
			mtlsEnabled:    true,
			mtlsCA:         "/path/to/ca.crt",
			mtlsAllowedCNs: "client1,client2",
		}
		cfg, _ := buildServerConfiguration(f)
		if cfg.MTLS == nil {
			t.Fatal("expected mTLS config")
		}
		if len(cfg.MTLS.AllowedCNs) != 2 {
			t.Errorf("allowed CNs: got %d, want 2", len(cfg.MTLS.AllowedCNs))
		}
	})

	t.Run("CORS config from flags", func(t *testing.T) {
		f := &serveFlags{
			corsOrigins: "https://app.example.com, https://other.com",
		}
		cfg, _ := buildServerConfiguration(f)
		if cfg.CORS == nil {
			t.Fatal("expected CORS config")
		}
		if len(cfg.CORS.AllowOrigins) != 2 {
			t.Errorf("origins: got %d, want 2", len(cfg.CORS.AllowOrigins))
		}
	})

	t.Run("rate limit config from flags", func(t *testing.T) {
		f := &serveFlags{
			rateLimit: 100.0,
		}
		cfg, _ := buildServerConfiguration(f)
		if cfg.RateLimit == nil {
			t.Fatal("expected rate limit config")
		}
		if cfg.RateLimit.RequestsPerSecond != 100.0 {
			t.Errorf("rps: got %f, want 100.0", cfg.RateLimit.RequestsPerSecond)
		}
		if cfg.RateLimit.BurstSize != 200 {
			t.Errorf("burst: got %d, want 200", cfg.RateLimit.BurstSize)
		}
	})
}

func TestFormatPortError(t *testing.T) {
	t.Run("error message format", func(t *testing.T) {
		err := formatPortError(3000, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		msg := err.Error()
		if msg == "" {
			t.Error("expected non-empty error message")
		}
	})

	t.Run("permission denied is not reported as in use", func(t *testing.T) {
		err := formatPortError(3000, fmt.Errorf("listen tcp :3000: bind: %w", syscall.EPERM))
		if err == nil {
			t.Fatal("expected error")
		}
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "already in use") {
			t.Fatalf("unexpected in-use message: %q", err.Error())
		}
		if !strings.Contains(msg, "could not bind port 3000") {
			t.Fatalf("unexpected message: %q", err.Error())
		}
	})

	t.Run("unexpected port check error is surfaced", func(t *testing.T) {
		err := formatPortError(3000, errors.New("network namespace unavailable"))
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "failed to check port 3000 availability") {
			t.Fatalf("unexpected message: %q", err.Error())
		}
	})
}
