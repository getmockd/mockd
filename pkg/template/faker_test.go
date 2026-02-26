package template

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// =============================================================================
// Internet Faker Tests
// =============================================================================

func TestFakerIPv4(t *testing.T) {
	engine := New()

	t.Run("returns valid IPv4 format", func(t *testing.T) {
		result, err := engine.Process("{{faker.ipv4}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.ipv4 should produce non-empty output")
		}
		pattern := `^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`
		if matched, _ := regexp.MatchString(pattern, result); !matched {
			t.Errorf("faker.ipv4 = %q doesn't match IPv4 format", result)
		}
	})

	t.Run("octets are in valid range", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.ipv4}}", nil)
			parts := strings.Split(result, ".")
			if len(parts) != 4 {
				t.Fatalf("faker.ipv4 = %q should have 4 octets", result)
			}
			for _, p := range parts {
				n, err := strconv.Atoi(p)
				if err != nil {
					t.Fatalf("faker.ipv4 octet %q is not a number", p)
				}
				if n < 0 || n > 255 {
					t.Errorf("faker.ipv4 octet %d not in range 0-255", n)
				}
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.ipv4}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.ipv4 should produce different values across calls")
		}
	})
}

func TestFakerIPv6(t *testing.T) {
	engine := New()

	t.Run("returns valid IPv6 format", func(t *testing.T) {
		result, err := engine.Process("{{faker.ipv6}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.ipv6 should produce non-empty output")
		}
		// Full expanded IPv6: 8 groups of 4 hex digits separated by colons
		pattern := `^[0-9a-f]{4}(:[0-9a-f]{4}){7}$`
		if matched, _ := regexp.MatchString(pattern, result); !matched {
			t.Errorf("faker.ipv6 = %q doesn't match IPv6 format", result)
		}
	})

	t.Run("has 8 groups", func(t *testing.T) {
		result, _ := engine.Process("{{faker.ipv6}}", nil)
		groups := strings.Split(result, ":")
		if len(groups) != 8 {
			t.Errorf("faker.ipv6 = %q should have 8 groups, got %d", result, len(groups))
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.ipv6}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.ipv6 should produce different values across calls")
		}
	})
}

func TestFakerMACAddress(t *testing.T) {
	engine := New()

	t.Run("returns valid MAC address format", func(t *testing.T) {
		result, err := engine.Process("{{faker.mac_address}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.mac_address should produce non-empty output")
		}
		// Uppercase hex pairs separated by colons
		pattern := `^[0-9A-F]{2}(:[0-9A-F]{2}){5}$`
		if matched, _ := regexp.MatchString(pattern, result); !matched {
			t.Errorf("faker.mac_address = %q doesn't match MAC format", result)
		}
	})

	t.Run("has 6 octets", func(t *testing.T) {
		result, _ := engine.Process("{{faker.mac_address}}", nil)
		parts := strings.Split(result, ":")
		if len(parts) != 6 {
			t.Errorf("faker.mac_address = %q should have 6 octets, got %d", result, len(parts))
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.mac_address}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.mac_address should produce different values across calls")
		}
	})
}

func TestFakerUserAgent(t *testing.T) {
	engine := New()

	t.Run("returns non-empty user agent", func(t *testing.T) {
		result, err := engine.Process("{{faker.user_agent}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.user_agent should produce non-empty output")
		}
	})

	t.Run("starts with Mozilla", func(t *testing.T) {
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.user_agent}}", nil)
			if !strings.HasPrefix(result, "Mozilla/") {
				t.Errorf("faker.user_agent = %q should start with Mozilla/", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.user_agent}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.user_agent should produce different values across calls")
		}
	})
}

// =============================================================================
// Finance Faker Tests
// =============================================================================

func TestFakerCreditCard(t *testing.T) {
	engine := New()

	t.Run("returns 16-digit number", func(t *testing.T) {
		result, err := engine.Process("{{faker.credit_card}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if len(result) != 16 {
			t.Errorf("faker.credit_card should be 16 digits, got %d: %q", len(result), result)
		}
		if matched, _ := regexp.MatchString(`^\d{16}$`, result); !matched {
			t.Errorf("faker.credit_card = %q is not all digits", result)
		}
	})

	t.Run("starts with 4", func(t *testing.T) {
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.credit_card}}", nil)
			if result[0] != '4' {
				t.Errorf("faker.credit_card = %q should start with 4", result)
			}
		}
	})

	t.Run("passes Luhn check", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			result, _ := engine.Process("{{faker.credit_card}}", nil)
			if !luhnValid(result) {
				t.Errorf("faker.credit_card %q fails Luhn validation", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.credit_card}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.credit_card should produce different values across calls")
		}
	})
}

