// Package grpc provides gRPC protocol support for the mockd engine.
//
// This package enables parsing of Protocol Buffer definition files (.proto) and
// provides configuration types for mocking gRPC services. It supports all gRPC
// streaming modes: unary, server streaming, client streaming, and bidirectional.
//
// # Proto Parsing
//
// The package uses protoreflect to parse .proto files and extract service and
// method definitions:
//
//	schema, err := grpc.ParseProtoFile("api/service.proto", []string{"api/"})
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// List all services
//	for _, svc := range schema.ListServices() {
//	    fmt.Println(svc)
//	}
//
//	// Get specific service and method
//	svc := schema.GetService("MyService")
//	method := svc.GetMethod("GetUser")
//
//	if method.IsUnary() {
//	    // Handle unary call
//	}
//
// # Configuration
//
// The package provides configuration types for defining gRPC mock behavior:
//
//	cfg := &grpc.GRPCConfig{
//	    ID:        "my-grpc-mock",
//	    Port:      50051,
//	    ProtoFile: "api/service.proto",
//	    Services: map[string]grpc.ServiceConfig{
//	        "UserService": {
//	            Methods: map[string]grpc.MethodConfig{
//	                "GetUser": {
//	                    Response: map[string]interface{}{
//	                        "id":   "123",
//	                        "name": "Test User",
//	                    },
//	                },
//	            },
//	        },
//	    },
//	    Reflection: true,
//	    Enabled:    true,
//	}
//
// # Streaming Support
//
// For streaming methods, use the Responses field to define multiple messages:
//
//	cfg := grpc.MethodConfig{
//	    Responses: []interface{}{
//	        map[string]interface{}{"message": "first"},
//	        map[string]interface{}{"message": "second"},
//	        map[string]interface{}{"message": "third"},
//	    },
//	    StreamDelay: "100ms", // Delay between stream messages
//	}
//
// # Error Responses
//
// Configure gRPC error responses using standard gRPC status codes:
//
//	cfg := grpc.MethodConfig{
//	    Error: &grpc.GRPCErrorConfig{
//	        Code:    "NOT_FOUND",
//	        Message: "User not found",
//	    },
//	}
//
// # Request Matching
//
// Match requests based on metadata or request fields:
//
//	cfg := grpc.MethodConfig{
//	    Match: &grpc.MethodMatch{
//	        Metadata: map[string]string{
//	            "authorization": "Bearer token123",
//	        },
//	        Request: map[string]interface{}{
//	            "user_id": "123",
//	        },
//	    },
//	    Response: map[string]interface{}{"name": "Matched User"},
//	}
package grpc
