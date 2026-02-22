package recording

import (
	"testing"
	"time"
)

func TestNewSOAPStore(t *testing.T) {
	t.Run("creates store with specified max size", func(t *testing.T) {
		store := NewSOAPStore(100)
		if store == nil {
			t.Fatal("Expected store to be created")
			return
		}
		if store.maxSize != 100 {
			t.Errorf("Expected maxSize 100, got %d", store.maxSize)
		}
		if store.recordings == nil {
			t.Error("Expected recordings map to be initialized")
		}
		if store.order == nil {
			t.Error("Expected order slice to be initialized")
		}
	})

	t.Run("uses default max size when zero or negative", func(t *testing.T) {
		store := NewSOAPStore(0)
		if store.maxSize != 1000 {
			t.Errorf("Expected default maxSize 1000 for zero, got %d", store.maxSize)
		}

		store = NewSOAPStore(-5)
		if store.maxSize != 1000 {
			t.Errorf("Expected default maxSize 1000 for negative, got %d", store.maxSize)
		}
	})
}

func TestSOAPStoreAdd(t *testing.T) {
	t.Run("adds recording successfully", func(t *testing.T) {
		store := NewSOAPStore(10)
		rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")

		err := store.Add(rec)
		if err != nil {
			t.Errorf("Failed to add recording: %v", err)
		}
		if store.Count() != 1 {
			t.Errorf("Expected count 1, got %d", store.Count())
		}
	})

	t.Run("maintains insertion order", func(t *testing.T) {
		store := NewSOAPStore(10)
		rec1 := NewSOAPRecording("/soap/service", "Op1", "1.1")
		rec2 := NewSOAPRecording("/soap/service", "Op2", "1.1")
		rec3 := NewSOAPRecording("/soap/service", "Op3", "1.1")

		store.Add(rec1)
		store.Add(rec2)
		store.Add(rec3)

		if len(store.order) != 3 {
			t.Fatalf("Expected 3 items in order, got %d", len(store.order))
		}
		if store.order[0] != rec1.ID {
			t.Error("Expected rec1 to be first in order")
		}
		if store.order[2] != rec3.ID {
			t.Error("Expected rec3 to be last in order")
		}
	})

	t.Run("evicts oldest when full", func(t *testing.T) {
		store := NewSOAPStore(3)

		ids := make([]string, 5)
		for i := 0; i < 5; i++ {
			rec := NewSOAPRecording("/soap/service", "Method", "1.1")
			ids[i] = rec.ID
			store.Add(rec)
		}

		if store.Count() != 3 {
			t.Errorf("Expected count 3, got %d", store.Count())
		}

		// First two should be evicted
		if store.Get(ids[0]) != nil {
			t.Error("Expected first recording to be evicted")
		}
		if store.Get(ids[1]) != nil {
			t.Error("Expected second recording to be evicted")
		}

		// Last three should remain
		if store.Get(ids[2]) == nil {
			t.Error("Expected third recording to remain")
		}
		if store.Get(ids[3]) == nil {
			t.Error("Expected fourth recording to remain")
		}
		if store.Get(ids[4]) == nil {
			t.Error("Expected fifth recording to remain")
		}
	})

	t.Run("evicts when exactly at max size", func(t *testing.T) {
		store := NewSOAPStore(2)

		rec1 := NewSOAPRecording("/soap/service", "Op1", "1.1")
		rec2 := NewSOAPRecording("/soap/service", "Op2", "1.1")
		rec3 := NewSOAPRecording("/soap/service", "Op3", "1.1")

		store.Add(rec1)
		store.Add(rec2)
		store.Add(rec3)

		if store.Count() != 2 {
			t.Errorf("Expected count 2, got %d", store.Count())
		}
		if store.Get(rec1.ID) != nil {
			t.Error("Expected rec1 to be evicted")
		}
	})
}

