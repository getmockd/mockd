package store

import (
	"context"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// ============================================================================
// Workspace Types - Foundation for multi-source support
// ============================================================================

// WorkspaceType represents the type of workspace backend.
type WorkspaceType string

const (
	// WorkspaceTypeLocal is a local file-based workspace (default)
	WorkspaceTypeLocal WorkspaceType = "local"
	// WorkspaceTypeGit is a git repository workspace
	WorkspaceTypeGit WorkspaceType = "git"
	// WorkspaceTypeCloud is a cloud-synced workspace
	WorkspaceTypeCloud WorkspaceType = "cloud"
	// WorkspaceTypeConfig is a read-only config file workspace
	WorkspaceTypeConfig WorkspaceType = "config"
)

// SyncStatus represents the sync state of a workspace.
type SyncStatus string

const (
	SyncStatusSynced  SyncStatus = "synced"
	SyncStatusPending SyncStatus = "pending"
	SyncStatusError   SyncStatus = "error"
	SyncStatusLocal   SyncStatus = "local" // local-only, no sync
)

// Workspace represents a collection of mocks from a specific source.
type Workspace struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        WorkspaceType `json:"type"`
	Description string        `json:"description,omitempty"`

	// Routing
	// BasePath is the URL prefix for this workspace's mocks on the engine.
	// Empty string means this workspace is the "root" workspace on any engine
	// that designates it as root (via Engine.RootWorkspaceID). Non-root
	// workspaces must have a non-empty BasePath (e.g., "/payment-api").
	// The default workspace starts with BasePath="" (root). New workspaces
	// auto-generate a BasePath from a slugified version of their name.
	BasePath string `json:"basePath"`

	// Backend configuration
	Path     string `json:"path,omitempty"`     // Local path or git subdir
	URL      string `json:"url,omitempty"`      // Git URL or cloud API URL
	Branch   string `json:"branch,omitempty"`   // Git branch
	ReadOnly bool   `json:"readOnly,omitempty"` // Prevent local edits

	// Sync state
	SyncStatus   SyncStatus `json:"syncStatus,omitempty"`
	LastSyncedAt int64      `json:"lastSyncedAt,omitempty"`
	AutoSync     bool       `json:"autoSync,omitempty"`

	// Metadata
	CreatedAt int64 `json:"createdAt"`
	UpdatedAt int64 `json:"updatedAt"`
}

// DefaultWorkspaceID is the ID of the default local workspace.
const DefaultWorkspaceID = "local"

// WorkspaceStore handles workspace persistence.
type WorkspaceStore interface {
	List(ctx context.Context) ([]*Workspace, error)
	Get(ctx context.Context, id string) (*Workspace, error)
	Create(ctx context.Context, workspace *Workspace) error
	Update(ctx context.Context, workspace *Workspace) error
	Delete(ctx context.Context, id string) error
}

// ============================================================================
// Entity Metadata - Common fields for all stored entities
// ============================================================================

// EntityMeta is an alias to config.EntityMeta for backward compatibility.
// Use config.EntityMeta directly in new code.
type EntityMeta = config.EntityMeta

// ============================================================================
// Mock Store (Unified)
// ============================================================================

// MockFilter provides filtering criteria for mock list operations.
type MockFilter struct {
	WorkspaceID string    // Filter by workspace ("" = no filter)
	Type        mock.Type // Filter by mock type ("" = all types)
	ParentID    *string   // Filter by parent folder (nil = no filter, "" = root level)
	Enabled     *bool     // Filter by enabled state (nil = no filter)
	Search      string    // Search in name/path
}

// MockStore handles persistence for all mock types in a unified manner.
type MockStore interface {
	// List returns all mocks matching the filter.
	List(ctx context.Context, filter *MockFilter) ([]*mock.Mock, error)

	// Get returns a single mock by ID.
	Get(ctx context.Context, id string) (*mock.Mock, error)

	// Create creates a new mock.
	Create(ctx context.Context, m *mock.Mock) error

	// Update updates an existing mock.
	Update(ctx context.Context, m *mock.Mock) error

	// Delete deletes a mock by ID.
	Delete(ctx context.Context, id string) error

	// DeleteByType deletes all mocks of a specific type.
	DeleteByType(ctx context.Context, mockType mock.Type) error

	// DeleteAll deletes all mocks.
	DeleteAll(ctx context.Context) error

	// Count returns the total number of mocks, optionally filtered by type.
	Count(ctx context.Context, mockType mock.Type) (int, error)

	// BulkCreate creates multiple mocks in a single operation.
	BulkCreate(ctx context.Context, mocks []*mock.Mock) error

	// BulkUpdate updates multiple mocks in a single operation.
	BulkUpdate(ctx context.Context, mocks []*mock.Mock) error
}

