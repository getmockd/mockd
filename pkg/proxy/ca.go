// Package proxy provides CA certificate generation for HTTPS interception.
package proxy

import (
	"container/list"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// DefaultCAOrganization is the organization name for generated certificates.
	DefaultCAOrganization = "mockd Local CA"
	// DefaultCAValidityDays is the default validity period for CA certificates.
	DefaultCAValidityDays = 3650 // 10 years
	// DefaultKeyBits is the default RSA key size.
	DefaultKeyBits = 2048
	// DefaultCertCacheSize is the default maximum number of certificates to cache.
	DefaultCertCacheSize = 1000
)

// certCacheEntry holds a cached certificate pair with LRU tracking.
type certCacheEntry struct {
	key        string
	pair       *CertPair
	lastAccess time.Time
}

// certLRUCache is a thread-safe LRU cache for certificate pairs.
type certLRUCache struct {
	mu      sync.RWMutex
	items   map[string]*list.Element
	order   *list.List // front = most recently used
	maxSize int
}

// newCertLRUCache creates a new LRU cache with the specified maximum size.
func newCertLRUCache(maxSize int) *certLRUCache {
	if maxSize <= 0 {
		maxSize = DefaultCertCacheSize
	}
	return &certLRUCache{
		items:   make(map[string]*list.Element),
		order:   list.New(),
		maxSize: maxSize,
	}
}

// get retrieves a certificate pair from the cache, updating access time.
func (c *certLRUCache) get(key string) (*CertPair, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, ok := c.items[key]
	if !ok {
		return nil, false
	}

	// Move to front (most recently used)
	c.order.MoveToFront(elem)
	entry, ok := elem.Value.(*certCacheEntry)
	if !ok {
		return nil, false
	}
	entry.lastAccess = time.Now()

	return entry.pair, true
}

// set adds or updates a certificate pair in the cache, evicting oldest if needed.
func (c *certLRUCache) set(key string, pair *CertPair) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		entry, ok := elem.Value.(*certCacheEntry)
		if !ok {
			return
		}
		entry.pair = pair
		entry.lastAccess = time.Now()
		return
	}

	// Evict oldest if at capacity
	if c.order.Len() >= c.maxSize {
		c.evictOldest()
	}

	// Add new entry
	entry := &certCacheEntry{
		key:        key,
		pair:       pair,
		lastAccess: time.Now(),
	}
	elem := c.order.PushFront(entry)
	c.items[key] = elem
}

// evictOldest removes the least recently used entry. Caller must hold the lock.
func (c *certLRUCache) evictOldest() {
	oldest := c.order.Back()
	if oldest == nil {
		return
	}
	entry, ok := oldest.Value.(*certCacheEntry)
	if !ok {
		return
	}
	delete(c.items, entry.key)
	c.order.Remove(oldest)
}

// len returns the current number of cached entries.
func (c *certLRUCache) len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

// CAManager handles CA certificate generation and per-host certificate signing.
type CAManager struct {
	mu sync.RWMutex

	caCert    *x509.Certificate
	caKey     *rsa.PrivateKey
	certPath  string
	keyPath   string
	certCache *certLRUCache
}

// CertPair holds a certificate and its private key.
type CertPair struct {
	Cert *x509.Certificate
	Key  *rsa.PrivateKey
}

// CAManagerOption is a functional option for configuring CAManager.
type CAManagerOption func(*CAManager)

// WithCertCacheSize sets the maximum number of certificates to cache.
func WithCertCacheSize(size int) CAManagerOption {
	return func(m *CAManager) {
		m.certCache = newCertLRUCache(size)
	}
}

