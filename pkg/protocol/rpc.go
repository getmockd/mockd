package protocol

// RPCHandler handles RPC-style protocols (gRPC, JSON-RPC, Thrift).
// Implement this interface for protocols that expose services and methods.
//
// Example implementation:
//
//	func (h *GRPCHandler) ListServices() []protocol.ServiceInfo {
//	    services := make([]protocol.ServiceInfo, 0)
//	    for _, svc := range h.server.GetServiceInfo() {
//	        methods := make([]string, 0, len(svc.Methods))
//	        for _, m := range svc.Methods {
//	            methods = append(methods, m.Name)
//	        }
//	        services = append(services, protocol.ServiceInfo{
//	            Name:     svc.Metadata.(string),
//	            FullName: svc.Metadata.(string),
//	            Methods:  methods,
//	        })
//	    }
//	    return services
//	}
type RPCHandler interface {
	Handler

	// ListServices returns available services.
	// Each service contains its name and list of method names.
	ListServices() []ServiceInfo

	// ListMethods returns methods for a specific service.
	// Returns empty slice if the service does not exist.
	ListMethods(service string) []MethodInfo
}

// ServiceInfo describes an RPC service.
// Returned by RPCHandler.ListServices() and used by the Admin API.
type ServiceInfo struct {
	// Name is the short service name.
	Name string `json:"name"`

	// FullName is the fully-qualified service name.
	// For gRPC, this is the package.service format.
	FullName string `json:"fullName"`

	// Description is an optional description of the service.
	Description string `json:"description,omitempty"`

	// Methods is a list of method names in this service.
	Methods []string `json:"methods"`
}

// MethodInfo describes an RPC method.
// Returned by RPCHandler.ListMethods() and used by the Admin API.
type MethodInfo struct {
	// Name is the short method name.
	Name string `json:"name"`

	// FullName is the fully-qualified method name.
	// For gRPC, this is the /package.service/method format.
	FullName string `json:"fullName"`

	// Description is an optional description of the method.
	Description string `json:"description,omitempty"`

	// RequestType is the name of the request message type.
	RequestType string `json:"requestType"`

	// ResponseType is the name of the response message type.
	ResponseType string `json:"responseType"`

	// ClientStreaming indicates if the client streams multiple messages.
	ClientStreaming bool `json:"clientStreaming"`

	// ServerStreaming indicates if the server streams multiple messages.
	ServerStreaming bool `json:"serverStreaming"`
}
