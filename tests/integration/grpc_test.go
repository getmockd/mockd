package integration

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/grpc"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// ============================================================================
// Test Helpers
// ============================================================================

const testProtoFile = "../../tests/fixtures/grpc/test.proto"

// getFreeGRPCPort returns an available port for gRPC testing
func getFreeGRPCPort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

// setupGRPCServer creates and starts a gRPC server for testing
func setupGRPCServer(t *testing.T, cfg *grpc.GRPCConfig) *grpc.Server {
	// Parse proto file
	schema, err := grpc.ParseProtoFile(testProtoFile, nil)
	require.NoError(t, err, "Failed to parse proto file")

	server, err := grpc.NewServer(cfg, schema)
	require.NoError(t, err, "Failed to create gRPC server")

	err = server.Start(context.Background())
	require.NoError(t, err, "Failed to start gRPC server")

	t.Cleanup(func() {
		server.Stop(context.Background(), 5*time.Second)
	})

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	return server
}

// createGRPCConnection creates a gRPC client connection for testing
func createGRPCConnection(t *testing.T, port int) *grpclib.ClientConn {
	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := grpclib.NewClient(addr,
		grpclib.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err, "Failed to create gRPC connection")

	t.Cleanup(func() {
		conn.Close()
	})

	return conn
}

// createDynamicStub creates a dynamic gRPC stub for testing
func createDynamicStub(t *testing.T, port int) grpcdynamic.Stub {
	conn := createGRPCConnection(t, port)
	return grpcdynamic.NewStub(conn)
}

// getSchema returns the parsed proto schema for testing
func getSchema(t *testing.T) *grpc.ProtoSchema {
	schema, err := grpc.ParseProtoFile(testProtoFile, nil)
	require.NoError(t, err)
	return schema
}

// getJhumpDescriptors parses proto using jhump for client-side dynamic invocation.
// grpcdynamic.Stub requires jhump's *desc.MethodDescriptor, not protoreflect types.
func getJhumpDescriptors(t *testing.T) []*desc.FileDescriptor {
	t.Helper()
	parser := protoparse.Parser{}
	files, err := parser.ParseFiles(testProtoFile)
	require.NoError(t, err)
	return files
}

// getJhumpMethodDesc returns jhump method descriptor for use with grpcdynamic.Stub
func getJhumpMethodDesc(t *testing.T, files []*desc.FileDescriptor, serviceName, methodName string) *desc.MethodDescriptor {
	t.Helper()
	for _, file := range files {
		for _, svc := range file.GetServices() {
			if svc.GetFullyQualifiedName() == serviceName {
				for _, method := range svc.GetMethods() {
					if method.GetName() == methodName {
						return method
					}
				}
			}
		}
	}
	t.Fatalf("method %s/%s not found", serviceName, methodName)
	return nil
}

// ============================================================================
// User Story 1: Basic gRPC Server
// ============================================================================

func TestGRPC_US1_ServerStartStop(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-server",
		Name:      "Test Server",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services:  make(map[string]grpc.ServiceConfig),
	}

	schema, err := grpc.ParseProtoFile(testProtoFile, nil)
	require.NoError(t, err)

	server, err := grpc.NewServer(cfg, schema)
	require.NoError(t, err)

	// Start server
	err = server.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, server.IsRunning())

	// Verify port is listening
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), time.Second)
	require.NoError(t, err)
	conn.Close()

	// Stop server
	err = server.Stop(context.Background(), 5*time.Second)
	require.NoError(t, err)
	assert.False(t, server.IsRunning())
}

func TestGRPC_US1_ServerDoubleStart(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-double-start",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services:  make(map[string]grpc.ServiceConfig),
	}

	schema, err := grpc.ParseProtoFile(testProtoFile, nil)
	require.NoError(t, err)

	server, err := grpc.NewServer(cfg, schema)
	require.NoError(t, err)

	// Start once
	err = server.Start(context.Background())
	require.NoError(t, err)

	// Second start should fail
	err = server.Start(context.Background())
	assert.Error(t, err)
	assert.Equal(t, grpc.ErrServerAlreadyRunning, err)

	// Cleanup
	server.Stop(context.Background(), 5*time.Second)
}

// ============================================================================
// User Story 2: Unary RPC Calls
// ============================================================================

