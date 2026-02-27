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

func TestFakerMacAddress(t *testing.T) {
	engine := New()

	t.Run("returns valid MAC address format", func(t *testing.T) {
		result, err := engine.Process("{{faker.macAddress}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.macAddress should produce non-empty output")
		}
		// Uppercase hex pairs separated by colons
		pattern := `^[0-9A-F]{2}(:[0-9A-F]{2}){5}$`
		if matched, _ := regexp.MatchString(pattern, result); !matched {
			t.Errorf("faker.macAddress = %q doesn't match MAC format", result)
		}
	})

	t.Run("has 6 octets", func(t *testing.T) {
		result, _ := engine.Process("{{faker.macAddress}}", nil)
		parts := strings.Split(result, ":")
		if len(parts) != 6 {
			t.Errorf("faker.macAddress = %q should have 6 octets, got %d", result, len(parts))
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.macAddress}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.macAddress should produce different values across calls")
		}
	})
}

func TestFakerUserAgent(t *testing.T) {
	engine := New()

	t.Run("returns non-empty user agent", func(t *testing.T) {
		result, err := engine.Process("{{faker.userAgent}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.userAgent should produce non-empty output")
		}
	})

	t.Run("starts with Mozilla", func(t *testing.T) {
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.userAgent}}", nil)
			if !strings.HasPrefix(result, "Mozilla/") {
				t.Errorf("faker.userAgent = %q should start with Mozilla/", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.userAgent}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.userAgent should produce different values across calls")
		}
	})
}

// =============================================================================
// Finance Faker Tests
// =============================================================================

func TestFakerCreditCardNumber(t *testing.T) {
	engine := New()

	t.Run("returns 16-digit number", func(t *testing.T) {
		result, err := engine.Process("{{faker.creditCard}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if len(result) != 16 {
			t.Errorf("faker.creditCard should be 16 digits, got %d: %q", len(result), result)
		}
		if matched, _ := regexp.MatchString(`^\d{16}$`, result); !matched {
			t.Errorf("faker.creditCard = %q is not all digits", result)
		}
	})

	t.Run("starts with 4", func(t *testing.T) {
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.creditCard}}", nil)
			if result[0] != '4' {
				t.Errorf("faker.creditCard = %q should start with 4", result)
			}
		}
	})

	t.Run("passes Luhn check", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			result, _ := engine.Process("{{faker.creditCard}}", nil)
			if !luhnValid(result) {
				t.Errorf("faker.creditCard %q fails Luhn validation", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.creditCard}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.creditCard should produce different values across calls")
		}
	})
}

func TestFakerCurrencyCode(t *testing.T) {
	engine := New()

	t.Run("returns valid currency code", func(t *testing.T) {
		result, err := engine.Process("{{faker.currencyCode}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.currencyCode should produce non-empty output")
		}
		// ISO 4217: 3 uppercase letters
		if matched, _ := regexp.MatchString(`^[A-Z]{3}$`, result); !matched {
			t.Errorf("faker.currencyCode = %q doesn't match 3-letter format", result)
		}
	})

	t.Run("returns known currency codes", func(t *testing.T) {
		known := make(map[string]bool)
		for _, c := range fakerCurrencyCodes {
			known[c] = true
		}
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.currencyCode}}", nil)
			if !known[result] {
				t.Errorf("faker.currencyCode = %q is not in known list", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.currencyCode}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.currencyCode should produce different values across calls")
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
		result, err := engine.Process("{{faker.productName}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.productName should produce non-empty output")
		}
		words := strings.Fields(result)
		if len(words) != 3 {
			t.Errorf("faker.productName = %q should have 3 words, got %d", result, len(words))
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
			result, _ := engine.Process("{{faker.productName}}", nil)
			words := strings.Fields(result)
			if len(words) != 3 {
				t.Errorf("faker.productName = %q should have 3 words", result)
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
			result, _ := engine.Process("{{faker.productName}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.productName should produce different values across calls")
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
		result, err := engine.Process("{{faker.jobTitle}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.jobTitle should produce non-empty output")
		}
		words := strings.Fields(result)
		if len(words) != 3 {
			t.Errorf("faker.jobTitle = %q should have 3 words, got %d", result, len(words))
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
			result, _ := engine.Process("{{faker.jobTitle}}", nil)
			words := strings.Fields(result)
			if len(words) != 3 {
				t.Errorf("faker.jobTitle = %q should have 3 words", result)
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
			result, _ := engine.Process("{{faker.jobTitle}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.jobTitle should produce different values across calls")
		}
	})
}

