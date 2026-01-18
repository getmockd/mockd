package websocket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewScenario(t *testing.T) {
	cfg := &ScenarioConfig{
		Name: "test-scenario",
		Steps: []*ScenarioStepConfig{
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "hello"}},
			{Type: "wait", Duration: Duration(100 * time.Millisecond)},
			{Type: "expect", Match: &MatchCriteria{Type: "exact", Value: "ready"}},
		},
		Loop: false,
	}

	s, err := NewScenario(cfg)
	require.NoError(t, err)
	assert.Equal(t, "test-scenario", s.Name())
	assert.Equal(t, 3, s.StepCount())
	assert.False(t, s.Loop())
	assert.True(t, s.ResetOnReconnect())
}

func TestScenarioState_Advance(t *testing.T) {
	cfg := &ScenarioConfig{
		Name: "test",
		Steps: []*ScenarioStepConfig{
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "1"}},
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "2"}},
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "3"}},
		},
	}

	s, err := NewScenario(cfg)
	require.NoError(t, err)

	state := NewScenarioState(s)

	assert.Equal(t, 0, state.CurrentStep())
	assert.False(t, state.Completed())

	state.AdvanceStep()
	assert.Equal(t, 1, state.CurrentStep())
	assert.False(t, state.Completed())

	state.AdvanceStep()
	assert.Equal(t, 2, state.CurrentStep())
	assert.False(t, state.Completed())

	state.AdvanceStep()
	assert.True(t, state.Completed())
}

func TestScenarioState_Loop(t *testing.T) {
	cfg := &ScenarioConfig{
		Name: "loop-test",
		Steps: []*ScenarioStepConfig{
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "1"}},
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "2"}},
		},
		Loop: true,
	}

	s, err := NewScenario(cfg)
	require.NoError(t, err)

	state := NewScenarioState(s)

	state.AdvanceStep()
	assert.Equal(t, 1, state.CurrentStep())

	state.AdvanceStep()
	assert.Equal(t, 0, state.CurrentStep()) // Should loop back
	assert.False(t, state.Completed())      // Never completes
}

func TestScenarioState_Reset(t *testing.T) {
	cfg := &ScenarioConfig{
		Name: "reset-test",
		Steps: []*ScenarioStepConfig{
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "1"}},
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "2"}},
		},
	}

	s, err := NewScenario(cfg)
	require.NoError(t, err)

	state := NewScenarioState(s)
	state.AdvanceStep()
	state.AdvanceStep()
	assert.True(t, state.Completed())

	state.Reset()
	assert.Equal(t, 0, state.CurrentStep())
	assert.False(t, state.Completed())
}

func TestScenarioState_GetCurrentStep(t *testing.T) {
	cfg := &ScenarioConfig{
		Name: "step-test",
		Steps: []*ScenarioStepConfig{
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "first"}},
			{Type: "wait", Duration: Duration(100 * time.Millisecond)},
		},
	}

	s, err := NewScenario(cfg)
	require.NoError(t, err)

	state := NewScenarioState(s)

	step := state.GetCurrentStepConfig()
	require.NotNil(t, step)
	assert.Equal(t, "send", step.stepType)

	state.AdvanceStep()
	step = state.GetCurrentStepConfig()
	require.NotNil(t, step)
	assert.Equal(t, "wait", step.stepType)

	state.AdvanceStep()
	step = state.GetCurrentStepConfig()
	assert.Nil(t, step) // Completed
}

func TestScenarioState_Context(t *testing.T) {
	cfg := &ScenarioConfig{
		Name: "context-test",
		Steps: []*ScenarioStepConfig{
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "test"}},
		},
	}

	s, err := NewScenario(cfg)
	require.NoError(t, err)

	state := NewScenarioState(s)

	state.SetContextValue("key", "value")
	assert.Equal(t, "value", state.GetContextValue("key"))
	assert.Nil(t, state.GetContextValue("nonexistent"))
}

func TestScenarioState_Info(t *testing.T) {
	cfg := &ScenarioConfig{
		Name: "info-test",
		Steps: []*ScenarioStepConfig{
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "1"}},
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "2"}},
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "3"}},
		},
	}

	s, err := NewScenario(cfg)
	require.NoError(t, err)

	state := NewScenarioState(s)
	state.AdvanceStep()

	info := state.Info()
	assert.Equal(t, "info-test", info.Name)
	assert.Equal(t, 1, info.CurrentStep)
	assert.Equal(t, 3, info.TotalSteps)
	assert.False(t, info.Completed)
	assert.False(t, info.StartedAt.IsZero())
}

func TestInvalidScenarioStep(t *testing.T) {
	tests := []struct {
		name string
		step *ScenarioStepConfig
	}{
		{
			name: "send without message",
			step: &ScenarioStepConfig{Type: "send"},
		},
		{
			name: "wait without duration",
			step: &ScenarioStepConfig{Type: "wait"},
		},
		{
			name: "unknown type",
			step: &ScenarioStepConfig{Type: "invalid"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &ScenarioConfig{
				Name:  "test",
				Steps: []*ScenarioStepConfig{tt.step},
			}

			_, err := NewScenario(cfg)
			assert.Error(t, err)
		})
	}
}

func TestScenario_ResetOnReconnect(t *testing.T) {
	resetFalse := false
	cfg := &ScenarioConfig{
		Name: "no-reset",
		Steps: []*ScenarioStepConfig{
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "test"}},
		},
		ResetOnReconnect: &resetFalse,
	}

	s, err := NewScenario(cfg)
	require.NoError(t, err)
	assert.False(t, s.ResetOnReconnect())
}
