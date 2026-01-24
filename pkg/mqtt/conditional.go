package mqtt

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"
)

// ConditionalRule represents a single rule within a conditional response
type ConditionalRule struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Priority        int    `json:"priority"`        // Lower = higher priority
	Condition       string `json:"condition"`       // e.g., "$.command == 'on'"
	ResponseTopic   string `json:"responseTopic"`   // Template for response topic
	PayloadTemplate string `json:"payloadTemplate"` // Template for response payload
	DelayMs         int    `json:"delayMs"`         // Delay before response
	Enabled         bool   `json:"enabled"`
}

// ConditionalResponse represents a set of conditional rules for a topic pattern
type ConditionalResponse struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	TriggerPattern  string            `json:"triggerPattern"` // MQTT pattern with wildcards
	Rules           []ConditionalRule `json:"rules"`
	DefaultResponse *MockResponse     `json:"defaultResponse,omitempty"` // If no rules match
	Enabled         bool              `json:"enabled"`
	CreatedAt       int64             `json:"createdAt,omitempty"`
	UpdatedAt       int64             `json:"updatedAt,omitempty"`
}

// ConditionOperator represents the operator in a condition
type ConditionOperator string

const (
	OpEqual            ConditionOperator = "=="
	OpNotEqual         ConditionOperator = "!="
	OpGreaterThan      ConditionOperator = ">"
	OpLessThan         ConditionOperator = "<"
	OpGreaterThanEqual ConditionOperator = ">="
	OpLessThanEqual    ConditionOperator = "<="
	OpContains         ConditionOperator = "contains"
)

// ParsedCondition represents a parsed condition expression
type ParsedCondition struct {
	JSONPath string
	Operator ConditionOperator
	Value    any
}

// ConditionEvaluator evaluates conditions against JSON payloads
type ConditionEvaluator struct {
	cache   map[string]*ParsedCondition
	jpCache map[string]jp.Expr
	mu      sync.RWMutex
}

// NewConditionEvaluator creates a new condition evaluator
func NewConditionEvaluator() *ConditionEvaluator {
	return &ConditionEvaluator{
		cache:   make(map[string]*ParsedCondition),
		jpCache: make(map[string]jp.Expr),
	}
}

// conditionPattern matches condition expressions like "$.command == 'on'" or "$.temperature > 30"
var conditionPattern = regexp.MustCompile(`^\s*(\$[.\[\]\w]+)\s*(==|!=|>=|<=|>|<|contains)\s*(.+)\s*$`)

// ParseCondition parses a condition string into its components
func (e *ConditionEvaluator) ParseCondition(condition string) (*ParsedCondition, error) {
	e.mu.RLock()
	if cached, ok := e.cache[condition]; ok {
		e.mu.RUnlock()
		return cached, nil
	}
	e.mu.RUnlock()

	matches := conditionPattern.FindStringSubmatch(condition)
	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid condition format: %s", condition)
	}

	jsonPath := matches[1]
	operator := ConditionOperator(matches[2])
	valueStr := strings.TrimSpace(matches[3])

	// Parse the value
	value, err := parseConditionValue(valueStr)
	if err != nil {
		return nil, fmt.Errorf("invalid value in condition: %w", err)
	}

	// Validate JSONPath by attempting to parse it
	_, err = jp.ParseString(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("invalid JSONPath: %w", err)
	}

	parsed := &ParsedCondition{
		JSONPath: jsonPath,
		Operator: operator,
		Value:    value,
	}

	e.mu.Lock()
	e.cache[condition] = parsed
	e.mu.Unlock()

	return parsed, nil
}

// parseConditionValue parses a value string into its appropriate type
//
//nolint:unparam // error is always nil but kept for future validation
func parseConditionValue(valueStr string) (any, error) {
	// Check for quoted string
	if (strings.HasPrefix(valueStr, "'") && strings.HasSuffix(valueStr, "'")) ||
		(strings.HasPrefix(valueStr, "\"") && strings.HasSuffix(valueStr, "\"")) {
		return valueStr[1 : len(valueStr)-1], nil
	}

	// Check for boolean
	if valueStr == "true" {
		return true, nil
	}
	if valueStr == "false" {
		return false, nil
	}

	// Check for null
	if valueStr == "null" {
		return nil, nil
	}

	// Try to parse as integer
	if intVal, err := strconv.ParseInt(valueStr, 10, 64); err == nil {
		return intVal, nil
	}

	// Try to parse as float
	if floatVal, err := strconv.ParseFloat(valueStr, 64); err == nil {
		return floatVal, nil
	}

	// Return as string if nothing else matches
	return valueStr, nil
}

