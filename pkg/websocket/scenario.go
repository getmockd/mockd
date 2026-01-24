package websocket

import (
	"context"
	"sync"
	"time"
)

// ScenarioConfig defines configuration for a scripted message sequence.
type ScenarioConfig struct {
	// Name is the scenario name for identification.
	Name string `json:"name"`
	// Steps is the ordered list of scenario steps.
	Steps []*ScenarioStepConfig `json:"steps"`
	// Loop restarts the scenario on completion.
	Loop bool `json:"loop,omitempty"`
	// ResetOnReconnect resets to step 0 on reconnect (default: true).
	ResetOnReconnect *bool `json:"resetOnReconnect,omitempty"`
}

// ScenarioStepConfig defines a single step in a scenario.
type ScenarioStepConfig struct {
	// Type is the step type: "send", "expect", "wait".
	Type string `json:"type"`
	// Message is the message to send (for "send" type).
	Message *MessageResponse `json:"message,omitempty"`
	// Match is the expected message pattern (for "expect" type).
	Match *MatchCriteria `json:"match,omitempty"`
	// Duration is the wait duration (for "wait" type).
	Duration Duration `json:"duration,omitempty"`
	// Timeout is the maximum wait for "expect" (default: 30s).
	Timeout Duration `json:"timeout,omitempty"`
	// Optional can be skipped if timeout reached.
	Optional bool `json:"optional,omitempty"`
}

// Scenario represents a compiled scenario.
type Scenario struct {
	name             string
	steps            []*ScenarioStep
	loop             bool
	resetOnReconnect bool
}

// ScenarioStep is a compiled scenario step.
type ScenarioStep struct {
	stepType string
	message  *MessageResponse
	matcher  *Matcher
	duration time.Duration
	timeout  time.Duration
	optional bool
}

// NewScenario creates a new Scenario from configuration.
func NewScenario(cfg *ScenarioConfig) (*Scenario, error) {
	if cfg == nil {
		return nil, nil
	}

	s := &Scenario{
		name:             cfg.Name,
		loop:             cfg.Loop,
		resetOnReconnect: true, // default
	}

	if cfg.ResetOnReconnect != nil {
		s.resetOnReconnect = *cfg.ResetOnReconnect
	}

	// Compile steps
	for _, stepCfg := range cfg.Steps {
		step, err := newScenarioStep(stepCfg)
		if err != nil {
			return nil, err
		}
		s.steps = append(s.steps, step)
	}

	return s, nil
}

// newScenarioStep creates a compiled ScenarioStep from config.
func newScenarioStep(cfg *ScenarioStepConfig) (*ScenarioStep, error) {
	step := &ScenarioStep{
		stepType: cfg.Type,
		message:  cfg.Message,
		duration: cfg.Duration.Duration(),
		timeout:  cfg.Timeout.Duration(),
		optional: cfg.Optional,
	}

	// Set default timeout
	if step.timeout == 0 {
		step.timeout = 30 * time.Second
	}

	// Validate step type
	switch cfg.Type {
	case "send":
		if cfg.Message == nil {
			return nil, ErrInvalidScenarioStep
		}
	case "expect":
		if cfg.Match != nil {
			m, err := NewMatcher(&MatcherConfig{Match: cfg.Match})
			if err != nil {
				return nil, err
			}
			step.matcher = m
		}
	case "wait":
		if cfg.Duration == 0 {
			return nil, ErrInvalidScenarioStep
		}
	default:
		return nil, ErrInvalidScenarioStep
	}

	return step, nil
}

// Name returns the scenario name.
func (s *Scenario) Name() string {
	return s.name
}

// Steps returns the scenario steps.
func (s *Scenario) Steps() []*ScenarioStep {
	return s.steps
}

// StepCount returns the number of steps.
func (s *Scenario) StepCount() int {
	return len(s.steps)
}

// Loop returns whether the scenario loops.
func (s *Scenario) Loop() bool {
	return s.loop
}

// ResetOnReconnect returns whether the scenario resets on reconnect.
func (s *Scenario) ResetOnReconnect() bool {
	return s.resetOnReconnect
}

// ScenarioState tracks per-connection scenario progress.
type ScenarioState struct {
	scenario      *Scenario
	currentStep   int
	startedAt     time.Time
	stepStartedAt time.Time
	completed     bool
	context       map[string]interface{}
	mu            sync.RWMutex
}

// NewScenarioState creates a new ScenarioState for a scenario.
func NewScenarioState(scenario *Scenario) *ScenarioState {
	now := time.Now()
	return &ScenarioState{
		scenario:      scenario,
		currentStep:   0,
		startedAt:     now,
		stepStartedAt: now,
		context:       make(map[string]interface{}),
	}
}

// Scenario returns the underlying scenario.
func (s *ScenarioState) Scenario() *Scenario {
	return s.scenario
}

// CurrentStep returns the current step index.
func (s *ScenarioState) CurrentStep() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentStep
}

// Completed returns whether the scenario is complete.
func (s *ScenarioState) Completed() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.completed
}

