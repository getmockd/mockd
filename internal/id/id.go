// Package id provides unique identifier generation utilities.
// This is the canonical source for ID generation across the codebase.
package id

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// UUID generates a UUID v4 (random).
// Returns a string in the format: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
func UUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	// Set version (4) and variant bits per RFC 4122
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Short generates a short random hex ID (16 characters).
// Suitable for user-facing IDs where brevity matters.
func Short() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// --- ULID Implementation ---
// ULID: Universally Unique Lexicographically Sortable Identifier
// 26 characters, time-sortable, collision-free

// ulidEncoding uses Crockford's Base32 (excludes I, L, O, U to avoid ambiguity)
const ulidEncoding = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var (
	ulidMu      sync.Mutex
	ulidLastMs  int64
	ulidCounter uint16
)

// ULID generates a new ULID (Universally Unique Lexicographically Sortable Identifier).
// ULIDs are 26 characters long, time-sortable, and collision-free.
// Format: TTTTTTTTTTRRRRRRRRRRRRRRR (10 chars timestamp + 16 chars randomness)
func ULID() string {
	ulidMu.Lock()
	defer ulidMu.Unlock()

	now := time.Now().UnixMilli()

	// If same millisecond, increment counter
	if now == ulidLastMs {
		ulidCounter++
		if ulidCounter == 0 {
			// Counter overflow, wait for next millisecond
			for now == ulidLastMs {
				time.Sleep(time.Millisecond)
				now = time.Now().UnixMilli()
			}
		}
	} else {
		ulidLastMs = now
		ulidCounter = 0
	}

	return encodeULID(now, ulidCounter)
}

// encodeULID encodes a timestamp and counter into a ULID string.
func encodeULID(ms int64, counter uint16) string {
	ulid := make([]byte, 26)

	// Encode timestamp (first 10 characters, 48 bits)
	ulid[0] = ulidEncoding[(ms>>45)&0x1F]
	ulid[1] = ulidEncoding[(ms>>40)&0x1F]
	ulid[2] = ulidEncoding[(ms>>35)&0x1F]
	ulid[3] = ulidEncoding[(ms>>30)&0x1F]
	ulid[4] = ulidEncoding[(ms>>25)&0x1F]
	ulid[5] = ulidEncoding[(ms>>20)&0x1F]
	ulid[6] = ulidEncoding[(ms>>15)&0x1F]
	ulid[7] = ulidEncoding[(ms>>10)&0x1F]
	ulid[8] = ulidEncoding[(ms>>5)&0x1F]
	ulid[9] = ulidEncoding[ms&0x1F]

	// Generate random bytes for the remaining 16 characters (80 bits = 10 bytes)
	randomBytes := make([]byte, 10)
	_, _ = rand.Read(randomBytes)

	// Mix in counter to first 2 random bytes for uniqueness within same millisecond
	randomBytes[0] ^= byte(counter >> 8)
	randomBytes[1] ^= byte(counter)

	// Encode randomness (last 16 characters)
	ulid[10] = ulidEncoding[(randomBytes[0]>>3)&0x1F]
	ulid[11] = ulidEncoding[((randomBytes[0]&0x07)<<2)|((randomBytes[1]>>6)&0x03)]
	ulid[12] = ulidEncoding[(randomBytes[1]>>1)&0x1F]
	ulid[13] = ulidEncoding[((randomBytes[1]&0x01)<<4)|((randomBytes[2]>>4)&0x0F)]
	ulid[14] = ulidEncoding[((randomBytes[2]&0x0F)<<1)|((randomBytes[3]>>7)&0x01)]
	ulid[15] = ulidEncoding[(randomBytes[3]>>2)&0x1F]
	ulid[16] = ulidEncoding[((randomBytes[3]&0x03)<<3)|((randomBytes[4]>>5)&0x07)]
	ulid[17] = ulidEncoding[randomBytes[4]&0x1F]
	ulid[18] = ulidEncoding[(randomBytes[5]>>3)&0x1F]
	ulid[19] = ulidEncoding[((randomBytes[5]&0x07)<<2)|((randomBytes[6]>>6)&0x03)]
	ulid[20] = ulidEncoding[(randomBytes[6]>>1)&0x1F]
	ulid[21] = ulidEncoding[((randomBytes[6]&0x01)<<4)|((randomBytes[7]>>4)&0x0F)]
	ulid[22] = ulidEncoding[((randomBytes[7]&0x0F)<<1)|((randomBytes[8]>>7)&0x01)]
	ulid[23] = ulidEncoding[(randomBytes[8]>>2)&0x1F]
	ulid[24] = ulidEncoding[((randomBytes[8]&0x03)<<3)|((randomBytes[9]>>5)&0x07)]
	ulid[25] = ulidEncoding[randomBytes[9]&0x1F]

	return string(ulid)
}

// IsValidULID checks if a string is a valid ULID.
func IsValidULID(s string) bool {
	if len(s) != 26 {
		return false
	}
	for _, c := range s {
		if !isValidULIDChar(byte(c)) {
			return false
		}
	}
	return true
}

// isValidULIDChar checks if a byte is a valid ULID character.
func isValidULIDChar(c byte) bool {
	for i := 0; i < len(ulidEncoding); i++ {
		if ulidEncoding[i] == c {
			return true
		}
	}
	return false
}

// ULIDTime extracts the timestamp from a ULID.
func ULIDTime(ulid string) (time.Time, error) {
	if !IsValidULID(ulid) {
		return time.Time{}, fmt.Errorf("invalid ULID: %s", ulid)
	}

	var ms int64
	for i := 0; i < 10; i++ {
		val := decodeULIDChar(ulid[i])
		if val < 0 {
			return time.Time{}, fmt.Errorf("invalid ULID character at position %d", i)
		}
		ms = (ms << 5) | int64(val)
	}

	return time.UnixMilli(ms), nil
}

// decodeULIDChar decodes a single ULID character to its value.
func decodeULIDChar(c byte) int {
	for i := 0; i < len(ulidEncoding); i++ {
		if ulidEncoding[i] == c {
			return i
		}
	}
	return -1
}

// Alphanumeric generates a random alphanumeric string of the specified length.
// Uses uppercase, lowercase letters and digits.
func Alphanumeric(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	randBytes := make([]byte, length)
	_, _ = rand.Read(randBytes)
	for i := range b {
		b[i] = charset[int(randBytes[i])%len(charset)]
	}
	return string(b)
}
