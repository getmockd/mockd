package logging

import (
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		// Lowercase
		{"debug", LevelDebug},
		{"info", LevelInfo},
		{"warn", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},

		// Uppercase
		{"DEBUG", LevelDebug},
		{"INFO", LevelInfo},
		{"WARN", LevelWarn},
		{"WARNING", LevelWarn},
		{"ERROR", LevelError},

		// Mixed case (the fix: these should all work now)
		{"Debug", LevelDebug},
		{"Info", LevelInfo},
		{"Warn", LevelWarn},
		{"Warning", LevelWarn},
		{"Error", LevelError},
		{"dEbUg", LevelDebug},

		// Empty string defaults to Info
		{"", LevelInfo},

		// Unrecognized defaults to Info
		{"trace", LevelInfo},
		{"fatal", LevelInfo},
		{"unknown", LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected Format
	}{
		{"json", FormatJSON},
		{"JSON", FormatJSON},
		{"Json", FormatJSON},
		{"text", FormatText},
		{"TEXT", FormatText},
		{"", FormatText},
		{"yaml", FormatText}, // unrecognized defaults to text
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseFormat(tt.input)
			if result != tt.expected {
				t.Errorf("ParseFormat(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