func TestGRPC_US2_UnaryCall(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-unary",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services: map[string]grpc.ServiceConfig{
			"test.UserService": {
				Methods: map[string]grpc.MethodConfig{
					"GetUser": {
						Response: map[string]interface{}{
							"id":    "user-123",
							"name":  "Test User",
							"email": "test@example.com",
						},
					},
				},
			},
		},
	}

	_ = setupGRPCServer(t, cfg)
	files := getJhumpDescriptors(t)

	stub := createDynamicStub(t, port)

	// Get method descriptor (jhump type for grpcdynamic)
	methodDesc := getJhumpMethodDesc(t, files, "test.UserService", "GetUser")

	// Create request message
	inputDesc := methodDesc.GetInputType()
	reqMsg := dynamic.NewMessage(inputDesc)
	reqMsg.SetFieldByName("id", "123")

	// Make unary call
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)
	require.NoError(t, err)

	// Check response
	respMsg := resp.(*dynamic.Message)
	assert.Equal(t, "user-123", respMsg.GetFieldByName("id"))
	assert.Equal(t, "Test User", respMsg.GetFieldByName("name"))
	assert.Equal(t, "test@example.com", respMsg.GetFieldByName("email"))
}

func TestGRPC_US2_UnaryCallWithDelay(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-unary-delay",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services: map[string]grpc.ServiceConfig{
			"test.UserService": {
				Methods: map[string]grpc.MethodConfig{
					"GetUser": {
						Response: map[string]interface{}{
							"id":    "user-123",
							"name":  "Delayed User",
							"email": "delayed@example.com",
						},
						Delay: "200ms",
					},
				},
			},
		},
	}

	_ = setupGRPCServer(t, cfg)
	files := getJhumpDescriptors(t)
	stub := createDynamicStub(t, port)

	// Get method descriptor (jhump type for grpcdynamic)
	methodDesc := getJhumpMethodDesc(t, files, "test.UserService", "GetUser")

	// Create request message
	inputDesc := methodDesc.GetInputType()
	reqMsg := dynamic.NewMessage(inputDesc)
	reqMsg.SetFieldByName("id", "123")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	_, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(180), "Should have delay of at least 180ms")
}

// ============================================================================
// User Story 3: Error Responses
// ============================================================================

func TestGRPC_US3_NotFoundError(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-error-notfound",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services: map[string]grpc.ServiceConfig{
			"test.UserService": {
				Methods: map[string]grpc.MethodConfig{
					"GetUser": {
						Error: &grpc.GRPCErrorConfig{
							Code:    "NOT_FOUND",
							Message: "User not found",
						},
					},
				},
			},
		},
	}

	_ = setupGRPCServer(t, cfg)
	files := getJhumpDescriptors(t)
	stub := createDynamicStub(t, port)

	methodDesc := getJhumpMethodDesc(t, files, "test.UserService", "GetUser")

	inputDesc := methodDesc.GetInputType()
	reqMsg := dynamic.NewMessage(inputDesc)
	reqMsg.SetFieldByName("id", "123")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)

	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Equal(t, "User not found", st.Message())
}

func TestGRPC_US3_PermissionDeniedError(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-error-permission",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services: map[string]grpc.ServiceConfig{
			"test.UserService": {
				Methods: map[string]grpc.MethodConfig{
					"GetUser": {
						Error: &grpc.GRPCErrorConfig{
							Code:    "PERMISSION_DENIED",
							Message: "Access denied",
						},
					},
				},
			},
		},
	}

	_ = setupGRPCServer(t, cfg)
	files := getJhumpDescriptors(t)
	stub := createDynamicStub(t, port)

	methodDesc := getJhumpMethodDesc(t, files, "test.UserService", "GetUser")

	inputDesc := methodDesc.GetInputType()
	reqMsg := dynamic.NewMessage(inputDesc)
	reqMsg.SetFieldByName("id", "123")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)

	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.PermissionDenied, st.Code())
}

