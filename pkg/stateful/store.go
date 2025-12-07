package stateful

import (
	"fmt"
	"strings"
	"sync"
)

// StateStore is the global container managing all stateful resources.
type StateStore struct {
	mu        sync.RWMutex
	resources map[string]*StatefulResource
}

// NewStateStore creates a new StateStore.
func NewStateStore() *StateStore {
	return &StateStore{
		resources: make(map[string]*StatefulResource),
	}
}

// Register adds a new stateful resource to the store.
func (s *StateStore) Register(config *ResourceConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.Name == "" {
		return fmt.Errorf("resource name cannot be empty")
	}

	if config.BasePath == "" {
		return fmt.Errorf("resource basePath cannot be empty")
	}

	if !strings.HasPrefix(config.BasePath, "/") {
		return fmt.Errorf("resource basePath must start with /")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.resources[config.Name]; exists {
		return fmt.Errorf("resource %q already registered", config.Name)
	}

	resource := NewStatefulResource(config)

	// Load seed data if provided
	if err := resource.loadSeed(); err != nil {
		return fmt.Errorf("failed to load seed data for %q: %w", config.Name, err)
	}

	s.resources[config.Name] = resource
	return nil
}

// Get returns a stateful resource by name.
func (s *StateStore) Get(name string) *StatefulResource {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resources[name]
}

// List returns all resource names.
func (s *StateStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.resources))
	for name := range s.resources {
		names = append(names, name)
	}
	return names
}

// MatchPath finds a resource matching the given URL path.
// Returns the matched resource, the extracted ID (if single resource), and path params.
func (s *StateStore) MatchPath(path string) (*StatefulResource, string, map[string]string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, resource := range s.resources {
		if id, params, matched := resource.MatchPath(path); matched {
			return resource, id, params
		}
	}

	return nil, "", nil
}

// Reset resets stateful resources to their initial seed state.
// If resourceName is empty, all resources are reset.
func (s *StateStore) Reset(resourceName string) (*ResetResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var resetNames []string

	if resourceName == "" {
		// Reset all resources
		for name, resource := range s.resources {
			resource.Reset()
			resetNames = append(resetNames, name)
		}
	} else {
		// Reset specific resource
		resource, ok := s.resources[resourceName]
		if !ok {
			return nil, fmt.Errorf("resource %q not found", resourceName)
		}
		resource.Reset()
		resetNames = []string{resourceName}
	}

	return &ResetResponse{
		Reset:     true,
		Resources: resetNames,
		Message:   "State reset to seed data",
	}, nil
}

// Clear removes all resources from the store.
func (s *StateStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources = make(map[string]*StatefulResource)
}

// Overview returns information about all registered stateful resources.
func (s *StateStore) Overview() *StateOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalItems := 0
	names := make([]string, 0, len(s.resources))

	for name, resource := range s.resources {
		names = append(names, name)
		totalItems += resource.Count()
	}

	return &StateOverview{
		Resources:    len(s.resources),
		TotalItems:   totalItems,
		ResourceList: names,
	}
}

// ResourceInfo returns details about a specific stateful resource.
func (s *StateStore) ResourceInfo(name string) (*ResourceInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resource, ok := s.resources[name]
	if !ok {
		return nil, fmt.Errorf("resource %q not found", name)
	}

	return resource.Info(), nil
}

// ClearResource removes all items from a specific resource (does not restore seed data).
func (s *StateStore) ClearResource(name string) (int, error) {
	s.mu.RLock()
	resource, ok := s.resources[name]
	s.mu.RUnlock()

	if !ok {
		return 0, fmt.Errorf("resource %q not found", name)
	}

	return resource.Clear(), nil
}
