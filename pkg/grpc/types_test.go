package grpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGRPCConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  GRPCConfig
		wantErr error
	}{
		{
			name: "valid config with single proto file",
			config: GRPCConfig{
				ID:        "test",
				Port:      50051,
				ProtoFile: "test.proto",
			},
			wantErr: nil,
		},
		{
			name: "valid config with multiple proto files",
			config: GRPCConfig{
				ID:         "test",
				Port:       50051,
				ProtoFiles: []string{"a.proto", "b.proto"},
			},
			wantErr: nil,
		},
		{
			name: "missing id",
			config: GRPCConfig{
				Port:      50051,
				ProtoFile: "test.proto",
			},
			wantErr: ErrMissingID,
		},
		{
			name: "port zero",
			config: GRPCConfig{
				ID:        "test",
				Port:      0,
				ProtoFile: "test.proto",
			},
			wantErr: ErrInvalidPort,
		},
		{
			name: "port negative",
			config: GRPCConfig{
				ID:        "test",
				Port:      -1,
				ProtoFile: "test.proto",
			},
			wantErr: ErrInvalidPort,
		},
		{
			name: "port too high",
			config: GRPCConfig{
				ID:        "test",
				Port:      65536,
				ProtoFile: "test.proto",
			},
			wantErr: ErrInvalidPort,
		},
		{
			name: "missing proto files",
			config: GRPCConfig{
				ID:   "test",
				Port: 50051,
			},
			wantErr: ErrMissingProtoFile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestGRPCConfigGetProtoFiles(t *testing.T) {
	tests := []struct {
		name   string
		config GRPCConfig
		want   []string
	}{
		{
			name: "single proto file",
			config: GRPCConfig{
				ProtoFile: "a.proto",
			},
			want: []string{"a.proto"},
		},
		{
			name: "multiple proto files",
			config: GRPCConfig{
				ProtoFiles: []string{"a.proto", "b.proto"},
			},
			want: []string{"a.proto", "b.proto"},
		},
		{
			name: "both single and multiple",
			config: GRPCConfig{
				ProtoFile:  "main.proto",
				ProtoFiles: []string{"a.proto", "b.proto"},
			},
			want: []string{"main.proto", "a.proto", "b.proto"},
		},
		{
			name:   "no proto files",
			config: GRPCConfig{},
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.config.GetProtoFiles()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGRPCErrorConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  GRPCErrorConfig
		wantErr error
	}{
		{
			name: "valid error",
			config: GRPCErrorConfig{
				Code:    "NOT_FOUND",
				Message: "Resource not found",
			},
			wantErr: nil,
		},
		{
			name: "valid error with details",
			config: GRPCErrorConfig{
				Code:    "INVALID_ARGUMENT",
				Message: "Invalid input",
				Details: map[string]interface{}{"field": "name"},
			},
			wantErr: nil,
		},
		{
			name: "missing code",
			config: GRPCErrorConfig{
				Message: "Error message",
			},
			wantErr: ErrMissingErrorCode,
		},
		{
			name: "invalid status code",
			config: GRPCErrorConfig{
				Code:    "INVALID_CODE",
				Message: "Error message",
			},
			wantErr: ErrInvalidStatusCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateStatusCode(t *testing.T) {
	validCodes := []string{
		"OK", "CANCELLED", "UNKNOWN", "INVALID_ARGUMENT", "DEADLINE_EXCEEDED",
		"NOT_FOUND", "ALREADY_EXISTS", "PERMISSION_DENIED", "RESOURCE_EXHAUSTED",
		"FAILED_PRECONDITION", "ABORTED", "OUT_OF_RANGE", "UNIMPLEMENTED",
		"INTERNAL", "UNAVAILABLE", "DATA_LOSS", "UNAUTHENTICATED",
	}

	for _, code := range validCodes {
		t.Run(code, func(t *testing.T) {
			assert.True(t, ValidateStatusCode(code))
		})
	}

	invalidCodes := []string{"INVALID", "ERROR", "ok", "not_found", ""}
	for _, code := range invalidCodes {
		t.Run("invalid_"+code, func(t *testing.T) {
			assert.False(t, ValidateStatusCode(code))
		})
	}
}

func TestGRPCStatusCodeValues(t *testing.T) {
	// Verify the status code values match gRPC standard
	assert.Equal(t, 0, GRPCStatusCode["OK"])
	assert.Equal(t, 1, GRPCStatusCode["CANCELLED"])
	assert.Equal(t, 2, GRPCStatusCode["UNKNOWN"])
	assert.Equal(t, 3, GRPCStatusCode["INVALID_ARGUMENT"])
	assert.Equal(t, 4, GRPCStatusCode["DEADLINE_EXCEEDED"])
	assert.Equal(t, 5, GRPCStatusCode["NOT_FOUND"])
	assert.Equal(t, 6, GRPCStatusCode["ALREADY_EXISTS"])
	assert.Equal(t, 7, GRPCStatusCode["PERMISSION_DENIED"])
	assert.Equal(t, 8, GRPCStatusCode["RESOURCE_EXHAUSTED"])
	assert.Equal(t, 9, GRPCStatusCode["FAILED_PRECONDITION"])
	assert.Equal(t, 10, GRPCStatusCode["ABORTED"])
	assert.Equal(t, 11, GRPCStatusCode["OUT_OF_RANGE"])
	assert.Equal(t, 12, GRPCStatusCode["UNIMPLEMENTED"])
	assert.Equal(t, 13, GRPCStatusCode["INTERNAL"])
	assert.Equal(t, 14, GRPCStatusCode["UNAVAILABLE"])
	assert.Equal(t, 15, GRPCStatusCode["DATA_LOSS"])
	assert.Equal(t, 16, GRPCStatusCode["UNAUTHENTICATED"])
}