// getJSONPath returns a cached JSONPath expression
func (e *ConditionEvaluator) getJSONPath(path string) (jp.Expr, error) {
	e.mu.RLock()
	if cached, ok := e.jpCache[path]; ok {
		e.mu.RUnlock()
		return cached, nil
	}
	e.mu.RUnlock()

	expr, err := jp.ParseString(path)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	e.jpCache[path] = expr
	e.mu.Unlock()

	return expr, nil
}

// Evaluate evaluates a condition against a JSON payload
func (e *ConditionEvaluator) Evaluate(condition string, payload []byte) (bool, error) {
	parsed, err := e.ParseCondition(condition)
	if err != nil {
		return false, err
	}

	return e.EvaluateParsed(parsed, payload)
}

// EvaluateParsed evaluates a pre-parsed condition against a JSON payload
func (e *ConditionEvaluator) EvaluateParsed(parsed *ParsedCondition, payload []byte) (bool, error) {
	// Parse the JSON payload
	var data any
	if err := oj.Unmarshal(payload, &data); err != nil {
		return false, fmt.Errorf("invalid JSON payload: %w", err)
	}

	// Get the JSONPath expression
	expr, err := e.getJSONPath(parsed.JSONPath)
	if err != nil {
		return false, err
	}

	// Extract the value from the payload
	results := expr.Get(data)
	if len(results) == 0 {
		// Path not found - only match if comparing to null
		if parsed.Value == nil && parsed.Operator == OpEqual {
			return true, nil
		}
		if parsed.Value != nil && parsed.Operator == OpNotEqual {
			return true, nil
		}
		return false, nil
	}

	actualValue := results[0]

	// Compare values based on operator
	return compareValues(actualValue, parsed.Operator, parsed.Value)
}

// compareValues compares two values using the specified operator
func compareValues(actual any, operator ConditionOperator, expected any) (bool, error) {
	switch operator {
	case OpEqual:
		return valuesEqual(actual, expected), nil
	case OpNotEqual:
		return !valuesEqual(actual, expected), nil
	case OpGreaterThan:
		return compareNumeric(actual, expected, func(a, b float64) bool { return a > b })
	case OpLessThan:
		return compareNumeric(actual, expected, func(a, b float64) bool { return a < b })
	case OpGreaterThanEqual:
		return compareNumeric(actual, expected, func(a, b float64) bool { return a >= b })
	case OpLessThanEqual:
		return compareNumeric(actual, expected, func(a, b float64) bool { return a <= b })
	case OpContains:
		return containsValue(actual, expected)
	default:
		return false, fmt.Errorf("unsupported operator: %s", operator)
	}
}

// valuesEqual checks if two values are equal
func valuesEqual(a, b any) bool {
	// Handle nil cases
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Convert to comparable types
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)

	// Try numeric comparison first
	aNum, aErr := toFloat64(a)
	bNum, bErr := toFloat64(b)
	if aErr == nil && bErr == nil {
		return aNum == bNum
	}

	// Fall back to string comparison
	return aStr == bStr
}

// compareNumeric performs numeric comparison
func compareNumeric(actual, expected any, cmp func(a, b float64) bool) (bool, error) {
	aNum, err := toFloat64(actual)
	if err != nil {
		return false, fmt.Errorf("cannot compare non-numeric value: %v", actual)
	}

	bNum, err := toFloat64(expected)
	if err != nil {
		return false, fmt.Errorf("cannot compare to non-numeric value: %v", expected)
	}

	return cmp(aNum, bNum), nil
}

// toFloat64 converts a value to float64
func toFloat64(v any) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case uint:
		return float64(val), nil
	case uint64:
		return float64(val), nil
	case uint32:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	case json.Number:
		return val.Float64()
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}

