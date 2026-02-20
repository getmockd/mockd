package template

import (
	cryptorand "crypto/rand"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
)

// UUID functions

// funcUUID generates a UUID v4 using crypto/rand
func funcUUID() string {
	b := make([]byte, 16)
	_, err := cryptorand.Read(b)
	if err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// funcUUIDShort returns the first 8 characters of a UUID v4
func funcUUIDShort() string {
	uuid := funcUUID()
	if uuid == "" {
		return ""
	}
	if len(uuid) < 8 {
		return uuid
	}
	return uuid[:8]
}

// Random functions

// funcRandomInt returns a random integer between min and max (inclusive) as a string
func funcRandomInt(min, max int) string {
	if min > max {
		return ""
	}
	// Use math/rand/v2 which is automatically seeded
	n := rand.IntN(max-min+1) + min
	return strconv.Itoa(n)
}

// funcRandomFloat returns a random float between 0 and 1 as a string
func funcRandomFloat() string {
	return fmt.Sprintf("%f", rand.Float64())
}

// funcRandomFloatRange returns a random float in a range with optional precision.
// minStr and maxStr are parsed as float64; precisionStr is parsed as int (-1 if empty).
func funcRandomFloatRange(minStr, maxStr, precisionStr string) string {
	minVal, _ := strconv.ParseFloat(minStr, 64)
	maxVal, _ := strconv.ParseFloat(maxStr, 64)
	precision := -1
	if precisionStr != "" {
		precision, _ = strconv.Atoi(precisionStr)
	}

	if minVal >= maxVal {
		maxVal = minVal + 1
	}

	val := minVal + rand.Float64()*(maxVal-minVal)

	if precision >= 0 {
		format := fmt.Sprintf("%%.%df", precision)
		return fmt.Sprintf(format, val)
	}
	return strconv.FormatFloat(val, 'f', -1, 64)
}

// funcRandomString returns a random alphanumeric string of the given length.
func funcRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.IntN(len(charset))]
	}
	return string(b)
}

// String functions

// funcUpper converts a string to uppercase
func funcUpper(s string) string {
	return strings.ToUpper(s)
}

// funcLower converts a string to lowercase
func funcLower(s string) string {
	return strings.ToLower(s)
}

// Default function

// funcDefault returns value if non-empty, otherwise returns fallback
func funcDefault(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}
