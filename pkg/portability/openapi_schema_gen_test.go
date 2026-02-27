package portability

import (
	"encoding/json"
	"strings"
	"testing"
)

// intPtr returns a pointer to an int value.
func intPtr(v int) *int { return &v }

// floatPtr returns a pointer to a float64 value.
func floatPtr(v float64) *float64 { return &v }

// --- SchemaGenerator core priority chain ---

func TestGenerate_NilSchema(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	if got := gen.Generate(nil); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestGenerate_ExplicitExample(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string", Example: "hello world"}
	got := gen.Generate(schema)
	if got != "hello world" {
		t.Fatalf("expected 'hello world', got %v", got)
	}
}

func TestGenerate_XMockdFaker(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string", XMockdFaker: "email"}
	got, ok := gen.Generate(schema).(string)
	if !ok {
		t.Fatal("expected string")
	}
	if !strings.Contains(got, "@") {
		t.Fatalf("expected email-like string, got %q", got)
	}
}

func TestGenerate_Enum(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string", Enum: []interface{}{"red", "green", "blue"}}
	got := gen.Generate(schema)
	valid := map[interface{}]bool{"red": true, "green": true, "blue": true}
	if !valid[got] {
		t.Fatalf("expected one of red/green/blue, got %v", got)
	}
}

func TestGenerate_DefaultValue(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "integer", Default: 42}
	if got := gen.Generate(schema); got != 42 {
		t.Fatalf("expected 42, got %v", got)
	}
}

func TestGenerate_ExampleTakesPriorityOverEnum(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{
		Type:    "string",
		Example: "winner",
		Enum:    []interface{}{"a", "b", "c"},
	}
	if got := gen.Generate(schema); got != "winner" {
		t.Fatalf("expected 'winner', got %v", got)
	}
}

// --- $ref resolution & cycle detection ---

func TestGenerate_RefResolution(t *testing.T) {
	components := &OpenAPIComponents{
		Schemas: map[string]*Schema{
			"User": {
				Type: "object",
				Properties: map[string]*Schema{
					"name":  {Type: "string"},
					"email": {Type: "string", Format: "email"},
				},
			},
		},
	}
	gen := NewSchemaGenerator(components)
	schema := &Schema{Ref: "#/components/schemas/User"}
	got, ok := gen.Generate(schema).(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T: %v", gen.Generate(schema), gen.Generate(schema))
	}
	if _, exists := got["name"]; !exists {
		t.Fatal("expected 'name' property in resolved object")
	}
	if _, exists := got["email"]; !exists {
		t.Fatal("expected 'email' property in resolved object")
	}
}

func TestGenerate_CycleDetection(t *testing.T) {
	// Schema A references B, B references A → cycle
	components := &OpenAPIComponents{
		Schemas: map[string]*Schema{
			"A": {
				Type: "object",
				Properties: map[string]*Schema{
					"b": {Ref: "#/components/schemas/B"},
				},
			},
			"B": {
				Type: "object",
				Properties: map[string]*Schema{
					"a": {Ref: "#/components/schemas/A"},
				},
			},
		},
	}
	gen := NewSchemaGenerator(components)
	schema := &Schema{Ref: "#/components/schemas/A"}
	// Should not panic or infinite loop; one side will be nil
	got := gen.Generate(schema)
	if got == nil {
		t.Fatal("expected non-nil result even with cycle")
	}
}

func TestGenerate_SelfReferentialCycle(t *testing.T) {
	// TreeNode → children: [TreeNode] — self-referential
	components := &OpenAPIComponents{
		Schemas: map[string]*Schema{
			"TreeNode": {
				Type: "object",
				Properties: map[string]*Schema{
					"value":    {Type: "string"},
					"children": {Type: "array", Items: &Schema{Ref: "#/components/schemas/TreeNode"}},
				},
			},
		},
	}
	gen := NewSchemaGenerator(components)
	schema := &Schema{Ref: "#/components/schemas/TreeNode"}
	// Should not panic
	got := gen.Generate(schema)
	if got == nil {
		t.Fatal("expected non-nil result for self-referential schema")
	}
}

