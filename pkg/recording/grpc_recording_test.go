package recording

import (
	"testing"
	"time"
)

func TestNewGRPCRecording(t *testing.T) {
	rec := NewGRPCRecording("mypackage.MyService", "GetUser", GRPCStreamUnary)

	if rec.ID == "" {
		t.Error("Expected ID to be set")
	}
	if rec.Service != "mypackage.MyService" {
		t.Errorf("Expected service 'mypackage.MyService', got '%s'", rec.Service)
	}
	if rec.Method != "GetUser" {
		t.Errorf("Expected method 'GetUser', got '%s'", rec.Method)
	}
	if rec.StreamType != GRPCStreamUnary {
		t.Errorf("Expected stream type 'unary', got '%s'", rec.StreamType)
	}
	if rec.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}

func TestGRPCRecordingSetters(t *testing.T) {
	rec := NewGRPCRecording("test.Service", "TestMethod", GRPCStreamUnary)

	// Test SetRequest
	reqData := map[string]interface{}{"id": 123}
	rec.SetRequest(reqData)
	if rec.Request == nil {
		t.Error("Expected request to be set")
	}

	// Test SetResponse
	respData := map[string]interface{}{"name": "test"}
	rec.SetResponse(respData)
	if rec.Response == nil {
		t.Error("Expected response to be set")
	}

	// Test SetMetadata
	md := map[string][]string{"auth": {"token123"}}
	rec.SetMetadata(md)
	if rec.Metadata == nil {
		t.Error("Expected metadata to be set")
	}

	// Test SetError
	rec.SetError("NOT_FOUND", "User not found")
	if rec.Error == nil {
		t.Error("Expected error to be set")
	}
	if rec.Error.Code != "NOT_FOUND" {
		t.Errorf("Expected error code 'NOT_FOUND', got '%s'", rec.Error.Code)
	}

	// Test SetDuration
	rec.SetDuration(100 * time.Millisecond)
	if rec.Duration != 100*time.Millisecond {
		t.Errorf("Expected duration 100ms, got %v", rec.Duration)
	}

	// Test SetProtoFile
	rec.SetProtoFile("/path/to/service.proto")
	if rec.ProtoFile != "/path/to/service.proto" {
		t.Errorf("Expected proto file path, got '%s'", rec.ProtoFile)
	}
}

func TestGRPCRecordingFullMethod(t *testing.T) {
	rec := NewGRPCRecording("mypackage.MyService", "GetUser", GRPCStreamUnary)
	expected := "/mypackage.MyService/GetUser"
	if rec.FullMethod() != expected {
		t.Errorf("Expected '%s', got '%s'", expected, rec.FullMethod())
	}
}

func TestGRPCStore(t *testing.T) {
	store := NewGRPCStore(10)

	// Test Add
	rec1 := NewGRPCRecording("test.Service", "Method1", GRPCStreamUnary)
	if err := store.Add(rec1); err != nil {
		t.Errorf("Failed to add recording: %v", err)
	}

	// Test Get
	got := store.Get(rec1.ID)
	if got == nil {
		t.Error("Expected to get recording")
	}
	if got.ID != rec1.ID {
		t.Errorf("Expected ID '%s', got '%s'", rec1.ID, got.ID)
	}

	// Test Count
	if store.Count() != 1 {
		t.Errorf("Expected count 1, got %d", store.Count())
	}

	// Test List
	recordings, total := store.List(GRPCRecordingFilter{})
	if total != 1 {
		t.Errorf("Expected total 1, got %d", total)
	}
	if len(recordings) != 1 {
		t.Errorf("Expected 1 recording, got %d", len(recordings))
	}

	// Test Delete
	if err := store.Delete(rec1.ID); err != nil {
		t.Errorf("Failed to delete recording: %v", err)
	}
	if store.Count() != 0 {
		t.Errorf("Expected count 0, got %d", store.Count())
	}
}

func TestGRPCStoreMaxSize(t *testing.T) {
	store := NewGRPCStore(3)

	// Add 5 recordings, should only keep the last 3
	for i := 0; i < 5; i++ {
		rec := NewGRPCRecording("test.Service", "Method", GRPCStreamUnary)
		store.Add(rec)
	}

	if store.Count() != 3 {
		t.Errorf("Expected count 3, got %d", store.Count())
	}
}

func TestGRPCStoreFilter(t *testing.T) {
	store := NewGRPCStore(100)

	// Add recordings for different services
	rec1 := NewGRPCRecording("service.A", "Method1", GRPCStreamUnary)
	rec2 := NewGRPCRecording("service.A", "Method2", GRPCStreamUnary)
	rec3 := NewGRPCRecording("service.B", "Method1", GRPCStreamServerStream)
	rec4 := NewGRPCRecording("service.B", "Method1", GRPCStreamUnary)
	rec4.SetError("NOT_FOUND", "not found")

	store.Add(rec1)
	store.Add(rec2)
	store.Add(rec3)
	store.Add(rec4)

	// Filter by service
	recordings, _ := store.List(GRPCRecordingFilter{Service: "service.A"})
	if len(recordings) != 2 {
		t.Errorf("Expected 2 recordings for service.A, got %d", len(recordings))
	}

	// Filter by method
	recordings, _ = store.List(GRPCRecordingFilter{Method: "Method1"})
	if len(recordings) != 3 {
		t.Errorf("Expected 3 recordings for Method1, got %d", len(recordings))
	}

	// Filter by stream type
	recordings, _ = store.List(GRPCRecordingFilter{StreamType: "server_stream"})
	if len(recordings) != 1 {
		t.Errorf("Expected 1 server_stream recording, got %d", len(recordings))
	}

	// Filter by error
	hasError := true
	recordings, _ = store.List(GRPCRecordingFilter{HasError: &hasError})
	if len(recordings) != 1 {
		t.Errorf("Expected 1 recording with error, got %d", len(recordings))
	}
}

func TestGRPCStoreStats(t *testing.T) {
	store := NewGRPCStore(100)

	rec1 := NewGRPCRecording("service.A", "Method1", GRPCStreamUnary)
	rec2 := NewGRPCRecording("service.A", "Method2", GRPCStreamServerStream)
	rec3 := NewGRPCRecording("service.B", "Method1", GRPCStreamUnary)
	rec3.SetError("NOT_FOUND", "not found")

	store.Add(rec1)
	store.Add(rec2)
	store.Add(rec3)

	stats := store.Stats()

	if stats.TotalRecordings != 3 {
		t.Errorf("Expected 3 total recordings, got %d", stats.TotalRecordings)
	}
	if stats.ByService["service.A"] != 2 {
		t.Errorf("Expected 2 recordings for service.A, got %d", stats.ByService["service.A"])
	}
	if stats.ByStreamType["unary"] != 2 {
		t.Errorf("Expected 2 unary recordings, got %d", stats.ByStreamType["unary"])
	}
	if stats.ErrorCount != 1 {
		t.Errorf("Expected 1 error recording, got %d", stats.ErrorCount)
	}
}
