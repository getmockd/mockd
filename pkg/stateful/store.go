package stateful

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// DefaultWorkspaceID is the workspace used when no workspace is specified.
const DefaultWorkspaceID = ""

// StateStore is the global container managing all stateful resources,
// partitioned by workspace.
type StateStore struct {
	mu         sync.RWMutex
	workspaces map[string]map[string]*StatefulResource // workspaceID → name → resource
	observer   Observer
}

// NewStateStore creates a new StateStore.
func NewStateStore() *StateStore {
	return &StateStore{
		workspaces: make(map[string]map[string]*StatefulResource),
		observer:   &NoopObserver{},
	}
}

// SetObserver sets the observer for metrics/logging hooks.
// Pass nil to disable observation (uses NoopObserver).
func (s *StateStore) SetObserver(obs Observer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if obs == nil {
		s.observer = &NoopObserver{}
	} else {
		s.observer = obs
	}
}

// GetObserver returns the current observer.
func (s *StateStore) GetObserver() Observer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.observer
}

// workspace returns the resource map for a workspace, creating it if needed.
// Must be called with s.mu held for writing.
func (s *StateStore) workspace(workspaceID string) map[string]*StatefulResource {
	ws, ok := s.workspaces[workspaceID]
	if !ok {
		ws = make(map[string]*StatefulResource)
		s.workspaces[workspaceID] = ws
	}
	return ws
}

// workspaceRO returns the resource map for a workspace, or nil if it doesn't exist.
// Must be called with s.mu held for reading.
func (s *StateStore) workspaceRO(workspaceID string) map[string]*StatefulResource {
	return s.workspaces[workspaceID]
}

// Register adds a new stateful resource to the store in the given workspace.
func (s *StateStore) Register(workspaceID string, config *ResourceConfig) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}

	if config.Name == "" {
		return errors.New("resource name cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ws := s.workspace(workspaceID)
	if _, exists := ws[config.Name]; exists {
		return fmt.Errorf("resource %q already registered", config.Name)
	}

	resource := NewStatefulResource(config)

	// Load seed data if provided
	if err := resource.loadSeed(); err != nil {
		return fmt.Errorf("failed to load seed data for %q: %w", config.Name, err)
	}

	ws[config.Name] = resource
	return nil
}

// Get returns a stateful resource by name from the given workspace.
func (s *StateStore) Get(workspaceID string, name string) *StatefulResource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ws := s.workspaceRO(workspaceID)
	if ws == nil {
		return nil
	}
	return ws[name]
}

// List returns all resource names in the given workspace in sorted order.
func (s *StateStore) List(workspaceID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ws := s.workspaceRO(workspaceID)
	if ws == nil {
		return nil
	}

	names := make([]string, 0, len(ws))
	for name := range ws {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Reset resets stateful resources to their initial seed state.
// If resourceName is empty, all resources in the workspace are reset.
// The store lock is only held to snapshot which resources to reset;
// individual resource resets use the resource's own mutex so they
// don't block other store operations for the entire duration.
func (s *StateStore) Reset(workspaceID string, resourceName string) (*ResetResponse, error) {
	start := time.Now()

	// Snapshot the target resources under a read lock.
	s.mu.RLock()
	type target struct {
		name     string
		resource *StatefulResource
	}
	var targets []target

	ws := s.workspaceRO(workspaceID)
	if ws == nil && resourceName != "" {
		s.mu.RUnlock()
		return nil, fmt.Errorf("resource %q not found", resourceName)
	}

	if resourceName == "" {
		if ws != nil {
			targets = make([]target, 0, len(ws))
			for name, resource := range ws {
				targets = append(targets, target{name, resource})
			}
		}
	} else {
		resource, ok := ws[resourceName]
		if !ok {
			s.mu.RUnlock()
			return nil, fmt.Errorf("resource %q not found", resourceName)
		}
		targets = []target{{resourceName, resource}}
	}
	observer := s.observer
	s.mu.RUnlock()

	// Reset each resource outside the store lock.
	// Each resource.Reset() acquires its own per-resource mutex.
	resetNames := make([]string, 0, len(targets))
	for _, t := range targets {
		t.resource.Reset()
		resetNames = append(resetNames, t.name)
	}
	sort.Strings(resetNames) // deterministic ordering

	observer.OnReset(resetNames, time.Since(start))

	return &ResetResponse{
		Reset:     true,
		Resources: resetNames,
		Message:   "State reset to seed data",
	}, nil
}

// Clear removes all resources from the given workspace.
// If workspaceID is empty string, clears the default workspace.
func (s *StateStore) Clear(workspaceID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.workspaces, workspaceID)
}

// ClearAll removes all resources from all workspaces.
func (s *StateStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workspaces = make(map[string]map[string]*StatefulResource)
}

// Overview returns information about all registered stateful resources in a workspace.
// Resource list is sorted for deterministic output.
func (s *StateStore) Overview(workspaceID string) *StateOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ws := s.workspaceRO(workspaceID)
	if ws == nil {
		return &StateOverview{
			Resources:    0,
			TotalItems:   0,
			ResourceList: []string{},
		}
	}

	totalItems := 0
	names := make([]string, 0, len(ws))

	for name, resource := range ws {
		names = append(names, name)
		totalItems += resource.Count()
	}

	sort.Strings(names)

	return &StateOverview{
		Resources:    len(ws),
		TotalItems:   totalItems,
		ResourceList: names,
	}
}

// ResourceInfo returns details about a specific stateful resource in a workspace.
func (s *StateStore) ResourceInfo(workspaceID string, name string) (*ResourceInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ws := s.workspaceRO(workspaceID)
	if ws == nil {
		return nil, fmt.Errorf("resource %q not found", name)
	}

	resource, ok := ws[name]
	if !ok {
		return nil, fmt.Errorf("resource %q not found", name)
	}

	return resource.Info(), nil
}

// ListConfigs returns the config for every registered resource in a workspace.
// Used by Export to serialize resource definitions back to config format.
func (s *StateStore) ListConfigs(workspaceID string) []*ResourceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ws := s.workspaceRO(workspaceID)
	if ws == nil {
		return nil
	}

	configs := make([]*ResourceConfig, 0, len(ws))
	for _, r := range ws {
		configs = append(configs, r.Config())
	}
	return configs
}

// ClearResource removes all items from a specific resource (does not restore seed data).
func (s *StateStore) ClearResource(workspaceID string, name string) (int, error) {
	s.mu.RLock()
	ws := s.workspaceRO(workspaceID)
	if ws == nil {
		s.mu.RUnlock()
		return 0, fmt.Errorf("resource %q not found", name)
	}
	resource, ok := ws[name]
	s.mu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("resource %q not found", name)
	}

	return resource.Clear(), nil
}

// Unregister removes a stateful resource definition from the store entirely.
// Unlike ClearResource (which only empties items), this fully removes the resource.
func (s *StateStore) Unregister(workspaceID string, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ws := s.workspaceRO(workspaceID)
	if ws == nil {
		return fmt.Errorf("resource %q not found", name)
	}
	if _, exists := ws[name]; !exists {
		return fmt.Errorf("resource %q not found", name)
	}
	delete(ws, name)
	return nil
}
