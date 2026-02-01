package performance

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	ws "github.com/coder/websocket"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
)

func setupBenchServer(b *testing.B, wsEndpoints []*config.WebSocketEndpointConfig) *httptest.Server {
	cfg := config.DefaultServerConfiguration()
	srv := engine.NewServer(cfg)

	// Register WebSocket endpoints via ImportConfig
	collection := &config.MockCollection{
		Version:            "1.0",
		WebSocketEndpoints: wsEndpoints,
	}
	if err := srv.ImportConfig(collection, false); err != nil {
		b.Fatalf("failed to register endpoints: %v", err)
	}

	return httptest.NewServer(srv.Handler())
}

// BenchmarkWS_EchoLatency measures message round-trip latency.
// Target: <10ms (SC-003)
func BenchmarkWS_EchoLatency(b *testing.B) {
	echoMode := true
	ts := setupBenchServer(b, []*config.WebSocketEndpointConfig{
		{Path: "/ws/echo", EchoMode: &echoMode},
	})
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/echo"

	ctx := context.Background()
	conn, resp, err := ws.Dial(ctx, wsURL, nil)
	if err != nil {
		b.Fatalf("failed to connect: %v", err)
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	defer conn.Close(ws.StatusNormalClosure, "")

	msg := []byte("benchmark message")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := conn.Write(ctx, ws.MessageText, msg); err != nil {
			b.Fatalf("write error: %v", err)
		}
		if _, _, err := conn.Read(ctx); err != nil {
			b.Fatalf("read error: %v", err)
		}
	}
}

// BenchmarkWS_ConnectionEstablishment measures connection setup time.
// Target: <50ms (SC-007)
func BenchmarkWS_ConnectionEstablishment(b *testing.B) {
	ts := setupBenchServer(b, []*config.WebSocketEndpointConfig{
		{Path: "/ws/bench"},
	})
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/bench"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		conn, resp, err := ws.Dial(ctx, wsURL, nil)
		if err != nil {
			b.Fatalf("failed to connect: %v", err)
		}
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		conn.Close(ws.StatusNormalClosure, "")
	}
}

// BenchmarkWS_ConcurrentConnections tests connection handling under load.
// Target: 1000 concurrent connections (SC-002)
func BenchmarkWS_ConcurrentConnections(b *testing.B) {
	ts := setupBenchServer(b, []*config.WebSocketEndpointConfig{
		{Path: "/ws/concurrent"},
	})
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/concurrent"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		numConns := 100 // Use 100 for benchmark, full 1000 in dedicated test
		conns := make([]*ws.Conn, numConns)
		var wg sync.WaitGroup

		// Connect all
		for j := 0; j < numConns; j++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				conn, resp, err := ws.Dial(ctx, wsURL, nil)
				if resp != nil && resp.Body != nil {
					resp.Body.Close()
				}
				if err != nil {
					return
				}
				conns[idx] = conn
			}(j)
		}
		wg.Wait()

		// Close all
		for _, conn := range conns {
			if conn != nil {
				conn.Close(ws.StatusNormalClosure, "")
			}
		}
	}
}

// BenchmarkWS_MatcherPerformance tests message matching speed.
func BenchmarkWS_MatcherPerformance(b *testing.B) {
	ts := setupBenchServer(b, []*config.WebSocketEndpointConfig{
		{
			Path: "/ws/matcher",
			Matchers: []*mock.WSMatcherConfig{
				{
					Match:    &mock.WSMatchCriteria{Type: "exact", Value: "ping"},
					Response: &mock.WSMessageResponse{Type: "text", Value: "pong"},
				},
				{
					Match:    &mock.WSMatchCriteria{Type: "regex", Value: "^test.*"},
					Response: &mock.WSMessageResponse{Type: "text", Value: "matched"},
				},
				{
					Match:    &mock.WSMatchCriteria{Type: "json", Path: "$.type", Value: "message"},
					Response: &mock.WSMessageResponse{Type: "text", Value: "json matched"},
				},
			},
			DefaultResponse: &mock.WSMessageResponse{Type: "text", Value: "default"},
		},
	})
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/matcher"

	ctx := context.Background()
	conn, resp, err := ws.Dial(ctx, wsURL, nil)
	if err != nil {
		b.Fatalf("failed to connect: %v", err)
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	defer conn.Close(ws.StatusNormalClosure, "")

	messages := []string{
		"ping",
		"test123",
		`{"type": "message", "data": "hello"}`,
		"something else",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := messages[i%len(messages)]
		if err := conn.Write(ctx, ws.MessageText, []byte(msg)); err != nil {
			b.Fatalf("write error: %v", err)
		}
		if _, _, err := conn.Read(ctx); err != nil {
			b.Fatalf("read error: %v", err)
		}
	}
}

// BenchmarkWS_MessageThroughput measures messages per second.
func BenchmarkWS_MessageThroughput(b *testing.B) {
	echoMode := true
	maxMsgSize := int64(128 * 1024) // 128KB to accommodate 64KB test messages
	ts := setupBenchServer(b, []*config.WebSocketEndpointConfig{
		{Path: "/ws/throughput", EchoMode: &echoMode, MaxMessageSize: maxMsgSize},
	})
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/throughput"

	// Create message of varying sizes
	smallMsg := []byte("hello")
	mediumMsg := make([]byte, 1024)
	largeMsg := make([]byte, 64*1024)

	b.Run("small_64B", func(b *testing.B) {
		ctx := context.Background()
		conn, resp, err := ws.Dial(ctx, wsURL, nil)
		if err != nil {
			b.Fatalf("failed to connect: %v", err)
		}
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		defer conn.Close(ws.StatusNormalClosure, "")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := conn.Write(ctx, ws.MessageText, smallMsg); err != nil {
				b.Fatalf("write error: %v", err)
			}
			if _, _, err := conn.Read(ctx); err != nil {
				b.Fatalf("read error: %v", err)
			}
		}
	})

	b.Run("medium_1KB", func(b *testing.B) {
		ctx := context.Background()
		conn, resp, err := ws.Dial(ctx, wsURL, nil)
		if err != nil {
			b.Fatalf("failed to connect: %v", err)
		}
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		defer conn.Close(ws.StatusNormalClosure, "")

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := conn.Write(ctx, ws.MessageBinary, mediumMsg); err != nil {
				b.Fatalf("write error: %v", err)
			}
			if _, _, err := conn.Read(ctx); err != nil {
				b.Fatalf("read error: %v", err)
			}
		}
	})

	b.Run("large_64KB", func(b *testing.B) {
		ctx := context.Background()
		conn, resp, err := ws.Dial(ctx, wsURL, nil)
		if err != nil {
			b.Fatalf("failed to connect: %v", err)
		}
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		defer conn.Close(ws.StatusNormalClosure, "")
		conn.SetReadLimit(128 * 1024) // Allow reading 64KB echo responses

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := conn.Write(ctx, ws.MessageBinary, largeMsg); err != nil {
				b.Fatalf("write error: %v", err)
			}
			if _, _, err := conn.Read(ctx); err != nil {
				b.Fatalf("read error: %v", err)
			}
		}
	})
}
