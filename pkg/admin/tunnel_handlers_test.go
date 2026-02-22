package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/store"
)

func TestHandleListTunnels_EmptyReturnsArray(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	req := httptest.NewRequest(http.MethodGet, "/tunnels", nil)
	rec := httptest.NewRecorder()

	api.handleListTunnels(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(`"tunnels":null`)) {
		t.Fatalf("expected tunnels array, got null: %s", rec.Body.String())
	}
}

func TestPreviewExposedMocks_EmptyReturnsArray(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	req := httptest.NewRequest(http.MethodGet, "/tunnels", nil)
	result := api.previewExposedMocks(req, store.TunnelExposure{Mode: "all"})
	if result == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 mocks, got %d", len(result))
	}
}

func TestPreviewExposedMocks_NormalizesDefaultWorkspace(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	if err := api.dataStore.Mocks().Create(context.Background(), &config.MockConfiguration{
		ID:   "m1",
		Name: "Mock One",
		Type: mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Path: "/x"}},
	}); err != nil {
		t.Fatalf("create mock: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/tunnels", nil)
	result := api.previewExposedMocks(req, store.TunnelExposure{
		Mode:       "selected",
		Workspaces: []string{store.DefaultWorkspaceID},
	})
	if len(result) != 1 {
		b, _ := json.Marshal(result)
		t.Fatalf("expected 1 mock, got %d (%s)", len(result), string(b))
	}
	if result[0].Workspace != store.DefaultWorkspaceID {
		t.Fatalf("expected workspace %q, got %q", store.DefaultWorkspaceID, result[0].Workspace)
	}
}

func TestPreviewExposedMocks_ExcludeDefaultWorkspaceNormalizes(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	if err := api.dataStore.Mocks().Create(context.Background(), &config.MockConfiguration{
		ID:   "m1",
		Name: "Mock One",
		Type: mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Path: "/x"}},
	}); err != nil {
		t.Fatalf("create mock: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/tunnels", nil)
	result := api.previewExposedMocks(req, store.TunnelExposure{
		Mode: "all",
		Exclude: &store.TunnelExclude{
			Workspaces: []string{store.DefaultWorkspaceID},
		},
	})
	if len(result) != 0 {
		b, _ := json.Marshal(result)
		t.Fatalf("expected 0 mocks after default-workspace exclusion, got %d (%s)", len(result), string(b))
	}
}

func TestPreviewExposedMocks_MatchesWorkspaceByName(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	if err := api.dataStore.Mocks().Create(context.Background(), &config.MockConfiguration{
		ID:   "m1",
		Name: "Mock One",
		Type: mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Path: "/x"}},
	}); err != nil {
		t.Fatalf("create mock: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/tunnels", nil)
	result := api.previewExposedMocks(req, store.TunnelExposure{
		Mode:       "selected",
		Workspaces: []string{"Default"},
	})
	if len(result) != 1 {
		b, _ := json.Marshal(result)
		t.Fatalf("expected workspace-name selector to match default workspace mock; got %d (%s)", len(result), string(b))
	}
}

func TestPreviewExposedMocks_ExcludesWorkspaceByName(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	if err := api.dataStore.Mocks().Create(context.Background(), &config.MockConfiguration{
		ID:   "m1",
		Name: "Mock One",
		Type: mock.TypeHTTP,
		HTTP: &mock.HTTPSpec{Matcher: &mock.HTTPMatcher{Path: "/x"}},
	}); err != nil {
		t.Fatalf("create mock: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/tunnels", nil)
	result := api.previewExposedMocks(req, store.TunnelExposure{
		Mode: "all",
		Exclude: &store.TunnelExclude{
			Workspaces: []string{"Default"},
		},
	})
	if len(result) != 0 {
		b, _ := json.Marshal(result)
		t.Fatalf("expected workspace-name exclusion to remove default workspace mock; got %d (%s)", len(result), string(b))
	}
}

func TestHandleTunnelPreview_InvalidModeReturns400(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	if err := api.engineRegistry.Register(&store.Engine{ID: "eng-1", Name: "E"}); err != nil {
		t.Fatalf("register engine: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/engines/eng-1/tunnel/preview", bytes.NewBufferString(`{"expose":{"mode":"bogus"}}`))
	req.SetPathValue("id", "eng-1")
	rec := httptest.NewRecorder()

	api.handleTunnelPreview(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"invalid_mode"`)) {
		t.Fatalf("expected invalid_mode error, got %s", rec.Body.String())
	}
}

func TestHandleUpdateTunnelConfig_InvalidModeReturns400(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	api.setLocalTunnelConfig(&store.TunnelConfig{
		Enabled: true,
		Expose:  store.TunnelExposure{Mode: "all"},
	})

	req := httptest.NewRequest(http.MethodPut, "/engines/local/tunnel/config", bytes.NewBufferString(`{"expose":{"mode":"bogus"}}`))
	req.SetPathValue("id", LocalEngineID)
	rec := httptest.NewRecorder()

	api.handleUpdateTunnelConfig(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"invalid_mode"`)) {
		t.Fatalf("expected invalid_mode error, got %s", rec.Body.String())
	}
}

func TestHandleUpdateTunnelConfig_ClearCustomDomainRecomputesPublicURL(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	api.setLocalTunnelConfig(&store.TunnelConfig{
		Enabled:      true,
		Subdomain:    "abc123",
		CustomDomain: "example.com",
		PublicURL:    "https://example.com",
		Expose:       store.TunnelExposure{Mode: "all"},
	})

	req := httptest.NewRequest(http.MethodPut, "/engines/local/tunnel/config", bytes.NewBufferString(`{"customDomain":""}`))
	req.SetPathValue("id", LocalEngineID)
	rec := httptest.NewRecorder()

	api.handleUpdateTunnelConfig(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp store.TunnelConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.CustomDomain != "" {
		t.Fatalf("expected customDomain cleared, got %q", resp.CustomDomain)
	}
	if resp.PublicURL != "https://abc123.tunnel.mockd.io" {
		t.Fatalf("expected publicURL to fall back to subdomain, got %q", resp.PublicURL)
	}
}

func TestHandleUpdateTunnelConfig_RejectsEmptySubdomainWithoutCustomDomain(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	api.setLocalTunnelConfig(&store.TunnelConfig{
		Enabled:   true,
		Subdomain: "abc123",
		PublicURL: "https://abc123.tunnel.mockd.io",
		Expose:    store.TunnelExposure{Mode: "all"},
	})

	req := httptest.NewRequest(http.MethodPut, "/engines/local/tunnel/config", bytes.NewBufferString(`{"subdomain":""}`))
	req.SetPathValue("id", LocalEngineID)
	rec := httptest.NewRecorder()

	api.handleUpdateTunnelConfig(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"invalid_subdomain"`)) {
		t.Fatalf("expected invalid_subdomain error, got %s", rec.Body.String())
	}
}
