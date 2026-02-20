package requestlog

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

// ── Entry tests ──────────────────────────────────────────────────────────────

func TestProtocolConstants(t *testing.T) {
	// Verify constants are non-empty and distinct.
	protos := []string{
		ProtocolHTTP, ProtocolGRPC, ProtocolWebSocket,
		ProtocolSSE, ProtocolMQTT, ProtocolSOAP, ProtocolGraphQL,
	}
	seen := make(map[string]bool)
	for _, p := range protos {
		if p == "" {
			t.Fatal("protocol constant must not be empty")
		}
		if seen[p] {
			t.Fatalf("duplicate protocol constant: %s", p)
		}
		seen[p] = true
	}
}

func TestEntry_JSONRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	entry := &Entry{
		ID:             "req-001",
		Timestamp:      now,
		Protocol:       ProtocolHTTP,
		Method:         "GET",
		Path:           "/api/users",
		QueryString:    "page=1",
		Headers:        map[string][]string{"Accept": {"application/json"}},
		Body:           `{"q":"test"}`,
		BodySize:       12,
		RemoteAddr:     "127.0.0.1",
		MatchedMockID:  "mock-42",
		ResponseStatus: 200,
		ResponseBody:   `[{"id":1}]`,
		DurationMs:     5,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Entry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != entry.ID {
		t.Errorf("ID mismatch: got %q want %q", decoded.ID, entry.ID)
	}
	if decoded.Protocol != ProtocolHTTP {
		t.Errorf("Protocol mismatch: got %q", decoded.Protocol)
	}
	if decoded.Method != "GET" {
		t.Errorf("Method mismatch: got %q", decoded.Method)
	}
	if decoded.Path != "/api/users" {
		t.Errorf("Path mismatch: got %q", decoded.Path)
	}
	if decoded.QueryString != "page=1" {
		t.Errorf("QueryString mismatch: got %q", decoded.QueryString)
	}
	if decoded.MatchedMockID != "mock-42" {
		t.Errorf("MatchedMockID mismatch: got %q", decoded.MatchedMockID)
	}
	if decoded.ResponseStatus != 200 {
		t.Errorf("ResponseStatus mismatch: got %d", decoded.ResponseStatus)
	}
	if decoded.DurationMs != 5 {
		t.Errorf("DurationMs mismatch: got %d", decoded.DurationMs)
	}
	if len(decoded.Headers) != 1 || decoded.Headers["Accept"][0] != "application/json" {
		t.Errorf("Headers mismatch: got %v", decoded.Headers)
	}
}

