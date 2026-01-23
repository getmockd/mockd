package grpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/getmockd/mockd/pkg/logging"
	"github.com/getmockd/mockd/pkg/metrics"
	"github.com/getmockd/mockd/pkg/protocol"
	"github.com/getmockd/mockd/pkg/requestlog"
	"github.com/getmockd/mockd/pkg/util"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/protoadapt"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Interface compliance checks.
var (
	_ protocol.Handler          = (*Server)(nil)
	_ protocol.StandaloneServer = (*Server)(nil)
	_ protocol.RPCHandler       = (*Server)(nil)
	_ protocol.RequestLoggable  = (*Server)(nil)
	_ protocol.Observable       = (*Server)(nil)
	_ protocol.Loggable         = (*Server)(nil)
)

// Server errors.
var (
	// ErrServerNotRunning is returned when attempting operations on a stopped server.
	ErrServerNotRunning = errors.New("server is not running")

	// ErrServerAlreadyRunning is returned when attempting to start a running server.
	ErrServerAlreadyRunning = errors.New("server is already running")

	// ErrNilConfig is returned when server config is nil.
	ErrNilConfig = errors.New("config cannot be nil")

	// ErrNilSchema is returned when proto schema is nil.
	ErrNilSchema = errors.New("schema cannot be nil")
)

// streamType identifies the type of gRPC call (for internal logging).
type streamType string

const (
	streamUnary        streamType = "unary"
	streamClientStream streamType = "client_stream"
	streamServerStream streamType = "server_stream"
	streamBidi         streamType = "bidi"
)

// Server is a mock gRPC server that serves configured responses
// based on proto schema definitions.
type Server struct {
	config     *GRPCConfig
	schema     *ProtoSchema
	grpcServer *grpc.Server
	listener   net.Listener
	mu         sync.RWMutex
	running    bool
	startedAt  time.Time
	log        *slog.Logger

	// Request logging support
	requestLoggerMu sync.RWMutex
	requestLogger   requestlog.Logger
}

// NewServer creates a new gRPC mock server.
func NewServer(config *GRPCConfig, schema *ProtoSchema) (*Server, error) {
	if config == nil {
		return nil, ErrNilConfig
	}
	if schema == nil {
		return nil, ErrNilSchema
	}

	return &Server{
		config: config,
		schema: schema,
		log:    logging.Nop(),
	}, nil
}

// SetLogger sets the operational logger for the server.
func (s *Server) SetLogger(log *slog.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if log != nil {
		s.log = log
	} else {
		s.log = logging.Nop()
	}
}

// Start starts the gRPC server.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return ErrServerAlreadyRunning
	}

	// Create listener
	addr := fmt.Sprintf(":%d", s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", addr, err)
	}
	s.listener = listener

	// Create gRPC server with unknown service handler
	s.grpcServer = grpc.NewServer(
		grpc.UnknownServiceHandler(s.handleStream),
	)

	// Register all services dynamically
	s.registerServices()

	// Enable reflection if configured
	if s.config.Reflection {
		reflection.Register(s.grpcServer)
	}

	// Start serving in a goroutine
	go func() {
		if err := s.grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			// Log error but don't crash - server may have been stopped
			s.log.Error("gRPC server error", "error", err)
		}
	}()

	s.running = true
	s.startedAt = time.Now()
	return nil
}

// Stop stops the gRPC server gracefully.
func (s *Server) Stop(ctx context.Context, timeout time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	if s.grpcServer != nil {
		// Create a channel to signal graceful stop completion
		done := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(done)
		}()

		// Wait for graceful stop or timeout
		select {
		case <-done:
			// Graceful stop completed
		case <-time.After(timeout):
			// Timeout - force stop
			s.grpcServer.Stop()
		case <-ctx.Done():
			// Context cancelled - force stop
			s.grpcServer.Stop()
		}
	}

	s.running = false
	s.startedAt = time.Time{}
	return nil
}

// IsRunning returns true if server is running.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// Address returns the address the server is listening on.
func (s *Server) Address() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

// registerServices registers all services from the schema with the gRPC server.
func (s *Server) registerServices() {
	for _, serviceName := range s.schema.ListServices() {
		svc := s.schema.GetService(serviceName)
		if svc == nil {
			continue
		}

		// Create service description for registration
		methods := make([]grpc.MethodDesc, 0)
		streams := make([]grpc.StreamDesc, 0)

		for _, methodName := range svc.ListMethods() {
			method := svc.GetMethod(methodName)
			if method == nil {
				continue
			}

			if method.IsUnary() {
				methods = append(methods, grpc.MethodDesc{
					MethodName: methodName,
					Handler:    s.makeUnaryHandler(serviceName, methodName),
				})
			} else {
				streams = append(streams, grpc.StreamDesc{
					StreamName:    methodName,
					Handler:       s.makeStreamHandler(serviceName, methodName),
					ServerStreams: method.ServerStreaming,
					ClientStreams: method.ClientStreaming,
				})
			}
		}

		// Register the service
		s.grpcServer.RegisterService(&grpc.ServiceDesc{
			ServiceName: serviceName,
			HandlerType: (*interface{})(nil),
			Methods:     methods,
			Streams:     streams,
			Metadata:    nil,
		}, struct{}{})
	}
}

