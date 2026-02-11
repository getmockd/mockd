// Package file provides a file-based implementation of the store interfaces.
// Data is stored as JSON files in XDG-compliant directories.
package file

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
)

// Current data format version for migration support
const dataVersion = 1

// FileStore implements store.Store using JSON files.
type FileStore struct {
	cfg          store.Config
	mu           sync.RWMutex
	data         *storeData
	listeners    []store.ChangeListener
	listenersMu  sync.RWMutex
	dirty        atomic.Bool
	saving       atomic.Bool
	autoSave     bool
	saveDebounce time.Duration
	saveCh       chan struct{}
	closeCh      chan struct{}
	closeOnce    sync.Once
	closedCh     chan struct{} // signals when saveLoop has exited
	log          *slog.Logger
}

// storeData holds all persisted data.
type storeData struct {
	Version    int                `json:"version"`
	Workspaces []*store.Workspace `json:"workspaces,omitempty"`

	// Unified mocks - all mock types in one slice
	Mocks []*mock.Mock `json:"mocks,omitempty"`

	// Stateful resource configurations (persisted across restarts)
	StatefulResources []*config.StatefulResourceConfig `json:"statefulResources,omitempty"`

	Folders     []*config.Folder         `json:"folders,omitempty"`
	Recordings  []*store.Recording       `json:"recordings,omitempty"`
	RequestLog  []*store.RequestLogEntry `json:"requestLog,omitempty"`
	Preferences *store.Preferences       `json:"preferences,omitempty"`
	LastSync    int64                    `json:"lastSync,omitempty"`
}

// New creates a new FileStore with the given configuration.
func New(cfg store.Config) *FileStore {
	if cfg.DataDir == "" {
		cfg.DataDir = store.DefaultDataDir()
	}
	fs := &FileStore{
		cfg:          cfg,
		data:         &storeData{Version: dataVersion},
		autoSave:     true,
		saveDebounce: 500 * time.Millisecond,
		saveCh:       make(chan struct{}, 1),
		closeCh:      make(chan struct{}),
		closedCh:     make(chan struct{}),
		log:          slog.Default(),
	}
	// Start debounced save goroutine
	go fs.saveLoop()
	return fs
}

// NewWithDefaults creates a new FileStore with default configuration.
func NewWithDefaults() *FileStore {
	return New(store.DefaultConfig())
}

// saveLoop handles debounced saving to prevent excessive disk writes.
func (s *FileStore) saveLoop() {
	defer close(s.closedCh) // Signal that saveLoop has exited
	var timer *time.Timer
	for {
		select {
		case <-s.saveCh:
			// Reset or create timer for debounce
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(s.saveDebounce, func() {
				if s.dirty.Load() && !s.saving.Load() {
					if err := s.doSave(); err != nil {
						s.log.Error("failed to save store data", "error", err)
					}
				}
			})
		case <-s.closeCh:
			if timer != nil {
				timer.Stop()
			}
			// Final save on close
			if s.dirty.Load() {
				if err := s.doSave(); err != nil {
					s.log.Error("failed to save store data on close", "error", err)
				}
			}
			return
		}
	}
}

// Open initializes the store and loads data from disk.
func (s *FileStore) Open(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Ensure directories exist with secure permissions (0700)
	dirs := []string{s.cfg.DataDir, s.cfg.ConfigDir, s.cfg.CacheDir, s.cfg.StateDir}
	for _, dir := range dirs {
		if dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				return err
			}
		}
	}

	// Load data from disk
	dataFile := filepath.Join(s.cfg.DataDir, "data.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No data file yet, start fresh
			s.data = &storeData{Version: dataVersion}
			return nil
		}
		return err
	}

	var stored storeData
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}

	s.data = &stored
	s.dirty.Store(false)
	return nil
}

// Close saves any pending changes and closes the store. Safe to call multiple times.
func (s *FileStore) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeCh)
	})
	// Wait for saveLoop to complete its final save and exit
	<-s.closedCh
	return nil
}

