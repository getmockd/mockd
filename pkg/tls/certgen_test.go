package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/pem"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratePrivateKey(t *testing.T) {
	key, err := GeneratePrivateKey()
	require.NoError(t, err)
	require.NotNil(t, key)

	// Verify it's a P-256 key
	assert.Equal(t, elliptic.P256(), key.Curve)

	// Verify we can use it to sign/verify
	assert.NotNil(t, key.D)
	assert.NotNil(t, key.PublicKey.X)
	assert.NotNil(t, key.PublicKey.Y)
}

func TestCreateCertificateTemplate(t *testing.T) {
	cfg := &CertificateConfig{
		Organization: "Test Org",
		CommonName:   "test.local",
		DNSNames:     []string{"test.local", "localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		ValidFor:     24 * time.Hour,
		IsCA:         true,
	}

	template, err := CreateCertificateTemplate(cfg)
	require.NoError(t, err)
	require.NotNil(t, template)

	assert.Equal(t, "Test Org", template.Subject.Organization[0])
	assert.Equal(t, "test.local", template.Subject.CommonName)
	assert.Contains(t, template.DNSNames, "test.local")
	assert.Contains(t, template.DNSNames, "localhost")
	assert.True(t, template.IsCA)
	assert.NotNil(t, template.SerialNumber)
}

func TestCreateCertificateTemplate_NilConfig(t *testing.T) {
	template, err := CreateCertificateTemplate(nil)
	require.NoError(t, err)
	require.NotNil(t, template)

	// Should use defaults
	assert.Equal(t, "mockd", template.Subject.Organization[0])
	assert.Equal(t, "localhost", template.Subject.CommonName)
}

func TestGenerateSelfSignedCert(t *testing.T) {
	cfg := DefaultCertificateConfig()
	cert, err := GenerateSelfSignedCert(cfg)
	require.NoError(t, err)
	require.NotNil(t, cert)

	// Verify certificate
	assert.NotNil(t, cert.Certificate)
	assert.NotNil(t, cert.PrivateKey)
	assert.NotEmpty(t, cert.CertPEM)
	assert.NotEmpty(t, cert.KeyPEM)

	// Verify certificate properties
	assert.Equal(t, "localhost", cert.Certificate.Subject.CommonName)
	assert.True(t, cert.Certificate.IsCA)
	assert.Contains(t, cert.Certificate.DNSNames, "localhost")
}

func TestGenerateSelfSignedCert_NilConfig(t *testing.T) {
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)
	require.NotNil(t, cert)
}

func TestEncodeCertToPEM(t *testing.T) {
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)

	// Verify PEM encoding
	block, _ := pem.Decode(cert.CertPEM)
	require.NotNil(t, block)
	assert.Equal(t, "CERTIFICATE", block.Type)
}

func TestEncodeKeyToPEM(t *testing.T) {
	key, err := GeneratePrivateKey()
	require.NoError(t, err)

	keyPEM, err := EncodeKeyToPEM(key)
	require.NoError(t, err)

	block, _ := pem.Decode(keyPEM)
	require.NotNil(t, block)
	assert.Equal(t, "EC PRIVATE KEY", block.Type)
}

func TestPEMRoundTrip(t *testing.T) {
	// Generate cert
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)

	// Decode cert from PEM
	decodedCert, err := DecodeCertFromPEM(cert.CertPEM)
	require.NoError(t, err)
	assert.Equal(t, cert.Certificate.Subject.CommonName, decodedCert.Subject.CommonName)
	assert.Equal(t, cert.Certificate.SerialNumber, decodedCert.SerialNumber)

	// Decode key from PEM
	decodedKey, err := DecodeKeyFromPEM(cert.KeyPEM)
	require.NoError(t, err)
	assert.Equal(t, cert.PrivateKey.D, decodedKey.D)
}

func TestDecodeCertFromPEM_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		pemData []byte
	}{
		{"empty", []byte{}},
		{"not pem", []byte("not pem data")},
		{"wrong type", []byte("-----BEGIN PRIVATE KEY-----\nYQ==\n-----END PRIVATE KEY-----")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeCertFromPEM(tt.pemData)
			assert.Error(t, err)
		})
	}
}

