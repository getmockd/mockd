package engineclient

import (
	"errors"

	types "github.com/getmockd/mockd/pkg/api/types"
	"github.com/getmockd/mockd/pkg/store"
)

// Sentinel errors for engine client operations.
var (
	// ErrNotFound is an alias for store.ErrNotFound so that errors.Is works
	// consistently across packages.
	ErrNotFound = store.ErrNotFound
	// ErrDuplicate is returned when a resource already exists.
	ErrDuplicate = errors.New("resource already exists")
	// ErrConflict is returned when a create/update conflicts with existing state (409).
	ErrConflict = errors.New("conflict")
	// ErrCapacity is returned when a stateful resource is at max capacity (507).
	ErrCapacity = errors.New("capacity exceeded")
)

// Type aliases that point to the canonical shared types in pkg/api/types/.
// These exist for backward compatibility so existing code using engineclient.X
// types continues to compile.
type (
	DeployRequest    = types.DeployRequest
	DeployResponse   = types.DeployResponse
	StatusResponse   = types.StatusResponse
	ProtocolStatus   = types.ProtocolStatus
	MockListResponse = types.MockListResponse
	// RequestFilter is kept for backward compatibility; use requestlog.Filter directly.
	RequestFilter         = types.RequestLogFilter
	RequestLogEntry       = types.RequestLogEntry
	RequestListResponse   = types.RequestListResponse
	ErrorResponse         = types.ErrorResponse
	ChaosConfig           = types.ChaosConfig
	LatencyConfig         = types.LatencyConfig
	ErrorRateConfig       = types.ErrorRateConfig
	BandwidthConfig       = types.BandwidthConfig
	ChaosRuleConfig       = types.ChaosRuleConfig
	ChaosStats            = types.ChaosStats
	StatefulResource      = types.StatefulResource
	StatefulItemsResponse = types.StatefulItemsResponse
	StateOverview         = types.StateOverview
	ProtocolHandler       = types.ProtocolHandler
	SSEConnection         = types.SSEConnection
	SSEStats              = types.SSEStats
	CustomOperationInfo   = types.CustomOperationInfo
	CustomOperationDetail = types.CustomOperationDetail
	CustomOperationStep   = types.CustomOperationStep
)
