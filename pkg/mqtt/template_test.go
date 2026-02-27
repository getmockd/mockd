package mqtt

import (
	"strings"
	"testing"

	templatepkg "github.com/getmockd/mockd/pkg/template"
)

func TestProcessTemplate_BasicVariables(t *testing.T) {
	seq := templatepkg.NewSequenceStore()

	tests := []struct {
		name     string
		tmpl     string
		ctx      *templatepkg.Context
		contains string // substring that must be present
	}{
		{
			name:     "uuid",
			tmpl:     "{{ uuid }}",
			ctx:      &templatepkg.Context{},
			contains: "-", // UUIDs contain hyphens
		},
		{
			name:     "timestamp",
			tmpl:     "{{ timestamp }}",
			ctx:      &templatepkg.Context{},
			contains: "", // any non-empty string
		},
		{
			name: "topic variable",
			tmpl: "{{ topic }}",
			ctx: &templatepkg.Context{
				MQTT: templatepkg.MQTTContext{Topic: "sensors/temp"},
			},
			contains: "sensors/temp",
		},
		{
			name: "clientId variable",
			tmpl: "{{ clientId }}",
			ctx: &templatepkg.Context{
				MQTT: templatepkg.MQTTContext{ClientID: "device-001"},
			},
			contains: "device-001",
		},
		{
			name: "device_id variable",
			tmpl: "{{ device_id }}",
			ctx: &templatepkg.Context{
				MQTT: templatepkg.MQTTContext{DeviceID: "dev-42"},
			},
			contains: "dev-42",
		},
		{
			name: "payload field",
			tmpl: `{{ payload.temperature }}`,
			ctx: &templatepkg.Context{
				MQTT: templatepkg.MQTTContext{
					Payload: map[string]any{"temperature": 23.5},
				},
			},
			contains: "23.5",
		},
		// Note: {1} wildcard substitution in {{ }} template syntax doesn't work
		// because the } in {1} conflicts with the closing }} regex.
		// Wildcards are handled by RenderTopicTemplate for topic names instead.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProcessTemplate(tt.tmpl, tt.ctx, seq)
			if result == "" {
				t.Error("ProcessTemplate returned empty string")
			}
			if tt.contains != "" && !strings.Contains(result, tt.contains) {
				t.Errorf("ProcessTemplate(%q) = %q, want to contain %q", tt.tmpl, result, tt.contains)
			}
		})
	}
}

func TestProcessTemplate_NilContext(t *testing.T) {
	seq := templatepkg.NewSequenceStore()
	result := ProcessTemplate("{{ uuid }}", nil, seq)
	if result == "" {
		t.Error("ProcessTemplate with nil context should still resolve shared variables")
	}
	if !strings.Contains(result, "-") {
		t.Errorf("expected UUID with hyphens, got %q", result)
	}
}

func TestProcessTemplate_FakerTypes(t *testing.T) {
	seq := templatepkg.NewSequenceStore()
	ctx := &templatepkg.Context{}

	// All 34 faker types should resolve through the unified engine
	fakerTypes := []string{
		"faker.name", "faker.firstName", "faker.lastName",
		"faker.email", "faker.phone", "faker.address",
		"faker.company", "faker.uuid", "faker.boolean",
		"faker.word", "faker.sentence",
		"faker.ipv4", "faker.ipv6", "faker.macAddress", "faker.userAgent",
		"faker.creditCard", "faker.creditCardExp", "faker.cvv",
		"faker.currencyCode", "faker.iban",
		"faker.price", "faker.productName", "faker.color", "faker.hexColor",
		"faker.jobTitle", "faker.latitude", "faker.longitude",
		"faker.words", "faker.slug",
		"faker.mimeType", "faker.fileExtension",
		"faker.ssn", "faker.passport",
	}

	for _, ft := range fakerTypes {
		t.Run(ft, func(t *testing.T) {
			tmpl := "{{ " + ft + " }}"
			result := ProcessTemplate(tmpl, ctx, seq)
			if result == "" || result == tmpl {
				t.Errorf("faker type %s was not resolved, got %q", ft, result)
			}
		})
	}
}

func TestProcessTemplate_RandomInts(t *testing.T) {
	seq := templatepkg.NewSequenceStore()
	ctx := &templatepkg.Context{}

	result := ProcessTemplate("{{ random.int(1, 100) }}", ctx, seq)
	if result == "" {
		t.Error("random.int should resolve")
	}
}

func TestProcessTemplate_Sequences(t *testing.T) {
	seq := templatepkg.NewSequenceStore()
	ctx := &templatepkg.Context{}

	r1 := ProcessTemplate(`{{ sequence("test", 1) }}`, ctx, seq)
	r2 := ProcessTemplate(`{{ sequence("test", 1) }}`, ctx, seq)

	if r1 != "1" {
		t.Errorf("first sequence value should be 1, got %q", r1)
	}
	if r2 != "2" {
		t.Errorf("second sequence value should be 2, got %q", r2)
	}
}