func TestSOAPStoreGet(t *testing.T) {
	t.Run("retrieves existing recording by ID", func(t *testing.T) {
		store := NewSOAPStore(10)
		rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")
		rec.SetSOAPAction("http://example.com/GetUser")
		store.Add(rec)

		got := store.Get(rec.ID)
		if got == nil {
			t.Fatal("Expected to get recording")
			return
		}
		if got.ID != rec.ID {
			t.Errorf("Expected ID '%s', got '%s'", rec.ID, got.ID)
		}
		if got.Endpoint != "/soap/service" {
			t.Errorf("Expected endpoint '/soap/service', got '%s'", got.Endpoint)
		}
		if got.Operation != "GetUser" {
			t.Errorf("Expected operation 'GetUser', got '%s'", got.Operation)
		}
		if got.SOAPAction != "http://example.com/GetUser" {
			t.Errorf("Expected SOAPAction, got '%s'", got.SOAPAction)
		}
	})

	t.Run("returns nil for missing ID", func(t *testing.T) {
		store := NewSOAPStore(10)

		got := store.Get("nonexistent-id")
		if got != nil {
			t.Error("Expected nil for missing ID")
		}
	})

	t.Run("returns nil for empty store", func(t *testing.T) {
		store := NewSOAPStore(10)

		got := store.Get("any-id")
		if got != nil {
			t.Error("Expected nil for empty store")
		}
	})

	t.Run("returns nil for evicted recording", func(t *testing.T) {
		store := NewSOAPStore(2)

		rec1 := NewSOAPRecording("/soap/service", "Op1", "1.1")
		rec2 := NewSOAPRecording("/soap/service", "Op2", "1.1")
		rec3 := NewSOAPRecording("/soap/service", "Op3", "1.1")

		store.Add(rec1)
		store.Add(rec2)
		store.Add(rec3)

		got := store.Get(rec1.ID)
		if got != nil {
			t.Error("Expected nil for evicted recording")
		}
	})
}

