package mtls

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

// generateTestCertificate creates a client certificate signed by a CA for testing.
func generateTestCertificate(t *testing.T) *x509.Certificate {
	t.Helper()

	// Generate CA key and certificate
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate CA key: %v", err)
	}

	caTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "Test CA",
			Organization: []string{"Test CA Org"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create CA certificate: %v", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		t.Fatalf("failed to parse CA certificate: %v", err)
	}

	// Generate client key and certificate
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate client key: %v", err)
	}

	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(12345),
		Subject: pkix.Name{
			CommonName:         "test-client",
			Organization:       []string{"Test Org"},
			OrganizationalUnit: []string{"Test Unit"},
			Country:            []string{"US"},
		},
		NotBefore:      time.Now().Add(-1 * time.Hour),
		NotAfter:       time.Now().Add(24 * time.Hour),
		DNSNames:       []string{"localhost", "test.example.com"},
		EmailAddresses: []string{"test@example.com"},
		IPAddresses:    []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		KeyUsage:       x509.KeyUsageDigitalSignature,
		ExtKeyUsage:    []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	// Add URI SAN
	testURI, _ := url.Parse("spiffe://example.org/test")
	clientTemplate.URIs = []*url.URL{testURI}

	// Sign client cert with CA
	certDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatalf("failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse certificate: %v", err)
	}

	return cert
}

func TestExtractIdentity(t *testing.T) {
	cert := generateTestCertificate(t)

	t.Run("extracts identity from valid certificate", func(t *testing.T) {
		identity := ExtractIdentity(cert, true)

		if identity == nil {
			t.Fatal("expected identity, got nil")
		}

		if identity.CommonName != "test-client" {
			t.Errorf("expected CommonName 'test-client', got '%s'", identity.CommonName)
		}

		if len(identity.Organization) != 1 || identity.Organization[0] != "Test Org" {
			t.Errorf("expected Organization ['Test Org'], got %v", identity.Organization)
		}

		if len(identity.OrganizationalUnit) != 1 || identity.OrganizationalUnit[0] != "Test Unit" {
			t.Errorf("expected OrganizationalUnit ['Test Unit'], got %v", identity.OrganizationalUnit)
		}

		if len(identity.Country) != 1 || identity.Country[0] != "US" {
			t.Errorf("expected Country ['US'], got %v", identity.Country)
		}

		if identity.SerialNumber != "12345" {
			t.Errorf("expected SerialNumber '12345', got '%s'", identity.SerialNumber)
		}

		if identity.Issuer.CommonName != "Test CA" {
			t.Errorf("expected Issuer.CommonName 'Test CA', got '%s'", identity.Issuer.CommonName)
		}

		if !identity.Verified {
			t.Error("expected Verified to be true")
		}

		if identity.Fingerprint == "" {
			t.Error("expected non-empty fingerprint")
		}
	})

	t.Run("handles nil certificate", func(t *testing.T) {
		identity := ExtractIdentity(nil, false)
		if identity != nil {
			t.Error("expected nil identity for nil certificate")
		}
	})

	t.Run("extracts SANs correctly", func(t *testing.T) {
		identity := ExtractIdentity(cert, true)

		if len(identity.SANs.DNSNames) != 2 {
			t.Errorf("expected 2 DNS names, got %d", len(identity.SANs.DNSNames))
		}

		if len(identity.SANs.EmailAddresses) != 1 || identity.SANs.EmailAddresses[0] != "test@example.com" {
			t.Errorf("expected EmailAddresses ['test@example.com'], got %v", identity.SANs.EmailAddresses)
		}

		if len(identity.SANs.IPAddresses) != 2 {
			t.Errorf("expected 2 IP addresses, got %d", len(identity.SANs.IPAddresses))
		}

		if len(identity.SANs.URIs) != 1 || identity.SANs.URIs[0] != "spiffe://example.org/test" {
			t.Errorf("expected URIs ['spiffe://example.org/test'], got %v", identity.SANs.URIs)
		}
	})

	t.Run("sets verified flag correctly", func(t *testing.T) {
		verifiedIdentity := ExtractIdentity(cert, true)
		if !verifiedIdentity.Verified {
			t.Error("expected Verified to be true")
		}

		unverifiedIdentity := ExtractIdentity(cert, false)
		if unverifiedIdentity.Verified {
			t.Error("expected Verified to be false")
		}
	})
}

func TestFingerprint(t *testing.T) {
	cert := generateTestCertificate(t)

	t.Run("generates consistent fingerprint", func(t *testing.T) {
		fp1 := Fingerprint(cert)
		fp2 := Fingerprint(cert)

		if fp1 != fp2 {
			t.Errorf("fingerprints should be consistent, got '%s' and '%s'", fp1, fp2)
		}

		// SHA256 produces 64 hex characters
		if len(fp1) != 64 {
			t.Errorf("expected 64-character fingerprint, got %d characters", len(fp1))
		}
	})

	t.Run("handles nil certificate", func(t *testing.T) {
		fp := Fingerprint(nil)
		if fp != "" {
			t.Errorf("expected empty fingerprint for nil cert, got '%s'", fp)
		}
	})
}

