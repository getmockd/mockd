package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleListFolders_FilteredEmptyReturnsArray(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	req := httptest.NewRequest(http.MethodGet, "/folders?workspaceId=ws-empty", nil)
	rec := httptest.NewRecorder()

	api.handleListFolders(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if bytes.Equal(bytes.TrimSpace(rec.Body.Bytes()), []byte("null")) {
		t.Fatalf("expected JSON array, got null")
	}

	var folders []any
	if err := json.Unmarshal(rec.Body.Bytes(), &folders); err != nil {
		t.Fatalf("failed to decode folders list: %v", err)
	}
	if len(folders) != 0 {
		t.Fatalf("expected 0 folders, got %d", len(folders))
	}
}

func TestHandleCreateFolder_MaxBytesReturns413(t *testing.T) {
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	body := bytes.NewBufferString(`{"name":"too-big"}`)
	req := httptest.NewRequest(http.MethodPost, "/folders", body)
	rec := httptest.NewRecorder()
	req.Body = http.MaxBytesReader(rec, req.Body, 1)

	api.handleCreateFolder(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if resp.Error != "body_too_large" {
		t.Fatalf("expected body_too_large, got %q", resp.Error)
	}
}
