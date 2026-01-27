// Package engine provides the core mock server engine.
package engine

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/getmockd/mockd/pkg/config"
	mockdtls "github.com/getmockd/mockd/pkg/tls"
)

// TLSManager handles TLS and mTLS configuration for the mock server.
type TLSManager struct {
	cfg     *config.TLSConfig
	mtlsCfg *config.MTLSConfig
}

// NewTLSManager creates a new TLSManager.
func NewTLSManager(cfg *config.TLSConfig, mtlsCfg *config.MTLSConfig) *TLSManager {
	return &TLSManager{
		cfg:     cfg,
		mtlsCfg: mtlsCfg,
	}
}

// NewTLSManagerFromServerConfig creates a TLSManager from server configuration.
func NewTLSManagerFromServerConfig(cfg *config.ServerConfiguration) *TLSManager {
	return &TLSManager{
		cfg:     cfg.TLS,
		mtlsCfg: cfg.MTLS,
	}
}

// BuildConfig builds and returns the TLS configuration.
// Returns nil if TLS is not enabled.
func (tm *TLSManager) BuildConfig() (*tls.Config, error) {
	if tm.cfg == nil || !tm.cfg.Enabled {
		return nil, nil
	}

	var tlsCert tls.Certificate
	var err error

	// If auto-generate is enabled, generate a self-signed certificate
	if tm.cfg.AutoGenerateCert {
		genCert, genErr := mockdtls.GenerateSelfSignedCert(mockdtls.DefaultCertificateConfig())
		if genErr != nil {
			return nil, fmt.Errorf("failed to generate certificate: %w", genErr)
		}

		tlsCert, err = mockdtls.CreateTLSCertificate(genCert.CertPEM, genCert.KeyPEM)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS certificate: %w", err)
		}
	} else {
		// Load certificate from files
		tlsCert, err = tls.LoadX509KeyPair(tm.cfg.CertFile, tm.cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load certificate: %w", err)
		}
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		MinVersion:   tls.VersionTLS12,
	}

	// Configure mTLS if enabled
	if tm.mtlsCfg != nil && tm.mtlsCfg.Enabled {
		if err := tm.configureMTLS(tlsConfig); err != nil {
			return nil, fmt.Errorf("mTLS configuration failed: %w", err)
		}
	}

	return tlsConfig, nil
}

// configureMTLS configures mutual TLS client certificate authentication.
func (tm *TLSManager) configureMTLS(tlsConfig *tls.Config) error {
	mtlsCfg := tm.mtlsCfg

	// Parse ClientAuth mode
	switch mtlsCfg.ClientAuth {
	case "none", "":
		tlsConfig.ClientAuth = tls.NoClientCert
	case "request":
		tlsConfig.ClientAuth = tls.RequestClientCert
	case "require":
		tlsConfig.ClientAuth = tls.RequireAnyClientCert
	case "verify-if-given":
		tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
	case "require-and-verify":
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	default:
		return fmt.Errorf("invalid clientAuth mode: %s", mtlsCfg.ClientAuth)
	}

	// Load CA certificates for client verification
	certPool := x509.NewCertPool()
	certsLoaded := false

	// Load from single CACertFile if specified
	if mtlsCfg.CACertFile != "" {
		caCert, err := os.ReadFile(mtlsCfg.CACertFile)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate file %s: %w", mtlsCfg.CACertFile, err)
		}
		if !certPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to parse CA certificate from %s", mtlsCfg.CACertFile)
		}
		certsLoaded = true
	}

	// Load from multiple CACertFiles if specified
	for _, caFile := range mtlsCfg.CACertFiles {
		caCert, err := os.ReadFile(caFile)
		if err != nil {
			return fmt.Errorf("failed to read CA certificate file %s: %w", caFile, err)
		}
		if !certPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("failed to parse CA certificate from %s", caFile)
		}
		certsLoaded = true
	}

	// Only set ClientCAs if we loaded certificates
	if certsLoaded {
		tlsConfig.ClientCAs = certPool
	}

	// Configure CN/OU filtering if specified
	if len(mtlsCfg.AllowedCNs) > 0 || len(mtlsCfg.AllowedOUs) > 0 {
		// Create lookup maps for O(1) checking
		allowedCNs := make(map[string]struct{})
		for _, cn := range mtlsCfg.AllowedCNs {
			allowedCNs[cn] = struct{}{}
		}

		allowedOUs := make(map[string]struct{})
		for _, ou := range mtlsCfg.AllowedOUs {
			allowedOUs[ou] = struct{}{}
		}

		tlsConfig.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			// If no verified chains yet, let standard TLS verification handle it
			if len(verifiedChains) == 0 || len(verifiedChains[0]) == 0 {
				return nil
			}

			// Get the client certificate (first cert in first verified chain)
			clientCert := verifiedChains[0][0]

			// Check Common Name if AllowedCNs is configured
			if len(allowedCNs) > 0 {
				if _, ok := allowedCNs[clientCert.Subject.CommonName]; !ok {
					return fmt.Errorf("client certificate CN %q not in allowed list", clientCert.Subject.CommonName)
				}
			}

			// Check Organizational Units if AllowedOUs is configured
			if len(allowedOUs) > 0 {
				found := false
				for _, ou := range clientCert.Subject.OrganizationalUnit {
					if _, ok := allowedOUs[ou]; ok {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("client certificate OUs %v not in allowed list", clientCert.Subject.OrganizationalUnit)
				}
			}

			return nil
		}
	}

	return nil
}
