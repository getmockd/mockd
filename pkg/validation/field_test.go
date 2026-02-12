package validation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateField_Type(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		validator *FieldValidator
		wantValid bool
		wantCode  string
	}{
		{
			name:      "string type matches",
			value:     "hello",
			validator: &FieldValidator{Type: "string"},
			wantValid: true,
		},
		{
			name:      "string type mismatch",
			value:     123,
			validator: &FieldValidator{Type: "string"},
			wantValid: false,
			wantCode:  ErrCodeType,
		},
		{
			name:      "number type matches float",
			value:     123.45,
			validator: &FieldValidator{Type: "number"},
			wantValid: true,
		},
		{
			name:      "integer type matches whole number",
			value:     float64(42),
			validator: &FieldValidator{Type: "integer"},
			wantValid: true,
		},
		{
			name:      "integer type rejects decimal",
			value:     42.5,
			validator: &FieldValidator{Type: "integer"},
			wantValid: false,
			wantCode:  ErrCodeType,
		},
		{
			name:      "boolean type matches",
			value:     true,
			validator: &FieldValidator{Type: "boolean"},
			wantValid: true,
		},
		{
			name:      "array type matches",
			value:     []interface{}{"a", "b"},
			validator: &FieldValidator{Type: "array"},
			wantValid: true,
		},
		{
			name:      "object type matches",
			value:     map[string]interface{}{"key": "value"},
			validator: &FieldValidator{Type: "object"},
			wantValid: true,
		},
		{
			name:      "null value with nullable allowed",
			value:     nil,
			validator: &FieldValidator{Type: "string", Nullable: true},
			wantValid: true,
		},
		{
			name:      "null value without nullable",
			value:     nil,
			validator: &FieldValidator{Type: "string"},
			wantValid: false,
			wantCode:  ErrCodeType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateField("testField", LocationBody, tt.value, tt.validator)
			assert.Equal(t, tt.wantValid, result.Valid)
			if !tt.wantValid {
				require.NotEmpty(t, result.Errors)
				assert.Equal(t, tt.wantCode, result.Errors[0].Code)
			}
		})
	}
}

func TestValidateField_String(t *testing.T) {
	minLen := 3
	maxLen := 10

	tests := []struct {
		name      string
		value     string
		validator *FieldValidator
		wantValid bool
		wantCode  string
	}{
		{
			name:      "minLength passes",
			value:     "hello",
			validator: &FieldValidator{MinLength: &minLen},
			wantValid: true,
		},
		{
			name:      "minLength fails",
			value:     "hi",
			validator: &FieldValidator{MinLength: &minLen},
			wantValid: false,
			wantCode:  ErrCodeMinLength,
		},
		{
			name:      "maxLength passes",
			value:     "hello",
			validator: &FieldValidator{MaxLength: &maxLen},
			wantValid: true,
		},
		{
			name:      "maxLength fails",
			value:     "hello world!",
			validator: &FieldValidator{MaxLength: &maxLen},
			wantValid: false,
			wantCode:  ErrCodeMaxLength,
		},
		{
			name:      "pattern passes",
			value:     "abc123",
			validator: &FieldValidator{Pattern: `^[a-z]+\d+$`},
			wantValid: true,
		},
		{
			name:      "pattern fails",
			value:     "ABC123",
			validator: &FieldValidator{Pattern: `^[a-z]+\d+$`},
			wantValid: false,
			wantCode:  ErrCodePattern,
		},
		{
			name:      "format email passes",
			value:     "user@example.com",
			validator: &FieldValidator{Format: "email"},
			wantValid: true,
		},
		{
			name:      "format email fails",
			value:     "not-an-email",
			validator: &FieldValidator{Format: "email"},
			wantValid: false,
			wantCode:  ErrCodeFormat,
		},
		{
			name:      "format uuid passes",
			value:     "550e8400-e29b-41d4-a716-446655440000",
			validator: &FieldValidator{Format: "uuid"},
			wantValid: true,
		},
		{
			name:      "format uuid fails",
			value:     "not-a-uuid",
			validator: &FieldValidator{Format: "uuid"},
			wantValid: false,
			wantCode:  ErrCodeFormat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateField("testField", LocationBody, tt.value, tt.validator)
			assert.Equal(t, tt.wantValid, result.Valid)
			if !tt.wantValid {
				require.NotEmpty(t, result.Errors)
				assert.Equal(t, tt.wantCode, result.Errors[0].Code)
			}
		})
	}
}