// makeUnaryHandler creates a unary handler for a specific service/method.
func (s *Server) makeUnaryHandler(serviceName, methodName string) func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	return func(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
		return s.handleUnary(srv, ctx, dec, interceptor, serviceName, methodName)
	}
}

// makeStreamHandler creates a stream handler for a specific service/method.
func (s *Server) makeStreamHandler(serviceName, methodName string) func(srv interface{}, stream grpc.ServerStream) error {
	return func(srv interface{}, stream grpc.ServerStream) error {
		return s.handleStreamMethod(srv, stream, serviceName, methodName)
	}
}

// handleUnary handles unary RPC calls.
func (s *Server) handleUnary(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor, serviceName, methodName string) (interface{}, error) {
	startTime := time.Now()
	fullPath := fmt.Sprintf("/%s/%s", serviceName, methodName)

	// Get method descriptor
	svc := s.schema.GetService(serviceName)
	if svc == nil {
		err := status.Errorf(codes.Unimplemented, "service %s not found", serviceName)
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamUnary, nil, nil, nil, err)
		return nil, err
	}

	method := svc.GetMethod(methodName)
	if method == nil {
		err := status.Errorf(codes.Unimplemented, "method %s not found", methodName)
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamUnary, nil, nil, nil, err)
		return nil, err
	}

	// Create dynamic message for decoding request
	inputDesc := method.GetInputDescriptor()
	if inputDesc == nil {
		err := status.Error(codes.Internal, "cannot get input descriptor")
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamUnary, nil, nil, nil, err)
		return nil, err
	}

	reqMsg := dynamicpb.NewMessage(inputDesc)
	if err := dec(reqMsg); err != nil {
		grpcErr := status.Errorf(codes.InvalidArgument, "failed to decode request: %v", err)
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamUnary, nil, nil, nil, grpcErr)
		return nil, grpcErr
	}

	// Convert request to map for matching
	reqMap := dynamicMessageToMap(reqMsg)

	// Get metadata
	md, _ := metadata.FromIncomingContext(ctx)

	// Find matching method config
	methodCfg := s.findMethodConfig(serviceName, methodName, md, reqMap)
	if methodCfg == nil {
		err := status.Errorf(codes.Unimplemented, "no mock configured for %s/%s", serviceName, methodName)
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamUnary, md, reqMap, nil, err)
		return nil, err
	}

	// Apply delay if configured
	s.applyDelay(methodCfg.Delay)

	// Return error if configured
	if methodCfg.Error != nil {
		grpcErr := s.toGRPCError(methodCfg.Error)

		// Log the error call
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamUnary, md, reqMap, nil, grpcErr)

		return nil, grpcErr
	}

	// Build and return response
	resp, err := s.buildResponse(method, methodCfg.Response)
	if err != nil {
		grpcErr := status.Errorf(codes.Internal, "failed to build response: %v", err)
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamUnary, md, reqMap, nil, grpcErr)
		return nil, grpcErr
	}

	respMap := dynamicMessageToMap(resp)

	// Log the successful call
	s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamUnary, md, reqMap, respMap, nil)

	return resp, nil
}

// handleStream handles the unknown service handler (fallback for unregistered services).
func (s *Server) handleStream(srv interface{}, stream grpc.ServerStream) error {
	// Extract method info from stream
	fullMethod, ok := grpc.MethodFromServerStream(stream)
	if !ok {
		return status.Error(codes.Internal, "failed to get method from stream")
	}

	// Parse service and method name from full method path
	// Format: /package.ServiceName/MethodName
	parts := strings.Split(fullMethod, "/")
	if len(parts) != 3 {
		return status.Errorf(codes.Unimplemented, "invalid method path: %s", fullMethod)
	}

	serviceName := parts[1]
	methodName := parts[2]

	return s.handleStreamMethod(srv, stream, serviceName, methodName)
}

// handleStreamMethod handles streaming RPC calls for a specific method.
func (s *Server) handleStreamMethod(srv interface{}, stream grpc.ServerStream, serviceName, methodName string) error {
	// Get method descriptor
	svc := s.schema.GetService(serviceName)
	if svc == nil {
		return status.Errorf(codes.Unimplemented, "service %s not found", serviceName)
	}

	method := svc.GetMethod(methodName)
	if method == nil {
		return status.Errorf(codes.Unimplemented, "method %s not found", methodName)
	}

	// Get metadata
	md, _ := metadata.FromIncomingContext(stream.Context())

	// Handle based on streaming type
	switch {
	case method.IsServerStreaming():
		return s.handleServerStreaming(stream, method, serviceName, methodName, md)
	case method.IsClientStreaming():
		return s.handleClientStreaming(stream, method, serviceName, methodName, md)
	case method.IsBidirectional():
		return s.handleBidirectionalStreaming(stream, method, serviceName, methodName, md)
	default:
		// Unary - should be handled by handleUnary
		return status.Error(codes.Internal, "unary method in stream handler")
	}
}