// StatefulResourceStore handles persistence for stateful resource configurations.
type StatefulResourceStore interface {
	// List returns all persisted stateful resource configs.
	List(ctx context.Context) ([]*config.StatefulResourceConfig, error)
	// Create persists a new stateful resource config.
	Create(ctx context.Context, res *config.StatefulResourceConfig) error
	// Delete removes a stateful resource config by name.
	Delete(ctx context.Context, name string) error
	// DeleteAll removes all stateful resource configs.
	DeleteAll(ctx context.Context) error
}

// FolderFilter provides filtering criteria for folder list operations.
type FolderFilter struct {
	WorkspaceID string  // Filter by workspace ("" = no filter)
	ParentID    *string // Filter by parent folder (nil = no filter, "" = root level)
}

// FolderStore handles folder persistence.
type FolderStore interface {
	List(ctx context.Context, filter *FolderFilter) ([]*config.Folder, error)
	Get(ctx context.Context, id string) (*config.Folder, error)
	Create(ctx context.Context, folder *config.Folder) error
	Update(ctx context.Context, folder *config.Folder) error
	Delete(ctx context.Context, id string) error
	DeleteAll(ctx context.Context) error
}

// OrganizationMeta is an alias to config.OrganizationMeta for backward compatibility.
type OrganizationMeta = config.OrganizationMeta

// Recording represents a stored recording session.
type Recording struct {
	EntityMeta
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
	SessionID    string `json:"sessionId,omitempty"`
	Protocol     string `json:"protocol"` // http, grpc, websocket, etc.
	StartedAt    int64  `json:"startedAt"`
	EndedAt      int64  `json:"endedAt,omitempty"`
	RequestCount int    `json:"requestCount"`
	DataFile     string `json:"dataFile,omitempty"` // path to recording data
}

// RecordingStore handles recording persistence.
type RecordingStore interface {
	List(ctx context.Context) ([]*Recording, error)
	Get(ctx context.Context, id string) (*Recording, error)
	Create(ctx context.Context, recording *Recording) error
	Update(ctx context.Context, recording *Recording) error
	Delete(ctx context.Context, id string) error
	DeleteAll(ctx context.Context) error
}

// RequestLogEntry represents a logged request.
type RequestLogEntry struct {
	ID            string            `json:"id"`
	Protocol      string            `json:"protocol"`
	Method        string            `json:"method"`
	Path          string            `json:"path"`
	StatusCode    int               `json:"statusCode"`
	Duration      int64             `json:"duration"` // nanoseconds
	Timestamp     int64             `json:"timestamp"`
	MatchedMockID string            `json:"matchedMockId,omitempty"`
	RequestBody   string            `json:"requestBody,omitempty"`
	ResponseBody  string            `json:"responseBody,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
	Error         string            `json:"error,omitempty"`
}

// RequestLogStore handles request log persistence.
type RequestLogStore interface {
	List(ctx context.Context, limit, offset int) ([]*RequestLogEntry, error)
	Get(ctx context.Context, id string) (*RequestLogEntry, error)
	Append(ctx context.Context, entry *RequestLogEntry) error
	Clear(ctx context.Context) error
	Count(ctx context.Context) (int, error)
}

// Preferences represents user preferences.
type Preferences struct {
	Theme            string `json:"theme,omitempty"` // light, dark, system
	SidebarCollapsed bool   `json:"sidebarCollapsed,omitempty"`
	AutoScroll       bool   `json:"autoScroll,omitempty"`
	PollingInterval  int    `json:"pollingInterval,omitempty"` // milliseconds
	MinimizeToTray   bool   `json:"minimizeToTray,omitempty"`
	StartMinimized   bool   `json:"startMinimized,omitempty"`
	DefaultMockPort  int    `json:"defaultMockPort,omitempty"`
	DefaultAdminPort int    `json:"defaultAdminPort,omitempty"`
}

// PreferencesStore handles user preferences persistence.
type PreferencesStore interface {
	Get(ctx context.Context) (*Preferences, error)
	Set(ctx context.Context, prefs *Preferences) error
}
