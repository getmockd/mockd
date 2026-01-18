// Package runtime provides the control plane client for runtime mode.
package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// DefaultHeartbeatInterval is the default interval for heartbeat requests.
const DefaultHeartbeatInterval = 30 * time.Second

// DefaultHTTPTimeout is the default timeout for HTTP requests.
const DefaultHTTPTimeout = 10 * time.Second

// Config holds configuration for the runtime client.
type Config struct {
	ControlPlaneURL   string
	Token             string
	Name              string
	URL               string // URL where this runtime is accessible
	Labels            map[string]string
	Version           string
	HeartbeatInterval time.Duration
}

// Client is the control plane client for runtime mode.
type Client struct {
	config     Config
	httpClient *http.Client
	runtimeID  string

	// Deployment state
	mu          sync.RWMutex
	deployments map[string]*Deployment // key: deployment ID
	mocksByPath map[string]*Deployment // key: URL path -> deployment
}

// Deployment represents a deployed mock on this runtime.
type Deployment struct {
	ID          string          `json:"id"`
	MockID      string          `json:"mockId"`
	MockVersion int             `json:"mockVersion"`
	URLPath     string          `json:"urlPath"`
	Content     json.RawMessage `json:"content"`
	DeployedAt  time.Time       `json:"deployedAt"`
}

// RegistrationResponse is the response from runtime registration.
type RegistrationResponse struct {
	ID        string            `json:"id"`
	Token     string            `json:"token"`
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Labels    map[string]string `json:"labels"`
	Status    string            `json:"status"`
	CreatedAt time.Time         `json:"createdAt"`
}

// HeartbeatRequest is the request body for heartbeat.
type HeartbeatRequest struct {
	Status      string                `json:"status"`
	Version     string                `json:"version"`
	Deployments []DeploymentInfoShort `json:"deployments"`
}

// DeploymentInfoShort is a short deployment info for heartbeat.
type DeploymentInfoShort struct {
	MockID  string `json:"mockId"`
	Version int    `json:"version"`
	Path    string `json:"path"`
}

// HeartbeatResponse is the response from heartbeat.
type HeartbeatResponse struct {
	Commands []Command `json:"commands"`
}

// Command is a command from the control plane.
type Command struct {
	Type         string          `json:"type"` // "deploy", "undeploy"
	DeploymentID string          `json:"deploymentId,omitempty"`
	MockID       string          `json:"mockId,omitempty"`
	MockVersion  int             `json:"mockVersion,omitempty"`
	URLPath      string          `json:"urlPath,omitempty"`
	Content      json.RawMessage `json:"content,omitempty"`
}

// NewClient creates a new runtime client.
func NewClient(config Config) *Client {
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = DefaultHeartbeatInterval
	}

	return &Client{
		config: config,
		httpClient: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
		deployments: make(map[string]*Deployment),
		mocksByPath: make(map[string]*Deployment),
	}
}

