package mcp

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/getmockd/mockd/pkg/cli"
)

// =============================================================================
// handleGetChaosConfig Tests
// =============================================================================

func TestHandleGetChaosConfig_Success(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getChaosConfigFn: func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"enabled": true,
				"latency": map[string]interface{}{
					"min":         "100ms",
					"max":         "500ms",
					"probability": 1.0,
				},
			}, nil
		},
		getChaosStatsFn: func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"totalRequests":   float64(42),
				"faultsInjected":  float64(7),
				"latencyInjected": float64(5),
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetChaosConfig(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["enabled"] != true {
		t.Errorf("enabled = %v, want true", parsed["enabled"])
	}

	latency, ok := parsed["latency"].(map[string]interface{})
	if !ok {
		t.Fatal("expected latency map in response")
	}
	if latency["min"] != "100ms" {
		t.Errorf("latency.min = %v, want 100ms", latency["min"])
	}
	if latency["max"] != "500ms" {
		t.Errorf("latency.max = %v, want 500ms", latency["max"])
	}

	stats, ok := parsed["stats"].(map[string]interface{})
	if !ok {
		t.Fatal("expected stats map in response")
	}
	if stats["totalRequests"] != float64(42) {
		t.Errorf("stats.totalRequests = %v, want 42", stats["totalRequests"])
	}
	if stats["faultsInjected"] != float64(7) {
		t.Errorf("stats.faultsInjected = %v, want 7", stats["faultsInjected"])
	}
}

