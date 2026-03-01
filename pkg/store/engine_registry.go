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

// TunnelConfig defines tunnel settings for an engine.
// Stored by admin, delivered to engine, presented to relay via JWT.
type TunnelConfig struct {
	Enabled      bool           `json:"enabled"`
	Subdomain    string         `json:"subdomain,omitempty"`    // Custom subdomain (Pro+)
	CustomDomain string         `json:"customDomain,omitempty"` // Custom domain (Pro+)
	Expose       TunnelExposure `json:"expose"`
	Auth         *TunnelAuth    `json:"auth,omitempty"` // Incoming request auth

	// Runtime state (read-only, set by relay/engine)
	PublicURL   string     `json:"publicUrl,omitempty"`
	Status      string     `json:"status,omitempty"` // "connected","disconnected","connecting","error"
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`
	SessionID   string     `json:"sessionId,omitempty"`
	Transport   string     `json:"transport,omitempty"` // "quic","websocket"
}

// TunnelExposure defines what to expose through the tunnel.
type TunnelExposure struct {
	// Mode controls the exposure strategy.
	//   "all"      - expose everything on the engine
	//   "selected" - apply include/exclude filters
	//   "none"     - tunnel connected but nothing routed
	Mode string `json:"mode"` // "all", "selected", "none"

	// Include filters (union -- mock matches ANY of these)
	Workspaces []string `json:"workspaces,omitempty"` // Workspace IDs or names
	Folders    []string `json:"folders,omitempty"`    // Folder IDs (recursive into subfolders)
	Mocks      []string `json:"mocks,omitempty"`      // Specific mock IDs
	Types      []string `json:"types,omitempty"`      // mock.Type filter: "http","grpc","mqtt","websocket","graphql","soap","oauth"

	// Exclude filters (applied after includes)
	Exclude *TunnelExclude `json:"exclude,omitempty"`
}

// TunnelExclude defines what to exclude from tunnel exposure.
type TunnelExclude struct {
	Workspaces []string `json:"workspaces,omitempty"`
	Folders    []string `json:"folders,omitempty"`
	Mocks      []string `json:"mocks,omitempty"`
}

// TunnelAuth protects incoming requests through the tunnel.
type TunnelAuth struct {
	Type       string   `json:"type"`                 // "none","token","basic","ip"
	Token      string   `json:"token,omitempty"`      // For type=token: required header value
	Username   string   `json:"username,omitempty"`   // For type=basic
	Password   string   `json:"password,omitempty"`   // For type=basic
	AllowedIPs []string `json:"allowedIPs,omitempty"` // For type=ip: CIDR ranges
}

// TunnelStats holds runtime statistics for an active tunnel.
type TunnelStats struct {
	RequestsServed    int64  `json:"requestsServed"`
	BytesIn           int64  `json:"bytesIn"`
	BytesOut          int64  `json:"bytesOut"`
	Uptime            string `json:"uptime"`
	ActiveConnections int    `json:"activeConnections"`
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
	Tunnel         *TunnelConfig     `json:"tunnel,omitempty"`      // Tunnel configuration

	// RootWorkspaceID is the workspace whose mocks are served at "/" (no prefix)
	// on this engine. All other workspaces have their mocks prefixed with
	// their BasePath. Defaults to "local" for the local engine.
	RootWorkspaceID string `json:"rootWorkspaceId,omitempty"`
}

// EngineRegistry manages registered engines in memory.
type EngineRegistry struct {
	engines map[string]*Engine
	mu      sync.RWMutex

	// For background health check goroutine
	stopCh   chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
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
	if e.Tunnel != nil {
		tunnelCopy := *e.Tunnel
		if e.Tunnel.Auth != nil {
			authCopy := *e.Tunnel.Auth
			if e.Tunnel.Auth.AllowedIPs != nil {
				authCopy.AllowedIPs = make([]string, len(e.Tunnel.Auth.AllowedIPs))
				copy(authCopy.AllowedIPs, e.Tunnel.Auth.AllowedIPs)
			}
			tunnelCopy.Auth = &authCopy
		}
		// Deep copy exposure slices
		if e.Tunnel.Expose.Workspaces != nil {
			tunnelCopy.Expose.Workspaces = make([]string, len(e.Tunnel.Expose.Workspaces))
			copy(tunnelCopy.Expose.Workspaces, e.Tunnel.Expose.Workspaces)
		}
		if e.Tunnel.Expose.Folders != nil {
			tunnelCopy.Expose.Folders = make([]string, len(e.Tunnel.Expose.Folders))
			copy(tunnelCopy.Expose.Folders, e.Tunnel.Expose.Folders)
		}
		if e.Tunnel.Expose.Mocks != nil {
			tunnelCopy.Expose.Mocks = make([]string, len(e.Tunnel.Expose.Mocks))
			copy(tunnelCopy.Expose.Mocks, e.Tunnel.Expose.Mocks)
		}
		if e.Tunnel.Expose.Types != nil {
			tunnelCopy.Expose.Types = make([]string, len(e.Tunnel.Expose.Types))
			copy(tunnelCopy.Expose.Types, e.Tunnel.Expose.Types)
		}
		if e.Tunnel.Expose.Exclude != nil {
			excludeCopy := *e.Tunnel.Expose.Exclude
			if e.Tunnel.Expose.Exclude.Workspaces != nil {
				excludeCopy.Workspaces = make([]string, len(e.Tunnel.Expose.Exclude.Workspaces))
				copy(excludeCopy.Workspaces, e.Tunnel.Expose.Exclude.Workspaces)
			}
			if e.Tunnel.Expose.Exclude.Folders != nil {
				excludeCopy.Folders = make([]string, len(e.Tunnel.Expose.Exclude.Folders))
				copy(excludeCopy.Folders, e.Tunnel.Expose.Exclude.Folders)
			}
			if e.Tunnel.Expose.Exclude.Mocks != nil {
				excludeCopy.Mocks = make([]string, len(e.Tunnel.Expose.Exclude.Mocks))
				copy(excludeCopy.Mocks, e.Tunnel.Expose.Exclude.Mocks)
			}
			tunnelCopy.Expose.Exclude = &excludeCopy
		}
		engineCopy.Tunnel = &tunnelCopy
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
// It returns true if the engine was previously offline (an offline→online
// transition), which callers can use to trigger a store-to-engine sync.
func (r *EngineRegistry) Heartbeat(id string) (wasOffline bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	engine, exists := r.engines[id]
	if !exists {
		return false, ErrNotFound
	}

	wasOffline = engine.Status == EngineStatusOffline
	engine.LastSeen = time.Now()
	engine.Status = EngineStatusOnline
	return wasOffline, nil
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

// SetTunnelConfig sets the tunnel configuration for an engine.
func (r *EngineRegistry) SetTunnelConfig(engineID string, cfg *TunnelConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	engine, exists := r.engines[engineID]
	if !exists {
		return ErrNotFound
	}

	engine.Tunnel = cfg
	return nil
}

// GetTunnelConfig returns the tunnel configuration for an engine.
// The returned value is a deep copy — callers can safely modify it.
func (r *EngineRegistry) GetTunnelConfig(engineID string) (*TunnelConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	engine, exists := r.engines[engineID]
	if !exists {
		return nil, ErrNotFound
	}

	if engine.Tunnel == nil {
		return nil, nil
	}

	return deepCopyTunnelConfig(engine.Tunnel), nil
}

// deepCopyTunnelConfig returns a fully independent copy of a TunnelConfig.
func deepCopyTunnelConfig(src *TunnelConfig) *TunnelConfig {
	cfg := *src // shallow copy scalars and strings

	// Deep copy Expose slices
	cfg.Expose.Workspaces = copyStringSlice(src.Expose.Workspaces)
	cfg.Expose.Folders = copyStringSlice(src.Expose.Folders)
	cfg.Expose.Mocks = copyStringSlice(src.Expose.Mocks)
	cfg.Expose.Types = copyStringSlice(src.Expose.Types)

	// Deep copy Exclude
	if src.Expose.Exclude != nil {
		excl := *src.Expose.Exclude
		excl.Workspaces = copyStringSlice(src.Expose.Exclude.Workspaces)
		excl.Folders = copyStringSlice(src.Expose.Exclude.Folders)
		excl.Mocks = copyStringSlice(src.Expose.Exclude.Mocks)
		cfg.Expose.Exclude = &excl
	}

	// Deep copy Auth
	if src.Auth != nil {
		auth := *src.Auth
		auth.AllowedIPs = copyStringSlice(src.Auth.AllowedIPs)
		cfg.Auth = &auth
	}

	// Deep copy ConnectedAt
	if src.ConnectedAt != nil {
		t := *src.ConnectedAt
		cfg.ConnectedAt = &t
	}

	return &cfg
}

// copyStringSlice returns a new independent copy of a string slice.
func copyStringSlice(s []string) []string {
	if s == nil {
		return nil
	}
	cp := make([]string, len(s))
	copy(cp, s)
	return cp
}

// UpdateTunnelStatus updates the runtime tunnel status for an engine.
func (r *EngineRegistry) UpdateTunnelStatus(engineID, status, publicURL, sessionID, transport string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	engine, exists := r.engines[engineID]
	if !exists {
		return ErrNotFound
	}

	if engine.Tunnel == nil {
		return ErrNotFound
	}

	engine.Tunnel.Status = status
	if publicURL != "" {
		engine.Tunnel.PublicURL = publicURL
	}
	if sessionID != "" {
		engine.Tunnel.SessionID = sessionID
	}
	if transport != "" {
		engine.Tunnel.Transport = transport
	}
	if status == "connected" {
		now := time.Now()
		engine.Tunnel.ConnectedAt = &now
	}
	return nil
}

// ListTunnels returns all engines that have tunnel configurations.
func (r *EngineRegistry) ListTunnels() []*Engine {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var engines []*Engine
	for _, engine := range r.engines {
		if engine.Tunnel != nil && engine.Tunnel.Enabled {
			engines = append(engines, engine.Copy())
		}
	}
	return engines
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

// Stop stops the background health check goroutine. Safe to call multiple times.
func (r *EngineRegistry) Stop() {
	r.stopOnce.Do(func() {
		close(r.stopCh)
	})
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
