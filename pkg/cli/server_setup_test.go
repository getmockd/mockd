package cli

import (
	"flag"
	"testing"
)

func TestRegisterServerFlags(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var sf ServerFlags
	RegisterServerFlags(fs, &sf)

	// Parse with defaults
	if err := fs.Parse([]string{}); err != nil {
		t.Fatalf("Failed to parse empty args: %v", err)
	}

	// Check defaults
	if sf.Port != 4280 {
		t.Errorf("Expected default port 4280, got %d", sf.Port)
	}
	if sf.AdminPort != 4290 {
		t.Errorf("Expected default admin port 4290, got %d", sf.AdminPort)
	}
	if sf.HTTPSPort != 0 {
		t.Errorf("Expected default HTTPS port 0, got %d", sf.HTTPSPort)
	}
	if sf.GraphQLPath != "/graphql" {
		t.Errorf("Expected default GraphQL path /graphql, got %s", sf.GraphQLPath)
	}
}

func TestRegisterServerFlags_CustomValues(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	var sf ServerFlags
	RegisterServerFlags(fs, &sf)

	// Parse with custom values
	args := []string{
		"--port", "3000",
		"--admin-port", "4000",
		"--https-port", "8443",
		"--tls-cert", "/path/to/cert",
		"--tls-key", "/path/to/key",
		"--mtls-enabled",
		"--mtls-ca", "/path/to/ca",
		"--audit-enabled",
		"--graphql-schema", "/path/to/schema.graphql",
		"--grpc-port", "50051",
		"--grpc-proto", "/path/to/service.proto",
		"--oauth-enabled",
		"--mqtt-port", "1883",
		"--chaos-enabled",
		"--chaos-latency", "10ms-100ms",
		"--chaos-error-rate", "0.1",
		"--validate-spec", "/path/to/spec.yaml",
	}

	if err := fs.Parse(args); err != nil {
		t.Fatalf("Failed to parse args: %v", err)
	}

	if sf.Port != 3000 {
		t.Errorf("Expected port 3000, got %d", sf.Port)
	}
	if sf.AdminPort != 4000 {
		t.Errorf("Expected admin port 4000, got %d", sf.AdminPort)
	}
	if sf.HTTPSPort != 8443 {
		t.Errorf("Expected HTTPS port 8443, got %d", sf.HTTPSPort)
	}
	if sf.TLSCert != "/path/to/cert" {
		t.Errorf("Expected TLS cert path, got %s", sf.TLSCert)
	}
	if !sf.MTLSEnabled {
		t.Error("Expected mTLS enabled")
	}
	if !sf.AuditEnabled {
		t.Error("Expected audit enabled")
	}
	if sf.GRPCPort != 50051 {
		t.Errorf("Expected gRPC port 50051, got %d", sf.GRPCPort)
	}
	if !sf.OAuthEnabled {
		t.Error("Expected OAuth enabled")
	}
	if sf.MQTTPort != 1883 {
		t.Errorf("Expected MQTT port 1883, got %d", sf.MQTTPort)
	}
	if !sf.ChaosEnabled {
		t.Error("Expected chaos enabled")
	}
	if sf.ChaosLatency != "10ms-100ms" {
		t.Errorf("Expected chaos latency 10ms-100ms, got %s", sf.ChaosLatency)
	}
	if sf.ChaosErrorRate != 0.1 {
		t.Errorf("Expected chaos error rate 0.1, got %f", sf.ChaosErrorRate)
	}
}

