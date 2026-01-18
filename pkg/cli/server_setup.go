// Package cli provides command-line interface commands for the mock server.
// This file contains shared server configuration utilities used by start.go and serve.go.
package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/getmockd/mockd/internal/cliconfig"
	"github.com/getmockd/mockd/pkg/admin"
	"github.com/getmockd/mockd/pkg/audit"
	"github.com/getmockd/mockd/pkg/chaos"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/engine"
	"github.com/getmockd/mockd/pkg/graphql"
	"github.com/getmockd/mockd/pkg/grpc"
	"github.com/getmockd/mockd/pkg/mqtt"
	"github.com/getmockd/mockd/pkg/oauth"
	"github.com/getmockd/mockd/pkg/validation"
)

// ServerFlags holds common server configuration flags used by start and serve commands.
type ServerFlags struct {
	// Port configuration
	Port      int
	AdminPort int
	HTTPSPort int

	// Config file
	ConfigFile string

	// Timeouts
	ReadTimeout  int
	WriteTimeout int

	// Limits
	MaxLogEntries int

	// TLS flags
	TLSCert  string
	TLSKey   string
	TLSAuto  bool
	AutoCert bool

	// mTLS flags
	MTLSEnabled    bool
	MTLSClientAuth string
	MTLSCA         string
	MTLSAllowedCNs string

	// Audit flags
	AuditEnabled bool
	AuditFile    string
	AuditLevel   string

	// GraphQL flags
	GraphQLSchema string
	GraphQLPath   string

	// gRPC flags
	GRPCPort       int
	GRPCProto      string
	GRPCReflection bool

	// OAuth flags
	OAuthEnabled bool
	OAuthIssuer  string
	OAuthPort    int

	// MQTT flags
	MQTTPort int
	MQTTAuth bool

	// Chaos flags
	ChaosEnabled   bool
	ChaosLatency   string
	ChaosErrorRate float64

	// Validation flags
	ValidateSpec string
	ValidateFail bool

	// Auth flags
	NoAuth bool

	// Storage flags
	DataDir string
}

