package portability

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SchemaGenerator produces realistic example values from JSON Schema definitions.
// It supports format-aware faker mapping, enum selection, composition (allOf/oneOf/anyOf),
// numeric and string constraints, and field-name heuristics.
type SchemaGenerator struct {
	components *OpenAPIComponents
	visited    map[string]bool // Cycle detection for $ref resolution
}

// NewSchemaGenerator creates a new generator with the given components for $ref resolution.
func NewSchemaGenerator(components *OpenAPIComponents) *SchemaGenerator {
	return &SchemaGenerator{
		components: components,
		visited:    make(map[string]bool),
	}
}

// Generate produces an example value for the given schema.
// It follows this priority chain:
//  1. Explicit example value on the schema
//  2. x-mockd-faker vendor extension
//  3. Enum (random pick)
//  4. Default value
//  5. $ref resolution (with cycle detection)
//  6. Composition (allOf, oneOf, anyOf)
//  7. Type-specific generation with format→faker mapping
func (g *SchemaGenerator) Generate(schema *Schema) interface{} {
	return g.generate(schema, "")
}

// GenerateNamed produces an example value, using the property name for heuristics.
func (g *SchemaGenerator) GenerateNamed(schema *Schema, propertyName string) interface{} {
	return g.generate(schema, propertyName)
}

func (g *SchemaGenerator) generate(schema *Schema, propertyName string) interface{} {
	if schema == nil {
		return nil
	}

	// 1. Explicit example
	if schema.Example != nil {
		return schema.Example
	}

	// 2. x-mockd-faker vendor extension
	if schema.XMockdFaker != "" {
		if val := fakerByName(schema.XMockdFaker); val != "" {
			return val
		}
	}

	// 3. Enum — random pick
	if len(schema.Enum) > 0 {
		return schema.Enum[rand.IntN(len(schema.Enum))]
	}

	// 4. Default value
	if schema.Default != nil {
		return schema.Default
	}

	// 5. $ref resolution with cycle detection
	if schema.Ref != "" {
		if g.visited[schema.Ref] {
			return nil // Break cycle
		}
		g.visited[schema.Ref] = true
		defer delete(g.visited, schema.Ref)

		resolved := resolveSchemaRef(schema, g.components)
		if resolved != schema {
			return g.generate(resolved, propertyName)
		}
		return nil
	}

	// 6. Composition
	if len(schema.AllOf) > 0 {
		return g.generateAllOf(schema)
	}
	if len(schema.OneOf) > 0 {
		return g.generate(schema.OneOf[0], propertyName) // Pick first variant
	}
	if len(schema.AnyOf) > 0 {
		return g.generate(schema.AnyOf[0], propertyName) // Pick first variant
	}

	// 7. Type-specific generation
	switch schema.Type {
	case "object":
		return g.generateObject(schema)
	case "array":
		return g.generateArray(schema)
	case "string":
		return g.generateString(schema, propertyName)
	case "integer":
		return g.generateInteger(schema)
	case "number":
		return g.generateNumber(schema)
	case "boolean":
		return rand.IntN(2) == 0

	default:
		// Typeless schema with properties → treat as object
		if len(schema.Properties) > 0 {
			return g.generateObject(schema)
		}
		// Typeless with allOf
		if len(schema.AllOf) > 0 {
			return g.generateAllOf(schema)
		}
		return nil
	}
}

// generateObject produces a map with all properties generated.
func (g *SchemaGenerator) generateObject(schema *Schema) interface{} {
	if len(schema.Properties) == 0 {
		return map[string]interface{}{}
	}

	obj := make(map[string]interface{}, len(schema.Properties))
	for name, prop := range schema.Properties {
		obj[name] = g.generate(prop, name)
	}
	return obj
}