func TestGenerate_UnresolvableRef(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Ref: "#/components/schemas/DoesNotExist"}
	got := gen.Generate(schema)
	if got != nil {
		t.Fatalf("expected nil for unresolvable ref, got %v", got)
	}
}

// --- Composition ---

func TestGenerate_AllOf(t *testing.T) {
	components := &OpenAPIComponents{
		Schemas: map[string]*Schema{
			"Base": {
				Type: "object",
				Properties: map[string]*Schema{
					"id": {Type: "integer"},
				},
			},
		},
	}
	gen := NewSchemaGenerator(components)
	schema := &Schema{
		AllOf: []*Schema{
			{Ref: "#/components/schemas/Base"},
			{
				Type: "object",
				Properties: map[string]*Schema{
					"name": {Type: "string"},
				},
			},
		},
	}
	got, ok := gen.Generate(schema).(map[string]interface{})
	if !ok {
		t.Fatal("expected map from allOf")
	}
	if _, exists := got["id"]; !exists {
		t.Fatal("expected 'id' from Base ref")
	}
	if _, exists := got["name"]; !exists {
		t.Fatal("expected 'name' from inline schema")
	}
}

func TestGenerate_OneOf(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{
		OneOf: []*Schema{
			{Type: "string", Example: "variant-one"},
			{Type: "integer", Example: 99},
		},
	}
	// Should pick first variant
	if got := gen.Generate(schema); got != "variant-one" {
		t.Fatalf("expected 'variant-one', got %v", got)
	}
}

func TestGenerate_AnyOf(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{
		AnyOf: []*Schema{
			{Type: "string", Example: "any-variant"},
			{Type: "integer", Example: 42},
		},
	}
	if got := gen.Generate(schema); got != "any-variant" {
		t.Fatalf("expected 'any-variant', got %v", got)
	}
}

// --- String generation ---

func TestGenerateString_FormatEmail(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string", Format: "email"}
	got, ok := gen.Generate(schema).(string)
	if !ok || !strings.Contains(got, "@") {
		t.Fatalf("expected email, got %q", got)
	}
}

func TestGenerateString_FormatUUID(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string", Format: "uuid"}
	got, ok := gen.Generate(schema).(string)
	if !ok || len(got) != 36 {
		t.Fatalf("expected UUID (36 chars), got %q", got)
	}
}

func TestGenerateString_FormatDateTime(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string", Format: "date-time"}
	got, ok := gen.Generate(schema).(string)
	if !ok || !strings.Contains(got, "T") {
		t.Fatalf("expected RFC3339 datetime, got %q", got)
	}
}

func TestGenerateString_FormatDate(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string", Format: "date"}
	got, ok := gen.Generate(schema).(string)
	if !ok || len(got) != 10 { // "2006-01-02"
		t.Fatalf("expected date (10 chars), got %q", got)
	}
}

func TestGenerateString_FormatIPv4(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string", Format: "ipv4"}
	got, ok := gen.Generate(schema).(string)
	if !ok {
		t.Fatal("expected string")
	}
	parts := strings.Split(got, ".")
	if len(parts) != 4 {
		t.Fatalf("expected 4-part IPv4, got %q", got)
	}
}

func TestGenerateString_FormatURI(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string", Format: "uri"}
	got, ok := gen.Generate(schema).(string)
	if !ok || !strings.HasPrefix(got, "https://") {
		t.Fatalf("expected URI, got %q", got)
	}
}

func TestGenerateString_FormatPassword(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string", Format: "password"}
	got, ok := gen.Generate(schema).(string)
	if !ok || !strings.HasPrefix(got, "P@ss") {
		t.Fatalf("expected password-like string, got %q", got)
	}
}

func TestGenerateString_FormatByte(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string", Format: "byte"}
	if got := gen.Generate(schema); got != "dGVzdA==" {
		t.Fatalf("expected base64 string, got %v", got)
	}
}

