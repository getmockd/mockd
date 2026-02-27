package template

import (
	"fmt"
	mathrand "math/rand/v2"
)

// rngIntN returns a random int in [0, n) using the provided RNG if non-nil,
// otherwise falls back to the global math/rand/v2 source.
func rngIntN(rng *mathrand.Rand, n int) int {
	if n <= 0 {
		return 0
	}
	if rng != nil {
		return rng.IntN(n)
	}
	return mathrand.IntN(n)
}

// rngFloat64 returns a random float64 in [0, 1) using the provided RNG if non-nil,
// otherwise falls back to the global math/rand/v2 source.
func rngFloat64(rng *mathrand.Rand) float64 {
	if rng != nil {
		return rng.Float64()
	}
	return mathrand.Float64()
}

// rngUUID generates a UUID v4 string. When rng is non-nil, uses the seeded PRNG
// for deterministic output. When nil, uses crypto/rand for true randomness.
func rngUUID(rng *mathrand.Rand) string {
	if rng == nil {
		return funcUUID() // crypto/rand based
	}
	// Generate 16 random bytes from seeded PRNG
	var b [16]byte
	for i := range b {
		b[i] = byte(rng.IntN(256))
	}
	// Set version 4 and variant bits
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// ctxRNG extracts the seeded RNG from a Context, or returns nil (use global).
func ctxRNG(ctx *Context) *mathrand.Rand {
	if ctx == nil {
		return nil
	}
	return ctx.Rand
}