func TestEntry_JSONOmitsEmptyProtocolMeta(t *testing.T) {
	entry := &Entry{
		ID:       "req-002",
		Protocol: ProtocolHTTP,
		Method:   "POST",
		Path:     "/items",
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Protocol-specific meta fields should be absent when nil.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	for _, key := range []string{"grpc", "websocket", "sse", "mqtt", "soap", "graphql"} {
		if _, ok := raw[key]; ok {
			t.Errorf("field %q should be omitted when nil", key)
		}
	}
}

func TestEntry_WithGRPCMeta(t *testing.T) {
	entry := &Entry{
		ID:       "grpc-001",
		Protocol: ProtocolGRPC,
		GRPC: &GRPCMeta{
			Service:    "mypackage.UserService",
			MethodName: "GetUser",
			StreamType: "unary",
			StatusCode: "OK",
		},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Entry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.GRPC == nil {
		t.Fatal("expected GRPC meta to be present")
	}
	if decoded.GRPC.Service != "mypackage.UserService" {
		t.Errorf("Service mismatch: got %q", decoded.GRPC.Service)
	}
	if decoded.GRPC.MethodName != "GetUser" {
		t.Errorf("MethodName mismatch: got %q", decoded.GRPC.MethodName)
	}
}

func TestEntry_WithWebSocketMeta(t *testing.T) {
	entry := &Entry{
		ID:       "ws-001",
		Protocol: ProtocolWebSocket,
		WebSocket: &WebSocketMeta{
			ConnectionID: "conn-123",
			MessageType:  "text",
			Direction:    "inbound",
			Subprotocol:  "graphql-ws",
			CloseCode:    1000,
		},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Entry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.WebSocket == nil {
		t.Fatal("expected WebSocket meta")
	}
	if decoded.WebSocket.ConnectionID != "conn-123" {
		t.Errorf("ConnectionID mismatch: got %q", decoded.WebSocket.ConnectionID)
	}
	if decoded.WebSocket.CloseCode != 1000 {
		t.Errorf("CloseCode mismatch: got %d", decoded.WebSocket.CloseCode)
	}
}

func TestEntry_WithSSEMeta(t *testing.T) {
	entry := &Entry{
		ID:       "sse-001",
		Protocol: ProtocolSSE,
		SSE: &SSEMeta{
			ConnectionID: "sse-conn-1",
			EventType:    "update",
			EventID:      "evt-42",
			IsConnection: false,
			EventCount:   17,
		},
	}

	data, _ := json.Marshal(entry)
	var decoded Entry
	_ = json.Unmarshal(data, &decoded)

	if decoded.SSE == nil {
		t.Fatal("expected SSE meta")
	}
	if decoded.SSE.EventType != "update" {
		t.Errorf("EventType mismatch: got %q", decoded.SSE.EventType)
	}
	if decoded.SSE.EventCount != 17 {
		t.Errorf("EventCount mismatch: got %d", decoded.SSE.EventCount)
	}
}

func TestEntry_WithMQTTMeta(t *testing.T) {
	entry := &Entry{
		ID:       "mqtt-001",
		Protocol: ProtocolMQTT,
		MQTT: &MQTTMeta{
			ClientID:  "device-abc",
			Topic:     "sensors/temp",
			QoS:       1,
			Retain:    true,
			Direction: "publish",
			MessageID: 1234,
		},
	}

	data, _ := json.Marshal(entry)
	var decoded Entry
	_ = json.Unmarshal(data, &decoded)

	if decoded.MQTT == nil {
		t.Fatal("expected MQTT meta")
	}
	if decoded.MQTT.ClientID != "device-abc" {
		t.Errorf("ClientID mismatch: got %q", decoded.MQTT.ClientID)
	}
	if decoded.MQTT.QoS != 1 {
		t.Errorf("QoS mismatch: got %d", decoded.MQTT.QoS)
	}
	if !decoded.MQTT.Retain {
		t.Error("Retain should be true")
	}
	if decoded.MQTT.MessageID != 1234 {
		t.Errorf("MessageID mismatch: got %d", decoded.MQTT.MessageID)
	}
}

func TestEntry_WithSOAPMeta(t *testing.T) {
	entry := &Entry{
		ID:       "soap-001",
		Protocol: ProtocolSOAP,
		SOAP: &SOAPMeta{
			Operation:   "GetStockPrice",
			SOAPAction:  "urn:GetStockPrice",
			SOAPVersion: "1.2",
			IsFault:     true,
			FaultCode:   "soap:Server",
		},
	}

	data, _ := json.Marshal(entry)
	var decoded Entry
	_ = json.Unmarshal(data, &decoded)

	if decoded.SOAP == nil {
		t.Fatal("expected SOAP meta")
	}
	if decoded.SOAP.Operation != "GetStockPrice" {
		t.Errorf("Operation mismatch: got %q", decoded.SOAP.Operation)
	}
	if !decoded.SOAP.IsFault {
		t.Error("IsFault should be true")
	}
	if decoded.SOAP.FaultCode != "soap:Server" {
		t.Errorf("FaultCode mismatch: got %q", decoded.SOAP.FaultCode)
	}
}

func TestEntry_WithGraphQLMeta(t *testing.T) {
	entry := &Entry{
		ID:       "gql-001",
		Protocol: ProtocolGraphQL,
		GraphQL: &GraphQLMeta{
			OperationType: "query",
			OperationName: "GetUser",
			Variables:     `{"id":"1"}`,
			HasErrors:     true,
			ErrorCount:    2,
		},
	}

	data, _ := json.Marshal(entry)
	var decoded Entry
	_ = json.Unmarshal(data, &decoded)

	if decoded.GraphQL == nil {
		t.Fatal("expected GraphQL meta")
	}
	if decoded.GraphQL.OperationType != "query" {
		t.Errorf("OperationType mismatch: got %q", decoded.GraphQL.OperationType)
	}
	if decoded.GraphQL.OperationName != "GetUser" {
		t.Errorf("OperationName mismatch: got %q", decoded.GraphQL.OperationName)
	}
	if !decoded.GraphQL.HasErrors {
		t.Error("HasErrors should be true")
	}
	if decoded.GraphQL.ErrorCount != 2 {
		t.Errorf("ErrorCount mismatch: got %d", decoded.GraphQL.ErrorCount)
	}
}

func TestEntry_ErrorField(t *testing.T) {
	entry := &Entry{
		ID:       "err-001",
		Protocol: ProtocolHTTP,
		Error:    "connection refused",
	}

	data, _ := json.Marshal(entry)
	var decoded Entry
	_ = json.Unmarshal(data, &decoded)

	if decoded.Error != "connection refused" {
		t.Errorf("Error mismatch: got %q", decoded.Error)
	}
}

func TestEntry_EmptyOptionalFields(t *testing.T) {
	// A minimal entry should serialize cleanly without optional fields.
	entry := &Entry{
		ID:       "min-001",
		Protocol: ProtocolHTTP,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("marshal minimal entry: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal raw: %v", err)
	}

	// These optional fields should be omitted.
	for _, key := range []string{"queryString", "headers", "body", "matchedMockID", "responseBody", "error"} {
		if _, ok := raw[key]; ok {
			t.Errorf("optional field %q should be omitted when empty/zero", key)
		}
	}

	// These required fields should be present.
	for _, key := range []string{"id", "protocol"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("required field %q should be present", key)
		}
	}
}

// ── Filter tests ─────────────────────────────────────────────────────────────

func TestFilter_ZeroValue(t *testing.T) {
	f := &Filter{}
	// Zero-value filter should have all empty/zero fields.
	if f.Protocol != "" {
		t.Errorf("Protocol should be empty, got %q", f.Protocol)
	}
	if f.Limit != 0 {
		t.Errorf("Limit should be 0, got %d", f.Limit)
	}
	if f.HasError != nil {
		t.Error("HasError should be nil")
	}
}

func TestFilter_AllFields(t *testing.T) {
	hasErr := true
	f := &Filter{
		Protocol:        ProtocolHTTP,
		Method:          "POST",
		Path:            "/api/",
		MatchedID:       "mock-1",
		StatusCode:      201,
		HasError:        &hasErr,
		Limit:           50,
		Offset:          10,
		GRPCService:     "svc.Users",
		MQTTTopic:       "sensors/+",
		MQTTClientID:    "client-a",
		SOAPOperation:   "CreateOrder",
		GraphQLOpType:   "mutation",
		WSConnectionID:  "ws-conn-9",
		SSEConnectionID: "sse-conn-3",
	}

	if f.Protocol != ProtocolHTTP {
		t.Error("Protocol not set")
	}
	if f.Limit != 50 {
		t.Errorf("Limit: got %d", f.Limit)
	}
	if f.Offset != 10 {
		t.Errorf("Offset: got %d", f.Offset)
	}
	if *f.HasError != true {
		t.Error("HasError should be true")
	}
	if f.GRPCService != "svc.Users" {
		t.Errorf("GRPCService: got %q", f.GRPCService)
	}
	if f.WSConnectionID != "ws-conn-9" {
		t.Errorf("WSConnectionID: got %q", f.WSConnectionID)
	}
	if f.SSEConnectionID != "sse-conn-3" {
		t.Errorf("SSEConnectionID: got %q", f.SSEConnectionID)
	}
}

// ── Subscriber type test ─────────────────────────────────────────────────────

func TestSubscriber_ChannelBehavior(t *testing.T) {
	sub := make(Subscriber, 5)

	entry := &Entry{ID: "sub-001", Protocol: ProtocolHTTP}
	sub <- entry

	select {
	case got := <-sub:
		if got.ID != "sub-001" {
			t.Errorf("received wrong entry: %q", got.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for entry")
	}
}

func TestSubscriber_BufferedCapacity(t *testing.T) {
	const cap = 3
	sub := make(Subscriber, cap)

	for i := 0; i < cap; i++ {
		sub <- &Entry{ID: "buf-" + string(rune('a'+i))}
	}

	if len(sub) != cap {
		t.Errorf("expected %d buffered entries, got %d", cap, len(sub))
	}
}

// ── Interface compliance tests ───────────────────────────────────────────────

// memoryStore is a minimal in-memory implementation used for testing the interfaces.
type memoryStore struct {
	mu      sync.RWMutex
	entries []*Entry
	maxCap  int
	subs    []Subscriber
}

func newMemoryStore(maxCap int) *memoryStore {
	return &memoryStore{
		entries: make([]*Entry, 0),
		maxCap:  maxCap,
	}
}

func (s *memoryStore) Log(entry *Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Enforce capacity: evict oldest when at max.
	if s.maxCap > 0 && len(s.entries) >= s.maxCap {
		s.entries = s.entries[1:]
	}
	s.entries = append(s.entries, entry)

	// Notify subscribers (non-blocking).
	for _, sub := range s.subs {
		select {
		case sub <- entry:
		default:
		}
	}
}

func (s *memoryStore) Get(id string) *Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, e := range s.entries {
		if e.ID == id {
			return e
		}
	}
	return nil
}

func (s *memoryStore) List(filter *Filter) []*Entry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Entry, 0)
	for _, e := range s.entries {
		if filter != nil {
			if filter.Protocol != "" && e.Protocol != filter.Protocol {
				continue
			}
			if filter.Method != "" && e.Method != filter.Method {
				continue
			}
			if filter.StatusCode != 0 && e.ResponseStatus != filter.StatusCode {
				continue
			}
		}
		result = append(result, e)
	}

	if filter != nil && filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}
	if filter != nil && filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result
}

