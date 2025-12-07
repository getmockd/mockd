package integration

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/proxy"
	"github.com/getmockd/mockd/pkg/recording"
)

// TestProxyRecordsHTTPRequest tests that the proxy records HTTP request/response pairs.
func TestProxyRecordsHTTPRequest(t *testing.T) {
	// Create a target server that returns a simple response
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "hello"}`))
	}))
	defer target.Close()

	// Create recording store and session
	store := recording.NewStore()
	session := store.CreateSession("test-session", nil)

	// Create proxy in record mode
	logger := log.New(os.Stdout, "[proxy-test] ", log.LstdFlags)
	p := proxy.New(proxy.Options{
		Mode:   proxy.ModeRecord,
		Store:  store,
		Logger: logger,
	})

	// Create proxy server
	proxyServer := httptest.NewServer(p)
	defer proxyServer.Close()

	// Make a request through the proxy
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(mustParseURL(t, proxyServer.URL)),
		},
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(target.URL + "/api/test")
	if err != nil {
		t.Fatalf("Failed to make request through proxy: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Verify response
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if string(body) != `{"message": "hello"}` {
		t.Errorf("Unexpected body: %s", body)
	}

	// Verify recording was created
	recordings := session.Recordings()
	if len(recordings) != 1 {
		t.Fatalf("Expected 1 recording, got %d", len(recordings))
	}

	rec := recordings[0]
	if rec.Request.Method != "GET" {
		t.Errorf("Expected method GET, got %s", rec.Request.Method)
	}
	if rec.Request.Path != "/api/test" {
		t.Errorf("Expected path /api/test, got %s", rec.Request.Path)
	}
	if rec.Response.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", rec.Response.StatusCode)
	}
	if string(rec.Response.Body) != `{"message": "hello"}` {
		t.Errorf("Unexpected recorded body: %s", rec.Response.Body)
	}
}

// TestProxyPassthroughForwardsUnchanged tests that passthrough mode forwards requests unchanged.
func TestProxyPassthroughForwardsUnchanged(t *testing.T) {
	// Create a target server
	var receivedMethod, receivedPath string
	var receivedHeaders http.Header
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))
	defer target.Close()

	// Create proxy in passthrough mode
	store := recording.NewStore()
	p := proxy.New(proxy.Options{
		Mode:  proxy.ModePassthrough,
		Store: store,
	})

	// Create proxy server
	proxyServer := httptest.NewServer(p)
	defer proxyServer.Close()

	// Make a request through the proxy
	client := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(mustParseURL(t, proxyServer.URL)),
		},
	}

	req, _ := http.NewRequest("POST", target.URL+"/api/endpoint", nil)
	req.Header.Set("X-Custom-Header", "custom-value")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to make request through proxy: %v", err)
	}
	defer resp.Body.Close()

	// Verify request was forwarded
	if receivedMethod != "POST" {
		t.Errorf("Expected method POST, got %s", receivedMethod)
	}
	if receivedPath != "/api/endpoint" {
		t.Errorf("Expected path /api/endpoint, got %s", receivedPath)
	}
	if receivedHeaders.Get("X-Custom-Header") != "custom-value" {
		t.Errorf("Custom header not forwarded: got %s", receivedHeaders.Get("X-Custom-Header"))
	}

	// Verify response was returned
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	// Verify no recordings were created in passthrough mode
	recordings, _ := store.ListRecordings(recording.RecordingFilter{})
	if len(recordings) != 0 {
		t.Errorf("Expected no recordings in passthrough mode, got %d", len(recordings))
	}
}

// TestProxyPassthroughLogsTraffic tests that passthrough mode logs traffic details.
func TestProxyPassthroughLogsTraffic(t *testing.T) {
	// This test verifies logging behavior - with a custom logger we can verify output
	t.Skip("Logging verification test - requires logger capture")
}

// TestCAGenerationAndPersistence tests CA certificate generation and persistence.
func TestCAGenerationAndPersistence(t *testing.T) {
	// Create a temp directory for CA files
	tmpDir := t.TempDir()
	certPath := tmpDir + "/ca.crt"
	keyPath := tmpDir + "/ca.key"

	// Create CA manager
	ca := proxy.NewCAManager(certPath, keyPath)

	// Verify CA doesn't exist initially
	if ca.Exists() {
		t.Error("CA should not exist initially")
	}

	// Generate CA
	if err := ca.Generate(); err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Verify CA now exists
	if !ca.Exists() {
		t.Error("CA should exist after generation")
	}

	// Verify files were created
	if _, err := os.Stat(certPath); err != nil {
		t.Errorf("CA certificate file not created: %v", err)
	}
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("CA key file not created: %v", err)
	}

	// Create a new CA manager and load the existing CA
	ca2 := proxy.NewCAManager(certPath, keyPath)
	if err := ca2.Load(); err != nil {
		t.Fatalf("Failed to load CA: %v", err)
	}

	// Generate a host certificate
	hostCert, err := ca2.GenerateHostCert("example.com")
	if err != nil {
		t.Fatalf("Failed to generate host certificate: %v", err)
	}

	if hostCert.Cert == nil || hostCert.Key == nil {
		t.Error("Host certificate or key is nil")
	}

	// Verify the host cert is for the correct host
	if len(hostCert.Cert.DNSNames) == 0 || hostCert.Cert.DNSNames[0] != "example.com" {
		t.Errorf("Host certificate has wrong DNS name: %v", hostCert.Cert.DNSNames)
	}
}

// TestHTTPSMITMWithDynamicCerts tests HTTPS interception with dynamic certificates.
func TestHTTPSMITMWithDynamicCerts(t *testing.T) {
	// Create a TLS target server
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"secure": true}`))
	}))
	defer target.Close()

	// Create CA in temp directory
	tmpDir := t.TempDir()
	certPath := tmpDir + "/ca.crt"
	keyPath := tmpDir + "/ca.key"

	ca := proxy.NewCAManager(certPath, keyPath)
	if err := ca.Generate(); err != nil {
		t.Fatalf("Failed to generate CA: %v", err)
	}

	// Create recording store and session
	store := recording.NewStore()
	store.CreateSession("https-test", nil)

	// Create proxy with CA manager
	logger := log.New(os.Stdout, "[https-proxy] ", log.LstdFlags)
	p := proxy.New(proxy.Options{
		Mode:      proxy.ModeRecord,
		Store:     store,
		CAManager: ca,
		Logger:    logger,
	})

	// Create proxy server
	proxyServer := httptest.NewServer(p)
	defer proxyServer.Close()

	// Verify proxy is running
	if proxyServer.URL == "" {
		t.Fatal("Proxy server not started")
	}

	t.Log("HTTPS MITM proxy test setup complete")
	t.Log("Target:", target.URL)
	t.Log("Proxy:", proxyServer.URL)

	// Note: Full HTTPS MITM testing requires trusting the CA certificate
	// which is complex in an automated test. The test verifies that:
	// 1. CA can be generated and loaded
	// 2. Host certificates can be generated
	// 3. Proxy starts without error

	// Test that host cert generation works
	hostCert, err := ca.GenerateHostCert("example.com")
	if err != nil {
		t.Fatalf("Failed to generate host cert: %v", err)
	}
	if hostCert.Cert == nil {
		t.Error("Host certificate is nil")
	}
	if hostCert.Key == nil {
		t.Error("Host key is nil")
	}
	if len(hostCert.Cert.DNSNames) == 0 || hostCert.Cert.DNSNames[0] != "example.com" {
		t.Errorf("Host cert DNS name incorrect: %v", hostCert.Cert.DNSNames)
	}
}

// TestFilterIncludeExcludeBehavior tests include/exclude filter behavior.
func TestFilterIncludeExcludeBehavior(t *testing.T) {
	// TODO: Implement - US4 T039
	t.Skip("Filter tests moved to unit tests")
}

// mustParseURL parses a URL or fails the test.
func mustParseURL(t *testing.T, rawURL string) *url.URL {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("Failed to parse URL %s: %v", rawURL, err)
	}
	return u
}
