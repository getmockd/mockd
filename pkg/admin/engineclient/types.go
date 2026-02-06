package engineclient

import (
	"errors"

	types "github.com/getmockd/mockd/pkg/api/types"
)

// Sentinel errors for engine client operations.
var (
	// ErrNotFound is returned when a resource is not found.
	ErrNotFound = errors.New("not found")
	// ErrDuplicate is returned when a resource already exists.
	ErrDuplicate = errors.New("resource already exists")
)

// Type aliases that point to the canonical shared types in pkg/api/types/.
// These exist for backward compatibility so existing code using engineclient.X
// types continues to compile.
type (
	DeployRequest       = types.DeployRequest
	DeployResponse      = types.DeployResponse
	StatusResponse      = types.StatusResponse
	ProtocolStatus      = types.ProtocolStatus
	MockListResponse    = types.MockListResponse
	RequestFilter       = types.RequestLogFilter
	RequestLogEntry     = types.RequestLogEntry
	RequestListResponse = types.RequestListResponse
	ErrorResponse       = types.ErrorResponse
	ChaosConfig         = types.ChaosConfig
	LatencyConfig       = types.LatencyConfig
	ErrorRateConfig     = types.ErrorRateConfig
	BandwidthConfig     = types.BandwidthConfig
	ChaosRuleConfig     = types.ChaosRuleConfig
	StatefulResource    = types.StatefulResource
	StateOverview       = types.StateOverview
	ProtocolHandler     = types.ProtocolHandler
	SSEConnection       = types.SSEConnection
	SSEStats            = types.SSEStats
)
