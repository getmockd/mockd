package admin

import (
	"context"
	"testing"
	"time"
)

func TestTokenExpiration(t *testing.T) {
	// Create an API with very short expiration for testing
	api := NewAPI(0,
		WithRegistrationTokenExpiration(100*time.Millisecond),
		WithEngineTokenExpiration(100*time.Millisecond),
	)
	defer api.Stop()

	t.Run("registration token expires", func(t *testing.T) {
		token, err := api.GenerateRegistrationToken()
		if err != nil {
			t.Fatalf("failed to generate registration token: %v", err)
		}

		// Token should be valid immediately
		// Note: ValidateRegistrationToken consumes the token, so we generate a new one
		token2, err := api.GenerateRegistrationToken()
		if err != nil {
			t.Fatalf("failed to generate registration token: %v", err)
		}
		if !api.ValidateRegistrationToken(token2) {
			t.Error("fresh registration token should be valid")
		}

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Token should now be expired
		if api.ValidateRegistrationToken(token) {
			t.Error("expired registration token should not be valid")
		}
	})

	t.Run("engine token expires", func(t *testing.T) {
		engineID := "test-engine-1"
		token, err := api.generateEngineToken(engineID)
		if err != nil {
			t.Fatalf("failed to generate engine token: %v", err)
		}

		// Token should be valid immediately
		if !api.ValidateEngineToken(engineID, token) {
			t.Error("fresh engine token should be valid")
		}

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Token should now be expired
		if api.ValidateEngineToken(engineID, token) {
			t.Error("expired engine token should not be valid")
		}
	})

	t.Run("wrong engine token rejected", func(t *testing.T) {
		engineID := "test-engine-2"
		_, err := api.generateEngineToken(engineID)
		if err != nil {
			t.Fatalf("failed to generate engine token: %v", err)
		}

		if api.ValidateEngineToken(engineID, "wrong-token") {
			t.Error("wrong token should not be valid")
		}

		if api.ValidateEngineToken("wrong-engine", "any-token") {
			t.Error("non-existent engine should not validate")
		}
	})
}

func TestTokenCleanup(t *testing.T) {
	// Create API with short expiration
	api := NewAPI(0,
		WithRegistrationTokenExpiration(50*time.Millisecond),
		WithEngineTokenExpiration(50*time.Millisecond),
	)
	defer api.Stop()

	// Generate some tokens
	_, err := api.GenerateRegistrationToken()
	if err != nil {
		t.Fatalf("failed to generate registration token: %v", err)
	}
	_, err = api.GenerateRegistrationToken()
	if err != nil {
		t.Fatalf("failed to generate registration token: %v", err)
	}
	_, err = api.generateEngineToken("engine-1")
	if err != nil {
		t.Fatalf("failed to generate engine token: %v", err)
	}
	_, err = api.generateEngineToken("engine-2")
	if err != nil {
		t.Fatalf("failed to generate engine token: %v", err)
	}

	// Check initial stats
	stats := api.GetTokenStats()
	if stats.ActiveRegistrationTokens != 2 {
		t.Errorf("expected 2 active registration tokens, got %d", stats.ActiveRegistrationTokens)
	}
	if stats.ActiveEngineTokens != 2 {
		t.Errorf("expected 2 active engine tokens, got %d", stats.ActiveEngineTokens)
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Tokens should now be expired (but not yet cleaned up)
	stats = api.GetTokenStats()
	if stats.ExpiredRegistrationTokens != 2 {
		t.Errorf("expected 2 expired registration tokens, got %d", stats.ExpiredRegistrationTokens)
	}
	if stats.ExpiredEngineTokens != 2 {
		t.Errorf("expected 2 expired engine tokens, got %d", stats.ExpiredEngineTokens)
	}

	// Manually trigger cleanup
	api.cleanupExpiredTokens()

	// All tokens should be cleaned up
	stats = api.GetTokenStats()
	if stats.ActiveRegistrationTokens != 0 || stats.ExpiredRegistrationTokens != 0 {
		t.Errorf("expected 0 registration tokens after cleanup, got active=%d expired=%d",
			stats.ActiveRegistrationTokens, stats.ExpiredRegistrationTokens)
	}
	if stats.ActiveEngineTokens != 0 || stats.ExpiredEngineTokens != 0 {
		t.Errorf("expected 0 engine tokens after cleanup, got active=%d expired=%d",
			stats.ActiveEngineTokens, stats.ExpiredEngineTokens)
	}
}

func TestTokenCleanupGoroutine(t *testing.T) {
	// This test verifies the cleanup goroutine starts and stops properly
	ctx, cancel := context.WithCancel(context.Background())

	api := NewAPI(0)
	api.ctx = ctx
	api.cancel = cancel

	// Generate a token that will expire quickly
	api.registrationTokenExpiration = 10 * time.Millisecond
	_, err := api.GenerateRegistrationToken()
	if err != nil {
		t.Fatalf("failed to generate registration token: %v", err)
	}

	// Start cleanup with a very short interval for testing
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				api.cleanupExpiredTokens()
			}
		}
	}()

	// Wait a bit for cleanup to run
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop the goroutine
	cancel()

	// Wait for goroutine to exit
	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("cleanup goroutine did not stop in time")
	}
}