func TestHandleGetChaosConfig_StatsFailure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getChaosConfigFn: func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"enabled": false,
			}, nil
		},
		getChaosStatsFn: func() (map[string]interface{}, error) {
			return nil, fmt.Errorf("stats endpoint unavailable")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetChaosConfig(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success even when stats fail, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["enabled"] != false {
		t.Errorf("enabled = %v, want false", parsed["enabled"])
	}
	if _, ok := parsed["stats"]; ok {
		t.Error("expected no stats key when stats fetch fails")
	}
}

func TestHandleGetChaosConfig_Failure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getChaosConfigFn: func() (map[string]interface{}, error) {
			return nil, &cli.APIError{StatusCode: 500, Message: "internal error"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetChaosConfig(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetChaosConfig() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result on GetChaosConfig failure")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleGetChaosConfig_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	result, err := handleGetChaosConfig(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetChaosConfig() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

func TestHandleGetChaosConfig_WithProfiles(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getChaosConfigFn: func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"enabled": false,
			}, nil
		},
		listChaosProfilesFn: func() ([]cli.ChaosProfileInfo, error) {
			return []cli.ChaosProfileInfo{
				{Name: "slow-api", Description: "200-800ms latency"},
				{Name: "flaky", Description: "30% error rate"},
				{Name: "offline", Description: "100% 503 errors"},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetChaosConfig(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	profiles, ok := parsed["availableProfiles"].([]interface{})
	if !ok {
		t.Fatal("expected availableProfiles array in response")
	}
	if len(profiles) != 3 {
		t.Fatalf("expected 3 profiles, got %d", len(profiles))
	}
	if profiles[0] != "slow-api" {
		t.Errorf("profiles[0] = %v, want slow-api", profiles[0])
	}
	if profiles[1] != "flaky" {
		t.Errorf("profiles[1] = %v, want flaky", profiles[1])
	}
	if profiles[2] != "offline" {
		t.Errorf("profiles[2] = %v, want offline", profiles[2])
	}
}

func TestHandleGetChaosConfig_ProfilesFailure(t *testing.T) {
	t.Parallel()

	// When ListChaosProfiles fails, config should still be returned without profiles
	client := &mockAdminClient{
		getChaosConfigFn: func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"enabled": true,
			}, nil
		},
		listChaosProfilesFn: func() ([]cli.ChaosProfileInfo, error) {
			return nil, fmt.Errorf("profiles not supported")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetChaosConfig(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["enabled"] != true {
		t.Errorf("enabled = %v, want true", parsed["enabled"])
	}
	// availableProfiles should not be present when ListChaosProfiles fails
	if _, ok := parsed["availableProfiles"]; ok {
		t.Error("expected no availableProfiles when ListChaosProfiles fails")
	}
}

// =============================================================================
// handleSetChaosConfig Tests
// =============================================================================

func TestHandleSetChaosConfig_Profile(t *testing.T) {
	t.Parallel()

	appliedProfile := ""
	client := &mockAdminClient{
		applyChaosProfileFn: func(name string) error {
			appliedProfile = name
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"profile": "slow-api"}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if appliedProfile != "slow-api" {
		t.Errorf("applied profile = %q, want %q", appliedProfile, "slow-api")
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["applied"] != true {
		t.Errorf("applied = %v, want true", parsed["applied"])
	}
	if parsed["profile"] != "slow-api" {
		t.Errorf("profile = %v, want slow-api", parsed["profile"])
	}
}

func TestHandleSetChaosConfig_ProfileFailure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		applyChaosProfileFn: func(name string) error {
			return &cli.APIError{StatusCode: 404, Message: "unknown profile: bad-profile"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"profile": "bad-profile"}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result on ApplyChaosProfile failure")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleSetChaosConfig_RawRules(t *testing.T) {
	t.Parallel()

	var capturedConfig map[string]interface{}
	client := &mockAdminClient{
		setChaosConfigFn: func(cfg map[string]interface{}) error {
			capturedConfig = cfg
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	rules := []interface{}{
		map[string]interface{}{
			"probability": 0.5,
			"pathPattern": "/api/.*",
			"faults": []interface{}{
				map[string]interface{}{
					"type":        "latency",
					"probability": 1.0,
					"config":      map[string]interface{}{"min": "100ms", "max": "500ms"},
				},
			},
		},
	}

	args := map[string]interface{}{
		"enabled": true,
		"rules":   rules,
	}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if capturedConfig["enabled"] != true {
		t.Errorf("captured enabled = %v, want true", capturedConfig["enabled"])
	}
	if capturedConfig["rules"] == nil {
		t.Error("expected rules in captured config")
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["applied"] != true {
		t.Errorf("applied = %v, want true", parsed["applied"])
	}
	cfg, ok := parsed["config"].(map[string]interface{})
	if !ok {
		t.Fatal("expected config map in response")
	}
	if cfg["enabled"] != true {
		t.Errorf("config.enabled = %v, want true", cfg["enabled"])
	}
}

func TestHandleSetChaosConfig_RawRulesFailure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		setChaosConfigFn: func(cfg map[string]interface{}) error {
			return &cli.APIError{StatusCode: 400, Message: "invalid rules format"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"rules": []interface{}{},
	}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result on SetChaosConfig failure with rules")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleSetChaosConfig_FixedLatency(t *testing.T) {
	t.Parallel()

	var capturedConfig map[string]interface{}
	client := &mockAdminClient{
		setChaosConfigFn: func(cfg map[string]interface{}) error {
			capturedConfig = cfg
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"enabled":    true,
		"latency_ms": float64(500),
	}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	latency, ok := capturedConfig["latency"].(map[string]interface{})
	if !ok {
		t.Fatal("expected latency in captured config")
	}
	if latency["min"] != "500ms" {
		t.Errorf("latency.min = %v, want 500ms", latency["min"])
	}
	if latency["max"] != "500ms" {
		t.Errorf("latency.max = %v, want 500ms", latency["max"])
	}
	if latency["probability"] != 1.0 {
		t.Errorf("latency.probability = %v, want 1.0", latency["probability"])
	}
}

func TestHandleSetChaosConfig_RangeLatency(t *testing.T) {
	t.Parallel()

	var capturedConfig map[string]interface{}
	client := &mockAdminClient{
		setChaosConfigFn: func(cfg map[string]interface{}) error {
			capturedConfig = cfg
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"latency_min_ms": float64(100),
		"latency_max_ms": float64(1000),
	}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	latency, ok := capturedConfig["latency"].(map[string]interface{})
	if !ok {
		t.Fatal("expected latency in captured config")
	}
	if latency["min"] != "100ms" {
		t.Errorf("latency.min = %v, want 100ms", latency["min"])
	}
	if latency["max"] != "1s" {
		t.Errorf("latency.max = %v, want 1s", latency["max"])
	}
}

func TestHandleSetChaosConfig_OnlyMinLatency(t *testing.T) {
	t.Parallel()

	var capturedConfig map[string]interface{}
	client := &mockAdminClient{
		setChaosConfigFn: func(cfg map[string]interface{}) error {
			capturedConfig = cfg
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"latency_min_ms": float64(200),
	}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	latency, ok := capturedConfig["latency"].(map[string]interface{})
	if !ok {
		t.Fatal("expected latency in captured config")
	}
	// When only min is set, max defaults to min
	if latency["min"] != "200ms" {
		t.Errorf("latency.min = %v, want 200ms", latency["min"])
	}
	if latency["max"] != "200ms" {
		t.Errorf("latency.max = %v, want 200ms (should default to min)", latency["max"])
	}
}

func TestHandleSetChaosConfig_OnlyMaxLatency(t *testing.T) {
	t.Parallel()

	var capturedConfig map[string]interface{}
	client := &mockAdminClient{
		setChaosConfigFn: func(cfg map[string]interface{}) error {
			capturedConfig = cfg
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"latency_max_ms": float64(300),
	}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	latency, ok := capturedConfig["latency"].(map[string]interface{})
	if !ok {
		t.Fatal("expected latency in captured config")
	}
	// When only max is set, min defaults to max
	if latency["min"] != "300ms" {
		t.Errorf("latency.min = %v, want 300ms (should default to max)", latency["min"])
	}
	if latency["max"] != "300ms" {
		t.Errorf("latency.max = %v, want 300ms", latency["max"])
	}
}

func TestHandleSetChaosConfig_ErrorRate(t *testing.T) {
	t.Parallel()

	var capturedConfig map[string]interface{}
	client := &mockAdminClient{
		setChaosConfigFn: func(cfg map[string]interface{}) error {
			capturedConfig = cfg
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"error_rate": float64(0.3),
	}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	errCfg, ok := capturedConfig["errorRate"].(map[string]interface{})
	if !ok {
		t.Fatal("expected errorRate in captured config")
	}
	if errCfg["probability"] != float64(0.3) {
		t.Errorf("errorRate.probability = %v, want 0.3", errCfg["probability"])
	}
	if _, ok := errCfg["statusCodes"]; ok {
		t.Error("expected no statusCodes when error_codes not provided")
	}
}

func TestHandleSetChaosConfig_ErrorRateWithCodes(t *testing.T) {
	t.Parallel()

	var capturedConfig map[string]interface{}
	client := &mockAdminClient{
		setChaosConfigFn: func(cfg map[string]interface{}) error {
			capturedConfig = cfg
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	codes := []interface{}{float64(500), float64(502), float64(503)}
	args := map[string]interface{}{
		"error_rate":  float64(0.2),
		"error_codes": codes,
	}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	errCfg, ok := capturedConfig["errorRate"].(map[string]interface{})
	if !ok {
		t.Fatal("expected errorRate in captured config")
	}
	if errCfg["probability"] != float64(0.2) {
		t.Errorf("errorRate.probability = %v, want 0.2", errCfg["probability"])
	}
	statusCodes, ok := errCfg["statusCodes"].([]interface{})
	if !ok {
		t.Fatal("expected statusCodes in errorRate config")
	}
	if len(statusCodes) != 3 {
		t.Fatalf("expected 3 status codes, got %d", len(statusCodes))
	}
	if statusCodes[0] != float64(500) {
		t.Errorf("statusCodes[0] = %v, want 500", statusCodes[0])
	}
}

func TestHandleSetChaosConfig_Bandwidth(t *testing.T) {
	t.Parallel()

	var capturedConfig map[string]interface{}
	client := &mockAdminClient{
		setChaosConfigFn: func(cfg map[string]interface{}) error {
			capturedConfig = cfg
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"bandwidth_bytes_per_sec": float64(50000),
	}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	bw, ok := capturedConfig["bandwidth"].(map[string]interface{})
	if !ok {
		t.Fatal("expected bandwidth in captured config")
	}
	if bw["bytesPerSecond"] != float64(50000) {
		t.Errorf("bandwidth.bytesPerSecond = %v, want 50000", bw["bytesPerSecond"])
	}
	if bw["probability"] != 1.0 {
		t.Errorf("bandwidth.probability = %v, want 1.0", bw["probability"])
	}
}

func TestHandleSetChaosConfig_DisableChaos(t *testing.T) {
	t.Parallel()

	var capturedConfig map[string]interface{}
	client := &mockAdminClient{
		setChaosConfigFn: func(cfg map[string]interface{}) error {
			capturedConfig = cfg
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"enabled": false,
	}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if capturedConfig["enabled"] != false {
		t.Errorf("captured enabled = %v, want false", capturedConfig["enabled"])
	}

	// Should have no latency, errorRate, or bandwidth when disabled with no other args
	if _, ok := capturedConfig["latency"]; ok {
		t.Error("expected no latency when only disabling chaos")
	}
	if _, ok := capturedConfig["errorRate"]; ok {
		t.Error("expected no errorRate when only disabling chaos")
	}
	if _, ok := capturedConfig["bandwidth"]; ok {
		t.Error("expected no bandwidth when only disabling chaos")
	}
}

func TestHandleSetChaosConfig_CombinedConfig(t *testing.T) {
	t.Parallel()

	var capturedConfig map[string]interface{}
	client := &mockAdminClient{
		setChaosConfigFn: func(cfg map[string]interface{}) error {
			capturedConfig = cfg
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"enabled":                 true,
		"latency_min_ms":          float64(100),
		"latency_max_ms":          float64(500),
		"error_rate":              float64(0.1),
		"error_codes":             []interface{}{float64(503)},
		"bandwidth_bytes_per_sec": float64(10000),
	}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	// Verify all three sections present
	if capturedConfig["enabled"] != true {
		t.Errorf("enabled = %v, want true", capturedConfig["enabled"])
	}

	latency, ok := capturedConfig["latency"].(map[string]interface{})
	if !ok {
		t.Fatal("expected latency in combined config")
	}
	if latency["min"] != "100ms" {
		t.Errorf("latency.min = %v, want 100ms", latency["min"])
	}
	if latency["max"] != "500ms" {
		t.Errorf("latency.max = %v, want 500ms", latency["max"])
	}

	errCfg, ok := capturedConfig["errorRate"].(map[string]interface{})
	if !ok {
		t.Fatal("expected errorRate in combined config")
	}
	if errCfg["probability"] != float64(0.1) {
		t.Errorf("errorRate.probability = %v, want 0.1", errCfg["probability"])
	}

	bw, ok := capturedConfig["bandwidth"].(map[string]interface{})
	if !ok {
		t.Fatal("expected bandwidth in combined config")
	}
	if bw["bytesPerSecond"] != float64(10000) {
		t.Errorf("bandwidth.bytesPerSecond = %v, want 10000", bw["bytesPerSecond"])
	}

	// Verify the result structure
	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["applied"] != true {
		t.Errorf("applied = %v, want true", parsed["applied"])
	}
	cfg, ok := parsed["config"].(map[string]interface{})
	if !ok {
		t.Fatal("expected config in response")
	}
	if cfg["enabled"] != true {
		t.Errorf("config.enabled = %v, want true", cfg["enabled"])
	}
}

func TestHandleSetChaosConfig_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{"enabled": true}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

func TestHandleSetChaosConfig_Failure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		setChaosConfigFn: func(cfg map[string]interface{}) error {
			return fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{
		"enabled":    true,
		"latency_ms": float64(100),
	}
	result, err := handleSetChaosConfig(args, session, server)
	if err != nil {
		t.Fatalf("handleSetChaosConfig() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result on SetChaosConfig failure")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

// =============================================================================
// formatDurationMs Tests
// =============================================================================

func TestFormatDurationMs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input float64
		want  string
	}{
		{500, "500ms"},
		{1000, "1s"},
		{2000, "2s"},
		{1500, "1500ms"},
		{100, "100ms"},
		{3000, "3s"},
		{250, "250ms"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%.0fms", tt.input), func(t *testing.T) {
			t.Parallel()
			got := formatDurationMs(tt.input)
			if got != tt.want {
				t.Errorf("formatDurationMs(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// =============================================================================
// handleResetChaosStats Tests
// =============================================================================

func TestHandleResetChaosStats_Success(t *testing.T) {
	t.Parallel()

	resetCalled := false
	client := &mockAdminClient{
		resetChaosStatsFn: func() error {
			resetCalled = true
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleResetChaosStats(nil, session, server)
	if err != nil {
		t.Fatalf("handleResetChaosStats() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if !resetCalled {
		t.Error("expected ResetChaosStats to be called")
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["reset"] != true {
		t.Errorf("reset = %v, want true", parsed["reset"])
	}
	msg, ok := parsed["message"].(string)
	if !ok || msg == "" {
		t.Error("expected non-empty message in response")
	}
}

func TestHandleResetChaosStats_Failure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		resetChaosStatsFn: func() error {
			return &cli.APIError{StatusCode: 500, Message: "reset failed"}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleResetChaosStats(nil, session, server)
	if err != nil {
		t.Fatalf("handleResetChaosStats() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result on ResetChaosStats failure")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleResetChaosStats_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	result, err := handleResetChaosStats(nil, session, server)
	if err != nil {
		t.Fatalf("handleResetChaosStats() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

// =============================================================================
// handleGetStatefulFaults Tests
// =============================================================================

func TestHandleGetStatefulFaults_Success(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getStatefulFaultsFn: func() (map[string]interface{}, error) {
			return map[string]interface{}{
				"circuitBreakers": map[string]interface{}{
					"0:0": map[string]interface{}{
						"state":        "closed",
						"tripCount":    float64(0),
						"requestCount": float64(10),
					},
				},
				"retryAfter": map[string]interface{}{
					"limitedCount": float64(3),
					"passedCount":  float64(7),
				},
			}, nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetStatefulFaults(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetStatefulFaults() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["circuitBreakers"] == nil {
		t.Error("expected circuitBreakers in response")
	}
	if parsed["retryAfter"] == nil {
		t.Error("expected retryAfter in response")
	}
}

func TestHandleGetStatefulFaults_Failure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		getStatefulFaultsFn: func() (map[string]interface{}, error) {
			return nil, fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	result, err := handleGetStatefulFaults(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetStatefulFaults() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result on GetStatefulFaultStats failure")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleGetStatefulFaults_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	result, err := handleGetStatefulFaults(nil, session, server)
	if err != nil {
		t.Fatalf("handleGetStatefulFaults() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

// =============================================================================
// handleManageCircuitBreaker Tests
// =============================================================================

func TestHandleManageCircuitBreaker_Trip(t *testing.T) {
	t.Parallel()

	trippedKey := ""
	client := &mockAdminClient{
		tripCircuitBreakerFn: func(key string) error {
			trippedKey = key
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "trip", "key": "0:0"}
	result, err := handleManageCircuitBreaker(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCircuitBreaker() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if trippedKey != "0:0" {
		t.Errorf("tripped key = %q, want %q", trippedKey, "0:0")
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["action"] != "tripped" {
		t.Errorf("action = %v, want tripped", parsed["action"])
	}
	if parsed["key"] != "0:0" {
		t.Errorf("key = %v, want 0:0", parsed["key"])
	}
	msg, ok := parsed["message"].(string)
	if !ok || msg == "" {
		t.Error("expected non-empty message")
	}
}

func TestHandleManageCircuitBreaker_Reset(t *testing.T) {
	t.Parallel()

	resetKey := ""
	client := &mockAdminClient{
		resetCircuitBreakerFn: func(key string) error {
			resetKey = key
			return nil
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "reset", "key": "1:0"}
	result, err := handleManageCircuitBreaker(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCircuitBreaker() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", resultText(t, result))
	}

	if resetKey != "1:0" {
		t.Errorf("reset key = %q, want %q", resetKey, "1:0")
	}

	var parsed map[string]interface{}
	resultJSON(t, result, &parsed)

	if parsed["action"] != "reset" {
		t.Errorf("action = %v, want reset", parsed["action"])
	}
	if parsed["key"] != "1:0" {
		t.Errorf("key = %v, want 1:0", parsed["key"])
	}
	msg, ok := parsed["message"].(string)
	if !ok || msg == "" {
		t.Error("expected non-empty message")
	}
}

func TestHandleManageCircuitBreaker_TripFailure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		tripCircuitBreakerFn: func(key string) error {
			return &cli.APIError{StatusCode: 404, Message: "circuit breaker not found: " + key}
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "trip", "key": "99:99"}
	result, err := handleManageCircuitBreaker(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCircuitBreaker() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result on TripCircuitBreaker failure")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleManageCircuitBreaker_ResetFailure(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{
		resetCircuitBreakerFn: func(key string) error {
			return fmt.Errorf("dial tcp: connection refused")
		},
	}

	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "reset", "key": "0:0"}
	result, err := handleManageCircuitBreaker(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCircuitBreaker() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result on ResetCircuitBreaker failure")
	}

	text := resultText(t, result)
	if text == "" {
		t.Error("expected non-empty error message")
	}
}

func TestHandleManageCircuitBreaker_MissingAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"key": "0:0"}
	result, err := handleManageCircuitBreaker(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCircuitBreaker() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing action")
	}

	text := resultText(t, result)
	if text != "action is required (trip or reset)" {
		t.Errorf("error text = %q, want %q", text, "action is required (trip or reset)")
	}
}

func TestHandleManageCircuitBreaker_MissingKey(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "trip"}
	result, err := handleManageCircuitBreaker(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCircuitBreaker() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for missing key")
	}

	text := resultText(t, result)
	if text != "key is required (e.g., \"0:0\")" {
		t.Errorf("error text = %q, want %q", text, "key is required (e.g., \"0:0\")")
	}
}

func TestHandleManageCircuitBreaker_UnknownAction(t *testing.T) {
	t.Parallel()

	client := &mockAdminClient{}
	session := newTestSession(client)
	server := newTestServer(client)

	args := map[string]interface{}{"action": "explode", "key": "0:0"}
	result, err := handleManageCircuitBreaker(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCircuitBreaker() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result for unknown action")
	}

	text := resultText(t, result)
	if text != "invalid action: explode (must be 'trip' or 'reset')" {
		t.Errorf("error text = %q, want %q", text, "invalid action: explode (must be 'trip' or 'reset')")
	}
}

func TestHandleManageCircuitBreaker_NoClient(t *testing.T) {
	t.Parallel()

	session := NewSession()
	session.SetState(SessionStateReady)
	server := newTestServer(nil)

	args := map[string]interface{}{"action": "trip", "key": "0:0"}
	result, err := handleManageCircuitBreaker(args, session, server)
	if err != nil {
		t.Fatalf("handleManageCircuitBreaker() error = %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error result when admin client is nil")
	}

	text := resultText(t, result)
	if text != "admin client not available" {
		t.Errorf("error text = %q, want %q", text, "admin client not available")
	}
}

// Ensure json import is used (compiler satisfaction).
var _ = json.Marshal
