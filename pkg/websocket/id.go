package websocket

import (
	"crypto/rand"
	"encoding/hex"
	"sync/atomic"
	"time"
)

var connectionCounter uint64

// GenerateConnectionID generates a unique connection ID.
// The format is "conn-{timestamp_hex}-{counter}-{random}" for uniqueness and sortability.
func GenerateConnectionID() string {
	// Get timestamp in hex (8 chars)
	ts := time.Now().UnixNano()
	tsHex := make([]byte, 8)
	for i := 7; i >= 0; i-- {
		tsHex[i] = byte(ts & 0xff)
		ts >>= 8
	}

	// Get counter (4 chars)
	counter := atomic.AddUint64(&connectionCounter, 1)
	counterHex := make([]byte, 4)
	for i := 3; i >= 0; i-- {
		counterHex[i] = byte(counter & 0xff)
		counter >>= 8
	}

	// Get random bytes (4 chars)
	randomBytes := make([]byte, 4)
	_, _ = rand.Read(randomBytes)

	return "conn-" + hex.EncodeToString(tsHex) + "-" + hex.EncodeToString(counterHex) + "-" + hex.EncodeToString(randomBytes)
}
