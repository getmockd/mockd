package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	anthropicDefaultEndpoint = "https://api.anthropic.com/v1"
	anthropicAPIVersion      = "2023-06-01"
	anthropicTimeout         = 30 * time.Second
)

// AnthropicProvider implements the Provider interface using Anthropic's API.
type AnthropicProvider struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
	maxTokens  int
}

// NewAnthropicProvider creates a new Anthropic provider.
func NewAnthropicProvider(cfg *Config) (*AnthropicProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%w for Anthropic", ErrAPIKeyMissing)
	}

	baseURL := cfg.Endpoint
	if baseURL == "" {
		baseURL = anthropicDefaultEndpoint
	}

	model := cfg.Model
	if model == "" {
		model = DefaultAnthropicModel
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 500
	}

	return &AnthropicProvider{
		apiKey:  cfg.APIKey,
		model:   model,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: anthropicTimeout,
		},
		maxTokens: maxTokens,
	}, nil
}

// Name returns the provider name.
func (p *AnthropicProvider) Name() string {
	return ProviderAnthropic
}

// Generate produces a value based on the request.
func (p *AnthropicProvider) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	prompt := buildPrompt(req)

	response, err := p.callAPI(ctx, prompt)
	if err != nil {
		return nil, err
	}

	value, err := parseGeneratedValue(response, req.FieldType)
	if err != nil {
		return nil, &ProviderError{
			Provider: ProviderAnthropic,
			Message:  "failed to parse response",
			Cause:    err,
		}
	}

	return &GenerateResponse{
		Value:       value,
		RawResponse: response,
	}, nil
}

// GenerateBatch produces multiple values in a single request.
func (p *AnthropicProvider) GenerateBatch(ctx context.Context, reqs []*GenerateRequest) ([]*GenerateResponse, error) {
	if len(reqs) == 0 {
		return nil, nil
	}

	prompt := buildBatchPrompt(reqs)

	response, err := p.callAPI(ctx, prompt)
	if err != nil {
		return nil, err
	}

	values, err := parseBatchResponse(response, reqs)
	if err != nil {
		return nil, &ProviderError{
			Provider: ProviderAnthropic,
			Message:  "failed to parse batch response",
			Cause:    err,
		}
	}

	responses := make([]*GenerateResponse, len(values))
	for i, v := range values {
		responses[i] = &GenerateResponse{
			Value:       v,
			RawResponse: response,
		}
	}

	return responses, nil
}

// anthropicRequest represents the request to Anthropic's messages API.
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse represents the response from Anthropic.
type anthropicResponse struct {
	ID           string             `json:"id"`
	Type         string             `json:"type"`
	Role         string             `json:"role"`
	Content      []anthropicContent `json:"content"`
	Model        string             `json:"model"`
	StopReason   string             `json:"stop_reason"`
	StopSequence string             `json:"stop_sequence,omitempty"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *anthropicError `json:"error,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (p *AnthropicProvider) callAPI(ctx context.Context, prompt string) (string, error) {
	reqBody := anthropicRequest{
		Model:     p.model,
		MaxTokens: p.maxTokens,
		System:    systemPrompt,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", &ProviderError{
			Provider: ProviderAnthropic,
			Message:  "API request failed",
			Cause:    err,
		}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if anthropicResp.Error != nil {
		if anthropicResp.Error.Type == "rate_limit_error" {
			return "", ErrRateLimited
		}
		return "", &ProviderError{
			Provider: ProviderAnthropic,
			Message:  anthropicResp.Error.Message,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return "", &ProviderError{
			Provider: ProviderAnthropic,
			Message:  fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
		}
	}

	if len(anthropicResp.Content) == 0 {
		return "", ErrInvalidResponse
	}

	// Find text content
	for _, content := range anthropicResp.Content {
		if content.Type == "text" {
			return strings.TrimSpace(content.Text), nil
		}
	}

	return "", ErrInvalidResponse
}
