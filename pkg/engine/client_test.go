package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/store"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestBuildMocksURL_EncodesWorkspaceID(t *testing.T) {
	got, err := buildMocksURL("http://admin.example", "ws a&b=1")
	if err != nil {
		t.Fatalf("buildMocksURL returned error: %v", err)
	}

	if !strings.Contains(got, "/mocks?") {
		t.Fatalf("expected /mocks query URL, got %q", got)
	}
	if !strings.Contains(got, "workspaceId=ws+a%26b%3D1") {
		t.Fatalf("workspaceId not URL-encoded correctly: %q", got)
	}
}

func TestSyncWorkspaces_RemovesUnassignedWorkspace(t *testing.T) {
	t.Parallel()

	manager := NewWorkspaceManager(nil)
	manager.workspaces["ws-stale"] = &WorkspaceServer{
		WorkspaceID:   "ws-stale",
		WorkspaceName: "Stale Workspace",
		status:        WorkspaceServerStatusStopped,
		manager:       manager,
	}

	client := NewEngineClient(&EngineClientConfig{
		AdminURL:     "http://admin.example",
		EngineName:   "test-engine",
		LocalPort:    19000,
		PollInterval: time.Second,
	}, manager)
	client.httpClient = &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet || req.URL.String() != "http://admin.example/engines/eng-1" {
				return &http.Response{
					StatusCode: http.StatusNotFound,
					Body:       io.NopCloser(strings.NewReader("not found")),
					Header:     make(http.Header),
				}, nil
			}
			body, _ := json.Marshal(store.Engine{
				ID:         "eng-1",
				Workspaces: []store.EngineWorkspace{},
			})
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(body)),
				Header:     make(http.Header),
			}, nil
		}),
	}
	client.mu.Lock()
	client.engineID = "eng-1"
	client.mu.Unlock()

	if err := client.syncWorkspaces(context.Background()); err != nil {
		t.Fatalf("syncWorkspaces returned error: %v", err)
	}

	if got := manager.GetWorkspace("ws-stale"); got != nil {
		t.Fatalf("expected stale workspace to be removed, found %+v", got.StatusInfo())
	}
}
