package performance

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getmockd/mockd/pkg/soap"
)

// Sample SOAP request template
const soapRequestTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser xmlns="http://example.com/user">
      <userId>%s</userId>
    </GetUser>
  </soap:Body>
</soap:Envelope>`

// Sample SOAP response for GetUser operation
const soapResponseTemplate = `<GetUserResponse xmlns="http://example.com/user">
  <user>
    <id>{{xpath://userId}}</id>
    <name>Test User</name>
    <email>test@example.com</email>
  </user>
</GetUserResponse>`

func setupBenchSOAPHandler(b *testing.B) *httptest.Server {
	cfg := &soap.SOAPConfig{
		ID:      "bench-soap",
		Name:    "Benchmark SOAP Service",
		Path:    "/soap",
		Enabled: true,
		Operations: map[string]soap.OperationConfig{
			"GetUser": {
				SOAPAction: "http://example.com/GetUser",
				Response:   soapResponseTemplate,
			},
		},
	}

	handler := soap.NewHandler(cfg)
	return httptest.NewServer(handler)
}

// BenchmarkSOAP_RequestLatency measures single SOAP request latency.
func BenchmarkSOAP_RequestLatency(b *testing.B) {
	ts := setupBenchSOAPHandler(b)
	defer ts.Close()

	client := &http.Client{}
	soapRequest := fmt.Sprintf(soapRequestTemplate, "user-123")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", ts.URL+"/soap", strings.NewReader(soapRequest))
		req.Header.Set("Content-Type", "text/xml; charset=utf-8")
		req.Header.Set("SOAPAction", "http://example.com/GetUser")

		resp, err := client.Do(req)
		if err != nil {
			b.Fatalf("request failed: %v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			b.Fatalf("unexpected status: %d", resp.StatusCode)
		}
	}
}

// BenchmarkSOAP_ConcurrentRequests measures throughput under concurrent load.
func BenchmarkSOAP_ConcurrentRequests(b *testing.B) {
	ts := setupBenchSOAPHandler(b)
	defer ts.Close()

	client := &http.Client{}
	soapRequest := fmt.Sprintf(soapRequestTemplate, "user-456")

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest("POST", ts.URL+"/soap", strings.NewReader(soapRequest))
			req.Header.Set("Content-Type", "text/xml; charset=utf-8")
			req.Header.Set("SOAPAction", "http://example.com/GetUser")

			resp, err := client.Do(req)
			if err != nil {
				b.Errorf("request failed: %v", err)
				continue
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})
}

// BenchmarkSOAP_MessageSizes tests performance with different XML payload sizes.
func BenchmarkSOAP_MessageSizes(b *testing.B) {
	ts := setupBenchSOAPHandler(b)
	defer ts.Close()

	client := &http.Client{}

	sizes := []struct {
		name    string
		padding int
	}{
		{"small_500B", 0},
		{"medium_2KB", 1500},
		{"large_10KB", 9500},
	}

	for _, size := range sizes {
		b.Run(size.name, func(b *testing.B) {
			// Create padded request
			padding := strings.Repeat("X", size.padding)
			soapRequest := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser xmlns="http://example.com/user">
      <userId>user-789</userId>
      <padding>%s</padding>
    </GetUser>
  </soap:Body>
</soap:Envelope>`, padding)

			b.ResetTimer()
			b.SetBytes(int64(len(soapRequest)))

			for i := 0; i < b.N; i++ {
				req, _ := http.NewRequest("POST", ts.URL+"/soap", strings.NewReader(soapRequest))
				req.Header.Set("Content-Type", "text/xml; charset=utf-8")
				req.Header.Set("SOAPAction", "http://example.com/GetUser")

				resp, err := client.Do(req)
				if err != nil {
					b.Fatalf("request failed: %v", err)
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
			}
		})
	}
}

// BenchmarkSOAP_XMLParsing measures XML parsing overhead.
func BenchmarkSOAP_XMLParsing(b *testing.B) {
	ts := setupBenchSOAPHandler(b)
	defer ts.Close()

	client := &http.Client{}

	// Complex nested XML structure
	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <soap:Header>
    <auth:Credentials xmlns:auth="http://example.com/auth">
      <auth:Username>testuser</auth:Username>
      <auth:Token>abc123xyz</auth:Token>
    </auth:Credentials>
  </soap:Header>
  <soap:Body>
    <GetUser xmlns="http://example.com/user">
      <userId>complex-user</userId>
      <options>
        <includeDetails>true</includeDetails>
        <format>full</format>
      </options>
    </GetUser>
  </soap:Body>
</soap:Envelope>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", ts.URL+"/soap", strings.NewReader(soapRequest))
		req.Header.Set("Content-Type", "text/xml; charset=utf-8")
		req.Header.Set("SOAPAction", "http://example.com/GetUser")

		resp, err := client.Do(req)
		if err != nil {
			b.Fatalf("request failed: %v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// BenchmarkSOAP_SOAP12 measures SOAP 1.2 request handling.
func BenchmarkSOAP_SOAP12(b *testing.B) {
	ts := setupBenchSOAPHandler(b)
	defer ts.Close()

	client := &http.Client{}

	// SOAP 1.2 request
	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <GetUser xmlns="http://example.com/user">
      <userId>soap12-user</userId>
    </GetUser>
  </soap:Body>
</soap:Envelope>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", ts.URL+"/soap", strings.NewReader(soapRequest))
		req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8; action=\"http://example.com/GetUser\"")

		resp, err := client.Do(req)
		if err != nil {
			b.Fatalf("request failed: %v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// BenchmarkSOAP_ConnectionReuse measures performance with connection pooling.
func BenchmarkSOAP_ConnectionReuse(b *testing.B) {
	ts := setupBenchSOAPHandler(b)
	defer ts.Close()

	// Client with connection pooling
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	soapRequest := fmt.Sprintf(soapRequestTemplate, "pooled-user")
	requestBody := []byte(soapRequest)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", ts.URL+"/soap", bytes.NewReader(requestBody))
		req.Header.Set("Content-Type", "text/xml; charset=utf-8")
		req.Header.Set("SOAPAction", "http://example.com/GetUser")

		resp, err := client.Do(req)
		if err != nil {
			b.Fatalf("request failed: %v", err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

// BenchmarkSOAP_HighConcurrency tests under high concurrent load.
// Uses GOMAXPROCS * 50 parallel goroutines for realistic high-concurrency testing.
func BenchmarkSOAP_HighConcurrency(b *testing.B) {
	ts := setupBenchSOAPHandler(b)
	defer ts.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        200,
			MaxIdleConnsPerHost: 200,
		},
	}

	soapRequest := []byte(fmt.Sprintf(soapRequestTemplate, "concurrent-user"))

	b.ResetTimer()
	b.SetParallelism(50) // 50 goroutines per CPU
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest("POST", ts.URL+"/soap", bytes.NewReader(soapRequest))
			req.Header.Set("Content-Type", "text/xml; charset=utf-8")
			req.Header.Set("SOAPAction", "http://example.com/GetUser")

			resp, err := client.Do(req)
			if err != nil {
				b.Errorf("request failed: %v", err)
				continue
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	})
}
