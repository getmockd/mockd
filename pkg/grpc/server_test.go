package grpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	pref "google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
)

func getTestSchema(t *testing.T) *ProtoSchema {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)

	projectRoot := filepath.Join(wd, "..", "..")
	protoPath := filepath.Join(projectRoot, "tests", "fixtures", "grpc", "test.proto")

	schema, err := ParseProtoFile(protoPath, nil)
	require.NoError(t, err)
	return schema
}

// getTestDescriptors returns jhump/protoreflect descriptors for dynamic client testing
func getTestDescriptors(t *testing.T) []*desc.FileDescriptor {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)

	projectRoot := filepath.Join(wd, "..", "..")
	protoPath := filepath.Join(projectRoot, "tests", "fixtures", "grpc", "test.proto")

	parser := protoparse.Parser{}
	files, err := parser.ParseFiles(protoPath)
	require.NoError(t, err)
	return files
}

// getMethodDesc returns the jhump method descriptor for a service/method name
func getMethodDesc(t *testing.T, files []*desc.FileDescriptor, serviceName, methodName string) *desc.MethodDescriptor {
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

func TestNewServer(t *testing.T) {
	schema := getTestSchema(t)

	tests := []struct {
		name    string
		config  *GRPCConfig
		schema  *ProtoSchema
		wantErr error
	}{
		{
			name:    "valid config and schema",
			config:  &GRPCConfig{Port: 50051},
			schema:  schema,
			wantErr: nil,
		},
		{
			name:    "nil config",
			config:  nil,
			schema:  schema,
			wantErr: ErrNilConfig,
		},
		{
			name:    "nil schema",
			config:  &GRPCConfig{Port: 50051},
			schema:  nil,
			wantErr: ErrNilSchema,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := NewServer(tt.config, tt.schema)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				assert.Nil(t, srv)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, srv)
			}
		})
	}
}

func TestServerStartStop(t *testing.T) {
	schema := getTestSchema(t)
	config := &GRPCConfig{
		Port:       0, // Let OS assign port
		Reflection: true,
	}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	// Should not be running initially
	assert.False(t, srv.IsRunning())

	// Start server
	err = srv.Start(context.Background())
	require.NoError(t, err)
	assert.True(t, srv.IsRunning())
	assert.NotEmpty(t, srv.Address())

	// Starting again should error
	err = srv.Start(context.Background())
	assert.ErrorIs(t, err, ErrServerAlreadyRunning)

	// Stop server
	err = srv.Stop(context.Background(), 5*time.Second)
	require.NoError(t, err)
	assert.False(t, srv.IsRunning())

	// Stopping again should be fine (no-op)
	err = srv.Stop(context.Background(), 5*time.Second)
	assert.NoError(t, err)
}