// RegisterServerFlags adds common server flags to a FlagSet.
func RegisterServerFlags(fs *flag.FlagSet, f *ServerFlags) {
	// Port flags
	fs.IntVar(&f.Port, "port", cliconfig.DefaultPort, "HTTP server port")
	fs.IntVar(&f.Port, "p", cliconfig.DefaultPort, "HTTP server port (shorthand)")

	fs.IntVar(&f.AdminPort, "admin-port", cliconfig.DefaultAdminPort, "Admin API port")
	fs.IntVar(&f.AdminPort, "a", cliconfig.DefaultAdminPort, "Admin API port (shorthand)")

	fs.IntVar(&f.HTTPSPort, "https-port", cliconfig.DefaultHTTPSPort, "HTTPS server port (0 = disabled)")

	// Config file
	fs.StringVar(&f.ConfigFile, "config", "", "Path to mock configuration file")
	fs.StringVar(&f.ConfigFile, "c", "", "Path to mock configuration file (shorthand)")

	// Timeouts
	fs.IntVar(&f.ReadTimeout, "read-timeout", cliconfig.DefaultReadTimeout, "Read timeout in seconds")
	fs.IntVar(&f.WriteTimeout, "write-timeout", cliconfig.DefaultWriteTimeout, "Write timeout in seconds")

	// Limits
	fs.IntVar(&f.MaxLogEntries, "max-log-entries", cliconfig.DefaultMaxLogEntries, "Maximum request log entries")
	fs.BoolVar(&f.AutoCert, "auto-cert", cliconfig.DefaultAutoCert, "Auto-generate TLS certificate")

	// TLS flags
	fs.StringVar(&f.TLSCert, "tls-cert", "", "Path to TLS certificate file")
	fs.StringVar(&f.TLSKey, "tls-key", "", "Path to TLS private key file")
	fs.BoolVar(&f.TLSAuto, "tls-auto", false, "Auto-generate self-signed certificate")

	// mTLS flags
	fs.BoolVar(&f.MTLSEnabled, "mtls-enabled", false, "Enable mTLS client certificate validation")
	fs.StringVar(&f.MTLSClientAuth, "mtls-client-auth", "require-and-verify", "Client auth mode (none, request, require, verify-if-given, require-and-verify)")
	fs.StringVar(&f.MTLSCA, "mtls-ca", "", "Path to CA certificate for client validation")
	fs.StringVar(&f.MTLSAllowedCNs, "mtls-allowed-cns", "", "Comma-separated list of allowed Common Names")

	// Audit flags
	fs.BoolVar(&f.AuditEnabled, "audit-enabled", false, "Enable audit logging")
	fs.StringVar(&f.AuditFile, "audit-file", "", "Path to audit log file")
	fs.StringVar(&f.AuditLevel, "audit-level", "info", "Log level (debug, info, warn, error)")

	// GraphQL flags
	fs.StringVar(&f.GraphQLSchema, "graphql-schema", "", "Path to GraphQL schema file")
	fs.StringVar(&f.GraphQLPath, "graphql-path", "/graphql", "GraphQL endpoint path")

	// gRPC flags
	fs.IntVar(&f.GRPCPort, "grpc-port", 0, "gRPC server port (0 = disabled)")
	fs.StringVar(&f.GRPCProto, "grpc-proto", "", "Path to .proto file")
	fs.BoolVar(&f.GRPCReflection, "grpc-reflection", true, "Enable gRPC reflection")

	// OAuth flags
	fs.BoolVar(&f.OAuthEnabled, "oauth-enabled", false, "Enable OAuth provider")
	fs.StringVar(&f.OAuthIssuer, "oauth-issuer", "", "OAuth issuer URL")
	fs.IntVar(&f.OAuthPort, "oauth-port", 0, "OAuth server port")

	// MQTT flags
	fs.IntVar(&f.MQTTPort, "mqtt-port", 0, "MQTT broker port (0 = disabled)")
	fs.BoolVar(&f.MQTTAuth, "mqtt-auth", false, "Enable MQTT authentication")

	// Chaos flags
	fs.BoolVar(&f.ChaosEnabled, "chaos-enabled", false, "Enable chaos injection")
	fs.StringVar(&f.ChaosLatency, "chaos-latency", "", "Add random latency (e.g., \"10ms-100ms\")")
	fs.Float64Var(&f.ChaosErrorRate, "chaos-error-rate", 0, "Error rate (0.0-1.0)")

	// Validation flags
	fs.StringVar(&f.ValidateSpec, "validate-spec", "", "Path to OpenAPI spec for request validation")
	fs.BoolVar(&f.ValidateFail, "validate-fail", false, "Fail on validation error")

	// Auth flags
	fs.BoolVar(&f.NoAuth, "no-auth", false, "Disable API key authentication")

	// Storage flags
	fs.StringVar(&f.DataDir, "data-dir", "", "Data directory for persistent storage (default: ~/.local/share/mockd)")
}

// BuildServerConfig creates a ServerConfiguration from the flags.
func BuildServerConfig(f *ServerFlags) *config.ServerConfiguration {
	serverCfg := &config.ServerConfiguration{
		HTTPPort:      f.Port,
		HTTPSPort:     f.HTTPSPort,
		AdminPort:     f.AdminPort,
		ReadTimeout:   f.ReadTimeout,
		WriteTimeout:  f.WriteTimeout,
		MaxLogEntries: f.MaxLogEntries,
		LogRequests:   true,
	}

	// Configure TLS if any TLS flags are set or HTTPS port is configured
	if f.TLSCert != "" || f.TLSKey != "" || f.TLSAuto || f.HTTPSPort > 0 {
		serverCfg.TLS = BuildTLSConfig(f)
	}

	// Configure mTLS if enabled
	if f.MTLSEnabled {
		serverCfg.MTLS = BuildMTLSConfig(f)
	}

	// Configure audit if enabled
	if f.AuditEnabled {
		serverCfg.Audit = BuildAuditConfig(f)
	}

	// Configure GraphQL if schema specified
	if f.GraphQLSchema != "" {
		serverCfg.GraphQL = BuildGraphQLConfig(f)
	}

	// Configure gRPC if port specified
	if f.GRPCPort > 0 {
		serverCfg.GRPC = BuildGRPCConfig(f)
	}

	// Configure OAuth if enabled
	if f.OAuthEnabled {
		serverCfg.OAuth = BuildOAuthConfig(f)
	}

	// Configure validation if spec specified
	if f.ValidateSpec != "" {
		serverCfg.Validation = BuildValidationConfig(f)
	}

	return serverCfg
}

