package quic

import (
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/tunnel/protocol"
)

func TestBuildRawHTTPRequest(t *testing.T) {
	tests := []struct {
		name     string
		meta     *protocol.HTTPMetadata
		host     string
		contains []string
	}{
		{
			name: "basic GET",
			meta: &protocol.HTTPMetadata{
				Method: "GET",
				Path:   "/api/data",
				Host:   "example.com",
			},
			host: "127.0.0.1:8080",
			contains: []string{
				"GET /api/data HTTP/1.1\r\n",
				"Host: example.com\r\n",
				"\r\n\r\n", // end of headers
			},
		},
		{
			name: "WebSocket upgrade",
			meta: &protocol.HTTPMetadata{
				Method: "GET",
				Path:   "/ws",
				Host:   "abc123.tunnel.mockd.io",
				Header: map[string][]string{
					"Upgrade":               {"websocket"},
					"Connection":            {"Upgrade"},
					"Sec-Websocket-Key":     {"dGhlIHNhbXBsZSBub25jZQ=="},
					"Sec-Websocket-Version": {"13"},
				},
			},
			host: "127.0.0.1:8080",
			contains: []string{
				"GET /ws HTTP/1.1\r\n",
				"Host: abc123.tunnel.mockd.io\r\n",
				"Upgrade: websocket\r\n",
				"Connection: Upgrade\r\n",
			},
		},
		{
			name: "empty path defaults to /",
			meta: &protocol.HTTPMetadata{
				Method: "GET",
				Path:   "",
			},
			host: "127.0.0.1:4280",
			contains: []string{
				"GET / HTTP/1.1\r\n",
				"Host: 127.0.0.1:4280\r\n",
			},
		},
		{
			name: "empty method defaults to GET",
			meta: &protocol.HTTPMetadata{
				Path: "/test",
				Host: "example.com",
			},
			host: "127.0.0.1:4280",
			contains: []string{
				"GET /test HTTP/1.1\r\n",
			},
		},
		{
			name: "Host header in metadata is skipped in headers section",
			meta: &protocol.HTTPMetadata{
				Method: "POST",
				Path:   "/api",
				Host:   "my-host.io",
				Header: map[string][]string{
					"Host":         {"should-be-ignored"},
					"Content-Type": {"application/json"},
				},
			},
			host: "127.0.0.1:8080",
			contains: []string{
				"Host: my-host.io\r\n",
				"Content-Type: application/json\r\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(buildRawHTTPRequest(tt.meta, tt.host))

			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("expected request to contain %q\ngot:\n%s", want, got)
				}
			}

			// Every raw request must end with \r\n (after headers)
			if !strings.HasSuffix(got, "\r\n") {
				t.Errorf("expected request to end with \\r\\n, got:\n%s", got)
			}

			// Host header from meta.Header should NOT appear twice
			if tt.name == "Host header in metadata is skipped in headers section" {
				count := strings.Count(got, "Host:")
				if count != 1 {
					t.Errorf("expected exactly 1 Host header, found %d in:\n%s", count, got)
				}
			}
		})
	}
}