// =============================================================================
// Data Faker Tests
// =============================================================================

func TestFakerMIMEType(t *testing.T) {
	engine := New()

	t.Run("returns valid MIME type", func(t *testing.T) {
		result, err := engine.Process("{{faker.mimeType}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.mimeType should produce non-empty output")
		}
		if !strings.Contains(result, "/") {
			t.Errorf("faker.mimeType = %q should contain '/'", result)
		}
	})

	t.Run("is in known list", func(t *testing.T) {
		known := make(map[string]bool)
		for _, m := range fakerMIMETypes {
			known[m] = true
		}
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.mimeType}}", nil)
			if !known[result] {
				t.Errorf("faker.mimeType = %q is not in known list", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.mimeType}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.mimeType should produce different values across calls")
		}
	})
}

func TestFakerFileExtension(t *testing.T) {
	engine := New()

	t.Run("returns non-empty extension without dot", func(t *testing.T) {
		result, err := engine.Process("{{faker.fileExtension}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.fileExtension should produce non-empty output")
		}
		if strings.HasPrefix(result, ".") {
			t.Errorf("faker.fileExtension = %q should not start with a dot", result)
		}
	})

	t.Run("is in known list", func(t *testing.T) {
		known := make(map[string]bool)
		for _, e := range fakerFileExtensions {
			known[e] = true
		}
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.fileExtension}}", nil)
			if !known[result] {
				t.Errorf("faker.fileExtension = %q is not in known list", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.fileExtension}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.fileExtension should produce different values across calls")
		}
	})
}

// =============================================================================
// Finance Faker Tests (Batch 2)
// =============================================================================

func TestFakerCreditCardExp(t *testing.T) {
	engine := New()

	t.Run("returns MM/YY format", func(t *testing.T) {
		result, err := engine.Process("{{faker.creditCardExp}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.creditCardExp should produce non-empty output")
		}
		pattern := `^\d{2}/\d{2}$`
		if matched, _ := regexp.MatchString(pattern, result); !matched {
			t.Errorf("faker.creditCardExp = %q doesn't match MM/YY format", result)
		}
	})

	t.Run("month is valid 01-12", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.creditCardExp}}", nil)
			parts := strings.Split(result, "/")
			month, _ := strconv.Atoi(parts[0])
			if month < 1 || month > 12 {
				t.Errorf("faker.creditCardExp month %d not in 1-12 range", month)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.creditCardExp}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.creditCardExp should produce different values across calls")
		}
	})
}

func TestFakerCVV(t *testing.T) {
	engine := New()

	t.Run("returns 3-digit number", func(t *testing.T) {
		result, err := engine.Process("{{faker.cvv}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.cvv should produce non-empty output")
		}
		if len(result) != 3 {
			t.Errorf("faker.cvv = %q should be 3 characters", result)
		}
		pattern := `^\d{3}$`
		if matched, _ := regexp.MatchString(pattern, result); !matched {
			t.Errorf("faker.cvv = %q doesn't match 3-digit format", result)
		}
	})

	t.Run("value in range 000-999", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.cvv}}", nil)
			n, err := strconv.Atoi(result)
			if err != nil {
				t.Errorf("faker.cvv = %q is not a number", result)
			}
			if n < 0 || n > 999 {
				t.Errorf("faker.cvv = %d not in range 0-999", n)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.cvv}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.cvv should produce different values across calls")
		}
	})
}

func TestFakerCurrency(t *testing.T) {
	engine := New()

	t.Run("returns known currency name", func(t *testing.T) {
		result, err := engine.Process("{{faker.currency}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.currency should produce non-empty output")
		}
	})

	t.Run("is in known list", func(t *testing.T) {
		known := make(map[string]bool)
		for _, c := range fakerCurrencyNames {
			known[c] = true
		}
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.currency}}", nil)
			if !known[result] {
				t.Errorf("faker.currency = %q is not in known list", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.currency}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.currency should produce different values across calls")
		}
	})
}

