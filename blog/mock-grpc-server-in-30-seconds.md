---
title: How to Mock a gRPC Server in 30 Seconds
published: false
description: Most gRPC mocking tutorials start with "first, install the protoc compiler." This one starts with a working server.
tags: grpc, testing, go, devtools
canonical_url: https://mockd.io/blog/mock-grpc-server-in-30-seconds
---

I spent a solid afternoon last year trying to set up a mock gRPC server for integration tests. The options were: write a full Go server that implements the interface, spin up WireMock with the gRPC extension and figure out the Java classpath situation, or... just not test that code path.

I picked option three. Not proud of it.

When I built [mockd](https://github.com/getmockd/mockd), gRPC was the protocol I wanted to get right. Here's what "get right" looks like:

## The 30-second version

Install mockd:

```bash
# macOS
brew install getmockd/tap/mockd

# Linux / Windows
curl -fsSL https://get.mockd.io | sh
```

Create a proto file. You probably already have one — that's the whole point. I'll use a simple greeter:

```protobuf
// greeter.proto
syntax = "proto3";

package helloworld;

service Greeter {
  rpc SayHello (HelloRequest) returns (HelloReply) {}
}

message HelloRequest {
  string name = 1;
}

message HelloReply {
  string message = 1;
}
```

Start the mock server:

```bash
mockd add grpc \
  --proto greeter.proto \
  --service helloworld.Greeter \
  --rpc-method SayHello \
  --response '{"message": "Hello, World!"}'
```

That's it. You now have a gRPC server on port 50051 that responds to `SayHello`. Test it:

```bash
grpcurl -plaintext -d '{"name": "Alice"}' \
  localhost:50051 helloworld.Greeter/SayHello
```

```json
{
  "message": "Hello, World!"
}
```

30 seconds. No Java. No code generation. No writing a server.

## Why this is hard with other tools

The standard approach to mocking gRPC in Go looks something like this:

```go
type mockGreeterServer struct {
    pb.UnimplementedGreeterServer
}

func (s *mockGreeterServer) SayHello(ctx context.Context, req *pb.HelloRequest) (*pb.HelloReply, error) {
    return &pb.HelloReply{Message: "Hello, " + req.Name}, nil
}

func TestSomething(t *testing.T) {
    lis, _ := net.Listen("tcp", ":0")
    s := grpc.NewServer()
    pb.RegisterGreeterServer(s, &mockGreeterServer{})
    go s.Serve(lis)
    defer s.Stop()
    // ... now write your test
}
```

That's fine for one service. It's less fine when you have twelve services and you need different responses for different test scenarios. You end up with a `mock_servers_test.go` file that's 400 lines of boilerplate.

WireMock has a gRPC extension, but it requires the Java runtime, the WireMock standalone JAR, and the gRPC extension JAR. The setup instructions are a page long.

## The YAML version

For more complex setups, a config file is cleaner than CLI flags:

```yaml
# mockd.yaml
version: "1.0"

mocks:
  - name: Greeter Service
    type: grpc
    grpc:
      port: 50051
      protoFile: ./greeter.proto
      reflection: true
      services:
        helloworld.Greeter:
          methods:
            SayHello:
              response:
                message: "Hello, {{request.body.name}}!"
```

```bash
mockd serve --config mockd.yaml
```

A few things worth noting:

**`reflection: true`** enables [gRPC server reflection](https://github.com/grpc/grpc/blob/master/doc/server-reflection.md). This means you can use `grpcurl` without passing the proto file on the client side — the mock server tells the client what services and methods are available. Useful for debugging:

```bash
grpcurl -plaintext localhost:50051 list
# grpc.health.v1.Health
# grpc.reflection.v1.ServerReflection
# grpc.reflection.v1alpha.ServerReflection
# helloworld.Greeter
```

**`{{request.body.name}}`** pulls the `name` field from the incoming request and uses it in the response. So `grpcurl -d '{"name": "Alice"}' localhost:50051 helloworld.Greeter/SayHello` returns `{"message": "Hello, Alice!"}`.

## Server streaming

mockd handles all four gRPC patterns: unary, server streaming, client streaming, and bidirectional. Here's server streaming — you just supply a list of responses instead of a single one:

```yaml
services:
  helloworld.Greeter:
    methods:
      SayHelloStream:
        responses:
          - message: "Hello! (1/3)"
          - message: "Hello again! (2/3)"
          - message: "One more time! (3/3)"
        streamDelay: "500ms"
```

Each response is sent as a separate stream message with a 500ms delay between them. `grpcurl` shows them arriving one at a time:

```bash
grpcurl -plaintext -d '{"name": "World"}' \
  localhost:50051 helloworld.Greeter/SayHelloStream
# {  "message": "Hello! (1/3)" }
# {  "message": "Hello again! (2/3)" }
# {  "message": "One more time! (3/3)" }
```

## Dynamic response data

mockd has template functions you can use inside response fields. The `{{request.body.name}}` template we saw earlier is one example. Others:

```yaml
SayHello:
  response:
    message: "Hello {{request.body.name}}, your request ID is {{uuid}}"
```

`{{uuid}}` generates a unique ID on each call. `{{now}}` gives you an ISO-8601 timestamp. There are about 34 faker functions available — `{{faker.email}}`, `{{faker.name}}`, `{{faker.ipv4}}`, etc.

One thing to know: your response fields must match the proto message definition. If `HelloReply` only has a `message` field, you can't add `timestamp` or `requestId` to the response — the proto schema defines the shape. Templates go *inside* the field values, not as new fields.

## Using it in tests

The pattern I've settled on: start mockd in the background, run tests, stop it.

```bash
# Start in background
mockd serve --config mockd.yaml --detach

# Run tests (your gRPC client connects to localhost:50051)
go test ./...

# Stop
mockd stop
```

Or in Docker Compose:

```yaml
services:
  mockd:
    image: ghcr.io/getmockd/mockd:latest
    volumes:
      - ./mockd.yaml:/config/mockd.yaml
      - ./greeter.proto:/config/greeter.proto
    command: ["serve", "--config", "/config/mockd.yaml", "--no-auth"]
    ports:
      - "50051:50051"
      - "4280:4280"

  tests:
    build: .
    depends_on:
      mockd:
        condition: service_healthy
    environment:
      GRPC_SERVER: mockd:50051
```

## The part where I'm honest about limitations

mockd parses your proto files at startup. It does not generate Go code — it interprets the proto schema dynamically using protobuf descriptors. This means:

- **It works with any proto file.** You don't need `protoc` or `protoc-gen-go` installed.
- **It doesn't validate request payloads against the schema.** If you send `{"name": 42}` when the proto says `string name = 1`, mockd won't complain. It's a mock server, not a contract validator.
- **Proto imports work**, but you need to tell mockd where to find them with `--proto-path`.

If you need strict contract validation, you want a different tool. If you need a gRPC server that responds with predictable data so you can test your client code, mockd is probably the fastest way to get there.

If any of these limitations are blockers for you — or if you want something that doesn't exist yet — [open an issue](https://github.com/getmockd/mockd/issues). The roadmap is heavily shaped by what people actually need. And if mockd saved you some time, a [star on GitHub](https://github.com/getmockd/mockd) helps other developers find it.

## Links

- **GitHub:** [github.com/getmockd/mockd](https://github.com/getmockd/mockd) (Apache 2.0)
- **gRPC docs:** [docs.mockd.io/protocols/grpc](https://docs.mockd.io/protocols/grpc/)
- **Install:** `brew install getmockd/tap/mockd` or `curl -fsSL https://get.mockd.io | sh`