func TestUnaryCall(t *testing.T) {
	schema := getTestSchema(t)
	files := getTestDescriptors(t)
	config := &GRPCConfig{
		Port:       0,
		Reflection: true,
		Services: map[string]ServiceConfig{
			"test.UserService": {
				Methods: map[string]MethodConfig{
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

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)
	defer srv.Stop(context.Background(), 5*time.Second)

	// Connect client
	conn, err := grpc.NewClient(srv.Address(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	// Get method descriptor from jhump/protoreflect for dynamic client
	methodDesc := getMethodDesc(t, files, "test.UserService", "GetUser")

	// Create dynamic stub
	stub := grpcdynamic.NewStub(conn)

	// Create request
	reqMsg := dynamic.NewMessage(methodDesc.GetInputType())
	reqMsg.SetFieldByName("id", "user-123")

	// Make call
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	respMsg, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)
	require.NoError(t, err)

	// Check response
	resp := respMsg.(*dynamic.Message)
	assert.Equal(t, "user-123", resp.GetFieldByName("id"))
	assert.Equal(t, "Test User", resp.GetFieldByName("name"))
	assert.Equal(t, "test@example.com", resp.GetFieldByName("email"))
}

func TestUnaryCallWithDelay(t *testing.T) {
	schema := getTestSchema(t)
	files := getTestDescriptors(t)
	config := &GRPCConfig{
		Port: 0,
		Services: map[string]ServiceConfig{
			"test.HealthService": {
				Methods: map[string]MethodConfig{
					"Check": {
						Response: map[string]interface{}{
							"status": 1, // SERVING
						},
						Delay: "100ms",
					},
				},
			},
		},
	}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)
	defer srv.Stop(context.Background(), 5*time.Second)

	conn, err := grpc.NewClient(srv.Address(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	methodDesc := getMethodDesc(t, files, "test.HealthService", "Check")

	stub := grpcdynamic.NewStub(conn)
	reqMsg := dynamic.NewMessage(methodDesc.GetInputType())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	_, err = stub.InvokeRpc(ctx, methodDesc, reqMsg)
	elapsed := time.Since(start)

	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 100*time.Millisecond)
}

func TestUnaryCallWithError(t *testing.T) {
	schema := getTestSchema(t)
	files := getTestDescriptors(t)
	config := &GRPCConfig{
		Port: 0,
		Services: map[string]ServiceConfig{
			"test.UserService": {
				Methods: map[string]MethodConfig{
					"GetUser": {
						Error: &GRPCErrorConfig{
							Code:    "NOT_FOUND",
							Message: "user not found",
						},
					},
				},
			},
		},
	}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)
	defer srv.Stop(context.Background(), 5*time.Second)

	conn, err := grpc.NewClient(srv.Address(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	methodDesc := getMethodDesc(t, files, "test.UserService", "GetUser")

	stub := grpcdynamic.NewStub(conn)
	reqMsg := dynamic.NewMessage(methodDesc.GetInputType())
	reqMsg.SetFieldByName("id", "not-found")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = stub.InvokeRpc(ctx, methodDesc, reqMsg)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Equal(t, "user not found", st.Message())
}

func TestServerStreaming(t *testing.T) {
	schema := getTestSchema(t)
	files := getTestDescriptors(t)
	config := &GRPCConfig{
		Port: 0,
		Services: map[string]ServiceConfig{
			"test.UserService": {
				Methods: map[string]MethodConfig{
					"ListUsers": {
						Responses: []interface{}{
							map[string]interface{}{"id": "user-1", "name": "User One"},
							map[string]interface{}{"id": "user-2", "name": "User Two"},
							map[string]interface{}{"id": "user-3", "name": "User Three"},
						},
					},
				},
			},
		},
	}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)
	defer srv.Stop(context.Background(), 5*time.Second)

	conn, err := grpc.NewClient(srv.Address(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	methodDesc := getMethodDesc(t, files, "test.UserService", "ListUsers")

	stub := grpcdynamic.NewStub(conn)
	reqMsg := dynamic.NewMessage(methodDesc.GetInputType())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := stub.InvokeRpcServerStream(ctx, methodDesc, reqMsg)
	require.NoError(t, err)

	var responses []*dynamic.Message
	for {
		resp, err := stream.RecvMsg()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		responses = append(responses, resp.(*dynamic.Message))
	}

	require.Len(t, responses, 3)
	assert.Equal(t, "user-1", responses[0].GetFieldByName("id"))
	assert.Equal(t, "user-2", responses[1].GetFieldByName("id"))
	assert.Equal(t, "user-3", responses[2].GetFieldByName("id"))
}

func TestServerStreamingWithStreamDelay(t *testing.T) {
	schema := getTestSchema(t)
	files := getTestDescriptors(t)
	config := &GRPCConfig{
		Port: 0,
		Services: map[string]ServiceConfig{
			"test.UserService": {
				Methods: map[string]MethodConfig{
					"ListUsers": {
						Responses: []interface{}{
							map[string]interface{}{"id": "user-1"},
							map[string]interface{}{"id": "user-2"},
						},
						StreamDelay: "50ms",
					},
				},
			},
		},
	}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)
	defer srv.Stop(context.Background(), 5*time.Second)

	conn, err := grpc.NewClient(srv.Address(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	methodDesc := getMethodDesc(t, files, "test.UserService", "ListUsers")

	stub := grpcdynamic.NewStub(conn)
	reqMsg := dynamic.NewMessage(methodDesc.GetInputType())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := stub.InvokeRpcServerStream(ctx, methodDesc, reqMsg)
	require.NoError(t, err)

	start := time.Now()
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
	// Should have at least 50ms delay between messages
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)
}

func TestClientStreaming(t *testing.T) {
	schema := getTestSchema(t)
	files := getTestDescriptors(t)
	config := &GRPCConfig{
		Port: 0,
		Services: map[string]ServiceConfig{
			"test.UserService": {
				Methods: map[string]MethodConfig{
					"CreateUsers": {
						Response: map[string]interface{}{
							"created_count": 3,
							"ids":           []string{"id-1", "id-2", "id-3"},
						},
					},
				},
			},
		},
	}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)
	defer srv.Stop(context.Background(), 5*time.Second)

	conn, err := grpc.NewClient(srv.Address(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	methodDesc := getMethodDesc(t, files, "test.UserService", "CreateUsers")

	stub := grpcdynamic.NewStub(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := stub.InvokeRpcClientStream(ctx, methodDesc)
	require.NoError(t, err)

	// Send multiple requests
	for i := 0; i < 3; i++ {
		reqMsg := dynamic.NewMessage(methodDesc.GetInputType())
		reqMsg.SetFieldByName("name", fmt.Sprintf("User %d", i))
		reqMsg.SetFieldByName("email", fmt.Sprintf("user%d@example.com", i))
		err = stream.SendMsg(reqMsg)
		require.NoError(t, err)
	}

	resp, err := stream.CloseAndReceive()
	require.NoError(t, err)

	respMsg := resp.(*dynamic.Message)
	assert.Equal(t, int32(3), respMsg.GetFieldByName("created_count"))
}

func TestBidirectionalStreaming(t *testing.T) {
	schema := getTestSchema(t)
	files := getTestDescriptors(t)
	config := &GRPCConfig{
		Port: 0,
		Services: map[string]ServiceConfig{
			"test.UserService": {
				Methods: map[string]MethodConfig{
					"Chat": {
						Responses: []interface{}{
							map[string]interface{}{"user_id": "bot", "content": "Hello!"},
							map[string]interface{}{"user_id": "bot", "content": "How are you?"},
						},
					},
				},
			},
		},
	}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)
	defer srv.Stop(context.Background(), 5*time.Second)

	conn, err := grpc.NewClient(srv.Address(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	methodDesc := getMethodDesc(t, files, "test.UserService", "Chat")

	stub := grpcdynamic.NewStub(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := stub.InvokeRpcBidiStream(ctx, methodDesc)
	require.NoError(t, err)

	// Send messages and receive responses
	var responses []*dynamic.Message
	for i := 0; i < 2; i++ {
		reqMsg := dynamic.NewMessage(methodDesc.GetInputType())
		reqMsg.SetFieldByName("user_id", "user-1")
		reqMsg.SetFieldByName("content", fmt.Sprintf("Message %d", i))

		err = stream.SendMsg(reqMsg)
		require.NoError(t, err)

		resp, err := stream.RecvMsg()
		require.NoError(t, err)
		responses = append(responses, resp.(*dynamic.Message))
	}

	err = stream.CloseSend()
	require.NoError(t, err)

	require.Len(t, responses, 2)
	assert.Equal(t, "bot", responses[0].GetFieldByName("user_id"))
	assert.Equal(t, "Hello!", responses[0].GetFieldByName("content"))
}

func TestMetadataMatching(t *testing.T) {
	schema := getTestSchema(t)
	files := getTestDescriptors(t)
	config := &GRPCConfig{
		Port: 0,
		Services: map[string]ServiceConfig{
			"test.UserService": {
				Methods: map[string]MethodConfig{
					"GetUser": {
						Response: map[string]interface{}{
							"id":   "matched-user",
							"name": "Matched by metadata",
						},
						Match: &MethodMatch{
							Metadata: map[string]string{
								"x-api-key": "secret-key",
							},
						},
					},
				},
			},
		},
	}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)
	defer srv.Stop(context.Background(), 5*time.Second)

	conn, err := grpc.NewClient(srv.Address(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	methodDesc := getMethodDesc(t, files, "test.UserService", "GetUser")

	stub := grpcdynamic.NewStub(conn)
	reqMsg := dynamic.NewMessage(methodDesc.GetInputType())
	reqMsg.SetFieldByName("id", "some-id")

	// With matching metadata
	ctx := metadata.NewOutgoingContext(context.Background(),
		metadata.Pairs("x-api-key", "secret-key"))
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)
	require.NoError(t, err)

	respMsg := resp.(*dynamic.Message)
	assert.Equal(t, "matched-user", respMsg.GetFieldByName("id"))

	// Without matching metadata
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()

	_, err = stub.InvokeRpc(ctx2, methodDesc, reqMsg)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unimplemented, st.Code())
}

func TestRequestFieldMatching(t *testing.T) {
	schema := getTestSchema(t)
	files := getTestDescriptors(t)
	config := &GRPCConfig{
		Port: 0,
		Services: map[string]ServiceConfig{
			"test.UserService": {
				Methods: map[string]MethodConfig{
					"GetUser": {
						Response: map[string]interface{}{
							"id":   "vip-user",
							"name": "VIP User",
						},
						Match: &MethodMatch{
							Request: map[string]interface{}{
								"id": "vip-123",
							},
						},
					},
				},
			},
		},
	}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)
	defer srv.Stop(context.Background(), 5*time.Second)

	conn, err := grpc.NewClient(srv.Address(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	methodDesc := getMethodDesc(t, files, "test.UserService", "GetUser")

	stub := grpcdynamic.NewStub(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Matching request
	reqMsg := dynamic.NewMessage(methodDesc.GetInputType())
	reqMsg.SetFieldByName("id", "vip-123")

	resp, err := stub.InvokeRpc(ctx, methodDesc, reqMsg)
	require.NoError(t, err)

	respMsg := resp.(*dynamic.Message)
	assert.Equal(t, "vip-user", respMsg.GetFieldByName("id"))

	// Non-matching request
	reqMsg2 := dynamic.NewMessage(methodDesc.GetInputType())
	reqMsg2.SetFieldByName("id", "regular-user")

	_, err = stub.InvokeRpc(ctx, methodDesc, reqMsg2)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unimplemented, st.Code())
}

func TestNoMockConfigured(t *testing.T) {
	schema := getTestSchema(t)
	files := getTestDescriptors(t)
	config := &GRPCConfig{
		Port:     0,
		Services: map[string]ServiceConfig{}, // No services configured
	}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)
	defer srv.Stop(context.Background(), 5*time.Second)

	conn, err := grpc.NewClient(srv.Address(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	methodDesc := getMethodDesc(t, files, "test.UserService", "GetUser")

	stub := grpcdynamic.NewStub(conn)
	reqMsg := dynamic.NewMessage(methodDesc.GetInputType())
	reqMsg.SetFieldByName("id", "any-id")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = stub.InvokeRpc(ctx, methodDesc, reqMsg)
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unimplemented, st.Code())
}

func TestBuildResponse(t *testing.T) {
	schema := getTestSchema(t)
	config := &GRPCConfig{Port: 0}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	svc := schema.GetService("test.UserService")
	require.NotNil(t, svc)
	method := svc.GetMethod("GetUser")
	require.NotNil(t, method)

	tests := []struct {
		name    string
		data    interface{}
		wantErr bool
	}{
		{
			name: "valid map data",
			data: map[string]interface{}{
				"id":    "123",
				"name":  "Test",
				"email": "test@example.com",
			},
			wantErr: false,
		},
		{
			name:    "nil data",
			data:    nil,
			wantErr: false,
		},
		{
			name: "nested data",
			data: map[string]interface{}{
				"id": "456",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := srv.buildResponse(method, tt.data, nil)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
			}
		})
	}
}

func TestToGRPCError(t *testing.T) {
	srv := &Server{}

	tests := []struct {
		name       string
		errCfg     *GRPCErrorConfig
		wantCode   codes.Code
		wantMsg    string
		wantNilErr bool
	}{
		{
			name:       "nil error config",
			errCfg:     nil,
			wantNilErr: true,
		},
		{
			name: "not found error",
			errCfg: &GRPCErrorConfig{
				Code:    "NOT_FOUND",
				Message: "resource not found",
			},
			wantCode: codes.NotFound,
			wantMsg:  "resource not found",
		},
		{
			name: "permission denied",
			errCfg: &GRPCErrorConfig{
				Code:    "PERMISSION_DENIED",
				Message: "access denied",
			},
			wantCode: codes.PermissionDenied,
			wantMsg:  "access denied",
		},
		{
			name: "invalid code defaults to unknown",
			errCfg: &GRPCErrorConfig{
				Code:    "INVALID_CODE",
				Message: "unknown error",
			},
			wantCode: codes.Unknown,
			wantMsg:  "unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := srv.toGRPCError(tt.errCfg)
			if tt.wantNilErr {
				assert.Nil(t, err)
				return
			}

			require.NotNil(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, tt.wantCode, st.Code())
			assert.Equal(t, tt.wantMsg, st.Message())
		})
	}
}

func TestApplyDelay(t *testing.T) {
	srv := &Server{}

	tests := []struct {
		name        string
		delay       string
		minDuration time.Duration
	}{
		{
			name:        "empty delay",
			delay:       "",
			minDuration: 0,
		},
		{
			name:        "50ms delay",
			delay:       "50ms",
			minDuration: 50 * time.Millisecond,
		},
		{
			name:        "invalid delay format",
			delay:       "invalid",
			minDuration: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			srv.applyDelay(tt.delay)
			elapsed := time.Since(start)
			assert.GreaterOrEqual(t, elapsed, tt.minDuration)
		})
	}
}

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
		want     bool
	}{
		{
			name:     "both nil",
			expected: nil,
			actual:   nil,
			want:     true,
		},
		{
			name:     "expected nil",
			expected: nil,
			actual:   "value",
			want:     false,
		},
		{
			name:     "actual nil",
			expected: "value",
			actual:   nil,
			want:     false,
		},
		{
			name:     "equal strings",
			expected: "test",
			actual:   "test",
			want:     true,
		},
		{
			name:     "different strings",
			expected: "test",
			actual:   "other",
			want:     false,
		},
		{
			name:     "float64 to int",
			expected: float64(123),
			actual:   123,
			want:     true,
		},
		{
			name:     "float64 to int32",
			expected: float64(123),
			actual:   int32(123),
			want:     true,
		},
		{
			name:     "int to float64",
			expected: 123,
			actual:   float64(123),
			want:     true,
		},
		{
			name:     "equal floats",
			expected: float64(3.14),
			actual:   float64(3.14),
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valuesEqual(tt.expected, tt.actual)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDynamicMessageToMap(t *testing.T) {
	schema := getTestSchema(t)
	svc := schema.GetService("test.UserService")
	require.NotNil(t, svc)
	method := svc.GetMethod("GetUser")
	require.NotNil(t, method)

	// Create a dynamicpb message using protoreflect descriptor
	msg := dynamicpb.NewMessage(method.GetInputDescriptor())
	msg.ProtoReflect().Set(
		msg.ProtoReflect().Descriptor().Fields().ByName("id"),
		pref.ValueOfString("test-123"),
	)

	result := dynamicMessageToMap(msg)
	assert.NotNil(t, result)
	assert.Equal(t, "test-123", result["id"])
}

func TestDynamicMessageToMapNil(t *testing.T) {
	result := dynamicMessageToMap(nil)
	assert.Nil(t, result)
}

func TestGetDescriptors(t *testing.T) {
	schema := getTestSchema(t)
	config := &GRPCConfig{Port: 0}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	// Test GetServiceDescriptor
	svcDesc := srv.GetServiceDescriptor("test.UserService")
	assert.NotNil(t, svcDesc)
	assert.Equal(t, pref.FullName("test.UserService"), svcDesc.FullName())

	// Test non-existent service
	svcDesc = srv.GetServiceDescriptor("test.NonExistent")
	assert.Nil(t, svcDesc)

	// Test GetMethodDescriptor
	methodDesc := srv.GetMethodDescriptor("test.UserService", "GetUser")
	assert.NotNil(t, methodDesc)
	assert.Equal(t, pref.Name("GetUser"), methodDesc.Name())

	// Test non-existent method
	methodDesc = srv.GetMethodDescriptor("test.UserService", "NonExistent")
	assert.Nil(t, methodDesc)

	// Test non-existent service for method
	methodDesc = srv.GetMethodDescriptor("test.NonExistent", "GetUser")
	assert.Nil(t, methodDesc)
}

func TestMatchesCondition(t *testing.T) {
	srv := &Server{}

	tests := []struct {
		name  string
		match *MethodMatch
		md    metadata.MD
		req   map[string]interface{}
		want  bool
	}{
		{
			name:  "no conditions - matches",
			match: &MethodMatch{},
			md:    metadata.MD{},
			req:   map[string]interface{}{},
			want:  true,
		},
		{
			name: "metadata matches",
			match: &MethodMatch{
				Metadata: map[string]string{
					"x-api-key": "secret",
				},
			},
			md:   metadata.Pairs("x-api-key", "secret"),
			req:  nil,
			want: true,
		},
		{
			name: "metadata does not match",
			match: &MethodMatch{
				Metadata: map[string]string{
					"x-api-key": "secret",
				},
			},
			md:   metadata.Pairs("x-api-key", "wrong"),
			req:  nil,
			want: false,
		},
		{
			name: "metadata missing",
			match: &MethodMatch{
				Metadata: map[string]string{
					"x-api-key": "secret",
				},
			},
			md:   metadata.MD{},
			req:  nil,
			want: false,
		},
		{
			name: "request field matches",
			match: &MethodMatch{
				Request: map[string]interface{}{
					"id": "123",
				},
			},
			md:   nil,
			req:  map[string]interface{}{"id": "123"},
			want: true,
		},
		{
			name: "request field does not match",
			match: &MethodMatch{
				Request: map[string]interface{}{
					"id": "123",
				},
			},
			md:   nil,
			req:  map[string]interface{}{"id": "456"},
			want: false,
		},
		{
			name: "request field missing",
			match: &MethodMatch{
				Request: map[string]interface{}{
					"id": "123",
				},
			},
			md:   nil,
			req:  map[string]interface{}{},
			want: false,
		},
		{
			name: "both match",
			match: &MethodMatch{
				Metadata: map[string]string{
					"authorization": "Bearer token",
				},
				Request: map[string]interface{}{
					"id": "user-1",
				},
			},
			md:   metadata.Pairs("authorization", "Bearer token"),
			req:  map[string]interface{}{"id": "user-1"},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := srv.matchesCondition(tt.match, tt.md, tt.req)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestServerStreamingWithError(t *testing.T) {
	schema := getTestSchema(t)
	files := getTestDescriptors(t)
	config := &GRPCConfig{
		Port: 0,
		Services: map[string]ServiceConfig{
			"test.UserService": {
				Methods: map[string]MethodConfig{
					"ListUsers": {
						Error: &GRPCErrorConfig{
							Code:    "INTERNAL",
							Message: "database error",
						},
					},
				},
			},
		},
	}

	srv, err := NewServer(config, schema)
	require.NoError(t, err)

	err = srv.Start(context.Background())
	require.NoError(t, err)
	defer srv.Stop(context.Background(), 5*time.Second)

	conn, err := grpc.NewClient(srv.Address(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	defer conn.Close()

	methodDesc := getMethodDesc(t, files, "test.UserService", "ListUsers")

	stub := grpcdynamic.NewStub(conn)
	reqMsg := dynamic.NewMessage(methodDesc.GetInputType())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := stub.InvokeRpcServerStream(ctx, methodDesc, reqMsg)
	require.NoError(t, err)

	_, err = stream.RecvMsg()
	require.Error(t, err)

	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Equal(t, "database error", st.Message())
}

// Helper to avoid unused import warning
var _ = json.Marshal
