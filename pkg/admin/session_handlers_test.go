package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/recording"
)

func TestSessionFilters_RoundTripCreateAndGet(t *testing.T) {
	pm := &ProxyManager{store: recording.NewStore()}

	createReq := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(`{
		"name":"capture",
		"filters":{
			"includePaths":["/api/*"],
			"excludePaths":["/health"],
			"includeHosts":["example.com"],
			"excludeHosts":["internal.local"]
		}
	}`))
	createRec := httptest.NewRecorder()

	pm.handleCreateSession(createRec, createReq)

	if createRec.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d: %s", createRec.Code, createRec.Body.String())
	}

	var created SessionSummary
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected created session ID")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/sessions/"+created.ID, nil)
	getReq.SetPathValue("id", created.ID)
	getRec := httptest.NewRecorder()

	pm.handleGetSession(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	var resp SessionResponse
	if err := json.Unmarshal(getRec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}

	if resp.Filters == nil {
		t.Fatal("expected filters in session response")
	}
	if len(resp.Filters.IncludePaths) != 1 || resp.Filters.IncludePaths[0] != "/api/*" {
		t.Fatalf("unexpected includePaths: %#v", resp.Filters.IncludePaths)
	}
	if len(resp.Filters.ExcludePaths) != 1 || resp.Filters.ExcludePaths[0] != "/health" {
		t.Fatalf("unexpected excludePaths: %#v", resp.Filters.ExcludePaths)
	}
	if len(resp.Filters.IncludeHosts) != 1 || resp.Filters.IncludeHosts[0] != "example.com" {
		t.Fatalf("unexpected includeHosts: %#v", resp.Filters.IncludeHosts)
	}
	if len(resp.Filters.ExcludeHosts) != 1 || resp.Filters.ExcludeHosts[0] != "internal.local" {
		t.Fatalf("unexpected excludeHosts: %#v", resp.Filters.ExcludeHosts)
	}
}