func TestBuildServerConfig(t *testing.T) {
	sf := &ServerFlags{
		Port:          4280,
		AdminPort:     4290,
		HTTPSPort:     4283,
		ReadTimeout:   30,
		WriteTimeout:  30,
		MaxLogEntries: 1000,
		AutoCert:      true,
	}

	cfg := BuildServerConfig(sf)

	if cfg.HTTPPort != 4280 {
		t.Errorf("Expected HTTP port 4280, got %d", cfg.HTTPPort)
	}
	if cfg.HTTPSPort != 4283 {
		t.Errorf("Expected HTTPS port 4283, got %d", cfg.HTTPSPort)
	}
	if cfg.AdminPort != 4290 {
		t.Errorf("Expected admin port 4290, got %d", cfg.AdminPort)
	}
	if !cfg.LogRequests {
		t.Error("Expected LogRequests to be true")
	}
}

func TestBuildTLSConfig(t *testing.T) {
	sf := &ServerFlags{
		TLSCert: "/path/to/cert",
		TLSKey:  "/path/to/key",
		TLSAuto: true,
	}

	cfg := BuildTLSConfig(sf)

	if !cfg.Enabled {
		t.Error("Expected TLS enabled")
	}
	if cfg.CertFile != "/path/to/cert" {
		t.Errorf("Expected cert file path, got %s", cfg.CertFile)
	}
	if cfg.KeyFile != "/path/to/key" {
		t.Errorf("Expected key file path, got %s", cfg.KeyFile)
	}
	if !cfg.AutoGenerateCert {
		t.Error("Expected auto generate cert to be true")
	}
}

func TestBuildMTLSConfig(t *testing.T) {
	sf := &ServerFlags{
		MTLSEnabled:    true,
		MTLSClientAuth: "require-and-verify",
		MTLSCA:         "/path/to/ca",
		MTLSAllowedCNs: "client1,client2, client3 ",
	}

	cfg := BuildMTLSConfig(sf)

	if !cfg.Enabled {
		t.Error("Expected mTLS enabled")
	}
	if cfg.ClientAuth != "require-and-verify" {
		t.Errorf("Expected client auth, got %s", cfg.ClientAuth)
	}
	if cfg.CACertFile != "/path/to/ca" {
		t.Errorf("Expected CA cert file, got %s", cfg.CACertFile)
	}
	if len(cfg.AllowedCNs) != 3 {
		t.Errorf("Expected 3 allowed CNs, got %d", len(cfg.AllowedCNs))
	}
	// Check trimming works
	if cfg.AllowedCNs[2] != "client3" {
		t.Errorf("Expected trimmed CN 'client3', got '%s'", cfg.AllowedCNs[2])
	}
}

func TestBuildChaosConfig_Disabled(t *testing.T) {
	sf := &ServerFlags{
		ChaosEnabled: false,
	}

	cfg := BuildChaosConfig(sf)

	if cfg != nil {
		t.Error("Expected nil chaos config when disabled")
	}
}

func TestBuildChaosConfig_WithLatency(t *testing.T) {
	sf := &ServerFlags{
		ChaosEnabled: true,
		ChaosLatency: "10ms-100ms",
	}

	cfg := BuildChaosConfig(sf)

	if cfg == nil {
		t.Fatal("Expected non-nil chaos config")
	}
	if !cfg.Enabled {
		t.Error("Expected chaos enabled")
	}
	if cfg.GlobalRules == nil {
		t.Fatal("Expected global rules")
	}
	if cfg.GlobalRules.Latency == nil {
		t.Fatal("Expected latency fault")
	}
	if cfg.GlobalRules.Latency.Min != "10ms" {
		t.Errorf("Expected min latency 10ms, got %s", cfg.GlobalRules.Latency.Min)
	}
	if cfg.GlobalRules.Latency.Max != "100ms" {
		t.Errorf("Expected max latency 100ms, got %s", cfg.GlobalRules.Latency.Max)
	}
}

