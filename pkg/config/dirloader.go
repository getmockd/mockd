package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DirectoryLoader loads mock configurations from a directory.
type DirectoryLoader struct {
	// Path is the directory to load from
	Path string

	// Recursive if true, scans subdirectories
	Recursive bool

	// ValidateOnly if true, only validates without loading
	ValidateOnly bool

	// files tracks loaded files for watching
	files map[string]time.Time
	mu    sync.RWMutex
}

// LoadResult contains the result of loading a directory.
type LoadResult struct {
	// Collection is the merged mock collection
	Collection *MockCollection

	// FileCount is the number of files processed
	FileCount int

	// Errors are any non-fatal errors encountered
	Errors []LoadError
}

// LoadError represents an error loading a specific file.
type LoadError struct {
	Path    string
	Message string
	Err     error
}

func (e *LoadError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Path, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// NewDirectoryLoader creates a new directory loader.
func NewDirectoryLoader(path string) *DirectoryLoader {
	return &DirectoryLoader{
		Path:      path,
		Recursive: true,
		files:     make(map[string]time.Time),
	}
}

// Load loads all mock configurations from the directory.
func (d *DirectoryLoader) Load() (*LoadResult, error) {
	result := &LoadResult{
		Collection: &MockCollection{
			Version: "1.0",
			Name:    "Loaded from " + d.Path,
			Mocks:   make([]*MockConfiguration, 0),
		},
	}

	// Check if directory exists
	info, err := os.Stat(d.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("directory not found: %s", d.Path)
		}
		return nil, fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", d.Path)
	}

	// Find all config files
	files, err := d.findConfigFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	if len(files) == 0 {
		return result, nil
	}

	// Load each file
	idOffset := 0
	for _, file := range files {
		collection, err := LoadFromFile(file)
		if err != nil {
			result.Errors = append(result.Errors, LoadError{
				Path:    file,
				Message: "failed to load",
				Err:     err,
			})
			continue
		}

		// Track file for watching
		info, _ := os.Stat(file)
		d.mu.Lock()
		if info != nil {
			d.files[file] = info.ModTime()
		}
		d.mu.Unlock()

		// Merge mocks with unique IDs
		for _, mock := range collection.Mocks {
			// Make IDs unique across files using relative path to avoid collisions
			relPath, err := filepath.Rel(d.Path, file)
			if err != nil {
				relPath = filepath.Base(file)
			}
			// Sanitize: replace path separators with dashes, remove extension
			pathPrefix := strings.ReplaceAll(relPath, string(filepath.Separator), "-")
			pathPrefix = strings.TrimSuffix(pathPrefix, filepath.Ext(pathPrefix))

			if mock.ID != "" {
				mock.ID = fmt.Sprintf("%s-%s", pathPrefix, mock.ID)
			} else {
				idOffset++
				mock.ID = fmt.Sprintf("%s-mock-%d", pathPrefix, idOffset)
			}
			result.Collection.Mocks = append(result.Collection.Mocks, mock)
		}

		// Merge stateful resources
		result.Collection.StatefulResources = append(
			result.Collection.StatefulResources,
			collection.StatefulResources...,
		)

		// Merge WebSocket endpoints
		result.Collection.WebSocketEndpoints = append(
			result.Collection.WebSocketEndpoints,
			collection.WebSocketEndpoints...,
		)

		// Use first server config found
		if result.Collection.ServerConfig == nil && collection.ServerConfig != nil {
			result.Collection.ServerConfig = collection.ServerConfig
		}

		result.FileCount++
	}

	return result, nil
}

// findConfigFiles finds all .yaml, .yml, and .json files in the directory.
func (d *DirectoryLoader) findConfigFiles() ([]string, error) {
	var files []string

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			//nolint:nilerr // intentionally skipping files we cannot access during directory walk
			return nil
		}

		// Skip directories (but recurse into them)
		if info.IsDir() {
			// If not recursive and not the root, skip subdirectories
			if !d.Recursive && path != d.Path {
				return filepath.SkipDir
			}
			return nil
		}

		// Check extension
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".yaml" || ext == ".yml" || ext == ".json" {
			files = append(files, path)
		}

		return nil
	}

	if err := filepath.Walk(d.Path, walkFn); err != nil {
		return nil, err
	}

	return files, nil
}

// Validate validates all files in the directory without loading.
func (d *DirectoryLoader) Validate() ([]LoadError, error) {
	files, err := d.findConfigFiles()
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	var errors []LoadError
	for _, file := range files {
		// LoadFromFile already calls Validate() internally via ParseJSON/ParseYAML,
		// so there is no need to validate again here.
		_, err := LoadFromFile(file)
		if err != nil {
			errors = append(errors, LoadError{
				Path:    file,
				Message: "validation failed",
				Err:     err,
			})
		}
	}

	return errors, nil
}

// HasChanges checks if any tracked files have been modified.
func (d *DirectoryLoader) HasChanges() ([]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var changed []string
	for path, modTime := range d.files {
		info, err := os.Stat(path)
		if err != nil {
			// File deleted or inaccessible - count as changed
			changed = append(changed, path)
			continue
		}
		if info.ModTime().After(modTime) {
			changed = append(changed, path)
		}
	}

	return changed, nil
}

// ReloadFile reloads a single file and returns the updated mocks.
func (d *DirectoryLoader) ReloadFile(path string) (*MockCollection, error) {
	collection, err := LoadFromFile(path)
	if err != nil {
		return nil, err
	}

	// Update tracked modification time
	info, _ := os.Stat(path)
	d.mu.Lock()
	if info != nil {
		d.files[path] = info.ModTime()
	}
	d.mu.Unlock()

	return collection, nil
}

// WatchInterval is the default interval for file watching.
const WatchInterval = 2 * time.Second

// Watcher provides file watching functionality for the directory loader.
type Watcher struct {
	loader   *DirectoryLoader
	interval time.Duration
	stopCh   chan struct{}
	doneCh   chan struct{} // signals goroutine exit
	eventCh  chan WatchEvent
	mu       sync.Mutex
	running  bool
}

// WatchEvent represents a file change event.
type WatchEvent struct {
	Path  string
	Type  string // "modified", "added", "deleted"
	Error error
}

// NewWatcher creates a new file watcher for the directory.
func NewWatcher(loader *DirectoryLoader) *Watcher {
	return &Watcher{
		loader:   loader,
		interval: WatchInterval,
		stopCh:   make(chan struct{}),
		eventCh:  make(chan WatchEvent, 10),
	}
}

// Start begins watching for file changes.
func (w *Watcher) Start() <-chan WatchEvent {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return w.eventCh
	}

	w.stopCh = make(chan struct{})
	w.doneCh = make(chan struct{})
	w.running = true

	// Pass channels to avoid race on struct fields
	stopCh := w.stopCh
	doneCh := w.doneCh
	go w.watchLoop(stopCh, doneCh)

	return w.eventCh
}

// Stop stops the watcher.
func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}

	close(w.stopCh)
	w.running = false
	doneCh := w.doneCh
	w.mu.Unlock()

	// Wait outside lock for goroutine to exit
	<-doneCh
}

// watchLoop is the main watch loop.
func (w *Watcher) watchLoop(stopCh <-chan struct{}, doneCh chan<- struct{}) {
	defer close(doneCh)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			changed, err := w.loader.HasChanges()
			if err != nil {
				w.eventCh <- WatchEvent{Error: err}
				continue
			}

			for _, path := range changed {
				w.eventCh <- WatchEvent{
					Path: path,
					Type: "modified",
				}
			}
		}
	}
}
