package sse

import (
	"encoding/json"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/getmockd/mockd/internal/id"
)

// TemplateGeneratorFunc is a function that generates events from template parameters.
type TemplateGeneratorFunc func(params map[string]interface{}) []SSEEventDef

// TemplateRegistry manages built-in SSE templates.
type TemplateRegistry struct {
	templates map[string]TemplateGeneratorFunc
	mu        sync.RWMutex
}

// NewTemplateRegistry creates a new template registry with built-in templates.
func NewTemplateRegistry() *TemplateRegistry {
	r := &TemplateRegistry{
		templates: make(map[string]TemplateGeneratorFunc),
	}

	// Register built-in templates
	r.Register(TemplateOpenAIChat, generateOpenAIChatEvents)
	r.Register(TemplateNotificationStream, generateNotificationEvents)

	return r
}

// Register adds a template generator.
func (r *TemplateRegistry) Register(name string, generator TemplateGeneratorFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates[name] = generator
}

// Get returns a template generator by name.
func (r *TemplateRegistry) Get(name string) (TemplateGeneratorFunc, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	gen, ok := r.templates[name]
	return gen, ok
}

// List returns all registered template names.
func (r *TemplateRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.templates))
	for name := range r.templates {
		names = append(names, name)
	}
	return names
}

// generateOpenAIChatEvents generates OpenAI-compatible streaming events.
func generateOpenAIChatEvents(params map[string]interface{}) []SSEEventDef {
	// Extract parameters
	tokens := getStringSlice(params, "tokens")
	if len(tokens) == 0 {
		// Default tokens
		tokens = []string{"Hello", "!", " How", " can", " I", " help", " you", "?"}
	}

	model := getString(params, "model", "gpt-4-mock")
	finishReason := getString(params, "finishReason", "stop")
	includeDone := getBool(params, "includeDone", true)
	delayPerToken := getInt(params, "delayPerToken", 50)

	// Generate a chat completion ID
	chatID := "chatcmpl-" + id.Alphanumeric(24)
	created := time.Now().Unix()

	events := make([]SSEEventDef, 0, len(tokens)+2)

	// Generate token events
	for i, token := range tokens {
		isLast := i == len(tokens)-1
		var fr interface{}
		if isLast {
			fr = finishReason
		}

		data := map[string]interface{}{
			"id":      chatID,
			"object":  "chat.completion.chunk",
			"created": created,
			"model":   model,
			"choices": []map[string]interface{}{
				{
					"index": 0,
					"delta": map[string]interface{}{
						"content": token,
					},
					"finish_reason": fr,
				},
			},
		}

		delay := delayPerToken
		events = append(events, SSEEventDef{
			Data:  data,
			Delay: &delay,
		})
	}

	// Add [DONE] event if requested
	if includeDone {
		events = append(events, SSEEventDef{
			Data: "[DONE]",
		})
	}

	return events
}

// generateNotificationEvents generates generic notification stream events.
func generateNotificationEvents(params map[string]interface{}) []SSEEventDef {
	messages := getMapSlice(params, "messages")
	if len(messages) == 0 {
		return nil
	}

	interval := getInt(params, "interval", 1000)
	loop := getBool(params, "loop", false)

	events := make([]SSEEventDef, 0, len(messages))
	for _, msg := range messages {
		eventType := getString(msg, "type", "notification")
		payload := msg["payload"]

		delay := interval
		events = append(events, SSEEventDef{
			Type:  eventType,
			Data:  payload,
			Delay: &delay,
		})
	}

	// If loop is enabled, we'd need to handle this at the stream level
	_ = loop

	return events
}

// Helper functions for parameter extraction