// BuildTLSConfig creates TLS configuration from flags.
func BuildTLSConfig(f *ServerFlags) *config.TLSConfig {
	return &config.TLSConfig{
		Enabled:          true,
		CertFile:         f.TLSCert,
		KeyFile:          f.TLSKey,
		AutoGenerateCert: f.TLSAuto || f.AutoCert,
	}
}

// BuildMTLSConfig creates mTLS configuration from flags.
func BuildMTLSConfig(f *ServerFlags) *config.MTLSConfig {
	var allowedCNs []string
	if f.MTLSAllowedCNs != "" {
		for _, cn := range strings.Split(f.MTLSAllowedCNs, ",") {
			cn = strings.TrimSpace(cn)
			if cn != "" {
				allowedCNs = append(allowedCNs, cn)
			}
		}
	}
	return &config.MTLSConfig{
		Enabled:    true,
		ClientAuth: f.MTLSClientAuth,
		CACertFile: f.MTLSCA,
		AllowedCNs: allowedCNs,
	}
}

// BuildAuditConfig creates audit configuration from flags.
func BuildAuditConfig(f *ServerFlags) *audit.AuditConfig {
	return &audit.AuditConfig{
		Enabled:    true,
		Level:      f.AuditLevel,
		OutputFile: f.AuditFile,
	}
}

// BuildGraphQLConfig creates GraphQL configuration from flags.
func BuildGraphQLConfig(f *ServerFlags) []*graphql.GraphQLConfig {
	return []*graphql.GraphQLConfig{{
		ID:            "cli-graphql",
		Path:          f.GraphQLPath,
		SchemaFile:    f.GraphQLSchema,
		Introspection: true,
		Enabled:       true,
	}}
}

// BuildGRPCConfig creates gRPC configuration from flags.
func BuildGRPCConfig(f *ServerFlags) []*grpc.GRPCConfig {
	return []*grpc.GRPCConfig{{
		ID:         "cli-grpc",
		Port:       f.GRPCPort,
		ProtoFile:  f.GRPCProto,
		Reflection: f.GRPCReflection,
		Enabled:    true,
	}}
}

// BuildOAuthConfig creates OAuth configuration from flags.
func BuildOAuthConfig(f *ServerFlags) []*oauth.OAuthConfig {
	issuer := f.OAuthIssuer
	if issuer == "" {
		issuer = fmt.Sprintf("http://localhost:%d", f.OAuthPort)
	}
	return []*oauth.OAuthConfig{{
		ID:      "cli-oauth",
		Issuer:  issuer,
		Enabled: true,
	}}
}

// BuildValidationConfig creates validation configuration from flags.
func BuildValidationConfig(f *ServerFlags) *validation.ValidationConfig {
	return &validation.ValidationConfig{
		Enabled:         true,
		SpecFile:        f.ValidateSpec,
		ValidateRequest: true,
		FailOnError:     f.ValidateFail,
	}
}

