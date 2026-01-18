package mtls

import (
	"context"
	"net/http"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

// identityKey is the context key used to store and retrieve ClientIdentity.
const identityKey contextKey = "mtls-identity"

// FromContext retrieves the ClientIdentity from the given context.
// Returns nil if no identity is present in the context.
func FromContext(ctx context.Context) *ClientIdentity {
	if ctx == nil {
		return nil
	}
	identity, ok := ctx.Value(identityKey).(*ClientIdentity)
	if !ok {
		return nil
	}
	return identity
}

// WithIdentity returns a new context with the given ClientIdentity attached.
// The identity can later be retrieved using FromContext.
func WithIdentity(ctx context.Context, identity *ClientIdentity) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, identityKey, identity)
}

// ExtractFromRequest extracts the client identity from an HTTP request's
// TLS connection state. It examines the peer certificates provided during
// the TLS handshake and extracts identity information from the first
// (leaf) certificate.
//
// Returns nil if:
//   - The request is nil
//   - The connection is not using TLS
//   - No peer certificates were provided
func ExtractFromRequest(r *http.Request) *ClientIdentity {
	if r == nil {
		return nil
	}

	// Check if TLS connection state is available
	if r.TLS == nil {
		return nil
	}

	// Check if peer certificates are available
	if len(r.TLS.PeerCertificates) == 0 {
		return nil
	}

	// The first certificate is the leaf (client) certificate
	clientCert := r.TLS.PeerCertificates[0]

	// Determine if the certificate was verified
	// VerifiedChains will be populated if the certificate was verified
	verified := len(r.TLS.VerifiedChains) > 0

	return ExtractIdentity(clientCert, verified)
}
