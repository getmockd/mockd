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
	openAIDefaultEndpoint = "https://api.openai.com/v1"
	openAITimeout         = 30 * time.Second
)

// OpenAIProvider implements the Provider interface using OpenAI's API.
// It also supports OpenAI-compatible endpoints like OpenRouter.
type OpenAIProvider struct {
	apiKey       string
	model        string
	baseURL      string
	httpClient   *http.Client
	maxTokens    int
	extraHeaders map[string]string // Additional headers (e.g., OpenRouter attribution)
}

// NewOpenAIProvider creates a new OpenAI provider.
func NewOpenAIProvider(cfg *Config) (*OpenAIProvider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("%w for OpenAI", ErrAPIKeyMissing)
	}

	baseURL := cfg.Endpoint
	if baseURL == "" {
		baseURL = openAIDefaultEndpoint
	}

	model := cfg.Model
	if model == "" {
		model = DefaultOpenAIModel
	}

	maxTokens := cfg.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	p := &OpenAIProvider{
		apiKey:  cfg.APIKey,
		model:   model,
		baseURL: strings.TrimSuffix(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: openAITimeout,
		},
		maxTokens: maxTokens,
	}

	// Add OpenRouter attribution headers when using their endpoint.
	if cfg.Provider == ProviderOpenRouter || strings.Contains(baseURL, "openrouter.ai") {
		p.extraHeaders = map[string]string{
			"HTTP-Referer": "https://mockd.io",
			"X-Title":      "mockd",
		}
	}

	return p, nil
}

// Name returns the provider name.
func (p *OpenAIProvider) Name() string {
	return ProviderOpenAI
}

// Generate produces a value based on the request.
func (p *OpenAIProvider) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	prompt := buildPrompt(req)

	response, err := p.callAPI(ctx, prompt)
	if err != nil {
		return nil, err
	}

	value, err := parseGeneratedValue(response, req.FieldType)
	if err != nil {
		return nil, &ProviderError{
			Provider: ProviderOpenAI,
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
func (p *OpenAIProvider) GenerateBatch(ctx context.Context, reqs []*GenerateRequest) ([]*GenerateResponse, error) {
	if len(reqs) == 0 {
		return nil, nil
	}

	// Build a combined prompt for all fields
	prompt := buildBatchPrompt(reqs)

	response, err := p.callAPI(ctx, prompt)
	if err != nil {
		return nil, err
	}

	values, err := parseBatchResponse(response, reqs)
	if err != nil {
		return nil, &ProviderError{
			Provider: ProviderOpenAI,
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

// openAIChatRequest represents the request to OpenAI chat completions API.
type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openAIChatResponse represents the response from OpenAI.
type openAIChatResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *openAIError `json:"error,omitempty"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code"`
}

func (p *OpenAIProvider) callAPI(ctx context.Context, prompt string) (string, error) {
	reqBody := openAIChatRequest{
		Model: p.model,
		Messages: []openAIMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		MaxTokens:   p.maxTokens,
		Temperature: 0.7,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	for k, v := range p.extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", &ProviderError{
			Provider: ProviderOpenAI,
			Message:  "API request failed",
			Cause:    err,
		}
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var chatResp openAIChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if chatResp.Error != nil {
		if chatResp.Error.Code == "rate_limit_exceeded" {
			return "", ErrRateLimited
		}
		return "", &ProviderError{
			Provider: ProviderOpenAI,
			Message:  chatResp.Error.Message,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return "", &ProviderError{
			Provider: ProviderOpenAI,
			Message:  fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
		}
	}

	if len(chatResp.Choices) == 0 {
		return "", ErrInvalidResponse
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}
