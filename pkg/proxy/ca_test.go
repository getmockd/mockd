package proxy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCertLRUCache(t *testing.T) {
	cache := newCertLRUCache(3)

	// Test basic set and get
	pair1 := &CertPair{}
	cache.set("host1", pair1)
	got, ok := cache.get("host1")
	if !ok || got != pair1 {
		t.Error("expected to get host1")
	}

	// Test LRU eviction
	cache.set("host2", &CertPair{})
	cache.set("host3", &CertPair{})
	cache.set("host4", &CertPair{}) // should evict host1

	if _, ok := cache.get("host1"); ok {
		t.Error("host1 should have been evicted")
	}
	if _, ok := cache.get("host2"); !ok {
		t.Error("host2 should still exist")
	}

	// Test access updates LRU order
	cache.get("host2") // access host2, making it most recent
	cache.set("host5", &CertPair{}) // should evict host3 (not host2)

	if _, ok := cache.get("host3"); ok {
		t.Error("host3 should have been evicted")
	}
	if _, ok := cache.get("host2"); !ok {
		t.Error("host2 should still exist after being accessed")
	}
}

func TestCertLRUCacheLen(t *testing.T) {
	cache := newCertLRUCache(5)
	
	if cache.len() != 0 {
		t.Errorf("expected len 0, got %d", cache.len())
	}

	cache.set("host1", &CertPair{})
	cache.set("host2", &CertPair{})
	
	if cache.len() != 2 {
		t.Errorf("expected len 2, got %d", cache.len())
	}
}

func TestCertLRUCacheUpdate(t *testing.T) {
	cache := newCertLRUCache(3)
	
	pair1 := &CertPair{}
	pair2 := &CertPair{}
	
	cache.set("host1", pair1)
	cache.set("host1", pair2) // update existing key
	
	if cache.len() != 1 {
		t.Errorf("expected len 1 after update, got %d", cache.len())
	}
	
	got, ok := cache.get("host1")
	if !ok || got != pair2 {
		t.Error("expected to get updated pair")
	}
}

func TestCAManagerWithCacheSize(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "ca.crt")
	keyPath := filepath.Join(tmpDir, "ca.key")

	// Test default cache size
	ca := NewCAManager(certPath, keyPath)
	if ca.certCache.maxSize != DefaultCertCacheSize {
		t.Errorf("expected default cache size %d, got %d", DefaultCertCacheSize, ca.certCache.maxSize)
	}

	// Test custom cache size
	ca2 := NewCAManager(certPath, keyPath, WithCertCacheSize(100))
	if ca2.certCache.maxSize != 100 {
		t.Errorf("expected custom cache size 100, got %d", ca2.certCache.maxSize)
	}
}

func TestCAManagerGenerateHostCert(t *testing.T) {
	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "ca.crt")
	keyPath := filepath.Join(tmpDir, "ca.key")

	ca := NewCAManager(certPath, keyPath, WithCertCacheSize(3))
	if err := ca.EnsureCA(); err != nil {
		t.Fatalf("failed to ensure CA: %v", err)
	}

	// Generate certificates for multiple hosts
	hosts := []string{"example.com", "test.com", "api.example.com", "web.example.com"}
	for _, host := range hosts {
		pair, err := ca.GenerateHostCert(host)
		if err != nil {
			t.Fatalf("failed to generate cert for %s: %v", host, err)
		}
		if pair.Cert.Subject.CommonName != host {
			t.Errorf("expected CN %s, got %s", host, pair.Cert.Subject.CommonName)
		}
	}

	// First host should have been evicted (cache size is 3)
	if ca.certCache.len() != 3 {
		t.Errorf("expected cache len 3, got %d", ca.certCache.len())
	}

	// Verify cache hit returns same cert
	pair1, _ := ca.GenerateHostCert("web.example.com")
	pair2, _ := ca.GenerateHostCert("web.example.com")
	if pair1 != pair2 {
		t.Error("expected cache hit to return same cert pair")
	}

	// Clean up
	os.Remove(certPath)
	os.Remove(keyPath)
}
