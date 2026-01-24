package mcp

import (
	"encoding/json"
	"io"
	"strconv"
	"strings"
)

// ParseRequest parses a JSON-RPC request from an io.Reader.
func ParseRequest(r io.Reader) (*JSONRPCRequest, *JSONRPCError) {
	var req JSONRPCRequest
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&req); err != nil {
		return nil, ParseError(err.Error())
	}

	if err := ValidateRequest(&req); err != nil {
		return nil, err
	}

	return &req, nil
}

// ParseRequestBytes parses a JSON-RPC request from bytes.
func ParseRequestBytes(data []byte) (*JSONRPCRequest, *JSONRPCError) {
	var req JSONRPCRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, ParseError(err.Error())
	}

	if err := ValidateRequest(&req); err != nil {
		return nil, err
	}

	return &req, nil
}

// ValidateRequest validates a JSON-RPC request.
func ValidateRequest(req *JSONRPCRequest) *JSONRPCError {
	if req.JSONRPC != "2.0" {
		return InvalidRequestError("jsonrpc must be \"2.0\"")
	}

	if req.Method == "" {
		return InvalidRequestError("method is required")
	}

	return nil
}

// MarshalResponse marshals a JSON-RPC response to bytes.
func MarshalResponse(resp *JSONRPCResponse) ([]byte, error) {
	return json.Marshal(resp)
}

// MarshalNotification marshals a JSON-RPC notification to bytes.
func MarshalNotification(notif *JSONRPCNotification) ([]byte, error) {
	return json.Marshal(notif)
}

// NewNotification creates a new JSON-RPC notification.
func NewNotification(method string, params interface{}) *JSONRPCNotification {
	return &JSONRPCNotification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	}
}

// ResourceListChangedNotification creates a resources/list_changed notification.
func ResourceListChangedNotification() *JSONRPCNotification {
	return NewNotification("notifications/resources/list_changed", nil)
}

// ResourceUpdatedNotification creates a resources/updated notification.
func ResourceUpdatedNotification(uri string) *JSONRPCNotification {
	return NewNotification("notifications/resources/updated", &ResourceUpdatedParams{
		URI: uri,
	})
}

// UnmarshalParams unmarshals request params into a typed struct.
func UnmarshalParams[T any](params json.RawMessage) (*T, *JSONRPCError) {
	if len(params) == 0 {
		// Return zero value for optional params
		var result T
		return &result, nil
	}

	var result T
	if err := json.Unmarshal(params, &result); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	return &result, nil
}

// UnmarshalParamsRequired unmarshals required request params.
func UnmarshalParamsRequired[T any](params json.RawMessage) (*T, *JSONRPCError) {
	if len(params) == 0 {
		return nil, InvalidParamsError("params required")
	}

	var result T
	if err := json.Unmarshal(params, &result); err != nil {
		return nil, InvalidParamsError(err.Error())
	}
	return &result, nil
}

// ToolResultText creates a text content tool result.
func ToolResultText(text string) *ToolResult {
	return &ToolResult{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: text,
			},
		},
		IsError: false,
	}
}

// ToolResultJSON creates a JSON content tool result.
func ToolResultJSON(data interface{}) (*ToolResult, error) {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return ToolResultText(string(jsonBytes)), nil
}

// ToolResultError creates an error tool result.
func ToolResultError(message string) *ToolResult {
	return &ToolResult{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: message,
			},
		},
		IsError: true,
	}
}

// ToolResultErrorf creates a formatted error tool result.
func ToolResultErrorf(format string, args ...interface{}) *ToolResult {
	return ToolResultError(formatString(format, args...))
}

// formatString is a simple format function for error messages.
func formatString(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}

	var result strings.Builder
	result.Grow(len(format) + len(args)*8)

	argIndex := 0
	for i := 0; i < len(format); i++ {
		if format[i] == '%' && i+1 < len(format) {
			next := format[i+1]
			switch next {
			case 'v', 's', 'd':
				if argIndex < len(args) {
					result.WriteString(argToString(args[argIndex]))
					argIndex++
				} else {
					result.WriteByte('%')
					result.WriteByte(next)
				}
				i++ // Skip the format specifier
				continue
			case '%':
				result.WriteByte('%')
				i++
				continue
			}
		}
		result.WriteByte(format[i])
	}

	return result.String()
}

// argToString converts an argument to string.
func argToString(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		return v
	case error:
		return v.Error()
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		if v {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// BatchRequest represents a batch of JSON-RPC requests.
// Note: Batch requests are not currently supported in mockd MCP implementation.
type BatchRequest []*JSONRPCRequest

// BatchResponse represents a batch of JSON-RPC responses.
type BatchResponse []*JSONRPCResponse
