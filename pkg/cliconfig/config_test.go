package cliconfig

import (
	"strings"
	"testing"
)

func TestCLIConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  CLIConfig
		wantErr string
	}{
		{
			name:    "valid defaults",
			config:  *NewDefault(),
			wantErr: "",
		},
		{
			name: "valid custom ports",
			config: CLIConfig{
				Port:          8080,
				AdminPort:     8090,
				ReadTimeout:   60,
				WriteTimeout:  60,
				MaxLogEntries: 5000,
			},
			wantErr: "",
		},
		{
			name:    "port too high",
			config:  CLIConfig{Port: 70000},
			wantErr: "port 70000 is out of range",
		},
		{
			name:    "port negative",
			config:  CLIConfig{Port: -1},
			wantErr: "port -1 is out of range",
		},
		{
			name:    "admin port too high",
			config:  CLIConfig{Port: 4280, AdminPort: 99999},
			wantErr: "adminPort 99999 is out of range",
		},
		{
			name:    "https port negative",
			config:  CLIConfig{Port: 4280, AdminPort: 4290, HTTPSPort: -5},
			wantErr: "httpsPort -5 is out of range",
		},
		{
			name:    "read timeout too high",
			config:  CLIConfig{Port: 4280, AdminPort: 4290, ReadTimeout: 9999},
			wantErr: "readTimeout 9999 is out of range",
		},
		{
			name:    "write timeout negative",
			config:  CLIConfig{Port: 4280, AdminPort: 4290, WriteTimeout: -1},
			wantErr: "writeTimeout -1 is out of range",
		},
		{
			name:    "max log entries too high",
			config:  CLIConfig{Port: 4280, AdminPort: 4290, MaxLogEntries: 200000},
			wantErr: "maxLogEntries 200000 is out of range",
		},
		{
			name:    "port equals admin port",
			config:  CLIConfig{Port: 4280, AdminPort: 4280},
			wantErr: "port and adminPort cannot be the same",
		},
		{
			name: "zero ports allowed (disabled)",
			config: CLIConfig{
				Port:      0,
				AdminPort: 0,
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
			}
		})
	}
}

func TestMergeConfig_BasicFields(t *testing.T) {
	t.Run("merges non-zero values", func(t *testing.T) {
		target := NewDefault()
		source := &CLIConfig{
			Port:      9000,
			AdminURL:  "http://custom:9090",
			SetFields: map[string]bool{"port": true, "adminUrl": true},
		}

		MergeConfig(target, source, SourceLocal)

		if target.Port != 9000 {
			t.Errorf("expected port 9000, got %d", target.Port)
		}
		if target.AdminURL != "http://custom:9090" {
			t.Errorf("expected custom admin URL, got %q", target.AdminURL)
		}
		if target.Sources["port"] != SourceLocal {
			t.Errorf("expected source 'local', got %q", target.Sources["port"])
		}
	})

	t.Run("does not overwrite with zero values", func(t *testing.T) {
		target := NewDefault()
		source := &CLIConfig{
			Port: 0, // zero value should not overwrite
		}

		MergeConfig(target, source, SourceLocal)

		if target.Port != DefaultPort {
			t.Errorf("expected default port %d, got %d", DefaultPort, target.Port)
		}
	})

	t.Run("handles boolean false with SetFields", func(t *testing.T) {
		target := NewDefault()
		target.Verbose = true

		source := &CLIConfig{
			Verbose:   false,
			SetFields: map[string]bool{"verbose": true},
		}

		MergeConfig(target, source, SourceLocal)

		if target.Verbose != false {
			t.Error("expected verbose to be false after merge")
		}
	})

	t.Run("does not merge boolean false without SetFields", func(t *testing.T) {
		target := NewDefault()
		target.Verbose = true

		source := &CLIConfig{
			Verbose: false,
			// No SetFields — should not override
		}

		MergeConfig(target, source, SourceLocal)

		if target.Verbose != true {
			t.Error("expected verbose to remain true without SetFields")
		}
	})

	t.Run("nil source is no-op", func(t *testing.T) {
		target := NewDefault()
		originalPort := target.Port

		MergeConfig(target, nil, SourceLocal)

		if target.Port != originalPort {
			t.Errorf("expected port unchanged, got %d", target.Port)
		}
	})
}
