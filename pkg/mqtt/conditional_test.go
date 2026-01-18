package mqtt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConditionEvaluator_ParseCondition(t *testing.T) {
	evaluator := NewConditionEvaluator()

	tests := []struct {
		name         string
		condition    string
		wantJSONPath string
		wantOp       ConditionOperator
		wantValue    any
		wantErr      bool
	}{
		{
			name:         "string equality",
			condition:    "$.command == 'on'",
			wantJSONPath: "$.command",
			wantOp:       OpEqual,
			wantValue:    "on",
		},
		{
			name:         "string equality with double quotes",
			condition:    `$.status == "active"`,
			wantJSONPath: "$.status",
			wantOp:       OpEqual,
			wantValue:    "active",
		},
		{
			name:         "numeric greater than",
			condition:    "$.temperature > 30",
			wantJSONPath: "$.temperature",
			wantOp:       OpGreaterThan,
			wantValue:    int64(30),
		},
		{
			name:         "numeric less than",
			condition:    "$.value < 100.5",
			wantJSONPath: "$.value",
			wantOp:       OpLessThan,
			wantValue:    100.5,
		},
		{
			name:         "not equal",
			condition:    "$.status != 'error'",
			wantJSONPath: "$.status",
			wantOp:       OpNotEqual,
			wantValue:    "error",
		},
		{
			name:         "greater than or equal",
			condition:    "$.count >= 10",
			wantJSONPath: "$.count",
			wantOp:       OpGreaterThanEqual,
			wantValue:    int64(10),
		},
		{
			name:         "less than or equal",
			condition:    "$.count <= 5",
			wantJSONPath: "$.count",
			wantOp:       OpLessThanEqual,
			wantValue:    int64(5),
		},
		{
			name:         "contains",
			condition:    "$.message contains 'urgent'",
			wantJSONPath: "$.message",
			wantOp:       OpContains,
			wantValue:    "urgent",
		},
		{
			name:         "nested path",
			condition:    "$.devices[0].online == true",
			wantJSONPath: "$.devices[0].online",
			wantOp:       OpEqual,
			wantValue:    true,
		},
		{
			name:         "boolean false",
			condition:    "$.active == false",
			wantJSONPath: "$.active",
			wantOp:       OpEqual,
			wantValue:    false,
		},
		{
			name:         "null comparison",
			condition:    "$.value == null",
			wantJSONPath: "$.value",
			wantOp:       OpEqual,
			wantValue:    nil,
		},
		{
			name:      "invalid format - no operator",
			condition: "$.command",
			wantErr:   true,
		},
		{
			name:      "invalid format - no path",
			condition: "== 'on'",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := evaluator.ParseCondition(tt.condition)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantJSONPath, parsed.JSONPath)
			assert.Equal(t, tt.wantOp, parsed.Operator)
			assert.Equal(t, tt.wantValue, parsed.Value)
		})
	}
}

