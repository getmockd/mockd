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
