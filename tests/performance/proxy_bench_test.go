package performance

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"

	"github.com/getmockd/mockd/pkg/proxy"
	"github.com/getmockd/mockd/pkg/recording"
)

// BenchmarkProxyLatency measures proxy latency under load.
func BenchmarkProxyLatency(b *testing.B) {
	// Create a target server
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer target.Close()

	// Create recording store
	store := recording.NewStore()
	store.CreateSession("bench-session", nil)

	// Create proxy in record mode
	logger := log.New(io.Discard, "", 0)
	p := proxy.New(proxy.Options{
		Mode:   proxy.ModeRecord,
		Store:  store,
		Logger: logger,
	})

	// Create proxy server
	proxyServer := httptest.NewServer(p)
	defer proxyServer.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(target.URL + "/api/test")
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// BenchmarkProxyPassthroughLatency measures passthrough mode latency.
func BenchmarkProxyPassthroughLatency(b *testing.B) {
	// Create a target server
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer target.Close()

	// Create proxy in passthrough mode
	logger := log.New(io.Discard, "", 0)
	p := proxy.New(proxy.Options{
		Mode:   proxy.ModePassthrough,
		Logger: logger,
	})

	// Create proxy server
	proxyServer := httptest.NewServer(p)
	defer proxyServer.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := client.Get(target.URL + "/api/test")
		if err != nil {
			b.Fatalf("Request failed: %v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// BenchmarkProxyConcurrentConnections tests 100+ concurrent connections.
func BenchmarkProxyConcurrentConnections(b *testing.B) {
	// Create a target server
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer target.Close()

	// Create recording store
	store := recording.NewStore()
	store.CreateSession("bench-session", nil)

	// Create proxy
	logger := log.New(io.Discard, "", 0)
	p := proxy.New(proxy.Options{
		Mode:   proxy.ModeRecord,
		Store:  store,
		Logger: logger,
	})

	// Create proxy server
	proxyServer := httptest.NewServer(p)
	defer proxyServer.Close()

	proxyURL, _ := url.Parse(proxyServer.URL)

	concurrency := 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		wg.Add(concurrency)

		for j := 0; j < concurrency; j++ {
			go func() {
				defer wg.Done()
				client := &http.Client{
					Transport: &http.Transport{
						Proxy: http.ProxyURL(proxyURL),
					},
				}
				resp, err := client.Get(target.URL + "/api/test")
				if err != nil {
					return
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}()
		}
		wg.Wait()
	}
}

// BenchmarkRecordingsConversion benchmarks 1000 recordings conversion.
func BenchmarkRecordingsConversion(b *testing.B) {
	// Create 1000 recordings
	recordings := make([]*recording.Recording, 1000)
	for i := 0; i < 1000; i++ {
		rec := recording.NewRecording("")
		rec.Request.Method = "GET"
		rec.Request.Path = "/api/test"
		rec.Request.URL = "http://example.com/api/test"
		rec.Request.Host = "example.com"
		rec.Request.Headers = http.Header{"Content-Type": []string{"application/json"}}
		rec.Response.StatusCode = 200
		rec.Response.Headers = http.Header{"Content-Type": []string{"application/json"}}
		rec.Response.Body = []byte(`{"result": "success"}`)
		recordings[i] = rec
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opts := recording.ConvertOptions{
			Deduplicate:    true,
			IncludeHeaders: false,
		}
		_ = recording.ToMocks(recordings, opts)
	}
}

// BenchmarkCAGeneration benchmarks CA certificate generation.
func BenchmarkCAGeneration(b *testing.B) {
	tmpDir := b.TempDir()

	for i := 0; i < b.N; i++ {
		certPath := tmpDir + "/ca.crt"
		keyPath := tmpDir + "/ca.key"

		ca := proxy.NewCAManager(certPath, keyPath)
		if err := ca.Generate(); err != nil {
			b.Fatalf("Failed to generate CA: %v", err)
		}

		// Clean up for next iteration
		os.Remove(certPath)
		os.Remove(keyPath)
	}
}

// BenchmarkHostCertGeneration benchmarks per-host certificate generation.
func BenchmarkHostCertGeneration(b *testing.B) {
	tmpDir := b.TempDir()
	certPath := tmpDir + "/ca.crt"
	keyPath := tmpDir + "/ca.key"

	ca := proxy.NewCAManager(certPath, keyPath)
	if err := ca.Generate(); err != nil {
		b.Fatalf("Failed to generate CA: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Generate a unique host for each iteration to avoid cache hits
		host := "host" + string(rune('a'+(i%26))) + ".example.com"
		_, err := ca.GenerateHostCert(host)
		if err != nil {
			b.Fatalf("Failed to generate host cert: %v", err)
		}
	}
}
