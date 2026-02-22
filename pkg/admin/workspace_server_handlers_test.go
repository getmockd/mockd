package admin

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/internal/storage"
	"github.com/getmockd/mockd/pkg/store"
	"github.com/getmockd/mockd/pkg/workspace"
)

type stubWorkspaceServer struct {
	status workspace.ServerStatus
}

func (s *stubWorkspaceServer) Status() workspace.ServerStatus { return s.status }
func (s *stubWorkspaceServer) StatusInfo() *workspace.StatusInfo {
	return &workspace.StatusInfo{Status: s.status}
}

type stubWorkspaceManager struct {
	server workspace.Server
}

func (m *stubWorkspaceManager) SetLogger(_ *slog.Logger)               {}
func (m *stubWorkspaceManager) SetMockFetcher(_ workspace.MockFetcher) {}
func (m *stubWorkspaceManager) SetCentralStore(_ storage.MockStore)    {}
func (m *stubWorkspaceManager) StartWorkspace(_ context.Context, _ *store.EngineWorkspace) error {
	return nil
}
func (m *stubWorkspaceManager) StopWorkspace(_ string) error                      { return nil }
func (m *stubWorkspaceManager) RemoveWorkspace(_ string) error                    { return nil }
func (m *stubWorkspaceManager) ReloadWorkspace(_ context.Context, _ string) error { return nil }
func (m *stubWorkspaceManager) StopAll() error                                    { return nil }
func (m *stubWorkspaceManager) GetWorkspace(_ string) workspace.Server            { return m.server }
func (m *stubWorkspaceManager) ListWorkspaces() []workspace.Server                { return nil }
func (m *stubWorkspaceManager) GetWorkspaceStatus(_ string) *workspace.StatusInfo { return nil }

func TestHandleStopWorkspaceServer_NotRunningReturns400(t *testing.T) {
	api := NewAPI(0)
	defer api.Stop()

	engine := &store.Engine{ID: "eng-1", Workspaces: []store.EngineWorkspace{{WorkspaceID: "ws-1"}}}
	if err := api.engineRegistry.Register(engine); err != nil {
		t.Fatalf("register engine: %v", err)
	}
	api.workspaceManager = &stubWorkspaceManager{server: nil}

	req := httptest.NewRequest(http.MethodPost, "/engines/eng-1/workspaces/ws-1/stop", nil)
	req.SetPathValue("id", "eng-1")
	req.SetPathValue("workspaceId", "ws-1")
	rec := httptest.NewRecorder()

	api.handleStopWorkspaceServer(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleStartWorkspaceServer_AlreadyRunningReturns409(t *testing.T) {
	api := NewAPI(0)
	defer api.Stop()

	engine := &store.Engine{ID: "eng-1", Workspaces: []store.EngineWorkspace{{WorkspaceID: "ws-1", HTTPPort: 8081}}}
	if err := api.engineRegistry.Register(engine); err != nil {
		t.Fatalf("register engine: %v", err)
	}
	api.workspaceManager = &stubWorkspaceManager{server: &stubWorkspaceServer{status: workspace.ServerStatusRunning}}

	req := httptest.NewRequest(http.MethodPost, "/engines/eng-1/workspaces/ws-1/start", nil)
	req.SetPathValue("id", "eng-1")
	req.SetPathValue("workspaceId", "ws-1")
	rec := httptest.NewRecorder()

	api.handleStartWorkspaceServer(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleStopWorkspaceServer_StartingIsStoppable(t *testing.T) {
	api := NewAPI(0)
	defer api.Stop()

	engine := &store.Engine{ID: "eng-1", Workspaces: []store.EngineWorkspace{{WorkspaceID: "ws-1"}}}
	if err := api.engineRegistry.Register(engine); err != nil {
		t.Fatalf("register engine: %v", err)
	}
	api.workspaceManager = &stubWorkspaceManager{server: &stubWorkspaceServer{status: workspace.ServerStatusStarting}}

	req := httptest.NewRequest(http.MethodPost, "/engines/eng-1/workspaces/ws-1/stop", nil)
	req.SetPathValue("id", "eng-1")
	req.SetPathValue("workspaceId", "ws-1")
	rec := httptest.NewRecorder()

	api.handleStopWorkspaceServer(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d body=%s", rec.Code, rec.Body.String())
	}
}
