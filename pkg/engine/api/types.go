package api

import (
	types "github.com/getmockd/mockd/pkg/api/types"
)

// Type aliases that point to the canonical shared types in pkg/api/types/.
// These exist so that the EngineController interface and handlers can use
// short names without a package qualifier within this package.
type (
	ProtocolStatus      = types.ProtocolStatus
	StatusResponse      = types.StatusResponse
	HealthResponse      = types.HealthResponse
	ErrorResponse       = types.ErrorResponse
	MockListResponse    = types.MockListResponse
	DeployRequest       = types.DeployRequest
	DeployResponse      = types.DeployResponse
	RequestLogEntry     = types.RequestLogEntry
	RequestListResponse = types.RequestListResponse
	// RequestLogFilter is kept for backward compatibility; use requestlog.Filter directly.
	RequestLogFilter                = types.RequestLogFilter
	ToggleMockRequest               = types.ToggleRequest
	ImportConfigRequest             = types.ImportConfigRequest
	ChaosConfig                     = types.ChaosConfig
	LatencyConfig                   = types.LatencyConfig
	ErrorRateConfig                 = types.ErrorRateConfig
	BandwidthConfig                 = types.BandwidthConfig
	ChaosRuleConfig                 = types.ChaosRuleConfig
	ChaosFaultConfig                = types.ChaosFaultConfig
	ChaosStats                      = types.ChaosStats
	StatefulResource                = types.StatefulResource
	StatefulItemsResponse           = types.StatefulItemsResponse
	StateOverview                   = types.StateOverview
	ResetStateRequest               = types.ResetStateRequest
	ResetStateResponse              = types.ResetStateResponse
	ProtocolHandler                 = types.ProtocolHandler
	ProtocolHandlerListResponse     = types.ProtocolHandlerListResponse
	SSEConnection                   = types.SSEConnection
	SSEConnectionListResponse       = types.SSEConnectionListResponse
	SSEStats                        = types.SSEStats
	WebSocketConnection             = types.WebSocketConnection
	WebSocketConnectionListResponse = types.WebSocketConnectionListResponse
	WebSocketStats                  = types.WebSocketStats
	ConfigResponse                  = types.ConfigResponse
	CustomOperationInfo             = types.CustomOperationInfo
	CustomOperationDetail           = types.CustomOperationDetail
	CustomOperationStep             = types.CustomOperationStep
	StatefulFaultStats              = types.StatefulFaultStats
	CircuitBreakerStatus            = types.CircuitBreakerStatus
	RetryAfterStatus                = types.RetryAfterStatus
	ProgressiveDegradationStatus    = types.ProgressiveDegradationStatus
)

// ProtocolStatusInfo is an alias for ProtocolStatus for backward compatibility.
type ProtocolStatusInfo = types.ProtocolStatus