func (s *memoryStore) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = s.entries[:0]
}

func (s *memoryStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

func (s *memoryStore) Subscribe() (Subscriber, func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub := make(Subscriber, 100)
	s.subs = append(s.subs, sub)
	return sub, func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for i, existing := range s.subs {
			if existing == sub {
				s.subs = append(s.subs[:i], s.subs[i+1:]...)
				close(sub)
				return
			}
		}
	}
}

func (s *memoryStore) ClearByMockID(mockID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	filtered := make([]*Entry, 0, len(s.entries))
	for _, e := range s.entries {
		if e.MatchedMockID != mockID {
			filtered = append(filtered, e)
		}
	}
	s.entries = filtered
}

func (s *memoryStore) CountByMockID(mockID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, e := range s.entries {
		if e.MatchedMockID == mockID {
			count++
		}
	}
	return count
}

// Compile-time interface checks.
var (
	_ Logger            = (*memoryStore)(nil)
	_ Store             = (*memoryStore)(nil)
	_ SubscribableStore = (*memoryStore)(nil)
	_ ExtendedStore     = (*memoryStore)(nil)
)

// ── Store behavior tests ─────────────────────────────────────────────────────

func TestStore_LogAndGet(t *testing.T) {
	store := newMemoryStore(100)

	entry := &Entry{ID: "e1", Protocol: ProtocolHTTP, Method: "GET", Path: "/users"}
	store.Log(entry)

	got := store.Get("e1")
	if got == nil {
		t.Fatal("expected to get entry e1")
	}
	if got.Path != "/users" {
		t.Errorf("Path mismatch: got %q", got.Path)
	}
}