func TestGenerateString_MinLength(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string", MinLength: intPtr(20)}
	got, ok := gen.Generate(schema).(string)
	if !ok || len(got) < 20 {
		t.Fatalf("expected string of at least 20 chars, got %q (len=%d)", got, len(got))
	}
}

func TestGenerateString_FieldNameEmail(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string"}
	got, ok := gen.GenerateNamed(schema, "email").(string)
	if !ok || !strings.Contains(got, "@") {
		t.Fatalf("expected email from field name heuristic, got %q", got)
	}
}

func TestGenerateString_FieldNamePhone(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string"}
	got, ok := gen.GenerateNamed(schema, "phone").(string)
	if !ok || !strings.HasPrefix(got, "+") {
		t.Fatalf("expected phone from field name heuristic, got %q", got)
	}
}

func TestGenerateString_FieldNameID(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string"}
	got, ok := gen.GenerateNamed(schema, "id").(string)
	if !ok || len(got) != 36 {
		t.Fatalf("expected UUID from 'id' field name, got %q", got)
	}
}

func TestGenerateString_FieldNameTimestamp(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string"}
	got, ok := gen.GenerateNamed(schema, "created_at").(string)
	if !ok || !strings.Contains(got, "T") {
		t.Fatalf("expected datetime from '_at' field name, got %q", got)
	}
}

func TestGenerateString_FieldNameURL(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string"}
	got, ok := gen.GenerateNamed(schema, "website").(string)
	if !ok || !strings.HasPrefix(got, "https://") {
		t.Fatalf("expected URL from 'website' field name, got %q", got)
	}
}

func TestGenerateString_Plain(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "string"}
	if got := gen.Generate(schema); got != "string" {
		t.Fatalf("expected 'string', got %v", got)
	}
}

// --- Integer generation ---

func TestGenerateInteger_Default(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "integer"}
	got, ok := gen.Generate(schema).(int)
	if !ok {
		t.Fatal("expected int")
	}
	if got < 0 || got > 100 {
		t.Fatalf("expected integer in [0,100], got %d", got)
	}
}

func TestGenerateInteger_WithConstraints(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "integer", Minimum: floatPtr(10), Maximum: floatPtr(20)}
	for i := 0; i < 50; i++ {
		got, ok := gen.Generate(schema).(int)
		if !ok {
			t.Fatal("expected int")
		}
		if got < 10 || got > 20 {
			t.Fatalf("expected integer in [10,20], got %d", got)
		}
	}
}

func TestGenerateInteger_MinEqualsMax(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "integer", Minimum: floatPtr(42), Maximum: floatPtr(42)}
	if got := gen.Generate(schema); got != 42 {
		t.Fatalf("expected 42, got %v", got)
	}
}

func TestGenerateInteger_SwappedMinMax(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "integer", Minimum: floatPtr(50), Maximum: floatPtr(10)}
	got, ok := gen.Generate(schema).(int)
	if !ok {
		t.Fatal("expected int")
	}
	// After swap: [10, 50]
	if got < 10 || got > 50 {
		t.Fatalf("expected integer in [10,50], got %d", got)
	}
}

// --- Number generation ---

func TestGenerateNumber_Default(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "number"}
	got, ok := gen.Generate(schema).(float64)
	if !ok {
		t.Fatal("expected float64")
	}
	if got < 0.0 || got > 100.0 {
		t.Fatalf("expected float in [0,100], got %f", got)
	}
}

func TestGenerateNumber_WithConstraints(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "number", Minimum: floatPtr(1.5), Maximum: floatPtr(3.5)}
	for i := 0; i < 50; i++ {
		got, ok := gen.Generate(schema).(float64)
		if !ok {
			t.Fatal("expected float64")
		}
		if got < 1.5 || got > 3.5 {
			t.Fatalf("expected float in [1.5,3.5], got %f", got)
		}
	}
}

// --- Boolean ---

func TestGenerateBoolean(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "boolean"}
	seenTrue, seenFalse := false, false
	for i := 0; i < 100; i++ {
		got, ok := gen.Generate(schema).(bool)
		if !ok {
			t.Fatal("expected bool")
		}
		if got {
			seenTrue = true
		} else {
			seenFalse = true
		}
	}
	if !seenTrue || !seenFalse {
		t.Fatal("expected both true and false values from boolean generation")
	}
}

