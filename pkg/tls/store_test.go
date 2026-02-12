package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

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

// --- Expiry check tests ---

// generateExpiredCert creates a cert that expired at the given time,
// saving it to certPath/keyPath. Returns the serial number for comparison.
func generateExpiredCert(t *testing.T, notAfter time.Time, certPath, keyPath string) *big.Int {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "expired-test"},
		NotBefore:             notAfter.Add(-365 * 24 * time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	require.NoError(t, os.MkdirAll(filepath.Dir(certPath), 0755))
	require.NoError(t, os.WriteFile(certPath, certPEM, 0644))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))

	return serial
}

func TestIsCertExpired_Valid(t *testing.T) {
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)

	assert.False(t, IsCertExpired(cert.Certificate, 0))
	assert.False(t, IsCertExpired(cert.Certificate, 24*time.Hour))
}

func TestIsCertExpired_Expired(t *testing.T) {
	// Create a cert that expired 1 hour ago
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "expired"},
		NotBefore:             time.Now().Add(-48 * time.Hour),
		NotAfter:              time.Now().Add(-1 * time.Hour),
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	assert.True(t, IsCertExpired(cert, 0))
	assert.True(t, IsCertExpired(cert, 24*time.Hour))
}

func TestIsCertExpired_ExpiringSoon(t *testing.T) {
	// Create a cert that expires in 12 hours
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "expiring-soon"},
		NotBefore:             time.Now().Add(-365 * 24 * time.Hour),
		NotAfter:              time.Now().Add(12 * time.Hour),
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	// Not expired with 0 grace
	assert.False(t, IsCertExpired(cert, 0))
	// But expired with 24h grace
	assert.True(t, IsCertExpired(cert, 24*time.Hour))
}

func TestCheckCertExpiry_Valid(t *testing.T) {
	cert, err := GenerateSelfSignedCert(nil)
	require.NoError(t, err)

	assert.NoError(t, CheckCertExpiry(cert.Certificate, 0))
	assert.NoError(t, CheckCertExpiry(cert.Certificate, 24*time.Hour))
}

func TestCheckCertExpiry_Expired(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "expired"},
		NotBefore:             time.Now().Add(-48 * time.Hour),
		NotAfter:              time.Now().Add(-1 * time.Hour),
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	expiryErr := CheckCertExpiry(cert, 0)
	require.Error(t, expiryErr)
	assert.Contains(t, expiryErr.Error(), "certificate expired on")
	assert.Contains(t, expiryErr.Error(), "ago")
}

func TestCheckCertExpiry_ExpiringSoon(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "expiring-soon"},
		NotBefore:             time.Now().Add(-365 * 24 * time.Hour),
		NotAfter:              time.Now().Add(12 * time.Hour),
		BasicConstraintsValid: true,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(certDER)
	require.NoError(t, err)

	// Valid with 0 grace
	assert.NoError(t, CheckCertExpiry(cert, 0))
	// Warns with 24h grace
	expiryErr := CheckCertExpiry(cert, 24*time.Hour)
	require.Error(t, expiryErr)
	assert.Contains(t, expiryErr.Error(), "expires on")
	assert.Contains(t, expiryErr.Error(), "grace period")
}

func TestEnsureCertificate_RegeneratesExpired(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Create a cert that expired 1 hour ago
	expiredSerial := generateExpiredCert(t, time.Now().Add(-1*time.Hour), certPath, keyPath)

	// EnsureCertificate should detect the expired cert and regenerate
	cert, err := EnsureCertificate(nil, certPath, keyPath)
	require.NoError(t, err)
	require.NotNil(t, cert)

	// New cert should have a different serial
	assert.NotEqual(t, expiredSerial, cert.Certificate.SerialNumber)
	// New cert should not be expired
	assert.False(t, IsCertExpired(cert.Certificate, CertExpiryGracePeriod))
}

func TestEnsureCertificate_RegeneratesExpiringSoon(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Create a cert that expires in 12h (within the 24h grace period)
	expiringSerial := generateExpiredCert(t, time.Now().Add(12*time.Hour), certPath, keyPath)

	// EnsureCertificate should detect "expiring soon" and regenerate
	cert, err := EnsureCertificate(nil, certPath, keyPath)
	require.NoError(t, err)
	require.NotNil(t, cert)

	// Should be a fresh cert
	assert.NotEqual(t, expiringSerial, cert.Certificate.SerialNumber)
	assert.False(t, IsCertExpired(cert.Certificate, CertExpiryGracePeriod))
}

func TestEnsureCertificate_KeepsValidCert(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Generate a fresh valid cert
	original, err := GenerateAndSave(nil, certPath, keyPath)
	require.NoError(t, err)

	// EnsureCertificate should load (not regenerate)
	loaded, err := EnsureCertificate(nil, certPath, keyPath)
	require.NoError(t, err)

	// Same serial = same cert
	assert.Equal(t, original.Certificate.SerialNumber, loaded.Certificate.SerialNumber)
}

func TestEnsureCertificate_RegeneratesCorruptFiles(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")

	// Write garbage to both files
	require.NoError(t, os.WriteFile(certPath, []byte("not a cert"), 0644))
	require.NoError(t, os.WriteFile(keyPath, []byte("not a key"), 0600))

	// EnsureCertificate should detect the corrupt files and regenerate
	cert, err := EnsureCertificate(nil, certPath, keyPath)
	require.NoError(t, err)
	require.NotNil(t, cert)

	// New cert should be valid
	assert.False(t, IsCertExpired(cert.Certificate, CertExpiryGracePeriod))
	assert.Equal(t, "localhost", cert.Certificate.Subject.CommonName)
}
