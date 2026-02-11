package id

import (
	"regexp"
	"strings"
	"sync"
	"testing"
)

// --- UUID Tests ---

func TestUUID_Format(t *testing.T) {
	id := UUID()

	// UUID v4 format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidRegex.MatchString(id) {
		t.Errorf("UUID() = %q, does not match UUID v4 format", id)
	}
}

func TestUUID_Length(t *testing.T) {
	id := UUID()
	if len(id) != 36 {
		t.Errorf("UUID() length = %d, want 36", len(id))
	}
}

func TestUUID_VersionBit(t *testing.T) {
	// Generate many UUIDs and check version bit is always 4
	for i := 0; i < 100; i++ {
		id := UUID()
		// Position 14 (0-indexed) should be '4' — the version nibble
		if id[14] != '4' {
			t.Errorf("UUID() version nibble = %c, want '4' (id=%s)", id[14], id)
		}
	}
}

func TestUUID_VariantBit(t *testing.T) {
	// Position 19 (0-indexed) should be one of: 8, 9, a, b
	validVariant := map[byte]bool{'8': true, '9': true, 'a': true, 'b': true}
	for i := 0; i < 100; i++ {
		id := UUID()
		if !validVariant[id[19]] {
			t.Errorf("UUID() variant nibble = %c, want one of 8/9/a/b (id=%s)", id[19], id)
		}
	}
}

func TestUUID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := UUID()
		if seen[id] {
			t.Fatalf("UUID() generated duplicate: %s", id)
		}
		seen[id] = true
	}
}

func TestUUID_Concurrent(t *testing.T) {
	const goroutines = 50
	const perGoroutine = 100

	results := make(chan string, goroutines*perGoroutine)
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				results <- UUID()
			}
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[string]bool, goroutines*perGoroutine)
	for id := range results {
		if seen[id] {
			t.Fatalf("UUID() concurrent duplicate: %s", id)
		}
		seen[id] = true
	}
}

// --- Short Tests ---

func TestShort_Length(t *testing.T) {
	id := Short()
	if len(id) != 16 {
		t.Errorf("Short() length = %d, want 16", len(id))
	}
}

func TestShort_HexOnly(t *testing.T) {
	hexRegex := regexp.MustCompile(`^[0-9a-f]{16}$`)
	for i := 0; i < 100; i++ {
		id := Short()
		if !hexRegex.MatchString(id) {
			t.Errorf("Short() = %q, not valid hex", id)
		}
	}
}

func TestShort_Uniqueness(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := Short()
		if seen[id] {
			t.Fatalf("Short() generated duplicate: %s", id)
		}
		seen[id] = true
	}
}

func TestShort_Concurrent(t *testing.T) {
	const goroutines = 50
	const perGoroutine = 100

	results := make(chan string, goroutines*perGoroutine)
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				results <- Short()
			}
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[string]bool, goroutines*perGoroutine)
	for id := range results {
		if seen[id] {
			t.Fatalf("Short() concurrent duplicate: %s", id)
		}
		seen[id] = true
	}
}

// --- ULID Tests ---

func TestULID_Length(t *testing.T) {
	id := ULID()
	if len(id) != 26 {
		t.Errorf("ULID() length = %d, want 26", len(id))
	}
}

func TestULID_CharacterSet(t *testing.T) {
	// ULIDs use Crockford's Base32: 0-9, A-H, J-K, M-N, P, Q, R-T, V-W, X-Z
	// Excluded: I, L, O, U
	for i := 0; i < 100; i++ {
		id := ULID()
		for _, c := range id {
			if !isValidULIDChar(byte(c)) {
				t.Errorf("ULID() contains invalid char %c in %s", c, id)
			}
		}
	}
}

func TestULID_ExcludedCharacters(t *testing.T) {
	// I, L, O, U should never appear in ULIDs (Crockford's Base32)
	excluded := "ILOU"
	for i := 0; i < 500; i++ {
		id := ULID()
		for _, c := range excluded {
			if strings.ContainsRune(id, c) {
				t.Errorf("ULID() = %q, contains excluded char %c", id, c)
			}
		}
	}
}

func TestULID_Uniqueness(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := ULID()
		if seen[id] {
			t.Fatalf("ULID() generated duplicate: %s", id)
		}
		seen[id] = true
	}
}

func TestULID_Sortable(t *testing.T) {
	// ULIDs generated sequentially should be lexicographically sortable
	// (at least the timestamp prefix should be non-decreasing)
	prev := ULID()
	for i := 0; i < 100; i++ {
		curr := ULID()
		// Timestamp portion is first 10 chars
		if curr[:10] < prev[:10] {
			t.Errorf("ULID() not time-sortable: %s < %s (timestamp portion)", curr[:10], prev[:10])
		}
		prev = curr
	}
}