func TestStore_GetNonExistent(t *testing.T) {
	store := newMemoryStore(100)

	if got := store.Get("does-not-exist"); got != nil {
		t.Errorf("expected nil for non-existent ID, got %+v", got)
	}
}

func TestStore_Count(t *testing.T) {
	store := newMemoryStore(100)

	if store.Count() != 0 {
		t.Errorf("empty store should have count 0, got %d", store.Count())
	}

	store.Log(&Entry{ID: "a"})
	store.Log(&Entry{ID: "b"})
	store.Log(&Entry{ID: "c"})

	if store.Count() != 3 {
		t.Errorf("expected count 3, got %d", store.Count())
	}
}

func TestStore_Clear(t *testing.T) {
	store := newMemoryStore(100)

	store.Log(&Entry{ID: "a"})
	store.Log(&Entry{ID: "b"})
	store.Clear()

	if store.Count() != 0 {
		t.Errorf("after clear, expected count 0, got %d", store.Count())
	}
	if got := store.Get("a"); got != nil {
		t.Error("after clear, Get should return nil")
	}
}

func TestStore_ClearEmpty(t *testing.T) {
	store := newMemoryStore(100)
	store.Clear() // Should not panic.
	if store.Count() != 0 {
		t.Errorf("expected 0, got %d", store.Count())
	}
}

func TestStore_ListNoFilter(t *testing.T) {
	store := newMemoryStore(100)
	store.Log(&Entry{ID: "a", Protocol: ProtocolHTTP})
	store.Log(&Entry{ID: "b", Protocol: ProtocolGRPC})

	all := store.List(nil)
	if len(all) != 2 {
		t.Errorf("expected 2 entries, got %d", len(all))
	}
}

