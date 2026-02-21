package e2e_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func boolPtr(b bool) *bool {
	return &b
}

func TestGRPCProtocolE2E(t *testing.T) {
	ctx := context.Background()

	// 1. Boot up mockd core natively
	grpcPort := getFreePort(t)
	adminPort := getFreePort(t)
	httpPort := getFreePort(t) // Need HTTP port even for grpc engine base

	cfg := &config.ServerConfiguration{
		HTTPPort:       httpPort,
		ManagementPort: adminPort,
	}

	server := engine.NewServer(cfg)
	go func() {
		if err := server.Start(); err != nil {
			// Expected on teardown
		}
	}()
	defer server.Stop()

	adminURL := "http://localhost:" + strconv.Itoa(adminPort)
	waitForServer(t, adminURL+"/health")

	client := engineclient.New(adminURL)

	// Add a gRPC mock via API
	wd, _ := os.Getwd()
	protoFile := filepath.Join(wd, "..", "fixtures", "grpc", "test.proto")

	grpcMock := &config.MockConfiguration{
		Name:    "e2e-grpc-test",
		Enabled: boolPtr(true),
		Type:    mock.TypeGRPC,
		GRPC: &mock.GRPCSpec{
			Port:       grpcPort,
			ProtoFile:  protoFile,
			Reflection: true,
			Services: map[string]mock.ServiceConfig{
				"test.UserService": {
					Methods: map[string]mock.MethodConfig{
						"GetUser": {
							Response: map[string]interface{}{
								"id":    "1",
								"name":  "Alice",
								"email": "alice@test.com",
							},
						},
						"ListUsers": {
							Responses: []any{
								map[string]interface{}{"id": "1", "name": "Alice", "email": "alice@test.com"},
								map[string]interface{}{"id": "2", "name": "Bob", "email": "bob@test.com"},
							},
						},
					},
				},
			},
		},
	}

	_, err := client.CreateMock(ctx, grpcMock)
	require.NoError(t, err)

	// Wait for gRPC server to be fully bound on the host network
	time.Sleep(200 * time.Millisecond)

	// 2. Start Testcontainer to simulate Black-Box network bounding
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      "alpine:3.20",
			ExtraHosts: []string{"host.docker.internal:host-gateway"},
			Cmd:        []string{"tail", "-f", "/dev/null"},
			WaitingFor: wait.ForExec([]string{"echo", "ready"}),
		},
		Started: true,
	}

	runner, err := testcontainers.GenericContainer(ctx, req)
	require.NoError(t, err)
	defer testcontainers.CleanupContainer(t, runner)

	// Install grpcurl locally into the container
	_, _, err = runner.Exec(ctx, []string{"apk", "add", "--no-cache", "curl", "tar"})
	require.NoError(t, err)

	installCmd := []string{
		"sh", "-c",
		"curl -sSL https://github.com/fullstorydev/grpcurl/releases/download/v1.9.3/grpcurl_1.9.3_linux_x86_64.tar.gz | tar -xz -C /usr/local/bin grpcurl && chmod +x /usr/local/bin/grpcurl",
	}
	code, installReader, err := runner.Exec(ctx, installCmd)
	require.NoError(t, err)
	require.Equal(t, 0, code, "failed to install grpcurl")
	
	// Ensure stream is closed
	if installReader != nil {
		_ = installReader
	}

	hostGRPCURL := fmt.Sprintf("host.docker.internal:%d", grpcPort)

	t.Run("Reflection List", func(t *testing.T) {
		code, outReader, err := runner.Exec(ctx, []string{"grpcurl", "-plaintext", hostGRPCURL, "list"})
		require.NoError(t, err)
		assert.Equal(t, 0, code)

		if outReader != nil {
			outBytes, _ := io.ReadAll(outReader)
			assert.Contains(t, string(outBytes), "test.UserService")
		}
	})

	// GRPC-004: Unary GetUser returns Alice
	t.Run("Unary Request GetUser", func(t *testing.T) {
		code, outReader, err := runner.Exec(ctx, []string{"grpcurl", "-plaintext", "-d", `{"id": "1"}`, hostGRPCURL, "test.UserService/GetUser"})
		require.NoError(t, err)
		assert.Equal(t, 0, code)

		if outReader != nil {
			outBytes, _ := io.ReadAll(outReader)
			output := string(outBytes)
			assert.Contains(t, output, "Alice")
			assert.Contains(t, output, "alice@test.com")
		}
	})

	// GRPC-006: Server streaming returns 2+ users
	t.Run("Streaming Request ListUsers", func(t *testing.T) {
		code, outReader, err := runner.Exec(ctx, []string{"grpcurl", "-plaintext", "-d", `{"page_size": 10}`, hostGRPCURL, "test.UserService/ListUsers"})
		require.NoError(t, err)
		assert.Equal(t, 0, code)

		if outReader != nil {
			outBytes, _ := io.ReadAll(outReader)
			output := string(outBytes)
			assert.Contains(t, output, "Alice")
			assert.Contains(t, output, "Bob")
		}
	})
}
