// Package workspace defines shared types and interfaces for workspace server
// management. This package is imported by both pkg/admin (the consumer) and
// pkg/engine (the implementer), keeping the two layers decoupled.
package workspace

import (
	"context"
	"log/slog"
	"time"

	"github.com/getmockd/mockd/internal/storage"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/store"
)

// ServerStatus represents the status of a workspace server.
type ServerStatus string

const (
	ServerStatusStopped  ServerStatus = "stopped"
	ServerStatusRunning  ServerStatus = "running"
	ServerStatusStarting ServerStatus = "starting"
	ServerStatusError    ServerStatus = "error"
)

// StatusInfo contains status information for a workspace server.
type StatusInfo struct {
	WorkspaceID   string       `json:"workspaceId"`
	WorkspaceName string       `json:"workspaceName"`
	HTTPPort      int          `json:"httpPort"`
	GRPCPort      int          `json:"grpcPort,omitempty"`
	MQTTPort      int          `json:"mqttPort,omitempty"`
	Status        ServerStatus `json:"status"`
	StatusMessage string       `json:"statusMessage,omitempty"`
	MockCount     int          `json:"mockCount"`
	RequestCount  int          `json:"requestCount"`
	Uptime        int          `json:"uptime"` // seconds
}

// MockFetcher is a function that fetches mocks for a specific workspace.
type MockFetcher func(ctx context.Context, workspaceID string) ([]*config.MockConfiguration, error)

// ManagerConfig holds configuration for the workspace manager.
type ManagerConfig struct {
	DefaultReadTimeout  time.Duration
	DefaultWriteTimeout time.Duration
	MaxLogEntries       int
}

// DefaultManagerConfig returns sensible defaults.
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		DefaultReadTimeout:  30 * time.Second,
		DefaultWriteTimeout: 30 * time.Second,
		MaxLogEntries:       1000,
	}
}

// Server is the interface that workspace server implementations must satisfy.
// The admin layer uses this to query server status without importing pkg/engine.
type Server interface {
	Status() ServerStatus
	StatusInfo() *StatusInfo
}

// Manager is the interface for managing workspace servers. The admin layer
// depends on this interface rather than the concrete engine.WorkspaceManager,
// keeping the admin -> engine dependency boundary clean.
type Manager interface {
	SetLogger(log *slog.Logger)
	SetMockFetcher(fetcher MockFetcher)
	SetCentralStore(store storage.MockStore)

	StartWorkspace(ctx context.Context, ws *store.EngineWorkspace) error
	StopWorkspace(workspaceID string) error
	RemoveWorkspace(workspaceID string) error
	ReloadWorkspace(ctx context.Context, workspaceID string) error
	StopAll() error

	GetWorkspace(workspaceID string) Server
	ListWorkspaces() []Server
	GetWorkspaceStatus(workspaceID string) *StatusInfo
}
