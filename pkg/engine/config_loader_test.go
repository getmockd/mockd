package engine

import (
	"testing"

	"github.com/getmockd/mockd/pkg/chaos"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/oauth"
	"github.com/getmockd/mockd/pkg/stateful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ============================================================================
// ConfigLoader.LoadFromBytes Tests
// ============================================================================

func TestConfigLoader_LoadFromBytes(t *testing.T) {
	t.Parallel()

	t.Run("valid JSON with one mock registers it", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		cl := NewConfigLoader(srv)

		data := []byte(`{
			"version": "1.0",
			"mocks": [
				{
					"id": "test-1",
					"type": "http",
					"http": {
						"matcher": {"method": "GET", "path": "/test"},
						"response": {"statusCode": 200, "body": "ok"}
					}
				}
			]
		}`)

		err := cl.LoadFromBytes(data, false)
		require.NoError(t, err)

		m := srv.getMock("test-1")
		require.NotNil(t, m, "mock should be registered")
		assert.Equal(t, "test-1", m.ID)
		assert.Equal(t, "/test", m.HTTP.Matcher.Path)
		assert.Equal(t, 200, m.HTTP.Response.StatusCode)
	})

	t.Run("empty JSON object returns no error", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		cl := NewConfigLoader(srv)

		err := cl.LoadFromBytes([]byte(`{"version": "1.0"}`), false)
		require.NoError(t, err)
		assert.Equal(t, 0, srv.Store().Count())
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		cl := NewConfigLoader(srv)

		err := cl.LoadFromBytes([]byte(`{not valid json`), false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse JSON")
	})

	t.Run("mock without required HTTP fields fails validation", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		cl := NewConfigLoader(srv)

		// A mock with type http but no http spec should fail validation
		data := []byte(`{
			"version": "1.0",
			"mocks": [
				{
					"id": "bad-mock",
					"type": "http"
				}
			]
		}`)

		err := cl.LoadFromBytes(data, false)
		require.Error(t, err, "should fail validation for missing HTTP spec")
	})

	t.Run("replace=true clears existing mocks before loading", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		cl := NewConfigLoader(srv)

		// Load first mock
		first := []byte(`{
			"version": "1.0",
			"mocks": [
				{
					"id": "first",
					"type": "http",
					"http": {
						"matcher": {"method": "GET", "path": "/first"},
						"response": {"statusCode": 200, "body": "first"}
					}
				}
			]
		}`)
		require.NoError(t, cl.LoadFromBytes(first, false))
		require.NotNil(t, srv.getMock("first"))

		// Load second mock with replace=true
		second := []byte(`{
			"version": "1.0",
			"mocks": [
				{
					"id": "second",
					"type": "http",
					"http": {
						"matcher": {"method": "GET", "path": "/second"},
						"response": {"statusCode": 200, "body": "second"}
					}
				}
			]
		}`)
		require.NoError(t, cl.LoadFromBytes(second, true))

		assert.Nil(t, srv.getMock("first"), "first mock should be cleared")
		assert.NotNil(t, srv.getMock("second"), "second mock should exist")
		assert.Equal(t, 1, srv.Store().Count())
	})

	t.Run("replace=false preserves existing mocks", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		cl := NewConfigLoader(srv)

		first := []byte(`{
			"version": "1.0",
			"mocks": [
				{
					"id": "existing",
					"type": "http",
					"http": {
						"matcher": {"method": "GET", "path": "/existing"},
						"response": {"statusCode": 200, "body": "existing"}
					}
				}
			]
		}`)
		require.NoError(t, cl.LoadFromBytes(first, false))

		second := []byte(`{
			"version": "1.0",
			"mocks": [
				{
					"id": "new-mock",
					"type": "http",
					"http": {
						"matcher": {"method": "POST", "path": "/new"},
						"response": {"statusCode": 201, "body": "created"}
					}
				}
			]
		}`)
		require.NoError(t, cl.LoadFromBytes(second, false))

		assert.NotNil(t, srv.getMock("existing"), "existing mock should be preserved")
		assert.NotNil(t, srv.getMock("new-mock"), "new mock should be added")
		assert.Equal(t, 2, srv.Store().Count())
	})

	t.Run("multiple mocks loaded from single JSON", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		cl := NewConfigLoader(srv)

		data := []byte(`{
			"version": "1.0",
			"mocks": [
				{
					"id": "multi-1",
					"type": "http",
					"http": {
						"matcher": {"method": "GET", "path": "/a"},
						"response": {"statusCode": 200, "body": "a"}
					}
				},
				{
					"id": "multi-2",
					"type": "http",
					"http": {
						"matcher": {"method": "POST", "path": "/b"},
						"response": {"statusCode": 201, "body": "b"}
					}
				}
			]
		}`)

		require.NoError(t, cl.LoadFromBytes(data, false))
		assert.Equal(t, 2, srv.Store().Count())
		assert.NotNil(t, srv.getMock("multi-1"))
		assert.NotNil(t, srv.getMock("multi-2"))
	})
}

