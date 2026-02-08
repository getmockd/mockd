package performance

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/getmockd/mockd/internal/id"
	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Performance Benchmarks for Stateful Mocking
// =============================================================================

// benchServerBundle groups server, admin API, and ports for benchmark tests
type benchServerBundle struct {
	Server    *engine.Server
	AdminAPI  *admin.API
	HTTPPort  int
	AdminPort int
}

// Stop stops both the server and admin API
func (b *benchServerBundle) Stop() {
	if b.AdminAPI != nil {
		b.AdminAPI.Stop()
	}
	if b.Server != nil {
		b.Server.Stop()
	}
}

// createBenchmarkServer creates a stateful server for benchmarks
func createBenchmarkServer(b *testing.B, seedCount int) *benchServerBundle {
	b.Helper()

	httpPort := getFreePort()
	adminPort := getFreePort()
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ManagementPort:  managementPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Generate seed data
	seedData := make([]map[string]interface{}, seedCount)
	for i := 0; i < seedCount; i++ {
		seedData[i] = map[string]interface{}{
			"id":    fmt.Sprintf("item-%d", i),
			"name":  fmt.Sprintf("Item %d", i),
			"value": i,
			"tags":  []string{"benchmark", fmt.Sprintf("batch-%d", i/1000)},
		}
	}

	// Register resource via ImportConfig with a MockCollection
	collection := &config.MockCollection{
		Version: "1.0",
		StatefulResources: []*config.StatefulResourceConfig{
			{
				Name:     "items",
				BasePath: "/items",
				IDField:  "id",
				SeedData: seedData,
			},
		},
	}
	if err := srv.ImportConfig(collection, false); err != nil {
		b.Fatalf("failed to register resource: %v", err)
	}

	adminAPI := admin.NewAPI(adminPort,
		admin.WithLocalEngine(fmt.Sprintf("http://localhost:%d", srv.ManagementPort())),
		admin.WithAPIKeyDisabled(),
	)
	if err := adminAPI.Start(); err != nil {
		b.Fatalf("failed to start admin API: %v", err)
	}

	if err := srv.Start(); err != nil {
		b.Fatalf("failed to start server: %v", err)
	}

	// Wait for server to be ready
	time.Sleep(50 * time.Millisecond)

	return &benchServerBundle{
		Server:    srv,
		AdminAPI:  adminAPI,
		HTTPPort:  httpPort,
		AdminPort: adminPort,
	}
}

// createTestServer creates a stateful server for regular tests
func createTestServer(t *testing.T, seedCount int) *benchServerBundle {
	t.Helper()

	httpPort := getFreePort()
	adminPort := getFreePort()
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		AdminPort:    adminPort,
		ManagementPort:  managementPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
	}

	srv := engine.NewServer(cfg)

	// Generate seed data
	seedData := make([]map[string]interface{}, seedCount)
	for i := 0; i < seedCount; i++ {
		seedData[i] = map[string]interface{}{
			"id":    fmt.Sprintf("item-%d", i),
			"name":  fmt.Sprintf("Item %d", i),
			"value": i,
		}
	}

	// Register resource via ImportConfig with a MockCollection
	collection := &config.MockCollection{
		Version: "1.0",
		StatefulResources: []*config.StatefulResourceConfig{
			{
				Name:     "items",
				BasePath: "/items",
				IDField:  "id",
				SeedData: seedData,
			},
		},
	}
	require.NoError(t, srv.ImportConfig(collection, false))

	adminAPI := admin.NewAPI(adminPort,
		admin.WithLocalEngine(fmt.Sprintf("http://localhost:%d", srv.ManagementPort())),
		admin.WithAPIKeyDisabled(),
	)
	require.NoError(t, adminAPI.Start())
	require.NoError(t, srv.Start())

	// Wait for server to be ready
	time.Sleep(50 * time.Millisecond)

	return &benchServerBundle{
		Server:    srv,
		AdminAPI:  adminAPI,
		HTTPPort:  httpPort,
		AdminPort: adminPort,
	}
}

