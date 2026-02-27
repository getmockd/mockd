package admin

import (
	"context"
	"fmt"
	"sync"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
)

// engineTarget represents an engine that should receive a mock push.
type engineTarget struct {
	client *engineclient.Client
	label  string // human-readable label for logging (e.g., "local" or "engine-id:host:port")
}

// enginePushResult represents the result of pushing to a single engine.
type enginePushResult struct {
	Label string
	Err   error
}

// allEngineTargets returns all engines that should receive mock updates:
// the local engine (if set) plus all registered engines with "online" status.
//
// If a registered engine has the same base URL as the local engine, it is
// deduplicated (the local target takes precedence) to avoid sending the
// same request twice to the same engine.
func (a *API) allEngineTargets() []engineTarget {
	var targets []engineTarget

	// Track the local engine URL so we can skip duplicates from the registry.
	var localURL string

	// Include the local (co-located) engine
	if client := a.localEngine.Load(); client != nil {
		localURL = client.BaseURL()
		targets = append(targets, engineTarget{client: client, label: "local"})
	}

	// Include all registered remote engines that are online
	for _, eng := range a.engineRegistry.List() {
		if eng.Status != store.EngineStatusOnline {
			continue
		}
		url := fmt.Sprintf("http://%s:%d", eng.Host, eng.Port)

		// Deduplicate: skip if this registered engine points to the same
		// URL as the local engine (common in `mockd up` where the engine
		// auto-registers via HTTP and also sets localEngine).
		if localURL != "" && url == localURL {
			continue
		}

		client := engineclient.New(url)
		// If the engine has a token, set it
		if eng.Token != "" {
			client = engineclient.New(url, engineclient.WithToken(eng.Token))
		}
		targets = append(targets, engineTarget{
			client: client,
			label:  fmt.Sprintf("%s (%s:%d)", eng.ID, eng.Host, eng.Port),
		})
	}

	return targets
}

// pushCreateToEngines sends a CreateMock to all engine targets in parallel.
// Returns the first engine error (if any) and logs warnings for other failures.
// If localEngine fails, it is treated as a hard error (returned); remote engine
// failures are logged as warnings only (heartbeat sync will catch up).
func (a *API) pushCreateToEngines(ctx context.Context, m *mock.Mock) (*mock.Mock, error) {
	targets := a.allEngineTargets()
	if len(targets) == 0 {
		return m, nil
	}

	// If only localEngine, use the fast path (no goroutines)
	if len(targets) == 1 && targets[0].label == "local" {
		return targets[0].client.CreateMock(ctx, m)
	}

	// Fan-out to all targets in parallel
	type createResult struct {
		label string
		mock  *mock.Mock
		err   error
	}
	results := make(chan createResult, len(targets))

	for _, t := range targets {
		go func(t engineTarget) {
			created, err := t.client.CreateMock(ctx, m)
			results <- createResult{label: t.label, mock: created, err: err}
		}(t)
	}

	var (
		localErr    error
		localResult *mock.Mock
	)

	for range targets {
		r := <-results
		if r.err != nil {
			if r.label == "local" {
				localErr = r.err
			} else {
				a.logger().Warn("failed to push create to remote engine",
					"engine", r.label, "mockID", m.ID, "error", r.err)
			}
		} else if r.label == "local" {
			localResult = r.mock
		}
	}

	if localErr != nil {
		return nil, localErr
	}

	// Return the local result if available, otherwise the original mock
	if localResult != nil {
		return localResult, nil
	}
	return m, nil
}

// pushUpdateToEngines sends an UpdateMock to all engine targets in parallel.
//
//nolint:unparam // return value kept for API consistency with pushCreateToEngines
func (a *API) pushUpdateToEngines(ctx context.Context, id string, m *mock.Mock) (*mock.Mock, error) {
	targets := a.allEngineTargets()
	if len(targets) == 0 {
		return m, nil
	}

	if len(targets) == 1 && targets[0].label == "local" {
		return targets[0].client.UpdateMock(ctx, id, m)
	}

	type updateResult struct {
		label string
		mock  *mock.Mock
		err   error
	}
	results := make(chan updateResult, len(targets))

	for _, t := range targets {
		go func(t engineTarget) {
			updated, err := t.client.UpdateMock(ctx, id, m)
			results <- updateResult{label: t.label, mock: updated, err: err}
		}(t)
	}

	var (
		localErr    error
		localResult *mock.Mock
	)

	for range targets {
		r := <-results
		if r.err != nil {
			if r.label == "local" {
				localErr = r.err
			} else {
				a.logger().Warn("failed to push update to remote engine",
					"engine", r.label, "mockID", id, "error", r.err)
			}
		} else if r.label == "local" {
			localResult = r.mock
		}
	}

	if localErr != nil {
		return nil, localErr
	}
	if localResult != nil {
		return localResult, nil
	}
	return m, nil
}