func TestValidateField_Number(t *testing.T) {
	min := float64(0)
	max := float64(100)
	exclusiveMin := float64(0)
	exclusiveMax := float64(100)

	tests := []struct {
		name      string
		value     float64
		validator *FieldValidator
		wantValid bool
		wantCode  string
	}{
		{
			name:      "min passes",
			value:     50,
			validator: &FieldValidator{Min: &min},
			wantValid: true,
		},
		{
			name:      "min at boundary passes",
			value:     0,
			validator: &FieldValidator{Min: &min},
			wantValid: true,
		},
		{
			name:      "min fails",
			value:     -1,
			validator: &FieldValidator{Min: &min},
			wantValid: false,
			wantCode:  ErrCodeMin,
		},
		{
			name:      "max passes",
			value:     50,
			validator: &FieldValidator{Max: &max},
			wantValid: true,
		},
		{
			name:      "max at boundary passes",
			value:     100,
			validator: &FieldValidator{Max: &max},
			wantValid: true,
		},
		{
			name:      "max fails",
			value:     101,
			validator: &FieldValidator{Max: &max},
			wantValid: false,
			wantCode:  ErrCodeMax,
		},
		{
			name:      "exclusiveMin fails at boundary",
			value:     0,
			validator: &FieldValidator{ExclusiveMin: &exclusiveMin},
			wantValid: false,
			wantCode:  ErrCodeExclusiveMin,
		},
		{
			name:      "exclusiveMax fails at boundary",
			value:     100,
			validator: &FieldValidator{ExclusiveMax: &exclusiveMax},
			wantValid: false,
			wantCode:  ErrCodeExclusiveMax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateField("testField", LocationBody, tt.value, tt.validator)
			assert.Equal(t, tt.wantValid, result.Valid)
			if !tt.wantValid {
				require.NotEmpty(t, result.Errors)
				assert.Equal(t, tt.wantCode, result.Errors[0].Code)
			}
		})
	}
}

func TestValidateField_Array(t *testing.T) {
	minItems := 2
	maxItems := 5

	tests := []struct {
		name      string
		value     []interface{}
		validator *FieldValidator
		wantValid bool
		wantCode  string
	}{
		{
			name:      "minItems passes",
			value:     []interface{}{"a", "b", "c"},
			validator: &FieldValidator{MinItems: &minItems},
			wantValid: true,
		},
		{
			name:      "minItems fails",
			value:     []interface{}{"a"},
			validator: &FieldValidator{MinItems: &minItems},
			wantValid: false,
			wantCode:  ErrCodeMinItems,
		},
		{
			name:      "maxItems passes",
			value:     []interface{}{"a", "b", "c"},
			validator: &FieldValidator{MaxItems: &maxItems},
			wantValid: true,
		},
		{
			name:      "maxItems fails",
			value:     []interface{}{"a", "b", "c", "d", "e", "f"},
			validator: &FieldValidator{MaxItems: &maxItems},
			wantValid: false,
			wantCode:  ErrCodeMaxItems,
		},
		{
			name:      "uniqueItems passes",
			value:     []interface{}{"a", "b", "c"},
			validator: &FieldValidator{UniqueItems: true},
			wantValid: true,
		},
		{
			name:      "uniqueItems fails",
			value:     []interface{}{"a", "b", "a"},
			validator: &FieldValidator{UniqueItems: true},
			wantValid: false,
			wantCode:  ErrCodeUniqueItems,
		},
		{
			name:  "items validation passes",
			value: []interface{}{"hello", "world"},
			validator: &FieldValidator{
				Items: &FieldValidator{Type: "string"},
			},
			wantValid: true,
		},
		{
			name:  "items validation fails",
			value: []interface{}{"hello", 123},
			validator: &FieldValidator{
				Items: &FieldValidator{Type: "string"},
			},
			wantValid: false,
			wantCode:  ErrCodeType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateField("testField", LocationBody, tt.value, tt.validator)
			assert.Equal(t, tt.wantValid, result.Valid)
			if !tt.wantValid {
				require.NotEmpty(t, result.Errors)
				assert.Equal(t, tt.wantCode, result.Errors[0].Code)
			}
		})
	}
}

func TestValidateField_Enum(t *testing.T) {
	tests := []struct {
		name      string
		value     interface{}
		validator *FieldValidator
		wantValid bool
	}{
		{
			name:      "string enum passes",
			value:     "active",
			validator: &FieldValidator{Enum: []interface{}{"active", "inactive", "pending"}},
			wantValid: true,
		},
		{
			name:      "string enum fails",
			value:     "unknown",
			validator: &FieldValidator{Enum: []interface{}{"active", "inactive", "pending"}},
			wantValid: false,
		},
		{
			name:      "number enum passes",
			value:     float64(1),
			validator: &FieldValidator{Enum: []interface{}{float64(1), float64(2), float64(3)}},
			wantValid: true,
		},
		{
			name:      "number enum fails",
			value:     float64(5),
			validator: &FieldValidator{Enum: []interface{}{float64(1), float64(2), float64(3)}},
			wantValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateField("testField", LocationBody, tt.value, tt.validator)
			assert.Equal(t, tt.wantValid, result.Valid)
		})
	}
}

