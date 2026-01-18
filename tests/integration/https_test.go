package integration

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
	mockdtls "github.com/getmockd/mockd/pkg/tls"
)

// httpsTestBundle groups server and client for HTTPS tests
type httpsTestBundle struct {
	Server    *engine.Server
	Client    *engineclient.Client
	HTTPSPort int
	HTTPPort  int
}

func setupHTTPSServer(t *testing.T, withHTTP bool) *httpsTestBundle {
	httpsPort := getFreePort()
	managementPort := getFreePort()
	httpPort := 0
	if withHTTP {
		httpPort = getFreePort()
	}

	cfg := &config.ServerConfiguration{
		HTTPPort:     httpPort,
		HTTPSPort:    httpsPort,
		ManagementPort:  managementPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
		TLS: &config.TLSConfig{
			Enabled:          true,
			AutoGenerateCert: true,
		},
	}

	srv := engine.NewServer(cfg)
	err := srv.Start()
	require.NoError(t, err)

	t.Cleanup(func() {
		srv.Stop()
	})

	time.Sleep(50 * time.Millisecond)

	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	return &httpsTestBundle{
		Server:    srv,
		Client:    client,
		HTTPSPort: httpsPort,
		HTTPPort:  httpPort,
	}
}

// T111: HTTPS server starts
func TestHTTPSServerStarts(t *testing.T) {
	bundle := setupHTTPSServer(t, false)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "https-test",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/secure",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "secure response",
			},
		},
	})
	require.NoError(t, err)

	// Server should be running
	assert.True(t, bundle.Server.IsRunning())
}

// T112: HTTPS request returns response
func TestHTTPSRequestReturnsResponse(t *testing.T) {
	bundle := setupHTTPSServer(t, false)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "https-response",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/secure",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: `{"secure": true}`,
			},
		},
	})
	require.NoError(t, err)

	// Create client that trusts self-signed certs
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Get(fmt.Sprintf("https://localhost:%d/api/secure", bundle.HTTPSPort))
	require.NoError(t, err)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, `{"secure": true}`, string(body))
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

// T113: Both HTTP and HTTPS work simultaneously
func TestHTTPAndHTTPSSimultaneously(t *testing.T) {
	bundle := setupHTTPSServer(t, true)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "dual-mock",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/data",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "dual protocol response",
			},
		},
	})
	require.NoError(t, err)

	// Test HTTP
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/api/data", bundle.HTTPPort))
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

	resp, err = client.Get(fmt.Sprintf("https://localhost:%d/api/data", bundle.HTTPSPort))
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
	managementPort := getFreePort()

	cfg := &config.ServerConfiguration{
		HTTPSPort:    httpsPort,
		ManagementPort:  managementPort,
		ReadTimeout:  30,
		WriteTimeout: 30,
		TLS: &config.TLSConfig{
			Enabled:          true,
			AutoGenerateCert: false,
			CertFile:         certPath,
			KeyFile:          keyPath,
		},
	}

	srv := engine.NewServer(cfg)
	err = srv.Start()
	require.NoError(t, err)
	defer srv.Stop()

	time.Sleep(50 * time.Millisecond)

	client := engineclient.New(fmt.Sprintf("http://localhost:%d", srv.ManagementPort()))

	_, err = client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "cert-file-mock",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/test",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "loaded from cert file",
			},
		},
	})
	require.NoError(t, err)

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := httpClient.Get(fmt.Sprintf("https://localhost:%d/test", httpsPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "loaded from cert file", string(body))
}

// Test TLS version
func TestHTTPSTLSVersion(t *testing.T) {
	bundle := setupHTTPSServer(t, false)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "tls-version",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Path: "/tls-test",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "ok",
			},
		},
	})
	require.NoError(t, err)

	// Should work with TLS 1.2+
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				MinVersion:         tls.VersionTLS12,
			},
		},
	}

	resp, err := client.Get(fmt.Sprintf("https://localhost:%d/tls-test", bundle.HTTPSPort))
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// Test HTTPS with multiple mocks
func TestHTTPSMultipleMocks(t *testing.T) {
	bundle := setupHTTPSServer(t, false)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "users",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "users",
			},
		},
	})
	require.NoError(t, err)

	_, err = bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "orders",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: "GET",
				Path:   "/api/orders",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 200,
				Body:       "orders",
			},
		},
	})
	require.NoError(t, err)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	// Test users endpoint
	resp, err := client.Get(fmt.Sprintf("https://localhost:%d/api/users", bundle.HTTPSPort))
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "users", string(body))

	// Test orders endpoint
	resp, err = client.Get(fmt.Sprintf("https://localhost:%d/api/orders", bundle.HTTPSPort))
	require.NoError(t, err)
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, "orders", string(body))
}

// Test POST request over HTTPS
func TestHTTPSPostRequest(t *testing.T) {
	bundle := setupHTTPSServer(t, false)

	_, err := bundle.Client.CreateMock(context.Background(), &config.MockConfiguration{
		ID:      "create-user",
		Enabled: true,
		Type:    mock.MockTypeHTTP,
		HTTP: &mock.HTTPSpec{
			Matcher: &mock.HTTPMatcher{
				Method: "POST",
				Path:   "/api/users",
			},
			Response: &mock.HTTPResponse{
				StatusCode: 201,
				Body:       "created",
			},
		},
	})
	require.NoError(t, err)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := client.Post(
		fmt.Sprintf("https://localhost:%d/api/users", bundle.HTTPSPort),
		"application/json",
		nil,
	)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "created", string(body))
}
