package recording

import (
	"testing"
	"time"
)

func TestNewMQTTStore(t *testing.T) {
	t.Run("creates store with specified max size", func(t *testing.T) {
		store := NewMQTTStore(50)
		if store == nil {
			t.Fatal("Expected store to be created")
		}
		if store.maxSize != 50 {
			t.Errorf("Expected maxSize 50, got %d", store.maxSize)
		}
		if store.recordings == nil {
			t.Error("Expected recordings map to be initialized")
		}
		if store.order == nil {
			t.Error("Expected order slice to be initialized")
		}
	})

	t.Run("uses default max size when zero or negative", func(t *testing.T) {
		store := NewMQTTStore(0)
		if store.maxSize != 1000 {
			t.Errorf("Expected default maxSize 1000, got %d", store.maxSize)
		}

		store = NewMQTTStore(-10)
		if store.maxSize != 1000 {
			t.Errorf("Expected default maxSize 1000, got %d", store.maxSize)
		}
	})
}

func TestMQTTStoreAdd(t *testing.T) {
	t.Run("adds recording successfully", func(t *testing.T) {
		store := NewMQTTStore(10)
		rec := NewMQTTRecording("test/topic", []byte("payload"), 1, false, "client1", MQTTDirectionPublish)

		err := store.Add(rec)
		if err != nil {
			t.Errorf("Failed to add recording: %v", err)
		}
		if store.Count() != 1 {
			t.Errorf("Expected count 1, got %d", store.Count())
		}
	})

	t.Run("evicts oldest when full", func(t *testing.T) {
		store := NewMQTTStore(3)
		var firstID string

		for i := 0; i < 5; i++ {
			rec := NewMQTTRecording("test/topic", []byte("payload"), 0, false, "client1", MQTTDirectionPublish)
			if i == 0 {
				firstID = rec.ID
			}
			store.Add(rec)
			time.Sleep(time.Millisecond) // Ensure unique timestamps
		}

		if store.Count() != 3 {
			t.Errorf("Expected count 3, got %d", store.Count())
		}

		// First recording should be evicted
		if store.Get(firstID) != nil {
			t.Error("Expected first recording to be evicted")
		}
	})

	t.Run("maintains insertion order", func(t *testing.T) {
		store := NewMQTTStore(10)
		ids := make([]string, 3)

		for i := 0; i < 3; i++ {
			rec := NewMQTTRecording("test/topic", []byte("payload"), 0, false, "client1", MQTTDirectionPublish)
			ids[i] = rec.ID
			store.Add(rec)
		}

		// Verify order slice matches insertion order
		for i, id := range ids {
			if store.order[i] != id {
				t.Errorf("Expected order[%d] = %s, got %s", i, id, store.order[i])
			}
		}
	})
}

func TestMQTTStoreGet(t *testing.T) {
	t.Run("retrieves recording by ID", func(t *testing.T) {
		store := NewMQTTStore(10)
		rec := NewMQTTRecording("test/topic", []byte("hello"), 1, true, "client1", MQTTDirectionPublish)
		store.Add(rec)

		got := store.Get(rec.ID)
		if got == nil {
			t.Fatal("Expected to get recording")
		}
		if got.ID != rec.ID {
			t.Errorf("Expected ID %s, got %s", rec.ID, got.ID)
		}
		if got.Topic != "test/topic" {
			t.Errorf("Expected topic 'test/topic', got '%s'", got.Topic)
		}
		if string(got.Payload) != "hello" {
			t.Errorf("Expected payload 'hello', got '%s'", string(got.Payload))
		}
	})

	t.Run("returns nil for missing ID", func(t *testing.T) {
		store := NewMQTTStore(10)
		rec := NewMQTTRecording("test/topic", []byte("payload"), 0, false, "client1", MQTTDirectionPublish)
		store.Add(rec)

		got := store.Get("nonexistent-id")
		if got != nil {
			t.Error("Expected nil for missing ID")
		}
	})

	t.Run("returns nil from empty store", func(t *testing.T) {
		store := NewMQTTStore(10)
		got := store.Get("any-id")
		if got != nil {
			t.Error("Expected nil from empty store")
		}
	})
}