func TestFakerCurrencyCode(t *testing.T) {
	engine := New()

	t.Run("returns valid currency code", func(t *testing.T) {
		result, err := engine.Process("{{faker.currency_code}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.currency_code should produce non-empty output")
		}
		// ISO 4217: 3 uppercase letters
		if matched, _ := regexp.MatchString(`^[A-Z]{3}$`, result); !matched {
			t.Errorf("faker.currency_code = %q doesn't match 3-letter format", result)
		}
	})

	t.Run("returns known currency codes", func(t *testing.T) {
		known := make(map[string]bool)
		for _, c := range fakerCurrencyCodes {
			known[c] = true
		}
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.currency_code}}", nil)
			if !known[result] {
				t.Errorf("faker.currency_code = %q is not in known list", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.currency_code}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.currency_code should produce different values across calls")
		}
	})
}

func TestFakerIBAN(t *testing.T) {
	engine := New()

	t.Run("returns valid IBAN format", func(t *testing.T) {
		result, err := engine.Process("{{faker.iban}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.iban should produce non-empty output")
		}
		// Country code (2 letters) + check digits (2 digits) + bank code (4 letters) + account digits
		pattern := `^[A-Z]{2}\d{2}[A-Z]{4}\d+$`
		if matched, _ := regexp.MatchString(pattern, result); !matched {
			t.Errorf("faker.iban = %q doesn't match IBAN format", result)
		}
	})

	t.Run("has correct length for country", func(t *testing.T) {
		lengthByCountry := make(map[string]int)
		for _, p := range fakerIBANPrefixes {
			lengthByCountry[p.country] = p.length
		}
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.iban}}", nil)
			country := result[:2]
			expectedLen, ok := lengthByCountry[country]
			if !ok {
				t.Errorf("faker.iban = %q has unknown country %q", result, country)
				continue
			}
			if len(result) != expectedLen {
				t.Errorf("faker.iban = %q (country %s) has length %d, want %d",
					result, country, len(result), expectedLen)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.iban}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.iban should produce different values across calls")
		}
	})
}

// =============================================================================
// Commerce Faker Tests
// =============================================================================

func TestFakerPrice(t *testing.T) {
	engine := New()

	t.Run("returns price with 2 decimal places", func(t *testing.T) {
		result, err := engine.Process("{{faker.price}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.price should produce non-empty output")
		}
		pattern := `^\d+\.\d{2}$`
		if matched, _ := regexp.MatchString(pattern, result); !matched {
			t.Errorf("faker.price = %q doesn't match price format (N.NN)", result)
		}
	})

	t.Run("is parseable as float", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.price}}", nil)
			f, err := strconv.ParseFloat(result, 64)
			if err != nil {
				t.Errorf("faker.price = %q is not parseable as float: %v", result, err)
			}
			if f < 1.0 || f > 999.99 {
				t.Errorf("faker.price = %q (%f) not in expected range [1.00, 999.99]", result, f)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.price}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.price should produce different values across calls")
		}
	})
}

func TestFakerProductName(t *testing.T) {
	engine := New()

	t.Run("returns three-word product name", func(t *testing.T) {
		result, err := engine.Process("{{faker.product_name}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.product_name should produce non-empty output")
		}
		words := strings.Fields(result)
		if len(words) != 3 {
			t.Errorf("faker.product_name = %q should have 3 words, got %d", result, len(words))
		}
	})

	t.Run("components come from known lists", func(t *testing.T) {
		adjSet := make(map[string]bool)
		for _, a := range fakerProductAdjectives {
			adjSet[a] = true
		}
		matSet := make(map[string]bool)
		for _, m := range fakerProductMaterials {
			matSet[m] = true
		}
		nounSet := make(map[string]bool)
		for _, n := range fakerProductNouns {
			nounSet[n] = true
		}

		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.product_name}}", nil)
			words := strings.Fields(result)
			if len(words) != 3 {
				t.Errorf("faker.product_name = %q should have 3 words", result)
				continue
			}
			if !adjSet[words[0]] {
				t.Errorf("adjective %q not in known list", words[0])
			}
			if !matSet[words[1]] {
				t.Errorf("material %q not in known list", words[1])
			}
			if !nounSet[words[2]] {
				t.Errorf("noun %q not in known list", words[2])
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.product_name}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.product_name should produce different values across calls")
		}
	})
}