// containsValue checks if actual contains expected (string substring or array element)
func containsValue(actual, expected any) (bool, error) {
	// String contains
	if actualStr, ok := actual.(string); ok {
		expectedStr := fmt.Sprintf("%v", expected)
		return strings.Contains(actualStr, expectedStr), nil
	}

	// Array contains
	if actualSlice, ok := actual.([]any); ok {
		for _, item := range actualSlice {
			if valuesEqual(item, expected) {
				return true, nil
			}
		}
		return false, nil
	}

	return false, fmt.Errorf("contains operator requires string or array, got %T", actual)
}

// ConditionalResponseHandler handles conditional responses for published messages
type ConditionalResponseHandler struct {
	broker               *Broker
	conditionalResponses []*ConditionalResponse
	evaluator            *ConditionEvaluator
	sequences            *SequenceStore
	mu                   sync.RWMutex
}

// NewConditionalResponseHandler creates a new conditional response handler
func NewConditionalResponseHandler(broker *Broker) *ConditionalResponseHandler {
	return &ConditionalResponseHandler{
		broker:               broker,
		conditionalResponses: make([]*ConditionalResponse, 0),
		evaluator:            NewConditionEvaluator(),
		sequences:            NewSequenceStore(),
	}
}

// SetConditionalResponses updates the list of conditional responses
func (h *ConditionalResponseHandler) SetConditionalResponses(responses []*ConditionalResponse) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conditionalResponses = responses
}

// AddConditionalResponse adds a new conditional response
func (h *ConditionalResponseHandler) AddConditionalResponse(resp *ConditionalResponse) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conditionalResponses = append(h.conditionalResponses, resp)
}

// UpdateConditionalResponse updates an existing conditional response
func (h *ConditionalResponseHandler) UpdateConditionalResponse(resp *ConditionalResponse) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, existing := range h.conditionalResponses {
		if existing.ID == resp.ID {
			h.conditionalResponses[i] = resp
			return true
		}
	}
	return false
}

// RemoveConditionalResponse removes a conditional response by ID
func (h *ConditionalResponseHandler) RemoveConditionalResponse(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for i, resp := range h.conditionalResponses {
		if resp.ID == id {
			h.conditionalResponses = append(h.conditionalResponses[:i], h.conditionalResponses[i+1:]...)
			return true
		}
	}
	return false
}

// GetConditionalResponses returns all conditional responses
func (h *ConditionalResponseHandler) GetConditionalResponses() []*ConditionalResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()
	result := make([]*ConditionalResponse, len(h.conditionalResponses))
	copy(result, h.conditionalResponses)
	return result
}

// GetConditionalResponse returns a specific conditional response by ID
func (h *ConditionalResponseHandler) GetConditionalResponse(id string) *ConditionalResponse {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, resp := range h.conditionalResponses {
		if resp.ID == id {
			return resp
		}
	}
	return nil
}

// HandlePublish processes a published message and triggers any matching conditional responses
// Returns true if a conditional response was triggered (so regular responses can be skipped)
func (h *ConditionalResponseHandler) HandlePublish(topic string, payload []byte, clientID string) bool {
	h.mu.RLock()
	responses := make([]*ConditionalResponse, len(h.conditionalResponses))
	copy(responses, h.conditionalResponses)
	h.mu.RUnlock()

	for _, resp := range responses {
		if !resp.Enabled {
			continue
		}

		if matchTopic(resp.TriggerPattern, topic) {
			if h.executeConditionalResponse(resp, topic, payload, clientID) {
				return true
			}
		}
	}

	return false
}

// executeConditionalResponse evaluates rules and executes the matching one
// Returns true if a rule matched and was executed
func (h *ConditionalResponseHandler) executeConditionalResponse(resp *ConditionalResponse, triggerTopic string, payload []byte, clientID string) bool {
	// Sort rules by priority (lower = higher priority)
	rules := make([]ConditionalRule, len(resp.Rules))
	copy(rules, resp.Rules)
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})

	// Evaluate each rule in priority order
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		matched, err := h.evaluator.Evaluate(rule.Condition, payload)
		if err != nil {
			// Log error but continue to next rule
			continue
		}

		if matched {
			go h.executeRule(&rule, triggerTopic, payload, clientID, resp.TriggerPattern, resp.ID)
			return true
		}
	}

	// No rule matched, use default response if available
	if resp.DefaultResponse != nil && resp.DefaultResponse.Enabled {
		go h.executeDefaultResponse(resp.DefaultResponse, triggerTopic, payload, clientID, resp.TriggerPattern, resp.ID)
		return true
	}

	return false
}