// AdvanceStep moves to the next step.
func (s *ScenarioState) AdvanceStep() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.currentStep++
	s.stepStartedAt = time.Now()

	if s.currentStep >= len(s.scenario.steps) {
		if s.scenario.loop {
			s.currentStep = 0
		} else {
			s.completed = true
		}
	}
}

// Reset resets the scenario to the beginning.
func (s *ScenarioState) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.currentStep = 0
	s.startedAt = now
	s.stepStartedAt = now
	s.completed = false
	s.context = make(map[string]interface{})
}

// GetCurrentStepConfig returns the current step, or nil if complete.
func (s *ScenarioState) GetCurrentStepConfig() *ScenarioStep {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.completed || s.currentStep >= len(s.scenario.steps) {
		return nil
	}
	return s.scenario.steps[s.currentStep]
}

// SetContextValue sets a value in the scenario context.
func (s *ScenarioState) SetContextValue(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.context[key] = value
}

// GetContextValue gets a value from the scenario context.
func (s *ScenarioState) GetContextValue(key string) interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.context[key]
}

// Info returns public information about the scenario state.
func (s *ScenarioState) Info() *ScenarioStateInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return &ScenarioStateInfo{
		Name:        s.scenario.name,
		CurrentStep: s.currentStep,
		TotalSteps:  len(s.scenario.steps),
		Completed:   s.completed,
		StartedAt:   s.startedAt,
	}
}

// ScenarioExecutor runs a scenario on a connection.
type ScenarioExecutor struct {
	conn    *Connection
	state   *ScenarioState
	ctx     context.Context
	cancel  context.CancelFunc
	matchCh chan struct{} // signals when HandleMessage matches an "expect" step
}

// NewScenarioExecutor creates a new ScenarioExecutor.
func NewScenarioExecutor(conn *Connection, scenario *Scenario) *ScenarioExecutor {
	ctx, cancel := context.WithCancel(conn.Context())

	state := NewScenarioState(scenario)
	conn.SetScenarioState(state)

	return &ScenarioExecutor{
		conn:    conn,
		state:   state,
		ctx:     ctx,
		cancel:  cancel,
		matchCh: make(chan struct{}, 1), // buffered to prevent blocking
	}
}

// State returns the scenario state.
func (e *ScenarioExecutor) State() *ScenarioState {
	return e.state
}

// HandleMessage processes an incoming message for the scenario.
// Returns true if the message was consumed by the scenario.
// This method is safe to call concurrently with Run().
func (e *ScenarioExecutor) HandleMessage(msgType MessageType, data []byte) bool {
	step := e.state.GetCurrentStepConfig()
	if step == nil || step.stepType != "expect" {
		return false
	}

	// Check if message matches
	if step.matcher != nil && !step.matcher.Match(msgType, data) {
		return false
	}

	// Message matched - advance to next step
	e.state.AdvanceStep()

	// Signal to Run() that the step was handled (non-blocking)
	select {
	case e.matchCh <- struct{}{}:
	default:
		// Channel already has a signal, no need to add another
	}

	return true
}

// Run executes the scenario.
// This should be called in a goroutine.
func (e *ScenarioExecutor) Run() {
	for {
		select {
		case <-e.ctx.Done():
			return
		default:
		}

		step := e.state.GetCurrentStepConfig()
		if step == nil {
			// Scenario complete
			return
		}

		switch step.stepType {
		case "send":
			if err := e.executeSend(step); err != nil {
				return
			}
			e.state.AdvanceStep()

		case "wait":
			select {
			case <-time.After(step.duration):
				e.state.AdvanceStep()
			case <-e.ctx.Done():
				return
			}

		case "expect":
			// Wait for message via HandleMessage or timeout.
			// HandleMessage will call AdvanceStep() and signal matchCh if a message matches.
			// We only call AdvanceStep() here on timeout for optional steps.
			select {
			case <-e.matchCh:
				// HandleMessage matched and already advanced the step.
				// Continue to next iteration.
			case <-time.After(step.timeout):
				// Timeout reached - check if HandleMessage matched while we were selecting
				select {
				case <-e.matchCh:
					// HandleMessage matched just as we timed out, step already advanced
				default:
					// No match - handle timeout
					if step.optional {
						e.state.AdvanceStep()
					} else {
						// Required step timed out - end scenario
						return
					}
				}
			case <-e.ctx.Done():
				return
			}
		}
	}
}

// executeSend sends the step's message.
func (e *ScenarioExecutor) executeSend(step *ScenarioStep) error {
	if step.message == nil {
		return nil
	}

	// Apply delay if specified
	if step.message.Delay > 0 {
		select {
		case <-time.After(step.message.Delay.Duration()):
		case <-e.ctx.Done():
			return e.ctx.Err()
		}
	}

	data, msgType, err := step.message.GetData()
	if err != nil {
		return err
	}

	return e.conn.Send(msgType, data)
}

// Stop stops the scenario executor.
func (e *ScenarioExecutor) Stop() {
	e.cancel()
}