// ============================================================================
// ConfigLoader.Import Tests
// ============================================================================

func TestConfigLoader_Import(t *testing.T) {
	t.Parallel()

	t.Run("merge mode defaults Enabled to true when nil", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		cl := NewConfigLoader(srv)

		collection := &config.MockCollection{
			Version: "1.0",
			Mocks: []*config.MockConfiguration{
				{
					ID:   "import-merge-1",
					Type: mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/merge"},
						Response: &mock.HTTPResponse{StatusCode: 200, Body: "merge"},
					},
					// Enabled deliberately left nil
				},
			},
		}

		require.NoError(t, cl.Import(collection, false))

		m := srv.getMock("import-merge-1")
		require.NotNil(t, m, "imported mock should exist")
		require.NotNil(t, m.Enabled, "Enabled should be set")
		assert.True(t, *m.Enabled, "imported mock should be enabled by default")
	})

	t.Run("replace mode defaults Enabled to true when nil", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		cl := NewConfigLoader(srv)

		collection := &config.MockCollection{
			Version: "1.0",
			Mocks: []*config.MockConfiguration{
				{
					ID:   "import-replace-1",
					Type: mock.TypeHTTP,
					HTTP: &mock.HTTPSpec{
						Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/replace"},
						Response: &mock.HTTPResponse{StatusCode: 200, Body: "replace"},
					},
					// Enabled deliberately left nil
				},
			},
		}

		require.NoError(t, cl.Import(collection, true))

		m := srv.getMock("import-replace-1")
		require.NotNil(t, m, "imported mock should exist")
		require.NotNil(t, m.Enabled, "Enabled should be set")
		assert.True(t, *m.Enabled, "imported mock should be enabled by default")
	})

	t.Run("import preserves explicit Enabled=false", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		cl := NewConfigLoader(srv)

		disabled := false
		collection := &config.MockCollection{
			Version: "1.0",
			Mocks: []*config.MockConfiguration{
				{
					ID:      "import-disabled",
					Type:    mock.TypeHTTP,
					Enabled: &disabled,
					HTTP: &mock.HTTPSpec{
						Matcher:  &mock.HTTPMatcher{Method: "GET", Path: "/disabled"},
						Response: &mock.HTTPResponse{StatusCode: 200, Body: "disabled"},
					},
				},
			},
		}

		require.NoError(t, cl.Import(collection, false))

		m := srv.getMock("import-disabled")
		require.NotNil(t, m, "imported mock should exist")
		require.NotNil(t, m.Enabled, "Enabled should be set")
		assert.False(t, *m.Enabled, "explicitly disabled mock should remain disabled")
	})
}

// ============================================================================
// ConfigLoader.mergeServerConfig Tests
// ============================================================================

