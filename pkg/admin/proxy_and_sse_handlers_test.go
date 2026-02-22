package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

func TestHandleGenerateCA_InvalidJSONReturns400(t *testing.T) {
	pm := NewProxyManager()

	req := httptest.NewRequest(http.MethodPost, "/proxy/ca", strings.NewReader(`{"caPath":`))
	rec := httptest.NewRecorder()

	pm.handleGenerateCA(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleGenerateCA_EmptyBodyStillReturnsMissingPath(t *testing.T) {
	pm := NewProxyManager()

	req := httptest.NewRequest(http.MethodPost, "/proxy/ca", strings.NewReader(""))
	rec := httptest.NewRecorder()

	pm.handleGenerateCA(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "missing_path") {
		t.Fatalf("expected missing_path error, got %s", rec.Body.String())
	}
}

func TestHandleCloseSSEConnection_InvalidJSONReturns400(t *testing.T) {
	api := NewAPI(0, WithLocalEngineClient(engineclient.New("http://localhost:99999")))
	defer api.Stop()

	req := httptest.NewRequest(http.MethodDelete, "/sse/connections/conn-1", strings.NewReader(`{"graceful":`))
	req.SetPathValue("id", "conn-1")
	rec := httptest.NewRecorder()

	api.handleCloseSSEConnection(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHandleCloseSSEConnection_EmptyBodyDoesNotDecodeFail(t *testing.T) {
	api := NewAPI(0, WithLocalEngineClient(engineclient.New("http://127.0.0.1:1")))
	defer api.Stop()

	req := httptest.NewRequest(http.MethodDelete, "/sse/connections/conn-1", strings.NewReader(""))
	req.SetPathValue("id", "conn-1")
	rec := httptest.NewRecorder()

	api.handleCloseSSEConnection(rec, req)

	if rec.Code == http.StatusBadRequest {
		t.Fatalf("expected non-400 (empty body should be allowed), got %d body=%s", rec.Code, rec.Body.String())
	}
}
