package grpc

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// GRPCConfig represents a gRPC endpoint configuration for the mock server.
// It defines the port, proto files, and service configurations for handling
// gRPC requests.
type GRPCConfig struct {
	// ID is a unique identifier for this gRPC endpoint.
	ID string `json:"id" yaml:"id"`

	// Name is a human-readable name for this endpoint.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// ParentID is the folder ID this endpoint belongs to ("" = root level)
	ParentID string `json:"parentId,omitempty" yaml:"parentId,omitempty"`

	// MetaSortKey is used for manual ordering within a folder
	MetaSortKey float64 `json:"metaSortKey,omitempty" yaml:"metaSortKey,omitempty"`

	// Port is the TCP port number for the gRPC server.
	Port int `json:"port" yaml:"port"`

	// ProtoFile is the path to a single .proto file defining the service.
	// Use ProtoFiles for multiple files.
	ProtoFile string `json:"protoFile,omitempty" yaml:"protoFile,omitempty"`

	// ProtoFiles is a list of paths to .proto files.
	// Use this when services span multiple files.
	ProtoFiles []string `json:"protoFiles,omitempty" yaml:"protoFiles,omitempty"`

	// ImportPaths specifies directories to search for imported .proto files.
	// Similar to the -I flag in protoc.
	ImportPaths []string `json:"importPaths,omitempty" yaml:"importPaths,omitempty"`

	// Services configures mock responses for each gRPC service.
	// Keys are fully qualified service names (e.g., "package.ServiceName").
	Services map[string]ServiceConfig `json:"services,omitempty" yaml:"services,omitempty"`

	// Reflection enables gRPC server reflection, allowing clients like grpcurl
	// to discover services without the .proto files.
	Reflection bool `json:"reflection" yaml:"reflection"`

	// Enabled indicates whether this gRPC endpoint is active.
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// ServiceConfig configures mock responses for a gRPC service.
type ServiceConfig struct {
	// Methods maps method names to their mock configurations.
	Methods map[string]MethodConfig `json:"methods,omitempty" yaml:"methods,omitempty"`
}

// MethodConfig configures how a gRPC method responds to requests.
// For unary calls, use Response. For streaming, use Responses.
type MethodConfig struct {
	// Response is the mock response for unary calls.
	// Should be a map or struct matching the protobuf message schema.
	Response interface{} `json:"response,omitempty" yaml:"response,omitempty"`

	// Responses is a list of mock responses for streaming calls.
	// Each item is sent as a separate message in the stream.
	Responses []interface{} `json:"responses,omitempty" yaml:"responses,omitempty"`

	// Delay adds latency before sending the response.
	// Format: Go duration string (e.g., "100ms", "1s").
	Delay string `json:"delay,omitempty" yaml:"delay,omitempty"`

	// StreamDelay adds latency between each message in a stream.
	// Format: Go duration string (e.g., "50ms").
	StreamDelay string `json:"streamDelay,omitempty" yaml:"streamDelay,omitempty"`

	// Error configures the method to return a gRPC error instead of a response.
	Error *GRPCErrorConfig `json:"error,omitempty" yaml:"error,omitempty"`

	// Match specifies conditions that must be met for this config to apply.
	// If multiple MethodConfigs exist, the first matching one is used.
	Match *MethodMatch `json:"match,omitempty" yaml:"match,omitempty"`
}

// MethodMatch defines conditions for matching incoming gRPC requests.
// All specified conditions must match for the config to apply.
type MethodMatch struct {
	// Metadata matches against gRPC metadata (headers).
	// Keys are metadata names, values are expected values.
	Metadata map[string]string `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	// Request matches against fields in the request message.
	// Keys are field names, values are expected values.
	Request map[string]interface{} `json:"request,omitempty" yaml:"request,omitempty"`
}

// GRPCErrorConfig defines a gRPC error response.
type GRPCErrorConfig struct {
	// Code is the gRPC status code name.
	// Valid values: OK, CANCELLED, UNKNOWN, INVALID_ARGUMENT, DEADLINE_EXCEEDED,
	// NOT_FOUND, ALREADY_EXISTS, PERMISSION_DENIED, RESOURCE_EXHAUSTED,
	// FAILED_PRECONDITION, ABORTED, OUT_OF_RANGE, UNIMPLEMENTED, INTERNAL,
	// UNAVAILABLE, DATA_LOSS, UNAUTHENTICATED.
	Code string `json:"code" yaml:"code"`

	// Message is the human-readable error message.
	Message string `json:"message" yaml:"message"`

	// Details contains additional error details.
	// Structure depends on the error type being returned.
	Details map[string]interface{} `json:"details,omitempty" yaml:"details,omitempty"`
}

// GRPCStatusCode maps status code names to their integer values.
var GRPCStatusCode = map[string]int{
	"OK":                  0,
	"CANCELLED":           1,
	"UNKNOWN":             2,
	"INVALID_ARGUMENT":    3,
	"DEADLINE_EXCEEDED":   4,
	"NOT_FOUND":           5,
	"ALREADY_EXISTS":      6,
	"PERMISSION_DENIED":   7,
	"RESOURCE_EXHAUSTED":  8,
	"FAILED_PRECONDITION": 9,
	"ABORTED":             10,
	"OUT_OF_RANGE":        11,
	"UNIMPLEMENTED":       12,
	"INTERNAL":            13,
	"UNAVAILABLE":         14,
	"DATA_LOSS":           15,
	"UNAUTHENTICATED":     16,
}

// ValidateStatusCode checks if a status code name is valid.
func ValidateStatusCode(code string) bool {
	_, ok := GRPCStatusCode[code]
	return ok
}

// GetProtoFiles returns all proto files from the config.
// It combines ProtoFile and ProtoFiles into a single slice.
func (c *GRPCConfig) GetProtoFiles() []string {
	var files []string
	if c.ProtoFile != "" {
		files = append(files, c.ProtoFile)
	}
	files = append(files, c.ProtoFiles...)
	return files
}

// Validate checks the configuration for common errors.
func (c *GRPCConfig) Validate() error {
	if c.ID == "" {
		return ErrMissingID
	}
	if c.Port <= 0 || c.Port > 65535 {
		return ErrInvalidPort
	}
	if c.ProtoFile == "" && len(c.ProtoFiles) == 0 {
		return ErrMissingProtoFile
	}
	return nil
}

// grpcStatusName maps integer status codes to their string names.
var grpcStatusName = func() map[int]string {
	m := make(map[int]string, len(GRPCStatusCode))
	for name, code := range GRPCStatusCode {
		m[code] = name
	}
	return m
}()

// UnmarshalJSON allows GRPCErrorConfig.Code to accept both string names
// (e.g., "NOT_FOUND") and integer codes (e.g., 5).
func (e *GRPCErrorConfig) UnmarshalJSON(data []byte) error {
	// Use an alias to avoid infinite recursion
	type Alias GRPCErrorConfig
	aux := &struct {
		Code json.RawMessage `json:"code"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}

	if len(aux.Code) == 0 {
		return nil
	}

	// Try as string first
	var codeStr string
	if err := json.Unmarshal(aux.Code, &codeStr); err == nil {
		e.Code = strings.ToUpper(codeStr)
		return nil
	}

	// Try as number
	var codeNum int
	if err := json.Unmarshal(aux.Code, &codeNum); err == nil {
		if name, ok := grpcStatusName[codeNum]; ok {
			e.Code = name
			return nil
		}
		return fmt.Errorf("unknown gRPC status code: %d", codeNum)
	}

	return fmt.Errorf("gRPC error code must be a string name or integer, got: %s", string(aux.Code))
}

