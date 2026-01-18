// Package runtime provides the control plane client for runtime mode.
package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DeploymentCache manages local storage of deployed mocks for offline resilience.
type DeploymentCache struct {
	cacheDir string
	mu       sync.RWMutex
}

// CachedDeployment represents a cached deployment on disk.
type CachedDeployment struct {
	ID          string          `json:"id"`
	MockID      string          `json:"mockId"`
	MockVersion int             `json:"mockVersion"`
	URLPath     string          `json:"urlPath"`
	Content     json.RawMessage `json:"content"`
	CachedAt    time.Time       `json:"cachedAt"`
}

// NewDeploymentCache creates a new deployment cache.
func NewDeploymentCache(cacheDir string) (*DeploymentCache, error) {
	if cacheDir == "" {
		// Use default cache directory
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cacheDir = filepath.Join(home, ".mockd", "cache", "deployments")
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	return &DeploymentCache{
		cacheDir: cacheDir,
	}, nil
}

// Save saves a deployment to the cache.
func (c *DeploymentCache) Save(d *Deployment) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	cached := CachedDeployment{
		ID:          d.ID,
		MockID:      d.MockID,
		MockVersion: d.MockVersion,
		URLPath:     d.URLPath,
		Content:     d.Content,
		CachedAt:    time.Now(),
	}

	data, err := json.MarshalIndent(cached, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal deployment: %w", err)
	}

	filename := filepath.Join(c.cacheDir, d.ID+".json")
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	return nil
}

// Load loads a deployment from the cache by ID.
func (c *DeploymentCache) Load(id string) (*Deployment, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	filename := filepath.Join(c.cacheDir, id+".json")
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}

	var cached CachedDeployment
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, fmt.Errorf("failed to unmarshal deployment: %w", err)
	}

	return &Deployment{
		ID:          cached.ID,
		MockID:      cached.MockID,
		MockVersion: cached.MockVersion,
		URLPath:     cached.URLPath,
		Content:     cached.Content,
		DeployedAt:  cached.CachedAt,
	}, nil
}

// LoadAll loads all deployments from the cache.
func (c *DeploymentCache) LoadAll() ([]*Deployment, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	files, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read cache directory: %w", err)
	}

	var deployments []*Deployment
	for _, file := range files {
		if file.IsDir() || filepath.Ext(file.Name()) != ".json" {
			continue
		}

		filename := filepath.Join(c.cacheDir, file.Name())
		data, err := os.ReadFile(filename)
		if err != nil {
			continue // Skip unreadable files
		}

		var cached CachedDeployment
		if err := json.Unmarshal(data, &cached); err != nil {
			continue // Skip malformed files
		}

		deployments = append(deployments, &Deployment{
			ID:          cached.ID,
			MockID:      cached.MockID,
			MockVersion: cached.MockVersion,
			URLPath:     cached.URLPath,
			Content:     cached.Content,
			DeployedAt:  cached.CachedAt,
		})
	}

	return deployments, nil
}

// Delete removes a deployment from the cache.
func (c *DeploymentCache) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	filename := filepath.Join(c.cacheDir, id+".json")
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache file: %w", err)
	}

	return nil
}

// Clear removes all deployments from the cache.
func (c *DeploymentCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	files, err := os.ReadDir(c.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filename := filepath.Join(c.cacheDir, file.Name())
		_ = os.Remove(filename)
	}

	return nil
}

// DeploymentIndex maintains an index of deployments by URL path for fast lookups.
type DeploymentIndex struct {
	mu     sync.RWMutex
	byID   map[string]*Deployment
	byPath map[string]*Deployment
	cache  *DeploymentCache
}

// NewDeploymentIndex creates a new deployment index with optional cache.
func NewDeploymentIndex(cache *DeploymentCache) *DeploymentIndex {
	return &DeploymentIndex{
		byID:   make(map[string]*Deployment),
		byPath: make(map[string]*Deployment),
		cache:  cache,
	}
}

// LoadFromCache loads all deployments from cache into the index.
func (idx *DeploymentIndex) LoadFromCache() error {
	if idx.cache == nil {
		return nil
	}

	deployments, err := idx.cache.LoadAll()
	if err != nil {
		return err
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	for _, d := range deployments {
		idx.byID[d.ID] = d
		idx.byPath[d.URLPath] = d
	}

	return nil
}

// Add adds a deployment to the index and optionally caches it.
func (idx *DeploymentIndex) Add(d *Deployment) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Remove any existing deployment at this path
	if existing, ok := idx.byPath[d.URLPath]; ok {
		delete(idx.byID, existing.ID)
	}

	idx.byID[d.ID] = d
	idx.byPath[d.URLPath] = d

	// Persist to cache
	if idx.cache != nil {
		if err := idx.cache.Save(d); err != nil {
			return err
		}
	}

	return nil
}

// Remove removes a deployment from the index and cache.
func (idx *DeploymentIndex) Remove(id string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	d, ok := idx.byID[id]
	if !ok {
		return nil
	}

	delete(idx.byID, id)
	delete(idx.byPath, d.URLPath)

	// Remove from cache
	if idx.cache != nil {
		if err := idx.cache.Delete(id); err != nil {
			return err
		}
	}

	return nil
}

// GetByPath returns a deployment by URL path.
func (idx *DeploymentIndex) GetByPath(path string) (*Deployment, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	d, ok := idx.byPath[path]
	return d, ok
}

// GetByID returns a deployment by ID.
func (idx *DeploymentIndex) GetByID(id string) (*Deployment, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	d, ok := idx.byID[id]
	return d, ok
}

// All returns all deployments.
func (idx *DeploymentIndex) All() []*Deployment {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	deployments := make([]*Deployment, 0, len(idx.byID))
	for _, d := range idx.byID {
		deployments = append(deployments, d)
	}
	return deployments
}

// Count returns the number of deployments.
func (idx *DeploymentIndex) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.byID)
}
