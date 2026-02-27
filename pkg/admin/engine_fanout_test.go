package admin

import (
	"testing"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/store"
)

func TestAllEngineTargets_NoEngines(t *testing.T) {
	api := &API{}
	api.engineRegistry = store.NewEngineRegistry()

	targets := api.allEngineTargets()
	if len(targets) != 0 {
		t.Errorf("expected 0 targets with no engines, got %d", len(targets))
	}
}

func TestAllEngineTargets_LocalOnly(t *testing.T) {
	api := &API{}
	api.engineRegistry = store.NewEngineRegistry()

	// Set local engine
	api.SetLocalEngine(engineclient.New("http://localhost:14281"))

	targets := api.allEngineTargets()
	if len(targets) != 1 {
		t.Fatalf("expected 1 target (local), got %d", len(targets))
	}
	if targets[0].label != "local" {
		t.Errorf("expected label 'local', got %q", targets[0].label)
	}
}

func TestAllEngineTargets_LocalAndRegistered(t *testing.T) {
	api := &API{}
	api.engineRegistry = store.NewEngineRegistry()

	// Set local engine
	api.SetLocalEngine(engineclient.New("http://localhost:14281"))

	// Register a remote engine
	api.engineRegistry.Register(&store.Engine{
		ID:     "engine-1",
		Name:   "engine-1",
		Host:   "remote-host",
		Port:   4281,
		Status: store.EngineStatusOnline,
	})

	targets := api.allEngineTargets()
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets (local + remote), got %d", len(targets))
	}

	labels := make(map[string]bool)
	for _, target := range targets {
		labels[target.label] = true
	}
	if !labels["local"] {
		t.Error("expected 'local' target")
	}
}

func TestAllEngineTargets_SkipsOfflineEngines(t *testing.T) {
	api := &API{}
	api.engineRegistry = store.NewEngineRegistry()

	api.SetLocalEngine(engineclient.New("http://localhost:14281"))

	// Register an engine, then set it offline (Register always sets online)
	api.engineRegistry.Register(&store.Engine{
		ID:   "engine-offline",
		Name: "engine-offline",
		Host: "offline-host",
		Port: 4281,
	})
	_ = api.engineRegistry.UpdateStatus("engine-offline", store.EngineStatusOffline)

	targets := api.allEngineTargets()
	if len(targets) != 1 {
		t.Fatalf("expected 1 target (local only, offline skipped), got %d", len(targets))
	}
}

func TestPerEngineSyncMu_GetOrCreate(t *testing.T) {
	mu := newPerEngineSyncMu()

	m1 := mu.get("http://engine1:4281")
	m2 := mu.get("http://engine2:4281")
	m3 := mu.get("http://engine1:4281") // same as m1

	if m1 == m2 {
		t.Error("different engines should get different mutexes")
	}
	if m1 != m3 {
		t.Error("same engine should get same mutex")
	}
}

func TestAllEngineTargets_DeduplicatesLocalEngine(t *testing.T) {
	api := &API{}
	api.engineRegistry = store.NewEngineRegistry()

	// Set local engine to localhost:14281
	api.SetLocalEngine(engineclient.New("http://localhost:14281"))

	// Register an engine with the SAME URL (simulates `mockd up` auto-registration)
	api.engineRegistry.Register(&store.Engine{
		ID:   "engine-local",
		Name: "engine-local",
		Host: "localhost",
		Port: 14281,
	})

	targets := api.allEngineTargets()
	if len(targets) != 1 {
		t.Fatalf("expected 1 target (deduplicated), got %d", len(targets))
	}
	if targets[0].label != "local" {
		t.Errorf("expected label 'local', got %q", targets[0].label)
	}
}

func TestAllEngineTargets_MultipleRemoteEngines(t *testing.T) {
	api := &API{}
	api.engineRegistry = store.NewEngineRegistry()

	// No local engine, two remote engines
	api.engineRegistry.Register(&store.Engine{
		ID:     "engine-1",
		Name:   "engine-1",
		Host:   "host1",
		Port:   4281,
		Status: store.EngineStatusOnline,
	})
	api.engineRegistry.Register(&store.Engine{
		ID:     "engine-2",
		Name:   "engine-2",
		Host:   "host2",
		Port:   4282,
		Status: store.EngineStatusOnline,
	})

	targets := api.allEngineTargets()
	if len(targets) != 2 {
		t.Fatalf("expected 2 targets (two remotes, no local), got %d", len(targets))
	}

	for _, target := range targets {
		if target.label == "local" {
			t.Error("should not have local target when localEngine is nil")
		}
	}
}
