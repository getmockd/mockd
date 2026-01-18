// Package id provides unique identifier generation utilities.
//
// This is the canonical source for ID generation across the mockd codebase.
// It provides several ID formats for different use cases:
//
//   - UUID: Standard UUID v4 (random) for general-purpose unique identifiers
//   - ULID: Universally Unique Lexicographically Sortable Identifiers for
//     time-ordered IDs that are collision-free and sortable
//   - Short: 16-character hex IDs for user-facing contexts where brevity matters
//   - Alphanumeric: Configurable-length random alphanumeric strings
//
// All ID generation functions use crypto/rand for secure randomness.
//
// The ULID implementation follows the ULID specification, producing 26-character
// identifiers that encode a timestamp and random component, enabling natural
// chronological sorting while maintaining uniqueness.
package id