// responseConfig holds the common fields needed to execute a response.
type responseConfig struct {
	responseTopic   string
	payloadTemplate string
	delayMs         int
}

// executeResponse is the common logic for sending a mock response.
// It handles delay, template rendering, publishing, and notifications.
func (h *ConditionalResponseHandler) executeResponse(cfg responseConfig, triggerTopic string, payload []byte, clientID string, triggerPattern string, responseID string) {
	// Apply delay if configured
	if cfg.delayMs > 0 {
		time.Sleep(time.Duration(cfg.delayMs) * time.Millisecond)
	}

	// Extract wildcard values from the trigger topic
	wildcards := ExtractWildcardValues(triggerPattern, triggerTopic)

	// Parse payload as JSON if possible
	var payloadMap map[string]any
	_ = json.Unmarshal(payload, &payloadMap)

	// Build template context
	ctx := &TemplateContext{
		Topic:        triggerTopic,
		WildcardVals: wildcards,
		ClientID:     clientID,
		Payload:      payloadMap,
	}

	// Render response topic
	responseTopic := cfg.responseTopic
	if responseTopic == "" {
		responseTopic = triggerTopic + "/response"
	} else {
		responseTopic = RenderTopicTemplate(responseTopic, wildcards)
	}

	// Render payload template
	template := NewTemplate(cfg.payloadTemplate, h.sequences)
	responsePayload := template.Render(ctx)

	// Publish the response and log any errors
	if err := h.broker.Publish(responseTopic, []byte(responsePayload), 0, false); err != nil {
		h.broker.log.Error("failed to publish mock response",
			"topic", responseTopic,
			"responseID", responseID,
			"error", err)
	}

	// Notify test panel sessions
	h.broker.notifyMockResponse(responseTopic, []byte(responsePayload), responseID)
}

// executeRule sends a response for a matched rule
func (h *ConditionalResponseHandler) executeRule(rule *ConditionalRule, triggerTopic string, payload []byte, clientID string, triggerPattern string, conditionalRespID string) {
	h.executeResponse(responseConfig{
		responseTopic:   rule.ResponseTopic,
		payloadTemplate: rule.PayloadTemplate,
		delayMs:         rule.DelayMs,
	}, triggerTopic, payload, clientID, triggerPattern, conditionalRespID+":"+rule.ID)
}

// executeDefaultResponse sends the default response when no rules match
func (h *ConditionalResponseHandler) executeDefaultResponse(resp *MockResponse, triggerTopic string, payload []byte, clientID string, triggerPattern string, conditionalRespID string) {
	h.executeResponse(responseConfig{
		responseTopic:   resp.ResponseTopic,
		payloadTemplate: resp.PayloadTemplate,
		delayMs:         resp.DelayMs,
	}, triggerTopic, payload, clientID, triggerPattern, conditionalRespID+":default")
}

// ValidateCondition validates a condition string and returns any errors
func (h *ConditionalResponseHandler) ValidateCondition(condition string) error {
	_, err := h.evaluator.ParseCondition(condition)
	return err
}

// ValidateConditionalRule validates a conditional rule
func ValidateConditionalRule(rule *ConditionalRule) []string {
	var errors []string

	if rule.Condition == "" {
		errors = append(errors, "condition is required")
	} else {
		evaluator := NewConditionEvaluator()
		if _, err := evaluator.ParseCondition(rule.Condition); err != nil {
			errors = append(errors, fmt.Sprintf("invalid condition: %v", err))
		}
	}

	if rule.PayloadTemplate == "" {
		errors = append(errors, "payloadTemplate is required")
	}

	return errors
}

// ValidateConditionalResponse validates a conditional response configuration
func ValidateConditionalResponse(resp *ConditionalResponse) []string {
	var errors []string

	if resp.TriggerPattern == "" {
		errors = append(errors, "triggerPattern is required")
	}

	if len(resp.Rules) == 0 && resp.DefaultResponse == nil {
		errors = append(errors, "at least one rule or a default response is required")
	}

	for i, rule := range resp.Rules {
		ruleErrors := ValidateConditionalRule(&rule)
		for _, err := range ruleErrors {
			errors = append(errors, fmt.Sprintf("rule[%d]: %s", i, err))
		}
	}

	return errors
}