func TestGRPC_US3_UnavailableError(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-error-unavailable",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services: map[string]grpc.ServiceConfig{
			"test.HealthService": {
				Methods: map[string]grpc.MethodConfig{
					"Check": {
						Error: &grpc.GRPCErrorConfig{
							Code:    "UNAVAILABLE",
							Message: "Service temporarily unavailable",
						},
					},
				},
			},
		},
	}

	_ = setupGRPCServer(t, cfg)
	files := getJhumpDescriptors(t)
	stub := createDynamicStub(t, port)

	methodDesc := getJhumpMethodDesc(t, files, "test.HealthService", "Check")

	inputDesc := methodDesc.GetInputType()
	reqMsg := dynamic.NewMessage(inputDesc)
	reqMsg.SetFieldByName("service", "test")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)

	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unavailable, st.Code())
}

// ============================================================================
// User Story 4: Unconfigured Method
// ============================================================================

func TestGRPC_US4_UnconfiguredMethod(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-unconfigured",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services:  map[string]grpc.ServiceConfig{},
	}

	_ = setupGRPCServer(t, cfg)
	files := getJhumpDescriptors(t)
	stub := createDynamicStub(t, port)

	methodDesc := getJhumpMethodDesc(t, files, "test.UserService", "GetUser")

	inputDesc := methodDesc.GetInputType()
	reqMsg := dynamic.NewMessage(inputDesc)
	reqMsg.SetFieldByName("id", "123")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)

	assert.Error(t, err)
	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Unimplemented, st.Code())
}

// ============================================================================
// User Story 5: Health Check Service
// ============================================================================

func TestGRPC_US5_HealthCheck(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-health",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services: map[string]grpc.ServiceConfig{
			"test.HealthService": {
				Methods: map[string]grpc.MethodConfig{
					"Check": {
						Response: map[string]interface{}{
							"status": 1, // SERVING
						},
					},
				},
			},
		},
	}

	_ = setupGRPCServer(t, cfg)
	files := getJhumpDescriptors(t)
	stub := createDynamicStub(t, port)

	methodDesc := getJhumpMethodDesc(t, files, "test.HealthService", "Check")

	inputDesc := methodDesc.GetInputType()
	reqMsg := dynamic.NewMessage(inputDesc)
	reqMsg.SetFieldByName("service", "test")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)
	require.NoError(t, err)

	respMsg := resp.(*dynamic.Message)
	// Status 1 = SERVING
	assert.Equal(t, int32(1), respMsg.GetFieldByName("status"))
}

// ============================================================================
// User Story 6: Metadata Handling
// ============================================================================

func TestGRPC_US6_MetadataInContext(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-metadata",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services: map[string]grpc.ServiceConfig{
			"test.UserService": {
				Methods: map[string]grpc.MethodConfig{
					"GetUser": {
						Response: map[string]interface{}{
							"id":    "user-meta",
							"name":  "Metadata User",
							"email": "meta@example.com",
						},
					},
				},
			},
		},
	}

	_ = setupGRPCServer(t, cfg)
	files := getJhumpDescriptors(t)
	stub := createDynamicStub(t, port)

	methodDesc := getJhumpMethodDesc(t, files, "test.UserService", "GetUser")

	inputDesc := methodDesc.GetInputType()
	reqMsg := dynamic.NewMessage(inputDesc)
	reqMsg.SetFieldByName("id", "123")

	// Create context with metadata
	md := metadata.New(map[string]string{
		"authorization": "Bearer test-token",
		"x-request-id":  "req-12345",
	})
	ctx := metadata.NewOutgoingContext(context.Background(), md)
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)
	require.NoError(t, err)

	respMsg := resp.(*dynamic.Message)
	assert.Equal(t, "user-meta", respMsg.GetFieldByName("id"))
}

// ============================================================================
// User Story 7: Server Stats
// ============================================================================

func TestGRPC_US7_ServerStats(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-stats",
		Name:      "Stats Test Server",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services:  make(map[string]grpc.ServiceConfig),
	}

	server := setupGRPCServer(t, cfg)

	// Check stats
	stats := server.Stats()
	assert.True(t, stats.Running)
	assert.False(t, stats.StartedAt.IsZero())
	assert.Greater(t, stats.Uptime, time.Duration(0))

	// Check metadata
	meta := server.Metadata()
	assert.Equal(t, "test-stats", meta.ID)
	assert.Equal(t, "Stats Test Server", meta.Name)

	// Check health
	health := server.Health(context.Background())
	assert.Equal(t, "healthy", string(health.Status))
}

// ============================================================================
// User Story 8: Multiple Services
// ============================================================================