func TestSOAPStoreList(t *testing.T) {
	store := NewSOAPStore(100)

	// Add recordings for different endpoints and operations
	rec1 := NewSOAPRecording("/soap/serviceA", "GetUser", "1.1")
	rec1.SetSOAPAction("http://example.com/GetUser")

	rec2 := NewSOAPRecording("/soap/serviceA", "CreateUser", "1.1")
	rec2.SetSOAPAction("http://example.com/CreateUser")

	rec3 := NewSOAPRecording("/soap/serviceB", "GetUser", "1.2")
	rec3.SetSOAPAction("http://example.com/GetUser")

	rec4 := NewSOAPRecording("/soap/serviceB", "DeleteUser", "1.2")
	rec4.SetFault("SOAP-ENV:Client", "Invalid request")

	store.Add(rec1)
	store.Add(rec2)
	store.Add(rec3)
	store.Add(rec4)

	t.Run("returns all recordings with empty filter", func(t *testing.T) {
		recordings, total := store.List(SOAPRecordingFilter{})
		if total != 4 {
			t.Errorf("Expected total 4, got %d", total)
		}
		if len(recordings) != 4 {
			t.Errorf("Expected 4 recordings, got %d", len(recordings))
		}
	})

	t.Run("returns newest first", func(t *testing.T) {
		recordings, _ := store.List(SOAPRecordingFilter{})
		if len(recordings) < 2 {
			t.Fatal("Expected at least 2 recordings")
		}
		// rec4 was added last, should be first
		if recordings[0].ID != rec4.ID {
			t.Error("Expected newest recording first")
		}
		// rec1 was added first, should be last
		if recordings[3].ID != rec1.ID {
			t.Error("Expected oldest recording last")
		}
	})

	t.Run("filters by endpoint", func(t *testing.T) {
		recordings, total := store.List(SOAPRecordingFilter{Endpoint: "/soap/serviceA"})
		if total != 2 {
			t.Errorf("Expected total 2 for serviceA, got %d", total)
		}
		if len(recordings) != 2 {
			t.Errorf("Expected 2 recordings for serviceA, got %d", len(recordings))
		}
		for _, r := range recordings {
			if r.Endpoint != "/soap/serviceA" {
				t.Errorf("Expected endpoint '/soap/serviceA', got '%s'", r.Endpoint)
			}
		}
	})

	t.Run("filters by operation", func(t *testing.T) {
		recordings, total := store.List(SOAPRecordingFilter{Operation: "GetUser"})
		if total != 2 {
			t.Errorf("Expected total 2 for GetUser, got %d", total)
		}
		if len(recordings) != 2 {
			t.Errorf("Expected 2 recordings for GetUser, got %d", len(recordings))
		}
		for _, r := range recordings {
			if r.Operation != "GetUser" {
				t.Errorf("Expected operation 'GetUser', got '%s'", r.Operation)
			}
		}
	})

	t.Run("filters by soapAction", func(t *testing.T) {
		recordings, total := store.List(SOAPRecordingFilter{SOAPAction: "http://example.com/GetUser"})
		if total != 2 {
			t.Errorf("Expected total 2 for GetUser action, got %d", total)
		}
		if len(recordings) != 2 {
			t.Errorf("Expected 2 recordings, got %d", len(recordings))
		}
	})

	t.Run("filters by hasFault true", func(t *testing.T) {
		hasFault := true
		recordings, total := store.List(SOAPRecordingFilter{HasFault: &hasFault})
		if total != 1 {
			t.Errorf("Expected total 1 with fault, got %d", total)
		}
		if len(recordings) != 1 {
			t.Errorf("Expected 1 recording with fault, got %d", len(recordings))
		}
		if recordings[0].ID != rec4.ID {
			t.Error("Expected rec4 (the one with fault)")
		}
	})

	t.Run("filters by hasFault false", func(t *testing.T) {
		hasFault := false
		recordings, total := store.List(SOAPRecordingFilter{HasFault: &hasFault})
		if total != 3 {
			t.Errorf("Expected total 3 without fault, got %d", total)
		}
		if len(recordings) != 3 {
			t.Errorf("Expected 3 recordings without fault, got %d", len(recordings))
		}
	})

	t.Run("applies multiple filters", func(t *testing.T) {
		recordings, total := store.List(SOAPRecordingFilter{
			Endpoint:  "/soap/serviceA",
			Operation: "GetUser",
		})
		if total != 1 {
			t.Errorf("Expected total 1, got %d", total)
		}
		if len(recordings) != 1 {
			t.Errorf("Expected 1 recording, got %d", len(recordings))
		}
		if recordings[0].ID != rec1.ID {
			t.Error("Expected rec1")
		}
	})

	t.Run("returns empty for no matches", func(t *testing.T) {
		recordings, total := store.List(SOAPRecordingFilter{Endpoint: "/nonexistent"})
		if total != 0 {
			t.Errorf("Expected total 0, got %d", total)
		}
		if len(recordings) != 0 {
			t.Errorf("Expected 0 recordings, got %d", len(recordings))
		}
	})

	t.Run("applies pagination with limit", func(t *testing.T) {
		recordings, total := store.List(SOAPRecordingFilter{Limit: 2})
		if total != 4 {
			t.Errorf("Expected total 4, got %d", total)
		}
		if len(recordings) != 2 {
			t.Errorf("Expected 2 recordings with limit, got %d", len(recordings))
		}
	})

	t.Run("applies pagination with offset", func(t *testing.T) {
		recordings, total := store.List(SOAPRecordingFilter{Offset: 2})
		if total != 4 {
			t.Errorf("Expected total 4, got %d", total)
		}
		if len(recordings) != 2 {
			t.Errorf("Expected 2 recordings with offset, got %d", len(recordings))
		}
	})

	t.Run("applies pagination with limit and offset", func(t *testing.T) {
		recordings, total := store.List(SOAPRecordingFilter{Offset: 1, Limit: 2})
		if total != 4 {
			t.Errorf("Expected total 4, got %d", total)
		}
		if len(recordings) != 2 {
			t.Errorf("Expected 2 recordings, got %d", len(recordings))
		}
	})

	t.Run("returns empty when offset exceeds total", func(t *testing.T) {
		recordings, total := store.List(SOAPRecordingFilter{Offset: 10})
		if total != 4 {
			t.Errorf("Expected total 4, got %d", total)
		}
		if len(recordings) != 0 {
			t.Errorf("Expected 0 recordings, got %d", len(recordings))
		}
		if recordings == nil {
			t.Error("Expected empty slice, got nil")
		}
	})
}

