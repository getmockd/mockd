// Package store provides engine registry for tracking mock engines.
package store

import (
	"context"
	"sync"
	"time"
)

// EngineStatus represents the status of an engine.
type EngineStatus string

const (
	EngineStatusOnline  EngineStatus = "online"
	EngineStatusOffline EngineStatus = "offline"
)

// Default port range for workspace assignments
const (
	DefaultPortRangeStart = 9000
	DefaultPortRangeEnd   = 9999
)

// EngineWorkspace represents a workspace assigned to an engine.
type EngineWorkspace struct {
	WorkspaceID   string    `json:"workspaceId"`
	WorkspaceName string    `json:"workspaceName"`
	HTTPPort      int       `json:"httpPort"`
	GRPCPort      int       `json:"grpcPort,omitempty"`
	MQTTPort      int       `json:"mqttPort,omitempty"`
	MockCount     int       `json:"mockCount"`
	WSCount       int       `json:"wsCount"`
	GraphQLCount  int       `json:"graphqlCount"`
	GRPCCount     int       `json:"grpcCount"`
	SOAPCount     int       `json:"soapCount"`
	MQTTCount     int       `json:"mqttCount"`
	Status        string    `json:"status"` // "running", "stopped", "error"
	LastSynced    time.Time `json:"lastSynced,omitempty"`
}

// Engine represents a registered mock engine.
type Engine struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Host           string            `json:"host"`
	Port           int               `json:"port"`
	WorkspaceID    string            `json:"workspaceId,omitempty"` // Deprecated: use Workspaces instead
	Workspaces     []EngineWorkspace `json:"workspaces"`
	PortRangeStart int               `json:"portRangeStart"`
	PortRangeEnd   int               `json:"portRangeEnd"`
	Status         EngineStatus      `json:"status"`
	LastSeen       time.Time         `json:"lastSeen"`
	Version        string            `json:"version,omitempty"`
	RegisteredAt   time.Time         `json:"registeredAt"`
	Fingerprint    string            `json:"fingerprint,omitempty"` // Machine fingerprint for identity verification
	Token          string            `json:"-"`                     // Engine-specific token, not exposed in JSON
}

// EngineRegistry manages registered engines in memory.
type EngineRegistry struct {
	engines map[string]*Engine
	mu      sync.RWMutex

	// For background health check goroutine
	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewEngineRegistry creates a new engine registry.
func NewEngineRegistry() *EngineRegistry {
	return &EngineRegistry{
		engines: make(map[string]*Engine),
		stopCh:  make(chan struct{}),
	}
}

// Register adds or updates an engine in the registry.
func (r *EngineRegistry) Register(engine *Engine) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if engine.RegisteredAt.IsZero() {
		engine.RegisteredAt = now
	}
	engine.LastSeen = now
	engine.Status = EngineStatusOnline

	// Initialize default port range if not set
	if engine.PortRangeStart == 0 {
		engine.PortRangeStart = DefaultPortRangeStart
	}
	if engine.PortRangeEnd == 0 {
		engine.PortRangeEnd = DefaultPortRangeEnd
	}

	// Initialize workspaces slice if nil
	if engine.Workspaces == nil {
		engine.Workspaces = []EngineWorkspace{}
	}

	r.engines[engine.ID] = engine
	return nil
}

// Unregister removes an engine from the registry.
func (r *EngineRegistry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.engines[id]; !exists {
		return ErrNotFound
	}

	delete(r.engines, id)
	return nil
}

// Get retrieves an engine by ID.
func (r *EngineRegistry) Get(id string) (*Engine, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	engine, exists := r.engines[id]
	if !exists {
		return nil, ErrNotFound
	}

	// Return a deep copy to avoid race conditions
	return engine.Copy(), nil
}

// List returns all registered engines.
func (r *EngineRegistry) List() []*Engine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	engines := make([]*Engine, 0, len(r.engines))
	for _, engine := range r.engines {
		engines = append(engines, engine.Copy())
	}
	return engines
}

// Copy returns a deep copy of the engine.
func (e *Engine) Copy() *Engine {
	engineCopy := *e
	if e.Workspaces != nil {
		engineCopy.Workspaces = make([]EngineWorkspace, len(e.Workspaces))
		copy(engineCopy.Workspaces, e.Workspaces)
	}
	return &engineCopy
}

// UpdateStatus updates an engine's status.
func (r *EngineRegistry) UpdateStatus(id string, status EngineStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	engine, exists := r.engines[id]
	if !exists {
		return ErrNotFound
	}

	engine.Status = status
	return nil
}

// Heartbeat updates an engine's lastSeen time and sets status to online.
func (r *EngineRegistry) Heartbeat(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	engine, exists := r.engines[id]
	if !exists {
		return ErrNotFound
	}

	engine.LastSeen = time.Now()
	engine.Status = EngineStatusOnline
	return nil
}