// --- Object generation ---

func TestGenerateObject_Empty(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "object"}
	got, ok := gen.Generate(schema).(map[string]interface{})
	if !ok {
		t.Fatal("expected map")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

func TestGenerateObject_WithProperties(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name":  {Type: "string"},
			"age":   {Type: "integer", Minimum: floatPtr(0), Maximum: floatPtr(120)},
			"email": {Type: "string", Format: "email"},
		},
	}
	got, ok := gen.Generate(schema).(map[string]interface{})
	if !ok {
		t.Fatal("expected map")
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 properties, got %d: %v", len(got), got)
	}

	// name should be a string (field-name heuristic → faker name)
	if _, ok := got["name"].(string); !ok {
		t.Fatal("expected 'name' to be a string")
	}
	// age should be an int in [0,120]
	age, ok := got["age"].(int)
	if !ok {
		t.Fatal("expected 'age' to be an int")
	}
	if age < 0 || age > 120 {
		t.Fatalf("expected age in [0,120], got %d", age)
	}
	// email should contain @
	email, ok := got["email"].(string)
	if !ok || !strings.Contains(email, "@") {
		t.Fatalf("expected email, got %q", email)
	}
}

func TestGenerateObject_TypelessWithProperties(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	// No type specified but has properties — should be treated as object
	schema := &Schema{
		Properties: map[string]*Schema{
			"foo": {Type: "string"},
		},
	}
	got, ok := gen.Generate(schema).(map[string]interface{})
	if !ok {
		t.Fatal("expected map for typeless schema with properties")
	}
	if _, exists := got["foo"]; !exists {
		t.Fatal("expected 'foo' property")
	}
}

// --- Array generation ---

func TestGenerateArray_Default(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{
		Type:  "array",
		Items: &Schema{Type: "string"},
	}
	got, ok := gen.Generate(schema).([]interface{})
	if !ok {
		t.Fatal("expected slice")
	}
	if len(got) != 1 { // default count = 1
		t.Fatalf("expected 1 item, got %d", len(got))
	}
}

func TestGenerateArray_MinItems(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{
		Type:     "array",
		Items:    &Schema{Type: "integer"},
		MinItems: intPtr(3),
	}
	got, ok := gen.Generate(schema).([]interface{})
	if !ok {
		t.Fatal("expected slice")
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}
}

func TestGenerateArray_CappedAt3(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{
		Type:     "array",
		Items:    &Schema{Type: "string"},
		MinItems: intPtr(10),
	}
	got, ok := gen.Generate(schema).([]interface{})
	if !ok {
		t.Fatal("expected slice")
	}
	if len(got) != 3 {
		t.Fatalf("expected cap at 3 items, got %d", len(got))
	}
}

func TestGenerateArray_MaxItemsLessThanDefault(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{
		Type:     "array",
		Items:    &Schema{Type: "string"},
		MinItems: intPtr(2),
		MaxItems: intPtr(1),
	}
	got, ok := gen.Generate(schema).([]interface{})
	if !ok {
		t.Fatal("expected slice")
	}
	// maxItems clamps count to 1
	if len(got) != 1 {
		t.Fatalf("expected 1 item (maxItems clamp), got %d", len(got))
	}
}

func TestGenerateArray_NoItems(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	schema := &Schema{Type: "array"}
	got, ok := gen.Generate(schema).([]interface{})
	if !ok {
		t.Fatal("expected slice")
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 placeholder item, got %d", len(got))
	}
	if got[0] != "item" {
		t.Fatalf("expected 'item' placeholder, got %v", got[0])
	}
}

// --- Field-name heuristic coverage ---