func TestSOAPStoreDelete(t *testing.T) {
	t.Run("deletes existing recording", func(t *testing.T) {
		store := NewSOAPStore(10)
		rec := NewSOAPRecording("/soap/service", "GetUser", "1.1")
		store.Add(rec)

		err := store.Delete(rec.ID)
		if err != nil {
			t.Errorf("Failed to delete recording: %v", err)
		}
		if store.Count() != 0 {
			t.Errorf("Expected count 0, got %d", store.Count())
		}
		if store.Get(rec.ID) != nil {
			t.Error("Expected recording to be deleted")
		}
	})

	t.Run("returns ErrNotFound for missing ID", func(t *testing.T) {
		store := NewSOAPStore(10)

		err := store.Delete("nonexistent-id")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	t.Run("returns ErrNotFound for empty store", func(t *testing.T) {
		store := NewSOAPStore(10)

		err := store.Delete("any-id")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	t.Run("removes from order slice", func(t *testing.T) {
		store := NewSOAPStore(10)
		rec1 := NewSOAPRecording("/soap/service", "Op1", "1.1")
		rec2 := NewSOAPRecording("/soap/service", "Op2", "1.1")
		rec3 := NewSOAPRecording("/soap/service", "Op3", "1.1")

		store.Add(rec1)
		store.Add(rec2)
		store.Add(rec3)

		store.Delete(rec2.ID)

		if len(store.order) != 2 {
			t.Errorf("Expected order length 2, got %d", len(store.order))
		}
		for _, id := range store.order {
			if id == rec2.ID {
				t.Error("Deleted recording should not be in order slice")
			}
		}
	})

	t.Run("does not affect other recordings", func(t *testing.T) {
		store := NewSOAPStore(10)
		rec1 := NewSOAPRecording("/soap/service", "Op1", "1.1")
		rec2 := NewSOAPRecording("/soap/service", "Op2", "1.1")

		store.Add(rec1)
		store.Add(rec2)

		store.Delete(rec1.ID)

		if store.Get(rec2.ID) == nil {
			t.Error("Other recording should not be affected")
		}
		if store.Count() != 1 {
			t.Errorf("Expected count 1, got %d", store.Count())
		}
	})
}

func TestSOAPStoreClear(t *testing.T) {
	t.Run("removes all recordings and returns count", func(t *testing.T) {
		store := NewSOAPStore(10)

		for i := 0; i < 5; i++ {
			rec := NewSOAPRecording("/soap/service", "Method", "1.1")
			store.Add(rec)
		}

		count := store.Clear()
		if count != 5 {
			t.Errorf("Expected count 5, got %d", count)
		}
		if store.Count() != 0 {
			t.Errorf("Expected store count 0, got %d", store.Count())
		}
	})

	t.Run("returns zero for empty store", func(t *testing.T) {
		store := NewSOAPStore(10)

		count := store.Clear()
		if count != 0 {
			t.Errorf("Expected count 0, got %d", count)
		}
	})

	t.Run("clears order slice", func(t *testing.T) {
		store := NewSOAPStore(10)

		rec := NewSOAPRecording("/soap/service", "Method", "1.1")
		store.Add(rec)

		store.Clear()

		if len(store.order) != 0 {
			t.Errorf("Expected order length 0, got %d", len(store.order))
		}
	})

	t.Run("allows adding after clear", func(t *testing.T) {
		store := NewSOAPStore(10)

		rec1 := NewSOAPRecording("/soap/service", "Method", "1.1")
		store.Add(rec1)
		store.Clear()

		rec2 := NewSOAPRecording("/soap/service", "Method", "1.1")
		err := store.Add(rec2)
		if err != nil {
			t.Errorf("Failed to add after clear: %v", err)
		}
		if store.Count() != 1 {
			t.Errorf("Expected count 1, got %d", store.Count())
		}
	})
}

func TestSOAPStoreCount(t *testing.T) {
	t.Run("returns zero for empty store", func(t *testing.T) {
		store := NewSOAPStore(10)

		if store.Count() != 0 {
			t.Errorf("Expected count 0, got %d", store.Count())
		}
	})

	t.Run("returns correct count after adds", func(t *testing.T) {
		store := NewSOAPStore(10)

		for i := 0; i < 5; i++ {
			rec := NewSOAPRecording("/soap/service", "Method", "1.1")
			store.Add(rec)
		}

		if store.Count() != 5 {
			t.Errorf("Expected count 5, got %d", store.Count())
		}
	})

	t.Run("returns correct count after delete", func(t *testing.T) {
		store := NewSOAPStore(10)

		rec1 := NewSOAPRecording("/soap/service", "Method", "1.1")
		rec2 := NewSOAPRecording("/soap/service", "Method", "1.1")
		store.Add(rec1)
		store.Add(rec2)

		store.Delete(rec1.ID)

		if store.Count() != 1 {
			t.Errorf("Expected count 1, got %d", store.Count())
		}
	})

	t.Run("respects max size", func(t *testing.T) {
		store := NewSOAPStore(3)

		for i := 0; i < 10; i++ {
			rec := NewSOAPRecording("/soap/service", "Method", "1.1")
			store.Add(rec)
		}

		if store.Count() != 3 {
			t.Errorf("Expected count 3 (max size), got %d", store.Count())
		}
	})
}

func TestSOAPStoreListByEndpoint(t *testing.T) {
	store := NewSOAPStore(100)

	rec1 := NewSOAPRecording("/soap/serviceA", "GetUser", "1.1")
	rec2 := NewSOAPRecording("/soap/serviceA", "CreateUser", "1.1")
	rec3 := NewSOAPRecording("/soap/serviceB", "GetUser", "1.2")

	store.Add(rec1)
	store.Add(rec2)
	store.Add(rec3)

	t.Run("returns recordings for matching endpoint", func(t *testing.T) {
		recordings := store.ListByEndpoint("/soap/serviceA")
		if len(recordings) != 2 {
			t.Errorf("Expected 2 recordings, got %d", len(recordings))
		}
		for _, r := range recordings {
			if r.Endpoint != "/soap/serviceA" {
				t.Errorf("Expected endpoint '/soap/serviceA', got '%s'", r.Endpoint)
			}
		}
	})

	t.Run("returns empty for no matching endpoint", func(t *testing.T) {
		recordings := store.ListByEndpoint("/nonexistent")
		if len(recordings) != 0 {
			t.Errorf("Expected 0 recordings, got %d", len(recordings))
		}
	})

	t.Run("returns empty for empty store", func(t *testing.T) {
		emptyStore := NewSOAPStore(10)
		recordings := emptyStore.ListByEndpoint("/soap/serviceA")
		if len(recordings) != 0 {
			t.Errorf("Expected 0 recordings, got %d", len(recordings))
		}
	})
}

func TestSOAPStoreListByOperation(t *testing.T) {
	store := NewSOAPStore(100)

	rec1 := NewSOAPRecording("/soap/serviceA", "GetUser", "1.1")
	rec2 := NewSOAPRecording("/soap/serviceB", "GetUser", "1.2")
	rec3 := NewSOAPRecording("/soap/serviceA", "CreateUser", "1.1")

	store.Add(rec1)
	store.Add(rec2)
	store.Add(rec3)

	t.Run("returns recordings for matching operation", func(t *testing.T) {
		recordings := store.ListByOperation("GetUser")
		if len(recordings) != 2 {
			t.Errorf("Expected 2 recordings, got %d", len(recordings))
		}
		for _, r := range recordings {
			if r.Operation != "GetUser" {
				t.Errorf("Expected operation 'GetUser', got '%s'", r.Operation)
			}
		}
	})

	t.Run("returns empty for no matching operation", func(t *testing.T) {
		recordings := store.ListByOperation("NonexistentOp")
		if len(recordings) != 0 {
			t.Errorf("Expected 0 recordings, got %d", len(recordings))
		}
	})

	t.Run("returns empty for empty store", func(t *testing.T) {
		emptyStore := NewSOAPStore(10)
		recordings := emptyStore.ListByOperation("GetUser")
		if len(recordings) != 0 {
			t.Errorf("Expected 0 recordings, got %d", len(recordings))
		}
	})
}

func TestSOAPStoreStats(t *testing.T) {
	t.Run("returns correct statistics", func(t *testing.T) {
		store := NewSOAPStore(100)

		rec1 := NewSOAPRecording("/soap/serviceA", "GetUser", "1.1")
		rec2 := NewSOAPRecording("/soap/serviceA", "CreateUser", "1.1")
		rec3 := NewSOAPRecording("/soap/serviceB", "GetUser", "1.2")
		rec4 := NewSOAPRecording("/soap/serviceB", "DeleteUser", "1.2")
		rec4.SetFault("SOAP-ENV:Client", "Invalid request")

		store.Add(rec1)
		store.Add(rec2)
		store.Add(rec3)
		store.Add(rec4)

		stats := store.Stats()

		if stats.TotalRecordings != 4 {
			t.Errorf("Expected 4 total recordings, got %d", stats.TotalRecordings)
		}
		if stats.ByEndpoint["/soap/serviceA"] != 2 {
			t.Errorf("Expected 2 recordings for serviceA, got %d", stats.ByEndpoint["/soap/serviceA"])
		}
		if stats.ByEndpoint["/soap/serviceB"] != 2 {
			t.Errorf("Expected 2 recordings for serviceB, got %d", stats.ByEndpoint["/soap/serviceB"])
		}
		if stats.ByOperation["GetUser"] != 2 {
			t.Errorf("Expected 2 recordings for GetUser, got %d", stats.ByOperation["GetUser"])
		}
		if stats.ByOperation["CreateUser"] != 1 {
			t.Errorf("Expected 1 recording for CreateUser, got %d", stats.ByOperation["CreateUser"])
		}
		if stats.ByOperation["DeleteUser"] != 1 {
			t.Errorf("Expected 1 recording for DeleteUser, got %d", stats.ByOperation["DeleteUser"])
		}
		if stats.FaultCount != 1 {
			t.Errorf("Expected 1 fault, got %d", stats.FaultCount)
		}
	})

	t.Run("returns empty stats for empty store", func(t *testing.T) {
		store := NewSOAPStore(10)
		stats := store.Stats()

		if stats.TotalRecordings != 0 {
			t.Errorf("Expected 0 total recordings, got %d", stats.TotalRecordings)
		}
		if len(stats.ByEndpoint) != 0 {
			t.Errorf("Expected empty ByEndpoint, got %d entries", len(stats.ByEndpoint))
		}
		if len(stats.ByOperation) != 0 {
			t.Errorf("Expected empty ByOperation, got %d entries", len(stats.ByOperation))
		}
		if stats.FaultCount != 0 {
			t.Errorf("Expected 0 faults, got %d", stats.FaultCount)
		}
		if stats.OldestTimestamp != nil {
			t.Error("Expected nil OldestTimestamp for empty store")
		}
		if stats.NewestTimestamp != nil {
			t.Error("Expected nil NewestTimestamp for empty store")
		}
	})

	t.Run("tracks timestamps correctly", func(t *testing.T) {
		store := NewSOAPStore(100)

		rec1 := NewSOAPRecording("/soap/service", "Op1", "1.1")
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
		rec2 := NewSOAPRecording("/soap/service", "Op2", "1.1")

		store.Add(rec1)
		store.Add(rec2)

		stats := store.Stats()

		if stats.OldestTimestamp == nil {
			t.Fatal("Expected OldestTimestamp to be set")
		}
		if stats.NewestTimestamp == nil {
			t.Fatal("Expected NewestTimestamp to be set")
		}
		if !stats.OldestTimestamp.Before(*stats.NewestTimestamp) && !stats.OldestTimestamp.Equal(*stats.NewestTimestamp) {
			t.Error("Expected OldestTimestamp to be before or equal to NewestTimestamp")
		}
	})

	t.Run("counts multiple faults", func(t *testing.T) {
		store := NewSOAPStore(100)

		rec1 := NewSOAPRecording("/soap/service", "Op1", "1.1")
		rec1.SetFault("SOAP-ENV:Client", "Error 1")

		rec2 := NewSOAPRecording("/soap/service", "Op2", "1.1")
		rec2.SetFault("SOAP-ENV:Server", "Error 2")

		rec3 := NewSOAPRecording("/soap/service", "Op3", "1.1")
		// No fault

		store.Add(rec1)
		store.Add(rec2)
		store.Add(rec3)

		stats := store.Stats()

		if stats.FaultCount != 2 {
			t.Errorf("Expected 2 faults, got %d", stats.FaultCount)
		}
	})
}

func TestSOAPStoreExport(t *testing.T) {
	t.Run("exports recordings as JSON", func(t *testing.T) {
		store := NewSOAPStore(10)

		rec1 := NewSOAPRecording("/soap/service", "GetUser", "1.1")
		rec1.SetRequestBody("<soap:Envelope>...</soap:Envelope>")
		rec2 := NewSOAPRecording("/soap/service", "CreateUser", "1.1")

		store.Add(rec1)
		store.Add(rec2)

		data, err := store.Export()
		if err != nil {
			t.Fatalf("Failed to export: %v", err)
		}
		if len(data) == 0 {
			t.Error("Expected non-empty export data")
		}
		// Basic check that it's valid JSON array
		if data[0] != '[' || data[len(data)-1] != ']' {
			t.Error("Expected JSON array")
		}
	})

	t.Run("exports empty array for empty store", func(t *testing.T) {
		store := NewSOAPStore(10)

		data, err := store.Export()
		if err != nil {
			t.Fatalf("Failed to export: %v", err)
		}
		if string(data) != "[]" {
			t.Errorf("Expected '[]', got '%s'", string(data))
		}
	})

	t.Run("maintains insertion order in export", func(t *testing.T) {
		store := NewSOAPStore(10)

		rec1 := NewSOAPRecording("/soap/service", "Op1", "1.1")
		rec2 := NewSOAPRecording("/soap/service", "Op2", "1.1")
		rec3 := NewSOAPRecording("/soap/service", "Op3", "1.1")

		store.Add(rec1)
		store.Add(rec2)
		store.Add(rec3)

		data, err := store.Export()
		if err != nil {
			t.Fatalf("Failed to export: %v", err)
		}

		// Verify order by checking string positions
		pos1 := findSubstringPosition(string(data), rec1.ID)
		pos2 := findSubstringPosition(string(data), rec2.ID)
		pos3 := findSubstringPosition(string(data), rec3.ID)

		if pos1 > pos2 || pos2 > pos3 {
			t.Error("Expected recordings in insertion order")
		}
	})
}

// Helper function to find substring position
func findSubstringPosition(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func TestSOAPStoreConcurrency(t *testing.T) {
	t.Run("handles concurrent adds", func(t *testing.T) {
		store := NewSOAPStore(1000)
		done := make(chan bool)

		for i := 0; i < 10; i++ {
			go func() {
				for j := 0; j < 100; j++ {
					rec := NewSOAPRecording("/soap/service", "Method", "1.1")
					store.Add(rec)
				}
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}

		if store.Count() != 1000 {
			t.Errorf("Expected count 1000, got %d", store.Count())
		}
	})

	t.Run("handles concurrent reads and writes", func(t *testing.T) {
		store := NewSOAPStore(100)
		done := make(chan bool)

		// Add initial recordings
		for i := 0; i < 50; i++ {
			rec := NewSOAPRecording("/soap/service", "Method", "1.1")
			store.Add(rec)
		}

		// Concurrent reads
		go func() {
			for i := 0; i < 100; i++ {
				store.List(SOAPRecordingFilter{})
				store.Count()
				store.Stats()
			}
			done <- true
		}()

		// Concurrent writes
		go func() {
			for i := 0; i < 50; i++ {
				rec := NewSOAPRecording("/soap/service", "Method", "1.1")
				store.Add(rec)
			}
			done <- true
		}()

		<-done
		<-done

		// Just verify no panics/deadlocks occurred
		if store.Count() == 0 {
			t.Error("Expected recordings in store")
		}
	})
}
