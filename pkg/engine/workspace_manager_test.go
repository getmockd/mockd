package engine

import (
	"context"
	"testing"

	"github.com/getmockd/mockd/pkg/store"
)

func TestStartWorkspace_ValidatesInput(t *testing.T) {
	t.Parallel()

	mgr := NewWorkspaceManager(nil)

	tests := []struct {
		name string
		ws   *store.EngineWorkspace
	}{
		{
			name: "nil workspace",
			ws:   nil,
		},
		{
			name: "missing workspace ID",
			ws: &store.EngineWorkspace{
				WorkspaceID: "   ",
				HTTPPort:    9000,
			},
		},
		{
			name: "invalid zero port",
			ws: &store.EngineWorkspace{
				WorkspaceID: "ws-1",
				HTTPPort:    0,
			},
		},
		{
			name: "invalid high port",
			ws: &store.EngineWorkspace{
				WorkspaceID: "ws-1",
				HTTPPort:    70000,
			},
		},
		{
			name: "invalid gRPC port",
			ws: &store.EngineWorkspace{
				WorkspaceID: "ws-1",
				HTTPPort:    9000,
				GRPCPort:    -1,
			},
		},
		{
			name: "invalid MQTT port",
			ws: &store.EngineWorkspace{
				WorkspaceID: "ws-1",
				HTTPPort:    9000,
				MQTTPort:    70000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if err := mgr.StartWorkspace(context.Background(), tt.ws); err == nil {
				t.Fatalf("expected validation error, got nil")
			}
		})
	}
}