func TestFakerColor(t *testing.T) {
	engine := New()

	t.Run("returns known color", func(t *testing.T) {
		result, err := engine.Process("{{faker.color}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.color should produce non-empty output")
		}
	})

	t.Run("is in known color list", func(t *testing.T) {
		known := make(map[string]bool)
		for _, c := range fakerColors {
			known[c] = true
		}
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.color}}", nil)
			if !known[result] {
				t.Errorf("faker.color = %q is not in known list", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.color}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.color should produce different values across calls")
		}
	})
}

// =============================================================================
// Identity Faker Tests
// =============================================================================

func TestFakerSSN(t *testing.T) {
	engine := New()

	t.Run("returns valid SSN format", func(t *testing.T) {
		result, err := engine.Process("{{faker.ssn}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.ssn should produce non-empty output")
		}
		pattern := `^\d{3}-\d{2}-\d{4}$`
		if matched, _ := regexp.MatchString(pattern, result); !matched {
			t.Errorf("faker.ssn = %q doesn't match SSN format (###-##-####)", result)
		}
	})

	t.Run("area number is in valid range", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.ssn}}", nil)
			parts := strings.Split(result, "-")
			area, _ := strconv.Atoi(parts[0])
			if area < 100 || area > 998 {
				t.Errorf("faker.ssn area %d not in range 100-998", area)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.ssn}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.ssn should produce different values across calls")
		}
	})
}

func TestFakerPassport(t *testing.T) {
	engine := New()

	t.Run("returns valid passport format", func(t *testing.T) {
		result, err := engine.Process("{{faker.passport}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.passport should produce non-empty output")
		}
		// 2 uppercase letters + 7 digits
		pattern := `^[A-Z]{2}\d{7}$`
		if matched, _ := regexp.MatchString(pattern, result); !matched {
			t.Errorf("faker.passport = %q doesn't match passport format (AA#######)", result)
		}
	})

	t.Run("has correct length", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.passport}}", nil)
			if len(result) != 9 {
				t.Errorf("faker.passport = %q should be 9 characters, got %d", result, len(result))
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.passport}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.passport should produce different values across calls")
		}
	})
}

func TestFakerJobTitle(t *testing.T) {
	engine := New()

	t.Run("returns three-word job title", func(t *testing.T) {
		result, err := engine.Process("{{faker.job_title}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.job_title should produce non-empty output")
		}
		words := strings.Fields(result)
		if len(words) != 3 {
			t.Errorf("faker.job_title = %q should have 3 words, got %d", result, len(words))
		}
	})

	t.Run("components come from known lists", func(t *testing.T) {
		lvlSet := make(map[string]bool)
		for _, l := range fakerJobLevels {
			lvlSet[l] = true
		}
		fldSet := make(map[string]bool)
		for _, f := range fakerJobFields {
			fldSet[f] = true
		}
		roleSet := make(map[string]bool)
		for _, r := range fakerJobRoles {
			roleSet[r] = true
		}

		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.job_title}}", nil)
			words := strings.Fields(result)
			if len(words) != 3 {
				t.Errorf("faker.job_title = %q should have 3 words", result)
				continue
			}
			if !lvlSet[words[0]] {
				t.Errorf("level %q not in known list", words[0])
			}
			if !fldSet[words[1]] {
				t.Errorf("field %q not in known list", words[1])
			}
			if !roleSet[words[2]] {
				t.Errorf("role %q not in known list", words[2])
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.job_title}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.job_title should produce different values across calls")
		}
	})
}

// =============================================================================
// Data Faker Tests
// =============================================================================

