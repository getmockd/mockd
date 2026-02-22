package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/store"
)

func TestHandleAssignWorkspace_ValidatesWorkspaceID(t *testing.T) {
	api := NewAPI(0, WithAPIKeyDisabled())
	if err := api.engineRegistry.Register(&store.Engine{
		ID:   "eng-1",
		Name: "test-engine",
		Host: "localhost",
		Port: 4280,
	}); err != nil {
		t.Fatalf("register engine: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/engines/eng-1/workspace", bytes.NewBufferString(`{}`))
	req.SetPathValue("id", "eng-1")
	rec := httptest.NewRecorder()

	api.handleAssignWorkspace(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != "missing_workspace_id" {
		t.Fatalf("error = %q, want %q", resp.Error, "missing_workspace_id")
	}
}

