package performance

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/getmockd/mockd/pkg/grpc"
	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
	"github.com/jhump/protoreflect/dynamic"
	"github.com/jhump/protoreflect/dynamic/grpcdynamic"
	grpclib "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const benchProtoFile = "../../tests/fixtures/grpc/test.proto"

func setupBenchGRPCServer(b *testing.B) (*grpc.Server, int, *grpc.ProtoSchema) {
	// Find free port
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		b.Fatalf("failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	// Parse proto
	schema, err := grpc.ParseProtoFile(benchProtoFile, nil)
	if err != nil {
		b.Fatalf("failed to parse proto: %v", err)
	}

	cfg := &grpc.GRPCConfig{
		ID:        "bench-server",
		Port:      port,
		ProtoFile: benchProtoFile,
		Enabled:   true,
		Reflection: true,
		Services: map[string]grpc.ServiceConfig{
			"test.UserService": {
				Methods: map[string]grpc.MethodConfig{
					"GetUser": {
						Response: map[string]interface{}{
							"id":    "bench-user-123",
							"name":  "Benchmark User",
							"email": "bench@example.com",
						},
					},
				},
			},
		},
	}

	server, err := grpc.NewServer(cfg, schema)
	if err != nil {
		b.Fatalf("failed to create server: %v", err)
	}

	if err := server.Start(context.Background()); err != nil {
		b.Fatalf("failed to start server: %v", err)
	}

	b.Cleanup(func() {
		server.Stop(context.Background(), time.Second)
	})

	time.Sleep(50 * time.Millisecond)
	return server, port, schema
}

func createBenchGRPCConnection(b *testing.B, port int) *grpclib.ClientConn {
	addr := fmt.Sprintf("localhost:%d", port)
	conn, err := grpclib.NewClient(addr,
		grpclib.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		b.Fatalf("failed to create connection: %v", err)
	}
	return conn
}

// getJhumpDescriptorsB parses proto using jhump for client-side dynamic invocation.
func getJhumpDescriptorsB(b *testing.B) []*desc.FileDescriptor {
	b.Helper()
	parser := protoparse.Parser{}
	files, err := parser.ParseFiles(benchProtoFile)
	if err != nil {
		b.Fatalf("failed to parse proto: %v", err)
	}
	return files
}

// getJhumpMethodDescB returns jhump method descriptor for use with grpcdynamic.Stub
func getJhumpMethodDescB(b *testing.B, files []*desc.FileDescriptor, serviceName, methodName string) *desc.MethodDescriptor {
	b.Helper()
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
	b.Fatalf("method %s/%s not found", serviceName, methodName)
	return nil
}

// BenchmarkGRPC_UnaryLatency measures single unary call latency.
func BenchmarkGRPC_UnaryLatency(b *testing.B) {
	_, port, _ := setupBenchGRPCServer(b)
	files := getJhumpDescriptorsB(b)

	conn := createBenchGRPCConnection(b, port)
	defer conn.Close()

	stub := grpcdynamic.NewStub(conn)

	// Get method descriptor using jhump for grpcdynamic
	md := getJhumpMethodDescB(b, files, "test.UserService", "GetUser")

	// Create request message
	reqMsg := dynamic.NewMessage(md.GetInputType())
	reqMsg.SetFieldByName("id", "bench-123")

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := stub.InvokeRpc(ctx, md, reqMsg)
		if err != nil {
			b.Fatalf("RPC failed: %v", err)
		}
	}
}

// BenchmarkGRPC_ConnectionEstablishment measures connection setup time.
func BenchmarkGRPC_ConnectionEstablishment(b *testing.B) {
	_, port, _ := setupBenchGRPCServer(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn := createBenchGRPCConnection(b, port)
		conn.Close()
	}
}

// BenchmarkGRPC_ConcurrentUnary measures concurrent unary calls.
func BenchmarkGRPC_ConcurrentUnary(b *testing.B) {
	_, port, _ := setupBenchGRPCServer(b)
	files := getJhumpDescriptorsB(b)

	md := getJhumpMethodDescB(b, files, "test.UserService", "GetUser")

	numClients := 10
	callsPerClient := b.N / numClients
	if callsPerClient < 1 {
		callsPerClient = 1
	}

	b.ResetTimer()

	var wg sync.WaitGroup
	for c := 0; c < numClients; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			conn := createBenchGRPCConnection(b, port)
			defer conn.Close()

			stub := grpcdynamic.NewStub(conn)
			reqMsg := dynamic.NewMessage(md.GetInputType())
			reqMsg.SetFieldByName("id", "concurrent-bench")

			ctx := context.Background()
			for i := 0; i < callsPerClient; i++ {
				stub.InvokeRpc(ctx, md, reqMsg)
			}
		}()
	}
	wg.Wait()
}

// BenchmarkGRPC_Throughput measures maximum calls per second with different message sizes.
func BenchmarkGRPC_Throughput(b *testing.B) {
	_, port, _ := setupBenchGRPCServer(b)
	files := getJhumpDescriptorsB(b)

	conn := createBenchGRPCConnection(b, port)
	defer conn.Close()

	stub := grpcdynamic.NewStub(conn)
	md := getJhumpMethodDescB(b, files, "test.UserService", "GetUser")

	ctx := context.Background()

	b.Run("unary", func(b *testing.B) {
		reqMsg := dynamic.NewMessage(md.GetInputType())
		reqMsg.SetFieldByName("id", "throughput-test")

		for i := 0; i < b.N; i++ {
			stub.InvokeRpc(ctx, md, reqMsg)
		}
	})
}
