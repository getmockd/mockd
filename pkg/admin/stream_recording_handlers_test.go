package admin

import (
	"testing"

	"github.com/getmockd/mockd/pkg/recording"
)

func TestStreamAddEndpointPath(t *testing.T) {
	if got := streamAddEndpointPath(recording.ProtocolSSE, ""); got != "/sse/recorded" {
		t.Fatalf("expected default SSE path, got %q", got)
	}
	if got := streamAddEndpointPath(recording.ProtocolSSE, "/events"); got != "/events" {
		t.Fatalf("expected explicit SSE path, got %q", got)
	}
	if got := streamAddEndpointPath(recording.ProtocolWebSocket, "/ws"); got != "/ws" {
		t.Fatalf("expected websocket path unchanged, got %q", got)
	}
}
