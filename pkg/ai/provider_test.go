package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestConfigFromEnv(t *testing.T) {
	t.Run("returns nil when no provider set", func(t *testing.T) {
		t.Setenv(EnvProvider, "")
		cfg := ConfigFromEnv()
		if cfg != nil {
			t.Error("expected nil config when provider not set")
		}
	})

	t.Run("returns config when provider set", func(t *testing.T) {
		t.Setenv(EnvProvider, "openai")
		t.Setenv(EnvAPIKey, "test-key")
		t.Setenv(EnvModel, "gpt-4")

		cfg := ConfigFromEnv()
		if cfg == nil {
			t.Fatal("expected config to be returned")
		}
		if cfg.Provider != "openai" {
			t.Errorf("expected provider=openai, got %s", cfg.Provider)
		}
		if cfg.APIKey != "test-key" {
			t.Errorf("expected apiKey=test-key, got %s", cfg.APIKey)
		}
		if cfg.Model != "gpt-4" {
			t.Errorf("expected model=gpt-4, got %s", cfg.Model)
		}
	})

	t.Run("applies defaults for openai", func(t *testing.T) {
		t.Setenv(EnvProvider, "openai")
		t.Setenv(EnvAPIKey, "test-key")
		t.Setenv(EnvModel, "")

		cfg := ConfigFromEnv()
		if cfg.Model != DefaultOpenAIModel {
			t.Errorf("expected default model %s, got %s", DefaultOpenAIModel, cfg.Model)
		}
	})

	t.Run("applies defaults for ollama", func(t *testing.T) {
		t.Setenv(EnvProvider, "ollama")
		t.Setenv(EnvModel, "")
		t.Setenv(EnvEndpoint, "")

		cfg := ConfigFromEnv()
		if cfg.Model != DefaultOllamaModel {
			t.Errorf("expected default model %s, got %s", DefaultOllamaModel, cfg.Model)
		}
		if cfg.Endpoint != DefaultOllamaEndpoint {
			t.Errorf("expected default endpoint %s, got %s", DefaultOllamaEndpoint, cfg.Endpoint)
		}
	})
}