// UnmarshalYAML allows GRPCErrorConfig.Code to accept both string names
// and integer codes in YAML configuration.
func (e *GRPCErrorConfig) UnmarshalYAML(unmarshal func(interface{}) error) error {
	// Use an alias to avoid infinite recursion
	type Alias GRPCErrorConfig
	aux := &struct {
		Code interface{} `yaml:"code"`
		*Alias
	}{
		Alias: (*Alias)(e),
	}

	if err := unmarshal(aux); err != nil {
		return err
	}

	switch v := aux.Code.(type) {
	case string:
		e.Code = strings.ToUpper(v)
	case int:
		if name, ok := grpcStatusName[v]; ok {
			e.Code = name
		} else {
			return fmt.Errorf("unknown gRPC status code: %d", v)
		}
	case float64:
		// YAML sometimes decodes integers as float64
		code := int(v)
		if name, ok := grpcStatusName[code]; ok {
			e.Code = name
		} else {
			return fmt.Errorf("unknown gRPC status code: %s", strconv.FormatFloat(v, 'f', -1, 64))
		}
	case nil:
		// Code not specified
	default:
		return fmt.Errorf("gRPC error code must be a string name or integer, got: %T", v)
	}

	return nil
}

// Validate checks the error config for common errors.
func (e *GRPCErrorConfig) Validate() error {
	if e.Code == "" {
		return ErrMissingErrorCode
	}
	if !ValidateStatusCode(e.Code) {
		return ErrInvalidStatusCode
	}
	return nil
}