// BuildChaosConfig creates chaos configuration from flags and applies it to the server config.
// Returns the chaos config if enabled, nil otherwise.
func BuildChaosConfig(f *ServerFlags) *chaos.ChaosConfig {
	if !f.ChaosEnabled {
		return nil
	}

	chaosCfg := &chaos.ChaosConfig{
		Enabled: true,
	}
	if f.ChaosLatency != "" {
		min, max := ParseLatencyRange(f.ChaosLatency)
		chaosCfg.GlobalRules = &chaos.GlobalChaosRules{
			Latency: &chaos.LatencyFault{
				Min:         min,
				Max:         max,
				Probability: 1.0,
			},
		}
	}
	if f.ChaosErrorRate > 0 {
		if chaosCfg.GlobalRules == nil {
			chaosCfg.GlobalRules = &chaos.GlobalChaosRules{}
		}
		chaosCfg.GlobalRules.ErrorRate = &chaos.ErrorRateFault{
			Probability: f.ChaosErrorRate,
			DefaultCode: 500,
		}
	}
	return chaosCfg
}

// StartMQTTBroker creates and starts an MQTT broker if configured.
// Returns the broker (nil if not configured) and any error.
func StartMQTTBroker(f *ServerFlags) (*mqtt.Broker, error) {
	if f.MQTTPort <= 0 {
		return nil, nil
	}

	mqttCfg := &mqtt.MQTTConfig{
		ID:      "cli-mqtt",
		Port:    f.MQTTPort,
		Enabled: true,
	}
	if f.MQTTAuth {
		mqttCfg.Auth = &mqtt.MQTTAuthConfig{
			Enabled: true,
		}
	}

	broker, err := mqtt.NewBroker(mqttCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create MQTT broker: %w", err)
	}
	if err := broker.Start(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to start MQTT broker: %w", err)
	}
	fmt.Printf("MQTT broker running on port %d\n", f.MQTTPort)
	return broker, nil
}

// ValidateGRPCFlags checks if gRPC flags are valid.
func ValidateGRPCFlags(f *ServerFlags) error {
	if f.GRPCPort > 0 && f.GRPCProto == "" {
		return fmt.Errorf("--grpc-proto is required when --grpc-port is specified")
	}
	return nil
}

// ParseLatencyRange parses a latency range string like "10ms-100ms" into min and max values.
func ParseLatencyRange(s string) (min, max string) {
	parts := strings.Split(s, "-")
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	// If no range, use the same value for both
	return s, s
}

// WaitForShutdown blocks until interrupt, then gracefully stops servers.
func WaitForShutdown(server *engine.Server, adminAPI *admin.AdminAPI) {
	WaitForShutdownWithCallback(server, adminAPI, nil)
}

// ShutdownCallback is called during shutdown for additional cleanup.
type ShutdownCallback func()

// WaitForShutdownWithCallback blocks until interrupt, then gracefully stops servers.
// The callback is invoked before stopping servers for additional cleanup (e.g., deregistration).
func WaitForShutdownWithCallback(server *engine.Server, adminAPI *admin.AdminAPI, callback ShutdownCallback) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\nShutting down...")

	// Run callback if provided (e.g., deregister from control plane)
	if callback != nil {
		callback()
	}

	// Stop admin API first
	if err := adminAPI.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: admin API shutdown error: %v\n", err)
	}

	// Stop mock server
	if err := server.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: server shutdown error: %v\n", err)
	}

	fmt.Println("Server stopped")
}

// WaitForShutdownWithContext blocks until interrupt or context cancellation,
// then gracefully stops servers. This variant supports context cancellation
// for coordinated shutdown (e.g., runtime mode with heartbeat loops).
func WaitForShutdownWithContext(ctx context.Context, server *engine.Server, adminAPI *admin.AdminAPI, callback ShutdownCallback) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
	case <-ctx.Done():
	}

	fmt.Println("\nShutting down...")

	// Run callback if provided
	if callback != nil {
		callback()
	}

	// Stop admin API first
	if err := adminAPI.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: admin API shutdown error: %v\n", err)
	}

	// Stop mock server
	if err := server.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: server shutdown error: %v\n", err)
	}

	fmt.Println("Server stopped")
}