// BenchmarkCollectionQuery10000Records tests collection query performance.
// Target: 10,000 records should complete in <500ms (SC-003)
func BenchmarkCollectionQuery10000Records(b *testing.B) {
	// T077: Benchmark: 10,000 record collection query <500ms
	bundle := createBenchmarkServer(b, 10000)
	defer bundle.Stop()

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("http://localhost:%d/items", bundle.HTTPPort)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		resp, err := client.Get(url)
		duration := time.Since(start)

		if err != nil {
			b.Fatalf("request failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b.Fatalf("unexpected status: %d", resp.StatusCode)
		}

		// Verify target: <500ms
		if duration > 500*time.Millisecond {
			b.Errorf("query took too long: %v (target: <500ms)", duration)
		}
	}
}

// BenchmarkStateReset tests state reset performance.
// Target: <100ms regardless of data volume (SC-004)
func BenchmarkStateReset(b *testing.B) {
	// T078: Benchmark: State reset <100ms regardless of data volume
	bundle := createBenchmarkServer(b, 10000)
	defer bundle.Stop()

	client := &http.Client{Timeout: 10 * time.Second}
	resetURL := fmt.Sprintf("http://localhost:%d/state/reset", bundle.AdminPort)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		start := time.Now()
		resp, err := client.Post(resetURL, "application/json", nil)
		duration := time.Since(start)

		if err != nil {
			b.Fatalf("reset request failed: %v", err)
		}
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b.Fatalf("unexpected status: %d", resp.StatusCode)
		}

		// Verify target: <100ms
		if duration > 100*time.Millisecond {
			b.Errorf("reset took too long: %v (target: <100ms)", duration)
		}
	}
}

// BenchmarkConcurrentCRUD tests concurrent access handling.
// Target: 100 concurrent CRUD operations with no race conditions (SC-007)
func BenchmarkConcurrentCRUD(b *testing.B) {
	// T079: Benchmark: 100 concurrent CRUD operations no race conditions
	bundle := createBenchmarkServer(b, 0)
	defer bundle.Stop()

	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := fmt.Sprintf("http://localhost:%d/items", bundle.HTTPPort)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		concurrency := 100
		wg.Add(concurrency)

		for j := 0; j < concurrency; j++ {
			go func(idx int) {
				defer wg.Done()

				// Create
				itemID := fmt.Sprintf("concurrent-%d-%d", i, idx)
				body, _ := json.Marshal(map[string]interface{}{
					"id":    itemID,
					"name":  fmt.Sprintf("Item %d", idx),
					"value": idx,
				})

				resp, err := client.Post(baseURL, "application/json", bytes.NewReader(body))
				if err != nil {
					return
				}
				resp.Body.Close()

				// Read
				resp, err = client.Get(fmt.Sprintf("%s/%s", baseURL, itemID))
				if err != nil {
					return
				}
				resp.Body.Close()

				// Update
				body, _ = json.Marshal(map[string]interface{}{
					"id":    itemID,
					"name":  fmt.Sprintf("Updated Item %d", idx),
					"value": idx * 2,
				})
				req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s", baseURL, itemID), bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				resp, err = client.Do(req)
				if err != nil {
					return
				}
				resp.Body.Close()

				// Delete
				req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%s", baseURL, itemID), nil)
				resp, err = client.Do(req)
				if err != nil {
					return
				}
				resp.Body.Close()
			}(j)
		}

		wg.Wait()
	}
}

// BenchmarkIDGeneration tests ID generation uniqueness.
// Target: 100,000 ID generations with zero collisions (SC-005)
func BenchmarkIDGeneration100000(b *testing.B) {
	// T080: Benchmark: 100,000 ID generations zero collisions
	for i := 0; i < b.N; i++ {
		seen := make(map[string]bool)
		count := 100000

		for j := 0; j < count; j++ {
			id := id.UUID()
			if seen[id] {
				b.Fatalf("collision detected at iteration %d: %s", j, id)
			}
			seen[id] = true
		}
	}
}

