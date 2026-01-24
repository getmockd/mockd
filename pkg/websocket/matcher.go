package websocket

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// MatcherConfig defines configuration for a message matcher.
type MatcherConfig struct {
	// Match defines the matching criteria.
	Match *MatchCriteria `json:"match"`
	// Response is the response to send when matched.
	Response *MessageResponse `json:"response,omitempty"`
	// NoResponse if true, matches but doesn't respond.
	NoResponse bool `json:"noResponse,omitempty"`
}

// MatchCriteria defines how to match a message.
type MatchCriteria struct {
	// Type is the match type: "exact", "regex", "json", "contains", "prefix", "suffix".
	Type string `json:"type"`
	// Value is the match value (string for exact/regex/contains/prefix/suffix).
	Value string `json:"value,omitempty"`
	// Path is the JSON path for json type (e.g., "$.action" or "action").
	Path string `json:"path,omitempty"`
	// MessageType restricts to specific message types: "text", "binary", or empty for both.
	MessageType string `json:"messageType,omitempty"`
}

// Matcher is a compiled message matcher.
type Matcher struct {
	matchType     string
	value         string
	jsonPath      string
	msgTypeFilter MessageType
	regex         *regexp.Regexp
	response      *MessageResponse
	noResponse    bool
}

// NewMatcher creates a new Matcher from configuration.
func NewMatcher(cfg *MatcherConfig) (*Matcher, error) {
	if cfg == nil || cfg.Match == nil {
		return nil, ErrInvalidMatcherType
	}

	m := &Matcher{
		matchType:  cfg.Match.Type,
		value:      cfg.Match.Value,
		jsonPath:   cfg.Match.Path,
		response:   cfg.Response,
		noResponse: cfg.NoResponse,
	}

	// Parse message type filter
	switch cfg.Match.MessageType {
	case "text":
		m.msgTypeFilter = MessageText
	case "binary":
		m.msgTypeFilter = MessageBinary
	default:
		m.msgTypeFilter = 0 // Match any
	}

	// Compile regex if needed
	if m.matchType == "regex" {
		r, err := regexp.Compile(m.value)
		if err != nil {
			return nil, err
		}
		m.regex = r
	}

	// Validate match type
	switch m.matchType {
	case "exact", "regex", "json", "contains", "prefix", "suffix":
		// Valid types
	default:
		return nil, ErrInvalidMatcherType
	}

	return m, nil
}

// Match returns true if the message matches this matcher.
func (m *Matcher) Match(msgType MessageType, data []byte) bool {
	// Check message type filter
	if m.msgTypeFilter != 0 && m.msgTypeFilter != msgType {
		return false
	}

	switch m.matchType {
	case "exact":
		return m.matchExact(data)
	case "regex":
		return m.matchRegex(data)
	case "json":
		return m.matchJSON(data)
	case "contains":
		return m.matchContains(data)
	case "prefix":
		return m.matchPrefix(data)
	case "suffix":
		return m.matchSuffix(data)
	default:
		return false
	}
}

// Response returns the response for this matcher.
func (m *Matcher) Response() *MessageResponse {
	if m.noResponse {
		return nil
	}
	return m.response
}

// ShouldRespond returns whether a response should be sent.
func (m *Matcher) ShouldRespond() bool {
	return !m.noResponse && m.response != nil
}

// matchExact performs exact string match.
func (m *Matcher) matchExact(data []byte) bool {
	return string(data) == m.value
}

// matchRegex performs regex match.
func (m *Matcher) matchRegex(data []byte) bool {
	if m.regex == nil {
		return false
	}
	return m.regex.Match(data)
}

// matchContains checks if data contains the value.
func (m *Matcher) matchContains(data []byte) bool {
	return strings.Contains(string(data), m.value)
}

// matchPrefix checks if data starts with the value.
func (m *Matcher) matchPrefix(data []byte) bool {
	return strings.HasPrefix(string(data), m.value)
}

// matchSuffix checks if data ends with the value.
func (m *Matcher) matchSuffix(data []byte) bool {
	return strings.HasSuffix(string(data), m.value)
}

// matchJSON performs JSON path matching.
func (m *Matcher) matchJSON(data []byte) bool {
	// Parse JSON
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return false
	}

	// Simple JSON path support: "$.field" or "field" or "$.field.subfield"
	path := strings.TrimPrefix(m.jsonPath, "$.")

	// Get the value at path
	value := getJSONValue(obj, path)
	if value == nil {
		return false
	}

	// Compare value
	switch v := value.(type) {
	case string:
		return v == m.value
	case float64:
		return m.value == toString(v)
	case bool:
		return m.value == toString(v)
	default:
		// Try JSON marshaling for complex types
		b, err := json.Marshal(v)
		if err != nil {
			return false
		}
		return string(b) == m.value
	}
}

// getJSONValue extracts a value from a JSON object using a dot-notation path.
func getJSONValue(obj map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = obj

	for _, part := range parts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[part]
			if !ok {
				return nil
			}
		default:
			return nil
		}
	}

	return current
}

// toString converts a value to string.
func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		// Handle integers
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		b, _ := json.Marshal(val)
		return string(b)
	}
}

// ExactMatcher creates a matcher that matches exact strings.
func ExactMatcher(value string, response *MessageResponse) *Matcher {
	m, _ := NewMatcher(&MatcherConfig{
		Match:    &MatchCriteria{Type: "exact", Value: value},
		Response: response,
	})
	return m
}

// RegexMatcher creates a matcher that matches regex patterns.
func RegexMatcher(pattern string, response *MessageResponse) (*Matcher, error) {
	return NewMatcher(&MatcherConfig{
		Match:    &MatchCriteria{Type: "regex", Value: pattern},
		Response: response,
	})
}

// JSONMatcher creates a matcher that matches JSON path values.
func JSONMatcher(path, value string, response *MessageResponse) *Matcher {
	m, _ := NewMatcher(&MatcherConfig{
		Match:    &MatchCriteria{Type: "json", Path: path, Value: value},
		Response: response,
	})
	return m
}

// ContainsMatcher creates a matcher that matches if message contains value.
func ContainsMatcher(value string, response *MessageResponse) *Matcher {
	m, _ := NewMatcher(&MatcherConfig{
		Match:    &MatchCriteria{Type: "contains", Value: value},
		Response: response,
	})
	return m
}
