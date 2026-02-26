package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleGenerateFromTemplate_InvalidJSONReturns400(t *testing.T) {
	api := NewAPI(0)
	defer api.Stop()

	req := httptest.NewRequest(http.MethodPost, "/templates/blank", strings.NewReader(`{"parameters":`))
	req.SetPathValue("name", "blank")
	rec := httptest.NewRecorder()

	api.handleGenerateFromTemplate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleGenerateFromTemplate_NoEngineReturns503(t *testing.T) {
	// No engine configured — ImportConfigDirect should fail with "no engine connected"
	api := NewAPI(0, WithDataDir(t.TempDir()))
	defer api.Stop()

	req := httptest.NewRequest(http.MethodPost, "/templates/blank", strings.NewReader(`{}`))
	req.SetPathValue("name", "blank")
	rec := httptest.NewRecorder()

	api.handleGenerateFromTemplate(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "engine_error") {
		t.Fatalf("expected engine_error response, got %s", rec.Body.String())
	}
}

func TestHandleGenerateFromTemplate_DualWriteWithEngine(t *testing.T) {
	server := newMockEngineServer()
	defer server.Close()

	api := NewAPI(0,
		WithDataDir(t.TempDir()),
		WithLocalEngineClient(server.client()),
	)
	defer api.Stop()

	// Use "crud" template which generates actual mocks (not "blank" which produces zero).
	body := `{"parameters":{"resource":"products"}}`
	req := httptest.NewRequest(http.MethodPost, "/templates/crud", strings.NewReader(body))
	req.SetPathValue("name", "crud")
	rec := httptest.NewRecorder()

	api.handleGenerateFromTemplate(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Template generated successfully", resp["message"])
	assert.Equal(t, "crud", resp["template"])

	// Verify mocks were written to admin store.
	ctx := t.Context()
	storeMocks, err := api.dataStore.Mocks().List(ctx, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, storeMocks, "template mocks must be in admin store")

	// Verify mocks were pushed to engine.
	assert.NotEmpty(t, server.mocks, "template mocks must be in engine")
}

func TestHandleGenerateFromTemplate_ImportFailureMapped503(t *testing.T) {
	// Engine that is unreachable — ImportConfigDirect should fail.
	client := engineclient.New("http://localhost:99999")
	api := NewAPI(0,
		WithDataDir(t.TempDir()),
		WithLocalEngineClient(client),
	)
	defer api.Stop()

	req := httptest.NewRequest(http.MethodPost, "/templates/blank", strings.NewReader(`{}`))
	req.SetPathValue("name", "blank")
	rec := httptest.NewRecorder()

	api.handleGenerateFromTemplate(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "engine_error") {
		t.Fatalf("expected engine_error response, got %s", rec.Body.String())
	}
}
