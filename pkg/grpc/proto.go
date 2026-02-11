package grpc

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/linker"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ProtoSchema represents parsed .proto file(s) and provides access to
// service and method descriptors.
type ProtoSchema struct {
	files    []protoreflect.FileDescriptor
	services map[string]*ServiceDescriptor
}

// ServiceDescriptor describes a gRPC service and its methods.
type ServiceDescriptor struct {
	// Name is the fully qualified service name (e.g., "package.ServiceName").
	Name string

	// Methods maps method names to their descriptors.
	Methods map[string]*MethodDescriptor

	// desc is the underlying protoreflect descriptor.
	desc protoreflect.ServiceDescriptor
}

// MethodDescriptor describes a gRPC method including its streaming characteristics.
type MethodDescriptor struct {
	// Name is the method name.
	Name string

	// FullName is the fully qualified method name (e.g., "package.Service/Method").
	FullName string

	// InputType is the fully qualified name of the request message type.
	InputType string

	// OutputType is the fully qualified name of the response message type.
	OutputType string

	// ClientStreaming indicates if the client streams requests.
	ClientStreaming bool

	// ServerStreaming indicates if the server streams responses.
	ServerStreaming bool

	// desc is the underlying protoreflect descriptor.
	desc protoreflect.MethodDescriptor
}

// ParseProtoFile parses a single .proto file and returns a ProtoSchema.
// importPaths specifies directories to search for imported files.
func ParseProtoFile(path string, importPaths []string) (*ProtoSchema, error) {
	return ParseProtoFiles([]string{path}, importPaths)
}

// ParseProtoFiles parses multiple .proto files and returns a unified ProtoSchema.
// importPaths specifies directories to search for imported files.
func ParseProtoFiles(paths []string, importPaths []string) (*ProtoSchema, error) {
	if len(paths) == 0 {
		return nil, ErrNoProtoFiles
	}

	// Create a resolver that searches import paths
	resolver := &protocompile.SourceResolver{
		ImportPaths: importPaths,
		Accessor:    protocompile.SourceAccessorFromMap(nil), // Will fall through to file system
	}

	// Wrap with a file system accessor for import paths
	fsResolver := &fileSystemResolver{
		importPaths: importPaths,
		basePaths:   paths,
	}

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(
			protocompile.CompositeResolver{
				resolver,
				fsResolver,
			},
		),
	}

	// Compile all proto files
	compiled, err := compiler.Compile(context.Background(), paths...)
	if err != nil {
		return nil, err
	}

	schema := &ProtoSchema{
		files:    make([]protoreflect.FileDescriptor, 0, len(compiled)),
		services: make(map[string]*ServiceDescriptor),
	}

	// Convert linker.Files to []protoreflect.FileDescriptor and extract services
	for _, file := range compiled {
		schema.files = append(schema.files, file)

		services := file.Services()
		for i := 0; i < services.Len(); i++ {
			svc := services.Get(i)
			svcDesc := &ServiceDescriptor{
				Name:    string(svc.FullName()),
				Methods: make(map[string]*MethodDescriptor),
				desc:    svc,
			}

			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				method := methods.Get(j)
				methodDesc := &MethodDescriptor{
					Name:            string(method.Name()),
					FullName:        string(method.FullName()),
					InputType:       string(method.Input().FullName()),
					OutputType:      string(method.Output().FullName()),
					ClientStreaming: method.IsStreamingClient(),
					ServerStreaming: method.IsStreamingServer(),
					desc:            method,
				}
				svcDesc.Methods[string(method.Name())] = methodDesc
			}

			schema.services[string(svc.FullName())] = svcDesc
		}
	}

	return schema, nil
}

// fileSystemResolver implements protocompile.Resolver for file system access.
type fileSystemResolver struct {
	importPaths []string
	basePaths   []string
}

