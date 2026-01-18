package grpc

import "errors"

// Configuration validation errors.
var (
	// ErrMissingID is returned when a GRPCConfig has no ID.
	ErrMissingID = errors.New("grpc config: missing id")

	// ErrInvalidPort is returned when a GRPCConfig has an invalid port number.
	ErrInvalidPort = errors.New("grpc config: port must be between 1 and 65535")

	// ErrMissingProtoFile is returned when no proto files are specified.
	ErrMissingProtoFile = errors.New("grpc config: at least one proto file is required")

	// ErrMissingErrorCode is returned when a GRPCErrorConfig has no code.
	ErrMissingErrorCode = errors.New("grpc error config: missing code")

	// ErrInvalidStatusCode is returned when a GRPCErrorConfig has an invalid code.
	ErrInvalidStatusCode = errors.New("grpc error config: invalid status code")
)

// Proto parsing errors.
var (
	// ErrNoProtoFiles is returned when ParseProtoFiles is called with an empty slice.
	ErrNoProtoFiles = errors.New("no proto files provided")

	// ErrServiceNotFound is returned when a requested service is not found in the schema.
	ErrServiceNotFound = errors.New("service not found")

	// ErrMethodNotFound is returned when a requested method is not found in the service.
	ErrMethodNotFound = errors.New("method not found")
)
