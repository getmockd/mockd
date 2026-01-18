package unit

import (
	"encoding/json"
	"testing"

	"github.com/getmockd/mockd/pkg/mcp"
)

func TestJSONRPCRequest_Parse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(*mcp.JSONRPCRequest) error
	}{
		{
			name:    "valid request with id",
			input:   `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
			wantErr: false,
			check: func(r *mcp.JSONRPCRequest) error {
				if r.Method != "initialize" {
					t.Errorf("expected method 'initialize', got %s", r.Method)
				}
				return nil
			},
		},
		{
			name:    "valid notification (no id)",
			input:   `{"jsonrpc":"2.0","method":"initialized"}`,
			wantErr: false,
			check: func(r *mcp.JSONRPCRequest) error {
				if !r.IsNotification() {
					t.Error("expected notification (no id)")
				}
				return nil
			},
		},
		{
			name:    "string id",
			input:   `{"jsonrpc":"2.0","id":"abc-123","method":"test"}`,
			wantErr: false,
			check: func(r *mcp.JSONRPCRequest) error {
				if r.ID != "abc-123" {
					t.Errorf("expected id 'abc-123', got %v", r.ID)
				}
				return nil
			},
		},
		{
			name:    "numeric id",
			input:   `{"jsonrpc":"2.0","id":42,"method":"test"}`,
			wantErr: false,
			check: func(r *mcp.JSONRPCRequest) error {
				// JSON numbers unmarshal as float64
				if id, ok := r.ID.(float64); !ok || id != 42 {
					t.Errorf("expected id 42, got %v", r.ID)
				}
				return nil
			},
		},
		{
			name:    "with params object",
			input:   `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"test"}}`,
			wantErr: false,
			check: func(r *mcp.JSONRPCRequest) error {
				if r.Params == nil {
					t.Error("expected params")
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req mcp.JSONRPCRequest
			err := json.Unmarshal([]byte(tt.input), &req)
			if (err != nil) != tt.wantErr {
				t.Errorf("Unmarshal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil {
				tt.check(&req)
			}
		})
	}
}

func TestJSONRPCResponse_Marshal(t *testing.T) {
	tests := []struct {
		name string
		resp mcp.JSONRPCResponse
		want string
	}{
		{
			name: "success response",
			resp: mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      1,
				Result:  map[string]string{"status": "ok"},
			},
			want: `{"jsonrpc":"2.0","id":1,"result":{"status":"ok"}}`,
		},
		{
			name: "error response",
			resp: mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      1,
				Error: &mcp.JSONRPCError{
					Code:    -32600,
					Message: "Invalid request",
				},
			},
			want: `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"Invalid request"}}`,
		},
		{
			name: "error with data",
			resp: mcp.JSONRPCResponse{
				JSONRPC: "2.0",
				ID:      1,
				Error: &mcp.JSONRPCError{
					Code:    -32001,
					Message: "Mock not found",
					Data:    map[string]string{"path": "/api/test"},
				},
			},
			want: `{"jsonrpc":"2.0","id":1,"error":{"code":-32001,"message":"Mock not found","data":{"path":"/api/test"}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.resp)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}

			// Compare as JSON objects
			var gotObj, wantObj map[string]interface{}
			json.Unmarshal(got, &gotObj)
			json.Unmarshal([]byte(tt.want), &wantObj)

			gotJSON, _ := json.Marshal(gotObj)
			wantJSON, _ := json.Marshal(wantObj)

			if string(gotJSON) != string(wantJSON) {
				t.Errorf("Marshal() = %s, want %s", gotJSON, wantJSON)
			}
		})
	}
}

func TestJSONRPCRequest_IsNotification(t *testing.T) {
	tests := []struct {
		name   string
		req    mcp.JSONRPCRequest
		isNote bool
	}{
		{
			name:   "request with numeric id",
			req:    mcp.JSONRPCRequest{ID: 1},
			isNote: false,
		},
		{
			name:   "request with string id",
			req:    mcp.JSONRPCRequest{ID: "abc"},
			isNote: false,
		},
		{
			name:   "notification (nil id)",
			req:    mcp.JSONRPCRequest{ID: nil},
			isNote: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.req.IsNotification(); got != tt.isNote {
				t.Errorf("IsNotification() = %v, want %v", got, tt.isNote)
			}
		})
	}
}

func TestMCPConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     mcp.Config
		wantErr bool
	}{
		{
			name:    "default config is valid",
			cfg:     *mcp.DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid port (too low)",
			cfg: mcp.Config{
				Port:           0,
				Path:           "/mcp",
				MaxSessions:    100,
				SessionTimeout: 60,
			},
			wantErr: true,
		},
		{
			name: "invalid port (too high)",
			cfg: mcp.Config{
				Port:           70000,
				Path:           "/mcp",
				MaxSessions:    100,
				SessionTimeout: 60,
			},
			wantErr: true,
		},
		{
			name: "empty path",
			cfg: mcp.Config{
				Port:           9091,
				Path:           "",
				MaxSessions:    100,
				SessionTimeout: 60,
			},
			wantErr: true,
		},
		{
			name: "path without leading slash",
			cfg: mcp.Config{
				Port:           9091,
				Path:           "mcp",
				MaxSessions:    100,
				SessionTimeout: 60,
			},
			wantErr: true,
		},
		{
			name: "zero max sessions",
			cfg: mcp.Config{
				Port:           9091,
				Path:           "/mcp",
				MaxSessions:    0,
				SessionTimeout: 60,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMCPConfig_Address(t *testing.T) {
	tests := []struct {
		name        string
		cfg         mcp.Config
		wantAddress string
	}{
		{
			name: "localhost only",
			cfg: mcp.Config{
				Port:        9091,
				AllowRemote: false,
			},
			wantAddress: "127.0.0.1:9091",
		},
		{
			name: "allow remote",
			cfg: mcp.Config{
				Port:        9091,
				AllowRemote: true,
			},
			wantAddress: ":9091",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.cfg.Address(); got != tt.wantAddress {
				t.Errorf("Address() = %v, want %v", got, tt.wantAddress)
			}
		})
	}
}
