package engine

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/config"
	mockdtls "github.com/getmockd/mockd/pkg/tls"
)

// generateTestCert is a test helper that generates a self-signed cert and
// writes it to temp files, returning the cert/key file paths.
func generateTestCert(t *testing.T) (certPath, keyPath string) {
	t.Helper()

	cert, err := mockdtls.GenerateSelfSignedCert(mockdtls.DefaultCertificateConfig())
	require.NoError(t, err)

	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")

	require.NoError(t, os.WriteFile(certPath, cert.CertPEM, 0644))
	require.NoError(t, os.WriteFile(keyPath, cert.KeyPEM, 0600))

	return certPath, keyPath
}

// generateTestCACert generates a CA cert and writes it to a temp file,
// returning the file path and the parsed certificate.
func generateTestCACert(t *testing.T) (caPath string, caCert *x509.Certificate) {
	t.Helper()

	cfg := &mockdtls.CertificateConfig{
		Organization: "Test CA",
		CommonName:   "Test CA",
		IsCA:         true,
		ValidFor:     365 * 24 * 60 * 60 * 1e9, // 1 year in nanoseconds as Duration
	}
	gen, err := mockdtls.GenerateSelfSignedCert(cfg)
	require.NoError(t, err)

	dir := t.TempDir()
	caPath = filepath.Join(dir, "ca.pem")
	require.NoError(t, os.WriteFile(caPath, gen.CertPEM, 0644))

	return caPath, gen.Certificate
}

func TestTLSManager_NewFromServerConfig(t *testing.T) {
	t.Parallel()

	t.Run("extracts TLS and MTLS config", func(t *testing.T) {
		t.Parallel()

		tlsCfg := &config.TLSConfig{Enabled: true, AutoGenerateCert: true}
		mtlsCfg := &config.MTLSConfig{Enabled: true, ClientAuth: "require"}

		tm := NewTLSManagerFromServerConfig(&config.ServerConfiguration{
			TLS:  tlsCfg,
			MTLS: mtlsCfg,
		})

		assert.Equal(t, tlsCfg, tm.cfg)
		assert.Equal(t, mtlsCfg, tm.mtlsCfg)
	})

	t.Run("nil TLS and MTLS in server config", func(t *testing.T) {
		t.Parallel()

		tm := NewTLSManagerFromServerConfig(&config.ServerConfiguration{})

		assert.Nil(t, tm.cfg)
		assert.Nil(t, tm.mtlsCfg)
	})
}

func TestTLSManager_BuildConfig(t *testing.T) {
	t.Parallel()

	t.Run("nil TLS config returns nil nil", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{cfg: nil}
		got, err := tm.BuildConfig()

		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("disabled TLS returns nil nil", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{cfg: &config.TLSConfig{Enabled: false}}
		got, err := tm.BuildConfig()

		assert.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("auto-generate cert produces valid tls.Config", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			cfg: &config.TLSConfig{Enabled: true, AutoGenerateCert: true},
		}

		got, err := tm.BuildConfig()
		require.NoError(t, err)
		require.NotNil(t, got)

		assert.Len(t, got.Certificates, 1)
		assert.Equal(t, uint16(tls.VersionTLS12), got.MinVersion)

		// The certificate should be parseable.
		leaf, parseErr := x509.ParseCertificate(got.Certificates[0].Certificate[0])
		require.NoError(t, parseErr)
		assert.Equal(t, "localhost", leaf.Subject.CommonName)
	})

	t.Run("cert/key files loads from disk", func(t *testing.T) {
		t.Parallel()

		certPath, keyPath := generateTestCert(t)

		tm := &TLSManager{
			cfg: &config.TLSConfig{
				Enabled:  true,
				CertFile: certPath,
				KeyFile:  keyPath,
			},
		}

		got, err := tm.BuildConfig()
		require.NoError(t, err)
		require.NotNil(t, got)

		assert.Len(t, got.Certificates, 1)
		assert.Equal(t, uint16(tls.VersionTLS12), got.MinVersion)
	})

	t.Run("missing cert file returns error", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			cfg: &config.TLSConfig{
				Enabled:  true,
				CertFile: "/nonexistent/cert.pem",
				KeyFile:  "/nonexistent/key.pem",
			},
		}

		got, err := tm.BuildConfig()
		assert.Error(t, err)
		assert.Nil(t, got)
		assert.Contains(t, err.Error(), "failed to load certificate")
	})

	t.Run("TLS with mTLS enabled configures client auth", func(t *testing.T) {
		t.Parallel()

		caPath, _ := generateTestCACert(t)

		tm := &TLSManager{
			cfg: &config.TLSConfig{Enabled: true, AutoGenerateCert: true},
			mtlsCfg: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "require",
				CACertFile: caPath,
			},
		}

		got, err := tm.BuildConfig()
		require.NoError(t, err)
		require.NotNil(t, got)

		assert.Equal(t, tls.RequireAnyClientCert, got.ClientAuth)
		assert.NotNil(t, got.ClientCAs)
	})

	t.Run("mTLS disabled is ignored", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			cfg: &config.TLSConfig{Enabled: true, AutoGenerateCert: true},
			mtlsCfg: &config.MTLSConfig{
				Enabled:    false,
				ClientAuth: "require",
			},
		}

		got, err := tm.BuildConfig()
		require.NoError(t, err)
		require.NotNil(t, got)

		// ClientAuth should be default (NoClientCert) since mTLS is disabled.
		assert.Equal(t, tls.NoClientCert, got.ClientAuth)
	})

	t.Run("nil mTLS config is ignored", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			cfg:     &config.TLSConfig{Enabled: true, AutoGenerateCert: true},
			mtlsCfg: nil,
		}

		got, err := tm.BuildConfig()
		require.NoError(t, err)
		require.NotNil(t, got)

		assert.Equal(t, tls.NoClientCert, got.ClientAuth)
	})
}