// handleServerStreaming handles server-streaming RPC calls.
func (s *Server) handleServerStreaming(stream grpc.ServerStream, method *MethodDescriptor, serviceName, methodName string, md metadata.MD) error {
	startTime := time.Now()
	fullPath := fmt.Sprintf("/%s/%s", serviceName, methodName)

	// Track active stream connection
	if metrics.ActiveConnections != nil {
		if vec, err := metrics.ActiveConnections.WithLabels("grpc"); err == nil {
			vec.Inc()
			defer vec.Dec()
		}
	}

	// Read single request from client
	inputDesc := method.GetInputDescriptor()
	if inputDesc == nil {
		err := status.Error(codes.Internal, "cannot get input descriptor")
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamServerStream, md, nil, nil, err)
		return err
	}

	reqMsg := dynamicpb.NewMessage(inputDesc)
	if err := stream.RecvMsg(reqMsg); err != nil {
		grpcErr := status.Errorf(codes.InvalidArgument, "failed to receive request: %v", err)
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamServerStream, md, nil, nil, grpcErr)
		return grpcErr
	}

	reqMap := dynamicMessageToMap(reqMsg)

	// Find matching method config
	methodCfg := s.findMethodConfig(serviceName, methodName, md, reqMap)
	if methodCfg == nil {
		err := status.Errorf(codes.Unimplemented, "no mock configured for %s/%s", serviceName, methodName)
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamServerStream, md, reqMap, nil, err)
		return err
	}

	// Apply initial delay
	s.applyDelay(methodCfg.Delay)

	// Return error if configured
	if methodCfg.Error != nil {
		grpcErr := s.toGRPCError(methodCfg.Error)

		// Log the error call
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamServerStream, md, reqMap, nil, grpcErr)

		return grpcErr
	}

	// Send responses
	responses := methodCfg.Responses
	if len(responses) == 0 && methodCfg.Response != nil {
		// Use single response if Responses not specified
		responses = []interface{}{methodCfg.Response}
	}

	// Collect responses for logging
	var collectedResponses []interface{}

	for i, respData := range responses {
		resp, err := s.buildResponse(method, respData)
		if err != nil {
			grpcErr := status.Errorf(codes.Internal, "failed to build response: %v", err)
			s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamServerStream, md, reqMap, collectedResponses, grpcErr)
			return grpcErr
		}

		if err := stream.SendMsg(resp); err != nil {
			grpcErr := status.Errorf(codes.Internal, "failed to send response: %v", err)
			s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamServerStream, md, reqMap, collectedResponses, grpcErr)
			return grpcErr
		}

		// Collect for logging
		collectedResponses = append(collectedResponses, dynamicMessageToMap(resp))

		// Apply stream delay between messages (not after last)
		if i < len(responses)-1 {
			s.applyDelay(methodCfg.StreamDelay)
		}
	}

	// Log the successful call
	s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamServerStream, md, reqMap, collectedResponses, nil)

	return nil
}

// handleClientStreaming handles client-streaming RPC calls.
func (s *Server) handleClientStreaming(stream grpc.ServerStream, method *MethodDescriptor, serviceName, methodName string, md metadata.MD) error {
	startTime := time.Now()
	fullPath := fmt.Sprintf("/%s/%s", serviceName, methodName)

	// Track active stream connection
	if metrics.ActiveConnections != nil {
		if vec, err := metrics.ActiveConnections.WithLabels("grpc"); err == nil {
			vec.Inc()
			defer vec.Dec()
		}
	}

	inputDesc := method.GetInputDescriptor()
	if inputDesc == nil {
		err := status.Error(codes.Internal, "cannot get input descriptor")
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamClientStream, md, nil, nil, err)
		return err
	}

	// Read all messages from client (always collect for logging)
	var allRequests []interface{}
	var lastReqMap map[string]interface{}
	for {
		reqMsg := dynamicpb.NewMessage(inputDesc)
		if err := stream.RecvMsg(reqMsg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			grpcErr := status.Errorf(codes.InvalidArgument, "failed to receive request: %v", err)
			s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamClientStream, md, allRequests, nil, grpcErr)
			return grpcErr
		}
		lastReqMap = dynamicMessageToMap(reqMsg)
		allRequests = append(allRequests, lastReqMap)
	}

	// Find matching method config using last request
	methodCfg := s.findMethodConfig(serviceName, methodName, md, lastReqMap)
	if methodCfg == nil {
		err := status.Errorf(codes.Unimplemented, "no mock configured for %s/%s", serviceName, methodName)
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamClientStream, md, allRequests, nil, err)
		return err
	}

	// Apply delay
	s.applyDelay(methodCfg.Delay)

	// Return error if configured
	if methodCfg.Error != nil {
		grpcErr := s.toGRPCError(methodCfg.Error)

		// Log the error call
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamClientStream, md, allRequests, nil, grpcErr)

		return grpcErr
	}

	// Build and send response
	resp, err := s.buildResponse(method, methodCfg.Response)
	if err != nil {
		grpcErr := status.Errorf(codes.Internal, "failed to build response: %v", err)
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamClientStream, md, allRequests, nil, grpcErr)
		return grpcErr
	}

	if err := stream.SendMsg(resp); err != nil {
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamClientStream, md, allRequests, nil, err)
		return err
	}

	respMap := dynamicMessageToMap(resp)

	// Log the successful call
	s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamClientStream, md, allRequests, respMap, nil)

	return nil
}

