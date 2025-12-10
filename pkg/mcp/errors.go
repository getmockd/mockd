package mcp

import "fmt"

// Standard JSON-RPC 2.0 error codes.
const (
	// ErrCodeParseError indicates invalid JSON was received.
	ErrCodeParseError = -32700

	// ErrCodeInvalidRequest indicates the JSON is not a valid JSON-RPC request.
	ErrCodeInvalidRequest = -32600

	// ErrCodeMethodNotFound indicates the method does not exist or is unavailable.
	ErrCodeMethodNotFound = -32601

	// ErrCodeInvalidParams indicates invalid method parameters.
	ErrCodeInvalidParams = -32602

	// ErrCodeInternalError indicates an internal JSON-RPC error.
	ErrCodeInternalError = -32603
)

// Custom mockd-specific error codes (-32001 to -32099).
const (
	// ErrCodeMockNotFound indicates no matching mock was found.
	ErrCodeMockNotFound = -32001

	// ErrCodeInvalidMock indicates invalid mock configuration.
	ErrCodeInvalidMock = -32002

	// ErrCodeResourceNotFound indicates unknown resource URI.
	ErrCodeResourceNotFound = -32003

	// ErrCodeSessionExpired indicates the session is no longer valid.
	ErrCodeSessionExpired = -32004

	// ErrCodeToolError indicates tool execution failed.
	ErrCodeToolError = -32005

	// ErrCodeSessionRequired indicates a session is required for this operation.
	ErrCodeSessionRequired = -32006

	// ErrCodeNotInitialized indicates the session has not been initialized.
	ErrCodeNotInitialized = -32007

	// ErrCodeStatefulResourceNotFound indicates the stateful resource was not found.
	ErrCodeStatefulResourceNotFound = -32008

	// ErrCodeProtocolVersion indicates unsupported protocol version.
	ErrCodeProtocolVersion = -32009
)

// Standard error messages.
var errorMessages = map[int]string{
	ErrCodeParseError:               "Parse error",
	ErrCodeInvalidRequest:           "Invalid request",
	ErrCodeMethodNotFound:           "Method not found",
	ErrCodeInvalidParams:            "Invalid params",
	ErrCodeInternalError:            "Internal error",
	ErrCodeMockNotFound:             "Mock not found",
	ErrCodeInvalidMock:              "Invalid mock configuration",
	ErrCodeResourceNotFound:         "Resource not found",
	ErrCodeSessionExpired:           "Session expired",
	ErrCodeToolError:                "Tool execution error",
	ErrCodeSessionRequired:          "Session required",
	ErrCodeNotInitialized:           "Session not initialized",
	ErrCodeStatefulResourceNotFound: "Stateful resource not found",
	ErrCodeProtocolVersion:          "Unsupported protocol version",
}

// NewJSONRPCError creates a new JSON-RPC error with the given code.
func NewJSONRPCError(code int, data interface{}) *JSONRPCError {
	msg, ok := errorMessages[code]
	if !ok {
		msg = "Unknown error"
	}
	return &JSONRPCError{
		Code:    code,
		Message: msg,
		Data:    data,
	}
}

// NewJSONRPCErrorWithMessage creates a JSON-RPC error with a custom message.
func NewJSONRPCErrorWithMessage(code int, message string, data interface{}) *JSONRPCError {
	return &JSONRPCError{
		Code:    code,
		Message: message,
		Data:    data,
	}
}

// ParseError creates a parse error response.
func ParseError(detail string) *JSONRPCError {
	return NewJSONRPCErrorWithMessage(ErrCodeParseError, "Parse error: "+detail, nil)
}

// InvalidRequestError creates an invalid request error.
func InvalidRequestError(detail string) *JSONRPCError {
	data := map[string]string{}
	if detail != "" {
		data["detail"] = detail
	}
	return NewJSONRPCError(ErrCodeInvalidRequest, data)
}

// MethodNotFoundError creates a method not found error.
func MethodNotFoundError(method string) *JSONRPCError {
	return NewJSONRPCError(ErrCodeMethodNotFound, map[string]string{
		"method": method,
	})
}

// InvalidParamsError creates an invalid params error.
func InvalidParamsError(detail string) *JSONRPCError {
	return NewJSONRPCErrorWithMessage(ErrCodeInvalidParams, "Invalid params: "+detail, nil)
}

// InternalError creates an internal error.
func InternalError(err error) *JSONRPCError {
	data := map[string]string{}
	if err != nil {
		data["detail"] = err.Error()
	}
	return NewJSONRPCError(ErrCodeInternalError, data)
}

// MockNotFoundError creates a mock not found error.
func MockNotFoundError(path, method string) *JSONRPCError {
	return NewJSONRPCError(ErrCodeMockNotFound, map[string]string{
		"path":   path,
		"method": method,
	})
}

// InvalidMockError creates an invalid mock error.
func InvalidMockError(detail string) *JSONRPCError {
	return NewJSONRPCError(ErrCodeInvalidMock, map[string]string{
		"detail": detail,
	})
}

// ResourceNotFoundError creates a resource not found error.
func ResourceNotFoundError(uri string) *JSONRPCError {
	return NewJSONRPCError(ErrCodeResourceNotFound, map[string]string{
		"uri": uri,
	})
}

// SessionExpiredError creates a session expired error.
func SessionExpiredError(sessionID string) *JSONRPCError {
	return NewJSONRPCError(ErrCodeSessionExpired, map[string]string{
		"sessionId": sessionID,
	})
}

// ToolError creates a tool execution error.
func ToolError(toolName string, err error) *JSONRPCError {
	data := map[string]string{
		"tool": toolName,
	}
	if err != nil {
		data["detail"] = err.Error()
	}
	return NewJSONRPCError(ErrCodeToolError, data)
}

// SessionRequiredError creates a session required error.
func SessionRequiredError() *JSONRPCError {
	return NewJSONRPCError(ErrCodeSessionRequired, nil)
}

// NotInitializedError creates a not initialized error.
func NotInitializedError() *JSONRPCError {
	return NewJSONRPCError(ErrCodeNotInitialized, nil)
}

// StatefulResourceNotFoundError creates a stateful resource not found error.
func StatefulResourceNotFoundError(resource string) *JSONRPCError {
	return NewJSONRPCError(ErrCodeStatefulResourceNotFound, map[string]string{
		"resource": resource,
	})
}

// ProtocolVersionError creates an unsupported protocol version error.
func ProtocolVersionError(requested, supported string) *JSONRPCError {
	return NewJSONRPCError(ErrCodeProtocolVersion, map[string]string{
		"requested": requested,
		"supported": supported,
	})
}

// Error implements the error interface for JSONRPCError.
func (e *JSONRPCError) Error() string {
	if e.Data != nil {
		return fmt.Sprintf("%s (%d): %v", e.Message, e.Code, e.Data)
	}
	return fmt.Sprintf("%s (%d)", e.Message, e.Code)
}

// ErrorResponse creates a JSON-RPC error response.
func ErrorResponse(id interface{}, err *JSONRPCError) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   err,
	}
}

// SuccessResponse creates a JSON-RPC success response.
func SuccessResponse(id interface{}, result interface{}) *JSONRPCResponse {
	return &JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}