func TestTLSManager_ConfigureMTLS_ClientAuthModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		clientAuth string
		want       tls.ClientAuthType
	}{
		{name: "none", clientAuth: "none", want: tls.NoClientCert},
		{name: "empty string", clientAuth: "", want: tls.NoClientCert},
		{name: "request", clientAuth: "request", want: tls.RequestClientCert},
		{name: "require", clientAuth: "require", want: tls.RequireAnyClientCert},
		{name: "verify-if-given", clientAuth: "verify-if-given", want: tls.VerifyClientCertIfGiven},
		{name: "require-and-verify", clientAuth: "require-and-verify", want: tls.RequireAndVerifyClientCert},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tm := &TLSManager{
				mtlsCfg: &config.MTLSConfig{
					Enabled:    true,
					ClientAuth: tt.clientAuth,
				},
			}

			tlsConfig := &tls.Config{}
			err := tm.configureMTLS(tlsConfig)
			require.NoError(t, err)
			assert.Equal(t, tt.want, tlsConfig.ClientAuth)
		})
	}
}

func TestTLSManager_ConfigureMTLS_InvalidClientAuth(t *testing.T) {
	t.Parallel()

	tm := &TLSManager{
		mtlsCfg: &config.MTLSConfig{
			Enabled:    true,
			ClientAuth: "bogus-mode",
		},
	}

	tlsConfig := &tls.Config{}
	err := tm.configureMTLS(tlsConfig)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid clientAuth mode")
	assert.Contains(t, err.Error(), "bogus-mode")
}