func TestGRPC_US8_MultipleServices(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-multi-service",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services: map[string]grpc.ServiceConfig{
			"test.UserService": {
				Methods: map[string]grpc.MethodConfig{
					"GetUser": {
						Response: map[string]interface{}{
							"id":    "user-1",
							"name":  "User One",
							"email": "user1@example.com",
						},
					},
				},
			},
			"test.HealthService": {
				Methods: map[string]grpc.MethodConfig{
					"Check": {
						Response: map[string]interface{}{
							"status": 1,
						},
					},
				},
			},
		},
	}

	_ = setupGRPCServer(t, cfg)
	files := getJhumpDescriptors(t)
	stub := createDynamicStub(t, port)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call UserService
	userMethodDesc := getJhumpMethodDesc(t, files, "test.UserService", "GetUser")

	userReq := dynamic.NewMessage(userMethodDesc.GetInputType())
	userReq.SetFieldByName("id", "1")

	userResp, err := stub.InvokeRpc(ctx, userMethodDesc, userReq)
	require.NoError(t, err)
	assert.Equal(t, "user-1", userResp.(*dynamic.Message).GetFieldByName("id"))

	// Call HealthService
	healthMethodDesc := getJhumpMethodDesc(t, files, "test.HealthService", "Check")

	healthReq := dynamic.NewMessage(healthMethodDesc.GetInputType())
	healthReq.SetFieldByName("service", "test")

	healthResp, err := stub.InvokeRpc(ctx, healthMethodDesc, healthReq)
	require.NoError(t, err)
	assert.Equal(t, int32(1), healthResp.(*dynamic.Message).GetFieldByName("status"))
}

// ============================================================================
// User Story 9: Schema Inspection
// ============================================================================

func TestGRPC_US9_ListServices(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-schema",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services:  make(map[string]grpc.ServiceConfig),
	}

	server := setupGRPCServer(t, cfg)

	services := server.ListServices()
	assert.Len(t, services, 2)

	// Check service names
	serviceNames := make([]string, 0)
	for _, svc := range services {
		serviceNames = append(serviceNames, svc.Name)
	}
	assert.Contains(t, serviceNames, "test.UserService")
	assert.Contains(t, serviceNames, "test.HealthService")
}

func TestGRPC_US9_ListMethods(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-methods",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services:  make(map[string]grpc.ServiceConfig),
	}

	server := setupGRPCServer(t, cfg)

	methods := server.ListMethods("test.UserService")
	assert.Len(t, methods, 4) // GetUser, ListUsers, CreateUsers, Chat

	// Check method names
	methodNames := make([]string, 0)
	for _, m := range methods {
		methodNames = append(methodNames, m.Name)
	}
	assert.Contains(t, methodNames, "GetUser")
	assert.Contains(t, methodNames, "ListUsers")
	assert.Contains(t, methodNames, "CreateUsers")
	assert.Contains(t, methodNames, "Chat")
}

// ============================================================================
// User Story 10: Proto Schema Parsing
// ============================================================================

func TestGRPC_US10_ProtoSchemaParsing(t *testing.T) {
	schema, err := grpc.ParseProtoFile(testProtoFile, nil)
	require.NoError(t, err)

	// Check service count
	assert.Equal(t, 2, schema.ServiceCount())

	// Check method count
	assert.Equal(t, 5, schema.MethodCount()) // 4 in UserService + 1 in HealthService

	// Check UserService
	userSvc := schema.GetService("test.UserService")
	require.NotNil(t, userSvc)
	assert.Len(t, userSvc.Methods, 4)

	// Check GetUser method
	getUser := userSvc.GetMethod("GetUser")
	require.NotNil(t, getUser)
	assert.True(t, getUser.IsUnary())
	assert.False(t, getUser.ClientStreaming)
	assert.False(t, getUser.ServerStreaming)
	assert.Equal(t, "unary", getUser.GetStreamingType())

	// Check ListUsers method (server streaming)
	listUsers := userSvc.GetMethod("ListUsers")
	require.NotNil(t, listUsers)
	assert.True(t, listUsers.IsServerStreaming())
	assert.False(t, listUsers.ClientStreaming)
	assert.True(t, listUsers.ServerStreaming)
	assert.Equal(t, "server_streaming", listUsers.GetStreamingType())

	// Check CreateUsers method (client streaming)
	createUsers := userSvc.GetMethod("CreateUsers")
	require.NotNil(t, createUsers)
	assert.True(t, createUsers.IsClientStreaming())
	assert.True(t, createUsers.ClientStreaming)
	assert.False(t, createUsers.ServerStreaming)
	assert.Equal(t, "client_streaming", createUsers.GetStreamingType())

	// Check Chat method (bidirectional)
	chat := userSvc.GetMethod("Chat")
	require.NotNil(t, chat)
	assert.True(t, chat.IsBidirectional())
	assert.True(t, chat.ClientStreaming)
	assert.True(t, chat.ServerStreaming)
	assert.Equal(t, "bidirectional", chat.GetStreamingType())
}