func TestMergeServerConfig(t *testing.T) {
	t.Parallel()

	t.Run("merge CORS when dst has none", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.CORS = nil // Simulate CLI not setting CORS
		cl := NewConfigLoader(srv)

		src := &config.ServerConfiguration{
			CORS: &config.CORSConfig{
				Enabled:      true,
				AllowOrigins: []string{"https://example.com"},
			},
		}

		cl.mergeServerConfig(src)

		require.NotNil(t, srv.cfg.CORS)
		assert.True(t, srv.cfg.CORS.Enabled)
		assert.Equal(t, []string{"https://example.com"}, srv.cfg.CORS.AllowOrigins)
	})

	t.Run("CORS not overwritten when dst already has it", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.CORS = &config.CORSConfig{
			Enabled:      true,
			AllowOrigins: []string{"http://localhost:3000"},
		}
		cl := NewConfigLoader(srv)

		src := &config.ServerConfiguration{
			CORS: &config.CORSConfig{
				Enabled:      true,
				AllowOrigins: []string{"https://other.com"},
			},
		}

		cl.mergeServerConfig(src)

		// dst should be preserved, not overwritten
		assert.Equal(t, []string{"http://localhost:3000"}, srv.cfg.CORS.AllowOrigins)
	})

	t.Run("merge RateLimit when dst has none", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.RateLimit = nil
		cl := NewConfigLoader(srv)

		src := &config.ServerConfiguration{
			RateLimit: &config.RateLimitConfig{
				Enabled:           true,
				RequestsPerSecond: 100,
				BurstSize:         200,
			},
		}

		cl.mergeServerConfig(src)

		require.NotNil(t, srv.cfg.RateLimit)
		assert.True(t, srv.cfg.RateLimit.Enabled)
		assert.Equal(t, float64(100), srv.cfg.RateLimit.RequestsPerSecond)
		assert.Equal(t, 200, srv.cfg.RateLimit.BurstSize)
	})

	t.Run("RateLimit not overwritten when dst already has it", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.RateLimit = &config.RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 50,
			BurstSize:         100,
		}
		cl := NewConfigLoader(srv)

		src := &config.ServerConfiguration{
			RateLimit: &config.RateLimitConfig{
				Enabled:           true,
				RequestsPerSecond: 999,
				BurstSize:         9999,
			},
		}

		cl.mergeServerConfig(src)

		assert.Equal(t, float64(50), srv.cfg.RateLimit.RequestsPerSecond)
		assert.Equal(t, 100, srv.cfg.RateLimit.BurstSize)
	})

	t.Run("merge TLS when dst has none", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.TLS = nil
		cl := NewConfigLoader(srv)

		src := &config.ServerConfiguration{
			TLS: &config.TLSConfig{
				Enabled:          true,
				AutoGenerateCert: true,
			},
		}

		cl.mergeServerConfig(src)

		require.NotNil(t, srv.cfg.TLS)
		assert.True(t, srv.cfg.TLS.Enabled)
		assert.True(t, srv.cfg.TLS.AutoGenerateCert)
	})

	t.Run("TLS not overwritten when dst already has it", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.TLS = &config.TLSConfig{
			Enabled:  true,
			CertFile: "/path/to/cert.pem",
			KeyFile:  "/path/to/key.pem",
		}
		cl := NewConfigLoader(srv)

		src := &config.ServerConfiguration{
			TLS: &config.TLSConfig{
				Enabled:          true,
				AutoGenerateCert: true,
			},
		}

		cl.mergeServerConfig(src)

		assert.Equal(t, "/path/to/cert.pem", srv.cfg.TLS.CertFile)
		assert.Equal(t, "/path/to/key.pem", srv.cfg.TLS.KeyFile)
		assert.False(t, srv.cfg.TLS.AutoGenerateCert)
	})

	t.Run("merge MTLS when dst has none", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.MTLS = nil
		cl := NewConfigLoader(srv)

		src := &config.ServerConfiguration{
			MTLS: &config.MTLSConfig{
				Enabled:    true,
				ClientAuth: "require-and-verify",
			},
		}

		cl.mergeServerConfig(src)

		require.NotNil(t, srv.cfg.MTLS)
		assert.True(t, srv.cfg.MTLS.Enabled)
		assert.Equal(t, "require-and-verify", srv.cfg.MTLS.ClientAuth)
	})

	t.Run("merge Chaos when dst has none", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.Chaos = nil
		cl := NewConfigLoader(srv)

		src := &config.ServerConfiguration{
			Chaos: &chaos.ChaosConfig{
				Enabled: true,
				Rules: []chaos.ChaosRule{
					{PathPattern: "/api/.*", Probability: 0.5},
				},
			},
		}

		cl.mergeServerConfig(src)

		require.NotNil(t, srv.cfg.Chaos)
		assert.True(t, srv.cfg.Chaos.Enabled)
		assert.Len(t, srv.cfg.Chaos.Rules, 1)
	})

	t.Run("Chaos not overwritten when dst already has it", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.Chaos = &chaos.ChaosConfig{
			Enabled: false,
		}
		cl := NewConfigLoader(srv)

		src := &config.ServerConfiguration{
			Chaos: &chaos.ChaosConfig{
				Enabled: true,
				Rules: []chaos.ChaosRule{
					{PathPattern: "/api/.*"},
				},
			},
		}

		cl.mergeServerConfig(src)

		// dst preserved
		assert.False(t, srv.cfg.Chaos.Enabled)
		assert.Empty(t, srv.cfg.Chaos.Rules)
	})

	t.Run("merge OAuth when dst has none", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.OAuth = nil
		cl := NewConfigLoader(srv)

		src := &config.ServerConfiguration{
			OAuth: []*oauth.OAuthConfig{
				{Issuer: "https://auth.example.com"},
			},
		}

		cl.mergeServerConfig(src)

		require.Len(t, srv.cfg.OAuth, 1)
		assert.Equal(t, "https://auth.example.com", srv.cfg.OAuth[0].Issuer)
	})

	t.Run("OAuth not overwritten when dst already has entries", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.OAuth = []*oauth.OAuthConfig{
			{Issuer: "https://existing.example.com"},
		}
		cl := NewConfigLoader(srv)

		src := &config.ServerConfiguration{
			OAuth: []*oauth.OAuthConfig{
				{Issuer: "https://other.example.com"},
			},
		}

		cl.mergeServerConfig(src)

		require.Len(t, srv.cfg.OAuth, 1)
		assert.Equal(t, "https://existing.example.com", srv.cfg.OAuth[0].Issuer)
	})

	t.Run("merge multiple fields at once", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.CORS = nil
		srv.cfg.RateLimit = nil
		srv.cfg.TLS = nil
		srv.cfg.MTLS = nil
		srv.cfg.Chaos = nil
		srv.cfg.OAuth = nil
		cl := NewConfigLoader(srv)

		src := &config.ServerConfiguration{
			CORS: &config.CORSConfig{
				Enabled:      true,
				AllowOrigins: []string{"*"},
			},
			RateLimit: &config.RateLimitConfig{
				Enabled:           true,
				RequestsPerSecond: 500,
			},
			TLS: &config.TLSConfig{
				Enabled:          true,
				AutoGenerateCert: true,
			},
		}

		cl.mergeServerConfig(src)

		assert.NotNil(t, srv.cfg.CORS, "CORS should be merged")
		assert.NotNil(t, srv.cfg.RateLimit, "RateLimit should be merged")
		assert.NotNil(t, srv.cfg.TLS, "TLS should be merged")
		// Fields not in src should stay nil
		assert.Nil(t, srv.cfg.MTLS, "MTLS should remain nil")
		assert.Nil(t, srv.cfg.Chaos, "Chaos should remain nil")
		assert.Empty(t, srv.cfg.OAuth, "OAuth should remain empty")
	})

	t.Run("nil source is a no-op", func(t *testing.T) {
		t.Parallel()
		srv := NewServer(nil)
		srv.cfg.CORS = nil
		srv.cfg.RateLimit = nil
		cl := NewConfigLoader(srv)

		// Should not panic
		cl.mergeServerConfig(&config.ServerConfiguration{})

		assert.Nil(t, srv.cfg.CORS)
		assert.Nil(t, srv.cfg.RateLimit)
	})
}