func TestTLSManager_ConfigureMTLS_CACertFile(t *testing.T) {
	t.Parallel()

	t.Run("loads single CA cert file", func(t *testing.T) {
		t.Parallel()

		caPath, _ := generateTestCACert(t)

		tm := &TLSManager{
			mtlsCfg: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "require-and-verify",
				CACertFile: caPath,
			},
		}

		tlsConfig := &tls.Config{}
		err := tm.configureMTLS(tlsConfig)
		require.NoError(t, err)
		assert.NotNil(t, tlsConfig.ClientCAs)
	})

	t.Run("loads multiple CA cert files", func(t *testing.T) {
		t.Parallel()

		caPath1, _ := generateTestCACert(t)
		caPath2, _ := generateTestCACert(t)

		tm := &TLSManager{
			mtlsCfg: &config.MTLSConfig{
				Enabled:     true,
				ClientAuth:  "require-and-verify",
				CACertFiles: []string{caPath1, caPath2},
			},
		}

		tlsConfig := &tls.Config{}
		err := tm.configureMTLS(tlsConfig)
		require.NoError(t, err)
		assert.NotNil(t, tlsConfig.ClientCAs)
	})

	t.Run("missing CA cert file returns error", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			mtlsCfg: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "require",
				CACertFile: "/nonexistent/ca.pem",
			},
		}

		tlsConfig := &tls.Config{}
		err := tm.configureMTLS(tlsConfig)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read CA certificate file")
	})

	t.Run("invalid CA cert content returns error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		badCA := filepath.Join(dir, "bad-ca.pem")
		require.NoError(t, os.WriteFile(badCA, []byte("not a certificate"), 0644))

		tm := &TLSManager{
			mtlsCfg: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "require",
				CACertFile: badCA,
			},
		}

		tlsConfig := &tls.Config{}
		err := tm.configureMTLS(tlsConfig)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse CA certificate")
	})

	t.Run("no CA cert files leaves ClientCAs nil", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			mtlsCfg: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "request",
			},
		}

		tlsConfig := &tls.Config{}
		err := tm.configureMTLS(tlsConfig)
		require.NoError(t, err)
		assert.Nil(t, tlsConfig.ClientCAs)
	})
}

func TestTLSManager_ConfigureMTLS_CNFiltering(t *testing.T) {
	t.Parallel()

	t.Run("allowed CN passes", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			mtlsCfg: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "require",
				AllowedCNs: []string{"allowed-client"},
			},
		}

		tlsConfig := &tls.Config{}
		err := tm.configureMTLS(tlsConfig)
		require.NoError(t, err)
		require.NotNil(t, tlsConfig.VerifyConnection)

		// Simulate a connection with allowed CN.
		cs := tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{
				{Subject: pkix.Name{CommonName: "allowed-client"}},
			},
		}
		assert.NoError(t, tlsConfig.VerifyConnection(cs))
	})

	t.Run("disallowed CN rejected", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			mtlsCfg: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "require",
				AllowedCNs: []string{"allowed-client"},
			},
		}

		tlsConfig := &tls.Config{}
		err := tm.configureMTLS(tlsConfig)
		require.NoError(t, err)
		require.NotNil(t, tlsConfig.VerifyConnection)

		cs := tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{
				{Subject: pkix.Name{CommonName: "evil-client"}},
			},
		}
		verifyErr := tlsConfig.VerifyConnection(cs)
		require.Error(t, verifyErr)
		assert.Contains(t, verifyErr.Error(), "not in allowed list")
	})

	t.Run("no peer certs passes (let ClientAuth decide)", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			mtlsCfg: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "request",
				AllowedCNs: []string{"allowed-client"},
			},
		}

		tlsConfig := &tls.Config{}
		err := tm.configureMTLS(tlsConfig)
		require.NoError(t, err)
		require.NotNil(t, tlsConfig.VerifyConnection)

		cs := tls.ConnectionState{PeerCertificates: nil}
		assert.NoError(t, tlsConfig.VerifyConnection(cs))
	})
}

func TestTLSManager_ConfigureMTLS_OUFiltering(t *testing.T) {
	t.Parallel()

	t.Run("allowed OU passes", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			mtlsCfg: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "require",
				AllowedOUs: []string{"Engineering"},
			},
		}

		tlsConfig := &tls.Config{}
		err := tm.configureMTLS(tlsConfig)
		require.NoError(t, err)
		require.NotNil(t, tlsConfig.VerifyConnection)

		cs := tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{
				{Subject: pkix.Name{OrganizationalUnit: []string{"Engineering"}}},
			},
		}
		assert.NoError(t, tlsConfig.VerifyConnection(cs))
	})

	t.Run("disallowed OU rejected", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			mtlsCfg: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "require",
				AllowedOUs: []string{"Engineering"},
			},
		}

		tlsConfig := &tls.Config{}
		err := tm.configureMTLS(tlsConfig)
		require.NoError(t, err)
		require.NotNil(t, tlsConfig.VerifyConnection)

		cs := tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{
				{Subject: pkix.Name{OrganizationalUnit: []string{"Marketing"}}},
			},
		}
		verifyErr := tlsConfig.VerifyConnection(cs)
		require.Error(t, verifyErr)
		assert.Contains(t, verifyErr.Error(), "not in allowed list")
	})

	t.Run("one matching OU among several passes", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			mtlsCfg: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "require",
				AllowedOUs: []string{"Engineering"},
			},
		}

		tlsConfig := &tls.Config{}
		err := tm.configureMTLS(tlsConfig)
		require.NoError(t, err)

		cs := tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{
				{Subject: pkix.Name{OrganizationalUnit: []string{"Sales", "Engineering"}}},
			},
		}
		assert.NoError(t, tlsConfig.VerifyConnection(cs))
	})
}

