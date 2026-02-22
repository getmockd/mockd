package admin

import (
	"errors"
	"testing"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
)

func TestMapCreateMockAddError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "duplicate",
			err:        engineclient.ErrDuplicate,
			wantStatus: 409,
			wantCode:   "conflict",
		},
		{
			name:       "port error",
			err:        errors.New("bind: address already in use"),
			wantStatus: 409,
			wantCode:   "port_unavailable",
		},
		{
			name:       "validation error",
			err:        errors.New("validation failed: path is required"),
			wantStatus: 400,
			wantCode:   "validation_error",
		},
		{
			name:       "generic engine error",
			err:        errors.New("connection refused"),
			wantStatus: 503,
			wantCode:   "engine_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, _ := mapCreateMockAddError(tt.err, nil, "test")
			if status != tt.wantStatus || code != tt.wantCode {
				t.Fatalf("mapCreateMockAddError(%v) = (%d, %q), want (%d, %q)", tt.err, status, code, tt.wantStatus, tt.wantCode)
			}
		})
	}
}

func TestMapStreamAddError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "missing endpoint path",
			err:        errors.New("endpointPath is required for WebSocket mocks"),
			wantStatus: 400,
			wantCode:   "validation_error",
		},
		{
			name:       "unexpected config type",
			err:        errors.New("unexpected config type for WebSocket: *mock.SSEConfig"),
			wantStatus: 500,
			wantCode:   "add_error",
		},
		{
			name:       "engine duplicate",
			err:        engineclient.ErrDuplicate,
			wantStatus: 409,
			wantCode:   "conflict",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, code, _ := mapStreamAddError(tt.err, nil)
			if status != tt.wantStatus || code != tt.wantCode {
				t.Fatalf("mapStreamAddError(%v) = (%d, %q), want (%d, %q)", tt.err, status, code, tt.wantStatus, tt.wantCode)
			}
		})
	}
}