func TestBuildChaosConfig_WithErrorRate(t *testing.T) {
	sf := &ServerFlags{
		ChaosEnabled:   true,
		ChaosErrorRate: 0.5,
	}

	cfg := BuildChaosConfig(sf)

	if cfg == nil {
		t.Fatal("Expected non-nil chaos config")
	}
	if cfg.GlobalRules == nil {
		t.Fatal("Expected global rules")
	}
	if cfg.GlobalRules.ErrorRate == nil {
		t.Fatal("Expected error rate fault")
	}
	if cfg.GlobalRules.ErrorRate.Probability != 0.5 {
		t.Errorf("Expected error probability 0.5, got %f", cfg.GlobalRules.ErrorRate.Probability)
	}
	if cfg.GlobalRules.ErrorRate.DefaultCode != 500 {
		t.Errorf("Expected default code 500, got %d", cfg.GlobalRules.ErrorRate.DefaultCode)
	}
}

func TestParseLatencyRange(t *testing.T) {
	tests := []struct {
		input     string
		expectMin string
		expectMax string
	}{
		{"10ms-100ms", "10ms", "100ms"},
		{"50ms", "50ms", "50ms"},
		{"1s-5s", "1s", "5s"},
		{" 10ms - 100ms ", "10ms", "100ms"},
	}

	for _, tt := range tests {
		min, max := ParseLatencyRange(tt.input)
		if min != tt.expectMin {
			t.Errorf("For %q, expected min %q, got %q", tt.input, tt.expectMin, min)
		}
		if max != tt.expectMax {
			t.Errorf("For %q, expected max %q, got %q", tt.input, tt.expectMax, max)
		}
	}
}

func TestValidateGRPCFlags(t *testing.T) {
	// Valid: no gRPC
	sf := &ServerFlags{
		GRPCPort: 0,
	}
	if err := ValidateGRPCFlags(sf); err != nil {
		t.Errorf("Expected no error for disabled gRPC, got %v", err)
	}

	// Valid: gRPC with proto
	sf = &ServerFlags{
		GRPCPort:  50051,
		GRPCProto: "/path/to/service.proto",
	}
	if err := ValidateGRPCFlags(sf); err != nil {
		t.Errorf("Expected no error for valid gRPC config, got %v", err)
	}

	// Invalid: gRPC without proto
	sf = &ServerFlags{
		GRPCPort: 50051,
	}
	if err := ValidateGRPCFlags(sf); err == nil {
		t.Error("Expected error for gRPC without proto")
	}
}

func TestBuildGraphQLConfig(t *testing.T) {
	sf := &ServerFlags{
		GraphQLSchema: "/path/to/schema.graphql",
		GraphQLPath:   "/api/graphql",
	}

	cfgs := BuildGraphQLConfig(sf)

	if len(cfgs) != 1 {
		t.Fatalf("Expected 1 GraphQL config, got %d", len(cfgs))
	}
	if cfgs[0].Path != "/api/graphql" {
		t.Errorf("Expected path /api/graphql, got %s", cfgs[0].Path)
	}
	if cfgs[0].SchemaFile != "/path/to/schema.graphql" {
		t.Errorf("Expected schema file path, got %s", cfgs[0].SchemaFile)
	}
	if !cfgs[0].Introspection {
		t.Error("Expected introspection enabled")
	}
}

func TestBuildOAuthConfig(t *testing.T) {
	// With explicit issuer
	sf := &ServerFlags{
		OAuthEnabled: true,
		OAuthIssuer:  "https://auth.example.com",
		OAuthPort:    8080,
	}

	cfgs := BuildOAuthConfig(sf)

	if len(cfgs) != 1 {
		t.Fatalf("Expected 1 OAuth config, got %d", len(cfgs))
	}
	if cfgs[0].Issuer != "https://auth.example.com" {
		t.Errorf("Expected issuer URL, got %s", cfgs[0].Issuer)
	}

	// Without explicit issuer (defaults to localhost:port)
	sf = &ServerFlags{
		OAuthEnabled: true,
		OAuthPort:    9000,
	}

	cfgs = BuildOAuthConfig(sf)
	if cfgs[0].Issuer != "http://localhost:9000" {
		t.Errorf("Expected default issuer, got %s", cfgs[0].Issuer)
	}
}