func TestValidateRequired(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string]interface{}
		required  []string
		wantValid bool
		wantCount int
	}{
		{
			name:      "all required present",
			data:      map[string]interface{}{"email": "test@example.com", "name": "Test"},
			required:  []string{"email", "name"},
			wantValid: true,
			wantCount: 0,
		},
		{
			name:      "missing one required",
			data:      map[string]interface{}{"email": "test@example.com"},
			required:  []string{"email", "name"},
			wantValid: false,
			wantCount: 1,
		},
		{
			name:      "missing all required",
			data:      map[string]interface{}{},
			required:  []string{"email", "name"},
			wantValid: false,
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateRequired(LocationBody, tt.data, tt.required)
			assert.Equal(t, tt.wantValid, result.Valid)
			assert.Len(t, result.Errors, tt.wantCount)
		})
	}
}

func TestValidateFields(t *testing.T) {
	t.Run("validates multiple fields", func(t *testing.T) {
		data := map[string]interface{}{
			"email": "test@example.com",
			"age":   float64(25),
		}
		fields := map[string]*FieldValidator{
			"email": {Format: "email"},
			"age":   {Type: "number", Min: ptrFloat(0), Max: ptrFloat(150)},
		}

		result := ValidateFields(LocationBody, data, fields)
		assert.True(t, result.Valid)
	})

	t.Run("reports multiple errors", func(t *testing.T) {
		data := map[string]interface{}{
			"email": "not-an-email",
			"age":   float64(-5),
		}
		fields := map[string]*FieldValidator{
			"email": {Format: "email"},
			"age":   {Type: "number", Min: ptrFloat(0)},
		}

		result := ValidateFields(LocationBody, data, fields)
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 2)
	})
}

func TestValidateString_RuneCount(t *testing.T) {
	ptrInt := func(i int) *int { return &i }

	t.Run("ASCII minLength works normally", func(t *testing.T) {
		validator := &FieldValidator{Type: "string", MinLength: ptrInt(3)}
		result := ValidateField("name", "body", "ab", validator)
		assert.False(t, result.Valid, "2-char ASCII string should fail minLength=3")
	})

	t.Run("ASCII maxLength works normally", func(t *testing.T) {
		validator := &FieldValidator{Type: "string", MaxLength: ptrInt(5)}
		result := ValidateField("name", "body", "abcdef", validator)
		assert.False(t, result.Valid, "6-char ASCII string should fail maxLength=5")
	})

	t.Run("multi-byte chars counted as single rune for minLength", func(t *testing.T) {
		// "café" is 4 runes but 5 bytes (é = 2 bytes in UTF-8)
		validator := &FieldValidator{Type: "string", MinLength: ptrInt(5)}
		result := ValidateField("name", "body", "café", validator)
		assert.False(t, result.Valid, "'café' is 4 runes, should fail minLength=5")

		// But should pass minLength=4
		validator2 := &FieldValidator{Type: "string", MinLength: ptrInt(4)}
		result2 := ValidateField("name", "body", "café", validator2)
		assert.True(t, result2.Valid, "'café' is 4 runes, should pass minLength=4")
	})

	t.Run("multi-byte chars counted as single rune for maxLength", func(t *testing.T) {
		// "日本語" is 3 runes but 9 bytes (each CJK char = 3 bytes in UTF-8)
		validator := &FieldValidator{Type: "string", MaxLength: ptrInt(3)}
		result := ValidateField("name", "body", "日本語", validator)
		assert.True(t, result.Valid, "'日本語' is 3 runes, should pass maxLength=3")

		// But should fail maxLength=2
		validator2 := &FieldValidator{Type: "string", MaxLength: ptrInt(2)}
		result2 := ValidateField("name", "body", "日本語", validator2)
		assert.False(t, result2.Valid, "'日本語' is 3 runes, should fail maxLength=2")
	})

	t.Run("emoji counted as single rune", func(t *testing.T) {
		// "👍" is 1 rune but 4 bytes in UTF-8
		validator := &FieldValidator{Type: "string", MaxLength: ptrInt(1)}
		result := ValidateField("name", "body", "👍", validator)
		assert.True(t, result.Valid, "'👍' is 1 rune, should pass maxLength=1")
	})
}

// Helper function
func ptrFloat(f float64) *float64 {
	return &f
}