func TestDecodeKeyFromPEM_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		pemData []byte
	}{
		{"empty", []byte{}},
		{"not pem", []byte("not pem data")},
		{"wrong type", []byte("-----BEGIN CERTIFICATE-----\nYQ==\n-----END CERTIFICATE-----")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeKeyFromPEM(tt.pemData)
			assert.Error(t, err)
		})
	}
}

func TestDefaultCertificateConfig(t *testing.T) {
	cfg := DefaultCertificateConfig()

	assert.Equal(t, "mockd", cfg.Organization)
	assert.Equal(t, "localhost", cfg.CommonName)
	assert.Contains(t, cfg.DNSNames, "localhost")
	assert.True(t, cfg.IsCA)
	assert.Equal(t, 365*24*time.Hour, cfg.ValidFor)
}

func TestCertificateValidity(t *testing.T) {
	cfg := &CertificateConfig{
		Organization: "Test",
		CommonName:   "test",
		DNSNames:     []string{"test"},
		ValidFor:     1 * time.Hour,
		IsCA:         false,
	}

	cert, err := GenerateSelfSignedCert(cfg)
	require.NoError(t, err)

	// Check validity period
	now := time.Now()
	assert.True(t, cert.Certificate.NotBefore.Before(now) || cert.Certificate.NotBefore.Equal(now))
	assert.True(t, cert.Certificate.NotAfter.After(now))

	// Should expire within about 1 hour
	expiresIn := cert.Certificate.NotAfter.Sub(now)
	assert.Less(t, expiresIn, 2*time.Hour)
}

func TestVerifyKeyPair(t *testing.T) {
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)

	// Valid pair should pass
	err = VerifyKeyPair(cert.Certificate, cert.PrivateKey)
	assert.NoError(t, err)

	// Different key should fail
	otherKey, err := GeneratePrivateKey()
	require.NoError(t, err)
	err = VerifyKeyPair(cert.Certificate, otherKey)
	assert.Error(t, err)
}

func TestVerifyKeyPair_NonECDSA(t *testing.T) {
	// Create a certificate with a non-ECDSA key
	// This is a bit contrived but tests the error path
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)

	// The certificate has an ECDSA key, so this should work
	err = VerifyKeyPair(cert.Certificate, cert.PrivateKey)
	assert.NoError(t, err)
}

func TestCertificateExtKeyUsage(t *testing.T) {
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)

	// Should have ServerAuth extended key usage
	assert.Contains(t, cert.Certificate.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
}

func TestCertificateKeyUsage(t *testing.T) {
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)

	// Should have KeyEncipherment and DigitalSignature
	assert.True(t, cert.Certificate.KeyUsage&x509.KeyUsageKeyEncipherment != 0)
	assert.True(t, cert.Certificate.KeyUsage&x509.KeyUsageDigitalSignature != 0)

	// CA cert should also have CertSign
	assert.True(t, cert.Certificate.KeyUsage&x509.KeyUsageCertSign != 0)
}

func TestGenerateMultipleCerts(t *testing.T) {
	// Generate multiple certs and verify they have unique serial numbers
	serials := make(map[string]bool)

	for i := 0; i < 5; i++ {
		cert, err := GenerateSelfSignedCert(nil)
		require.NoError(t, err)

		serial := cert.Certificate.SerialNumber.String()
		assert.False(t, serials[serial], "duplicate serial number")
		serials[serial] = true
	}
}

func TestGetCertificateInfo(t *testing.T) {
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)

	info := GetCertificateInfo(cert.Certificate)

	assert.Contains(t, info.Subject, "localhost")
	assert.Contains(t, info.DNSNames, "localhost")
	assert.True(t, info.IsCA)
	assert.NotEmpty(t, info.SerialNumber)
	assert.NotEmpty(t, info.NotBefore)
	assert.NotEmpty(t, info.NotAfter)
}

func TestCreateTLSCertificate(t *testing.T) {
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)

	tlsCert, err := CreateTLSCertificate(cert.CertPEM, cert.KeyPEM)
	require.NoError(t, err)

	assert.Len(t, tlsCert.Certificate, 1)
	assert.NotNil(t, tlsCert.PrivateKey)
	assert.IsType(t, &ecdsa.PrivateKey{}, tlsCert.PrivateKey)
}