// generateAllOf merges all sub-schemas (only handles object merging).
func (g *SchemaGenerator) generateAllOf(schema *Schema) interface{} {
	merged := make(map[string]interface{})
	for _, sub := range schema.AllOf {
		resolved := resolveSchemaRef(sub, g.components)
		val := g.generate(resolved, "")
		if m, ok := val.(map[string]interface{}); ok {
			for k, v := range m {
				merged[k] = v
			}
		}
	}
	// Also include direct properties from the parent schema
	for name, prop := range schema.Properties {
		merged[name] = g.generate(prop, name)
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

// generateArray produces a slice with the appropriate number of items.
func (g *SchemaGenerator) generateArray(schema *Schema) interface{} {
	count := 1 // Default to 1 item
	if schema.MinItems != nil && *schema.MinItems > count {
		count = *schema.MinItems
	}
	if schema.MaxItems != nil && *schema.MaxItems < count {
		count = *schema.MaxItems
	}
	// Cap at 3 for reasonable example size
	if count > 3 {
		count = 3
	}

	if schema.Items == nil {
		items := make([]interface{}, count)
		for i := range items {
			items[i] = "item"
		}
		return items
	}

	items := make([]interface{}, count)
	for i := range items {
		items[i] = g.generate(schema.Items, "")
	}
	return items
}

// generateString returns a realistic string based on format, property name, or constraints.
func (g *SchemaGenerator) generateString(schema *Schema, propertyName string) interface{} {
	// Format-based generation
	if schema.Format != "" {
		if val := stringByFormat(schema.Format); val != "" {
			return val
		}
	}

	// Property-name heuristic
	if propertyName != "" {
		if val := stringByFieldName(propertyName); val != "" {
			return val
		}
	}

	// Constraint-aware default
	if schema.MinLength != nil && *schema.MinLength > 6 {
		return generateStringOfLength(*schema.MinLength)
	}

	return "string"
}

// generateInteger returns a constrained integer.
func (g *SchemaGenerator) generateInteger(schema *Schema) interface{} {
	lo, hi := 0, 100

	if schema.Minimum != nil {
		lo = int(*schema.Minimum)
	}
	if schema.Maximum != nil {
		hi = int(*schema.Maximum)
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo == hi {
		return lo
	}
	return lo + rand.IntN(hi-lo+1)
}

// generateNumber returns a constrained float.
func (g *SchemaGenerator) generateNumber(schema *Schema) interface{} {
	lo, hi := 0.0, 100.0

	if schema.Minimum != nil {
		lo = *schema.Minimum
	}
	if schema.Maximum != nil {
		hi = *schema.Maximum
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	if lo == hi {
		return lo
	}
	// Generate a float with 2 decimal places
	val := lo + rand.Float64()*(hi-lo)
	return float64(int(val*100)) / 100
}

// stringByFormat maps OpenAPI format strings to faker-generated values.
func stringByFormat(format string) string {
	switch format {
	case "email":
		return fakerByName("email")
	case "uuid":
		return uuid.New().String()
	case "uri", "url":
		return "https://example.com/" + randomSlug()
	case "hostname":
		return randomWord() + ".example.com"
	case "ipv4":
		return fakerByName("ipv4")
	case "ipv6":
		return fakerByName("ipv6")
	case "date-time":
		return time.Now().UTC().Format(time.RFC3339)
	case "date":
		return time.Now().UTC().Format("2006-01-02")
	case "time":
		return time.Now().UTC().Format("15:04:05Z")
	case "phone":
		return fakerByName("phone")
	case "password":
		return "P@ss" + randomWord() + "42!"
	case "byte":
		return "dGVzdA==" // base64("test")
	case "binary":
		return "48656c6c6f" // hex("Hello")
	default:
		return ""
	}
}

// stringByFieldName maps common property names to realistic values.
//
//nolint:gocyclo // Large switch for heuristic mapping is clearer than splitting.
func stringByFieldName(name string) string {
	lower := strings.ToLower(name)

	switch {
	case lower == "email" || strings.HasSuffix(lower, "_email") || strings.HasSuffix(lower, "email"):
		return fakerByName("email")
	case lower == "phone" || lower == "mobile" || lower == "tel" ||
		strings.HasSuffix(lower, "_phone") || strings.HasSuffix(lower, "phone"):
		return fakerByName("phone")
	case lower == "name" || lower == "full_name" || lower == "fullname":
		return fakerByName("name")
	case lower == "first_name" || lower == "firstname" || lower == "given_name":
		return fakerByName("firstName")
	case lower == "last_name" || lower == "lastname" || lower == "surname" || lower == "family_name":
		return fakerByName("lastName")
	case lower == "address" || lower == "street" || lower == "street_address":
		return fakerByName("address")
	case lower == "company" || lower == "organization" || lower == "org":
		return fakerByName("company")
	case lower == "url" || lower == "uri" || lower == "href" || lower == "link" || lower == "website":
		return "https://example.com/" + randomSlug()
	case lower == "ip" || lower == "ip_address" || lower == "ipaddress":
		return fakerByName("ipv4")
	case lower == "latitude" || lower == "lat":
		return fakerByName("latitude")
	case lower == "longitude" || lower == "lng" || lower == "lon":
		return fakerByName("longitude")
	case lower == "price" || lower == "amount" || lower == "cost" || lower == "total":
		return fakerByName("price")
	case lower == "color" || lower == "colour":
		return fakerByName("color")
	case lower == "title" || lower == "job_title" || lower == "jobtitle":
		return fakerByName("jobTitle")
	case lower == "description" || lower == "bio" || lower == "summary" || lower == "about":
		return fakerByName("sentence")
	case lower == "id" || lower == "uuid":
		return uuid.New().String()
	case lower == "ssn":
		return fakerByName("ssn")
	case lower == "slug":
		return fakerByName("slug")
	case strings.HasSuffix(lower, "_at") || lower == "created" || lower == "updated" ||
		strings.HasSuffix(lower, "date") || lower == "timestamp":
		return time.Now().UTC().Format(time.RFC3339)
	case lower == "currency" || lower == "currency_code":
		return fakerByName("currencyCode")
	case lower == "country":
		countries := []string{"US", "GB", "CA", "DE", "FR", "JP", "AU"}
		return countries[rand.IntN(len(countries))]
	case lower == "city":
		cities := []string{"New York", "Los Angeles", "Chicago", "Houston", "Phoenix",
			"San Francisco", "Seattle", "Austin", "Denver", "Boston"}
		return cities[rand.IntN(len(cities))]
	case lower == "state" || lower == "province":
		states := []string{"California", "Texas", "New York", "Florida", "Illinois",
			"Washington", "Colorado", "Massachusetts"}
		return states[rand.IntN(len(states))]
	case lower == "zip" || lower == "zipcode" || lower == "zip_code" || lower == "postal_code" || lower == "postalcode":
		return randomDigits(5)
	case lower == "username" || lower == "user_name" || lower == "login":
		return strings.ToLower(fakerByName("firstName")) + randomDigits(2)
	}

	return ""
}

// fakerByName calls the appropriate faker function by short name.
// This mirrors the template engine's resolveFaker() but returns the value directly.
func fakerByName(name string) string {
	// Import the template engine's faker functions indirectly
	// to avoid circular imports. We use a local mapping to the
	// most common faker data patterns.
	switch name {
	case "uuid":
		return uuid.New().String()
	case "email":
		prefixes := []string{"john", "jane", "alex", "maria", "dev", "test", "user"}
		domains := []string{"example.com", "test.io", "demo.org"}
		return prefixes[rand.IntN(len(prefixes))] + "." +
			prefixes[rand.IntN(len(prefixes))] + "@" +
			domains[rand.IntN(len(domains))]
	case "name":
		first := []string{"John", "Jane", "Alex", "Maria", "Sam", "Taylor", "Jordan"}
		last := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller"}
		return first[rand.IntN(len(first))] + " " + last[rand.IntN(len(last))]
	case "firstName":
		names := []string{"John", "Jane", "Alex", "Maria", "Sam", "Taylor", "Jordan", "Morgan"}
		return names[rand.IntN(len(names))]
	case "lastName":
		names := []string{"Smith", "Johnson", "Williams", "Brown", "Jones", "Garcia", "Miller", "Davis"}
		return names[rand.IntN(len(names))]
	case "phone":
		return "+1-555-" + randomDigits(3) + "-" + randomDigits(4)
	case "address":
		streets := []string{"Main St", "Oak Ave", "Park Blvd", "Cedar Ln", "Elm St"}
		cities := []string{"New York", "Los Angeles", "Chicago", "Houston", "Phoenix"}
		return randomDigits(4) + " " + streets[rand.IntN(len(streets))] + ", " +
			cities[rand.IntN(len(cities))]
	case "company":
		names := []string{"Acme", "Globex", "Initech", "Umbrella", "Stark", "Wayne", "Pied Piper"}
		suffixes := []string{"Corp", "Inc", "LLC", "Ltd", "Group"}
		return names[rand.IntN(len(names))] + " " + suffixes[rand.IntN(len(suffixes))]
	case "ipv4":
		return randomDigit() + "." + randomDigit() + "." + randomDigit() + "." + randomDigit()
	case "ipv6":
		return "2001:0db8:" + randomHex(4) + ":" + randomHex(4) + ":" +
			randomHex(4) + ":" + randomHex(4) + ":" + randomHex(4) + ":" + randomHex(4)
	case "sentence":
		words := []string{"The", "quick", "brown", "fox", "jumps", "over", "the", "lazy", "dog",
			"A", "modern", "approach", "to", "building", "scalable", "applications"}
		n := 5 + rand.IntN(6)
		parts := make([]string, n)
		for i := range parts {
			parts[i] = words[rand.IntN(len(words))]
		}
		return strings.Join(parts, " ") + "."
	case "word":
		words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "theta"}
		return words[rand.IntN(len(words))]
	case "slug":
		return randomSlug()
	case "latitude":
		lat := -90.0 + rand.Float64()*180.0
		return formatFloat(lat)
	case "longitude":
		lon := -180.0 + rand.Float64()*360.0
		return formatFloat(lon)
	case "price":
		p := 1.0 + rand.Float64()*999.0
		return formatFloat(float64(int(p*100)) / 100)
	case "color":
		colors := []string{"Red", "Blue", "Green", "Yellow", "Purple", "Orange", "Crimson", "Teal"}
		return colors[rand.IntN(len(colors))]
	case "jobTitle":
		prefixes := []string{"Senior", "Lead", "Junior", "Principal", "Staff"}
		roles := []string{"Engineer", "Designer", "Manager", "Analyst", "Developer"}
		return prefixes[rand.IntN(len(prefixes))] + " Software " + roles[rand.IntN(len(roles))]
	case "ssn":
		return randomDigits(3) + "-" + randomDigits(2) + "-" + randomDigits(4)
	case "currencyCode":
		codes := []string{"USD", "EUR", "GBP", "JPY", "CAD", "AUD", "CHF"}
		return codes[rand.IntN(len(codes))]
	case "boolean":
		if rand.IntN(2) == 0 {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

// Helper functions

func randomDigits(n int) string {
	digits := "0123456789"
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = digits[rand.IntN(10)]
	}
	return string(buf)
}

func randomDigit() string {
	return strconv.Itoa(rand.IntN(256))
}

func randomHex(n int) string {
	hex := "0123456789abcdef"
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = hex[rand.IntN(16)]
	}
	return string(buf)
}

func randomWord() string {
	words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "omega", "sigma", "theta"}
	return words[rand.IntN(len(words))]
}

func randomSlug() string {
	words := []string{"quick", "brown", "fox", "lazy", "dog", "red", "blue", "green"}
	n := 2 + rand.IntN(2)
	parts := make([]string, n)
	for i := range parts {
		parts[i] = words[rand.IntN(len(words))]
	}
	return strings.Join(parts, "-")
}

func generateStringOfLength(n int) string {
	chars := "abcdefghijklmnopqrstuvwxyz"
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = chars[rand.IntN(26)]
	}
	return string(buf)
}

func formatFloat(f float64) string {
	s := fmt.Sprintf("%.6f", f)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
