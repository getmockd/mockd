package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/recording"
)

func TestHandleListSOAPRecordings_HasFaultBoolParsing(t *testing.T) {
	mgr := NewSOAPRecordingManager()

	normal := recording.NewSOAPRecording("/soap", "GetUser", "1.1")
	if err := mgr.store.Add(normal); err != nil {
		t.Fatalf("add normal recording: %v", err)
	}

	fault := recording.NewSOAPRecording("/soap", "CreateUser", "1.1")
	fault.SetFault("Server", "boom")
	if err := mgr.store.Add(fault); err != nil {
		t.Fatalf("add fault recording: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/soap-recordings?hasFault=1", nil)
	rec := httptest.NewRecorder()

	mgr.handleListSOAPRecordings(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp SOAPRecordingListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Total != 1 {
		t.Fatalf("expected total=1 fault recording, got %d", resp.Total)
	}
	if len(resp.Recordings) != 1 || !resp.Recordings[0].HasFault {
		t.Fatalf("expected exactly one fault recording in response")
	}
}