func (r *fileSystemResolver) FindFileByPath(path string) (protocompile.SearchResult, error) {
	// First check the import paths
	for _, importPath := range r.importPaths {
		fullPath := filepath.Join(importPath, path)
		if _, err := os.Stat(fullPath); err == nil {
			rc, err := readFile(fullPath)
			if err != nil {
				return protocompile.SearchResult{}, err
			}
			return protocompile.SearchResult{Source: rc}, nil
		}
	}

	// Check relative to base paths' directories
	for _, basePath := range r.basePaths {
		dir := filepath.Dir(basePath)
		fullPath := filepath.Join(dir, path)
		if _, err := os.Stat(fullPath); err == nil {
			rc, err := readFile(fullPath)
			if err != nil {
				return protocompile.SearchResult{}, err
			}
			return protocompile.SearchResult{Source: rc}, nil
		}
	}

	// Try the path directly
	if _, err := os.Stat(path); err == nil {
		rc, err := readFile(path)
		if err != nil {
			return protocompile.SearchResult{}, err
		}
		return protocompile.SearchResult{Source: rc}, nil
	}

	return protocompile.SearchResult{}, fs.ErrNotExist
}

func readFile(path string) (io.ReadCloser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return f, nil
}

// GetService returns a service descriptor by its fully qualified name.
// Returns nil if the service is not found.
func (p *ProtoSchema) GetService(name string) *ServiceDescriptor {
	return p.services[name]
}

// ListServices returns all service names in sorted order.
func (p *ProtoSchema) ListServices() []string {
	names := make([]string, 0, len(p.services))
	for name := range p.services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetMethod returns a method descriptor by name.
// Returns nil if the method is not found.
func (s *ServiceDescriptor) GetMethod(name string) *MethodDescriptor {
	return s.Methods[name]
}

// ListMethods returns all method names in sorted order.
func (s *ServiceDescriptor) ListMethods() []string {
	names := make([]string, 0, len(s.Methods))
	for name := range s.Methods {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetDescriptor returns the underlying protoreflect service descriptor.
func (s *ServiceDescriptor) GetDescriptor() protoreflect.ServiceDescriptor {
	return s.desc
}

// IsUnary returns true if the method is unary (no streaming in either direction).
func (m *MethodDescriptor) IsUnary() bool {
	return !m.ClientStreaming && !m.ServerStreaming
}

// IsServerStreaming returns true if only the server streams responses.
func (m *MethodDescriptor) IsServerStreaming() bool {
	return !m.ClientStreaming && m.ServerStreaming
}

// IsClientStreaming returns true if only the client streams requests.
func (m *MethodDescriptor) IsClientStreaming() bool {
	return m.ClientStreaming && !m.ServerStreaming
}

// IsBidirectional returns true if both client and server stream.
func (m *MethodDescriptor) IsBidirectional() bool {
	return m.ClientStreaming && m.ServerStreaming
}

// GetStreamingType returns a string describing the streaming type.
func (m *MethodDescriptor) GetStreamingType() string {
	switch {
	case m.IsBidirectional():
		return "bidirectional"
	case m.IsClientStreaming():
		return "client_streaming"
	case m.IsServerStreaming():
		return "server_streaming"
	default:
		return "unary"
	}
}

// GetDescriptor returns the underlying protoreflect method descriptor.
func (m *MethodDescriptor) GetDescriptor() protoreflect.MethodDescriptor {
	return m.desc
}

// GetInputDescriptor returns the message descriptor for the input type.
func (m *MethodDescriptor) GetInputDescriptor() protoreflect.MessageDescriptor {
	if m.desc == nil {
		return nil
	}
	return m.desc.Input()
}

// GetOutputDescriptor returns the message descriptor for the output type.
func (m *MethodDescriptor) GetOutputDescriptor() protoreflect.MessageDescriptor {
	if m.desc == nil {
		return nil
	}
	return m.desc.Output()
}

// Files returns the parsed file descriptors.
func (p *ProtoSchema) Files() []protoreflect.FileDescriptor {
	return p.files
}

// ServiceCount returns the number of services in the schema.
func (p *ProtoSchema) ServiceCount() int {
	return len(p.services)
}

// MethodCount returns the total number of methods across all services.
func (p *ProtoSchema) MethodCount() int {
	count := 0
	for _, svc := range p.services {
		count += len(svc.Methods)
	}
	return count
}

// Ensure linker.File satisfies protoreflect.FileDescriptor at compile time.
var _ protoreflect.FileDescriptor = (linker.File)(nil)