// handleBidirectionalStreaming handles bidirectional streaming RPC calls.
func (s *Server) handleBidirectionalStreaming(stream grpc.ServerStream, method *MethodDescriptor, serviceName, methodName string, md metadata.MD) error {
	startTime := time.Now()
	fullPath := fmt.Sprintf("/%s/%s", serviceName, methodName)

	// Track active stream connection
	if metrics.ActiveConnections != nil {
		if vec, err := metrics.ActiveConnections.WithLabels("grpc"); err == nil {
			vec.Inc()
			defer vec.Dec()
		}
	}

	inputDesc := method.GetInputDescriptor()
	if inputDesc == nil {
		err := status.Error(codes.Internal, "cannot get input descriptor")
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamBidi, md, nil, nil, err)
		return err
	}

	// Find config based on metadata only (no request yet)
	methodCfg := s.findMethodConfig(serviceName, methodName, md, nil)
	if methodCfg == nil {
		err := status.Errorf(codes.Unimplemented, "no mock configured for %s/%s", serviceName, methodName)
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamBidi, md, nil, nil, err)
		return err
	}

	// Apply initial delay
	s.applyDelay(methodCfg.Delay)

	// Return error if configured
	if methodCfg.Error != nil {
		grpcErr := s.toGRPCError(methodCfg.Error)

		// Log the error call
		s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamBidi, md, nil, nil, grpcErr)

		return grpcErr
	}

	// Prepare responses
	responses := methodCfg.Responses
	if len(responses) == 0 && methodCfg.Response != nil {
		responses = []interface{}{methodCfg.Response}
	}

	respIndex := 0

	// Collect for logging
	var allRequests []interface{}
	var allResponses []interface{}

	// Echo pattern: for each received message, send a response
	for {
		reqMsg := dynamicpb.NewMessage(inputDesc)
		if err := stream.RecvMsg(reqMsg); err != nil {
			if errors.Is(err, io.EOF) {
				// Log the successful call
				s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamBidi, md, allRequests, allResponses, nil)

				return nil
			}
			grpcErr := status.Errorf(codes.InvalidArgument, "failed to receive request: %v", err)
			s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamBidi, md, allRequests, allResponses, grpcErr)
			return grpcErr
		}

		// Collect request for logging
		allRequests = append(allRequests, dynamicMessageToMap(reqMsg))

		// Send response if available
		if respIndex < len(responses) {
			resp, err := s.buildResponse(method, responses[respIndex])
			if err != nil {
				grpcErr := status.Errorf(codes.Internal, "failed to build response: %v", err)
				s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamBidi, md, allRequests, allResponses, grpcErr)
				return grpcErr
			}

			if err := stream.SendMsg(resp); err != nil {
				grpcErr := status.Errorf(codes.Internal, "failed to send response: %v", err)
				s.logGRPCCall(startTime, fullPath, serviceName, methodName, streamBidi, md, allRequests, allResponses, grpcErr)
				return grpcErr
			}

			// Collect response for logging
			allResponses = append(allResponses, dynamicMessageToMap(resp))

			respIndex++
			s.applyDelay(methodCfg.StreamDelay)
		}
	}
}

// findMethodConfig finds config for a method based on service name, method name,
// metadata, and request body matching.
func (s *Server) findMethodConfig(serviceName, methodName string, md metadata.MD, req map[string]interface{}) *MethodConfig {
	svcCfg, ok := s.config.Services[serviceName]
	if !ok {
		return nil
	}

	methodCfg, ok := svcCfg.Methods[methodName]
	if !ok {
		return nil
	}

	// Check if this config has a match condition
	if methodCfg.Match != nil {
		if !s.matchesCondition(methodCfg.Match, md, req) {
			return nil
		}
	}

	return &methodCfg
}