// =============================================================================
// Commerce Faker Tests (Batch 2)
// =============================================================================

func TestFakerHexColor(t *testing.T) {
	engine := New()

	t.Run("returns valid hex color format", func(t *testing.T) {
		result, err := engine.Process("{{faker.hexColor}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.hexColor should produce non-empty output")
		}
		pattern := `^#[0-9A-F]{6}$`
		if matched, _ := regexp.MatchString(pattern, result); !matched {
			t.Errorf("faker.hexColor = %q doesn't match #RRGGBB format", result)
		}
	})

	t.Run("starts with hash", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.hexColor}}", nil)
			if !strings.HasPrefix(result, "#") {
				t.Errorf("faker.hexColor = %q should start with #", result)
			}
		}
	})

	t.Run("has correct length", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.hexColor}}", nil)
			if len(result) != 7 {
				t.Errorf("faker.hexColor = %q should be 7 characters (including #), got %d", result, len(result))
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.hexColor}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.hexColor should produce different values across calls")
		}
	})
}

// =============================================================================
// Geo Faker Tests
// =============================================================================

func TestFakerLatitude(t *testing.T) {
	engine := New()

	t.Run("returns valid latitude", func(t *testing.T) {
		result, err := engine.Process("{{faker.latitude}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.latitude should produce non-empty output")
		}
		f, err := strconv.ParseFloat(result, 64)
		if err != nil {
			t.Fatalf("faker.latitude = %q is not a valid float", result)
		}
		if f < -90.0 || f > 90.0 {
			t.Errorf("faker.latitude = %f not in range [-90, 90]", f)
		}
	})

	t.Run("has 6 decimal places", func(t *testing.T) {
		re := regexp.MustCompile(`^-?\d+\.\d{6}$`)
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.latitude}}", nil)
			if !re.MatchString(result) {
				t.Errorf("faker.latitude = %q doesn't have 6 decimal places", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.latitude}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.latitude should produce different values across calls")
		}
	})
}

func TestFakerLongitude(t *testing.T) {
	engine := New()

	t.Run("returns valid longitude", func(t *testing.T) {
		result, err := engine.Process("{{faker.longitude}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.longitude should produce non-empty output")
		}
		f, err := strconv.ParseFloat(result, 64)
		if err != nil {
			t.Fatalf("faker.longitude = %q is not a valid float", result)
		}
		if f < -180.0 || f > 180.0 {
			t.Errorf("faker.longitude = %f not in range [-180, 180]", f)
		}
	})

	t.Run("has 6 decimal places", func(t *testing.T) {
		re := regexp.MustCompile(`^-?\d+\.\d{6}$`)
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.longitude}}", nil)
			if !re.MatchString(result) {
				t.Errorf("faker.longitude = %q doesn't have 6 decimal places", result)
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.longitude}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.longitude should produce different values across calls")
		}
	})
}

// =============================================================================
// Text Faker Tests
// =============================================================================

func TestFakerWords(t *testing.T) {
	engine := New()

	t.Run("returns multiple words (no-arg)", func(t *testing.T) {
		result, err := engine.Process("{{faker.words}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.words should produce non-empty output")
		}
		words := strings.Fields(result)
		if len(words) < 3 || len(words) > 5 {
			t.Errorf("faker.words = %q should have 3-5 words, got %d", result, len(words))
		}
	})

	t.Run("words come from known list", func(t *testing.T) {
		known := make(map[string]bool)
		for _, w := range fakerWordList {
			known[w] = true
		}
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.words}}", nil)
			for _, w := range strings.Fields(result) {
				if !known[w] {
					t.Errorf("word %q not in known list", w)
				}
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.words}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.words should produce different values across calls")
		}
	})
}