// TestConcurrentCRUDRaceCondition runs with -race flag to detect race conditions.
func TestConcurrentCRUDRaceCondition(t *testing.T) {
	// This test should be run with `go test -race` to detect race conditions
	bundle := createTestServer(t, 0)
	defer bundle.Stop()

	client := &http.Client{Timeout: 10 * time.Second}
	baseURL := fmt.Sprintf("http://localhost:%d/items", bundle.HTTPPort)

	var wg sync.WaitGroup
	concurrency := 50
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()

			itemID := fmt.Sprintf("race-%d", idx)
			body, _ := json.Marshal(map[string]interface{}{
				"id":    itemID,
				"name":  fmt.Sprintf("Item %d", idx),
				"value": idx,
			})

			// Create
			resp, err := client.Post(baseURL, "application/json", bytes.NewReader(body))
			if err != nil {
				return
			}
			resp.Body.Close()

			// Multiple concurrent reads
			for j := 0; j < 5; j++ {
				resp, err = client.Get(fmt.Sprintf("%s/%s", baseURL, itemID))
				if err != nil {
					return
				}
				resp.Body.Close()
			}

			// Update
			body, _ = json.Marshal(map[string]interface{}{
				"id":    itemID,
				"name":  fmt.Sprintf("Updated %d", idx),
				"value": idx * 10,
			})
			req, _ := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/%s", baseURL, itemID), bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err = client.Do(req)
			if err != nil {
				return
			}
			resp.Body.Close()

			// Delete
			req, _ = http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/%s", baseURL, itemID), nil)
			resp, err = client.Do(req)
			if err != nil {
				return
			}
			resp.Body.Close()
		}(i)
	}

	wg.Wait()

	// Verify state is consistent after concurrent operations
	resp, err := client.Get(baseURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCollectionQueryPerformance validates 10K record query <500ms target
func TestCollectionQueryPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	bundle := createTestServer(t, 10000)
	defer bundle.Stop()

	// Give server a bit more time to load 10K records
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 10 * time.Second}
	url := fmt.Sprintf("http://localhost:%d/items", bundle.HTTPPort)

	// Run multiple iterations to get stable results
	var totalDuration time.Duration
	iterations := 5

	for i := 0; i < iterations; i++ {
		start := time.Now()
		resp, err := client.Get(url)
		duration := time.Since(start)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		totalDuration += duration
	}

	avgDuration := totalDuration / time.Duration(iterations)
	t.Logf("Average query time for 10K records: %v", avgDuration)

	assert.Less(t, avgDuration, 500*time.Millisecond,
		"10K record query should complete in <500ms, got %v", avgDuration)
}

// TestStateResetPerformance validates state reset <100ms target
func TestStateResetPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping performance test in short mode")
	}

	bundle := createTestServer(t, 10000)
	defer bundle.Stop()

	// Give server a bit more time to load 10K records
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 10 * time.Second}
	resetURL := fmt.Sprintf("http://localhost:%d/state/reset", bundle.AdminPort)

	// Run multiple iterations
	var totalDuration time.Duration
	iterations := 5

	for i := 0; i < iterations; i++ {
		start := time.Now()
		resp, err := client.Post(resetURL, "application/json", nil)
		duration := time.Since(start)
		require.NoError(t, err)
		resp.Body.Close()

		require.Equal(t, http.StatusOK, resp.StatusCode)
		totalDuration += duration
	}

	avgDuration := totalDuration / time.Duration(iterations)
	t.Logf("Average state reset time with 10K records: %v", avgDuration)

	assert.Less(t, avgDuration, 100*time.Millisecond,
		"State reset should complete in <100ms, got %v", avgDuration)
}
