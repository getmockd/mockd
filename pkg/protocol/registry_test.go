package protocol

import (
	"context"
	"testing"
	"time"
)

// mockHandler is a minimal handler implementation for testing.
type mockHandler struct {
	id          string
	proto       Protocol
	caps        []Capability
	started     bool
	stopped     bool
	startErr    error
	stopErr     error
	healthState HealthState
}

func (h *mockHandler) Metadata() Metadata {
	return Metadata{
		ID:           h.id,
		Protocol:     h.proto,
		Capabilities: h.caps,
	}
}

func (h *mockHandler) Start(ctx context.Context) error {
	h.started = true
	return h.startErr
}

func (h *mockHandler) Stop(ctx context.Context, timeout time.Duration) error {
	h.stopped = true
	return h.stopErr
}

func (h *mockHandler) Health(ctx context.Context) HealthStatus {
	return HealthStatus{
		Status:    h.healthState,
		CheckedAt: time.Now(),
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	h := &mockHandler{id: "test-handler", proto: ProtocolGRPC}
	err := r.Register(h)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Count() != 1 {
		t.Errorf("expected count 1, got %d", r.Count())
	}

	// Duplicate registration should fail
	err = r.Register(h)
	if err == nil {
		t.Error("expected error for duplicate registration")
	}
}

func TestRegistry_Register_NilHandler(t *testing.T) {
	r := NewRegistry()

	err := r.Register(nil)
	if err != ErrNilHandler {
		t.Errorf("expected ErrNilHandler, got %v", err)
	}
}

func TestRegistry_Register_EmptyID(t *testing.T) {
	r := NewRegistry()

	h := &mockHandler{id: "", proto: ProtocolGRPC}
	err := r.Register(h)
	if err != ErrEmptyHandlerID {
		t.Errorf("expected ErrEmptyHandlerID, got %v", err)
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()

	h := &mockHandler{id: "test-handler", proto: ProtocolGRPC}
	_ = r.Register(h)

	err := r.Unregister("test-handler")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if r.Count() != 0 {
		t.Errorf("expected count 0, got %d", r.Count())
	}

	// Unregister non-existent should fail
	err = r.Unregister("non-existent")
	if err == nil {
		t.Error("expected error for non-existent handler")
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()

	h := &mockHandler{id: "test-handler", proto: ProtocolGRPC}
	_ = r.Register(h)

	got, ok := r.Get("test-handler")
	if !ok {
		t.Error("expected handler to be found")
	}
	if got != h {
		t.Error("expected same handler instance")
	}

	_, ok = r.Get("non-existent")
	if ok {
		t.Error("expected handler not to be found")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()

	h1 := &mockHandler{id: "handler-1", proto: ProtocolGRPC}
	h2 := &mockHandler{id: "handler-2", proto: ProtocolMQTT}
	_ = r.Register(h1)
	_ = r.Register(h2)

	handlers := r.List()
	if len(handlers) != 2 {
		t.Errorf("expected 2 handlers, got %d", len(handlers))
	}
}

func TestRegistry_ListByProtocol(t *testing.T) {
	r := NewRegistry()

	h1 := &mockHandler{id: "grpc-1", proto: ProtocolGRPC}
	h2 := &mockHandler{id: "mqtt-1", proto: ProtocolMQTT}
	h3 := &mockHandler{id: "grpc-2", proto: ProtocolGRPC}
	_ = r.Register(h1)
	_ = r.Register(h2)
	_ = r.Register(h3)

	grpcHandlers := r.ListByProtocol(ProtocolGRPC)
	if len(grpcHandlers) != 2 {
		t.Errorf("expected 2 gRPC handlers, got %d", len(grpcHandlers))
	}

	mqttHandlers := r.ListByProtocol(ProtocolMQTT)
	if len(mqttHandlers) != 1 {
		t.Errorf("expected 1 MQTT handler, got %d", len(mqttHandlers))
	}
}

func TestRegistry_ListByCapability(t *testing.T) {
	r := NewRegistry()

	h1 := &mockHandler{id: "handler-1", caps: []Capability{CapabilityRecording, CapabilityConnections}}
	h2 := &mockHandler{id: "handler-2", caps: []Capability{CapabilityBroadcast}}
	h3 := &mockHandler{id: "handler-3", caps: []Capability{CapabilityRecording}}
	_ = r.Register(h1)
	_ = r.Register(h2)
	_ = r.Register(h3)

	recordable := r.ListByCapability(CapabilityRecording)
	if len(recordable) != 2 {
		t.Errorf("expected 2 recordable handlers, got %d", len(recordable))
	}
}

func TestRegistry_StartAll(t *testing.T) {
	r := NewRegistry()

	h1 := &mockHandler{id: "handler-1", healthState: HealthHealthy}
	h2 := &mockHandler{id: "handler-2", healthState: HealthHealthy}
	_ = r.Register(h1)
	_ = r.Register(h2)

	err := r.StartAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !h1.started || !h2.started {
		t.Error("expected all handlers to be started")
	}
}

func TestRegistry_StopAll(t *testing.T) {
	r := NewRegistry()

	h1 := &mockHandler{id: "handler-1"}
	h2 := &mockHandler{id: "handler-2"}
	_ = r.Register(h1)
	_ = r.Register(h2)

	err := r.StopAll(context.Background(), 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !h1.stopped || !h2.stopped {
		t.Error("expected all handlers to be stopped")
	}
}

func TestRegistry_HealthAll(t *testing.T) {
	r := NewRegistry()

	h1 := &mockHandler{id: "handler-1", healthState: HealthHealthy}
	h2 := &mockHandler{id: "handler-2", healthState: HealthDegraded}
	_ = r.Register(h1)
	_ = r.Register(h2)

	health := r.HealthAll(context.Background())
	if len(health) != 2 {
		t.Errorf("expected 2 health entries, got %d", len(health))
	}

	if health["handler-1"].Status != HealthHealthy {
		t.Errorf("expected handler-1 to be healthy, got %s", health["handler-1"].Status)
	}
	if health["handler-2"].Status != HealthDegraded {
		t.Errorf("expected handler-2 to be degraded, got %s", health["handler-2"].Status)
	}
}
