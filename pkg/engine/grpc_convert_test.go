package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/getmockd/mockd/pkg/mock"
)

// TestConvertGRPCMethodConfig_CarriesVariants verifies that converting a
// mock.MethodConfig into the grpc.MethodConfig the server consumes carries ALL
// match variants through in order — the core of the issue #30 fix on the engine
// side.
func TestConvertGRPCMethodConfig_CarriesVariants(t *testing.T) {
	src := mock.MethodConfig{
		Match:    &mock.MethodMatch{Request: map[string]any{"id": "123"}},
		Response: map[string]any{"id": "123", "name": "John Doe"},
		Delay:    "10ms",
		Variants: []mock.MethodConfig{
			{
				Match:    &mock.MethodMatch{Request: map[string]any{"id": "999"}},
				Response: map[string]any{"id": "999", "name": "Jane Doe"},
			},
			{
				// Unconditioned default, ordered last.
				Response: map[string]any{"id": "0", "name": "Default"},
			},
		},
	}

	got := convertGRPCMethodConfig(src)

	// Primary fields preserved.
	require.NotNil(t, got.Match)
	assert.Equal(t, "123", got.Match.Request["id"])
	assert.Equal(t, "10ms", got.Delay)

	// Both variants carried through in order.
	require.Len(t, got.Variants, 2)
	require.NotNil(t, got.Variants[0].Match)
	assert.Equal(t, "999", got.Variants[0].Match.Request["id"])
	assert.Nil(t, got.Variants[1].Match, "default variant has no match")
	assert.Equal(t, map[string]any{"id": "0", "name": "Default"}, got.Variants[1].Response)

	// Error config is also converted on variants when present.
	withErr := mock.MethodConfig{
		Variants: []mock.MethodConfig{
			{
				Match: &mock.MethodMatch{Request: map[string]any{"id": "boom"}},
				Error: &mock.GRPCErrorConfig{Code: "NOT_FOUND", Message: "nope"},
			},
		},
	}
	gotErr := convertGRPCMethodConfig(withErr)
	require.Len(t, gotErr.Variants, 1)
	require.NotNil(t, gotErr.Variants[0].Error)
	assert.Equal(t, "NOT_FOUND", gotErr.Variants[0].Error.Code)
}

// TestConvertGRPCMethodConfig_SingleConfig_NoVariants verifies backward-compat:
// a single config with no variants converts cleanly and produces no variants.
func TestConvertGRPCMethodConfig_SingleConfig_NoVariants(t *testing.T) {
	src := mock.MethodConfig{
		Match:    &mock.MethodMatch{Request: map[string]any{"id": "123"}},
		Response: map[string]any{"id": "123"},
	}

	got := convertGRPCMethodConfig(src)

	require.NotNil(t, got.Match)
	assert.Equal(t, "123", got.Match.Request["id"])
	assert.Empty(t, got.Variants)
}