// ============================================================================
// User Story 11: Config Validation
// ============================================================================

func TestGRPC_US11_ConfigValidation(t *testing.T) {
	// Missing ID
	cfg := &grpc.GRPCConfig{
		Port:      50051,
		ProtoFile: testProtoFile,
	}
	assert.Error(t, cfg.Validate())

	// Invalid port
	cfg = &grpc.GRPCConfig{
		ID:        "test",
		Port:      -1,
		ProtoFile: testProtoFile,
	}
	assert.Error(t, cfg.Validate())

	// Missing proto file
	cfg = &grpc.GRPCConfig{
		ID:   "test",
		Port: 50051,
	}
	assert.Error(t, cfg.Validate())

	// Valid config
	cfg = &grpc.GRPCConfig{
		ID:        "test",
		Port:      50051,
		ProtoFile: testProtoFile,
	}
	assert.NoError(t, cfg.Validate())
}

func TestGRPC_US11_ErrorConfigValidation(t *testing.T) {
	// Missing code
	errCfg := &grpc.GRPCErrorConfig{
		Message: "Error message",
	}
	assert.Error(t, errCfg.Validate())

	// Invalid code
	errCfg = &grpc.GRPCErrorConfig{
		Code:    "INVALID_CODE",
		Message: "Error message",
	}
	assert.Error(t, errCfg.Validate())

	// Valid config
	errCfg = &grpc.GRPCErrorConfig{
		Code:    "NOT_FOUND",
		Message: "Resource not found",
	}
	assert.NoError(t, errCfg.Validate())
}

// ============================================================================
// User Story 12: Server Address and Port
// ============================================================================

func TestGRPC_US12_ServerAddress(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-address",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services:  make(map[string]grpc.ServiceConfig),
	}

	server := setupGRPCServer(t, cfg)

	// Check address is set
	addr := server.Address()
	assert.NotEmpty(t, addr)

	// Check port matches
	actualPort := server.Port()
	assert.Equal(t, port, actualPort)
}

// ============================================================================
// User Story 13: Server Stop Timeout
// ============================================================================

func TestGRPC_US13_StopWithContext(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-stop",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services:  make(map[string]grpc.ServiceConfig),
	}

	schema, err := grpc.ParseProtoFile(testProtoFile, nil)
	require.NoError(t, err)

	server, err := grpc.NewServer(cfg, schema)
	require.NoError(t, err)

	err = server.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, server.IsRunning())

	// Stop with context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = server.Stop(ctx, 5*time.Second)
	assert.NoError(t, err)
	assert.False(t, server.IsRunning())

	// Stop again should be no-op
	err = server.Stop(context.Background(), 5*time.Second)
	assert.NoError(t, err)
}

// ============================================================================
// User Story 14: Nil Config/Schema Handling
// ============================================================================

func TestGRPC_US14_NilConfig(t *testing.T) {
	schema, err := grpc.ParseProtoFile(testProtoFile, nil)
	require.NoError(t, err)

	_, err = grpc.NewServer(nil, schema)
	assert.Error(t, err)
	assert.Equal(t, grpc.ErrNilConfig, err)
}

func TestGRPC_US14_NilSchema(t *testing.T) {
	cfg := &grpc.GRPCConfig{
		ID:        "test",
		Port:      50051,
		ProtoFile: testProtoFile,
	}

	_, err := grpc.NewServer(cfg, nil)
	assert.Error(t, err)
	assert.Equal(t, grpc.ErrNilSchema, err)
}