func TestFakerMIMEType(t *testing.T) {
	engine := New()

	t.Run("returns valid MIME type", func(t *testing.T) {
		result, err := engine.Process("{{faker.mime_type}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.mime_type should produce non-empty output")
		}
		if !strings.Contains(result, "/") {
			t.Errorf("faker.mime_type = %q should contain '/'", result)
		}
	})

	t.Run("is in known list", func(t *testing.T) {
		known := make(map[string]bool)
		for _, m := range fakerMIMETypes {
			known[m] = true
		}
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.mime_type}}", nil)
			if !known[result] {
				t.Errorf("faker.mime_type = %q is not in known list", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.mime_type}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.mime_type should produce different values across calls")
		}
	})
}

func TestFakerFileExtension(t *testing.T) {
	engine := New()

	t.Run("returns non-empty extension without dot", func(t *testing.T) {
		result, err := engine.Process("{{faker.file_extension}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.file_extension should produce non-empty output")
		}
		if strings.HasPrefix(result, ".") {
			t.Errorf("faker.file_extension = %q should not start with a dot", result)
		}
	})

	t.Run("is in known list", func(t *testing.T) {
		known := make(map[string]bool)
		for _, e := range fakerFileExtensions {
			known[e] = true
		}
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.file_extension}}", nil)
			if !known[result] {
				t.Errorf("faker.file_extension = %q is not in known list", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.file_extension}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.file_extension should produce different values across calls")
		}
	})
}

// =============================================================================
// Existing Faker Table-Driven Tests (extended with new functions)
// =============================================================================

func TestNewFakerVariablesTableDriven(t *testing.T) {
	engine := New()

	fakerTypes := []struct {
		name    string
		pattern string
	}{
		{"ipv4", `^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`},
		{"ipv6", `^[0-9a-f]{4}(:[0-9a-f]{4}){7}$`},
		{"mac_address", `^[0-9A-F]{2}(:[0-9A-F]{2}){5}$`},
		{"user_agent", `^Mozilla/.+`},
		{"credit_card", `^\d{16}$`},
		{"currency_code", `^[A-Z]{3}$`},
		{"iban", `^[A-Z]{2}\d{2}[A-Z]{4}\d+$`},
		{"price", `^\d+\.\d{2}$`},
		{"product_name", `^\S+ \S+ \S+$`},
		{"color", `^[A-Z][a-z]+$`},
		{"ssn", `^\d{3}-\d{2}-\d{4}$`},
		{"passport", `^[A-Z]{2}\d{7}$`},
		{"job_title", `^\S+ \S+ \S+$`},
		{"mime_type", `.+/.+`},
		{"file_extension", `^[a-z0-9]+$`},
	}

	for _, ft := range fakerTypes {
		t.Run("faker."+ft.name, func(t *testing.T) {
			result, err := engine.Process("{{faker."+ft.name+"}}", nil)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}
			if result == "" {
				t.Errorf("faker.%s should produce non-empty output", ft.name)
			}
			if matched, _ := regexp.MatchString(ft.pattern, result); !matched {
				t.Errorf("faker.%s = %q doesn't match pattern %q", ft.name, result, ft.pattern)
			}
		})
	}
}

// =============================================================================
// Luhn Validation Helper
// =============================================================================

// luhnValid validates a number string using the Luhn algorithm.
func luhnValid(number string) bool {
	sum := 0
	alt := false
	for i := len(number) - 1; i >= 0; i-- {
		d := int(number[i] - '0')
		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return sum%10 == 0
}

// =============================================================================
// Luhn Validator Unit Test
// =============================================================================

func TestLuhnValid(t *testing.T) {
	tests := []struct {
		number string
		valid  bool
	}{
		{"4532015112830366", true},  // known valid Visa
		{"4111111111111111", true},  // test Visa number
		{"5500000000000004", true},  // test Mastercard
		{"1234567890123456", false}, // random invalid
		{"0000000000000000", true},  // edge case: all zeros pass Luhn
	}

	for _, tt := range tests {
		t.Run(tt.number, func(t *testing.T) {
			if got := luhnValid(tt.number); got != tt.valid {
				t.Errorf("luhnValid(%q) = %v, want %v", tt.number, got, tt.valid)
			}
		})
	}
}
