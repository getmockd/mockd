package integration

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	mockdtls "github.com/getmockd/mockd/pkg/tls"
)

// T111: HTTPS server starts
func TestHTTPSServerStarts(t *testing.T) {
	httpsPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPSPort:        httpsPort,
		AdminPort:        adminPort,
		AutoGenerateCert: true,
		ReadTimeout:      30,
		WriteTimeout:     30,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "https-test",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/secure",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "secure response",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Server should be running
	assert.True(t, srv.IsRunning())
}

// T112: HTTPS request returns response
func TestHTTPSRequestReturnsResponse(t *testing.T) {
	httpsPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPSPort:        httpsPort,
		AdminPort:        adminPort,
		AutoGenerateCert: true,
		ReadTimeout:      30,
		WriteTimeout:     30,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "https-response",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/secure",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: `{"secure": true}`,
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Create client that trusts self-signed certs
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Get(fmt.Sprintf("https://localhost:%d/api/secure", httpsPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"secure": true}`, string(body))
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

// T113: Both HTTP and HTTPS work simultaneously
func TestHTTPAndHTTPSSimultaneously(t *testing.T) {
	httpPort := getFreePort()
	httpsPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPPort:         httpPort,
		HTTPSPort:        httpsPort,
		AdminPort:        adminPort,
		AutoGenerateCert: true,
		ReadTimeout:      30,
		WriteTimeout:     30,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "dual-mock",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/data",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "dual protocol response",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Test HTTP
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/data", httpPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "dual protocol response", string(body))

	// Test HTTPS
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err = client.Get(fmt.Sprintf("https://localhost:%d/api/data", httpsPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "dual protocol response", string(body))
}

// Test HTTPS with certificate from files
func TestHTTPSWithCertificateFiles(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Generate certificate files
	_, err := mockdtls.GenerateAndSave(nil, certPath, keyPath)
	require.NoError(t, err)

	httpsPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPSPort:        httpsPort,
		AdminPort:        adminPort,
		AutoGenerateCert: false,
		CertFile:         certPath,
		KeyFile:          keyPath,
		ReadTimeout:      30,
		WriteTimeout:     30,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "cert-file-mock",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/test",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "loaded from cert file",
		},
	})

	err = srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Get(fmt.Sprintf("https://localhost:%d/test", httpsPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "loaded from cert file", string(body))
}

// Test TLS version
func TestHTTPSTLSVersion(t *testing.T) {
	httpsPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPSPort:        httpsPort,
		AdminPort:        adminPort,
		AutoGenerateCert: true,
		ReadTimeout:      30,
		WriteTimeout:     30,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "tls-version",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Path: "/tls-test",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "ok",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	// Should work with TLS 1.2+
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			},
		},
	}

	resp, err := client.Get(fmt.Sprintf("https://localhost:%d/tls-test", httpsPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Test HTTPS with multiple mocks
func TestHTTPSMultipleMocks(t *testing.T) {
	httpsPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPSPort:        httpsPort,
		AdminPort:        adminPort,
		AutoGenerateCert: true,
		ReadTimeout:      30,
		WriteTimeout:     30,
	}

	srv := engine.NewServer(cfg)

	srv.AddMock(&config.MockConfiguration{
		ID:       "users",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "users",
		},
	})

	srv.AddMock(&config.MockConfiguration{
		ID:       "orders",
		Priority: 0,
		Enabled:  true,
		Matcher: &config.RequestMatcher{
			Method: "GET",
			Path:   "/api/orders",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 200,
			Body:       "orders",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	// Test users endpoint
	resp, err := client.Get(fmt.Sprintf("https://localhost:%d/api/users", httpsPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "users", string(body))

	// Test orders endpoint
	resp, err = client.Get(fmt.Sprintf("https://localhost:%d/api/orders", httpsPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "orders", string(body))
}

// Test POST request over HTTPS
func TestHTTPSPostRequest(t *testing.T) {
	httpsPort := getFreePort()
	adminPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPSPort:        httpsPort,
		AdminPort:        adminPort,
		AutoGenerateCert: true,
		ReadTimeout:      30,
		WriteTimeout:     30,
	}

	srv := engine.NewServer(cfg)
	srv.AddMock(&config.MockConfiguration{
		ID:      "create-user",
		Enabled: true,
		Matcher: &config.RequestMatcher{
			Method: "POST",
			Path:   "/api/users",
		},
		Response: &config.ResponseDefinition{
			StatusCode: 201,
			Body:       "created",
		},
	})

	err := srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Post(
		fmt.Sprintf("https://localhost:%d/api/users", httpsPort),
		"application/json",
		nil,
	)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "created", string(body))
}