func TestListRegistrationTokensExcludesExpired(t *testing.T) {
	api := NewAPI(0,
		WithRegistrationTokenExpiration(50*time.Millisecond),
	)
	defer api.Stop()

	// Generate tokens
	_, err := api.GenerateRegistrationToken()
	if err != nil {
		t.Fatalf("failed to generate registration token: %v", err)
	}

	// All tokens should be listed initially
	tokens := api.ListRegistrationTokens()
	if len(tokens) != 1 {
		t.Errorf("expected 1 token, got %d", len(tokens))
	}

	// Wait for expiration
	time.Sleep(100 * time.Millisecond)

	// Expired tokens should not be listed
	tokens = api.ListRegistrationTokens()
	if len(tokens) != 0 {
		t.Errorf("expected 0 tokens (expired filtered), got %d", len(tokens))
	}
}

func TestStoredTokenIsExpired(t *testing.T) {
	now := time.Now()

	t.Run("not expired", func(t *testing.T) {
		token := storedToken{
			Token:     "test",
			CreatedAt: now,
			ExpiresAt: now.Add(time.Hour),
		}
		if token.isExpired() {
			t.Error("token should not be expired")
		}
	})

	t.Run("expired", func(t *testing.T) {
		token := storedToken{
			Token:     "test",
			CreatedAt: now.Add(-2 * time.Hour),
			ExpiresAt: now.Add(-time.Hour),
		}
		if !token.isExpired() {
			t.Error("token should be expired")
		}
	})
}

func TestDefaultExpirationValues(t *testing.T) {
	api := NewAPI(0)
	defer api.Stop()

	if api.registrationTokenExpiration != RegistrationTokenExpiration {
		t.Errorf("expected default registration token expiration %v, got %v",
			RegistrationTokenExpiration, api.registrationTokenExpiration)
	}

	if api.engineTokenExpiration != EngineTokenExpiration {
		t.Errorf("expected default engine token expiration %v, got %v",
			EngineTokenExpiration, api.engineTokenExpiration)
	}
}

func TestCustomExpirationValues(t *testing.T) {
	customRegExp := 30 * time.Minute
	customEngExp := 12 * time.Hour

	api := NewAPI(0,
		WithRegistrationTokenExpiration(customRegExp),
		WithEngineTokenExpiration(customEngExp),
	)
	defer api.Stop()

	if api.registrationTokenExpiration != customRegExp {
		t.Errorf("expected custom registration token expiration %v, got %v",
			customRegExp, api.registrationTokenExpiration)
	}

	if api.engineTokenExpiration != customEngExp {
		t.Errorf("expected custom engine token expiration %v, got %v",
			customEngExp, api.engineTokenExpiration)
	}
}

func TestInvalidExpirationValuesIgnored(t *testing.T) {
	// Zero or negative values should be ignored
	api := NewAPI(0,
		WithRegistrationTokenExpiration(0),
		WithEngineTokenExpiration(-1*time.Hour),
	)
	defer api.Stop()

	// Should still have default values
	if api.registrationTokenExpiration != RegistrationTokenExpiration {
		t.Errorf("expected default registration token expiration %v, got %v",
			RegistrationTokenExpiration, api.registrationTokenExpiration)
	}

	if api.engineTokenExpiration != EngineTokenExpiration {
		t.Errorf("expected default engine token expiration %v, got %v",
			EngineTokenExpiration, api.engineTokenExpiration)
	}
}