func TestStringByFieldName_Coverage(t *testing.T) {
	tests := []struct {
		name     string
		contains string
	}{
		{"email", "@"},
		{"user_email", "@"},
		{"phone", "+"},
		{"mobile", "+"},
		{"name", " "}, // "First Last"
		{"first_name", ""},
		{"last_name", ""},
		{"firstname", ""},
		{"lastname", ""},
		{"address", " "},
		{"company", " "},
		{"url", "https://"},
		{"href", "https://"},
		{"website", "https://"},
		{"ip", "."},
		{"ip_address", "."},
		{"latitude", ""},
		{"longitude", ""},
		{"price", ""},
		{"color", ""},
		{"title", ""},
		{"description", ""},
		{"bio", ""},
		{"id", "-"}, // UUID has dashes
		{"uuid", "-"},
		{"ssn", "-"},
		{"slug", "-"},
		{"created_at", "T"}, // datetime
		{"updated_at", "T"},
		{"timestamp", "T"},
		{"currency", ""},
		{"country", ""},
		{"city", ""},
		{"state", ""},
		{"zip", ""},
		{"postal_code", ""},
		{"username", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringByFieldName(tt.name)
			if got == "" {
				t.Fatalf("stringByFieldName(%q) returned empty string", tt.name)
			}
			if tt.contains != "" && !strings.Contains(got, tt.contains) {
				t.Fatalf("stringByFieldName(%q) = %q, expected to contain %q", tt.name, got, tt.contains)
			}
		})
	}
}

func TestStringByFieldName_UnknownReturnsEmpty(t *testing.T) {
	got := stringByFieldName("xyzzy_unknown_field")
	if got != "" {
		t.Fatalf("expected empty for unknown field, got %q", got)
	}
}

// --- stringByFormat coverage ---

func TestStringByFormat_Coverage(t *testing.T) {
	tests := []struct {
		format   string
		contains string
	}{
		{"email", "@"},
		{"uuid", "-"},
		{"uri", "https://"},
		{"url", "https://"},
		{"hostname", ".example.com"},
		{"ipv4", "."},
		{"ipv6", ":"},
		{"date-time", "T"},
		{"date", "-"},
		{"time", ":"},
		{"phone", "+"},
		{"password", "P@ss"},
		{"byte", "dGVzdA=="},
		{"binary", "48656c6c6f"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			got := stringByFormat(tt.format)
			if got == "" {
				t.Fatalf("stringByFormat(%q) returned empty string", tt.format)
			}
			if tt.contains != "" && !strings.Contains(got, tt.contains) {
				t.Fatalf("stringByFormat(%q) = %q, expected to contain %q", tt.format, got, tt.contains)
			}
		})
	}
}

func TestStringByFormat_UnknownReturnsEmpty(t *testing.T) {
	got := stringByFormat("custom-format-xyz")
	if got != "" {
		t.Fatalf("expected empty for unknown format, got %q", got)
	}
}

// --- fakerByName coverage ---

func TestFakerByName_AllKnown(t *testing.T) {
	names := []string{
		"uuid", "email", "name", "firstName", "lastName",
		"phone", "address", "company", "ipv4", "ipv6",
		"sentence", "word", "slug", "latitude", "longitude",
		"price", "color", "jobTitle", "ssn", "currencyCode", "boolean",
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			got := fakerByName(name)
			if got == "" {
				t.Fatalf("fakerByName(%q) returned empty string", name)
			}
		})
	}
}

func TestFakerByName_UnknownReturnsEmpty(t *testing.T) {
	got := fakerByName("nonexistent_faker")
	if got != "" {
		t.Fatalf("expected empty for unknown faker, got %q", got)
	}
}

// --- formatFloat ---

