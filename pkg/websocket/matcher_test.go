package websocket

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExactMatcher(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		input    string
		expected bool
	}{
		{"exact match", "hello", "hello", true},
		{"no match", "hello", "world", false},
		{"case sensitive", "Hello", "hello", false},
		{"partial no match", "hello", "hello world", false},
		{"empty match", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(&MatcherConfig{
				Match: &MatchCriteria{Type: "exact", Value: tt.value},
			})
			require.NoError(t, err)

			result := m.Match(MessageText, []byte(tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegexMatcher(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		input    string
		expected bool
	}{
		{"simple match", "^hello", "hello world", true},
		{"no match", "^hello", "world hello", false},
		{"wildcard", "h.llo", "hello", true},
		{"anchored", "^test$", "test", true},
		{"anchored no match", "^test$", "test123", false},
		{"number pattern", "[0-9]+", "abc123def", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(&MatcherConfig{
				Match: &MatchCriteria{Type: "regex", Value: tt.pattern},
			})
			require.NoError(t, err)

			result := m.Match(MessageText, []byte(tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRegexMatcherInvalidPattern(t *testing.T) {
	_, err := NewMatcher(&MatcherConfig{
		Match: &MatchCriteria{Type: "regex", Value: "[invalid"},
	})
	assert.Error(t, err)
}

func TestContainsMatcher(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		input    string
		expected bool
	}{
		{"contains at start", "hello", "hello world", true},
		{"contains in middle", "needle", "hay needle stack", true},
		{"contains at end", "world", "hello world", true},
		{"no match", "xyz", "hello world", false},
		{"case sensitive", "Hello", "hello world", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(&MatcherConfig{
				Match: &MatchCriteria{Type: "contains", Value: tt.value},
			})
			require.NoError(t, err)

			result := m.Match(MessageText, []byte(tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrefixMatcher(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		input    string
		expected bool
	}{
		{"prefix match", "hello", "hello world", true},
		{"no prefix", "world", "hello world", false},
		{"exact match", "hello", "hello", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(&MatcherConfig{
				Match: &MatchCriteria{Type: "prefix", Value: tt.value},
			})
			require.NoError(t, err)

			result := m.Match(MessageText, []byte(tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSuffixMatcher(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		input    string
		expected bool
	}{
		{"suffix match", "world", "hello world", true},
		{"no suffix", "hello", "hello world", false},
		{"exact match", "hello", "hello", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(&MatcherConfig{
				Match: &MatchCriteria{Type: "suffix", Value: tt.value},
			})
			require.NoError(t, err)

			result := m.Match(MessageText, []byte(tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestJSONMatcher(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		value    string
		input    string
		expected bool
	}{
		{
			name:     "simple field match",
			path:     "$.type",
			value:    "subscribe",
			input:    `{"type": "subscribe", "channel": "news"}`,
			expected: true,
		},
		{
			name:     "no dollar prefix",
			path:     "type",
			value:    "subscribe",
			input:    `{"type": "subscribe"}`,
			expected: true,
		},
		{
			name:     "nested field",
			path:     "$.data.action",
			value:    "create",
			input:    `{"data": {"action": "create"}}`,
			expected: true,
		},
		{
			name:     "no match",
			path:     "$.type",
			value:    "subscribe",
			input:    `{"type": "unsubscribe"}`,
			expected: false,
		},
		{
			name:     "field not found",
			path:     "$.missing",
			value:    "test",
			input:    `{"type": "test"}`,
			expected: false,
		},
		{
			name:     "invalid json",
			path:     "$.type",
			value:    "test",
			input:    `not json`,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewMatcher(&MatcherConfig{
				Match: &MatchCriteria{Type: "json", Path: tt.path, Value: tt.value},
			})
			require.NoError(t, err)

			result := m.Match(MessageText, []byte(tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMessageTypeFilter(t *testing.T) {
	m, err := NewMatcher(&MatcherConfig{
		Match: &MatchCriteria{Type: "exact", Value: "test", MessageType: "text"},
	})
	require.NoError(t, err)

	// Should match text
	assert.True(t, m.Match(MessageText, []byte("test")))

	// Should not match binary
	assert.False(t, m.Match(MessageBinary, []byte("test")))
}

func TestInvalidMatcherType(t *testing.T) {
	_, err := NewMatcher(&MatcherConfig{
		Match: &MatchCriteria{Type: "invalid", Value: "test"},
	})
	assert.Error(t, err)
}

func TestMatcherResponse(t *testing.T) {
	m, err := NewMatcher(&MatcherConfig{
		Match:    &MatchCriteria{Type: "exact", Value: "test"},
		Response: &MessageResponse{Type: "text", Value: "response"},
	})
	require.NoError(t, err)

	assert.NotNil(t, m.Response())
	assert.True(t, m.ShouldRespond())
}

func TestMatcherNoResponse(t *testing.T) {
	m, err := NewMatcher(&MatcherConfig{
		Match:      &MatchCriteria{Type: "exact", Value: "test"},
		NoResponse: true,
	})
	require.NoError(t, err)

	assert.Nil(t, m.Response())
	assert.False(t, m.ShouldRespond())
}