func TestTLSManager_ConfigureMTLS_CNAndOUFiltering(t *testing.T) {
	t.Parallel()

	t.Run("both CN and OU must match", func(t *testing.T) {
		t.Parallel()

		tm := &TLSManager{
			mtlsCfg: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "require",
				AllowedCNs: []string{"good-client"},
				AllowedOUs: []string{"Engineering"},
			},
		}

		tlsConfig := &tls.Config{}
		err := tm.configureMTLS(tlsConfig)
		require.NoError(t, err)
		require.NotNil(t, tlsConfig.VerifyConnection)

		// Good CN + Good OU → pass
		cs := tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{
				{Subject: pkix.Name{
					CommonName:         "good-client",
					OrganizationalUnit: []string{"Engineering"},
				}},
			},
		}
		assert.NoError(t, tlsConfig.VerifyConnection(cs))

		// Good CN + Bad OU → fail
		cs.PeerCertificates[0].Subject.OrganizationalUnit = []string{"Marketing"}
		assert.Error(t, tlsConfig.VerifyConnection(cs))

		// Bad CN + Good OU → fail
		cs.PeerCertificates[0].Subject.CommonName = "bad-client"
		cs.PeerCertificates[0].Subject.OrganizationalUnit = []string{"Engineering"}
		assert.Error(t, tlsConfig.VerifyConnection(cs))
	})
}

func TestTLSManager_ConfigureMTLS_NoFilters(t *testing.T) {
	t.Parallel()

	tm := &TLSManager{
		mtlsCfg: &config.MTLSConfig{
			Enabled:    true,
			ClientAuth: "require",
			// No AllowedCNs or AllowedOUs
		},
	}

	tlsConfig := &tls.Config{}
	err := tm.configureMTLS(tlsConfig)
	require.NoError(t, err)

	// No VerifyConnection callback when no CN/OU filters configured.
	assert.Nil(t, tlsConfig.VerifyConnection)
}

func TestTLSManager_FullRoundTrip(t *testing.T) {
	t.Parallel()

	caPath, _ := generateTestCACert(t)
	certPath, keyPath := generateTestCert(t)

	tm := NewTLSManagerFromServerConfig(&config.ServerConfiguration{
		TLS: &config.TLSConfig{
			Enabled:  true,
			CertFile: certPath,
			KeyFile:  keyPath,
		},
		MTLS: &config.MTLSConfig{
			Enabled:    true,
			ClientAuth: "require-and-verify",
			CACertFile: caPath,
			AllowedCNs: []string{"my-service"},
			AllowedOUs: []string{"Platform"},
		},
	})

	got, err := tm.BuildConfig()
	require.NoError(t, err)
	require.NotNil(t, got)

	// TLS basics
	assert.Len(t, got.Certificates, 1)
	assert.Equal(t, uint16(tls.VersionTLS12), got.MinVersion)

	// mTLS
	assert.Equal(t, tls.RequireAndVerifyClientCert, got.ClientAuth)
	assert.NotNil(t, got.ClientCAs)
	assert.NotNil(t, got.VerifyConnection)

	// VerifyConnection should accept matching CN+OU
	cs := tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{
			{Subject: pkix.Name{
				CommonName:         "my-service",
				OrganizationalUnit: []string{"Platform"},
			}},
		},
	}
	assert.NoError(t, got.VerifyConnection(cs))

	// VerifyConnection should reject wrong CN
	cs.PeerCertificates[0].Subject.CommonName = "wrong-service"
	assert.Error(t, got.VerifyConnection(cs))
}
