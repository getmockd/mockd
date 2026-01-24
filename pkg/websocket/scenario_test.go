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

func TestScenarioState_ConcurrentAdvance(t *testing.T) {
	// This test verifies that ScenarioState is safe for concurrent access.
	// Run with -race flag to detect race conditions.
	cfg := &ScenarioConfig{
		Name: "concurrent-test",
		Steps: []*ScenarioStepConfig{
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "1"}},
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "2"}},
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "3"}},
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "4"}},
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "5"}},
		},
		Loop: true,
	}

	s, err := NewScenario(cfg)
	require.NoError(t, err)

	state := NewScenarioState(s)

	// Launch multiple goroutines that all access state concurrently
	const numGoroutines = 50
	const opsPerGoroutine = 100
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			for j := 0; j < opsPerGoroutine; j++ {
				switch j % 5 {
				case 0:
					state.AdvanceStep()
				case 1:
					_ = state.CurrentStep()
				case 2:
					_ = state.Completed()
				case 3:
					_ = state.GetCurrentStepConfig()
				case 4:
					_ = state.Info()
				}
			}
			done <- true
		}()
	}

	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify state is still consistent
	_ = state.CurrentStep()
	_ = state.Completed()
}

func TestScenarioExecutor_HandleMessageChannelSignaling(t *testing.T) {
	// Test that HandleMessage properly signals the matchCh channel
	// and doesn't cause double-advance with Run().

	cfg := &ScenarioConfig{
		Name: "match-test",
		Steps: []*ScenarioStepConfig{
			{Type: "expect", Timeout: Duration(100 * time.Millisecond)},
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "done"}},
		},
	}

	s, err := NewScenario(cfg)
	require.NoError(t, err)

	state := NewScenarioState(s)

	// Create executor manually with channel (normally done via NewScenarioExecutor with a connection)
	exec := &ScenarioExecutor{
		state:   state,
		matchCh: make(chan struct{}, 1),
	}

	// Initial state should be at step 0 (expect)
	assert.Equal(t, 0, state.CurrentStep())

	// Simulate HandleMessage matching
	matched := exec.HandleMessage(MessageText, []byte("test"))
	assert.True(t, matched)

	// Should have advanced to step 1
	assert.Equal(t, 1, state.CurrentStep())

	// matchCh should have a signal
	select {
	case <-exec.matchCh:
		// Expected - signal was sent
	default:
		t.Error("Expected matchCh to have a signal")
	}
}

func TestScenarioExecutor_HandleMessageNoDoubleAdvance(t *testing.T) {
	// Test that when HandleMessage matches, the step doesn't advance twice
	// (once in HandleMessage, once in Run timeout).

	cfg := &ScenarioConfig{
		Name: "no-double-advance",
		Steps: []*ScenarioStepConfig{
			{Type: "expect", Timeout: Duration(50 * time.Millisecond)},
			{Type: "expect", Timeout: Duration(50 * time.Millisecond)},
			{Type: "expect", Timeout: Duration(50 * time.Millisecond)},
		},
	}

	s, err := NewScenario(cfg)
	require.NoError(t, err)

	// Advance through steps using HandleMessage
	state := NewScenarioState(s)
	exec := &ScenarioExecutor{
		state:   state,
		matchCh: make(chan struct{}, 1),
	}

	// Call HandleMessage multiple times rapidly
	for i := 0; i < 3; i++ {
		startStep := state.CurrentStep()
		exec.HandleMessage(MessageText, []byte("test"))

		// Should advance exactly by 1
		endStep := state.CurrentStep()
		if i < 2 {
			assert.Equal(t, startStep+1, endStep, "Step should advance by exactly 1")
		}
	}

	// Should be at step 3 (completed since no loop)
	assert.True(t, state.Completed())
}

func TestScenarioExecutor_HandleMessageMismatch(t *testing.T) {
	// Test that HandleMessage returns false when step is not "expect"
	cfg := &ScenarioConfig{
		Name: "mismatch-test",
		Steps: []*ScenarioStepConfig{
			{Type: "send", Message: &MessageResponse{Type: "text", Value: "hello"}},
			{Type: "expect", Match: &MatchCriteria{Type: "exact", Value: "specific"}},
		},
	}

	s, err := NewScenario(cfg)
	require.NoError(t, err)

	state := NewScenarioState(s)
	exec := &ScenarioExecutor{
		state:   state,
		matchCh: make(chan struct{}, 1),
	}

	// Step 0 is "send", HandleMessage should return false
	matched := exec.HandleMessage(MessageText, []byte("test"))
	assert.False(t, matched)
	assert.Equal(t, 0, state.CurrentStep())

	// Advance to step 1 (expect)
	state.AdvanceStep()

	// Now HandleMessage should work but only if message matches
	matched = exec.HandleMessage(MessageText, []byte("wrong"))
	assert.False(t, matched) // Doesn't match "specific"

	matched = exec.HandleMessage(MessageText, []byte("specific"))
	assert.True(t, matched) // Matches
}