// pushDeleteToEngines sends a DeleteMock to all engine targets in parallel.
func (a *API) pushDeleteToEngines(ctx context.Context, id string) error {
	targets := a.allEngineTargets()
	if len(targets) == 0 {
		return nil
	}

	if len(targets) == 1 && targets[0].label == "local" {
		return targets[0].client.DeleteMock(ctx, id)
	}

	results := make(chan enginePushResult, len(targets))

	for _, t := range targets {
		go func(t engineTarget) {
			err := t.client.DeleteMock(ctx, id)
			results <- enginePushResult{Label: t.label, Err: err}
		}(t)
	}

	var localErr error
	for range targets {
		r := <-results
		if r.Err != nil {
			if r.Label == "local" {
				localErr = r.Err
			} else {
				a.logger().Warn("failed to push delete to remote engine",
					"engine", r.Label, "mockID", id, "error", r.Err)
			}
		}
	}

	return localErr
}

// pushToggleToEngines sends a ToggleMock to all engine targets in parallel.
func (a *API) pushToggleToEngines(ctx context.Context, id string, enabled bool) (*mock.Mock, error) {
	targets := a.allEngineTargets()
	if len(targets) == 0 {
		return nil, nil
	}

	if len(targets) == 1 && targets[0].label == "local" {
		return targets[0].client.ToggleMock(ctx, id, enabled)
	}

	type toggleResult struct {
		label string
		mock  *mock.Mock
		err   error
	}
	results := make(chan toggleResult, len(targets))

	for _, t := range targets {
		go func(t engineTarget) {
			toggled, err := t.client.ToggleMock(ctx, id, enabled)
			results <- toggleResult{label: t.label, mock: toggled, err: err}
		}(t)
	}

	var (
		localErr    error
		localResult *mock.Mock
	)

	for range targets {
		r := <-results
		if r.err != nil {
			if r.label == "local" {
				localErr = r.err
			} else {
				a.logger().Warn("failed to push toggle to remote engine",
					"engine", r.label, "mockID", id, "error", r.err)
			}
		} else if r.label == "local" {
			localResult = r.mock
		}
	}

	if localErr != nil {
		return nil, localErr
	}
	return localResult, nil
}

// pushImportToEngines sends an ImportConfig to all engine targets in parallel.
// Returns the result from the local engine (if any), or the first remote result.
func (a *API) pushImportToEngines(ctx context.Context, collection *config.MockCollection, replace bool) (*engineclient.ImportResult, error) {
	targets := a.allEngineTargets()
	if len(targets) == 0 {
		return &engineclient.ImportResult{Imported: 0, Total: len(collection.Mocks)}, nil
	}

	if len(targets) == 1 && targets[0].label == "local" {
		return targets[0].client.ImportConfig(ctx, collection, replace)
	}

	type importResult struct {
		label  string
		result *engineclient.ImportResult
		err    error
	}
	results := make(chan importResult, len(targets))

	for _, t := range targets {
		go func(t engineTarget) {
			res, err := t.client.ImportConfig(ctx, collection, replace)
			results <- importResult{label: t.label, result: res, err: err}
		}(t)
	}

	var (
		localErr    error
		localResult *engineclient.ImportResult
	)

	for range targets {
		r := <-results
		if r.err != nil {
			if r.label == "local" {
				localErr = r.err
			} else {
				a.logger().Warn("failed to push import to remote engine",
					"engine", r.label, "error", r.err)
			}
		} else if r.label == "local" {
			localResult = r.result
		}
	}

	if localErr != nil {
		return nil, localErr
	}
	if localResult != nil {
		return localResult, nil
	}
	return &engineclient.ImportResult{Imported: len(collection.Mocks), Total: len(collection.Mocks)}, nil
}

// pushDeleteBulkToEngines sends DeleteMock for each mock to all engine targets.
// Failures on remote engines are logged as warnings; localEngine failures are best-effort.
func (a *API) pushDeleteBulkToEngines(ctx context.Context, mocks []*mock.Mock) {
	targets := a.allEngineTargets()
	if len(targets) == 0 {
		return
	}

	for _, t := range targets {
		for _, m := range mocks {
			if err := t.client.DeleteMock(ctx, m.ID); err != nil {
				a.logger().Warn("failed to notify engine of mock deletion (bulk)",
					"engine", t.label, "mockID", m.ID, "error", err)
			}
		}
	}
}

// perEngineSyncMu manages per-engine sync mutexes so concurrent full-syncs
// to different engines don't block each other.
type perEngineSyncMu struct {
	mu      sync.Mutex
	engines map[string]*sync.Mutex
}

// newPerEngineSyncMu creates a new per-engine mutex manager.
func newPerEngineSyncMu() *perEngineSyncMu {
	return &perEngineSyncMu{
		engines: make(map[string]*sync.Mutex),
	}
}

// get returns the mutex for a given engine URL, creating one if needed.
func (p *perEngineSyncMu) get(engineURL string) *sync.Mutex {
	p.mu.Lock()
	defer p.mu.Unlock()

	if m, ok := p.engines[engineURL]; ok {
		return m
	}

	m := &sync.Mutex{}
	p.engines[engineURL] = m
	return m
}
