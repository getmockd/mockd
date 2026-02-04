package ai

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Provider name constants
const (
	ProviderOpenAI     = "openai"
	ProviderAnthropic  = "anthropic"
	ProviderOllama     = "ollama"
	ProviderOpenRouter = "openrouter"
)

// Environment variable names
const (
	EnvProvider = "MOCKD_AI_PROVIDER"
	EnvAPIKey   = "MOCKD_AI_API_KEY"
	EnvModel    = "MOCKD_AI_MODEL"
	EnvEndpoint = "MOCKD_AI_ENDPOINT"
)

// Default model names for each provider
const (
	DefaultOpenAIModel        = "gpt-4o-mini"
	DefaultAnthropicModel     = "claude-3-haiku-20240307"
	DefaultOllamaModel        = "llama3.2"
	DefaultOllamaEndpoint     = "http://localhost:11434"
	DefaultOpenRouterModel    = "google/gemini-2.5-flash"
	DefaultOpenRouterEndpoint = "https://openrouter.ai/api/v1"
)

// Config holds the configuration for AI providers.
type Config struct {
	// Provider is the AI provider to use ("openai", "anthropic", "ollama").
	Provider string `json:"provider" yaml:"provider"`

	// APIKey is the API key for the provider (not needed for Ollama).
	APIKey string `json:"apiKey,omitempty" yaml:"apiKey,omitempty"`

	// Model is the model name to use.
	Model string `json:"model,omitempty" yaml:"model,omitempty"`

	// Endpoint is the API endpoint URL (required for Ollama, optional for others).
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`

	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens int `json:"maxTokens,omitempty" yaml:"maxTokens,omitempty"`

	// Temperature controls randomness (0.0-2.0, default varies by provider).
	Temperature *float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`
}

// ConfigFromEnv reads AI configuration from environment variables.
func ConfigFromEnv() *Config {
	provider := os.Getenv(EnvProvider)
	if provider == "" {
		return nil
	}

	cfg := &Config{
		Provider: strings.ToLower(provider),
		APIKey:   os.Getenv(EnvAPIKey),
		Model:    os.Getenv(EnvModel),
		Endpoint: os.Getenv(EnvEndpoint),
	}

	// Set defaults based on provider
	cfg.applyDefaults()

	return cfg
}

// applyDefaults sets default values based on the provider.
func (c *Config) applyDefaults() {
	switch c.Provider {
	case ProviderOpenAI:
		if c.Model == "" {
			c.Model = DefaultOpenAIModel
		}
	case ProviderAnthropic:
		if c.Model == "" {
			c.Model = DefaultAnthropicModel
		}
	case ProviderOllama:
		if c.Model == "" {
			c.Model = DefaultOllamaModel
		}
		if c.Endpoint == "" {
			c.Endpoint = DefaultOllamaEndpoint
		}
	case ProviderOpenRouter:
		if c.Model == "" {
			c.Model = DefaultOpenRouterModel
		}
		if c.Endpoint == "" {
			c.Endpoint = DefaultOpenRouterEndpoint
		}
	}
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c == nil {
		return ErrProviderNotConfigured
	}

	if c.Provider == "" {
		return errors.New("provider is required")
	}

	switch c.Provider {
	case ProviderOpenAI, ProviderAnthropic, ProviderOpenRouter:
		if c.APIKey == "" {
			return fmt.Errorf("%w for %s", ErrAPIKeyMissing, c.Provider)
		}
	case ProviderOllama:
		if c.Endpoint == "" {
			return errors.New("endpoint is required for ollama")
		}
	default:
		return fmt.Errorf("unknown provider: %s", c.Provider)
	}

	return nil
}

// IsConfigured returns true if AI is configured via environment.
func IsConfigured() bool {
	return os.Getenv(EnvProvider) != ""
}

// SupportedProviders returns the list of supported provider names.
func SupportedProviders() []string {
	return []string{ProviderOpenAI, ProviderAnthropic, ProviderOllama, ProviderOpenRouter}
}
