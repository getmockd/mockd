package tls

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// SaveCertToFiles saves a certificate and private key to PEM files.
func SaveCertToFiles(cert *GeneratedCertificate, certPath, keyPath string) error {
	if cert == nil {
		return errors.New("certificate cannot be nil")
	}

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(certPath), 0755); err != nil {
		return fmt.Errorf("failed to create certificate directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0755); err != nil {
		return fmt.Errorf("failed to create key directory: %w", err)
	}

	// Write certificate
	if err := os.WriteFile(certPath, cert.CertPEM, 0644); err != nil {
		return fmt.Errorf("failed to write certificate file: %w", err)
	}

	// Write private key with restricted permissions
	if err := os.WriteFile(keyPath, cert.KeyPEM, 0600); err != nil {
		// Clean up cert file if key write fails
		_ = os.Remove(certPath)
		return fmt.Errorf("failed to write key file: %w", err)
	}

	return nil
}

// LoadCertFromFiles loads a certificate and private key from PEM files.
func LoadCertFromFiles(certPath, keyPath string) (*GeneratedCertificate, error) {
	// Read certificate file
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate file: %w", err)
	}

	// Read key file
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	// Decode certificate
	cert, err := DecodeCertFromPEM(certPEM)
	if err != nil {
		return nil, err
	}

	// Decode key
	key, err := DecodeKeyFromPEM(keyPEM)
	if err != nil {
		return nil, err
	}

	return &GeneratedCertificate{
		Certificate: cert,
		PrivateKey:  key,
		CertPEM:     certPEM,
		KeyPEM:      keyPEM,
	}, nil
}

// LoadTLSCertificate loads certificate and key files and returns a tls.Certificate.
func LoadTLSCertificate(certPath, keyPath string) (tls.Certificate, error) {
	return tls.LoadX509KeyPair(certPath, keyPath)
}

// CreateTLSCertificate creates a tls.Certificate from PEM bytes.
func CreateTLSCertificate(certPEM, keyPEM []byte) (tls.Certificate, error) {
	return tls.X509KeyPair(certPEM, keyPEM)
}

// GenerateAndSave generates a new self-signed certificate and saves it to files.
func GenerateAndSave(cfg *CertificateConfig, certPath, keyPath string) (*GeneratedCertificate, error) {
	cert, err := GenerateSelfSignedCert(cfg)
	if err != nil {
		return nil, err
	}

	if err := SaveCertToFiles(cert, certPath, keyPath); err != nil {
		return nil, err
	}

	return cert, nil
}

// EnsureCertificate ensures a certificate exists at the given paths.
// If files don't exist, generates new ones. If they exist, loads them.
func EnsureCertificate(cfg *CertificateConfig, certPath, keyPath string) (*GeneratedCertificate, error) {
	// Check if both files exist
	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)

	if certErr == nil && keyErr == nil {
		// Both files exist, load them
		return LoadCertFromFiles(certPath, keyPath)
	}

	// Generate new certificate
	return GenerateAndSave(cfg, certPath, keyPath)
}

// CertificateInfo contains human-readable information about a certificate.
type CertificateInfo struct {
	Subject      string
	Issuer       string
	SerialNumber string
	NotBefore    string
	NotAfter     string
	DNSNames     []string
	IPAddresses  []string
	IsCA         bool
}

// GetCertificateInfo extracts information from a certificate.
func GetCertificateInfo(cert *x509.Certificate) *CertificateInfo {
	ipAddresses := make([]string, len(cert.IPAddresses))
	for i, ip := range cert.IPAddresses {
		ipAddresses[i] = ip.String()
	}

	return &CertificateInfo{
		Subject:      cert.Subject.String(),
		Issuer:       cert.Issuer.String(),
		SerialNumber: cert.SerialNumber.String(),
		NotBefore:    cert.NotBefore.Format("2006-01-02 15:04:05"),
		NotAfter:     cert.NotAfter.Format("2006-01-02 15:04:05"),
		DNSNames:     cert.DNSNames,
		IPAddresses:  ipAddresses,
		IsCA:         cert.IsCA,
	}
}

// VerifyKeyPair verifies that a certificate and private key form a valid pair.
func VerifyKeyPair(cert *x509.Certificate, key *ecdsa.PrivateKey) error {
	// Get public key from certificate
	certPubKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return errors.New("certificate public key is not ECDSA")
	}

	// Compare public keys
	if certPubKey.X.Cmp(key.X) != 0 || certPubKey.Y.Cmp(key.Y) != 0 {
		return errors.New("private key does not match certificate public key")
	}

	return nil
}