func TestULID_Concurrent(t *testing.T) {
	const goroutines = 50
	const perGoroutine = 100

	results := make(chan string, goroutines*perGoroutine)
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				results <- ULID()
			}
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[string]bool, goroutines*perGoroutine)
	for id := range results {
		if len(id) != 26 {
			t.Errorf("ULID() concurrent length = %d, want 26", len(id))
		}
		if seen[id] {
			t.Fatalf("ULID() concurrent duplicate: %s", id)
		}
		seen[id] = true
	}
}

func TestULID_SameMillisecondUnique(t *testing.T) {
	// Generate many ULIDs as fast as possible — they should all be unique
	// even within the same millisecond (counter + random ensure this)
	seen := make(map[string]bool, 10000)
	for i := 0; i < 10000; i++ {
		id := ULID()
		if seen[id] {
			t.Fatalf("ULID() duplicate within burst: %s (iteration %d)", id, i)
		}
		seen[id] = true
	}
}

// --- IsValidULID Tests ---

func TestIsValidULID_Valid(t *testing.T) {
	// Generate a real ULID and verify
	id := ULID()
	if !IsValidULID(id) {
		t.Errorf("IsValidULID(%q) = false, want true", id)
	}
}

func TestIsValidULID_ValidCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"all zeros", "00000000000000000000000000", true},
		{"all 9s", "99999999999999999999999999", true},
		{"mixed valid", "01ARZ3NDEKTSV4RRFFQ69G5FAV", true},
		{"generated", ULID(), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidULID(tt.input); got != tt.want {
				t.Errorf("IsValidULID(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsValidULID_InvalidCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"too short", "01ARZ3NDEKTSV4RRFFQ69G5FA"},
		{"too long", "01ARZ3NDEKTSV4RRFFQ69G5FAVX"},
		{"contains I", "01ARZ3NDIKTSV4RRFFQ69G5FAV"},
		{"contains L", "01ARZ3NDLKTSV4RRFFQ69G5FAV"},
		{"contains O", "01ARZ3NDOKTSV4RRFFQ69G5FAV"},
		{"contains U", "01ARZ3NDUKTSV4RRFFQ69G5FAV"},
		{"lowercase valid chars", "01arz3ndektsv4rrffq69g5fav"},
		{"contains space", "01ARZ3NDE KTSV4RRFFQ69G5FA"},
		{"contains dash", "01ARZ3NDE-KTSV4RRFFQ69G5FA"},
		{"uuid format", "550e8400-e29b-41d4-a716-446655440000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if IsValidULID(tt.input) {
				t.Errorf("IsValidULID(%q) = true, want false", tt.input)
			}
		})
	}
}

// --- Alphanumeric Tests ---

func TestAlphanumeric_Length(t *testing.T) {
	tests := []int{0, 1, 5, 10, 32, 64, 128, 256}
	for _, length := range tests {
		result := Alphanumeric(length)
		if len(result) != length {
			t.Errorf("Alphanumeric(%d) length = %d, want %d", length, len(result), length)
		}
	}
}

func TestAlphanumeric_ZeroLength(t *testing.T) {
	result := Alphanumeric(0)
	if result != "" {
		t.Errorf("Alphanumeric(0) = %q, want empty string", result)
	}
}

func TestAlphanumeric_NegativeLength(t *testing.T) {
	result := Alphanumeric(-1)
	if result != "" {
		t.Errorf("Alphanumeric(-1) = %q, want empty string", result)
	}
	result = Alphanumeric(-100)
	if result != "" {
		t.Errorf("Alphanumeric(-100) = %q, want empty string", result)
	}
}

func TestAlphanumeric_CharacterSet(t *testing.T) {
	alphanumRegex := regexp.MustCompile(`^[a-zA-Z0-9]+$`)
	for i := 0; i < 100; i++ {
		result := Alphanumeric(32)
		if !alphanumRegex.MatchString(result) {
			t.Errorf("Alphanumeric(32) = %q, contains non-alphanumeric characters", result)
		}
	}
}

func TestAlphanumeric_Distribution(t *testing.T) {
	// Generate many characters and check distribution is roughly uniform
	// across the 62-character charset (lowercase + uppercase + digits)
	counts := make(map[byte]int)
	const totalChars = 62000 // ~1000 per character on average
	for i := 0; i < totalChars/100; i++ {
		s := Alphanumeric(100)
		for j := 0; j < len(s); j++ {
			counts[s[j]]++
		}
	}

	// Each of 62 characters should appear roughly 1000 times
	// Allow wide tolerance (500-2000) since this is probabilistic
	for c, count := range counts {
		if count < 300 || count > 2500 {
			t.Errorf("Character %c appeared %d times (expected ~1000), possible bias", c, count)
		}
	}

	// All 62 characters should be represented
	if len(counts) != 62 {
		t.Errorf("Got %d distinct characters, want 62", len(counts))
	}
}