// AssignWorkspace assigns a workspace to an engine (deprecated, use AddWorkspace instead).
func (r *EngineRegistry) AssignWorkspace(id string, workspaceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	engine, exists := r.engines[id]
	if !exists {
		return ErrNotFound
	}

	engine.WorkspaceID = workspaceID
	return nil
}

// AddWorkspace adds a workspace to an engine with port assignments.
func (e *Engine) AddWorkspace(workspaceID, workspaceName string, httpPort, grpcPort, mqttPort int) *EngineWorkspace {
	// Check if workspace already exists
	for i := range e.Workspaces {
		if e.Workspaces[i].WorkspaceID == workspaceID {
			// Update existing workspace
			e.Workspaces[i].WorkspaceName = workspaceName
			if httpPort > 0 {
				e.Workspaces[i].HTTPPort = httpPort
			}
			if grpcPort > 0 {
				e.Workspaces[i].GRPCPort = grpcPort
			}
			if mqttPort > 0 {
				e.Workspaces[i].MQTTPort = mqttPort
			}
			return &e.Workspaces[i]
		}
	}

	// Create new workspace
	ws := EngineWorkspace{
		WorkspaceID:   workspaceID,
		WorkspaceName: workspaceName,
		HTTPPort:      httpPort,
		GRPCPort:      grpcPort,
		MQTTPort:      mqttPort,
		Status:        "stopped",
	}

	// Auto-assign HTTP port if not provided
	if ws.HTTPPort == 0 {
		ws.HTTPPort = e.FindAvailablePort()
	}

	e.Workspaces = append(e.Workspaces, ws)
	return &e.Workspaces[len(e.Workspaces)-1]
}

// RemoveWorkspace removes a workspace from an engine.
func (e *Engine) RemoveWorkspace(workspaceID string) bool {
	for i := range e.Workspaces {
		if e.Workspaces[i].WorkspaceID == workspaceID {
			e.Workspaces = append(e.Workspaces[:i], e.Workspaces[i+1:]...)
			return true
		}
	}
	return false
}

// GetWorkspace returns a workspace by ID.
func (e *Engine) GetWorkspace(workspaceID string) *EngineWorkspace {
	for i := range e.Workspaces {
		if e.Workspaces[i].WorkspaceID == workspaceID {
			return &e.Workspaces[i]
		}
	}
	return nil
}

// FindAvailablePort finds the first available port in the engine's port range.
func (e *Engine) FindAvailablePort() int {
	usedPorts := e.GetUsedPorts()

	start := e.PortRangeStart
	end := e.PortRangeEnd
	if start == 0 {
		start = DefaultPortRangeStart
	}
	if end == 0 {
		end = DefaultPortRangeEnd
	}

	for port := start; port <= end; port++ {
		if !usedPorts[port] {
			return port
		}
	}
	return 0 // No available port
}

// GetUsedPorts returns a map of all ports currently in use by workspaces.
func (e *Engine) GetUsedPorts() map[int]bool {
	used := make(map[int]bool)
	for _, ws := range e.Workspaces {
		if ws.HTTPPort > 0 {
			used[ws.HTTPPort] = true
		}
		if ws.GRPCPort > 0 {
			used[ws.GRPCPort] = true
		}
		if ws.MQTTPort > 0 {
			used[ws.MQTTPort] = true
		}
	}
	return used
}

// UpdateWorkspace updates a workspace's port assignments.
func (e *Engine) UpdateWorkspace(workspaceID string, httpPort, grpcPort, mqttPort int) *EngineWorkspace {
	for i := range e.Workspaces {
		if e.Workspaces[i].WorkspaceID == workspaceID {
			if httpPort > 0 {
				e.Workspaces[i].HTTPPort = httpPort
			}
			if grpcPort >= 0 {
				e.Workspaces[i].GRPCPort = grpcPort
			}
			if mqttPort >= 0 {
				e.Workspaces[i].MQTTPort = mqttPort
			}
			return &e.Workspaces[i]
		}
	}
	return nil
}

// SyncWorkspace updates the last synced time for a workspace.
func (e *Engine) SyncWorkspace(workspaceID string) *EngineWorkspace {
	for i := range e.Workspaces {
		if e.Workspaces[i].WorkspaceID == workspaceID {
			e.Workspaces[i].LastSynced = time.Now()
			return &e.Workspaces[i]
		}
	}
	return nil
}

// AddWorkspaceToEngine adds a workspace to an engine by engine ID (registry method).
func (r *EngineRegistry) AddWorkspaceToEngine(engineID, workspaceID, workspaceName string, httpPort, grpcPort, mqttPort int) (*EngineWorkspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	engine, exists := r.engines[engineID]
	if !exists {
		return nil, ErrNotFound
	}

	ws := engine.AddWorkspace(workspaceID, workspaceName, httpPort, grpcPort, mqttPort)
	return ws, nil
}