// NewCAManager creates a new CA manager with the given paths.
func NewCAManager(certPath, keyPath string, opts ...CAManagerOption) *CAManager {
	m := &CAManager{
		certPath:  certPath,
		keyPath:   keyPath,
		certCache: newCertLRUCache(DefaultCertCacheSize),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// CertPath returns the path to the CA certificate file.
func (m *CAManager) CertPath() string {
	return m.certPath
}

// KeyPath returns the path to the CA private key file.
func (m *CAManager) KeyPath() string {
	return m.keyPath
}

// Exists checks if the CA certificate and key exist on disk.
func (m *CAManager) Exists() bool {
	_, certErr := os.Stat(m.certPath)
	_, keyErr := os.Stat(m.keyPath)
	return certErr == nil && keyErr == nil
}

// Generate creates a new self-signed CA certificate and private key.
func (m *CAManager) Generate() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Generate RSA key pair
	key, err := rsa.GenerateKey(rand.Reader, DefaultKeyBits)
	if err != nil {
		return err
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{DefaultCAOrganization},
			CommonName:   DefaultCAOrganization,
		},
		NotBefore:             now,
		NotAfter:              now.AddDate(0, 0, DefaultCAValidityDays),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
	}

	// Create self-signed certificate
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(m.certPath), 0700); err != nil {
		return err
	}

	// Write certificate PEM
	certOut, err := os.Create(m.certPath)
	if err != nil {
		return err
	}
	defer func() { _ = certOut.Close() }()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER}); err != nil {
		return err
	}

	// Write key PEM
	keyOut, err := os.OpenFile(m.keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() { _ = keyOut.Close() }()

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	if err := pem.Encode(keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return err
	}

	m.caCert = cert
	m.caKey = key

	return nil
}

// Load reads the CA certificate and key from disk.
func (m *CAManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read certificate
	certPEM, err := os.ReadFile(m.certPath)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return os.ErrInvalid
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}

	// Read key
	keyPEM, err := os.ReadFile(m.keyPath)
	if err != nil {
		return err
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return os.ErrInvalid
	}

	key, err := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}

	m.caCert = cert
	m.caKey = key

	return nil
}

// EnsureCA loads existing CA or generates a new one.
func (m *CAManager) EnsureCA() error {
	if m.Exists() {
		return m.Load()
	}
	return m.Generate()
}

// GenerateHostCert creates a certificate for a specific host signed by the CA.
func (m *CAManager) GenerateHostCert(host string) (*CertPair, error) {
	// Check cache first (LRU cache is thread-safe)
	if pair, ok := m.certCache.get(host); ok {
		return pair, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check after acquiring lock
	if pair, ok := m.certCache.get(host); ok {
		return pair, nil
	}

	if m.caCert == nil || m.caKey == nil {
		return nil, os.ErrInvalid
	}

	// Generate key for host
	key, err := rsa.GenerateKey(rand.Reader, DefaultKeyBits)
	if err != nil {
		return nil, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName: host,
		},
		NotBefore:   now,
		NotAfter:    now.AddDate(0, 0, 365), // 1 year
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{host},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, m.caCert, &key.PublicKey, m.caKey)
	if err != nil {
		return nil, err
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, err
	}

	pair := &CertPair{
		Cert: cert,
		Key:  key,
	}

	m.certCache.set(host, pair)
	return pair, nil
}

// CACertPEM returns the CA certificate in PEM format.
func (m *CAManager) CACertPEM() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.caCert == nil {
		return nil, os.ErrInvalid
	}

	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: m.caCert.Raw,
	}), nil
}

// CertInfo holds certificate information.
type CertInfo struct {
	Fingerprint  string
	NotAfter     time.Time
	Organization string
}

// CertInfo returns information about the CA certificate.
func (m *CAManager) CertInfo() (*CertInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.caCert == nil {
		return nil, os.ErrInvalid
	}

	org := ""
	if len(m.caCert.Subject.Organization) > 0 {
		org = m.caCert.Subject.Organization[0]
	}

	// Create a simple fingerprint (first 20 hex chars of the raw cert)
	fingerprint := ""
	if len(m.caCert.Raw) >= 10 {
		for i := 0; i < 10; i++ {
			fingerprint += string("0123456789abcdef"[m.caCert.Raw[i]>>4])
			fingerprint += string("0123456789abcdef"[m.caCert.Raw[i]&0x0f])
			if i < 9 {
				fingerprint += ":"
			}
		}
	}

	return &CertInfo{
		Fingerprint:  fingerprint,
		NotAfter:     m.caCert.NotAfter,
		Organization: org,
	}, nil
}
