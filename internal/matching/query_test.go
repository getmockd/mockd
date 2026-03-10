package matching

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchQueryParam(t *testing.T) {
	tests := []struct {
		name     string
		param    string
		expected string
		params   url.Values
		want     bool
	}{
		{
			name:     "exact match",
			param:    "status",
			expected: "active",
			params:   url.Values{"status": []string{"active"}},
			want:     true,
		},
		{
			name:     "value mismatch",
			param:    "status",
			expected: "active",
			params:   url.Values{"status": []string{"inactive"}},
			want:     false,
		},
		{
			name:     "missing param",
			param:    "status",
			expected: "active",
			params:   url.Values{},
			want:     false,
		},
		{
			name:     "empty expected matches empty value",
			param:    "flag",
			expected: "",
			params:   url.Values{"flag": []string{""}},
			want:     true,
		},
		{
			name:     "empty expected matches missing param",
			param:    "missing",
			expected: "",
			params:   url.Values{},
			want:     true, // url.Values.Get returns "" for missing key
		},
		{
			name:     "multi-value param returns first via Get",
			param:    "tag",
			expected: "alpha",
			params:   url.Values{"tag": []string{"alpha", "beta"}},
			want:     true,
		},
		{
			name:     "multi-value param second value not matched by Get",
			param:    "tag",
			expected: "beta",
			params:   url.Values{"tag": []string{"alpha", "beta"}},
			want:     false,
		},
		{
			name:     "case-sensitive param name",
			param:    "Status",
			expected: "active",
			params:   url.Values{"status": []string{"active"}},
			want:     false, // url.Values keys are case-sensitive
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchQueryParam(tt.param, tt.expected, tt.params)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMatchQueryParams(t *testing.T) {
	tests := []struct {
		name     string
		expected map[string]string
		params   url.Values
		want     bool
	}{
		{
			name:     "nil expected matches anything",
			expected: nil,
			params:   url.Values{"any": []string{"value"}},
			want:     true,
		},
		{
			name:     "empty expected matches anything",
			expected: map[string]string{},
			params:   url.Values{"any": []string{"value"}},
			want:     true,
		},
		{
			name: "single param match",
			expected: map[string]string{
				"status": "active",
			},
			params: url.Values{"status": []string{"active"}},
			want:   true,
		},
		{
			name: "single param mismatch",
			expected: map[string]string{
				"status": "active",
			},
			params: url.Values{"status": []string{"inactive"}},
			want:   false,
		},
		{
			name: "multiple params all match",
			expected: map[string]string{
				"status": "active",
				"page":   "1",
			},
			params: url.Values{
				"status": []string{"active"},
				"page":   []string{"1"},
			},
			want: true,
		},
		{
			name: "multiple params one wrong value",
			expected: map[string]string{
				"status": "active",
				"page":   "1",
			},
			params: url.Values{
				"status": []string{"active"},
				"page":   []string{"2"},
			},
			want: false,
		},
		{
			name: "multiple params one missing",
			expected: map[string]string{
				"status": "active",
				"page":   "1",
			},
			params: url.Values{
				"status": []string{"active"},
			},
			want: false,
		},
		{
			name: "extra params in request do not prevent match",
			expected: map[string]string{
				"status": "active",
			},
			params: url.Values{
				"status": []string{"active"},
				"page":   []string{"1"},
				"sort":   []string{"name"},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchQueryParams(tt.expected, tt.params)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestHasQueryParam(t *testing.T) {
	tests := []struct {
		name   string
		param  string
		params url.Values
		want   bool
	}{
		{
			name:   "param present with value",
			param:  "status",
			params: url.Values{"status": []string{"active"}},
			want:   true,
		},
		{
			name:   "param present with empty value",
			param:  "flag",
			params: url.Values{"flag": []string{""}},
			want:   true,
		},
		{
			name:   "param missing",
			param:  "missing",
			params: url.Values{"other": []string{"value"}},
			want:   false,
		},
		{
			name:   "param missing from empty params",
			param:  "anything",
			params: url.Values{},
			want:   false,
		},
		{
			name:   "param present with multiple values",
			param:  "tag",
			params: url.Values{"tag": []string{"a", "b", "c"}},
			want:   true,
		},
		{
			name:   "case-sensitive key check",
			param:  "Status",
			params: url.Values{"status": []string{"active"}},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasQueryParam(tt.param, tt.params)
			assert.Equal(t, tt.want, got)
		})
	}
}