// RemoveWorkspaceFromEngine removes a workspace from an engine by engine ID.
func (r *EngineRegistry) RemoveWorkspaceFromEngine(engineID, workspaceID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	engine, exists := r.engines[engineID]
	if !exists {
		return ErrNotFound
	}

	if !engine.RemoveWorkspace(workspaceID) {
		return ErrNotFound
	}
	return nil
}

// UpdateWorkspaceInEngine updates a workspace's ports in an engine.
func (r *EngineRegistry) UpdateWorkspaceInEngine(engineID, workspaceID string, httpPort, grpcPort, mqttPort int) (*EngineWorkspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	engine, exists := r.engines[engineID]
	if !exists {
		return nil, ErrNotFound
	}

	ws := engine.UpdateWorkspace(workspaceID, httpPort, grpcPort, mqttPort)
	if ws == nil {
		return nil, ErrNotFound
	}
	return ws, nil
}

// SyncWorkspaceInEngine marks a workspace as synced.
func (r *EngineRegistry) SyncWorkspaceInEngine(engineID, workspaceID string) (*EngineWorkspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	engine, exists := r.engines[engineID]
	if !exists {
		return nil, ErrNotFound
	}

	ws := engine.SyncWorkspace(workspaceID)
	if ws == nil {
		return nil, ErrNotFound
	}
	return ws, nil
}

// GetWorkspaceFromEngine retrieves a workspace from an engine.
func (r *EngineRegistry) GetWorkspaceFromEngine(engineID, workspaceID string) (*EngineWorkspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	engine, exists := r.engines[engineID]
	if !exists {
		return nil, ErrNotFound
	}

	ws := engine.GetWorkspace(workspaceID)
	if ws == nil {
		return nil, ErrNotFound
	}

	// Return a copy
	wsCopy := *ws
	return &wsCopy, nil
}

// GetEnginesForWorkspace returns all engines that have the given workspace assigned.
// Returns empty slice if workspace is not assigned to any engine.
func (r *EngineRegistry) GetEnginesForWorkspace(workspaceID string) []*Engine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var engines []*Engine
	for _, engine := range r.engines {
		for _, ws := range engine.Workspaces {
			if ws.WorkspaceID == workspaceID {
				// Return a copy
				engineCopy := *engine
				engineCopy.Workspaces = make([]EngineWorkspace, len(engine.Workspaces))
				copy(engineCopy.Workspaces, engine.Workspaces)
				engines = append(engines, &engineCopy)
				break
			}
		}
	}
	return engines
}

// GetSiblingWorkspaceIDs returns all workspace IDs that share an engine with the given workspace.
// This is useful for checking port conflicts across workspaces on the same engine.
// Returns empty slice if workspace is not assigned to any engine.
func (r *EngineRegistry) GetSiblingWorkspaceIDs(workspaceID string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	siblingSet := make(map[string]bool)
	for _, engine := range r.engines {
		// Check if this engine has the target workspace
		hasWorkspace := false
		for _, ws := range engine.Workspaces {
			if ws.WorkspaceID == workspaceID {
				hasWorkspace = true
				break
			}
		}
		// If yes, collect all workspace IDs from this engine
		if hasWorkspace {
			for _, ws := range engine.Workspaces {
				siblingSet[ws.WorkspaceID] = true
			}
		}
	}

	siblings := make([]string, 0, len(siblingSet))
	for wsID := range siblingSet {
		siblings = append(siblings, wsID)
	}
	return siblings
}

// UpdateWorkspaceStatus updates the status of a workspace in an engine.
func (r *EngineRegistry) UpdateWorkspaceStatus(engineID, workspaceID, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	engine, exists := r.engines[engineID]
	if !exists {
		return ErrNotFound
	}

	for i := range engine.Workspaces {
		if engine.Workspaces[i].WorkspaceID == workspaceID {
			engine.Workspaces[i].Status = status
			return nil
		}
	}
	return ErrNotFound
}

// StartHealthCheck starts a background goroutine that marks engines as offline
// if no heartbeat is received within the given timeout duration.
func (r *EngineRegistry) StartHealthCheck(ctx context.Context, timeout time.Duration) {
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()

		ticker := time.NewTicker(timeout / 2) // Check twice per timeout period
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-r.stopCh:
				return
			case <-ticker.C:
				r.checkEngineHealth(timeout)
			}
		}
	}()
}

// checkEngineHealth marks engines as offline if no heartbeat received within timeout.
func (r *EngineRegistry) checkEngineHealth(timeout time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for _, engine := range r.engines {
		if engine.Status == EngineStatusOnline && now.Sub(engine.LastSeen) > timeout {
			engine.Status = EngineStatusOffline
		}
	}
}

// Stop stops the background health check goroutine.
func (r *EngineRegistry) Stop() {
	close(r.stopCh)
	r.wg.Wait()
}

// Count returns the number of registered engines.
func (r *EngineRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.engines)
}

// CountByStatus returns the number of engines with the given status.
func (r *EngineRegistry) CountByStatus(status EngineStatus) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, engine := range r.engines {
		if engine.Status == status {
			count++
		}
	}
	return count
}
