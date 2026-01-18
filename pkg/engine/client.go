// Package engine provides the core mock server engine.
package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/store"
)

// EngineClient handles communication with a remote Admin API.
// It registers the engine, receives workspace assignments, and syncs mocks.
type EngineClient struct {
	adminURL   string
	engineName string
	engineID   string
	token      string
	localPort  int
	log        *slog.Logger

	httpClient *http.Client
	manager    *WorkspaceManager

	// Polling
	pollInterval time.Duration
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.RWMutex
}

// EngineClientConfig holds configuration for the engine client.
type EngineClientConfig struct {
	AdminURL     string
	EngineName   string
	LocalPort    int
	PollInterval time.Duration
}

// NewEngineClient creates a new engine client.
func NewEngineClient(cfg *EngineClientConfig, manager *WorkspaceManager) *EngineClient {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 10 * time.Second
	}
	if cfg.EngineName == "" {
		hostname, _ := os.Hostname()
		cfg.EngineName = fmt.Sprintf("engine-%s", hostname)
	}

	return &EngineClient{
		adminURL:     cfg.AdminURL,
		engineName:   cfg.EngineName,
		localPort:    cfg.LocalPort,
		pollInterval: cfg.PollInterval,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		manager:      manager,
		stopCh:       make(chan struct{}),
		log:          logging.Nop(),
	}
}

// SetLogger sets the operational logger for the client.
func (c *EngineClient) SetLogger(log *slog.Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if log != nil {
		c.log = log
	} else {
		c.log = logging.Nop()
	}
}

// RegisterResponse is the response from engine registration.
type RegisterResponse struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

// Register registers this engine with the remote admin.
func (c *EngineClient) Register(ctx context.Context) error {
	body := map[string]interface{}{
		"name": c.engineName,
		"host": "localhost", // TODO: detect actual host
		"port": c.localPort,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal registration request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.adminURL+"/engines/register", bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create registration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to register with admin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed (status %d): %s", resp.StatusCode, string(body))
	}

	var regResp RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return fmt.Errorf("failed to decode registration response: %w", err)
	}

	c.mu.Lock()
	c.engineID = regResp.ID
	c.token = regResp.Token
	c.mu.Unlock()

	c.log.Info("registered as engine", "name", c.engineName, "id", c.engineID)
	return nil
}

// Start starts the polling loop to sync with the admin.
func (c *EngineClient) Start(ctx context.Context) error {
	// Initial registration
	if err := c.Register(ctx); err != nil {
		return err
	}

	// Initial sync
	if err := c.syncWorkspaces(ctx); err != nil {
		c.log.Warn("initial workspace sync failed", "error", err)
	}

	// Start polling loop
	c.wg.Add(1)
	go c.pollLoop(ctx)

	return nil
}

// Stop stops the engine client.
func (c *EngineClient) Stop() {
	close(c.stopCh)
	c.wg.Wait()
}

// pollLoop periodically syncs with the admin.
func (c *EngineClient) pollLoop(ctx context.Context) {
	defer c.wg.Done()

	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Send heartbeat
			if err := c.sendHeartbeat(ctx); err != nil {
				c.log.Warn("heartbeat failed", "error", err)
			}

			// Sync workspaces
			if err := c.syncWorkspaces(ctx); err != nil {
				c.log.Warn("workspace sync failed", "error", err)
			}
		}
	}
}

// sendHeartbeat sends a heartbeat to the admin.
func (c *EngineClient) sendHeartbeat(ctx context.Context) error {
	c.mu.RLock()
	engineID := c.engineID
	token := c.token
	c.mu.RUnlock()

	if engineID == "" {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.adminURL+"/engines/"+engineID+"/heartbeat", nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("heartbeat failed with status %d", resp.StatusCode)
	}

	return nil
}

// WorkspacesResponse is the response from fetching engine workspaces.
type WorkspacesResponse struct {
	Workspaces []store.EngineWorkspace `json:"workspaces"`
}

// syncWorkspaces fetches workspace assignments and starts/stops servers as needed.
func (c *EngineClient) syncWorkspaces(ctx context.Context) error {
	c.mu.RLock()
	engineID := c.engineID
	token := c.token
	c.mu.RUnlock()

	if engineID == "" {
		return nil
	}

	// Fetch engine details to get workspaces
	req, err := http.NewRequestWithContext(ctx, "GET", c.adminURL+"/engines/"+engineID, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to fetch engine (status %d)", resp.StatusCode)
	}

	var engine store.Engine
	if err := json.NewDecoder(resp.Body).Decode(&engine); err != nil {
		return fmt.Errorf("failed to decode engine response: %w", err)
	}

	// Set up mock fetcher to pull from admin (do this once)
	c.manager.SetMockFetcher(func(ctx context.Context, workspaceID string) ([]*config.MockConfiguration, error) {
		return c.fetchMocks(ctx, workspaceID)
	})

	// Sync workspaces
	currentWorkspaces := make(map[string]bool)
	for _, ws := range engine.Workspaces {
		currentWorkspaces[ws.WorkspaceID] = true

		// Check if already running
		existing := c.manager.GetWorkspace(ws.WorkspaceID)
		if existing != nil && existing.Status() == WorkspaceServerStatusRunning {
			// Already running - reload mocks to pick up changes
			if err := c.manager.ReloadWorkspace(ctx, ws.WorkspaceID); err != nil {
				c.log.Warn("failed to reload workspace", "workspace", ws.WorkspaceName, "error", err)
			}
			continue
		}

		// Start the workspace
		c.log.Info("starting workspace", "workspace", ws.WorkspaceName, "port", ws.HTTPPort)

		if err := c.manager.StartWorkspace(ctx, &ws); err != nil {
			c.log.Error("failed to start workspace", "workspace", ws.WorkspaceName, "error", err)
		}
	}

	// Stop workspaces that are no longer assigned
	for _, server := range c.manager.ListWorkspaces() {
		if !currentWorkspaces[server.WorkspaceID] {
			c.log.Info("stopping workspace (no longer assigned)", "workspace", server.WorkspaceName)
			c.manager.StopWorkspace(server.WorkspaceID)
		}
	}

	return nil
}

// MocksResponse is the response from the mocks endpoint.
type MocksResponse struct {
	Mocks []*config.MockConfiguration `json:"mocks"`
	Count int                         `json:"count"`
}

// fetchMocks fetches mocks for a workspace from the admin.
func (c *EngineClient) fetchMocks(ctx context.Context, workspaceID string) ([]*config.MockConfiguration, error) {
	c.mu.RLock()
	token := c.token
	c.mu.RUnlock()

	req, err := http.NewRequestWithContext(ctx, "GET", c.adminURL+"/mocks?workspaceId="+workspaceID, nil)
	if err != nil {
		return nil, err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch mocks (status %d)", resp.StatusCode)
	}

	var mocksResp MocksResponse
	if err := json.NewDecoder(resp.Body).Decode(&mocksResp); err != nil {
		return nil, fmt.Errorf("failed to decode mocks: %w", err)
	}

	return mocksResp.Mocks, nil
}

// EngineID returns the registered engine ID.
func (c *EngineClient) EngineID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.engineID
}
