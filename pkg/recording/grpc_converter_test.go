package recording

import (
	"testing"
	"time"
)

func TestToGRPCMethodConfig(t *testing.T) {
	t.Run("converts unary recording", func(t *testing.T) {
		rec := &GRPCRecording{
			ID:         "test-1",
			Service:    "test.Service",
			Method:     "GetUser",
			StreamType: GRPCStreamUnary,
			Response:   map[string]interface{}{"name": "John"},
			Duration:   100 * time.Millisecond,
		}

		opts := DefaultGRPCConvertOptions()
		cfg := ToGRPCMethodConfig(rec, opts)

		if cfg == nil {
			t.Fatal("Expected config to be created")
		}
		if cfg.Response == nil {
			t.Error("Expected response to be set")
		}
		if cfg.Delay != "" {
			t.Error("Expected no delay when IncludeDelay is false")
		}
	})

	t.Run("includes delay when enabled", func(t *testing.T) {
		rec := &GRPCRecording{
			ID:         "test-2",
			StreamType: GRPCStreamUnary,
			Response:   map[string]interface{}{"ok": true},
			Duration:   250 * time.Millisecond,
		}

		opts := GRPCConvertOptions{IncludeDelay: true}
		cfg := ToGRPCMethodConfig(rec, opts)

		if cfg.Delay == "" {
			t.Error("Expected delay to be set")
		}
	})

	t.Run("includes metadata match when enabled", func(t *testing.T) {
		rec := &GRPCRecording{
			ID:         "test-3",
			StreamType: GRPCStreamUnary,
			Response:   map[string]interface{}{},
			Metadata:   map[string][]string{"auth": {"token123"}},
		}

		opts := GRPCConvertOptions{IncludeMetadata: true}
		cfg := ToGRPCMethodConfig(rec, opts)

		if cfg.Match == nil {
			t.Error("Expected match to be set")
		}
		if cfg.Match.Metadata["auth"] != "token123" {
			t.Error("Expected auth metadata to be included")
		}
	})

	t.Run("handles error recording", func(t *testing.T) {
		rec := &GRPCRecording{
			ID:         "test-4",
			StreamType: GRPCStreamUnary,
			Error: &GRPCRecordedError{
				Code:    "NOT_FOUND",
				Message: "User not found",
			},
		}

		opts := DefaultGRPCConvertOptions()
		cfg := ToGRPCMethodConfig(rec, opts)

		if cfg.Error == nil {
			t.Error("Expected error to be set")
		}
		if cfg.Error.Code != "NOT_FOUND" {
			t.Errorf("Expected code 'NOT_FOUND', got '%s'", cfg.Error.Code)
		}
	})

	t.Run("handles server streaming recording", func(t *testing.T) {
		rec := &GRPCRecording{
			ID:         "test-5",
			StreamType: GRPCStreamServerStream,
			Response: []interface{}{
				map[string]interface{}{"id": 1},
				map[string]interface{}{"id": 2},
			},
		}

		opts := DefaultGRPCConvertOptions()
		cfg := ToGRPCMethodConfig(rec, opts)

		if len(cfg.Responses) != 2 {
			t.Errorf("Expected 2 responses, got %d", len(cfg.Responses))
		}
	})
}

func TestToGRPCServiceConfig(t *testing.T) {
	recordings := []*GRPCRecording{
		{
			ID:         "1",
			Service:    "test.UserService",
			Method:     "GetUser",
			StreamType: GRPCStreamUnary,
			Response:   map[string]interface{}{"id": 1},
		},
		{
			ID:         "2",
			Service:    "test.UserService",
			Method:     "ListUsers",
			StreamType: GRPCStreamServerStream,
			Response: []interface{}{
				map[string]interface{}{"id": 1},
				map[string]interface{}{"id": 2},
			},
		},
		{
			ID:         "3",
			Service:    "test.OrderService",
			Method:     "GetOrder",
			StreamType: GRPCStreamUnary,
			Response:   map[string]interface{}{"orderId": "abc"},
		},
	}

	opts := DefaultGRPCConvertOptions()
	configs := ToGRPCServiceConfig(recordings, opts)

	if len(configs) != 2 {
		t.Errorf("Expected 2 services, got %d", len(configs))
	}

	userSvc, ok := configs["test.UserService"]
	if !ok {
		t.Error("Expected test.UserService to be present")
	}
	if len(userSvc.Methods) != 2 {
		t.Errorf("Expected 2 methods for UserService, got %d", len(userSvc.Methods))
	}

	orderSvc, ok := configs["test.OrderService"]
	if !ok {
		t.Error("Expected test.OrderService to be present")
	}
	if len(orderSvc.Methods) != 1 {
		t.Errorf("Expected 1 method for OrderService, got %d", len(orderSvc.Methods))
	}
}

