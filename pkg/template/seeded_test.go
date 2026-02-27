package template

import (
	mathrand "math/rand/v2"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// newSeededCtx creates a template context with a fixed-seed RNG.
func newSeededCtx(seed uint64) *Context {
	r := httptest.NewRequest("GET", "/test", nil)
	ctx := NewContext(r, nil)
	ctx.Rand = mathrand.New(mathrand.NewPCG(seed, 0))
	return ctx
}

// =============================================================================
// Core Determinism Tests
// =============================================================================

func TestSeeded_Deterministic(t *testing.T) {
	engine := New()

	// Every expression that uses randomness should produce the same output
	// when given the same seed.
	templates := []struct {
		name string
		tmpl string
	}{
		{"uuid", "{{uuid}}"},
		{"uuid.short", "{{uuid.short}}"},
		{"random", "{{random}}"},
		{"random.int", "{{random.int}}"},
		{"random.int range", "{{random.int(1, 1000)}}"},
		{"random.float", "{{random.float}}"},
		{"random.float range", "{{random.float(1.0, 100.0, 2)}}"},
		{"random.string", "{{random.string}}"},
		{"random.string len", "{{random.string(20)}}"},
		{"faker.name", "{{faker.name}}"},
		{"faker.email", "{{faker.email}}"},
		{"faker.uuid", "{{faker.uuid}}"},
		{"faker.boolean", "{{faker.boolean}}"},
		{"faker.firstName", "{{faker.firstName}}"},
		{"faker.lastName", "{{faker.lastName}}"},
		{"faker.address", "{{faker.address}}"},
		{"faker.phone", "{{faker.phone}}"},
		{"faker.company", "{{faker.company}}"},
		{"faker.word", "{{faker.word}}"},
		{"faker.sentence", "{{faker.sentence}}"},
		{"faker.ipv4", "{{faker.ipv4}}"},
		{"faker.ipv6", "{{faker.ipv6}}"},
		{"faker.macAddress", "{{faker.macAddress}}"},
		{"faker.userAgent", "{{faker.userAgent}}"},
		{"faker.creditCard", "{{faker.creditCard}}"},
		{"faker.creditCardExp", "{{faker.creditCardExp}}"},
		{"faker.cvv", "{{faker.cvv}}"},
		{"faker.currencyCode", "{{faker.currencyCode}}"},
		{"faker.currency", "{{faker.currency}}"},
		{"faker.iban", "{{faker.iban}}"},
		{"faker.price", "{{faker.price}}"},
		{"faker.productName", "{{faker.productName}}"},
		{"faker.color", "{{faker.color}}"},
		{"faker.hexColor", "{{faker.hexColor}}"},
		{"faker.ssn", "{{faker.ssn}}"},
		{"faker.passport", "{{faker.passport}}"},
		{"faker.jobTitle", "{{faker.jobTitle}}"},
		{"faker.latitude", "{{faker.latitude}}"},
		{"faker.longitude", "{{faker.longitude}}"},
		{"faker.words", "{{faker.words}}"},
		{"faker.words(5)", "{{faker.words(5)}}"},
		{"faker.slug", "{{faker.slug}}"},
		{"faker.mimeType", "{{faker.mimeType}}"},
		{"faker.fileExtension", "{{faker.fileExtension}}"},
	}

	for _, tt := range templates {
		t.Run(tt.name, func(t *testing.T) {
			const seed uint64 = 42

			ctx1 := newSeededCtx(seed)
			result1, err := engine.Process(tt.tmpl, ctx1)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			ctx2 := newSeededCtx(seed)
			result2, err := engine.Process(tt.tmpl, ctx2)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			if result1 != result2 {
				t.Errorf("same seed should produce same result:\n  first:  %q\n  second: %q", result1, result2)
			}

			if result1 == "" {
				t.Errorf("seeded template %q should produce non-empty output", tt.tmpl)
			}
		})
	}
}

func TestSeeded_DifferentSeeds_DifferentOutput(t *testing.T) {
	engine := New()

	// Different seeds should (almost certainly) produce different output
	templates := []string{
		"{{uuid}}",
		"{{random.int(1, 1000000)}}",
		"{{random.string(30)}}",
		"{{faker.email}}",
		"{{faker.ipv4}}",
	}

	for _, tmpl := range templates {
		t.Run(tmpl, func(t *testing.T) {
			ctx1 := newSeededCtx(1)
			result1, _ := engine.Process(tmpl, ctx1)

			ctx2 := newSeededCtx(999999)
			result2, _ := engine.Process(tmpl, ctx2)

			if result1 == result2 {
				t.Errorf("different seeds should produce different results for %q, both got %q", tmpl, result1)
			}
		})
	}
}

// =============================================================================
// Multi-Expression Determinism
// =============================================================================

func TestSeeded_MultiExpression_Deterministic(t *testing.T) {
	engine := New()

	// A template with multiple expressions should be fully deterministic
	tmpl := `{"id": "{{uuid}}", "name": "{{faker.name}}", "age": {{random.int(18, 80)}}, "email": "{{faker.email}}"}`

	const seed uint64 = 12345

	ctx1 := newSeededCtx(seed)
	result1, err := engine.Process(tmpl, ctx1)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	ctx2 := newSeededCtx(seed)
	result2, err := engine.Process(tmpl, ctx2)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if result1 != result2 {
		t.Errorf("multi-expression template should be deterministic:\n  first:  %s\n  second: %s", result1, result2)
	}
}

// =============================================================================
// Nil RNG Fallback (Unseeded) Tests
// =============================================================================

func TestUnseeded_ProducesDifferentValues(t *testing.T) {
	engine := New()

	// Without a seed, calling Process multiple times should produce
	// different values (probabilistically — we check across 10 calls).
	tmpl := "{{random.int(1, 1000000)}}"

	seen := make(map[string]bool)
	for i := 0; i < 10; i++ {
		r := httptest.NewRequest("GET", "/test", nil)
		ctx := NewContext(r, nil)
		result, _ := engine.Process(tmpl, ctx)
		seen[result] = true
	}

	if len(seen) < 2 {
		t.Errorf("unseeded random.int should produce variation across 10 calls, got %d unique values", len(seen))
	}
}

func TestUnseeded_NilContext(t *testing.T) {
	engine := New()

	// Nil context should work (falls back to global RNG)
	result, err := engine.Process("{{random.int(1, 100)}}", nil)
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result == "" {
		t.Error("nil context should still produce output for random.int")
	}
}

// =============================================================================
// RNG Helper Function Tests
// =============================================================================

func TestRngIntN(t *testing.T) {
	rng := mathrand.New(mathrand.NewPCG(42, 0))

	t.Run("seeded produces deterministic values", func(t *testing.T) {
		rng1 := mathrand.New(mathrand.NewPCG(42, 0))
		rng2 := mathrand.New(mathrand.NewPCG(42, 0))

		for i := 0; i < 100; i++ {
			v1 := rngIntN(rng1, 1000)
			v2 := rngIntN(rng2, 1000)
			if v1 != v2 {
				t.Fatalf("iteration %d: rngIntN(42) = %d vs %d", i, v1, v2)
			}
		}
	})

	t.Run("nil falls back to global", func(t *testing.T) {
		// Should not panic
		v := rngIntN(nil, 100)
		if v < 0 || v >= 100 {
			t.Errorf("rngIntN(nil, 100) = %d, out of range", v)
		}
	})

	t.Run("n=0 returns 0", func(t *testing.T) {
		v := rngIntN(rng, 0)
		if v != 0 {
			t.Errorf("rngIntN(rng, 0) = %d, want 0", v)
		}
	})

	t.Run("negative n returns 0", func(t *testing.T) {
		v := rngIntN(rng, -5)
		if v != 0 {
			t.Errorf("rngIntN(rng, -5) = %d, want 0", v)
		}
	})
}

func TestRngFloat64(t *testing.T) {
	t.Run("seeded deterministic", func(t *testing.T) {
		rng1 := mathrand.New(mathrand.NewPCG(99, 0))
		rng2 := mathrand.New(mathrand.NewPCG(99, 0))

		for i := 0; i < 100; i++ {
			v1 := rngFloat64(rng1)
			v2 := rngFloat64(rng2)
			if v1 != v2 {
				t.Fatalf("iteration %d: rngFloat64(99) = %f vs %f", i, v1, v2)
			}
		}
	})

	t.Run("nil fallback", func(t *testing.T) {
		v := rngFloat64(nil)
		if v < 0 || v >= 1 {
			t.Errorf("rngFloat64(nil) = %f, expected [0, 1)", v)
		}
	})
}

func TestRngUUID(t *testing.T) {
	t.Run("seeded deterministic", func(t *testing.T) {
		rng1 := mathrand.New(mathrand.NewPCG(7, 0))
		rng2 := mathrand.New(mathrand.NewPCG(7, 0))

		u1 := rngUUID(rng1)
		u2 := rngUUID(rng2)
		if u1 != u2 {
			t.Errorf("same seed should produce same UUID: %q vs %q", u1, u2)
		}
	})

	t.Run("valid UUID v4 format", func(t *testing.T) {
		rng := mathrand.New(mathrand.NewPCG(123, 0))
		u := rngUUID(rng)

		pattern := `^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`
		if matched, _ := regexp.MatchString(pattern, u); !matched {
			t.Errorf("seeded UUID %q doesn't match UUID v4 format", u)
		}
	})

	t.Run("nil uses crypto/rand", func(t *testing.T) {
		u := rngUUID(nil)
		if u == "" {
			t.Error("nil RNG should still produce a UUID")
		}
		// Should be valid format
		pattern := `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
		if matched, _ := regexp.MatchString(pattern, u); !matched {
			t.Errorf("crypto UUID %q doesn't match UUID format", u)
		}
	})
}

func TestCtxRNG(t *testing.T) {
	t.Run("nil context returns nil", func(t *testing.T) {
		if rng := ctxRNG(nil); rng != nil {
			t.Error("ctxRNG(nil) should return nil")
		}
	})

	t.Run("context without Rand returns nil", func(t *testing.T) {
		ctx := &Context{}
		if rng := ctxRNG(ctx); rng != nil {
			t.Error("ctxRNG with nil Rand should return nil")
		}
	})

	t.Run("context with Rand returns RNG", func(t *testing.T) {
		rng := mathrand.New(mathrand.NewPCG(42, 0))
		ctx := &Context{Rand: rng}
		if got := ctxRNG(ctx); got != rng {
			t.Error("ctxRNG should return the context's RNG")
		}
	})
}

// =============================================================================
// Seeded Faker Function Tests
// =============================================================================

func TestSeeded_FakerFunctions_Deterministic(t *testing.T) {
	// Test that all faker helper functions produce deterministic output with a seeded RNG
	const seed uint64 = 777

	fakerFuncs := []struct {
		name string
		fn   func(rng *mathrand.Rand) string
	}{
		{"fakerIPv4", fakerIPv4},
		{"fakerIPv6", fakerIPv6},
		{"fakerMACAddress", fakerMACAddress},
		{"fakerCreditCard", fakerCreditCard},
		{"fakerCreditCardExp", fakerCreditCardExp},
		{"fakerCVV", fakerCVV},
		{"fakerIBAN", fakerIBAN},
		{"fakerPrice", fakerPrice},
		{"fakerSSN", fakerSSN},
		{"fakerPassport", fakerPassport},
		{"fakerHexColor", fakerHexColor},
		{"fakerLatitude", fakerLatitude},
		{"fakerLongitude", fakerLongitude},
		{"fakerSlug", func(rng *mathrand.Rand) string { return fakerSlug(rng) }},
	}

	for _, tt := range fakerFuncs {
		t.Run(tt.name, func(t *testing.T) {
			rng1 := mathrand.New(mathrand.NewPCG(seed, 0))
			rng2 := mathrand.New(mathrand.NewPCG(seed, 0))

			r1 := tt.fn(rng1)
			r2 := tt.fn(rng2)
			if r1 != r2 {
				t.Errorf("%s: same seed produced different results: %q vs %q", tt.name, r1, r2)
			}
			if r1 == "" {
				t.Errorf("%s: should produce non-empty output", tt.name)
			}
		})
	}
}

func TestSeeded_FakerWords_Deterministic(t *testing.T) {
	const seed uint64 = 555

	t.Run("same seed same words", func(t *testing.T) {
		rng1 := mathrand.New(mathrand.NewPCG(seed, 0))
		rng2 := mathrand.New(mathrand.NewPCG(seed, 0))

		w1 := fakerWords(rng1, 5)
		w2 := fakerWords(rng2, 5)
		if w1 != w2 {
			t.Errorf("same seed should produce same words: %q vs %q", w1, w2)
		}
	})

	t.Run("correct word count", func(t *testing.T) {
		rng := mathrand.New(mathrand.NewPCG(seed, 0))
		w := fakerWords(rng, 7)
		parts := strings.Fields(w)
		if len(parts) != 7 {
			t.Errorf("fakerWords(7) produced %d words: %q", len(parts), w)
		}
	})
}

// =============================================================================
// Seeded Random Function Tests
// =============================================================================

func TestSeeded_FuncRandomInt_Deterministic(t *testing.T) {
	const seed uint64 = 100

	rng1 := mathrand.New(mathrand.NewPCG(seed, 0))
	rng2 := mathrand.New(mathrand.NewPCG(seed, 0))

	r1 := funcRandomInt(rng1, 1, 1000)
	r2 := funcRandomInt(rng2, 1, 1000)
	if r1 != r2 {
		t.Errorf("same seed funcRandomInt: %q vs %q", r1, r2)
	}
}

func TestSeeded_FuncRandomFloat_Deterministic(t *testing.T) {
	const seed uint64 = 200

	rng1 := mathrand.New(mathrand.NewPCG(seed, 0))
	rng2 := mathrand.New(mathrand.NewPCG(seed, 0))

	r1 := funcRandomFloat(rng1)
	r2 := funcRandomFloat(rng2)
	if r1 != r2 {
		t.Errorf("same seed funcRandomFloat: %q vs %q", r1, r2)
	}
}

func TestSeeded_FuncRandomFloatRange_Deterministic(t *testing.T) {
	const seed uint64 = 300

	rng1 := mathrand.New(mathrand.NewPCG(seed, 0))
	rng2 := mathrand.New(mathrand.NewPCG(seed, 0))

	r1 := funcRandomFloatRange(rng1, "1.0", "100.0", "2")
	r2 := funcRandomFloatRange(rng2, "1.0", "100.0", "2")
	if r1 != r2 {
		t.Errorf("same seed funcRandomFloatRange: %q vs %q", r1, r2)
	}
}

func TestSeeded_FuncRandomString_Deterministic(t *testing.T) {
	const seed uint64 = 400

	rng1 := mathrand.New(mathrand.NewPCG(seed, 0))
	rng2 := mathrand.New(mathrand.NewPCG(seed, 0))

	r1 := funcRandomString(rng1, 20)
	r2 := funcRandomString(rng2, 20)
	if r1 != r2 {
		t.Errorf("same seed funcRandomString: %q vs %q", r1, r2)
	}
	if len(r1) != 20 {
		t.Errorf("funcRandomString(20) length = %d, want 20", len(r1))
	}
}

// =============================================================================
// Seeded resolveFaker Tests
// =============================================================================

func TestSeeded_ResolveFaker_AllTypes(t *testing.T) {
	const seed uint64 = 42

	fakerTypes := []string{
		"uuid", "boolean", "name", "firstName", "lastName", "email",
		"address", "phone", "company", "word", "sentence",
		"ipv4", "ipv6", "macAddress", "userAgent",
		"creditCard", "creditCardExp", "cvv", "currencyCode", "currency", "iban",
		"price", "productName", "color", "hexColor",
		"ssn", "passport", "jobTitle",
		"latitude", "longitude",
		"words", "slug",
		"mimeType", "fileExtension",
	}

	for _, ft := range fakerTypes {
		t.Run(ft, func(t *testing.T) {
			rng1 := mathrand.New(mathrand.NewPCG(seed, 0))
			rng2 := mathrand.New(mathrand.NewPCG(seed, 0))

			r1 := resolveFaker(rng1, ft)
			r2 := resolveFaker(rng2, ft)
			if r1 != r2 {
				t.Errorf("resolveFaker(%q) same seed: %q vs %q", ft, r1, r2)
			}
			if r1 == "" {
				t.Errorf("resolveFaker(%q) should produce non-empty output", ft)
			}
		})
	}
}

// =============================================================================
// Non-Random Expressions Unaffected
// =============================================================================

func TestSeeded_NonRandomExpressionsUnaffected(t *testing.T) {
	engine := New()

	// Non-random expressions should work normally with a seeded context
	ctx := newSeededCtx(42)

	tests := []struct {
		name     string
		tmpl     string
		checkFn  func(string) bool
		errorMsg string
	}{
		{
			name:     "now",
			tmpl:     "{{now}}",
			checkFn:  func(s string) bool { return len(s) > 0 && strings.Contains(s, "T") },
			errorMsg: "now should produce RFC3339 timestamp",
		},
		{
			name:     "timestamp",
			tmpl:     "{{timestamp}}",
			checkFn:  func(s string) bool { return len(s) > 0 },
			errorMsg: "timestamp should produce non-empty value",
		},
		{
			name:     "literal text",
			tmpl:     "hello world",
			checkFn:  func(s string) bool { return s == "hello world" },
			errorMsg: "literal text should pass through",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := engine.Process(tt.tmpl, ctx)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if !tt.checkFn(result) {
				t.Errorf("%s: got %q", tt.errorMsg, result)
			}
		})
	}
}

// =============================================================================
// Seeded Space-Separated Syntax
// =============================================================================

func TestSeeded_SpaceSeparatedSyntax(t *testing.T) {
	engine := New()
	const seed uint64 = 42

	t.Run("random.int min max", func(t *testing.T) {
		ctx1 := newSeededCtx(seed)
		ctx2 := newSeededCtx(seed)

		r1, _ := engine.Process("{{random.int 1 100}}", ctx1)
		r2, _ := engine.Process("{{random.int 1 100}}", ctx2)
		if r1 != r2 {
			t.Errorf("space-separated random.int: %q vs %q", r1, r2)
		}
	})
}

// =============================================================================
// Edge Cases
// =============================================================================

func TestSeeded_ZeroSeed(t *testing.T) {
	engine := New()

	// Seed of 0 should still work (it's a valid seed)
	ctx1 := newSeededCtx(0)
	ctx2 := newSeededCtx(0)

	r1, _ := engine.Process("{{uuid}}", ctx1)
	r2, _ := engine.Process("{{uuid}}", ctx2)
	if r1 != r2 {
		t.Errorf("seed=0 should be deterministic: %q vs %q", r1, r2)
	}
}

func TestSeeded_LargeSeed(t *testing.T) {
	engine := New()

	// Very large seed values should work
	const seed uint64 = 18446744073709551615 // max uint64

	ctx1 := newSeededCtx(seed)
	ctx2 := newSeededCtx(seed)

	r1, _ := engine.Process("{{faker.email}}", ctx1)
	r2, _ := engine.Process("{{faker.email}}", ctx2)
	if r1 != r2 {
		t.Errorf("large seed should be deterministic: %q vs %q", r1, r2)
	}
}

func TestSeeded_ConcurrentSafety(t *testing.T) {
	engine := New()

	// Each goroutine gets its own seeded context, so there should be no data race.
	// Run with -race to verify.
	done := make(chan string, 20)

	for i := 0; i < 20; i++ {
		go func(id int) {
			ctx := newSeededCtx(uint64(id))
			result, _ := engine.Process("{{faker.name}} {{random.int(1, 1000)}}", ctx)
			done <- result
		}(i)
	}

	results := make([]string, 20)
	for i := 0; i < 20; i++ {
		results[i] = <-done
	}

	// All should be non-empty
	for i, r := range results {
		if r == "" {
			t.Errorf("goroutine %d produced empty result", i)
		}
	}
}
