package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleCreateWorkspace_DefaultPathUsesAdminDataDir(t *testing.T) {
	dataDir := t.TempDir()
	api := NewAPI(0, WithDataDir(dataDir))
	defer api.Stop()

	req := httptest.NewRequest(http.MethodPost, "/workspaces", bytes.NewBufferString(`{"name":"ws-test"}`))
	rec := httptest.NewRecorder()

	api.handleCreateWorkspace(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	var ws WorkspaceDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &ws); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	wantPrefix := filepath.Join(dataDir, "workspaces") + string(filepath.Separator)
	if !strings.HasPrefix(ws.Path, wantPrefix) {
		t.Fatalf("workspace path=%q, want prefix %q", ws.Path, wantPrefix)
	}
}

