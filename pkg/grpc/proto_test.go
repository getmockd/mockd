package grpc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func getTestProtoPath(t *testing.T) string {
	t.Helper()
	// Find the test proto file relative to the project root
	wd, err := os.Getwd()
	require.NoError(t, err)

	// Navigate from pkg/grpc to project root
	projectRoot := filepath.Join(wd, "..", "..")
	protoPath := filepath.Join(projectRoot, "tests", "fixtures", "grpc", "test.proto")

	// Verify the file exists
	_, err = os.Stat(protoPath)
	require.NoError(t, err, "test.proto fixture not found at %s", protoPath)

	return protoPath
}

func TestParseProtoFile(t *testing.T) {
	protoPath := getTestProtoPath(t)

	schema, err := ParseProtoFile(protoPath, nil)
	require.NoError(t, err)
	require.NotNil(t, schema)

	// Should have 2 services
	assert.Equal(t, 2, schema.ServiceCount())
}

func TestParseProtoFiles(t *testing.T) {
	protoPath := getTestProtoPath(t)

	schema, err := ParseProtoFiles([]string{protoPath}, nil)
	require.NoError(t, err)
	require.NotNil(t, schema)

	assert.Equal(t, 2, schema.ServiceCount())
}

func TestParseProtoFilesEmpty(t *testing.T) {
	schema, err := ParseProtoFiles([]string{}, nil)
	assert.ErrorIs(t, err, ErrNoProtoFiles)
	assert.Nil(t, schema)
}

func TestParseProtoFileNotFound(t *testing.T) {
	schema, err := ParseProtoFile("/nonexistent/path/to/file.proto", nil)
	assert.Error(t, err)
	assert.Nil(t, schema)
}

func TestListServices(t *testing.T) {
	protoPath := getTestProtoPath(t)

	schema, err := ParseProtoFile(protoPath, nil)
	require.NoError(t, err)

	services := schema.ListServices()
	assert.Len(t, services, 2)
	assert.Contains(t, services, "test.UserService")
	assert.Contains(t, services, "test.HealthService")
}

func TestGetService(t *testing.T) {
	protoPath := getTestProtoPath(t)

	schema, err := ParseProtoFile(protoPath, nil)
	require.NoError(t, err)

	// Get existing service
	svc := schema.GetService("test.UserService")
	require.NotNil(t, svc)
	assert.Equal(t, "test.UserService", svc.Name)

	// Get non-existing service
	svc = schema.GetService("test.NonExistent")
	assert.Nil(t, svc)
}

func TestServiceMethods(t *testing.T) {
	protoPath := getTestProtoPath(t)

	schema, err := ParseProtoFile(protoPath, nil)
	require.NoError(t, err)

	svc := schema.GetService("test.UserService")
	require.NotNil(t, svc)

	methods := svc.ListMethods()
	assert.Len(t, methods, 4)
	assert.Contains(t, methods, "GetUser")
	assert.Contains(t, methods, "ListUsers")
	assert.Contains(t, methods, "CreateUsers")
	assert.Contains(t, methods, "Chat")
}

func TestGetMethod(t *testing.T) {
	protoPath := getTestProtoPath(t)

	schema, err := ParseProtoFile(protoPath, nil)
	require.NoError(t, err)

	svc := schema.GetService("test.UserService")
	require.NotNil(t, svc)

	// Get existing method
	method := svc.GetMethod("GetUser")
	require.NotNil(t, method)
	assert.Equal(t, "GetUser", method.Name)

	// Get non-existing method
	method = svc.GetMethod("NonExistent")
	assert.Nil(t, method)
}

func TestMethodTypes(t *testing.T) {
	protoPath := getTestProtoPath(t)

	schema, err := ParseProtoFile(protoPath, nil)
	require.NoError(t, err)

	svc := schema.GetService("test.UserService")
	require.NotNil(t, svc)

	// Test unary method
	method := svc.GetMethod("GetUser")
	require.NotNil(t, method)
	assert.Equal(t, "test.GetUserRequest", method.InputType)
	assert.Equal(t, "test.User", method.OutputType)
}

func TestMethodStreamingTypes(t *testing.T) {
	protoPath := getTestProtoPath(t)

	schema, err := ParseProtoFile(protoPath, nil)
	require.NoError(t, err)

	svc := schema.GetService("test.UserService")
	require.NotNil(t, svc)

	tests := []struct {
		methodName      string
		isUnary         bool
		isServerStream  bool
		isClientStream  bool
		isBidirectional bool
		streamingType   string
	}{
		{
			methodName:    "GetUser",
			isUnary:       true,
			streamingType: "unary",
		},
		{
			methodName:     "ListUsers",
			isServerStream: true,
			streamingType:  "server_streaming",
		},
		{
			methodName:     "CreateUsers",
			isClientStream: true,
			streamingType:  "client_streaming",
		},
		{
			methodName:      "Chat",
			isBidirectional: true,
			streamingType:   "bidirectional",
		},
	}

	for _, tt := range tests {
		t.Run(tt.methodName, func(t *testing.T) {
			method := svc.GetMethod(tt.methodName)
			require.NotNil(t, method)

			assert.Equal(t, tt.isUnary, method.IsUnary(), "IsUnary")
			assert.Equal(t, tt.isServerStream, method.IsServerStreaming(), "IsServerStreaming")
			assert.Equal(t, tt.isClientStream, method.IsClientStreaming(), "IsClientStreaming")
			assert.Equal(t, tt.isBidirectional, method.IsBidirectional(), "IsBidirectional")
			assert.Equal(t, tt.streamingType, method.GetStreamingType())
		})
	}
}

func TestMethodDescriptors(t *testing.T) {
	protoPath := getTestProtoPath(t)

	schema, err := ParseProtoFile(protoPath, nil)
	require.NoError(t, err)

	svc := schema.GetService("test.UserService")
	require.NotNil(t, svc)

	method := svc.GetMethod("GetUser")
	require.NotNil(t, method)

	// Test underlying descriptors are accessible
	assert.NotNil(t, method.GetDescriptor())
	assert.NotNil(t, method.GetInputDescriptor())
	assert.NotNil(t, method.GetOutputDescriptor())
	assert.NotNil(t, svc.GetDescriptor())
}

func TestSchemaFiles(t *testing.T) {
	protoPath := getTestProtoPath(t)

	schema, err := ParseProtoFile(protoPath, nil)
	require.NoError(t, err)

	files := schema.Files()
	assert.Len(t, files, 1)
}

func TestMethodCount(t *testing.T) {
	protoPath := getTestProtoPath(t)

	schema, err := ParseProtoFile(protoPath, nil)
	require.NoError(t, err)

	// UserService has 4 methods, HealthService has 1 method
	assert.Equal(t, 5, schema.MethodCount())
}

func TestHealthService(t *testing.T) {
	protoPath := getTestProtoPath(t)

	schema, err := ParseProtoFile(protoPath, nil)
	require.NoError(t, err)

	svc := schema.GetService("test.HealthService")
	require.NotNil(t, svc)

	methods := svc.ListMethods()
	assert.Len(t, methods, 1)
	assert.Contains(t, methods, "Check")

	method := svc.GetMethod("Check")
	require.NotNil(t, method)
	assert.True(t, method.IsUnary())
	assert.Equal(t, "test.HealthCheckRequest", method.InputType)
	assert.Equal(t, "test.HealthCheckResponse", method.OutputType)
}
