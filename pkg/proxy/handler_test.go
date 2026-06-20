package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// TestHandleHTTPRejectsNonProxyRequest verifies that a request sent directly to
// the proxy (origin-form, i.e. not via a proxy client) is rejected promptly with
// a 4xx status instead of being forwarded back to the proxy's own listen address.
//
// Regression test for issue #35: a plain request like `curl http://localhost:8888`
// (no -x/--proxy) arrives in origin-form with an empty r.URL.Host. The old code
// forwarded it to r.Host (the proxy itself), creating a self-referential loop that
// hung until the 30s client timeout and returned 502.
func TestHandleHTTPRejectsNonProxyRequest(t *testing.T) {
	p := New(Options{Mode: ModePassthrough})

	// Origin-form request: no absolute URL, Host points at the proxy itself.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Host = "localhost:8888"
	req.URL.Host = "" // ensure origin-form (httptest sets this from the path already)

	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		p.handleHTTP(rec, req)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handleHTTP did not return promptly for a non-proxy request (self-loop / hang)")
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected %d for non-proxy request, got %d", http.StatusBadRequest, rec.Code)
	}
}

// TestHandleHTTPForwardsProxyRequest is a sanity check that a properly proxied
// (absolute-form) request is still forwarded and recorded.
func TestHandleHTTPForwardsProxyRequest(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer target.Close()

	store := recording.NewStore()
	store.CreateSession("t", nil)
	p := New(Options{Mode: ModeRecord, Store: store})

	// Absolute-form request as a proxy client would send.
	req := httptest.NewRequest(http.MethodGet, target.URL+"/api/test", nil)

	rec := httptest.NewRecorder()
	p.handleHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from proxied request, got %d", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}
