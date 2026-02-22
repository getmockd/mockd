package admin

import "testing"

func TestParsePositiveInt(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   int
		wantOK bool
	}{
		{name: "empty", in: "", want: 0, wantOK: false},
		{name: "invalid", in: "10x", want: 0, wantOK: false},
		{name: "zero", in: "0", want: 0, wantOK: false},
		{name: "negative", in: "-1", want: 0, wantOK: false},
		{name: "positive", in: "25", want: 25, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parsePositiveInt(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("parsePositiveInt(%q) = (%d, %v), want (%d, %v)", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestParseNonNegativeInt(t *testing.T) {
	tests := []struct {
		name   string
		in     string
		want   int
		wantOK bool
	}{
		{name: "empty", in: "", want: 0, wantOK: false},
		{name: "invalid", in: "3x", want: 0, wantOK: false},
		{name: "negative", in: "-1", want: 0, wantOK: false},
		{name: "zero", in: "0", want: 0, wantOK: true},
		{name: "positive", in: "7", want: 7, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseNonNegativeInt(tt.in)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("parseNonNegativeInt(%q) = (%d, %v), want (%d, %v)", tt.in, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}