func TestConditionEvaluator_Evaluate(t *testing.T) {
	evaluator := NewConditionEvaluator()

	tests := []struct {
		name      string
		condition string
		payload   string
		want      bool
		wantErr   bool
	}{
		// String equality tests
		{
			name:      "string equal - match",
			condition: "$.command == 'on'",
			payload:   `{"command": "on"}`,
			want:      true,
		},
		{
			name:      "string equal - no match",
			condition: "$.command == 'on'",
			payload:   `{"command": "off"}`,
			want:      false,
		},
		{
			name:      "string not equal - match",
			condition: "$.status != 'error'",
			payload:   `{"status": "success"}`,
			want:      true,
		},
		{
			name:      "string not equal - no match",
			condition: "$.status != 'error'",
			payload:   `{"status": "error"}`,
			want:      false,
		},

		// Numeric comparison tests
		{
			name:      "temperature greater than - match",
			condition: "$.temperature > 30",
			payload:   `{"temperature": 35}`,
			want:      true,
		},
		{
			name:      "temperature greater than - no match",
			condition: "$.temperature > 30",
			payload:   `{"temperature": 25}`,
			want:      false,
		},
		{
			name:      "temperature greater than - equal (no match)",
			condition: "$.temperature > 30",
			payload:   `{"temperature": 30}`,
			want:      false,
		},
		{
			name:      "temperature less than - match",
			condition: "$.temperature < 30",
			payload:   `{"temperature": 25}`,
			want:      true,
		},
		{
			name:      "count greater than or equal - equal",
			condition: "$.count >= 10",
			payload:   `{"count": 10}`,
			want:      true,
		},
		{
			name:      "count greater than or equal - greater",
			condition: "$.count >= 10",
			payload:   `{"count": 15}`,
			want:      true,
		},
		{
			name:      "count less than or equal - equal",
			condition: "$.count <= 5",
			payload:   `{"count": 5}`,
			want:      true,
		},
		{
			name:      "float comparison",
			condition: "$.price > 99.99",
			payload:   `{"price": 100.50}`,
			want:      true,
		},

		// Contains tests
		{
			name:      "string contains - match",
			condition: "$.message contains 'urgent'",
			payload:   `{"message": "This is an urgent alert"}`,
			want:      true,
		},
		{
			name:      "string contains - no match",
			condition: "$.message contains 'urgent'",
			payload:   `{"message": "This is a normal message"}`,
			want:      false,
		},

		// Nested path tests
		{
			name:      "nested object - match",
			condition: "$.device.status == 'online'",
			payload:   `{"device": {"status": "online"}}`,
			want:      true,
		},
		{
			name:      "array index - match",
			condition: "$.devices[0].online == true",
			payload:   `{"devices": [{"online": true}, {"online": false}]}`,
			want:      true,
		},
		{
			name:      "array index - no match",
			condition: "$.devices[1].online == true",
			payload:   `{"devices": [{"online": true}, {"online": false}]}`,
			want:      false,
		},

		// Boolean tests
		{
			name:      "boolean true",
			condition: "$.active == true",
			payload:   `{"active": true}`,
			want:      true,
		},
		{
			name:      "boolean false",
			condition: "$.active == false",
			payload:   `{"active": false}`,
			want:      true,
		},

		// Null/missing field tests
		{
			name:      "null comparison - match",
			condition: "$.value == null",
			payload:   `{"value": null}`,
			want:      true,
		},
		{
			name:      "missing field - equal null",
			condition: "$.missing == null",
			payload:   `{"other": "value"}`,
			want:      true,
		},
		{
			name:      "missing field - not equal value",
			condition: "$.missing != 'something'",
			payload:   `{"other": "value"}`,
			want:      true,
		},

		// Error cases
		{
			name:      "invalid JSON",
			condition: "$.command == 'on'",
			payload:   `not valid json`,
			wantErr:   true,
		},
		{
			name:      "numeric comparison on string",
			condition: "$.value > 10",
			payload:   `{"value": "text"}`,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := evaluator.Evaluate(tt.condition, []byte(tt.payload))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestConditionEvaluator_Caching(t *testing.T) {
	evaluator := NewConditionEvaluator()

	condition := "$.command == 'on'"

	// Parse once
	parsed1, err := evaluator.ParseCondition(condition)
	require.NoError(t, err)

	// Parse again - should return cached
	parsed2, err := evaluator.ParseCondition(condition)
	require.NoError(t, err)

	// Should be the same pointer
	assert.Equal(t, parsed1, parsed2)
}

func TestValidateConditionalRule(t *testing.T) {
	tests := []struct {
		name       string
		rule       *ConditionalRule
		wantErrors int
	}{
		{
			name: "valid rule",
			rule: &ConditionalRule{
				Condition:       "$.command == 'on'",
				PayloadTemplate: `{"status": "powered"}`,
			},
			wantErrors: 0,
		},
		{
			name: "missing condition",
			rule: &ConditionalRule{
				PayloadTemplate: `{"status": "powered"}`,
			},
			wantErrors: 1,
		},
		{
			name: "missing payload template",
			rule: &ConditionalRule{
				Condition: "$.command == 'on'",
			},
			wantErrors: 1,
		},
		{
			name: "invalid condition",
			rule: &ConditionalRule{
				Condition:       "invalid condition",
				PayloadTemplate: `{"status": "powered"}`,
			},
			wantErrors: 1,
		},
		{
			name:       "empty rule",
			rule:       &ConditionalRule{},
			wantErrors: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateConditionalRule(tt.rule)
			assert.Equal(t, tt.wantErrors, len(errors))
		})
	}
}

func TestValidateConditionalResponse(t *testing.T) {
	tests := []struct {
		name       string
		resp       *ConditionalResponse
		wantErrors int
	}{
		{
			name: "valid response with rules",
			resp: &ConditionalResponse{
				TriggerPattern: "devices/+/command",
				Rules: []ConditionalRule{
					{
						Condition:       "$.command == 'on'",
						PayloadTemplate: `{"status": "powered"}`,
					},
				},
			},
			wantErrors: 0,
		},
		{
			name: "valid response with default only",
			resp: &ConditionalResponse{
				TriggerPattern: "devices/+/command",
				DefaultResponse: &MockResponse{
					PayloadTemplate: `{"status": "unknown"}`,
				},
			},
			wantErrors: 0,
		},
		{
			name: "missing trigger pattern",
			resp: &ConditionalResponse{
				Rules: []ConditionalRule{
					{
						Condition:       "$.command == 'on'",
						PayloadTemplate: `{"status": "powered"}`,
					},
				},
			},
			wantErrors: 1,
		},
		{
			name: "no rules and no default",
			resp: &ConditionalResponse{
				TriggerPattern: "devices/+/command",
			},
			wantErrors: 1,
		},
		{
			name: "invalid rule in list",
			resp: &ConditionalResponse{
				TriggerPattern: "devices/+/command",
				Rules: []ConditionalRule{
					{
						Condition: "", // invalid
					},
				},
			},
			wantErrors: 2, // condition and payloadTemplate missing
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := ValidateConditionalResponse(tt.resp)
			assert.Equal(t, tt.wantErrors, len(errors))
		})
	}
}

func TestConditionalResponseHandler_Priority(t *testing.T) {
	// Create a broker for testing
	config := &MQTTConfig{
		ID:      "test-broker",
		Port:    0, // Don't actually start
		Enabled: true,
	}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	handler := NewConditionalResponseHandler(broker)

	// Add a response with multiple rules at different priorities
	resp := &ConditionalResponse{
		ID:             "test-resp",
		Name:           "Test Response",
		TriggerPattern: "test/#",
		Rules: []ConditionalRule{
			{
				ID:              "rule-3",
				Name:            "Low Priority",
				Priority:        10,
				Condition:       "$.value > 0",
				PayloadTemplate: `{"matched": "low"}`,
				Enabled:         true,
			},
			{
				ID:              "rule-1",
				Name:            "High Priority",
				Priority:        1,
				Condition:       "$.value > 0",
				PayloadTemplate: `{"matched": "high"}`,
				Enabled:         true,
			},
			{
				ID:              "rule-2",
				Name:            "Medium Priority",
				Priority:        5,
				Condition:       "$.value > 0",
				PayloadTemplate: `{"matched": "medium"}`,
				Enabled:         true,
			},
		},
		Enabled: true,
	}

	handler.AddConditionalResponse(resp)

	responses := handler.GetConditionalResponses()
	require.Len(t, responses, 1)
	require.Len(t, responses[0].Rules, 3)
}

func TestConditionalResponseHandler_CRUD(t *testing.T) {
	config := &MQTTConfig{
		ID:      "test-broker",
		Port:    0,
		Enabled: true,
	}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	handler := NewConditionalResponseHandler(broker)

	// Test Add
	resp := &ConditionalResponse{
		ID:             "test-resp-1",
		Name:           "Test Response",
		TriggerPattern: "test/#",
		Rules: []ConditionalRule{
			{
				ID:              "rule-1",
				Condition:       "$.command == 'on'",
				PayloadTemplate: `{"status": "on"}`,
				Enabled:         true,
			},
		},
		Enabled: true,
	}
	handler.AddConditionalResponse(resp)

	// Test Get
	retrieved := handler.GetConditionalResponse("test-resp-1")
	require.NotNil(t, retrieved)
	assert.Equal(t, "Test Response", retrieved.Name)

	// Test Update
	resp.Name = "Updated Response"
	success := handler.UpdateConditionalResponse(resp)
	assert.True(t, success)

	retrieved = handler.GetConditionalResponse("test-resp-1")
	assert.Equal(t, "Updated Response", retrieved.Name)

	// Test GetAll
	responses := handler.GetConditionalResponses()
	assert.Len(t, responses, 1)

	// Test Remove
	success = handler.RemoveConditionalResponse("test-resp-1")
	assert.True(t, success)

	retrieved = handler.GetConditionalResponse("test-resp-1")
	assert.Nil(t, retrieved)

	// Test Remove non-existent
	success = handler.RemoveConditionalResponse("non-existent")
	assert.False(t, success)
}

func TestConditionalResponseHandler_ValidateCondition(t *testing.T) {
	config := &MQTTConfig{
		ID:      "test-broker",
		Port:    0,
		Enabled: true,
	}
	broker, err := NewBroker(config)
	require.NoError(t, err)

	handler := NewConditionalResponseHandler(broker)

	// Valid condition
	err = handler.ValidateCondition("$.command == 'on'")
	assert.NoError(t, err)

	// Invalid condition
	err = handler.ValidateCondition("invalid")
	assert.Error(t, err)
}

func TestContainsWithArray(t *testing.T) {
	evaluator := NewConditionEvaluator()

	// Test array contains
	payload := `{"tags": ["urgent", "important", "review"]}`

	// This tests the contains operator with arrays
	result, err := evaluator.Evaluate("$.tags contains 'urgent'", []byte(payload))
	require.NoError(t, err)
	assert.True(t, result)

	result, err = evaluator.Evaluate("$.tags contains 'missing'", []byte(payload))
	require.NoError(t, err)
	assert.False(t, result)
}
