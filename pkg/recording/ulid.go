// Package recording provides ULID generation for recording identifiers.
package recording

import (
	"crypto/rand"
	"sync"
	"time"
)

// ULID encoding uses Crockford's Base32 (excludes I, L, O, U to avoid ambiguity)
const ulidEncoding = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

var (
	ulidMu      sync.Mutex
	ulidLastMs  int64
	ulidCounter uint16
)

// NewULID generates a new ULID (Universally Unique Lexicographically Sortable Identifier).
// ULIDs are 26 characters long, time-sortable, and collision-free.
// Format: TTTTTTTTTTRRRRRRRRRRRRRRR (10 chars timestamp + 16 chars randomness)
func NewULID() string {
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

	// Encode timestamp (first 10 characters, 48 bits = 6 bytes)
	// ULID timestamp is big-endian
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
		return time.Time{}, ErrInvalidULID
	}

	var ms int64
	for i := 0; i < 10; i++ {
		val := decodeChar(ulid[i])
		if val < 0 {
			return time.Time{}, ErrInvalidULID
		}
		ms = (ms << 5) | int64(val)
	}

	return time.UnixMilli(ms), nil
}

// decodeChar decodes a single ULID character to its value.
func decodeChar(c byte) int {
	for i := 0; i < len(ulidEncoding); i++ {
		if ulidEncoding[i] == c {
			return i
		}
	}
	return -1
}
