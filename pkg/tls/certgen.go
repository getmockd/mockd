// Package tls provides utilities for TLS certificate generation and management.
package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"
)

// CertificateConfig contains options for certificate generation.
type CertificateConfig struct {
	// Organization name for the certificate
	Organization string
	// Common name (CN) for the certificate
	CommonName string
	// Additional DNS names for the certificate
	DNSNames []string
	// Additional IP addresses for the certificate
	IPAddresses []net.IP
	// Validity duration
	ValidFor time.Duration
	// Whether this is a CA certificate
	IsCA bool
}

// DefaultCertificateConfig returns a default configuration suitable for local development.
func DefaultCertificateConfig() *CertificateConfig {
	return &CertificateConfig{
		Organization: "mockd",
		CommonName:   "localhost",
		DNSNames:     []string{"localhost", "127.0.0.1", "::1"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		ValidFor:     365 * 24 * time.Hour, // 1 year
		IsCA:         true,
	}
}

// GeneratedCertificate contains a generated certificate and its private key.
type GeneratedCertificate struct {
	Certificate *x509.Certificate
	PrivateKey  *ecdsa.PrivateKey
	CertPEM     []byte
	KeyPEM      []byte
}

// GeneratePrivateKey generates a new ECDSA private key using P-256 curve.
func GeneratePrivateKey() (*ecdsa.PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}
	return key, nil
}

// CreateCertificateTemplate creates an x509 certificate template with the given config.
func CreateCertificateTemplate(cfg *CertificateConfig) (*x509.Certificate, error) {
	if cfg == nil {
		cfg = DefaultCertificateConfig()
	}

	// Generate a random serial number
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(cfg.ValidFor)

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{cfg.Organization},
			CommonName:   cfg.CommonName,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              cfg.DNSNames,
		IPAddresses:           cfg.IPAddresses,
	}

	if cfg.IsCA {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	return template, nil
}

// GenerateSelfSignedCert generates a self-signed certificate with the given configuration.
func GenerateSelfSignedCert(cfg *CertificateConfig) (*GeneratedCertificate, error) {
	if cfg == nil {
		cfg = DefaultCertificateConfig()
	}

	// Generate private key
	privateKey, err := GeneratePrivateKey()
	if err != nil {
		return nil, err
	}

	// Create certificate template
	template, err := CreateCertificateTemplate(cfg)
	if err != nil {
		return nil, err
	}

	// Create certificate (self-signed, so parent = template)
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	// Parse the created certificate
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Encode to PEM
	certPEM, err := EncodeCertToPEM(certDER)
	if err != nil {
		return nil, err
	}

	keyPEM, err := EncodeKeyToPEM(privateKey)
	if err != nil {
		return nil, err
	}

	return &GeneratedCertificate{
		Certificate: cert,
		PrivateKey:  privateKey,
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
	}, nil
}

// EncodeCertToPEM encodes a DER certificate to PEM format.
func EncodeCertToPEM(certDER []byte) ([]byte, error) {
	block := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDER,
	}
	return pem.EncodeToMemory(block), nil
}

// EncodeKeyToPEM encodes an ECDSA private key to PEM format.
func EncodeKeyToPEM(key *ecdsa.PrivateKey) ([]byte, error) {
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	block := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDER,
	}
	return pem.EncodeToMemory(block), nil
}

// DecodeCertFromPEM decodes a PEM-encoded certificate.
func DecodeCertFromPEM(certPEM []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("unexpected PEM block type: %s", block.Type)
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, nil
}

// DecodeKeyFromPEM decodes a PEM-encoded ECDSA private key.
func DecodeKeyFromPEM(keyPEM []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(keyPEM)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	if block.Type != "EC PRIVATE KEY" {
		return nil, fmt.Errorf("unexpected PEM block type: %s", block.Type)
	}

	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return key, nil
}
