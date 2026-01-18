package websocket

import (
	"time"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// EndpointFromConfig creates an Endpoint from config.WebSocketEndpointConfig.
func EndpointFromConfig(cfg *config.WebSocketEndpointConfig) (*Endpoint, error) {
	if cfg == nil {
		return nil, nil
	}

	endpointCfg := &EndpointConfig{
		Path:               cfg.Path,
		Subprotocols:       cfg.Subprotocols,
		RequireSubprotocol: cfg.RequireSubprotocol,
		MaxMessageSize:     cfg.MaxMessageSize,
		MaxConnections:     cfg.MaxConnections,
		EchoMode:           cfg.EchoMode,
	}

	// Parse idle timeout
	if cfg.IdleTimeout != "" {
		d, err := time.ParseDuration(cfg.IdleTimeout)
		if err != nil {
			return nil, err
		}
		endpointCfg.IdleTimeout = Duration(d)
	}

	// Convert matchers
	for _, mc := range cfg.Matchers {
		matcherCfg := &MatcherConfig{
			NoResponse: mc.NoResponse,
		}

		if mc.Match != nil {
			matcherCfg.Match = &MatchCriteria{
				Type:        mc.Match.Type,
				Value:       mc.Match.Value,
				Path:        mc.Match.Path,
				MessageType: mc.Match.MessageType,
			}
		}

		if mc.Response != nil {
			resp, err := messageResponseFromConfig(mc.Response)
			if err != nil {
				return nil, err
			}
			matcherCfg.Response = resp
		}

		endpointCfg.Matchers = append(endpointCfg.Matchers, matcherCfg)
	}

	// Convert default response
	if cfg.DefaultResponse != nil {
		resp, err := messageResponseFromConfig(cfg.DefaultResponse)
		if err != nil {
			return nil, err
		}
		endpointCfg.DefaultResponse = resp
	}

	// Convert scenario
	if cfg.Scenario != nil {
		scenarioCfg, err := scenarioFromConfig(cfg.Scenario)
		if err != nil {
			return nil, err
		}
		endpointCfg.Scenario = scenarioCfg
	}

	// Convert heartbeat
	if cfg.Heartbeat != nil {
		hbCfg, err := heartbeatFromConfig(cfg.Heartbeat)
		if err != nil {
			return nil, err
		}
		endpointCfg.Heartbeat = hbCfg
	}

	return NewEndpoint(endpointCfg)
}

// messageResponseFromConfig converts mock.WSMessageResponse to MessageResponse.
func messageResponseFromConfig(cfg *mock.WSMessageResponse) (*MessageResponse, error) {
	if cfg == nil {
		return nil, nil
	}

	resp := &MessageResponse{
		Type:  cfg.Type,
		Value: cfg.Value,
	}

	if cfg.Delay != "" {
		d, err := time.ParseDuration(cfg.Delay)
		if err != nil {
			return nil, err
		}
		resp.Delay = Duration(d)
	}

	return resp, nil
}

// scenarioFromConfig converts mock.WSScenarioConfig to ScenarioConfig.
func scenarioFromConfig(cfg *mock.WSScenarioConfig) (*ScenarioConfig, error) {
	if cfg == nil {
		return nil, nil
	}

	scenarioCfg := &ScenarioConfig{
		Name:             cfg.Name,
		Loop:             cfg.Loop,
		ResetOnReconnect: cfg.ResetOnReconnect,
	}

	for i := range cfg.Steps {
		stepCfg, err := scenarioStepFromConfig(&cfg.Steps[i])
		if err != nil {
			return nil, err
		}
		scenarioCfg.Steps = append(scenarioCfg.Steps, stepCfg)
	}

	return scenarioCfg, nil
}

// scenarioStepFromConfig converts mock.WSScenarioStepConfig to ScenarioStepConfig.
func scenarioStepFromConfig(cfg *mock.WSScenarioStepConfig) (*ScenarioStepConfig, error) {
	if cfg == nil {
		return nil, nil
	}

	stepCfg := &ScenarioStepConfig{
		Type:     cfg.Type,
		Optional: cfg.Optional,
	}

	// Convert message
	if cfg.Message != nil {
		msg, err := messageResponseFromConfig(cfg.Message)
		if err != nil {
			return nil, err
		}
		stepCfg.Message = msg
	}

	// Convert match criteria
	if cfg.Match != nil {
		stepCfg.Match = &MatchCriteria{
			Type:        cfg.Match.Type,
			Value:       cfg.Match.Value,
			Path:        cfg.Match.Path,
			MessageType: cfg.Match.MessageType,
		}
	}

	// Parse duration
	if cfg.Duration != "" {
		d, err := time.ParseDuration(cfg.Duration)
		if err != nil {
			return nil, err
		}
		stepCfg.Duration = Duration(d)
	}

	// Parse timeout
	if cfg.Timeout != "" {
		d, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, err
		}
		stepCfg.Timeout = Duration(d)
	}

	return stepCfg, nil
}

// heartbeatFromConfig converts mock.WSHeartbeatConfig to HeartbeatConfig.
func heartbeatFromConfig(cfg *mock.WSHeartbeatConfig) (*HeartbeatConfig, error) {
	if cfg == nil {
		return nil, nil
	}

	hbCfg := &HeartbeatConfig{
		Enabled: cfg.Enabled,
	}

	if cfg.Interval != "" {
		d, err := time.ParseDuration(cfg.Interval)
		if err != nil {
			return nil, err
		}
		hbCfg.Interval = Duration(d)
	}

	if cfg.Timeout != "" {
		d, err := time.ParseDuration(cfg.Timeout)
		if err != nil {
			return nil, err
		}
		hbCfg.Timeout = Duration(d)
	}

	return hbCfg, nil
}
