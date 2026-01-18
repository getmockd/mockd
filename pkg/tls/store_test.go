package tls

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSaveAndLoadCertFromFiles(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Generate certificate
	original, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)

	// Save to files
	err = SaveCertToFiles(original, certPath, keyPath)
	require.NoError(t, err)

	// Verify files exist
	_, err = os.Stat(certPath)
	require.NoError(t, err)
	_, err = os.Stat(keyPath)
	require.NoError(t, err)

	// Verify key file has restricted permissions
	keyInfo, err := os.Stat(keyPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), keyInfo.Mode().Perm())

	// Load from files
	loaded, err := LoadCertFromFiles(certPath, keyPath)
	require.NoError(t, err)

	// Compare
	assert.Equal(t, original.Certificate.SerialNumber, loaded.Certificate.SerialNumber)
	assert.Equal(t, original.Certificate.Subject.CommonName, loaded.Certificate.Subject.CommonName)
	assert.Equal(t, original.PrivateKey.D, loaded.PrivateKey.D)
}

func TestSaveCertToFiles_NilCert(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	err := SaveCertToFiles(nil, certPath, keyPath)
	assert.Error(t, err)
}

func TestSaveCertToFiles_CreateNestedDirs(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "a", "b", "c", "cert.pem")
	keyPath := filepath.Join(tmpDir, "x", "y", "z", "key.pem")

	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)

	err = SaveCertToFiles(cert, certPath, keyPath)
	require.NoError(t, err)

	// Verify both files exist
	_, err = os.Stat(certPath)
	assert.NoError(t, err)
	_, err = os.Stat(keyPath)
	assert.NoError(t, err)
}

func TestLoadCertFromFiles_FileNotFound(t *testing.T) {
	_, err := LoadCertFromFiles("/nonexistent/cert.pem", "/nonexistent/key.pem")
	assert.Error(t, err)
}

func TestLoadTLSCertificate(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Generate and save certificate
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)
	err = SaveCertToFiles(cert, certPath, keyPath)
	require.NoError(t, err)

	// Load as tls.Certificate
	tlsCert, err := LoadTLSCertificate(certPath, keyPath)
	require.NoError(t, err)

	assert.Len(t, tlsCert.Certificate, 1)
	assert.NotNil(t, tlsCert.PrivateKey)
}

func TestGenerateAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "gen-cert.pem")
	keyPath := filepath.Join(tmpDir, "gen-key.pem")

	cert, err := GenerateAndSave(nil, certPath, keyPath)
	require.NoError(t, err)
	require.NotNil(t, cert)

	// Verify files exist
	_, err = os.Stat(certPath)
	assert.NoError(t, err)
	_, err = os.Stat(keyPath)
	assert.NoError(t, err)

	// Verify content
	assert.Equal(t, "localhost", cert.Certificate.Subject.CommonName)
}

func TestEnsureCertificate_Generate(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "ensure-cert.pem")
	keyPath := filepath.Join(tmpDir, "ensure-key.pem")

	// Files don't exist, should generate
	cert, err := EnsureCertificate(nil, certPath, keyPath)
	require.NoError(t, err)
	require.NotNil(t, cert)

	// Files should now exist
	_, err = os.Stat(certPath)
	assert.NoError(t, err)
	_, err = os.Stat(keyPath)
	assert.NoError(t, err)
}

func TestEnsureCertificate_Load(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "ensure-cert.pem")
	keyPath := filepath.Join(tmpDir, "ensure-key.pem")

	// Create certificate first
	original, err := GenerateAndSave(nil, certPath, keyPath)
	require.NoError(t, err)

	// Should load existing certificate
	loaded, err := EnsureCertificate(nil, certPath, keyPath)
	require.NoError(t, err)

	// Should be same certificate
	assert.Equal(t, original.Certificate.SerialNumber, loaded.Certificate.SerialNumber)
}

func TestEnsureCertificate_OnlyCertExists(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Create only cert file
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)
	err = os.WriteFile(certPath, cert.CertPEM, 0644)
	require.NoError(t, err)

	// Should generate new since key is missing
	loaded, err := EnsureCertificate(nil, certPath, keyPath)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	// Serial should be different (new cert)
	assert.NotEqual(t, cert.Certificate.SerialNumber, loaded.Certificate.SerialNumber)
}

func TestVerifyKeyPairAfterLoad(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "verify-cert.pem")
	keyPath := filepath.Join(tmpDir, "verify-key.pem")

	// Generate and save
	original, err := GenerateAndSave(nil, certPath, keyPath)
	require.NoError(t, err)

	// Verify original pair
	err = VerifyKeyPair(original.Certificate, original.PrivateKey)
	require.NoError(t, err)

	// Load and verify
	loaded, err := LoadCertFromFiles(certPath, keyPath)
	require.NoError(t, err)

	err = VerifyKeyPair(loaded.Certificate, loaded.PrivateKey)
	assert.NoError(t, err)
}