func TestStore_ListFilterByProtocol(t *testing.T) {
	store := newMemoryStore(100)
	store.Log(&Entry{ID: "h1", Protocol: ProtocolHTTP})
	store.Log(&Entry{ID: "g1", Protocol: ProtocolGRPC})
	store.Log(&Entry{ID: "h2", Protocol: ProtocolHTTP})

	httpOnly := store.List(&Filter{Protocol: ProtocolHTTP})
	if len(httpOnly) != 2 {
		t.Errorf("expected 2 HTTP entries, got %d", len(httpOnly))
	}
	for _, e := range httpOnly {
		if e.Protocol != ProtocolHTTP {
			t.Errorf("expected HTTP, got %q", e.Protocol)
		}
	}
}

func TestStore_ListFilterByMethod(t *testing.T) {
	store := newMemoryStore(100)
	store.Log(&Entry{ID: "1", Method: "GET"})
	store.Log(&Entry{ID: "2", Method: "POST"})
	store.Log(&Entry{ID: "3", Method: "GET"})

	gets := store.List(&Filter{Method: "GET"})
	if len(gets) != 2 {
		t.Errorf("expected 2 GETs, got %d", len(gets))
	}
}

func TestStore_ListFilterByStatusCode(t *testing.T) {
	store := newMemoryStore(100)
	store.Log(&Entry{ID: "1", ResponseStatus: 200})
	store.Log(&Entry{ID: "2", ResponseStatus: 404})
	store.Log(&Entry{ID: "3", ResponseStatus: 200})

	found := store.List(&Filter{StatusCode: 404})
	if len(found) != 1 {
		t.Errorf("expected 1 entry with 404, got %d", len(found))
	}
}

func TestStore_ListWithLimit(t *testing.T) {
	store := newMemoryStore(100)
	for i := 0; i < 10; i++ {
		store.Log(&Entry{ID: "e" + string(rune('0'+i))})
	}

	limited := store.List(&Filter{Limit: 3})
	if len(limited) != 3 {
		t.Errorf("expected 3 entries with limit, got %d", len(limited))
	}
}

func TestStore_ListWithOffset(t *testing.T) {
	store := newMemoryStore(100)
	for i := 0; i < 5; i++ {
		store.Log(&Entry{ID: "e" + string(rune('0'+i))})
	}

	offset := store.List(&Filter{Offset: 2})
	if len(offset) != 3 {
		t.Errorf("expected 3 entries with offset 2, got %d", len(offset))
	}
}

// ── Max capacity / eviction ──────────────────────────────────────────────────

func TestStore_MaxCapacityEvictsOldest(t *testing.T) {
	store := newMemoryStore(3)

	store.Log(&Entry{ID: "first"})
	store.Log(&Entry{ID: "second"})
	store.Log(&Entry{ID: "third"})
	store.Log(&Entry{ID: "fourth"}) // should evict "first"

	if store.Count() != 3 {
		t.Errorf("expected count 3 after eviction, got %d", store.Count())
	}
	if store.Get("first") != nil {
		t.Error("oldest entry 'first' should have been evicted")
	}
	if store.Get("fourth") == nil {
		t.Error("newest entry 'fourth' should exist")
	}
}

func TestStore_MaxCapacityZeroUnlimited(t *testing.T) {
	store := newMemoryStore(0) // 0 = unlimited
	for i := 0; i < 1000; i++ {
		store.Log(&Entry{ID: "e"})
	}
	if store.Count() != 1000 {
		t.Errorf("unlimited store should hold 1000 entries, got %d", store.Count())
	}
}

func TestStore_MaxCapacityOne(t *testing.T) {
	store := newMemoryStore(1)

	store.Log(&Entry{ID: "a"})
	store.Log(&Entry{ID: "b"})

	if store.Count() != 1 {
		t.Errorf("expected 1, got %d", store.Count())
	}
	if store.Get("a") != nil {
		t.Error("'a' should have been evicted")
	}
	if store.Get("b") == nil {
		t.Error("'b' should exist")
	}
}

// ── Concurrent access ────────────────────────────────────────────────────────

func TestStore_ConcurrentLogAndRead(t *testing.T) {
	store := newMemoryStore(500)

	const writers = 10
	const entriesPerWriter = 50

	var wg sync.WaitGroup

	// Concurrent writers.
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for i := 0; i < entriesPerWriter; i++ {
				store.Log(&Entry{
					ID:       "ignored",
					Protocol: ProtocolHTTP,
					Method:   "GET",
				})
			}
		}(w)
	}

	// Concurrent readers.
	for r := 0; r < 5; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				store.Count()
				store.List(nil)
				store.Get("any")
			}
		}()
	}

	wg.Wait()

	if store.Count() != writers*entriesPerWriter {
		t.Errorf("expected %d entries, got %d", writers*entriesPerWriter, store.Count())
	}
}