func TestFromContext(t *testing.T) {
	t.Run("retrieves identity from context", func(t *testing.T) {
		identity := &ClientIdentity{
			CommonName: "test-client",
			Verified:   true,
		}

		ctx := WithIdentity(context.Background(), identity)
		retrieved := FromContext(ctx)

		if retrieved == nil {
			t.Fatal("expected identity, got nil")
		}

		if retrieved.CommonName != "test-client" {
			t.Errorf("expected CommonName 'test-client', got '%s'", retrieved.CommonName)
		}
	})

	t.Run("returns nil for missing identity", func(t *testing.T) {
		ctx := context.Background()
		retrieved := FromContext(ctx)

		if retrieved != nil {
			t.Errorf("expected nil, got %v", retrieved)
		}
	})

	t.Run("handles nil context", func(t *testing.T) {
		retrieved := FromContext(context.TODO())
		if retrieved != nil {
			t.Errorf("expected nil for nil context, got %v", retrieved)
		}
	})
}

func TestWithIdentity(t *testing.T) {
	t.Run("creates context with identity", func(t *testing.T) {
		identity := &ClientIdentity{
			CommonName: "test-client",
		}

		ctx := WithIdentity(context.Background(), identity)

		if ctx == nil {
			t.Fatal("expected non-nil context")
		}
	})

	t.Run("handles nil parent context", func(t *testing.T) {
		identity := &ClientIdentity{
			CommonName: "test-client",
		}

		ctx := WithIdentity(context.TODO(), identity)

		if ctx == nil {
			t.Fatal("expected non-nil context")
		}

		retrieved := FromContext(ctx)
		if retrieved == nil || retrieved.CommonName != "test-client" {
			t.Error("expected identity to be retrievable from context")
		}
	})
}

func TestExtractFromRequest(t *testing.T) {
	cert := generateTestCertificate(t)

	t.Run("extracts identity from TLS request", func(t *testing.T) {
		req := &http.Request{
			TLS: &tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{cert},
				VerifiedChains:   [][]*x509.Certificate{{cert}},
			},
		}

		identity := ExtractFromRequest(req)

		if identity == nil {
			t.Fatal("expected identity, got nil")
		}

		if identity.CommonName != "test-client" {
			t.Errorf("expected CommonName 'test-client', got '%s'", identity.CommonName)
		}

		if !identity.Verified {
			t.Error("expected Verified to be true when VerifiedChains is populated")
		}
	})

	t.Run("returns nil for nil request", func(t *testing.T) {
		identity := ExtractFromRequest(nil)
		if identity != nil {
			t.Errorf("expected nil for nil request, got %v", identity)
		}
	})

	t.Run("returns nil for non-TLS request", func(t *testing.T) {
		req := &http.Request{}
		identity := ExtractFromRequest(req)
		if identity != nil {
			t.Errorf("expected nil for non-TLS request, got %v", identity)
		}
	})

	t.Run("returns nil when no peer certificates", func(t *testing.T) {
		req := &http.Request{
			TLS: &tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{},
			},
		}
		identity := ExtractFromRequest(req)
		if identity != nil {
			t.Errorf("expected nil when no peer certificates, got %v", identity)
		}
	})

	t.Run("marks as unverified when no verified chains", func(t *testing.T) {
		req := &http.Request{
			TLS: &tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{cert},
				VerifiedChains:   nil,
			},
		}

		identity := ExtractFromRequest(req)

		if identity == nil {
			t.Fatal("expected identity, got nil")
		}

		if identity.Verified {
			t.Error("expected Verified to be false when VerifiedChains is empty")
		}
	})
}

func TestCopyStrings(t *testing.T) {
	t.Run("copies string slice", func(t *testing.T) {
		original := []string{"a", "b", "c"}
		copied := copyStrings(original)

		if len(copied) != len(original) {
			t.Errorf("expected length %d, got %d", len(original), len(copied))
		}

		// Modify original to ensure copy is independent
		original[0] = "modified"
		if copied[0] == "modified" {
			t.Error("copy should be independent of original")
		}
	})

	t.Run("returns nil for empty slice", func(t *testing.T) {
		result := copyStrings([]string{})
		if result != nil {
			t.Errorf("expected nil for empty slice, got %v", result)
		}
	})

	t.Run("returns nil for nil slice", func(t *testing.T) {
		result := copyStrings(nil)
		if result != nil {
			t.Errorf("expected nil for nil slice, got %v", result)
		}
	})
}