func TestNewTemplateContext(t *testing.T) {
	payload := map[string]any{"temp": 22.5}
	wildcards := []string{"device1", "temp"}

	ctx := NewTemplateContext("sensors/device1/temp", "client-1", payload, wildcards)

	if ctx.MQTT.Topic != "sensors/device1/temp" {
		t.Errorf("Topic = %q, want sensors/device1/temp", ctx.MQTT.Topic)
	}
	if ctx.MQTT.ClientID != "client-1" {
		t.Errorf("ClientID = %q, want client-1", ctx.MQTT.ClientID)
	}
	if ctx.MQTT.Payload["temp"] != 22.5 {
		t.Errorf("Payload[temp] = %v, want 22.5", ctx.MQTT.Payload["temp"])
	}
	if len(ctx.MQTT.WildcardVals) != 2 || ctx.MQTT.WildcardVals[0] != "device1" {
		t.Errorf("WildcardVals = %v, want [device1, temp]", ctx.MQTT.WildcardVals)
	}
}

func TestNewDeviceTemplateContext(t *testing.T) {
	ctx := NewDeviceTemplateContext("dev-42", "sensors/temp")

	if ctx.MQTT.DeviceID != "dev-42" {
		t.Errorf("DeviceID = %q, want dev-42", ctx.MQTT.DeviceID)
	}
	if ctx.MQTT.Topic != "sensors/temp" {
		t.Errorf("Topic = %q, want sensors/temp", ctx.MQTT.Topic)
	}
}

func TestExtractWildcardValues(t *testing.T) {
	tests := []struct {
		name     string
		pattern  string
		topic    string
		expected []string
	}{
		{
			name:     "single plus wildcard",
			pattern:  "sensors/+/temperature",
			topic:    "sensors/device1/temperature",
			expected: []string{"device1"},
		},
		{
			name:     "hash wildcard",
			pattern:  "devices/#",
			topic:    "devices/home/living/light",
			expected: []string{"home/living/light"},
		},
		{
			name:     "multiple plus wildcards",
			pattern:  "+/+/data",
			topic:    "region1/device2/data",
			expected: []string{"region1", "device2"},
		},
		{
			name:     "no wildcards",
			pattern:  "sensors/temp",
			topic:    "sensors/temp",
			expected: nil,
		},
		{
			name:     "plus then hash",
			pattern:  "+/#",
			topic:    "a/b/c/d",
			expected: []string{"a", "b/c/d"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractWildcardValues(tt.pattern, tt.topic)
			if len(result) != len(tt.expected) {
				t.Fatalf("ExtractWildcardValues(%q, %q) = %v, want %v", tt.pattern, tt.topic, result, tt.expected)
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("wildcard[%d] = %q, want %q", i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestRenderTopicTemplate(t *testing.T) {
	tests := []struct {
		name      string
		template  string
		wildcards []string
		expected  string
	}{
		{
			name:      "single substitution",
			template:  "commands/{1}/response",
			wildcards: []string{"device1"},
			expected:  "commands/device1/response",
		},
		{
			name:      "multiple substitutions",
			template:  "{1}/data/{2}/response",
			wildcards: []string{"region1", "temp"},
			expected:  "region1/data/temp/response",
		},
		{
			name:      "no placeholders",
			template:  "static/topic",
			wildcards: []string{"ignored"},
			expected:  "static/topic",
		},
		{
			name:      "empty wildcards",
			template:  "commands/{1}/response",
			wildcards: nil,
			expected:  "commands/{1}/response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderTopicTemplate(tt.template, tt.wildcards)
			if result != tt.expected {
				t.Errorf("RenderTopicTemplate(%q, %v) = %q, want %q", tt.template, tt.wildcards, result, tt.expected)
			}
		})
	}
}

func TestProcessTemplate_MixedContent(t *testing.T) {
	seq := templatepkg.NewSequenceStore()
	ctx := &templatepkg.Context{
		MQTT: templatepkg.MQTTContext{
			Topic:    "sensors/living-room/temp",
			ClientID: "sensor-01",
			DeviceID: "dev-99",
			Payload:  map[string]any{"temp": 23.5, "unit": "celsius"},
		},
	}

	tmpl := `{"topic":"{{ topic }}","device":"{{ device_id }}","temp":{{ payload.temp }},"id":"{{ uuid }}"}`
	result := ProcessTemplate(tmpl, ctx, seq)

	if !strings.Contains(result, `"topic":"sensors/living-room/temp"`) {
		t.Errorf("topic not substituted in %q", result)
	}
	if !strings.Contains(result, `"device":"dev-99"`) {
		t.Errorf("device_id not substituted in %q", result)
	}
	if !strings.Contains(result, `"temp":23.5`) {
		t.Errorf("payload.temp not substituted in %q", result)
	}
	// UUID should be present (contains hyphens)
	if strings.Contains(result, "{{ uuid }}") {
		t.Errorf("uuid template was not resolved in %q", result)
	}
}