// ============================================================================
// User Story 15: Status Codes
// ============================================================================

func TestGRPC_US15_AllStatusCodes(t *testing.T) {
	// Test all known status codes are valid
	validCodes := []string{
		"OK", "CANCELLED", "UNKNOWN", "INVALID_ARGUMENT", "DEADLINE_EXCEEDED",
		"NOT_FOUND", "ALREADY_EXISTS", "PERMISSION_DENIED", "RESOURCE_EXHAUSTED",
		"FAILED_PRECONDITION", "ABORTED", "OUT_OF_RANGE", "UNIMPLEMENTED",
		"INTERNAL", "UNAVAILABLE", "DATA_LOSS", "UNAUTHENTICATED",
	}

	for _, code := range validCodes {
		assert.True(t, grpc.ValidateStatusCode(code), "Expected %s to be valid", code)
	}

	// Test invalid code
	assert.False(t, grpc.ValidateStatusCode("INVALID"))
}

// ============================================================================
// User Story 16: Server Streaming
// ============================================================================

func TestGRPC_US16_ServerStreaming(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-server-streaming",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services: map[string]grpc.ServiceConfig{
			"test.UserService": {
				Methods: map[string]grpc.MethodConfig{
					"ListUsers": {
						Responses: []interface{}{
							map[string]interface{}{"id": "user-1", "name": "User 1", "email": "user1@example.com"},
							map[string]interface{}{"id": "user-2", "name": "User 2", "email": "user2@example.com"},
							map[string]interface{}{"id": "user-3", "name": "User 3", "email": "user3@example.com"},
						},
					},
				},
			},
		},
	}

	_ = setupGRPCServer(t, cfg)
	files := getJhumpDescriptors(t)
	stub := createDynamicStub(t, port)

	methodDesc := getJhumpMethodDesc(t, files, "test.UserService", "ListUsers")

	inputDesc := methodDesc.GetInputType()
	reqMsg := dynamic.NewMessage(inputDesc)
	reqMsg.SetFieldByName("page_size", int32(10))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Call server streaming method
	stream, err := stub.InvokeRpcServerStream(ctx, methodDesc, reqMsg)
	require.NoError(t, err)

	// Collect all responses
	var users []string
	for {
		resp, err := stream.RecvMsg()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		respMsg := resp.(*dynamic.Message)
		users = append(users, respMsg.GetFieldByName("id").(string))
	}

	assert.Len(t, users, 3)
	assert.Contains(t, users, "user-1")
	assert.Contains(t, users, "user-2")
	assert.Contains(t, users, "user-3")
}

// ============================================================================
// User Story 17: Server Streaming with Delay
// ============================================================================

func TestGRPC_US17_ServerStreamingWithDelay(t *testing.T) {
	port := getFreeGRPCPort(t)

	cfg := &grpc.GRPCConfig{
		ID:        "test-streaming-delay",
		Port:      port,
		ProtoFile: testProtoFile,
		Enabled:   true,
		Services: map[string]grpc.ServiceConfig{
			"test.UserService": {
				Methods: map[string]grpc.MethodConfig{
					"ListUsers": {
						Responses: []interface{}{
							map[string]interface{}{"id": "user-1", "name": "User 1", "email": "user1@example.com"},
							map[string]interface{}{"id": "user-2", "name": "User 2", "email": "user2@example.com"},
						},
						StreamDelay: "100ms",
					},
				},
			},
		},
	}

	_ = setupGRPCServer(t, cfg)
	files := getJhumpDescriptors(t)
	stub := createDynamicStub(t, port)

	methodDesc := getJhumpMethodDesc(t, files, "test.UserService", "ListUsers")

	inputDesc := methodDesc.GetInputType()
	reqMsg := dynamic.NewMessage(inputDesc)
	reqMsg.SetFieldByName("page_size", int32(10))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()

	stream, err := stub.InvokeRpcServerStream(ctx, methodDesc, reqMsg)
	require.NoError(t, err)

	count := 0
	for {
		_, err := stream.RecvMsg()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		count++
	}

	elapsed := time.Since(start)

	assert.Equal(t, 2, count)
	// Should have at least 100ms delay between messages (only 1 delay for 2 messages)
	assert.GreaterOrEqual(t, elapsed.Milliseconds(), int64(80), "Should have stream delay")
}