// doSave performs the actual save operation with atomic write.
func (s *FileStore) doSave() error {
	if !s.saving.CompareAndSwap(false, true) {
		return nil // Already saving
	}
	defer s.saving.Store(false)

	s.mu.RLock()
	if s.cfg.ReadOnly {
		s.mu.RUnlock()
		return store.ErrReadOnly
	}

	// Ensure version is set
	s.data.Version = dataVersion

	data, err := json.MarshalIndent(s.data, "", "  ")
	s.mu.RUnlock()

	if err != nil {
		return err
	}

	// Atomic write: write to temp file, then rename
	dataFile := filepath.Join(s.cfg.DataDir, "data.json")
	tmpFile := dataFile + ".tmp"

	if err := os.WriteFile(tmpFile, data, 0600); err != nil {
		return err
	}

	if err := os.Rename(tmpFile, dataFile); err != nil {
		_ = os.Remove(tmpFile) // Clean up temp file on failure
		return err
	}

	s.dirty.Store(false)
	return nil
}

// markDirty marks data as needing to be saved (thread-safe).
func (s *FileStore) markDirty() {
	s.dirty.Store(true)
	if s.autoSave {
		// Non-blocking send to trigger save
		select {
		case s.saveCh <- struct{}{}:
		default:
			// Channel full, save already pending
		}
	}
}

// ForceSave immediately saves data to disk.
func (s *FileStore) ForceSave() error {
	s.dirty.Store(true)
	return s.doSave()
}

// notify sends a change event to all listeners.
func (s *FileStore) notify(collection, operation, id string, data any) {
	s.listenersMu.RLock()
	listeners := make([]store.ChangeListener, len(s.listeners))
	copy(listeners, s.listeners)
	s.listenersMu.RUnlock()

	event := store.ChangeEvent{
		Collection: collection,
		Operation:  operation,
		ID:         id,
		Data:       data,
		Timestamp:  time.Now().UnixMilli(),
	}
	for _, l := range listeners {
		go func(listener store.ChangeListener) {
			defer func() { _ = recover() }() // Prevent listener panics from crashing store
			listener(event)
		}(l)
	}
}

// AddChangeListener adds a listener for data changes.
func (s *FileStore) AddChangeListener(listener store.ChangeListener) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	s.listeners = append(s.listeners, listener)
}

// Workspaces returns the workspace store.
func (s *FileStore) Workspaces() store.WorkspaceStore {
	return &workspaceStore{fs: s}
}

// Mocks returns the mock store.
func (s *FileStore) Mocks() store.MockStore {
	return &mockStore{fs: s}
}

// StatefulResources returns the stateful resource store.
func (s *FileStore) StatefulResources() store.StatefulResourceStore {
	return &statefulResourceStore{fs: s}
}

// Folders returns the folder store.
func (s *FileStore) Folders() store.FolderStore {
	return &folderStore{fs: s}
}

// Recordings returns the recordings store.
func (s *FileStore) Recordings() store.RecordingStore {
	return &recordingStore{fs: s}
}

// RequestLog returns the request log store.
func (s *FileStore) RequestLog() store.RequestLogStore {
	return &requestLogStore{fs: s}
}

// Preferences returns the preferences store.
func (s *FileStore) Preferences() store.PreferencesStore {
	return &preferencesStore{fs: s}
}

// Begin starts a transaction (snapshot-based for file store).
func (s *FileStore) Begin(ctx context.Context) (store.Transaction, error) {
	return &fileTransaction{fs: s}, nil
}

// Sync synchronizes with remote (future CRDT support).
func (s *FileStore) Sync(ctx context.Context) error {
	// TODO: Implement CRDT sync
	return nil
}

// LastSyncTime returns the last sync timestamp.
func (s *FileStore) LastSyncTime() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.LastSync
}

// DataDir returns the data directory path.
func (s *FileStore) DataDir() string {
	return s.cfg.DataDir
}

// fileTransaction provides basic transaction support.
type fileTransaction struct {
	fs *FileStore
}

func (t *fileTransaction) Commit() error {
	return t.fs.ForceSave()
}

func (t *fileTransaction) Rollback() error {
	// Reload from disk to discard in-memory changes
	return t.fs.reloadFromDisk()
}

// reloadFromDisk reloads the data from the JSON file, discarding in-memory changes.
func (s *FileStore) reloadFromDisk() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dataFile := filepath.Join(s.cfg.DataDir, "data.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			s.data = &storeData{Version: dataVersion}
			s.dirty.Store(false)
			return nil
		}
		return err
	}

	var stored storeData
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}

	s.data = &stored
	s.dirty.Store(false)
	return nil
}