func TestFormatFloat(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{3.14, "3.14"},
		{0.0, "0"},
		{1.0, "1"},
		{-42.5, "-42.5"},
		{100.0, "100"},
		{1.123456, "1.123456"},
		{99.990000, "99.99"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := formatFloat(tt.input)
			if got != tt.expected {
				t.Fatalf("formatFloat(%f) = %q, expected %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- Helper functions ---

func TestRandomDigits(t *testing.T) {
	got := randomDigits(5)
	if len(got) != 5 {
		t.Fatalf("expected 5 digits, got %q", got)
	}
	for _, c := range got {
		if c < '0' || c > '9' {
			t.Fatalf("expected digit, got %c", c)
		}
	}
}

func TestRandomHex(t *testing.T) {
	got := randomHex(8)
	if len(got) != 8 {
		t.Fatalf("expected 8 hex chars, got %q", got)
	}
	for _, c := range got {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Fatalf("expected hex char, got %c", c)
		}
	}
}

func TestGenerateStringOfLength(t *testing.T) {
	got := generateStringOfLength(15)
	if len(got) != 15 {
		t.Fatalf("expected 15 chars, got %d", len(got))
	}
}

// --- Integration: generateExampleFromSchema delegating to SchemaGenerator ---

func TestGenerateExampleFromSchema_Object(t *testing.T) {
	components := &OpenAPIComponents{
		Schemas: map[string]*Schema{
			"Address": {
				Type: "object",
				Properties: map[string]*Schema{
					"street": {Type: "string"},
					"city":   {Type: "string"},
				},
			},
		},
	}
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name":    {Type: "string"},
			"email":   {Type: "string", Format: "email"},
			"age":     {Type: "integer", Minimum: floatPtr(1), Maximum: floatPtr(99)},
			"active":  {Type: "boolean"},
			"address": {Ref: "#/components/schemas/Address"},
		},
	}

	got := generateExampleFromSchema(schema, components)
	obj, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map, got %T", got)
	}
	if len(obj) != 5 {
		t.Fatalf("expected 5 properties, got %d", len(obj))
	}

	// Verify email has @
	email, ok := obj["email"].(string)
	if !ok || !strings.Contains(email, "@") {
		t.Fatalf("expected email, got %v", obj["email"])
	}

	// Verify age is within range
	age, ok := obj["age"].(int)
	if !ok || age < 1 || age > 99 {
		t.Fatalf("expected age in [1,99], got %v", obj["age"])
	}

	// Verify address is a nested object
	addr, ok := obj["address"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected address to be a map, got %T", obj["address"])
	}
	if _, exists := addr["street"]; !exists {
		t.Fatal("expected 'street' in address")
	}
}

func TestGenerateExampleFromSchema_Array(t *testing.T) {
	schema := &Schema{
		Type: "array",
		Items: &Schema{
			Type: "object",
			Properties: map[string]*Schema{
				"id":   {Type: "string", Format: "uuid"},
				"name": {Type: "string"},
			},
		},
		MinItems: intPtr(2),
	}

	got := generateExampleFromSchema(schema, nil)
	arr, ok := got.([]interface{})
	if !ok {
		t.Fatalf("expected slice, got %T", got)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 items, got %d", len(arr))
	}
	// Each item should be an object
	for i, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("item[%d]: expected map, got %T", i, item)
		}
		id, ok := obj["id"].(string)
		if !ok || len(id) != 36 {
			t.Fatalf("item[%d]: expected UUID id, got %v", i, obj["id"])
		}
	}
}

func TestGenerateExampleFromSchema_UsesEnumAndConstraints(t *testing.T) {
	schema := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"status":   {Type: "string", Enum: []interface{}{"active", "inactive", "pending"}},
			"priority": {Type: "integer", Minimum: floatPtr(1), Maximum: floatPtr(5)},
			"role":     {Type: "string", Default: "user"},
		},
	}

	got := generateExampleFromSchema(schema, nil)
	obj, ok := got.(map[string]interface{})
	if !ok {
		t.Fatal("expected map")
	}

	// status should be one of the enum values
	status := obj["status"]
	validStatuses := map[interface{}]bool{"active": true, "inactive": true, "pending": true}
	if !validStatuses[status] {
		t.Fatalf("expected status to be enum value, got %v", status)
	}

	// priority should be in [1,5]
	priority, ok := obj["priority"].(int)
	if !ok || priority < 1 || priority > 5 {
		t.Fatalf("expected priority in [1,5], got %v", obj["priority"])
	}

	// role should be default "user"
	if obj["role"] != "user" {
		t.Fatalf("expected role to be default 'user', got %v", obj["role"])
	}
}