// matchesCondition checks if the request matches the match conditions.
func (s *Server) matchesCondition(match *MethodMatch, md metadata.MD, req map[string]interface{}) bool {
	// Check metadata matches
	for key, expectedValue := range match.Metadata {
		values := md.Get(key)
		if len(values) == 0 {
			return false
		}
		// Check if any value matches
		found := false
		for _, v := range values {
			if v == expectedValue {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check request field matches
	for key, expectedValue := range match.Request {
		if req == nil {
			return false
		}
		actualValue, exists := req[key]
		if !exists {
			return false
		}
		if !valuesEqual(expectedValue, actualValue) {
			return false
		}
	}

	return true
}

// valuesEqual compares two values for equality, handling type differences.
func valuesEqual(expected, actual interface{}) bool {
	// Handle nil
	if expected == nil && actual == nil {
		return true
	}
	if expected == nil || actual == nil {
		return false
	}

	// Handle numeric comparisons (JSON numbers come as float64)
	switch e := expected.(type) {
	case float64:
		switch a := actual.(type) {
		case float64:
			return e == a
		case int:
			return e == float64(a)
		case int32:
			return e == float64(a)
		case int64:
			return e == float64(a)
		case uint32:
			return e == float64(a)
		case uint64:
			return e == float64(a)
		}
	case int:
		if a, ok := actual.(int); ok {
			return e == a
		}
		if a, ok := actual.(float64); ok {
			return float64(e) == a
		}
	}

	// Deep equal for other types
	return reflect.DeepEqual(expected, actual)
}

// buildResponse builds a protobuf response from JSON config data.
// Returns a *dynamicpb.Message which can be used with gRPC streaming APIs.
func (s *Server) buildResponse(method *MethodDescriptor, data interface{}) (*dynamicpb.Message, error) {
	outputDesc := method.GetOutputDescriptor()
	if outputDesc == nil {
		return nil, errors.New("cannot get output descriptor")
	}

	respMsg := dynamicpb.NewMessage(outputDesc)

	if data == nil {
		return respMsg, nil
	}

	// Convert data to JSON and unmarshal into dynamic message using protojson
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}

	if err := protojson.Unmarshal(jsonData, respMsg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response into proto: %w", err)
	}

	return respMsg, nil
}

// applyDelay applies configured delay.
func (s *Server) applyDelay(delay string) {
	if delay == "" {
		return
	}

	d, err := time.ParseDuration(delay)
	if err != nil {
		return
	}

	if d > 0 {
		time.Sleep(d)
	}
}

// toGRPCError converts error config to gRPC status error.
func (s *Server) toGRPCError(errCfg *GRPCErrorConfig) error {
	if errCfg == nil {
		return nil
	}

	codeInt, ok := GRPCStatusCode[errCfg.Code]
	if !ok {
		codeInt = int(codes.Unknown)
	}

	st := status.New(codes.Code(codeInt), errCfg.Message)

	// Attach error details if provided
	if len(errCfg.Details) > 0 {
		details := s.buildErrorDetails(errCfg.Details)
		if len(details) > 0 {
			stWithDetails, err := st.WithDetails(details...)
			if err == nil {
				st = stWithDetails
			}
		}
	}

	return st.Err()
}

// buildErrorDetails converts a details map to protoadapt.MessageV1 slice.
// Supports common error detail types: BadRequest, ErrorInfo, RetryInfo,
// DebugInfo, QuotaFailure, PreconditionFailure, ResourceInfo, Help, LocalizedMessage.
func (s *Server) buildErrorDetails(details map[string]interface{}) []protoadapt.MessageV1 {
	var result []protoadapt.MessageV1

	// Handle BadRequest details
	if badReq, ok := details["bad_request"].(map[string]interface{}); ok {
		br := s.buildBadRequest(badReq)
		if br != nil {
			result = append(result, br)
		}
	}

	// Handle ErrorInfo details
	if errorInfo, ok := details["error_info"].(map[string]interface{}); ok {
		ei := s.buildErrorInfo(errorInfo)
		if ei != nil {
			result = append(result, ei)
		}
	}

	// Handle RetryInfo details
	if retryInfo, ok := details["retry_info"].(map[string]interface{}); ok {
		ri := s.buildRetryInfo(retryInfo)
		if ri != nil {
			result = append(result, ri)
		}
	}

	// Handle DebugInfo details
	if debugInfo, ok := details["debug_info"].(map[string]interface{}); ok {
		di := s.buildDebugInfo(debugInfo)
		if di != nil {
			result = append(result, di)
		}
	}

	// Handle QuotaFailure details
	if quotaFailure, ok := details["quota_failure"].(map[string]interface{}); ok {
		qf := s.buildQuotaFailure(quotaFailure)
		if qf != nil {
			result = append(result, qf)
		}
	}

	// Handle PreconditionFailure details
	if preconditionFailure, ok := details["precondition_failure"].(map[string]interface{}); ok {
		pf := s.buildPreconditionFailure(preconditionFailure)
		if pf != nil {
			result = append(result, pf)
		}
	}

	// Handle ResourceInfo details
	if resourceInfo, ok := details["resource_info"].(map[string]interface{}); ok {
		ri := s.buildResourceInfo(resourceInfo)
		if ri != nil {
			result = append(result, ri)
		}
	}

	// Handle Help details
	if help, ok := details["help"].(map[string]interface{}); ok {
		h := s.buildHelp(help)
		if h != nil {
			result = append(result, h)
		}
	}

	// Handle LocalizedMessage details
	if localizedMsg, ok := details["localized_message"].(map[string]interface{}); ok {
		lm := s.buildLocalizedMessage(localizedMsg)
		if lm != nil {
			result = append(result, lm)
		}
	}

	return result
}

// buildBadRequest creates a BadRequest error detail from a map.
func (s *Server) buildBadRequest(data map[string]interface{}) *errdetails.BadRequest {
	br := &errdetails.BadRequest{}

	if violations, ok := data["field_violations"].([]interface{}); ok {
		for _, v := range violations {
			if vMap, ok := v.(map[string]interface{}); ok {
				fv := &errdetails.BadRequest_FieldViolation{}
				if field, ok := vMap["field"].(string); ok {
					fv.Field = field
				}
				if desc, ok := vMap["description"].(string); ok {
					fv.Description = desc
				}
				br.FieldViolations = append(br.FieldViolations, fv)
			}
		}
	}

	if len(br.FieldViolations) == 0 {
		return nil
	}
	return br
}

// buildErrorInfo creates an ErrorInfo error detail from a map.
func (s *Server) buildErrorInfo(data map[string]interface{}) *errdetails.ErrorInfo {
	ei := &errdetails.ErrorInfo{}

	if reason, ok := data["reason"].(string); ok {
		ei.Reason = reason
	}
	if domain, ok := data["domain"].(string); ok {
		ei.Domain = domain
	}
	if metadata, ok := data["metadata"].(map[string]interface{}); ok {
		ei.Metadata = make(map[string]string)
		for k, v := range metadata {
			if str, ok := v.(string); ok {
				ei.Metadata[k] = str
			}
		}
	}

	if ei.Reason == "" && ei.Domain == "" {
		return nil
	}
	return ei
}

// buildRetryInfo creates a RetryInfo error detail from a map.
func (s *Server) buildRetryInfo(data map[string]interface{}) *errdetails.RetryInfo {
	ri := &errdetails.RetryInfo{}

	if delayStr, ok := data["retry_delay"].(string); ok {
		if d, err := time.ParseDuration(delayStr); err == nil {
			ri.RetryDelay = durationpb.New(d)
		}
	}

	if ri.RetryDelay == nil {
		return nil
	}
	return ri
}

// buildDebugInfo creates a DebugInfo error detail from a map.
func (s *Server) buildDebugInfo(data map[string]interface{}) *errdetails.DebugInfo {
	di := &errdetails.DebugInfo{}

	if stackEntries, ok := data["stack_entries"].([]interface{}); ok {
		for _, entry := range stackEntries {
			if str, ok := entry.(string); ok {
				di.StackEntries = append(di.StackEntries, str)
			}
		}
	}
	if detail, ok := data["detail"].(string); ok {
		di.Detail = detail
	}

	if len(di.StackEntries) == 0 && di.Detail == "" {
		return nil
	}
	return di
}

// buildQuotaFailure creates a QuotaFailure error detail from a map.
func (s *Server) buildQuotaFailure(data map[string]interface{}) *errdetails.QuotaFailure {
	qf := &errdetails.QuotaFailure{}

	if violations, ok := data["violations"].([]interface{}); ok {
		for _, v := range violations {
			if vMap, ok := v.(map[string]interface{}); ok {
				qv := &errdetails.QuotaFailure_Violation{}
				if subject, ok := vMap["subject"].(string); ok {
					qv.Subject = subject
				}
				if desc, ok := vMap["description"].(string); ok {
					qv.Description = desc
				}
				qf.Violations = append(qf.Violations, qv)
			}
		}
	}

	if len(qf.Violations) == 0 {
		return nil
	}
	return qf
}

// buildPreconditionFailure creates a PreconditionFailure error detail from a map.
func (s *Server) buildPreconditionFailure(data map[string]interface{}) *errdetails.PreconditionFailure {
	pf := &errdetails.PreconditionFailure{}

	if violations, ok := data["violations"].([]interface{}); ok {
		for _, v := range violations {
			if vMap, ok := v.(map[string]interface{}); ok {
				pv := &errdetails.PreconditionFailure_Violation{}
				if typ, ok := vMap["type"].(string); ok {
					pv.Type = typ
				}
				if subject, ok := vMap["subject"].(string); ok {
					pv.Subject = subject
				}
				if desc, ok := vMap["description"].(string); ok {
					pv.Description = desc
				}
				pf.Violations = append(pf.Violations, pv)
			}
		}
	}

	if len(pf.Violations) == 0 {
		return nil
	}
	return pf
}

// buildResourceInfo creates a ResourceInfo error detail from a map.
func (s *Server) buildResourceInfo(data map[string]interface{}) *errdetails.ResourceInfo {
	ri := &errdetails.ResourceInfo{}

	if resourceType, ok := data["resource_type"].(string); ok {
		ri.ResourceType = resourceType
	}
	if resourceName, ok := data["resource_name"].(string); ok {
		ri.ResourceName = resourceName
	}
	if owner, ok := data["owner"].(string); ok {
		ri.Owner = owner
	}
	if desc, ok := data["description"].(string); ok {
		ri.Description = desc
	}

	if ri.ResourceType == "" && ri.ResourceName == "" {
		return nil
	}
	return ri
}

// buildHelp creates a Help error detail from a map.
func (s *Server) buildHelp(data map[string]interface{}) *errdetails.Help {
	h := &errdetails.Help{}

	if links, ok := data["links"].([]interface{}); ok {
		for _, l := range links {
			if lMap, ok := l.(map[string]interface{}); ok {
				link := &errdetails.Help_Link{}
				if desc, ok := lMap["description"].(string); ok {
					link.Description = desc
				}
				if url, ok := lMap["url"].(string); ok {
					link.Url = url
				}
				h.Links = append(h.Links, link)
			}
		}
	}

	if len(h.Links) == 0 {
		return nil
	}
	return h
}

// buildLocalizedMessage creates a LocalizedMessage error detail from a map.
func (s *Server) buildLocalizedMessage(data map[string]interface{}) *errdetails.LocalizedMessage {
	lm := &errdetails.LocalizedMessage{}

	if locale, ok := data["locale"].(string); ok {
		lm.Locale = locale
	}
	if message, ok := data["message"].(string); ok {
		lm.Message = message
	}

	if lm.Locale == "" && lm.Message == "" {
		return nil
	}
	return lm
}

// dynamicMessageToMap converts a dynamic protobuf message to a map.
func dynamicMessageToMap(msg proto.Message) map[string]interface{} {
	if msg == nil {
		return nil
	}

	// Use protojson to convert to JSON, then unmarshal to map
	jsonBytes, err := protojson.Marshal(msg)
	if err != nil {
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil
	}

	return result
}

// GetServiceDescriptor returns the service descriptor for a service name.
func (s *Server) GetServiceDescriptor(serviceName string) protoreflect.ServiceDescriptor {
	svc := s.schema.GetService(serviceName)
	if svc == nil {
		return nil
	}
	return svc.GetDescriptor()
}

// GetMethodDescriptor returns the method descriptor for a service/method name.
func (s *Server) GetMethodDescriptor(serviceName, methodName string) protoreflect.MethodDescriptor {
	svc := s.schema.GetService(serviceName)
	if svc == nil {
		return nil
	}
	method := svc.GetMethod(methodName)
	if method == nil {
		return nil
	}
	return method.GetDescriptor()
}

// grpcCodeToString converts a gRPC status code to its string name.
func grpcCodeToString(code codes.Code) string {
	for name, c := range GRPCStatusCode {
		if codes.Code(c) == code {
			return name
		}
	}
	return "UNKNOWN"
}

// Config returns the server configuration.
func (s *Server) Config() *GRPCConfig {
	return s.config
}

// Schema returns the proto schema.
func (s *Server) Schema() *ProtoSchema {
	return s.schema
}

// ID returns the server ID from config.
func (s *Server) ID() string {
	if s.config == nil {
		return ""
	}
	return s.config.ID
}

// Protocol returns the protocol type.
func (s *Server) Protocol() protocol.Protocol {
	return protocol.ProtocolGRPC
}

// Metadata returns descriptive information about the handler.
func (s *Server) Metadata() protocol.Metadata {
	return protocol.Metadata{
		ID:                   s.ID(),
		Name:                 s.config.Name,
		Protocol:             protocol.ProtocolGRPC,
		Version:              "0.2.0",
		TransportType:        protocol.TransportHTTP2,
		ConnectionModel:      protocol.ConnectionModelStandalone,
		CommunicationPattern: protocol.PatternRequestResponse,
		Capabilities: []protocol.Capability{
			protocol.CapabilityStreaming,
			protocol.CapabilityBidirectional,
			protocol.CapabilitySchemaValidation,
			protocol.CapabilitySchemaIntrospect,
			protocol.CapabilityMetrics,
		},
	}
}

// Health returns the current health status.
func (s *Server) Health(ctx context.Context) protocol.HealthStatus {
	s.mu.RLock()
	running := s.running
	s.mu.RUnlock()

	status := protocol.HealthUnhealthy
	if running {
		status = protocol.HealthHealthy
	}
	return protocol.HealthStatus{
		Status:    status,
		CheckedAt: time.Now(),
	}
}

// Stats returns operational metrics for the handler.
func (s *Server) Stats() protocol.Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var uptime time.Duration
	if s.running && !s.startedAt.IsZero() {
		uptime = time.Since(s.startedAt)
	}

	return protocol.Stats{
		Running:   s.running,
		StartedAt: s.startedAt,
		Uptime:    uptime,
	}
}

// Port returns the port the server is listening on.
func (s *Server) Port() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listener == nil {
		return 0
	}
	if addr, ok := s.listener.Addr().(*net.TCPAddr); ok {
		return addr.Port
	}
	return 0
}

