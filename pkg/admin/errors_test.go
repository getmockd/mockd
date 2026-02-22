package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONDecodeError_MaxBytes(t *testing.T) {
	rec := httptest.NewRecorder()

	writeJSONDecodeError(rec, &http.MaxBytesError{Limit: 16}, nil)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected status 413, got %d", rec.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Error != "body_too_large" {
		t.Fatalf("expected body_too_large error code, got %q", resp.Error)
	}
}