func getString(params map[string]interface{}, key, defaultValue string) string {
	if v, ok := params[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultValue
}

func getInt(params map[string]interface{}, key string, defaultValue int) int {
	if v, ok := params[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return defaultValue
}

func getBool(params map[string]interface{}, key string, defaultValue bool) bool {
	if v, ok := params[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return defaultValue
}

func getStringSlice(params map[string]interface{}, key string) []string {
	if v, ok := params[key]; ok {
		switch s := v.(type) {
		case []string:
			return s
		case []interface{}:
			result := make([]string, 0, len(s))
			for _, item := range s {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return nil
}

func getMapSlice(params map[string]interface{}, key string) []map[string]interface{} {
	if v, ok := params[key]; ok {
		switch s := v.(type) {
		case []map[string]interface{}:
			return s
		case []interface{}:
			result := make([]map[string]interface{}, 0, len(s))
			for _, item := range s {
				if m, ok := item.(map[string]interface{}); ok {
					result = append(result, m)
				}
			}
			return result
		}
	}
	return nil
}

// Random placeholder processing

var rngMu sync.Mutex
var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// processRandomPlaceholder processes a string with random placeholders.
// Supports: $random(min,max), $uuid, $timestamp, $pick(a,b,c)
func processRandomPlaceholder(s string) interface{} {
	// $uuid
	if s == "$uuid" {
		return generateUUID()
	}

	// $timestamp
	if s == "$timestamp" {
		return time.Now().Unix()
	}

	// $random(min,max)
	if strings.HasPrefix(s, "$random(") && strings.HasSuffix(s, ")") {
		inner := s[8 : len(s)-1]
		parts := strings.Split(inner, ",")
		if len(parts) == 2 {
			min := parseIntOr(strings.TrimSpace(parts[0]), 0)
			max := parseIntOr(strings.TrimSpace(parts[1]), 100)
			return randomInt(min, max)
		}
	}

	// $pick(a,b,c)
	if strings.HasPrefix(s, "$pick(") && strings.HasSuffix(s, ")") {
		inner := s[6 : len(s)-1]
		parts := strings.Split(inner, ",")
		if len(parts) > 0 {
			// Trim spaces from each option
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			return randomPick(parts)
		}
	}

	// Not a placeholder, return as-is
	return s
}

// parseIntOr parses an integer or returns a default value.
func parseIntOr(s string, defaultValue int) int {
	n := 0
	negative := false
	start := 0

	if len(s) > 0 && s[0] == '-' {
		negative = true
		start = 1
	}

	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return defaultValue
		}
		n = n*10 + int(s[i]-'0')
	}

	if negative {
		n = -n
	}
	return n
}

// randomInt returns a random integer in [min, max].
func randomInt(min, max int) int {
	if max <= min {
		return min
	}
	rngMu.Lock()
	defer rngMu.Unlock()
	return min + rng.Intn(max-min+1)
}

// randomPick returns a random element from the slice.
func randomPick(options []string) string {
	if len(options) == 0 {
		return ""
	}
	rngMu.Lock()
	defer rngMu.Unlock()
	return options[rng.Intn(len(options))]
}

// generateUUID generates a random UUID v4.
func generateUUID() string {
	uuid := make([]byte, 16)
	rngMu.Lock()
	rng.Read(uuid)
	rngMu.Unlock()

	// Set version (4) and variant bits
	uuid[6] = (uuid[6] & 0x0f) | 0x40
	uuid[8] = (uuid[8] & 0x3f) | 0x80

	return formatUUID(uuid)
}

// formatUUID formats a 16-byte UUID as a string.
func formatUUID(uuid []byte) string {
	const hexChars = "0123456789abcdef"
	result := make([]byte, 36)

	hex := func(b byte) (byte, byte) {
		return hexChars[b>>4], hexChars[b&0x0f]
	}

	result[0], result[1] = hex(uuid[0])
	result[2], result[3] = hex(uuid[1])
	result[4], result[5] = hex(uuid[2])
	result[6], result[7] = hex(uuid[3])
	result[8] = '-'
	result[9], result[10] = hex(uuid[4])
	result[11], result[12] = hex(uuid[5])
	result[13] = '-'
	result[14], result[15] = hex(uuid[6])
	result[16], result[17] = hex(uuid[7])
	result[18] = '-'
	result[19], result[20] = hex(uuid[8])
	result[21], result[22] = hex(uuid[9])
	result[23] = '-'
	result[24], result[25] = hex(uuid[10])
	result[26], result[27] = hex(uuid[11])
	result[28], result[29] = hex(uuid[12])
	result[30], result[31] = hex(uuid[13])
	result[32], result[33] = hex(uuid[14])
	result[34], result[35] = hex(uuid[15])

	return string(result)
}

// OpenAIChatResponse represents an OpenAI chat completion chunk.
type OpenAIChatResponse struct {
	ID      string             `json:"id"`
	Object  string             `json:"object"`
	Created int64              `json:"created"`
	Model   string             `json:"model"`
	Choices []OpenAIChatChoice `json:"choices"`
}

// OpenAIChatChoice represents a choice in the completion.
type OpenAIChatChoice struct {
	Index        int             `json:"index"`
	Delta        OpenAIChatDelta `json:"delta"`
	FinishReason interface{}     `json:"finish_reason"`
}

// OpenAIChatDelta represents the delta content.
type OpenAIChatDelta struct {
	Content string `json:"content,omitempty"`
	Role    string `json:"role,omitempty"`
}

// FormatOpenAIChatChunk formats an OpenAI chat completion chunk.
func FormatOpenAIChatChunk(chatID, model, content string, created int64, finishReason interface{}) string {
	response := OpenAIChatResponse{
		ID:      chatID,
		Object:  "chat.completion.chunk",
		Created: created,
		Model:   model,
		Choices: []OpenAIChatChoice{
			{
				Index: 0,
				Delta: OpenAIChatDelta{
					Content: content,
				},
				FinishReason: finishReason,
			},
		},
	}

	data, _ := json.Marshal(response)
	return string(data)
}
