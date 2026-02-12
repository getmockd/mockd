package stateful

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// StateStore is the global container managing all stateful resources.
type StateStore struct {
	mu        sync.RWMutex
	resources map[string]*StatefulResource
	observer  Observer
}

// NewStateStore creates a new StateStore.
func NewStateStore() *StateStore {
	return &StateStore{
		resources: make(map[string]*StatefulResource),
		observer:  &NoopObserver{},
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

// Observer returns the current observer.
func (s *StateStore) GetObserver() Observer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.observer
}

// Register adds a new stateful resource to the store.
func (s *StateStore) Register(config *ResourceConfig) error {
	if config == nil {
		return errors.New("config cannot be nil")
	}

	if config.Name == "" {
		return errors.New("resource name cannot be empty")
	}

	if config.BasePath == "" {
		return errors.New("resource basePath cannot be empty")
	}

	if !strings.HasPrefix(config.BasePath, "/") {
		return errors.New("resource basePath must start with /")
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

// List returns all resource names in sorted order for deterministic output.
func (s *StateStore) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.resources))
	for name := range s.resources {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// MatchPath finds a resource matching the given URL path.
// Returns the matched resource, the extracted ID (if single resource), and path params.
// Resources are checked in order of longest BasePath first so that more specific
// routes (e.g. /api/users/:userId/orders) are matched before shorter ones (e.g. /api/users).
func (s *StateStore) MatchPath(path string) (*StatefulResource, string, map[string]string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Collect keys and sort by BasePath length descending (most specific first)
	keys := make([]string, 0, len(s.resources))
	for name := range s.resources {
		keys = append(keys, name)
	}
	sort.Slice(keys, func(i, j int) bool {
		pi := s.resources[keys[i]].BasePath()
		pj := s.resources[keys[j]].BasePath()
		if len(pi) != len(pj) {
			return len(pi) > len(pj)
		}
		return keys[i] < keys[j] // stable tiebreak by name
	})

	for _, name := range keys {
		if id, params, matched := s.resources[name].MatchPath(path); matched {
			return s.resources[name], id, params
		}
	}

	return nil, "", nil
}

// Reset resets stateful resources to their initial seed state.
// If resourceName is empty, all resources are reset.
// The store lock is only held to snapshot which resources to reset;
// individual resource resets use the resource's own mutex so they
// don't block other store operations for the entire duration.
func (s *StateStore) Reset(resourceName string) (*ResetResponse, error) {
	start := time.Now()

	// Snapshot the target resources under a read lock.
	s.mu.RLock()
	type target struct {
		name     string
		resource *StatefulResource
	}
	var targets []target
	if resourceName == "" {
		targets = make([]target, 0, len(s.resources))
		for name, resource := range s.resources {
			targets = append(targets, target{name, resource})
		}
	} else {
		resource, ok := s.resources[resourceName]
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

// Clear removes all resources from the store.
func (s *StateStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources = make(map[string]*StatefulResource)
}

// Overview returns information about all registered stateful resources.
// Resource list is sorted for deterministic output.
func (s *StateStore) Overview() *StateOverview {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalItems := 0
	names := make([]string, 0, len(s.resources))

	for name, resource := range s.resources {
		names = append(names, name)
		totalItems += resource.Count()
	}

	sort.Strings(names)

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
