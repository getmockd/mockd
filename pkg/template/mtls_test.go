package template

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/getmockd/mockd/pkg/mtls"
)

func TestMTLSTemplateVariables(t *testing.T) {
	engine := New()

	// Create a mock mTLS identity
	identity := &mtls.ClientIdentity{
		CommonName:         "test-client",
		Organization:       []string{"Test Org", "Secondary Org"},
		OrganizationalUnit: []string{"Engineering", "DevOps"},
		SerialNumber:       "123456789",
		Fingerprint:        "abc123def456",
		Issuer: mtls.IssuerInfo{
			CommonName:   "Test CA",
			Organization: []string{"CA Org"},
		},
		NotBefore: "2024-01-01T00:00:00Z",
		NotAfter:  "2025-01-01T00:00:00Z",
		SANs: mtls.SubjectAltNames{
			DNSNames:       []string{"client.example.com", "backup.example.com"},
			EmailAddresses: []string{"client@example.com", "alt@example.com"},
			IPAddresses:    []string{"192.168.1.100", "10.0.0.1"},
			URIs:           []string{"spiffe://example.org/client"},
		},
		Verified: true,
	}

	// Create a context with mTLS identity
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)
	ctx.SetMTLSFromIdentity(identity)

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "mtls.cn",
			template: "{{mtls.cn}}",
			expected: "test-client",
		},
		{
			name:     "mtls.o",
			template: "{{mtls.o}}",
			expected: "Test Org",
		},
		{
			name:     "mtls.ou",
			template: "{{mtls.ou}}",
			expected: "Engineering",
		},
		{
			name:     "mtls.serial",
			template: "{{mtls.serial}}",
			expected: "123456789",
		},
		{
			name:     "mtls.fingerprint",
			template: "{{mtls.fingerprint}}",
			expected: "abc123def456",
		},
		{
			name:     "mtls.issuer.cn",
			template: "{{mtls.issuer.cn}}",
			expected: "Test CA",
		},
		{
			name:     "mtls.notBefore",
			template: "{{mtls.notBefore}}",
			expected: "2024-01-01T00:00:00Z",
		},
		{
			name:     "mtls.notAfter",
			template: "{{mtls.notAfter}}",
			expected: "2025-01-01T00:00:00Z",
		},
		{
			name:     "mtls.san.dns",
			template: "{{mtls.san.dns}}",
			expected: "client.example.com",
		},
		{
			name:     "mtls.san.email",
			template: "{{mtls.san.email}}",
			expected: "client@example.com",
		},
		{
			name:     "mtls.san.ip",
			template: "{{mtls.san.ip}}",
			expected: "192.168.1.100",
		},
		{
			name:     "mtls.san.uri",
			template: "{{mtls.san.uri}}",
			expected: "spiffe://example.org/client",
		},
		{
			name:     "mtls.verified",
			template: "{{mtls.verified}}",
			expected: "true",
		},
		{
			name:     "combined template",
			template: `{"cn": "{{mtls.cn}}", "org": "{{mtls.o}}", "verified": {{mtls.verified}}}`,
			expected: `{"cn": "test-client", "org": "Test Org", "verified": true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Process(tt.template, ctx)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("Process() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMTLSTemplateVariables_NoMTLS(t *testing.T) {
	engine := New()

	// Create a context without mTLS identity
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)
	// Don't set mTLS identity - simulates non-mTLS request

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "mtls.cn returns empty",
			template: "{{mtls.cn}}",
			expected: "",
		},
		{
			name:     "mtls.verified returns empty",
			template: "{{mtls.verified}}",
			expected: "",
		},
		{
			name:     "mtls.issuer.cn returns empty",
			template: "{{mtls.issuer.cn}}",
			expected: "",
		},
		{
			name:     "mixed template",
			template: `{"client": "{{mtls.cn}}", "path": "{{request.path}}"}`,
			expected: `{"client": "", "path": "/test"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Process(tt.template, ctx)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("Process() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMTLSTemplateVariables_PartialIdentity(t *testing.T) {
	engine := New()

	// Create a minimal mTLS identity (no SANs, no Org/OU)
	identity := &mtls.ClientIdentity{
		CommonName:   "minimal-client",
		SerialNumber: "999",
		Fingerprint:  "minimalfp",
		Issuer: mtls.IssuerInfo{
			CommonName: "Minimal CA",
		},
		NotBefore: "2024-06-01T00:00:00Z",
		NotAfter:  "2024-12-01T00:00:00Z",
		Verified:  false,
	}

	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)
	ctx.SetMTLSFromIdentity(identity)

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "mtls.cn populated",
			template: "{{mtls.cn}}",
			expected: "minimal-client",
		},
		{
			name:     "mtls.o empty when no org",
			template: "{{mtls.o}}",
			expected: "",
		},
		{
			name:     "mtls.ou empty when no ou",
			template: "{{mtls.ou}}",
			expected: "",
		},
		{
			name:     "mtls.san.dns empty when no DNS SANs",
			template: "{{mtls.san.dns}}",
			expected: "",
		},
		{
			name:     "mtls.san.email empty when no email SANs",
			template: "{{mtls.san.email}}",
			expected: "",
		},
		{
			name:     "mtls.san.ip empty when no IP SANs",
			template: "{{mtls.san.ip}}",
			expected: "",
		},
		{
			name:     "mtls.san.uri empty when no URI SANs",
			template: "{{mtls.san.uri}}",
			expected: "",
		},
		{
			name:     "mtls.verified false",
			template: "{{mtls.verified}}",
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Process(tt.template, ctx)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if result != tt.expected {
				t.Errorf("Process() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestMTLSTemplateVariables_NilContext(t *testing.T) {
	engine := New()

	// Test with nil context - should not panic
	result, err := engine.Process("{{mtls.cn}}", nil)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result != "" {
		t.Errorf("Process() = %q, want empty string", result)
	}
}

func TestSetMTLSFromIdentity_NilIdentity(t *testing.T) {
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)

	// Should not panic when identity is nil
	ctx.SetMTLSFromIdentity(nil)

	if ctx.MTLS.Present {
		t.Error("MTLS.Present should be false when identity is nil")
	}
	if ctx.MTLS.CN != "" {
		t.Error("MTLS.CN should be empty when identity is nil")
	}
}

// Ensure the http import is used
var _ = http.MethodGet