func TestToGRPCConfig(t *testing.T) {
	recordings := []*GRPCRecording{
		{
			ID:         "1",
			Service:    "test.Service",
			Method:     "Method1",
			StreamType: GRPCStreamUnary,
			Response:   map[string]interface{}{},
			ProtoFile:  "/path/to/service.proto",
		},
	}

	opts := DefaultGRPCConvertOptions()
	cfg := ToGRPCConfig(recordings, opts)

	if cfg == nil {
		t.Fatal("Expected config to be created")
	}
	if cfg.ProtoFile != "/path/to/service.proto" {
		t.Errorf("Expected proto file path, got '%s'", cfg.ProtoFile)
	}
	if !cfg.Reflection {
		t.Error("Expected reflection to be enabled by default")
	}
	if !cfg.Enabled {
		t.Error("Expected config to be enabled by default")
	}
}

func TestConvertGRPCRecordings(t *testing.T) {
	recordings := []*GRPCRecording{
		{
			ID:         "1",
			Service:    "test.Service",
			Method:     "Unary",
			StreamType: GRPCStreamUnary,
			Response:   map[string]interface{}{},
		},
		{
			ID:         "2",
			Service:    "test.Service",
			Method:     "Streaming",
			StreamType: GRPCStreamServerStream,
			Response:   []interface{}{},
		},
	}

	opts := DefaultGRPCConvertOptions()
	result := ConvertGRPCRecordings(recordings, opts)

	if result.Total != 2 {
		t.Errorf("Expected total 2, got %d", result.Total)
	}
	if result.Services != 1 {
		t.Errorf("Expected 1 service, got %d", result.Services)
	}
	if result.Methods != 2 {
		t.Errorf("Expected 2 methods, got %d", result.Methods)
	}
	if len(result.Warnings) == 0 {
		t.Error("Expected warning for streaming call")
	}
}

func TestDeduplication(t *testing.T) {
	recordings := []*GRPCRecording{
		{
			ID:         "1",
			Timestamp:  time.Now().Add(-2 * time.Second),
			Service:    "test.Service",
			Method:     "GetUser",
			StreamType: GRPCStreamUnary,
			Response:   map[string]interface{}{"version": 1},
		},
		{
			ID:         "2",
			Timestamp:  time.Now().Add(-1 * time.Second),
			Service:    "test.Service",
			Method:     "GetUser",
			StreamType: GRPCStreamUnary,
			Response:   map[string]interface{}{"version": 2},
		},
	}

	t.Run("deduplicates by default", func(t *testing.T) {
		opts := GRPCConvertOptions{Deduplicate: true}
		configs := ToGRPCServiceConfig(recordings, opts)

		svc := configs["test.Service"]
		cfg := svc.Methods["GetUser"]

		// Should use the first recording (version 1) when deduplicating
		resp := cfg.Response.(map[string]interface{})
		if resp["version"] != 1 {
			t.Errorf("Expected version 1 when deduplicating, got %v", resp["version"])
		}
	})

	t.Run("uses last when not deduplicating", func(t *testing.T) {
		opts := GRPCConvertOptions{Deduplicate: false}
		configs := ToGRPCServiceConfig(recordings, opts)

		svc := configs["test.Service"]
		cfg := svc.Methods["GetUser"]

		// Should use the last recording (version 2) when not deduplicating
		resp := cfg.Response.(map[string]interface{})
		if resp["version"] != 2 {
			t.Errorf("Expected version 2 when not deduplicating, got %v", resp["version"])
		}
	})
}