// --- Full E2E: OpenAPI import with schema-driven bodies ---

func TestOpenAPIImport_SchemaGeneratedBodies(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Test API
  version: "1.0"
paths:
  /users:
    get:
      summary: List users
      responses:
        "200":
          description: Success
          content:
            application/json:
              schema:
                type: array
                items:
                  type: object
                  properties:
                    id:
                      type: string
                      format: uuid
                    email:
                      type: string
                      format: email
                    age:
                      type: integer
                      minimum: 1
                      maximum: 99
    post:
      summary: Create user
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: string
                    format: uuid
                  status:
                    type: string
                    enum: ["active", "pending"]
`
	importer := &OpenAPIImporter{}
	collection, err := importer.Import([]byte(spec))
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if len(collection.Mocks) != 2 {
		t.Fatalf("expected 2 mocks, got %d", len(collection.Mocks))
	}

	// GET /users should return a JSON array
	getMock := collection.Mocks[0]
	if getMock.HTTP.Matcher.Method != "GET" {
		t.Fatalf("expected GET, got %s", getMock.HTTP.Matcher.Method)
	}
	body := getMock.HTTP.Response.Body
	if body == "" {
		t.Fatal("expected non-empty body")
	}
	var arr []interface{}
	if err := json.Unmarshal([]byte(body), &arr); err != nil {
		t.Fatalf("expected JSON array body, got: %s", body)
	}
	if len(arr) == 0 {
		t.Fatal("expected at least 1 item in array")
	}
	item, ok := arr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected object in array, got %T", arr[0])
	}
	// id should be a UUID
	id, ok := item["id"].(string)
	if !ok || len(id) != 36 {
		t.Fatalf("expected UUID id, got %v", item["id"])
	}
	// email should contain @
	email, ok := item["email"].(string)
	if !ok || !strings.Contains(email, "@") {
		t.Fatalf("expected email, got %v", item["email"])
	}

	// POST /users should return a JSON object with status from enum
	postMock := collection.Mocks[1]
	if postMock.HTTP.Matcher.Method != "POST" {
		t.Fatalf("expected POST, got %s", postMock.HTTP.Matcher.Method)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(postMock.HTTP.Response.Body), &obj); err != nil {
		t.Fatalf("expected JSON object body, got: %s", postMock.HTTP.Response.Body)
	}
	status := obj["status"]
	if status != "active" && status != "pending" {
		t.Fatalf("expected status to be 'active' or 'pending', got %v", status)
	}
}

func TestOpenAPIImport_RefResolution_InSchema(t *testing.T) {
	spec := `openapi: "3.0.3"
info:
  title: Ref Test
  version: "1.0"
paths:
  /products:
    get:
      summary: List products
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/Product'
components:
  schemas:
    Product:
      type: object
      properties:
        name:
          type: string
        price:
          type: number
          minimum: 0.01
          maximum: 999.99
`
	importer := &OpenAPIImporter{}
	collection, err := importer.Import([]byte(spec))
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if len(collection.Mocks) != 1 {
		t.Fatalf("expected 1 mock, got %d", len(collection.Mocks))
	}

	body := collection.Mocks[0].HTTP.Response.Body
	var arr []interface{}
	if err := json.Unmarshal([]byte(body), &arr); err != nil {
		t.Fatalf("expected JSON array, got: %s", body)
	}
	if len(arr) == 0 {
		t.Fatal("expected at least 1 product")
	}
	product, ok := arr[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected object, got %T", arr[0])
	}
	// name should be resolved from field-name heuristic
	if _, ok := product["name"].(string); !ok {
		t.Fatal("expected 'name' to be a string")
	}
	// price should be a number in range
	price, ok := product["price"].(float64)
	if !ok {
		t.Fatalf("expected float64 price, got %T: %v", product["price"], product["price"])
	}
	if price < 0.01 || price > 999.99 {
		t.Fatalf("expected price in [0.01, 999.99], got %f", price)
	}
}