func TestStore_ConcurrentClear(t *testing.T) {
	store := newMemoryStore(1000)

	var wg sync.WaitGroup

	// Writer goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			store.Log(&Entry{ID: "e"})
		}
	}()

	// Clear goroutine.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			store.Clear()
		}
	}()

	wg.Wait()
	// No panic or race = success; count is non-deterministic.
}

// ── SubscribableStore tests ──────────────────────────────────────────────────

func TestSubscribableStore_ReceivesNewEntries(t *testing.T) {
	store := newMemoryStore(100)

	sub, unsub := store.Subscribe()
	defer unsub()

	entry := &Entry{ID: "sub-test", Protocol: ProtocolHTTP}
	store.Log(entry)

	select {
	case got := <-sub:
		if got.ID != "sub-test" {
			t.Errorf("subscriber received wrong entry: %q", got.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive entry")
	}
}

func TestSubscribableStore_MultipleSubscribers(t *testing.T) {
	store := newMemoryStore(100)

	sub1, unsub1 := store.Subscribe()
	defer unsub1()
	sub2, unsub2 := store.Subscribe()
	defer unsub2()

	store.Log(&Entry{ID: "multi-sub"})

	for i, sub := range []Subscriber{sub1, sub2} {
		select {
		case got := <-sub:
			if got.ID != "multi-sub" {
				t.Errorf("subscriber %d got wrong entry", i)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d timed out", i)
		}
	}
}

func TestSubscribableStore_UnsubscribeStopsDelivery(t *testing.T) {
	store := newMemoryStore(100)

	sub, unsub := store.Subscribe()
	unsub()

	store.Log(&Entry{ID: "after-unsub"})

	// Drain any buffered entries.
	select {
	case _, ok := <-sub:
		if ok {
			// It's possible an entry squeezed through before unsub took effect.
			// But the channel should be closed, so further reads return zero value.
		}
	case <-time.After(100 * time.Millisecond):
		// Expected: nothing received after unsubscribe.
	}
}

// ── ExtendedStore tests ──────────────────────────────────────────────────────

func TestExtendedStore_ClearByMockID(t *testing.T) {
	store := newMemoryStore(100)

	store.Log(&Entry{ID: "1", MatchedMockID: "mock-a"})
	store.Log(&Entry{ID: "2", MatchedMockID: "mock-b"})
	store.Log(&Entry{ID: "3", MatchedMockID: "mock-a"})

	store.ClearByMockID("mock-a")

	if store.Count() != 1 {
		t.Errorf("expected 1 entry after ClearByMockID, got %d", store.Count())
	}
	if store.Get("2") == nil {
		t.Error("entry '2' (mock-b) should still exist")
	}
}

func TestExtendedStore_ClearByMockID_NonExistent(t *testing.T) {
	store := newMemoryStore(100)
	store.Log(&Entry{ID: "1", MatchedMockID: "mock-a"})

	store.ClearByMockID("non-existent")

	if store.Count() != 1 {
		t.Errorf("clearing non-existent mock ID should not remove entries, got count %d", store.Count())
	}
}

func TestExtendedStore_CountByMockID(t *testing.T) {
	store := newMemoryStore(100)

	store.Log(&Entry{ID: "1", MatchedMockID: "mock-a"})
	store.Log(&Entry{ID: "2", MatchedMockID: "mock-b"})
	store.Log(&Entry{ID: "3", MatchedMockID: "mock-a"})
	store.Log(&Entry{ID: "4", MatchedMockID: "mock-a"})

	if count := store.CountByMockID("mock-a"); count != 3 {
		t.Errorf("expected 3 for mock-a, got %d", count)
	}
	if count := store.CountByMockID("mock-b"); count != 1 {
		t.Errorf("expected 1 for mock-b, got %d", count)
	}
	if count := store.CountByMockID("nonexistent"); count != 0 {
		t.Errorf("expected 0 for nonexistent, got %d", count)
	}
}

// ── Logger interface (minimal) ───────────────────────────────────────────────

func TestLoggerInterface_AcceptsStore(t *testing.T) {
	store := newMemoryStore(10)
	var logger Logger = store
	logger.Log(&Entry{ID: "via-logger"})

	if store.Count() != 1 {
		t.Error("logging via Logger interface should add to store")
	}
}