// ============================================================================
// convertCustomOperation Tests
// ============================================================================

func TestConvertCustomOperation(t *testing.T) {
	t.Parallel()

	t.Run("valid config with steps", func(t *testing.T) {
		t.Parallel()
		cfg := &config.CustomOperationConfig{
			Name:        "TransferFunds",
			Consistency: "atomic",
			Steps: []config.CustomStepConfig{
				{
					Type:     "read",
					Resource: "accounts",
					ID:       "input.sourceAccountId",
					As:       "source",
				},
				{
					Type:     "read",
					Resource: "accounts",
					ID:       "input.destAccountId",
					As:       "dest",
				},
				{
					Type:     "update",
					Resource: "accounts",
					ID:       "input.sourceAccountId",
					Set:      map[string]string{"balance": "source.balance - input.amount"},
				},
			},
			Response: map[string]string{
				"sourceBalance": "source.balance - input.amount",
			},
		}

		op, err := convertCustomOperation(cfg)
		require.NoError(t, err)
		require.NotNil(t, op)

		assert.Equal(t, "TransferFunds", op.Name)
		assert.Equal(t, stateful.ConsistencyAtomic, op.Consistency)
		require.Len(t, op.Steps, 3)

		// Verify first step
		assert.Equal(t, stateful.StepRead, op.Steps[0].Type)
		assert.Equal(t, "accounts", op.Steps[0].Resource)
		assert.Equal(t, "input.sourceAccountId", op.Steps[0].ID)
		assert.Equal(t, "source", op.Steps[0].As)

		// Verify third step (update with Set)
		assert.Equal(t, stateful.StepUpdate, op.Steps[2].Type)
		assert.Equal(t, map[string]string{"balance": "source.balance - input.amount"}, op.Steps[2].Set)

		// Verify response
		assert.Equal(t, map[string]string{"sourceBalance": "source.balance - input.amount"}, op.Response)
	})

	t.Run("empty consistency defaults to best_effort after normalization", func(t *testing.T) {
		t.Parallel()
		cfg := &config.CustomOperationConfig{
			Name:        "SimpleOp",
			Consistency: "", // Empty should default
			Steps: []config.CustomStepConfig{
				{
					Type:     "read",
					Resource: "items",
					ID:       "input.id",
					As:       "item",
				},
			},
		}

		op, err := convertCustomOperation(cfg)
		require.NoError(t, err)
		assert.Equal(t, stateful.ConsistencyBestEffort, op.Consistency)
	})

	t.Run("step type mapping preserves all step types", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			cfgType  string
			wantType stateful.StepType
		}{
			{"read", stateful.StepRead},
			{"update", stateful.StepUpdate},
			{"delete", stateful.StepDelete},
			{"create", stateful.StepCreate},
			{"set", stateful.StepSet},
			{"list", stateful.StepList},
			{"validate", stateful.StepValidate},
		}

		for _, tt := range tests {
			t.Run(tt.cfgType, func(t *testing.T) {
				t.Parallel()
				cfg := &config.CustomOperationConfig{
					Name: "op-" + tt.cfgType,
					Steps: []config.CustomStepConfig{
						{Type: tt.cfgType, Resource: "res", ID: "input.id", As: "result"},
					},
				}

				op, err := convertCustomOperation(cfg)
				require.NoError(t, err)
				assert.Equal(t, tt.wantType, op.Steps[0].Type)
			})
		}
	})

	t.Run("step fields are fully propagated", func(t *testing.T) {
		t.Parallel()
		cfg := &config.CustomOperationConfig{
			Name: "FieldCheck",
			Steps: []config.CustomStepConfig{
				{
					Type:     "read",
					Resource: "accounts",
					ID:       "input.accountId",
					As:       "acct",
				},
				{
					Type:  "set",
					Var:   "total",
					Value: "acct.balance + 100",
				},
				{
					Type:     "list",
					Resource: "transactions",
					As:       "txns",
					Filter:   map[string]string{"accountId": "input.accountId"},
				},
				{
					Type:         "validate",
					Condition:    "acct.balance > 0",
					ErrorMessage: "insufficient funds",
					ErrorStatus:  422,
				},
			},
		}

		op, err := convertCustomOperation(cfg)
		require.NoError(t, err)
		require.Len(t, op.Steps, 4)

		// read step
		assert.Equal(t, "accounts", op.Steps[0].Resource)
		assert.Equal(t, "input.accountId", op.Steps[0].ID)
		assert.Equal(t, "acct", op.Steps[0].As)

		// set step
		assert.Equal(t, "total", op.Steps[1].Var)
		assert.Equal(t, "acct.balance + 100", op.Steps[1].Value)

		// list step
		assert.Equal(t, "transactions", op.Steps[2].Resource)
		assert.Equal(t, "txns", op.Steps[2].As)
		assert.Equal(t, map[string]string{"accountId": "input.accountId"}, op.Steps[2].Filter)

		// validate step
		assert.Equal(t, "acct.balance > 0", op.Steps[3].Condition)
		assert.Equal(t, "insufficient funds", op.Steps[3].ErrorMessage)
		assert.Equal(t, 422, op.Steps[3].ErrorStatus)
	})

	t.Run("invalid consistency mode returns error", func(t *testing.T) {
		t.Parallel()
		cfg := &config.CustomOperationConfig{
			Name:        "BadConsistency",
			Consistency: "sometimes",
			Steps: []config.CustomStepConfig{
				{Type: "read", Resource: "items", ID: "input.id", As: "item"},
			},
		}

		_, err := convertCustomOperation(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported consistency mode")
	})

	t.Run("no steps produces valid operation", func(t *testing.T) {
		t.Parallel()
		cfg := &config.CustomOperationConfig{
			Name:  "EmptyOp",
			Steps: nil,
		}

		op, err := convertCustomOperation(cfg)
		require.NoError(t, err)
		assert.Empty(t, op.Steps)
		assert.Equal(t, stateful.ConsistencyBestEffort, op.Consistency)
	})
}
