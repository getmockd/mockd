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
	ollamaTimeout = 60 * time.Second // Ollama can be slower, especially for first requests
)

// OllamaProvider implements the Provider interface using a local Ollama instance.
type OllamaProvider struct {
	endpoint   string
	model      string
	httpClient *http.Client
}

// NewOllamaProvider creates a new Ollama provider.
func NewOllamaProvider(cfg *Config) (*OllamaProvider, error) {
	endpoint := cfg.Endpoint
	if endpoint == "" {
		endpoint = DefaultOllamaEndpoint
	}

	model := cfg.Model
	if model == "" {
		model = DefaultOllamaModel
	}

	return &OllamaProvider{
		endpoint: strings.TrimSuffix(endpoint, "/"),
		model:    model,
		httpClient: &http.Client{
			Timeout: ollamaTimeout,
		},
	}, nil
}

// Name returns the provider name.
func (p *OllamaProvider) Name() string {
	return ProviderOllama
}

// Generate produces a value based on the request.
func (p *OllamaProvider) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	prompt := buildPrompt(req)

	response, err := p.callAPI(ctx, prompt)
	if err != nil {
		return nil, err
	}

	value, err := parseGeneratedValue(response, req.FieldType)
	if err != nil {
		return nil, &ProviderError{
			Provider: ProviderOllama,
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
func (p *OllamaProvider) GenerateBatch(ctx context.Context, reqs []*GenerateRequest) ([]*GenerateResponse, error) {
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
			Provider: ProviderOllama,
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

// ollamaRequest represents the request to Ollama's chat API.
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

// ollamaResponse represents the response from Ollama.
type ollamaResponse struct {
	Model     string `json:"model"`
	CreatedAt string `json:"created_at"`
	Message   struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Done               bool   `json:"done"`
	TotalDuration      int64  `json:"total_duration"`
	LoadDuration       int64  `json:"load_duration"`
	PromptEvalCount    int    `json:"prompt_eval_count"`
	PromptEvalDuration int64  `json:"prompt_eval_duration"`
	EvalCount          int    `json:"eval_count"`
	EvalDuration       int64  `json:"eval_duration"`
	Error              string `json:"error,omitempty"`
}

func (p *OllamaProvider) callAPI(ctx context.Context, prompt string) (string, error) {
	reqBody := ollamaRequest{
		Model: p.model,
		Messages: []ollamaMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: prompt,
			},
		},
		Stream: false,
		Options: &ollamaOptions{
			Temperature: 0.7,
			NumPredict:  500,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint+"/api/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", &ProviderError{
			Provider: ProviderOllama,
			Message:  "API request failed - is Ollama running?",
			Cause:    err,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if ollamaResp.Error != "" {
		return "", &ProviderError{
			Provider: ProviderOllama,
			Message:  ollamaResp.Error,
		}
	}

	if resp.StatusCode != http.StatusOK {
		return "", &ProviderError{
			Provider: ProviderOllama,
			Message:  fmt.Sprintf("API returned status %d: %s", resp.StatusCode, string(body)),
		}
	}

	return strings.TrimSpace(ollamaResp.Message.Content), nil
}

// CheckConnection verifies that Ollama is running and the model is available.
func (p *OllamaProvider) CheckConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.endpoint+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return &ProviderError{
			Provider: ProviderOllama,
			Message:  "cannot connect to Ollama - is it running?",
			Cause:    err,
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &ProviderError{
			Provider: ProviderOllama,
			Message:  fmt.Sprintf("Ollama returned status %d", resp.StatusCode),
		}
	}

	return nil
}
