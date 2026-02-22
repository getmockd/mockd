package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

func TestHandleGenerateFromTemplate_InvalidJSONReturns400(t *testing.T) {
	api := NewAPI(0)
	defer api.Stop()

	req := httptest.NewRequest(http.MethodPost, "/templates/blank", strings.NewReader(`{"parameters":`))
	req.SetPathValue("name", "blank")
	rec := httptest.NewRecorder()

	api.handleGenerateFromTemplate(rec, req, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleGenerateFromTemplate_ImportFailureMapped503(t *testing.T) {
	api := NewAPI(0)
	defer api.Stop()

	req := httptest.NewRequest(http.MethodPost, "/templates/blank", strings.NewReader(`{}`))
	req.SetPathValue("name", "blank")
	rec := httptest.NewRecorder()

	// Invalid port causes request creation to fail before any network access.
	engine := engineclient.New("http://localhost:99999")
	api.handleGenerateFromTemplate(rec, req, engine)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "engine_error") {
		t.Fatalf("expected engine_error response, got %s", rec.Body.String())
	}
}
