package ai

import (
	"context"
	"errors"
	"fmt"
)

// Provider defines the interface for AI mock data generation providers.
type Provider interface {
	// Generate produces a value based on the provided request.
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)

	// GenerateBatch produces multiple values in a single request for efficiency.
	GenerateBatch(ctx context.Context, reqs []*GenerateRequest) ([]*GenerateResponse, error)

	// Name returns the provider identifier (e.g., "openai", "anthropic", "ollama").
	Name() string
}

// GenerateRequest contains the input for AI generation.
type GenerateRequest struct {
	// Schema is the OpenAPI schema or JSON schema for the field (optional).
	Schema interface{} `json:"schema,omitempty"`

	// Context provides additional context about the data being generated.
	Context string `json:"context,omitempty"`

	// FieldName is the name of the field being generated.
	FieldName string `json:"fieldName"`

	// FieldType is the expected type (e.g., "string", "integer", "object").
	FieldType string `json:"fieldType"`

	// Format is the optional format hint (e.g., "email", "date", "uuid").
	Format string `json:"format,omitempty"`

	// Examples are sample values from the schema to guide generation.
	Examples []string `json:"examples,omitempty"`

	// Description is a human-readable description of the field.
	Description string `json:"description,omitempty"`

	// Constraints contains validation constraints (min, max, pattern, etc.).
	Constraints *FieldConstraints `json:"constraints,omitempty"`
}

// FieldConstraints contains validation constraints for field generation.
type FieldConstraints struct {
	// MinLength for strings
	MinLength *int `json:"minLength,omitempty"`
	// MaxLength for strings
	MaxLength *int `json:"maxLength,omitempty"`
	// Minimum for numbers
	Minimum *float64 `json:"minimum,omitempty"`
	// Maximum for numbers
	Maximum *float64 `json:"maximum,omitempty"`
	// Pattern is a regex pattern the value should match
	Pattern string `json:"pattern,omitempty"`
	// Enum is a list of allowed values
	Enum []interface{} `json:"enum,omitempty"`
}

// GenerateResponse contains the AI-generated value.
type GenerateResponse struct {
	// Value is the generated value, typed appropriately.
	Value interface{} `json:"value"`

	// RawResponse is the raw response from the AI provider (for debugging).
	RawResponse string `json:"rawResponse,omitempty"`

	// TokensUsed is the number of tokens consumed (if available).
	TokensUsed int `json:"tokensUsed,omitempty"`
}

// Common errors
var (
	// ErrProviderNotConfigured is returned when the provider is not properly configured.
	ErrProviderNotConfigured = errors.New("AI provider not configured")

	// ErrAPIKeyMissing is returned when the API key is not set.
	ErrAPIKeyMissing = errors.New("API key is required")

	// ErrModelNotSupported is returned when the requested model is not supported.
	ErrModelNotSupported = errors.New("model not supported by provider")

	// ErrRateLimited is returned when the provider rate limits the request.
	ErrRateLimited = errors.New("rate limited by provider")

	// ErrGenerationFailed is returned when the AI generation fails.
	ErrGenerationFailed = errors.New("failed to generate value")

	// ErrInvalidResponse is returned when the AI response cannot be parsed.
	ErrInvalidResponse = errors.New("invalid response from provider")
)

// ProviderError wraps errors from AI providers with additional context.
type ProviderError struct {
	Provider string
	Message  string
	Cause    error
}

func (e *ProviderError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Provider, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Provider, e.Message)
}

func (e *ProviderError) Unwrap() error {
	return e.Cause
}

// NewProvider creates a provider based on the configuration.
func NewProvider(cfg *Config) (Provider, error) {
	if cfg == nil {
		return nil, ErrProviderNotConfigured
	}

	switch cfg.Provider {
	case ProviderOpenAI:
		return NewOpenAIProvider(cfg)
	case ProviderAnthropic:
		return NewAnthropicProvider(cfg)
	case ProviderOllama:
		return NewOllamaProvider(cfg)
	case ProviderOpenRouter:
		// OpenRouter uses an OpenAI-compatible API with a different base URL.
		if cfg.Endpoint == "" {
			cfg.Endpoint = DefaultOpenRouterEndpoint
		}
		return NewOpenAIProvider(cfg)
	default:
		return nil, fmt.Errorf("%w: unknown provider %q", ErrProviderNotConfigured, cfg.Provider)
	}
}