func TestConfigValidate(t *testing.T) {
	t.Run("returns error for nil config", func(t *testing.T) {
		var cfg *Config
		err := cfg.Validate()
		if err != ErrProviderNotConfigured {
			t.Errorf("expected ErrProviderNotConfigured, got %v", err)
		}
	})

	t.Run("returns error for empty provider", func(t *testing.T) {
		cfg := &Config{}
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for empty provider")
		}
	})

	t.Run("returns error for missing API key (openai)", func(t *testing.T) {
		cfg := &Config{
			Provider: ProviderOpenAI,
		}
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for missing API key")
		}
	})

	t.Run("returns error for missing API key (anthropic)", func(t *testing.T) {
		cfg := &Config{
			Provider: ProviderAnthropic,
		}
		err := cfg.Validate()
		if err == nil {
			t.Error("expected error for missing API key")
		}
	})

	t.Run("ollama does not require API key", func(t *testing.T) {
		cfg := &Config{
			Provider: ProviderOllama,
			Endpoint: "http://localhost:11434",
		}
		err := cfg.Validate()
		if err != nil {
			t.Errorf("expected no error for ollama without API key, got %v", err)
		}
	})

	t.Run("valid openai config", func(t *testing.T) {
		cfg := &Config{
			Provider: ProviderOpenAI,
			APIKey:   "test-key",
		}
		err := cfg.Validate()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}

func TestNewProvider(t *testing.T) {
	t.Run("returns error for nil config", func(t *testing.T) {
		_, err := NewProvider(nil)
		if err != ErrProviderNotConfigured {
			t.Errorf("expected ErrProviderNotConfigured, got %v", err)
		}
	})

	t.Run("creates openai provider", func(t *testing.T) {
		cfg := &Config{
			Provider: ProviderOpenAI,
			APIKey:   "test-key",
		}
		p, err := NewProvider(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if p.Name() != ProviderOpenAI {
			t.Errorf("expected name %s, got %s", ProviderOpenAI, p.Name())
		}
	})

	t.Run("creates anthropic provider", func(t *testing.T) {
		cfg := &Config{
			Provider: ProviderAnthropic,
			APIKey:   "test-key",
		}
		p, err := NewProvider(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if p.Name() != ProviderAnthropic {
			t.Errorf("expected name %s, got %s", ProviderAnthropic, p.Name())
		}
	})

	t.Run("creates ollama provider", func(t *testing.T) {
		cfg := &Config{
			Provider: ProviderOllama,
			Endpoint: "http://localhost:11434",
		}
		p, err := NewProvider(cfg)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if p.Name() != ProviderOllama {
			t.Errorf("expected name %s, got %s", ProviderOllama, p.Name())
		}
	})

	t.Run("returns error for unknown provider", func(t *testing.T) {
		cfg := &Config{
			Provider: "unknown",
		}
		_, err := NewProvider(cfg)
		if err == nil {
			t.Error("expected error for unknown provider")
		}
	})
}

func TestOpenAIProvider(t *testing.T) {
	t.Run("Generate makes correct API call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/chat/completions" {
				t.Errorf("expected /chat/completions, got %s", r.URL.Path)
			}
			if r.Header.Get("Authorization") != "Bearer test-key" {
				t.Errorf("expected Authorization header")
			}

			// Return mock response
			resp := openAIChatResponse{
				Choices: []struct {
					Index   int `json:"index"`
					Message struct {
						Role    string `json:"role"`
						Content string `json:"content"`
					} `json:"message"`
					FinishReason string `json:"finish_reason"`
				}{
					{
						Message: struct {
							Role    string `json:"role"`
							Content string `json:"content"`
						}{
							Role:    "assistant",
							Content: "test@example.com",
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider, err := NewOpenAIProvider(&Config{
			APIKey:   "test-key",
			Endpoint: server.URL,
		})
		if err != nil {
			t.Fatalf("failed to create provider: %v", err)
		}

		resp, err := provider.Generate(context.Background(), &GenerateRequest{
			FieldName: "email",
			FieldType: "string",
			Format:    "email",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.Value != "test@example.com" {
			t.Errorf("expected test@example.com, got %v", resp.Value)
		}
	})

	t.Run("handles rate limiting", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := openAIChatResponse{
				Error: &openAIError{
					Code:    "rate_limit_exceeded",
					Message: "Rate limit exceeded",
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider, _ := NewOpenAIProvider(&Config{
			APIKey:   "test-key",
			Endpoint: server.URL,
		})

		_, err := provider.Generate(context.Background(), &GenerateRequest{
			FieldName: "email",
			FieldType: "string",
		})

		if err != ErrRateLimited {
			t.Errorf("expected ErrRateLimited, got %v", err)
		}
	})
}

func TestAnthropicProvider(t *testing.T) {
	t.Run("Generate makes correct API call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/messages" {
				t.Errorf("expected /messages, got %s", r.URL.Path)
			}
			if r.Header.Get("x-api-key") != "test-key" {
				t.Errorf("expected x-api-key header")
			}
			if r.Header.Get("anthropic-version") != anthropicAPIVersion {
				t.Errorf("expected anthropic-version header")
			}

			// Return mock response
			resp := anthropicResponse{
				Content: []anthropicContent{
					{Type: "text", Text: "John Doe"},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider, err := NewAnthropicProvider(&Config{
			APIKey:   "test-key",
			Endpoint: server.URL,
		})
		if err != nil {
			t.Fatalf("failed to create provider: %v", err)
		}

		resp, err := provider.Generate(context.Background(), &GenerateRequest{
			FieldName: "name",
			FieldType: "string",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.Value != "John Doe" {
			t.Errorf("expected John Doe, got %v", resp.Value)
		}
	})
}

func TestOllamaProvider(t *testing.T) {
	t.Run("Generate makes correct API call", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.URL.Path != "/api/chat" {
				t.Errorf("expected /api/chat, got %s", r.URL.Path)
			}

			// Return mock response
			resp := ollamaResponse{
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{
					Role:    "assistant",
					Content: "42",
				},
				Done: true,
			}
			json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()

		provider, err := NewOllamaProvider(&Config{
			Endpoint: server.URL,
		})
		if err != nil {
			t.Fatalf("failed to create provider: %v", err)
		}

		resp, err := provider.Generate(context.Background(), &GenerateRequest{
			FieldName: "age",
			FieldType: "integer",
		})
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if resp.Value != int64(42) {
			t.Errorf("expected 42, got %v (type %T)", resp.Value, resp.Value)
		}
	})
}

func TestParseGeneratedValue(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		fieldType string
		expected  interface{}
		hasError  bool
	}{
		{
			name:      "string value",
			response:  "hello world",
			fieldType: "string",
			expected:  "hello world",
		},
		{
			name:      "quoted string",
			response:  `"hello world"`,
			fieldType: "string",
			expected:  "hello world",
		},
		{
			name:      "integer value",
			response:  "42",
			fieldType: "integer",
			expected:  int64(42),
		},
		{
			name:      "float as integer",
			response:  "42.0",
			fieldType: "integer",
			expected:  int64(42),
		},
		{
			name:      "number value",
			response:  "3.14",
			fieldType: "number",
			expected:  3.14,
		},
		{
			name:      "boolean true",
			response:  "true",
			fieldType: "boolean",
			expected:  true,
		},
		{
			name:      "boolean false",
			response:  "false",
			fieldType: "boolean",
			expected:  false,
		},
		{
			name:      "object value",
			response:  `{"name": "test"}`,
			fieldType: "object",
			expected:  map[string]interface{}{"name": "test"},
		},
		{
			name:      "array value",
			response:  `[1, 2, 3]`,
			fieldType: "array",
			expected:  []interface{}{float64(1), float64(2), float64(3)},
		},
		{
			name:      "code block stripped",
			response:  "```json\n{\"name\": \"test\"}\n```",
			fieldType: "object",
			expected:  map[string]interface{}{"name": "test"},
		},
		{
			name:      "invalid integer",
			response:  "not a number",
			fieldType: "integer",
			hasError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseGeneratedValue(tt.response, tt.fieldType)
			if tt.hasError {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Compare results
			expectedJSON, _ := json.Marshal(tt.expected)
			resultJSON, _ := json.Marshal(result)
			if string(expectedJSON) != string(resultJSON) {
				t.Errorf("expected %s, got %s", expectedJSON, resultJSON)
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	t.Run("includes basic fields", func(t *testing.T) {
		req := &GenerateRequest{
			FieldName: "email",
			FieldType: "string",
		}
		prompt := buildPrompt(req)
		if !containsString(prompt, "email") {
			t.Error("prompt should contain field name")
		}
		if !containsString(prompt, "string") {
			t.Error("prompt should contain field type")
		}
	})

	t.Run("includes format", func(t *testing.T) {
		req := &GenerateRequest{
			FieldName: "email",
			FieldType: "string",
			Format:    "email",
		}
		prompt := buildPrompt(req)
		if !containsString(prompt, "Format: email") {
			t.Error("prompt should contain format")
		}
	})

	t.Run("includes description", func(t *testing.T) {
		req := &GenerateRequest{
			FieldName:   "email",
			FieldType:   "string",
			Description: "User's primary email address",
		}
		prompt := buildPrompt(req)
		if !containsString(prompt, "User's primary email address") {
			t.Error("prompt should contain description")
		}
	})

	t.Run("includes constraints", func(t *testing.T) {
		minLen := 5
		maxLen := 100
		req := &GenerateRequest{
			FieldName: "username",
			FieldType: "string",
			Constraints: &FieldConstraints{
				MinLength: &minLen,
				MaxLength: &maxLen,
			},
		}
		prompt := buildPrompt(req)
		if !containsString(prompt, "Minimum length: 5") {
			t.Error("prompt should contain min length")
		}
		if !containsString(prompt, "Maximum length: 100") {
			t.Error("prompt should contain max length")
		}
	})
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSupportedProviders(t *testing.T) {
	providers := SupportedProviders()
	if len(providers) != 4 {
		t.Errorf("expected 4 providers, got %d", len(providers))
	}

	expected := map[string]bool{
		ProviderOpenAI:     true,
		ProviderAnthropic:  true,
		ProviderOllama:     true,
		ProviderOpenRouter: true,
	}

	for _, p := range providers {
		if !expected[p] {
			t.Errorf("unexpected provider: %s", p)
		}
	}
}

func TestIsConfigured(t *testing.T) {
	t.Run("returns false when not configured", func(t *testing.T) {
		t.Setenv(EnvProvider, "")
		if IsConfigured() {
			t.Error("expected false when provider not set")
		}
	})

	t.Run("returns true when configured", func(t *testing.T) {
		t.Setenv(EnvProvider, "openai")
		if !IsConfigured() {
			t.Error("expected true when provider is set")
		}
	})
}