// ListServices returns available gRPC services.
func (s *Server) ListServices() []protocol.ServiceInfo {
	services := make([]protocol.ServiceInfo, 0)
	for _, serviceName := range s.schema.ListServices() {
		svc := s.schema.GetService(serviceName)
		if svc == nil {
			continue
		}
		methods := svc.ListMethods()
		services = append(services, protocol.ServiceInfo{
			Name:     serviceName,
			FullName: serviceName,
			Methods:  methods,
		})
	}
	return services
}

// ListMethods returns methods for a specific service.
func (s *Server) ListMethods(serviceName string) []protocol.MethodInfo {
	svc := s.schema.GetService(serviceName)
	if svc == nil {
		return nil
	}

	methods := make([]protocol.MethodInfo, 0)
	for _, methodName := range svc.ListMethods() {
		method := svc.GetMethod(methodName)
		if method == nil {
			continue
		}

		methodInfo := protocol.MethodInfo{
			Name:            methodName,
			FullName:        fmt.Sprintf("/%s/%s", serviceName, methodName),
			ClientStreaming: method.ClientStreaming,
			ServerStreaming: method.ServerStreaming,
		}

		// Get type names from descriptors
		if inputDesc := method.GetInputDescriptor(); inputDesc != nil {
			methodInfo.RequestType = string(inputDesc.FullName())
		}
		if outputDesc := method.GetOutputDescriptor(); outputDesc != nil {
			methodInfo.ResponseType = string(outputDesc.FullName())
		}

		methods = append(methods, methodInfo)
	}
	return methods
}

