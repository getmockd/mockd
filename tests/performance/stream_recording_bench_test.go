package performance

import (
	"os"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/recording"
)

// BenchmarkRecordingFrameAppend measures frame append performance.
// Target: <1ms per frame for sustained recording
func BenchmarkRecordingFrameAppend(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-frame-append-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    1024 * 1024 * 1024, // 1GB
		WarnPercent: 80,
	})
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}

	hook, err := recording.NewFileStoreWebSocketHook(store, recording.RecordingMetadata{
		Path: "/bench/ws",
	})
	if err != nil {
		b.Fatalf("failed to create hook: %v", err)
	}

	startTime := time.Now()
	frameData := []byte(`{"type":"benchmark","data":"test message for benchmarking frame append performance"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame := recording.NewWebSocketFrame(
			int64(i+1),
			startTime,
			recording.DirectionServerToClient,
			recording.MessageTypeText,
			frameData,
		)
		if err := hook.OnFrame(frame); err != nil {
			b.Fatalf("failed to append frame: %v", err)
		}
	}
	b.StopTimer()

	hook.OnComplete()
}

// BenchmarkRecordingSSEEventAppend measures SSE event append performance.
func BenchmarkRecordingSSEEventAppend(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-sse-append-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    1024 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}

	hook, err := recording.NewFileStoreSSEHook(store, recording.RecordingMetadata{
		Path: "/bench/sse",
	})
	if err != nil {
		b.Fatalf("failed to create hook: %v", err)
	}

	hook.OnStreamStart()
	startTime := time.Now()
	eventData := `{"type":"benchmark","data":"test message for benchmarking SSE event append"}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		event := recording.NewSSEEvent(
			int64(i+1),
			startTime,
			"message",
			eventData,
			"",
			nil,
		)
		if err := hook.OnFrame(event); err != nil {
			b.Fatalf("failed to append event: %v", err)
		}
	}
	b.StopTimer()

	hook.OnStreamEnd()
	hook.OnComplete()
}

// BenchmarkRecordingComplete measures recording completion (save to disk).
func BenchmarkRecordingComplete(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-complete-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    1024 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		hook, _ := recording.NewFileStoreWebSocketHook(store, recording.RecordingMetadata{
			Path: "/bench/complete",
		})

		startTime := time.Now()
		// Add 100 frames
		for j := 0; j < 100; j++ {
			frame := recording.NewWebSocketFrame(
				int64(j+1),
				startTime,
				recording.DirectionServerToClient,
				recording.MessageTypeText,
				[]byte(`{"i":`+string(rune('0'+j%10))+`}`),
			)
			hook.OnFrame(frame)
		}

		b.StartTimer()
		hook.OnComplete()
	}
}

// BenchmarkRecordingLoad measures loading a recording from disk.
func BenchmarkRecordingLoad(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-load-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    1024 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}

	// Create a recording with 1000 frames
	hook, _ := recording.NewFileStoreWebSocketHook(store, recording.RecordingMetadata{
		Path: "/bench/load",
	})
	startTime := time.Now()
	for j := 0; j < 1000; j++ {
		frame := recording.NewWebSocketFrame(
			int64(j+1),
			startTime,
			recording.DirectionServerToClient,
			recording.MessageTypeText,
			[]byte(`{"index":`+string(rune('0'+j%10))+`,"data":"benchmark data"}`),
		)
		hook.OnFrame(frame)
	}
	hook.OnComplete()
	recordingID := hook.ID()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.Get(recordingID)
		if err != nil {
			b.Fatalf("failed to load recording: %v", err)
		}
	}
}

// BenchmarkRecordingList measures listing recordings.
func BenchmarkRecordingList(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-list-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    1024 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}

	// Create 100 recordings
	startTime := time.Now()
	for i := 0; i < 100; i++ {
		hook, _ := recording.NewFileStoreWebSocketHook(store, recording.RecordingMetadata{
			Path: "/bench/list",
		})
		for j := 0; j < 10; j++ {
			frame := recording.NewWebSocketFrame(
				int64(j+1),
				startTime,
				recording.DirectionServerToClient,
				recording.MessageTypeText,
				[]byte(`{}`),
			)
			hook.OnFrame(frame)
		}
		hook.OnComplete()
	}

	filter := recording.StreamRecordingFilter{
		Limit: 50,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := store.List(filter)
		if err != nil {
			b.Fatalf("failed to list: %v", err)
		}
	}
}

// BenchmarkRecordingConvert measures recording to mock conversion.
func BenchmarkRecordingConvert(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-convert-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    1024 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}

	// Create a recording with 500 frames
	hook, _ := recording.NewFileStoreWebSocketHook(store, recording.RecordingMetadata{
		Path: "/bench/convert",
	})
	startTime := time.Now()
	for j := 0; j < 500; j++ {
		frame := recording.NewWebSocketFrame(
			int64(j+1),
			startTime,
			recording.DirectionServerToClient,
			recording.MessageTypeText,
			[]byte(`{"seq":`+string(rune('0'+j%10))+`}`),
		)
		hook.OnFrame(frame)
		startTime = startTime.Add(50 * time.Millisecond)
	}
	hook.OnComplete()

	rec, _ := store.Get(hook.ID())
	opts := recording.StreamConvertOptions{
		SimplifyTiming:        true,
		MinDelay:              10,
		MaxDelay:              5000,
		IncludeClientMessages: false,
		Format:                "json",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := recording.ConvertStreamRecording(rec, opts)
		if err != nil {
			b.Fatalf("failed to convert: %v", err)
		}
	}
}

// BenchmarkConcurrentRecordings measures concurrent recording sessions.
func BenchmarkConcurrentRecordings(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-concurrent-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    1024 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			hook, err := recording.NewFileStoreWebSocketHook(store, recording.RecordingMetadata{
				Path: "/bench/concurrent",
			})
			if err != nil {
				b.Fatalf("failed to create hook: %v", err)
			}

			startTime := time.Now()
			for j := 0; j < 50; j++ {
				frame := recording.NewWebSocketFrame(
					int64(j+1),
					startTime,
					recording.DirectionServerToClient,
					recording.MessageTypeText,
					[]byte(`{"j":`+string(rune('0'+j%10))+`}`),
				)
				hook.OnFrame(frame)
			}
			hook.OnComplete()
		}
	})
}

// BenchmarkLargeRecording measures handling of large recordings (10k frames).
func BenchmarkLargeRecording(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-large-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := recording.NewFileStore(recording.StorageConfig{
		DataDir:     tmpDir,
		MaxBytes:    1024 * 1024 * 1024,
		WarnPercent: 80,
	})
	if err != nil {
		b.Fatalf("failed to create store: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hook, _ := recording.NewFileStoreWebSocketHook(store, recording.RecordingMetadata{
			Path: "/bench/large",
		})

		startTime := time.Now()
		for j := 0; j < 10000; j++ {
			frame := recording.NewWebSocketFrame(
				int64(j+1),
				startTime,
				recording.DirectionServerToClient,
				recording.MessageTypeText,
				[]byte(`{"index":`+string(rune('0'+j%10))+`,"payload":"some data here"}`),
			)
			hook.OnFrame(frame)
		}
		hook.OnComplete()
	}
}