func TestFakerWordsParameterized(t *testing.T) {
	engine := New()

	t.Run("faker.words(1) returns exactly 1 word", func(t *testing.T) {
		result, err := engine.Process("{{faker.words(1)}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		words := strings.Fields(result)
		if len(words) != 1 {
			t.Errorf("faker.words(1) = %q should have 1 word, got %d", result, len(words))
		}
	})

	t.Run("faker.words(5) returns exactly 5 words", func(t *testing.T) {
		result, err := engine.Process("{{faker.words(5)}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		words := strings.Fields(result)
		if len(words) != 5 {
			t.Errorf("faker.words(5) = %q should have 5 words, got %d", result, len(words))
		}
	})

	t.Run("faker.words(10) returns exactly 10 words", func(t *testing.T) {
		result, err := engine.Process("{{faker.words(10)}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		words := strings.Fields(result)
		if len(words) != 10 {
			t.Errorf("faker.words(10) = %q should have 10 words, got %d", result, len(words))
		}
	})

	t.Run("all words come from known list", func(t *testing.T) {
		known := make(map[string]bool)
		for _, w := range fakerWordList {
			known[w] = true
		}
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.words(7)}}", nil)
			for _, w := range strings.Fields(result) {
				if !known[w] {
					t.Errorf("word %q not in known list", w)
				}
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.words(4)}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.words(4) should produce different values across calls")
		}
	})
}

func TestFakerSlug(t *testing.T) {
	engine := New()

	t.Run("returns hyphen-separated slug", func(t *testing.T) {
		result, err := engine.Process("{{faker.slug}}", nil)
		if err != nil {
			t.Fatalf("Process() error = %v", err)
		}
		if result == "" {
			t.Fatal("faker.slug should produce non-empty output")
		}
		parts := strings.Split(result, "-")
		if len(parts) != 3 {
			t.Errorf("faker.slug = %q should have 3 hyphen-separated words, got %d", result, len(parts))
		}
	})

	t.Run("contains no spaces", func(t *testing.T) {
		for i := 0; i < 50; i++ {
			result, _ := engine.Process("{{faker.slug}}", nil)
			if strings.Contains(result, " ") {
				t.Errorf("faker.slug = %q should not contain spaces", result)
			}
		}
	})

	t.Run("words come from known list", func(t *testing.T) {
		known := make(map[string]bool)
		for _, w := range fakerWordList {
			known[w] = true
		}
		for i := 0; i < 30; i++ {
			result, _ := engine.Process("{{faker.slug}}", nil)
			for _, w := range strings.Split(result, "-") {
				if !known[w] {
					t.Errorf("slug word %q not in known list", w)
				}
			}
		}
	})

	t.Run("produces different values", func(t *testing.T) {
		results := make(map[string]bool)
		for i := 0; i < 20; i++ {
			result, _ := engine.Process("{{faker.slug}}", nil)
			results[result] = true
		}
		if len(results) < 2 {
			t.Error("faker.slug should produce different values across calls")
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
		{"macAddress", `^[0-9A-F]{2}(:[0-9A-F]{2}){5}$`},
		{"userAgent", `^Mozilla/.+`},
		{"creditCard", `^\d{16}$`},
		{"currencyCode", `^[A-Z]{3}$`},
		{"iban", `^[A-Z]{2}\d{2}[A-Z]{4}\d+$`},
		{"price", `^\d+\.\d{2}$`},
		{"productName", `^\S+ \S+ \S+$`},
		{"color", `^[A-Z][a-z]+$`},
		{"ssn", `^\d{3}-\d{2}-\d{4}$`},
		{"passport", `^[A-Z]{2}\d{7}$`},
		{"jobTitle", `^\S+ \S+ \S+$`},
		{"mimeType", `.+/.+`},
		{"fileExtension", `^[a-z0-9]+$`},
		// Batch 2 fakers
		{"creditCardExp", `^\d{2}/\d{2}$`},
		{"cvv", `^\d{3}$`},
		{"currency", `^[A-Z].+`},
		{"hexColor", `^#[0-9A-F]{6}$`},
		{"latitude", `^-?\d+\.\d{6}$`},
		{"longitude", `^-?\d+\.\d{6}$`},
		{"words", `^\w+( \w+){2,4}$`},
		{"slug", `^\w+-\w+-\w+$`},
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
