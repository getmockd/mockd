package validation

import (
	"net"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// FormatValidator is a function that validates a string against a format
type FormatValidator func(value string) bool

// formatValidators maps format names to their validation functions
var formatValidators = map[string]FormatValidator{
	"email":     validateEmail,
	"uuid":      validateUUID,
	"date":      validateDate,
	"datetime":  validateDateTime,
	"date-time": validateDateTime, // Alias
	"uri":       validateURI,
	"url":       validateURI, // Alias
	"ipv4":      validateIPv4,
	"ipv6":      validateIPv6,
	"ip":        validateIP,
	"hostname":  validateHostname,
}

// ValidateFormat checks if a value matches the specified format
func ValidateFormat(format, value string) bool {
	validator, ok := formatValidators[strings.ToLower(format)]
	if !ok {
		// Unknown format - pass validation (don't fail on unknown formats)
		return true
	}
	return validator(value)
}

// IsKnownFormat returns true if the format is recognized
func IsKnownFormat(format string) bool {
	_, ok := formatValidators[strings.ToLower(format)]
	return ok
}

// RegisterFormat allows registering custom format validators
func RegisterFormat(name string, validator FormatValidator) {
	formatValidators[strings.ToLower(name)] = validator
}

// Email validation using RFC 5322
func validateEmail(value string) bool {
	// Use Go's mail package for RFC 5322 compliance
	_, err := mail.ParseAddress(value)
	if err != nil {
		return false
	}
	// Additional check: must have domain part with dot
	parts := strings.Split(value, "@")
	if len(parts) != 2 {
		return false
	}
	return strings.Contains(parts[1], ".")
}

// UUID v4 pattern
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validateUUID(value string) bool {
	return uuidPattern.MatchString(value)
}

// Date validation (ISO 8601: YYYY-MM-DD)
func validateDate(value string) bool {
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

// DateTime validation (RFC 3339)
func validateDateTime(value string) bool {
	// Try multiple common formats
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if _, err := time.Parse(format, value); err == nil {
			return true
		}
	}
	return false
}

// URI validation (RFC 3986)
func validateURI(value string) bool {
	u, err := url.Parse(value)
	if err != nil {
		return false
	}
	// Must have scheme and host for a valid URI
	return u.Scheme != "" && u.Host != ""
}

// IPv4 validation
func validateIPv4(value string) bool {
	ip := net.ParseIP(value)
	if ip == nil {
		return false
	}
	// Ensure it's IPv4 (not IPv6)
	return ip.To4() != nil
}

// IPv6 validation
func validateIPv6(value string) bool {
	ip := net.ParseIP(value)
	if ip == nil {
		return false
	}
	// Ensure it's IPv6 (not IPv4)
	return ip.To4() == nil
}

// IP validation (either IPv4 or IPv6)
func validateIP(value string) bool {
	return net.ParseIP(value) != nil
}

// Hostname validation (RFC 1123)
var hostnamePattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

func validateHostname(value string) bool {
	if len(value) > 253 {
		return false
	}
	return hostnamePattern.MatchString(value)
}

// DetectFormat attempts to auto-detect the format of a string value
// Returns the detected format name or empty string if no format detected
func DetectFormat(value string) string {
	// Check in order of specificity (most specific first)
	if validateUUID(value) {
		return "uuid"
	}
	if validateEmail(value) {
		return "email"
	}
	if validateIPv4(value) {
		return "ipv4"
	}
	if validateIPv6(value) {
		return "ipv6"
	}
	if validateDate(value) {
		return "date"
	}
	if validateDateTime(value) {
		return "datetime"
	}
	if validateURI(value) {
		return "uri"
	}
	if validateHostname(value) && strings.Contains(value, ".") {
		return "hostname"
	}
	return ""
}