// SetRequestLogger sets the request logger for unified request logging.
func (s *Server) SetRequestLogger(logger requestlog.Logger) {
	s.requestLoggerMu.Lock()
	defer s.requestLoggerMu.Unlock()
	s.requestLogger = logger
}

// GetRequestLogger returns the current request logger.
func (s *Server) GetRequestLogger() requestlog.Logger {
	s.requestLoggerMu.RLock()
	defer s.requestLoggerMu.RUnlock()
	return s.requestLogger
}

// toJSONString converts an interface to a JSON string, handling errors gracefully.
func toJSONString(v interface{}) string {
	if v == nil {
		return ""
	}
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("<marshal error: %v>", err)
	}
	return string(data)
}

// streamTypeToString converts streamType to a string for logging.
func streamTypeToString(st streamType) string {
	return string(st)
}

// generateLogID generates a unique ID for request log entries.
func generateLogID() string {
	return fmt.Sprintf("grpc-%d", time.Now().UnixNano())
}

// logGRPCCall logs a gRPC call with all available information and records metrics.
func (s *Server) logGRPCCall(startTime time.Time, fullPath, serviceName, methodName string, st streamType, md metadata.MD, req interface{}, resp interface{}, grpcErr error) {
	duration := time.Since(startTime)

	// Record metrics
	s.recordGRPCMetrics(fullPath, grpcErr, duration)

	// Check if logging is enabled
	s.requestLoggerMu.RLock()
	logger := s.requestLogger
	s.requestLoggerMu.RUnlock()

	if logger == nil {
		return
	}

	// Build request body JSON
	reqBody := toJSONString(req)
	reqBodySize := len(reqBody)
	reqBody = util.TruncateBody(reqBody, 0)

	// Build response body JSON
	respBody := toJSONString(resp)
	respBody = util.TruncateBody(respBody, 0)

	// Extract status code and message from error
	var statusCode string
	var statusMessage string
	var responseStatus int
	var errorMsg string

	if grpcErr != nil {
		st, ok := status.FromError(grpcErr)
		if ok {
			statusCode = grpcCodeToString(st.Code())
			statusMessage = st.Message()
			responseStatus = int(st.Code())
		} else {
			statusCode = "UNKNOWN"
			statusMessage = grpcErr.Error()
			responseStatus = int(codes.Unknown)
		}
		errorMsg = grpcErr.Error()
	} else {
		statusCode = "OK"
		responseStatus = int(codes.OK)
	}

	// Convert metadata to headers map
	var headers map[string][]string
	if md != nil {
		headers = make(map[string][]string)
		for k, v := range md {
			headers[k] = v
		}
	}

	entry := &requestlog.Entry{
		ID:             generateLogID(),
		Timestamp:      startTime,
		Protocol:       requestlog.ProtocolGRPC,
		Method:         methodName,
		Path:           fullPath,
		Headers:        headers,
		Body:           reqBody,
		BodySize:       reqBodySize,
		ResponseStatus: responseStatus,
		ResponseBody:   respBody,
		DurationMs:     int(time.Since(startTime).Milliseconds()),
		Error:          errorMsg,
		GRPC: &requestlog.GRPCMeta{
			Service:       serviceName,
			MethodName:    methodName,
			StreamType:    streamTypeToString(st),
			StatusCode:    statusCode,
			StatusMessage: statusMessage,
		},
	}

	logger.Log(entry)
}

// recordGRPCMetrics records gRPC request metrics.
func (s *Server) recordGRPCMetrics(fullPath string, grpcErr error, duration time.Duration) {
	statusCode := "ok"
	if grpcErr != nil {
		if st, ok := status.FromError(grpcErr); ok {
			statusCode = strings.ToLower(st.Code().String())
		} else {
			statusCode = "unknown"
		}
	}

	if metrics.RequestsTotal != nil {
		if vec, err := metrics.RequestsTotal.WithLabels("grpc", fullPath, statusCode); err == nil {
			vec.Inc()
		}
	}
	if metrics.RequestDuration != nil {
		if vec, err := metrics.RequestDuration.WithLabels("grpc", fullPath); err == nil {
			vec.Observe(duration.Seconds())
		}
	}
}

// Interface compliance checks.
var (
	_ protocol.Handler          = (*Server)(nil)
	_ protocol.StandaloneServer = (*Server)(nil)
	_ protocol.RPCHandler       = (*Server)(nil)
	_ protocol.RequestLoggable  = (*Server)(nil)
	_ protocol.Observable       = (*Server)(nil)
	_ protocol.Loggable         = (*Server)(nil)
)