// Register registers this runtime with the control plane.
func (c *Client) Register(ctx context.Context) (*RegistrationResponse, error) {
	endpoint := c.config.ControlPlaneURL + "/api/v1/runtimes/register"

	reqBody := map[string]interface{}{
		"name":   c.config.Name,
		"url":    c.config.URL,
		"labels": c.config.Labels,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("registration failed: %s (status %d)", string(body), resp.StatusCode)
	}

	var regResp RegistrationResponse
	if err := json.Unmarshal(body, &regResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	c.runtimeID = regResp.ID
	// Store the new token if provided (for token rotation)
	if regResp.Token != "" {
		c.config.Token = regResp.Token
	}

	return &regResp, nil
}

// Heartbeat sends a heartbeat to the control plane and processes commands.
func (c *Client) Heartbeat(ctx context.Context) error {
	if c.runtimeID == "" {
		return fmt.Errorf("runtime not registered")
	}

	endpoint := fmt.Sprintf("%s/api/v1/runtimes/%s/heartbeat", c.config.ControlPlaneURL, c.runtimeID)

	// Build deployment info from current state
	c.mu.RLock()
	deploymentInfo := make([]DeploymentInfoShort, 0, len(c.deployments))
	for _, d := range c.deployments {
		deploymentInfo = append(deploymentInfo, DeploymentInfoShort{
			MockID:  d.MockID,
			Version: d.MockVersion,
			Path:    d.URLPath,
		})
	}
	c.mu.RUnlock()

	reqBody := HeartbeatRequest{
		Status:      "healthy",
		Version:     c.config.Version,
		Deployments: deploymentInfo,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal heartbeat: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create heartbeat request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("heartbeat failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat failed: %s (status %d)", string(body), resp.StatusCode)
	}

	var hbResp HeartbeatResponse
	if err := json.Unmarshal(body, &hbResp); err != nil {
		return fmt.Errorf("failed to unmarshal heartbeat response: %w", err)
	}

	// Process commands
	for _, cmd := range hbResp.Commands {
		if err := c.processCommand(cmd); err != nil {
			// Log error but continue processing other commands
			fmt.Printf("Error processing command %s: %v\n", cmd.Type, err)
		}
	}

	return nil
}

// HeartbeatLoop runs the heartbeat loop until context is cancelled.
func (c *Client) HeartbeatLoop(ctx context.Context) error {
	ticker := time.NewTicker(c.config.HeartbeatInterval)
	defer ticker.Stop()

	// Send initial heartbeat
	if err := c.Heartbeat(ctx); err != nil {
		fmt.Printf("Initial heartbeat failed: %v\n", err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := c.Heartbeat(ctx); err != nil {
				fmt.Printf("Heartbeat failed: %v\n", err)
				// Continue trying - transient failures are expected
			}
		}
	}
}

// processCommand processes a single command from the control plane.
func (c *Client) processCommand(cmd Command) error {
	switch cmd.Type {
	case "deploy":
		return c.handleDeploy(cmd)
	case "undeploy":
		return c.handleUndeploy(cmd)
	default:
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// handleDeploy handles a deploy command.
func (c *Client) handleDeploy(cmd Command) error {
	deployment := &Deployment{
		ID:          cmd.DeploymentID,
		MockID:      cmd.MockID,
		MockVersion: cmd.MockVersion,
		URLPath:     cmd.URLPath,
		Content:     cmd.Content,
		DeployedAt:  time.Now(),
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Remove any existing deployment at this path
	if existing, ok := c.mocksByPath[cmd.URLPath]; ok {
		delete(c.deployments, existing.ID)
	}

	c.deployments[cmd.DeploymentID] = deployment
	c.mocksByPath[cmd.URLPath] = deployment

	fmt.Printf("Deployed mock %s (version %d) at %s\n", cmd.MockID, cmd.MockVersion, cmd.URLPath)
	return nil
}

// handleUndeploy handles an undeploy command.
func (c *Client) handleUndeploy(cmd Command) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	deployment, ok := c.deployments[cmd.DeploymentID]
	if !ok {
		// Already undeployed, ignore
		return nil
	}

	delete(c.mocksByPath, deployment.URLPath)
	delete(c.deployments, cmd.DeploymentID)

	fmt.Printf("Undeployed mock %s from %s\n", deployment.MockID, deployment.URLPath)
	return nil
}

// GetDeployment returns a deployment by URL path.
func (c *Client) GetDeployment(path string) (*Deployment, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	deployment, ok := c.mocksByPath[path]
	return deployment, ok
}

// GetAllDeployments returns all current deployments.
func (c *Client) GetAllDeployments() []*Deployment {
	c.mu.RLock()
	defer c.mu.RUnlock()

	deployments := make([]*Deployment, 0, len(c.deployments))
	for _, d := range c.deployments {
		deployments = append(deployments, d)
	}
	return deployments
}

// GetRuntimeID returns the runtime ID assigned by the control plane.
func (c *Client) GetRuntimeID() string {
	return c.runtimeID
}

// SetRuntimeID sets the runtime ID (for reconnection scenarios).
func (c *Client) SetRuntimeID(id string) {
	c.runtimeID = id
}

// PullDeployments fetches all current deployments from the control plane.
func (c *Client) PullDeployments(ctx context.Context) error {
	if c.runtimeID == "" {
		return fmt.Errorf("runtime not registered")
	}

	endpoint := fmt.Sprintf("%s/api/v1/runtimes/%s/deployments", c.config.ControlPlaneURL, c.runtimeID)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch deployments: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch deployments: %s (status %d)", string(body), resp.StatusCode)
	}

	var pullResp struct {
		Deployments []struct {
			ID      string          `json:"id"`
			MockID  string          `json:"mockId"`
			Version int             `json:"version"`
			URLPath string          `json:"urlPath"`
			Content json.RawMessage `json:"content"`
		} `json:"deployments"`
	}

	if err := json.Unmarshal(body, &pullResp); err != nil {
		return fmt.Errorf("failed to unmarshal deployments: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Replace all deployments
	c.deployments = make(map[string]*Deployment)
	c.mocksByPath = make(map[string]*Deployment)

	for _, d := range pullResp.Deployments {
		deployment := &Deployment{
			ID:          d.ID,
			MockID:      d.MockID,
			MockVersion: d.Version,
			URLPath:     d.URLPath,
			Content:     d.Content,
			DeployedAt:  time.Now(),
		}
		c.deployments[d.ID] = deployment
		c.mocksByPath[d.URLPath] = deployment
	}

	fmt.Printf("Pulled %d deployments from control plane\n", len(pullResp.Deployments))
	return nil
}

// Pull fetches mocks from a mockd:// URI.
func (c *Client) Pull(ctx context.Context, uri string) (json.RawMessage, error) {
	parsedURI, err := ParseMockdURI(uri)
	if err != nil {
		return nil, err
	}

	endpoint := c.config.ControlPlaneURL + "/api/v1/pull"
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid control plane URL: %w", err)
	}

	q := reqURL.Query()
	q.Set("uri", uri)
	reqURL.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to pull: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("collection not found: %s/%s", parsedURI.Workspace, parsedURI.Collection)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("pull failed: %s (status %d)", string(body), resp.StatusCode)
	}

	var pullResp struct {
		Collection string          `json:"collection"`
		Version    string          `json:"version"`
		Content    json.RawMessage `json:"content"`
	}

	if err := json.Unmarshal(body, &pullResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pull response: %w", err)
	}

	return pullResp.Content, nil
}