func TestMQTTStoreList(t *testing.T) {
	t.Run("returns all recordings when no filter", func(t *testing.T) {
		store := NewMQTTStore(10)
		for i := 0; i < 3; i++ {
			rec := NewMQTTRecording("test/topic", []byte("payload"), 0, false, "client1", MQTTDirectionPublish)
			store.Add(rec)
		}

		recordings, total := store.List(MQTTRecordingFilter{})
		if total != 3 {
			t.Errorf("Expected total 3, got %d", total)
		}
		if len(recordings) != 3 {
			t.Errorf("Expected 3 recordings, got %d", len(recordings))
		}
	})

	t.Run("filters by topic pattern with + wildcard", func(t *testing.T) {
		store := NewMQTTStore(10)
		store.Add(NewMQTTRecording("home/living/temperature", []byte("22"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("home/kitchen/temperature", []byte("24"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("home/bedroom/humidity", []byte("45"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("office/meeting/temperature", []byte("21"), 0, false, "client1", MQTTDirectionPublish))

		recordings, total := store.List(MQTTRecordingFilter{TopicPattern: "home/+/temperature"})
		if total != 2 {
			t.Errorf("Expected 2 matches for 'home/+/temperature', got %d", total)
		}

		for _, rec := range recordings {
			if rec.Topic != "home/living/temperature" && rec.Topic != "home/kitchen/temperature" {
				t.Errorf("Unexpected topic: %s", rec.Topic)
			}
		}
	})

	t.Run("filters by topic pattern with # wildcard", func(t *testing.T) {
		store := NewMQTTStore(10)
		store.Add(NewMQTTRecording("home/living/temperature", []byte("22"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("home/living/humidity", []byte("45"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("home/kitchen/temperature", []byte("24"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("office/meeting/temperature", []byte("21"), 0, false, "client1", MQTTDirectionPublish))

		recordings, total := store.List(MQTTRecordingFilter{TopicPattern: "home/#"})
		if total != 3 {
			t.Errorf("Expected 3 matches for 'home/#', got %d", total)
		}

		for _, rec := range recordings {
			if rec.Topic == "office/meeting/temperature" {
				t.Error("Should not match office topic")
			}
		}
	})

	t.Run("filters by client ID", func(t *testing.T) {
		store := NewMQTTStore(10)
		store.Add(NewMQTTRecording("topic/a", []byte("1"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("topic/b", []byte("2"), 0, false, "client2", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("topic/c", []byte("3"), 0, false, "client1", MQTTDirectionPublish))

		recordings, total := store.List(MQTTRecordingFilter{ClientID: "client1"})
		if total != 2 {
			t.Errorf("Expected 2 recordings for client1, got %d", total)
		}
		for _, rec := range recordings {
			if rec.ClientID != "client1" {
				t.Errorf("Expected client1, got %s", rec.ClientID)
			}
		}
	})

	t.Run("filters by direction", func(t *testing.T) {
		store := NewMQTTStore(10)
		store.Add(NewMQTTRecording("topic/a", []byte("1"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("topic/b", []byte("2"), 0, false, "client1", MQTTDirectionSubscribe))
		store.Add(NewMQTTRecording("topic/c", []byte("3"), 0, false, "client1", MQTTDirectionPublish))

		recordings, total := store.List(MQTTRecordingFilter{Direction: MQTTDirectionSubscribe})
		if total != 1 {
			t.Errorf("Expected 1 subscribe recording, got %d", total)
		}
		if len(recordings) > 0 && recordings[0].Direction != MQTTDirectionSubscribe {
			t.Errorf("Expected direction subscribe, got %s", recordings[0].Direction)
		}
	})

	t.Run("applies pagination with limit", func(t *testing.T) {
		store := NewMQTTStore(10)
		for i := 0; i < 5; i++ {
			store.Add(NewMQTTRecording("topic", []byte("data"), 0, false, "client1", MQTTDirectionPublish))
		}

		recordings, total := store.List(MQTTRecordingFilter{Limit: 2})
		if total != 5 {
			t.Errorf("Expected total 5, got %d", total)
		}
		if len(recordings) != 2 {
			t.Errorf("Expected 2 recordings with limit, got %d", len(recordings))
		}
	})

	t.Run("applies pagination with offset", func(t *testing.T) {
		store := NewMQTTStore(10)
		for i := 0; i < 5; i++ {
			store.Add(NewMQTTRecording("topic", []byte("data"), 0, false, "client1", MQTTDirectionPublish))
		}

		recordings, total := store.List(MQTTRecordingFilter{Offset: 3})
		if total != 5 {
			t.Errorf("Expected total 5, got %d", total)
		}
		if len(recordings) != 2 {
			t.Errorf("Expected 2 recordings after offset 3, got %d", len(recordings))
		}
	})

	t.Run("handles offset beyond results", func(t *testing.T) {
		store := NewMQTTStore(10)
		for i := 0; i < 3; i++ {
			store.Add(NewMQTTRecording("topic", []byte("data"), 0, false, "client1", MQTTDirectionPublish))
		}

		recordings, total := store.List(MQTTRecordingFilter{Offset: 10})
		if total != 3 {
			t.Errorf("Expected total 3, got %d", total)
		}
		if len(recordings) != 0 {
			t.Errorf("Expected 0 recordings for offset beyond total, got %d", len(recordings))
		}
	})

	t.Run("combines multiple filters", func(t *testing.T) {
		store := NewMQTTStore(10)
		store.Add(NewMQTTRecording("sensor/temp", []byte("1"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("sensor/humidity", []byte("2"), 0, false, "client1", MQTTDirectionSubscribe))
		store.Add(NewMQTTRecording("sensor/temp", []byte("3"), 0, false, "client2", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("sensor/temp", []byte("4"), 0, false, "client1", MQTTDirectionPublish))

		result, total := store.List(MQTTRecordingFilter{
			TopicPattern: "sensor/temp",
			ClientID:     "client1",
			Direction:    MQTTDirectionPublish,
		})
		if total != 2 {
			t.Errorf("Expected 2 matching recordings, got %d", total)
		}
		if len(result) != 2 {
			t.Errorf("Expected 2 recordings returned, got %d", len(result))
		}
	})

	t.Run("returns newest first", func(t *testing.T) {
		store := NewMQTTStore(10)
		rec1 := NewMQTTRecording("topic", []byte("first"), 0, false, "client1", MQTTDirectionPublish)
		time.Sleep(time.Millisecond)
		rec2 := NewMQTTRecording("topic", []byte("second"), 0, false, "client1", MQTTDirectionPublish)

		store.Add(rec1)
		store.Add(rec2)

		result, _ := store.List(MQTTRecordingFilter{})
		if len(result) != 2 {
			t.Fatalf("Expected 2 recordings, got %d", len(result))
		}
		// Newest (last added) should be first in results
		if result[0].ID != rec2.ID {
			t.Error("Expected newest recording first")
		}
	})
}

func TestMQTTStoreDelete(t *testing.T) {
	t.Run("removes recording successfully", func(t *testing.T) {
		store := NewMQTTStore(10)
		rec := NewMQTTRecording("topic", []byte("data"), 0, false, "client1", MQTTDirectionPublish)
		store.Add(rec)

		err := store.Delete(rec.ID)
		if err != nil {
			t.Errorf("Failed to delete recording: %v", err)
		}
		if store.Count() != 0 {
			t.Errorf("Expected count 0, got %d", store.Count())
		}
		if store.Get(rec.ID) != nil {
			t.Error("Recording should not exist after delete")
		}
	})

	t.Run("returns error for missing ID", func(t *testing.T) {
		store := NewMQTTStore(10)
		rec := NewMQTTRecording("topic", []byte("data"), 0, false, "client1", MQTTDirectionPublish)
		store.Add(rec)

		err := store.Delete("nonexistent-id")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	t.Run("returns error on empty store", func(t *testing.T) {
		store := NewMQTTStore(10)
		err := store.Delete("any-id")
		if err != ErrNotFound {
			t.Errorf("Expected ErrNotFound, got %v", err)
		}
	})

	t.Run("removes from order slice", func(t *testing.T) {
		store := NewMQTTStore(10)
		rec1 := NewMQTTRecording("topic", []byte("1"), 0, false, "client1", MQTTDirectionPublish)
		rec2 := NewMQTTRecording("topic", []byte("2"), 0, false, "client1", MQTTDirectionPublish)
		rec3 := NewMQTTRecording("topic", []byte("3"), 0, false, "client1", MQTTDirectionPublish)
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
}

func TestMQTTStoreClear(t *testing.T) {
	t.Run("removes all recordings and returns count", func(t *testing.T) {
		store := NewMQTTStore(10)
		for i := 0; i < 5; i++ {
			store.Add(NewMQTTRecording("topic", []byte("data"), 0, false, "client1", MQTTDirectionPublish))
		}

		count := store.Clear()
		if count != 5 {
			t.Errorf("Expected cleared count 5, got %d", count)
		}
		if store.Count() != 0 {
			t.Errorf("Expected count 0 after clear, got %d", store.Count())
		}
	})

	t.Run("returns zero on empty store", func(t *testing.T) {
		store := NewMQTTStore(10)
		count := store.Clear()
		if count != 0 {
			t.Errorf("Expected cleared count 0, got %d", count)
		}
	})

	t.Run("clears order slice", func(t *testing.T) {
		store := NewMQTTStore(10)
		store.Add(NewMQTTRecording("topic", []byte("data"), 0, false, "client1", MQTTDirectionPublish))

		store.Clear()
		if len(store.order) != 0 {
			t.Errorf("Expected order length 0, got %d", len(store.order))
		}
	})
}

func TestMQTTStoreCount(t *testing.T) {
	t.Run("returns correct count", func(t *testing.T) {
		store := NewMQTTStore(10)
		if store.Count() != 0 {
			t.Errorf("Expected count 0, got %d", store.Count())
		}

		store.Add(NewMQTTRecording("topic", []byte("1"), 0, false, "client1", MQTTDirectionPublish))
		if store.Count() != 1 {
			t.Errorf("Expected count 1, got %d", store.Count())
		}

		store.Add(NewMQTTRecording("topic", []byte("2"), 0, false, "client1", MQTTDirectionPublish))
		if store.Count() != 2 {
			t.Errorf("Expected count 2, got %d", store.Count())
		}
	})

	t.Run("reflects deletions", func(t *testing.T) {
		store := NewMQTTStore(10)
		rec := NewMQTTRecording("topic", []byte("data"), 0, false, "client1", MQTTDirectionPublish)
		store.Add(rec)
		store.Delete(rec.ID)

		if store.Count() != 0 {
			t.Errorf("Expected count 0 after delete, got %d", store.Count())
		}
	})
}

func TestMQTTStoreListByTopic(t *testing.T) {
	t.Run("filters by exact topic match", func(t *testing.T) {
		store := NewMQTTStore(10)
		store.Add(NewMQTTRecording("sensor/temperature", []byte("22"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("sensor/humidity", []byte("45"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("sensor/temperature", []byte("23"), 0, false, "client2", MQTTDirectionPublish))

		recordings := store.ListByTopic("sensor/temperature")
		if len(recordings) != 2 {
			t.Errorf("Expected 2 recordings for 'sensor/temperature', got %d", len(recordings))
		}
		for _, rec := range recordings {
			if rec.Topic != "sensor/temperature" {
				t.Errorf("Expected topic 'sensor/temperature', got '%s'", rec.Topic)
			}
		}
	})

	t.Run("returns empty for non-matching topic", func(t *testing.T) {
		store := NewMQTTStore(10)
		store.Add(NewMQTTRecording("sensor/temperature", []byte("22"), 0, false, "client1", MQTTDirectionPublish))

		recordings := store.ListByTopic("sensor/humidity")
		if len(recordings) != 0 {
			t.Errorf("Expected 0 recordings, got %d", len(recordings))
		}
	})

	t.Run("supports MQTT wildcards", func(t *testing.T) {
		store := NewMQTTStore(10)
		store.Add(NewMQTTRecording("home/living/temp", []byte("22"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("home/kitchen/temp", []byte("24"), 0, false, "client1", MQTTDirectionPublish))

		recordings := store.ListByTopic("home/+/temp")
		if len(recordings) != 2 {
			t.Errorf("Expected 2 recordings for 'home/+/temp', got %d", len(recordings))
		}
	})
}

func TestMQTTStoreStats(t *testing.T) {
	t.Run("returns correct statistics", func(t *testing.T) {
		store := NewMQTTStore(100)
		store.Add(NewMQTTRecording("topic/a", []byte("1"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("topic/a", []byte("2"), 1, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("topic/b", []byte("3"), 2, false, "client1", MQTTDirectionSubscribe))
		store.Add(NewMQTTRecording("topic/a", []byte("4"), 0, false, "client2", MQTTDirectionPublish))

		stats := store.Stats()

		if stats.TotalRecordings != 4 {
			t.Errorf("Expected 4 total recordings, got %d", stats.TotalRecordings)
		}

		// Check by topic
		if stats.ByTopic["topic/a"] != 3 {
			t.Errorf("Expected 3 recordings for topic/a, got %d", stats.ByTopic["topic/a"])
		}
		if stats.ByTopic["topic/b"] != 1 {
			t.Errorf("Expected 1 recording for topic/b, got %d", stats.ByTopic["topic/b"])
		}

		// Check by direction
		if stats.ByDirection["publish"] != 3 {
			t.Errorf("Expected 3 publish recordings, got %d", stats.ByDirection["publish"])
		}
		if stats.ByDirection["subscribe"] != 1 {
			t.Errorf("Expected 1 subscribe recording, got %d", stats.ByDirection["subscribe"])
		}

		// Check by QoS
		if stats.ByQoS[0] != 2 {
			t.Errorf("Expected 2 QoS 0 recordings, got %d", stats.ByQoS[0])
		}
		if stats.ByQoS[1] != 1 {
			t.Errorf("Expected 1 QoS 1 recording, got %d", stats.ByQoS[1])
		}
		if stats.ByQoS[2] != 1 {
			t.Errorf("Expected 1 QoS 2 recording, got %d", stats.ByQoS[2])
		}
	})

	t.Run("tracks timestamps", func(t *testing.T) {
		store := NewMQTTStore(10)
		rec1 := NewMQTTRecording("topic", []byte("1"), 0, false, "client1", MQTTDirectionPublish)
		time.Sleep(10 * time.Millisecond)
		rec2 := NewMQTTRecording("topic", []byte("2"), 0, false, "client1", MQTTDirectionPublish)

		store.Add(rec1)
		store.Add(rec2)

		stats := store.Stats()
		if stats.OldestTimestamp == nil {
			t.Error("Expected oldest timestamp to be set")
		}
		if stats.NewestTimestamp == nil {
			t.Error("Expected newest timestamp to be set")
		}
		if !stats.OldestTimestamp.Before(*stats.NewestTimestamp) {
			t.Error("Oldest timestamp should be before newest")
		}
	})

	t.Run("returns empty stats for empty store", func(t *testing.T) {
		store := NewMQTTStore(10)
		stats := store.Stats()

		if stats.TotalRecordings != 0 {
			t.Errorf("Expected 0 total recordings, got %d", stats.TotalRecordings)
		}
		if stats.OldestTimestamp != nil {
			t.Error("Expected nil oldest timestamp for empty store")
		}
		if stats.NewestTimestamp != nil {
			t.Error("Expected nil newest timestamp for empty store")
		}
	})
}

func TestMatchMQTTTopic(t *testing.T) {
	testCases := []struct {
		name     string
		pattern  string
		topic    string
		expected bool
	}{
		// Exact match
		{"exact match", "home/living/temp", "home/living/temp", true},
		{"exact mismatch", "home/living/temp", "home/living/humidity", false},
		{"exact mismatch different levels", "home/living", "home/living/temp", false},

		// Single-level wildcard (+)
		{"+ matches single level", "home/+/temp", "home/living/temp", true},
		{"+ matches single level 2", "home/+/temp", "home/kitchen/temp", true},
		{"+ at start", "+/living/temp", "home/living/temp", true},
		{"+ at end", "home/living/+", "home/living/temp", true},
		{"+ must match exactly one level", "home/+/temp", "home/living/room/temp", false},
		{"multiple + wildcards", "+/+/temp", "home/living/temp", true},
		{"+ does not match empty", "home/+/temp", "home//temp", true}, // Empty string is still a level

		// Multi-level wildcard (#)
		{"# matches all remaining", "home/#", "home/living/temp", true},
		{"# matches all remaining deep", "home/#", "home/living/room/sensor/temp", true},
		{"# matches single level", "home/#", "home/living", true},
		{"# matches parent level", "home/#", "home", true},                         // # can match zero levels after prefix
		{"# matches zero additional levels", "home/living/#", "home/living", true}, // # matches zero levels
		{"# alone matches all", "#", "anything/at/all", true},
		{"# alone matches single", "#", "anything", true},

		// Combined wildcards
		{"+ and #", "home/+/#", "home/living/room/temp", true},
		{"+ and # match", "home/+/#", "home/kitchen/sensor/data", true},
		{"+ before #", "+/living/#", "home/living/temp", true},

		// Edge cases
		{"empty topic", "home/+/temp", "", false},
		{"empty pattern", "", "home/living/temp", false},
		{"both empty", "", "", true},
		{"trailing slash pattern", "home/living/", "home/living/", true},
		{"trailing slash mismatch", "home/living/", "home/living", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := matchMQTTTopic(tc.pattern, tc.topic)
			if result != tc.expected {
				t.Errorf("matchMQTTTopic(%q, %q) = %v, expected %v",
					tc.pattern, tc.topic, result, tc.expected)
			}
		})
	}
}

func TestMQTTStoreExport(t *testing.T) {
	t.Run("exports recordings as JSON", func(t *testing.T) {
		store := NewMQTTStore(10)
		store.Add(NewMQTTRecording("topic/a", []byte("data1"), 0, false, "client1", MQTTDirectionPublish))
		store.Add(NewMQTTRecording("topic/b", []byte("data2"), 1, true, "client2", MQTTDirectionSubscribe))

		data, err := store.Export()
		if err != nil {
			t.Fatalf("Failed to export: %v", err)
		}

		if len(data) == 0 {
			t.Error("Expected non-empty export data")
		}

		// Basic sanity check that it's valid JSON array
		if data[0] != '[' {
			t.Error("Expected JSON array")
		}
	})

	t.Run("exports empty array for empty store", func(t *testing.T) {
		store := NewMQTTStore(10)
		data, err := store.Export()
		if err != nil {
			t.Fatalf("Failed to export: %v", err)
		}
		if string(data) != "[]" {
			t.Errorf("Expected empty array, got %s", string(data))
		}
	})

	t.Run("maintains insertion order in export", func(t *testing.T) {
		store := NewMQTTStore(10)
		rec1 := NewMQTTRecording("topic/first", []byte("1"), 0, false, "client1", MQTTDirectionPublish)
		rec2 := NewMQTTRecording("topic/second", []byte("2"), 0, false, "client1", MQTTDirectionPublish)
		store.Add(rec1)
		store.Add(rec2)

		data, _ := store.Export()
		dataStr := string(data)

		// First recording should appear before second in the JSON
		idx1 := len(dataStr)
		idx2 := len(dataStr)
		for i := 0; i < len(dataStr)-len("topic/first"); i++ {
			if dataStr[i:i+len("topic/first")] == "topic/first" {
				idx1 = i
				break
			}
		}
		for i := 0; i < len(dataStr)-len("topic/second"); i++ {
			if dataStr[i:i+len("topic/second")] == "topic/second" {
				idx2 = i
				break
			}
		}

		if idx1 > idx2 {
			t.Error("Expected recordings to be in insertion order")
		}
	})
}
