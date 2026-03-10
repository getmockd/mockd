package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/websocket"
)

// ---------------------------------------------------------------------------
// formatDurationMs (stream_recordings.go)
// ---------------------------------------------------------------------------

func TestFormatDurationMs(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{name: "zero", ms: 0, want: "0ms"},
		{name: "sub-second", ms: 500, want: "500ms"},
		{name: "exactly_one_second", ms: 1000, want: "1.0s"},
		{name: "fractional_seconds", ms: 1500, want: "1.5s"},
		{name: "just_under_minute", ms: 59999, want: "60.0s"},
		{name: "exactly_one_minute", ms: 60000, want: "1m0s"},
		{name: "ninety_seconds", ms: 90000, want: "1m30s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDurationMs(tt.ms)
			if got != tt.want {
				t.Errorf("formatDurationMs(%d) = %q, want %q", tt.ms, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// formatBytes (stream_recordings.go)
// ---------------------------------------------------------------------------

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "zero", bytes: 0, want: "0 B"},
		{name: "small", bytes: 500, want: "500 B"},
		{name: "one_kb", bytes: 1024, want: "1.0 KB"},
		{name: "fractional_kb", bytes: 1536, want: "1.5 KB"},
		{name: "one_mb", bytes: 1048576, want: "1.0 MB"},
		{name: "large_mb", bytes: 5242880, want: "5.0 MB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBytes(tt.bytes)
			if got != tt.want {
				t.Errorf("formatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// truncatePath (stream_recordings.go)
// ---------------------------------------------------------------------------

func TestTruncatePath(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		maxLen int
		want   string
	}{
		{name: "short_enough", path: "/api/v1", maxLen: 20, want: "/api/v1"},
		{name: "exact_length", path: "/api", maxLen: 4, want: "/api"},
		{name: "truncated", path: "/very/long/api/path/here", maxLen: 15, want: "...pi/path/here"},
		{name: "minimal_max", path: "/a/b/c/d/e", maxLen: 5, want: ".../e"},
		{name: "empty_path", path: "", maxLen: 10, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncatePath(tt.path, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncatePath(%q, %d) = %q, want %q", tt.path, tt.maxLen, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// splitCSV (tunnel.go)
// ---------------------------------------------------------------------------

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want []string
	}{
		{name: "empty", s: "", want: nil},
		{name: "single", s: "alpha", want: []string{"alpha"}},
		{name: "multiple", s: "a,b,c", want: []string{"a", "b", "c"}},
		{name: "with_spaces", s: " a , b , c ", want: []string{"a", "b", "c"}},
		{name: "trailing_comma", s: "x,y,", want: []string{"x", "y", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitCSV(tt.s)
			if tt.want == nil {
				if got != nil {
					t.Errorf("splitCSV(%q) = %v, want nil", tt.s, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitCSV(%q) returned %d items, want %d", tt.s, len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("splitCSV(%q)[%d] = %q, want %q", tt.s, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// writeSessionMeta (proxy.go)
// ---------------------------------------------------------------------------

func TestWriteSessionMeta(t *testing.T) {
	tests := []struct {
		name string
		meta *SessionMeta
	}{
		{
			name: "basic_meta",
			meta: &SessionMeta{
				Name:      "test-session",
				StartTime: "2026-03-10T12:00:00Z",
				Port:      8888,
				Mode:      "record",
			},
		},
		{
			name: "meta_with_hosts",
			meta: &SessionMeta{
				Name:           "full-session",
				StartTime:      "2026-03-10T12:00:00Z",
				EndTime:        "2026-03-10T12:05:00Z",
				Port:           8888,
				Mode:           "record",
				RecordingCount: 42,
				Hosts:          []string{"api.example.com", "auth.example.com"},
			},
		},
		{
			name: "empty_name",
			meta: &SessionMeta{
				Name:      "",
				StartTime: "2026-03-10T12:00:00Z",
				Port:      0,
				Mode:      "passthrough",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := writeSessionMeta(dir, tt.meta); err != nil {
				t.Fatalf("writeSessionMeta() error = %v", err)
			}

			data, err := os.ReadFile(filepath.Join(dir, "meta.json"))
			if err != nil {
				t.Fatalf("failed to read meta.json: %v", err)
			}

			var got SessionMeta
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatalf("failed to unmarshal meta.json: %v", err)
			}

			if got.Name != tt.meta.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.meta.Name)
			}
			if got.Port != tt.meta.Port {
				t.Errorf("Port = %d, want %d", got.Port, tt.meta.Port)
			}
			if got.Mode != tt.meta.Mode {
				t.Errorf("Mode = %q, want %q", got.Mode, tt.meta.Mode)
			}
			if got.RecordingCount != tt.meta.RecordingCount {
				t.Errorf("RecordingCount = %d, want %d", got.RecordingCount, tt.meta.RecordingCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// discoverHosts (proxy.go)
// ---------------------------------------------------------------------------

func TestDiscoverHosts(t *testing.T) {
	t.Run("empty_dir", func(t *testing.T) {
		dir := t.TempDir()
		got := discoverHosts(dir)
		if len(got) != 0 {
			t.Errorf("discoverHosts(empty) = %v, want empty", got)
		}
	})

	t.Run("dirs_only", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "api.example.com"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(dir, "auth.example.com"), 0700); err != nil {
			t.Fatal(err)
		}
		got := discoverHosts(dir)
		if len(got) != 2 {
			t.Fatalf("discoverHosts() returned %d items, want 2", len(got))
		}
	})

	t.Run("mixed_files_and_dirs", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "host1"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "meta.json"), []byte("{}"), 0600); err != nil {
			t.Fatal(err)
		}
		got := discoverHosts(dir)
		if len(got) != 1 {
			t.Fatalf("discoverHosts() returned %d items, want 1", len(got))
		}
		if got[0] != "host1" {
			t.Errorf("discoverHosts()[0] = %q, want %q", got[0], "host1")
		}
	})

	t.Run("nonexistent_dir", func(t *testing.T) {
		got := discoverHosts("/nonexistent/path/that/does/not/exist")
		if got != nil {
			t.Errorf("discoverHosts(nonexistent) = %v, want nil", got)
		}
	})
}

// ---------------------------------------------------------------------------
// updateLatestSymlink (proxy.go)
// ---------------------------------------------------------------------------

func TestUpdateLatestSymlink(t *testing.T) {
	t.Run("creates_symlink", func(t *testing.T) {
		dir := t.TempDir()
		sessionName := "session-20260310"

		updateLatestSymlink(dir, sessionName)

		target, err := os.Readlink(filepath.Join(dir, "latest"))
		if err != nil {
			t.Fatalf("failed to read symlink: %v", err)
		}
		if target != sessionName {
			t.Errorf("symlink target = %q, want %q", target, sessionName)
		}
	})

	t.Run("updates_existing_symlink", func(t *testing.T) {
		dir := t.TempDir()

		updateLatestSymlink(dir, "session-old")
		updateLatestSymlink(dir, "session-new")

		target, err := os.Readlink(filepath.Join(dir, "latest"))
		if err != nil {
			t.Fatalf("failed to read symlink: %v", err)
		}
		if target != "session-new" {
			t.Errorf("symlink target = %q, want %q", target, "session-new")
		}
	})

	t.Run("replaces_regular_file", func(t *testing.T) {
		dir := t.TempDir()
		latestPath := filepath.Join(dir, "latest")

		// Create a regular file at "latest"
		if err := os.WriteFile(latestPath, []byte("old-session"), 0600); err != nil {
			t.Fatal(err)
		}

		updateLatestSymlink(dir, "session-replaced")

		target, err := os.Readlink(latestPath)
		if err != nil {
			t.Fatalf("failed to read symlink: %v", err)
		}
		if target != "session-replaced" {
			t.Errorf("symlink target = %q, want %q", target, "session-replaced")
		}
	})
}

// ---------------------------------------------------------------------------
// splitPatterns (proxy.go)
// ---------------------------------------------------------------------------

func TestSplitPatterns(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want []string
	}{
		{name: "empty", s: "", want: nil},
		{name: "single", s: "/api/*", want: []string{"/api/*"}},
		{name: "multiple", s: "/api/*,/health,/status", want: []string{"/api/*", "/health", "/status"}},
		{name: "no_trimming", s: " /api/* , /health ", want: []string{" /api/* ", " /health "}},
		{name: "trailing_comma", s: "a,b,", want: []string{"a", "b"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitPatterns(tt.s)
			if tt.want == nil {
				if got != nil {
					t.Errorf("splitPatterns(%q) = %v, want nil", tt.s, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitPatterns(%q) returned %d items, want %d", tt.s, len(got), len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("splitPatterns(%q)[%d] = %q, want %q", tt.s, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FormatConnectionError (client.go)
// ---------------------------------------------------------------------------

func TestFormatConnectionError(t *testing.T) {
	t.Run("api_connection_error", func(t *testing.T) {
		err := &APIError{
			StatusCode: 0,
			ErrorCode:  "connection_error",
			Message:    "cannot connect to admin API at http://localhost:4290",
		}
		got := FormatConnectionError(err)
		if got == "" {
			t.Fatal("FormatConnectionError returned empty string")
		}
		// Should contain the original message and suggestions
		if !containsSubstring(got, "cannot connect") {
			t.Errorf("expected 'cannot connect' in output, got %q", got)
		}
		if !containsSubstring(got, "mockd start") {
			t.Errorf("expected 'mockd start' suggestion in output, got %q", got)
		}
	})

	t.Run("non_api_error", func(t *testing.T) {
		err := fmt.Errorf("some random error")
		got := FormatConnectionError(err)
		if got != "some random error" {
			t.Errorf("FormatConnectionError(regular) = %q, want %q", got, "some random error")
		}
	})

	t.Run("api_error_non_connection", func(t *testing.T) {
		err := &APIError{
			StatusCode: 404,
			ErrorCode:  "not_found",
			Message:    "mock not found: abc",
		}
		got := FormatConnectionError(err)
		// Should fall through to err.Error() since it's not connection_error
		if got != "mock not found: abc" {
			t.Errorf("FormatConnectionError(not_found) = %q, want %q", got, "mock not found: abc")
		}
	})
}

// ---------------------------------------------------------------------------
// FormatNotFoundError (client.go)
// ---------------------------------------------------------------------------

func TestFormatNotFoundError(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		id           string
		wantContains []string
	}{
		{
			name:         "mock_not_found",
			resourceType: "mock",
			id:           "http_abc123",
			wantContains: []string{"mock", "http_abc123", "not found", "mockd list"},
		},
		{
			name:         "workspace_not_found",
			resourceType: "workspace",
			id:           "ws-001",
			wantContains: []string{"workspace", "ws-001", "not found"},
		},
		{
			name:         "empty_id",
			resourceType: "resource",
			id:           "",
			wantContains: []string{"resource", "not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatNotFoundError(tt.resourceType, tt.id)
			for _, want := range tt.wantContains {
				if !containsSubstring(got, want) {
					t.Errorf("FormatNotFoundError(%q, %q) missing %q, got %q",
						tt.resourceType, tt.id, want, got)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractMQTTHost (mqtt.go)
// ---------------------------------------------------------------------------

func TestExtractMQTTHost(t *testing.T) {
	tests := []struct {
		name   string
		broker string
		want   string
	}{
		{name: "host_and_port", broker: "localhost:1883", want: "localhost"},
		{name: "host_only", broker: "myhost", want: "myhost"},
		{name: "ip_and_port", broker: "192.168.1.1:1883", want: "192.168.1.1"},
		{name: "empty", broker: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMQTTHost(tt.broker)
			if got != tt.want {
				t.Errorf("extractMQTTHost(%q) = %q, want %q", tt.broker, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractMQTTPort (mqtt.go)
// ---------------------------------------------------------------------------

func TestExtractMQTTPort(t *testing.T) {
	tests := []struct {
		name   string
		broker string
		want   string
	}{
		{name: "host_and_port", broker: "localhost:1883", want: "1883"},
		{name: "host_only", broker: "myhost", want: "1883"},
		{name: "custom_port", broker: "broker.io:8883", want: "8883"},
		{name: "empty", broker: "", want: "1883"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractMQTTPort(tt.broker)
			if got != tt.want {
				t.Errorf("extractMQTTPort(%q) = %q, want %q", tt.broker, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// messageTypeString (websocket.go)
// ---------------------------------------------------------------------------

func TestMessageTypeString(t *testing.T) {
	tests := []struct {
		name    string
		msgType int
		want    string
	}{
		{name: "text", msgType: websocket.TextMessage, want: "text"},
		{name: "binary", msgType: websocket.BinaryMessage, want: "binary"},
		{name: "close", msgType: websocket.CloseMessage, want: "close"},
		{name: "ping", msgType: websocket.PingMessage, want: "ping"},
		{name: "pong", msgType: websocket.PongMessage, want: "pong"},
		{name: "unknown", msgType: 99, want: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := messageTypeString(tt.msgType)
			if got != tt.want {
				t.Errorf("messageTypeString(%d) = %q, want %q", tt.msgType, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractTypeName (grpc.go)
// ---------------------------------------------------------------------------

func TestExtractTypeName(t *testing.T) {
	tests := []struct {
		name string
		fqn  string
		want string
	}{
		{name: "fully_qualified", fqn: ".google.protobuf.Empty", want: "Empty"},
		{name: "package_prefixed", fqn: "greet.HelloRequest", want: "HelloRequest"},
		{name: "simple_name", fqn: "MyType", want: "MyType"},
		{name: "deep_nesting", fqn: "a.b.c.d.Response", want: "Response"},
		{name: "empty", fqn: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTypeName(tt.fqn)
			if got != tt.want {
				t.Errorf("extractTypeName(%q) = %q, want %q", tt.fqn, got, tt.want)
			}
		})
	}
}
