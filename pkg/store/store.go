// Package store provides a unified data persistence layer for mockd.
//
// The store package abstracts storage backends to support:
//   - Local file-based storage (desktop app, CLI)
//   - SQLite for embedded databases
//   - Remote database backends (PostgreSQL, etc.)
//   - Future CRDT-based distributed sync
//
// Directory structure follows XDG Base Directory Specification:
//   - Config: ~/.config/mockd/ (user preferences, settings)
//   - Data:   ~/.local/share/mockd/ (mocks, recordings, state)
//   - Cache:  ~/.cache/mockd/ (temporary data, logs)
//   - State:  ~/.local/state/mockd/ (runtime state, logs)
package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

// Common errors
var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalidID     = errors.New("invalid id")
	ErrReadOnly      = errors.New("store is read-only")
)

// Backend represents a storage backend type.
type Backend string

const (
	// BackendFile uses JSON/YAML files for storage
	BackendFile Backend = "file"
	// BackendSQLite uses embedded SQLite database
	BackendSQLite Backend = "sqlite"
	// BackendMemory uses in-memory storage (no persistence)
	BackendMemory Backend = "memory"
)

// Config holds store configuration.
type Config struct {
	// Backend specifies the storage backend to use
	Backend Backend `json:"backend" yaml:"backend"`

	// DataDir is the base directory for data storage
	// Defaults to XDG_DATA_HOME/mockd or ~/.local/share/mockd
	DataDir string `json:"dataDir,omitempty" yaml:"dataDir,omitempty"`

	// ConfigDir is the directory for configuration files
	// Defaults to XDG_CONFIG_HOME/mockd or ~/.config/mockd
	ConfigDir string `json:"configDir,omitempty" yaml:"configDir,omitempty"`

	// CacheDir is the directory for cache files
	// Defaults to XDG_CACHE_HOME/mockd or ~/.cache/mockd
	CacheDir string `json:"cacheDir,omitempty" yaml:"cacheDir,omitempty"`

	// StateDir is the directory for runtime state
	// Defaults to XDG_STATE_HOME/mockd or ~/.local/state/mockd
	StateDir string `json:"stateDir,omitempty" yaml:"stateDir,omitempty"`

	// ReadOnly prevents any write operations
	ReadOnly bool `json:"readOnly,omitempty" yaml:"readOnly,omitempty"`
}

// DefaultConfig returns the default store configuration.
func DefaultConfig() Config {
	return Config{
		Backend:   BackendFile,
		DataDir:   DefaultDataDir(),
		ConfigDir: DefaultConfigDir(),
		CacheDir:  DefaultCacheDir(),
		StateDir:  DefaultStateDir(),
	}
}

// DefaultDataDir returns the default data directory following XDG spec.
func DefaultDataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "mockd")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".mockd", "data")
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Application Support", "mockd")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("LOCALAPPDATA"); appData != "" {
			return filepath.Join(appData, "mockd")
		}
		return filepath.Join(home, "AppData", "Local", "mockd")
	}
	return filepath.Join(home, ".local", "share", "mockd")
}

// DefaultConfigDir returns the default config directory following XDG spec.
func DefaultConfigDir() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "mockd")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".mockd", "config")
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Preferences", "mockd")
	}
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "mockd")
		}
		return filepath.Join(home, "AppData", "Roaming", "mockd")
	}
	return filepath.Join(home, ".config", "mockd")
}

// DefaultCacheDir returns the default cache directory following XDG spec.
func DefaultCacheDir() string {
	if dir := os.Getenv("XDG_CACHE_HOME"); dir != "" {
		return filepath.Join(dir, "mockd")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".mockd", "cache")
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Caches", "mockd")
	}
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "mockd", "cache")
		}
		return filepath.Join(home, "AppData", "Local", "mockd", "cache")
	}
	return filepath.Join(home, ".cache", "mockd")
}

// DefaultStateDir returns the default state directory following XDG spec.
func DefaultStateDir() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "mockd")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".mockd", "state")
	}
	if runtime.GOOS == "darwin" {
		// macOS doesn't have a state dir convention, use data dir
		return filepath.Join(home, "Library", "Application Support", "mockd", "state")
	}
	if runtime.GOOS == "windows" {
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "mockd", "state")
		}
		return filepath.Join(home, "AppData", "Local", "mockd", "state")
	}
	return filepath.Join(home, ".local", "state", "mockd")
}

// DefaultRecordingsDir returns the default recordings directory.
// Recordings are user data, so they go in the data directory.
func DefaultRecordingsDir() string {
	return filepath.Join(DefaultDataDir(), "recordings")
}

// Store is the main interface for data persistence.
type Store interface {
	// Lifecycle
	Open(ctx context.Context) error
	Close() error

	// Workspace management (multi-source support)
	Workspaces() WorkspaceStore

	// Unified mock store - all mock types in one store
	Mocks() MockStore

	// Non-mock stores
	Folders() FolderStore
	Recordings() RecordingStore
	RequestLog() RequestLogStore
	Preferences() PreferencesStore

	// Transactions (for backends that support it)
	Begin(ctx context.Context) (Transaction, error)

	// Sync operations (future CRDT support)
	Sync(ctx context.Context) error
	LastSyncTime() int64
}

// Transaction represents a database transaction.
type Transaction interface {
	Commit() error
	Rollback() error
}

// ChangeEvent represents a change to the store for sync/notifications.
type ChangeEvent struct {
	Collection string      `json:"collection"`
	Operation  string      `json:"operation"` // "create", "update", "delete"
	ID         string      `json:"id"`
	Data       interface{} `json:"data,omitempty"`
	Timestamp  int64       `json:"timestamp"`
}

// ChangeListener is called when data changes.
type ChangeListener func(event ChangeEvent)