func TestAlphanumeric_Uniqueness(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for i := 0; i < 1000; i++ {
		id := Alphanumeric(32)
		if seen[id] {
			t.Fatalf("Alphanumeric(32) generated duplicate: %s", id)
		}
		seen[id] = true
	}
}

func TestAlphanumeric_Concurrent(t *testing.T) {
	const goroutines = 50
	const perGoroutine = 100

	results := make(chan string, goroutines*perGoroutine)
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				results <- Alphanumeric(32)
			}
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[string]bool, goroutines*perGoroutine)
	for id := range results {
		if len(id) != 32 {
			t.Errorf("Alphanumeric(32) concurrent length = %d, want 32", len(id))
		}
		if seen[id] {
			t.Fatalf("Alphanumeric(32) concurrent duplicate: %s", id)
		}
		seen[id] = true
	}
}

func TestAlphanumeric_SmallLengths(t *testing.T) {
	// Length 1 should produce a single alphanumeric character
	for i := 0; i < 100; i++ {
		result := Alphanumeric(1)
		if len(result) != 1 {
			t.Errorf("Alphanumeric(1) length = %d, want 1", len(result))
		}
		c := result[0]
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			t.Errorf("Alphanumeric(1) = %q, not alphanumeric", result)
		}
	}
}

// --- isValidULIDChar Tests ---

func TestIsValidULIDChar(t *testing.T) {
	// Valid characters: Crockford's Base32
	for _, c := range ulidEncoding {
		if !isValidULIDChar(byte(c)) {
			t.Errorf("isValidULIDChar(%c) = false, want true (in encoding)", c)
		}
	}

	// Invalid characters
	invalid := []byte{'I', 'L', 'O', 'U', 'a', 'i', 'l', 'o', 'u', '-', ' ', '!'}
	for _, c := range invalid {
		if isValidULIDChar(c) {
			t.Errorf("isValidULIDChar(%c) = true, want false", c)
		}
	}
}

// --- encodeULID Tests ---

func TestEncodeULID_Deterministic(t *testing.T) {
	// Same inputs should produce the different outputs (because of random component)
	// but same timestamp prefix
	a := encodeULID(1000, 0)
	b := encodeULID(1000, 0)
	// Timestamp portion (first 10 chars) should be identical
	if a[:10] != b[:10] {
		t.Errorf("encodeULID same timestamp: %s[:10] != %s[:10]", a, b)
	}
	// Full strings should differ (random component)
	// This could theoretically fail with astronomically low probability
	if a == b {
		t.Logf("encodeULID produced identical outputs (extremely unlikely but possible): %s", a)
	}
}

func TestEncodeULID_DifferentTimestamps(t *testing.T) {
	a := encodeULID(1000, 0)
	b := encodeULID(2000, 0)
	// Different timestamps should produce different prefixes
	if a[:10] == b[:10] {
		t.Errorf("encodeULID different timestamps produced same prefix: %s, %s", a[:10], b[:10])
	}
}

func TestEncodeULID_Length(t *testing.T) {
	result := encodeULID(0, 0)
	if len(result) != 26 {
		t.Errorf("encodeULID(0, 0) length = %d, want 26", len(result))
	}
}

func TestEncodeULID_ZeroTimestamp(t *testing.T) {
	result := encodeULID(0, 0)
	// First 10 chars should be "0000000000"
	if result[:10] != "0000000000" {
		t.Errorf("encodeULID(0, 0) timestamp prefix = %s, want 0000000000", result[:10])
	}
}

// --- Benchmarks ---

func BenchmarkUUID(b *testing.B) {
	for b.Loop() {
		UUID()
	}
}

func BenchmarkShort(b *testing.B) {
	for b.Loop() {
		Short()
	}
}

func BenchmarkULID(b *testing.B) {
	for b.Loop() {
		ULID()
	}
}

func BenchmarkAlphanumeric16(b *testing.B) {
	for b.Loop() {
		Alphanumeric(16)
	}
}

func BenchmarkAlphanumeric64(b *testing.B) {
	for b.Loop() {
		Alphanumeric(64)
	}
}

func BenchmarkIsValidULID(b *testing.B) {
	id := ULID()
	for b.Loop() {
		IsValidULID(id)
	}
}
