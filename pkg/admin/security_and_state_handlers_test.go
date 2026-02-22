package admin

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
)

func TestFolderUpdate_RejectsCrossWorkspaceParent(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	folderStore := api.getFolderStore()
	if folderStore == nil {
		t.Fatal("folder store is nil")
	}

	ctx := context.Background()
	rootWS1 := &config.Folder{ID: "ws1-root", Name: "WS1", EntityMeta: config.EntityMeta{WorkspaceID: "ws1"}}
	rootWS2 := &config.Folder{ID: "ws2-root", Name: "WS2", EntityMeta: config.EntityMeta{WorkspaceID: "ws2"}}
	childWS1 := &config.Folder{ID: "ws1-child", Name: "Child", EntityMeta: config.EntityMeta{WorkspaceID: "ws1"}}

	if err := folderStore.Create(ctx, rootWS1); err != nil {
		t.Fatalf("create rootWS1: %v", err)
	}
	if err := folderStore.Create(ctx, rootWS2); err != nil {
		t.Fatalf("create rootWS2: %v", err)
	}
	if err := folderStore.Create(ctx, childWS1); err != nil {
		t.Fatalf("create childWS1: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/folders/ws1-child", strings.NewReader(`{"parentId":"ws2-root"}`))
	req.SetPathValue("id", "ws1-child")
	rec := httptest.NewRecorder()

	api.handleUpdateFolder(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	updated, err := folderStore.Get(ctx, "ws1-child")
	if err != nil {
		t.Fatalf("get updated folder: %v", err)
	}
	if updated.ParentID != "" {
		t.Fatalf("expected parent unchanged, got %q", updated.ParentID)
	}
}

func TestAPIKeyMiddleware_DoesNotAcceptQueryParam(t *testing.T) {
	api := NewAPI(0, WithAPIKey("mk_test_key"))
	defer api.Stop()

	req := httptest.NewRequest(http.MethodGet, "/status?api_key=mk_test_key", nil)
	req.RemoteAddr = "10.0.0.2:12345"
	rec := httptest.NewRecorder()

	api.httpServer.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestMapCreateStatefulItemError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{
			name:       "not found",
			err:        engineclient.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
			wantMsg:    "Resource not found",
		},
		{
			name:       "conflict",
			err:        engineclient.ErrConflict,
			wantStatus: http.StatusConflict,
			wantCode:   "conflict",
			wantMsg:    "Item already exists",
		},
		{
			name:       "capacity",
			err:        engineclient.ErrCapacity,
			wantStatus: http.StatusInsufficientStorage,
			wantCode:   "capacity_exceeded",
			wantMsg:    "Resource capacity exceeded",
		},
		{
			name:       "generic",
			err:        errors.New("bad payload"),
			wantStatus: http.StatusBadRequest,
			wantCode:   "create_failed",
			wantMsg:    ErrMsgOperationFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, msg := mapCreateStatefulItemError(tt.err, nil)
			if status != tt.wantStatus {
				t.Fatalf("status=%d want=%d", status, tt.wantStatus)
			}
			if code != tt.wantCode {
				t.Fatalf("code=%q want=%q", code, tt.wantCode)
			}
			if msg != tt.wantMsg {
				t.Fatalf("msg=%q want=%q", msg, tt.wantMsg)
			}
		})
	}
}

func TestMapStatefulResourceError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{
			name:       "not found",
			err:        engineclient.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
			wantMsg:    "Resource not found",
		},
		{
			name:       "engine error",
			err:        errors.New("dial tcp 127.0.0.1:9999: connect: connection refused"),
			wantStatus: http.StatusServiceUnavailable,
			wantCode:   "engine_error",
			wantMsg:    ErrMsgEngineUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, msg := mapStatefulResourceError(tt.err, nil, "Resource not found", "get state resource")
			if status != tt.wantStatus {
				t.Fatalf("status=%d want=%d", status, tt.wantStatus)
			}
			if code != tt.wantCode {
				t.Fatalf("code=%q want=%q", code, tt.wantCode)
			}
			if msg != tt.wantMsg {
				t.Fatalf("msg=%q want=%q", msg, tt.wantMsg)
			}
		})
	}
}
