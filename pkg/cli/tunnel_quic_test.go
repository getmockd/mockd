package cli

import (
	"testing"
)

func TestMQTTFlag_Set(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantPort int
		wantName string
		wantErr  bool
	}{
		{
			name:     "port only",
			input:    "1883",
			wantPort: 1883,
			wantName: "",
		},
		{
			name:     "port with name",
			input:    "1883:sensors",
			wantPort: 1883,
			wantName: "sensors",
		},
		{
			name:     "high port",
			input:    "18830",
			wantPort: 18830,
			wantName: "",
		},
		{
			name:     "name with hyphens",
			input:    "1884:my-alerts-broker",
			wantPort: 1884,
			wantName: "my-alerts-broker",
		},
		{
			name:    "invalid port (not a number)",
			input:   "abc",
			wantErr: true,
		},
		{
			name:    "port zero",
			input:   "0",
			wantErr: true,
		},
		{
			name:    "port too high",
			input:   "99999",
			wantErr: true,
		},
		{
			name:    "empty name after colon",
			input:   "1883:",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var f mqttPortFlag
			err := f.Set(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(f) != 1 {
				t.Fatalf("expected 1 entry, got %d", len(f))
			}

			if f[0].Port != tt.wantPort {
				t.Errorf("port = %d, want %d", f[0].Port, tt.wantPort)
			}
			if f[0].Name != tt.wantName {
				t.Errorf("name = %q, want %q", f[0].Name, tt.wantName)
			}
			if f[0].Type != "mqtt" {
				t.Errorf("type = %q, want %q", f[0].Type, "mqtt")
			}
		})
	}
}

func TestMQTTFlag_Multiple(t *testing.T) {
	var f mqttPortFlag

	if err := f.Set("1883:sensors"); err != nil {
		t.Fatalf("first Set: %v", err)
	}
	if err := f.Set("1884:alerts"); err != nil {
		t.Fatalf("second Set: %v", err)
	}
	if err := f.Set("1885"); err != nil {
		t.Fatalf("third Set: %v", err)
	}

	if len(f) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(f))
	}

	// Verify order preserved
	if f[0].Port != 1883 || f[0].Name != "sensors" {
		t.Errorf("entry 0: got %d:%s, want 1883:sensors", f[0].Port, f[0].Name)
	}
	if f[1].Port != 1884 || f[1].Name != "alerts" {
		t.Errorf("entry 1: got %d:%s, want 1884:alerts", f[1].Port, f[1].Name)
	}
	if f[2].Port != 1885 || f[2].Name != "" {
		t.Errorf("entry 2: got %d:%s, want 1885:(empty)", f[2].Port, f[2].Name)
	}
}

func TestMQTTFlag_String(t *testing.T) {
	var f mqttPortFlag
	_ = f.Set("1883:sensors")
	_ = f.Set("1884")

	got := f.String()
	want := "1883:sensors, 1884"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestValidateTunnelQUICAuthInputs(t *testing.T) {
	tests := []struct {
		name      string
		authToken string
		authBasic string
		allowIPs  string
		authHeader string
		wantErr   bool
	}{
		{
			name:      "no auth flags",
			authToken: "",
			authBasic: "",
			allowIPs:  "",
			authHeader: "",
			wantErr:   false,
		},
		{
			name:      "token only",
			authToken: "secret123",
			authBasic: "",
			allowIPs:  "",
			authHeader: "",
			wantErr:   false,
		},
		{
			name:      "basic only",
			authToken: "",
			authBasic: "user:pass",
			allowIPs:  "",
			authHeader: "",
			wantErr:   false,
		},
		{
			name:      "allow ips only",
			authToken: "",
			authBasic: "",
			allowIPs:  "10.0.0.0/8",
			authHeader: "",
			wantErr:   false,
		},
		{
			name:      "token with custom header",
			authToken: "secret123",
			authBasic: "",
			allowIPs:  "",
			authHeader: "X-Custom-Token",
			wantErr:   false,
		},
		{
			name:      "token and basic",
			authToken: "secret123",
			authBasic: "user:pass",
			allowIPs:  "",
			authHeader: "",
			wantErr:   true,
		},
		{
			name:      "token and allow ips",
			authToken: "secret123",
			authBasic: "",
			allowIPs:  "10.0.0.0/8",
			authHeader: "",
			wantErr:   true,
		},
		{
			name:      "basic and allow ips",
			authToken: "",
			authBasic: "user:pass",
			allowIPs:  "10.0.0.0/8",
			authHeader: "",
			wantErr:   true,
		},
		{
			name:      "all three",
			authToken: "secret123",
			authBasic: "user:pass",
			allowIPs:  "10.0.0.0/8",
			authHeader: "",
			wantErr:   true,
		},
		{
			name:      "auth header without token",
			authToken: "",
			authBasic: "",
			allowIPs:  "",
			authHeader: "X-Custom-Token",
			wantErr:   true,
		},
		{
			name:      "auth header with basic only",
			authToken: "",
			authBasic: "user:pass",
			allowIPs:  "",
			authHeader: "X-Custom-Token",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTunnelQUICAuthInputs(tt.authToken, tt.authBasic, tt.allowIPs, tt.authHeader)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestResetTunnelQUICMQTTState(t *testing.T) {
	var f mqttPortFlag
	if err := f.Set("1883:sensors"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	tqMQTTPorts = f

	mqttFlag := tunnelQUICCmd.Flags().Lookup("mqtt")
	if mqttFlag == nil {
		t.Fatal("mqtt flag not found")
	}
	mqttFlag.Changed = true

	resetTunnelQUICMQTTState(tunnelQUICCmd)

	if len(tqMQTTPorts) != 0 {
		t.Fatalf("expected tqMQTTPorts to be reset, got %d entries", len(tqMQTTPorts))
	}
	if mqttFlag.Changed {
		t.Fatal("expected mqtt flag Changed=false after reset")
	}
}
