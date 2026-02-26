// Copyright 2025 Mockd LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package template

import (
	"fmt"
	mathrand "math/rand/v2"
	"strings"
)

// =============================================================================
// Faker Data — Internet
// =============================================================================

// fakerUserAgents contains realistic browser user agent strings.
var fakerUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Safari/605.1.15",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPad; CPU OS 17_2 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.2 Mobile/15E148 Safari/604.1",
}

// =============================================================================
// Faker Data — Finance
// =============================================================================

// fakerCurrencyCodes contains ISO 4217 currency codes.
var fakerCurrencyCodes = []string{
	"USD", "EUR", "GBP", "JPY", "AUD", "CAD", "CHF", "CNY",
	"SEK", "NZD", "MXN", "SGD", "HKD", "NOK", "KRW", "TRY",
	"INR", "RUB", "BRL", "ZAR",
}

// fakerIBANPrefix defines country code, total IBAN length, and a sample bank code.
type fakerIBANPrefix struct {
	country    string
	length     int
	bankPrefix string
}

// fakerIBANPrefixes contains simplified IBAN country definitions.
var fakerIBANPrefixes = []fakerIBANPrefix{
	{"GB", 22, "WEST"},
	{"DE", 22, "DEUT"},
	{"FR", 27, "BNPA"},
	{"ES", 24, "BBVA"},
	{"IT", 27, "UCRI"},
	{"NL", 18, "ABNA"},
}

// =============================================================================
// Faker Data — Commerce
// =============================================================================

// fakerProductAdjectives contains adjectives for product name generation.
var fakerProductAdjectives = []string{
	"Rustic", "Elegant", "Handcrafted", "Refined", "Sleek",
	"Gorgeous", "Practical", "Modern", "Vintage", "Premium",
	"Luxurious", "Compact", "Ergonomic", "Lightweight", "Durable",
}

// fakerProductMaterials contains material names for product name generation.
var fakerProductMaterials = []string{
	"Steel", "Wooden", "Granite", "Rubber", "Cotton",
	"Silk", "Leather", "Bamboo", "Bronze", "Copper",
	"Ceramic", "Plastic", "Glass", "Marble", "Titanium",
}

// fakerProductNouns contains product nouns for product name generation.
var fakerProductNouns = []string{
	"Chair", "Table", "Lamp", "Keyboard", "Mouse",
	"Backpack", "Watch", "Wallet", "Headphones", "Speaker",
	"Notebook", "Pen", "Mug", "Bottle", "Gloves",
}

// fakerColors contains color names.
var fakerColors = []string{
	"Crimson", "Azure", "Emerald", "Ivory", "Coral",
	"Indigo", "Amber", "Jade", "Scarlet", "Turquoise",
	"Lavender", "Maroon", "Teal", "Orchid", "Cyan",
	"Magenta", "Gold", "Silver", "Pearl", "Sapphire",
}

// =============================================================================
// Faker Data — Identity
// =============================================================================

// fakerJobLevels contains seniority levels for job title generation.
var fakerJobLevels = []string{
	"Senior", "Junior", "Lead", "Principal", "Staff",
}

// fakerJobFields contains domain fields for job title generation.
var fakerJobFields = []string{
	"Software", "Data", "Product", "Marketing", "Sales",
	"Operations", "Security", "Infrastructure", "Quality", "Research",
}

// fakerJobRoles contains role titles for job title generation.
var fakerJobRoles = []string{
	"Engineer", "Analyst", "Manager", "Designer", "Architect",
	"Consultant", "Developer", "Specialist", "Coordinator", "Strategist",
}

// =============================================================================
// Faker Data — Data/Files
// =============================================================================

// fakerMIMETypes contains common MIME type strings.
var fakerMIMETypes = []string{
	"application/json", "application/xml", "application/pdf",
	"application/zip", "application/gzip", "application/octet-stream",
	"text/html", "text/plain", "text/css", "text/csv",
	"image/png", "image/jpeg", "image/gif", "image/svg+xml", "image/webp",
	"audio/mpeg", "audio/wav", "audio/ogg",
	"video/mp4", "video/webm",
	"multipart/form-data",
}

// fakerFileExtensions contains common file extensions (without leading dot).
var fakerFileExtensions = []string{
	"pdf", "jpg", "png", "gif", "doc", "docx",
	"xls", "xlsx", "csv", "txt", "html", "css",
	"js", "json", "xml", "zip", "tar", "gz",
	"mp3", "mp4", "wav", "avi", "mov", "svg",
	"ppt", "pptx", "md", "yaml", "toml", "log",
}

// =============================================================================
// Faker Generation Functions
// =============================================================================

// fakerIPv4 generates a random IPv4 address.
func fakerIPv4() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		mathrand.IntN(256), mathrand.IntN(256),
		mathrand.IntN(256), mathrand.IntN(256))
}

// fakerIPv6 generates a random IPv6 address in full expanded notation.
func fakerIPv6() string {
	groups := make([]string, 8)
	for i := range groups {
		groups[i] = fmt.Sprintf("%04x", mathrand.IntN(65536))
	}
	return strings.Join(groups, ":")
}

// fakerMACAddress generates a random MAC address in uppercase hex notation.
func fakerMACAddress() string {
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		mathrand.IntN(256), mathrand.IntN(256),
		mathrand.IntN(256), mathrand.IntN(256),
		mathrand.IntN(256), mathrand.IntN(256))
}

// fakerCreditCard generates a Luhn-valid 16-digit credit card number.
// Uses a Visa-like prefix (starts with 4).
func fakerCreditCard() string {
	digits := make([]int, 16)
	digits[0] = 4 // Visa-like prefix
	for i := 1; i < 15; i++ {
		digits[i] = mathrand.IntN(10)
	}

	// Calculate Luhn check digit.
	// In a 16-digit number, digit at index i is at position (15-i) from right.
	// We double digits at odd positions from right: indices 0, 2, 4, ..., 14.
	sum := 0
	for i := 0; i < 15; i++ {
		d := digits[i]
		if i%2 == 0 {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	digits[15] = (10 - (sum % 10)) % 10

	var sb strings.Builder
	for _, d := range digits {
		sb.WriteByte(byte('0' + d))
	}
	return sb.String()
}

// fakerIBAN generates a simplified IBAN string with a realistic structure.
func fakerIBAN() string {
	prefix := fakerIBANPrefixes[mathrand.IntN(len(fakerIBANPrefixes))]
	checkDigits := fmt.Sprintf("%02d", mathrand.IntN(90)+10)

	// Fill remaining length with random digits
	remaining := prefix.length - len(prefix.country) - 2 - len(prefix.bankPrefix)
	var sb strings.Builder
	sb.WriteString(prefix.country)
	sb.WriteString(checkDigits)
	sb.WriteString(prefix.bankPrefix)
	for i := 0; i < remaining; i++ {
		sb.WriteByte(byte('0' + mathrand.IntN(10)))
	}
	return sb.String()
}

// fakerPrice generates a random price string with 2 decimal places.
func fakerPrice() string {
	dollars := mathrand.IntN(999) + 1
	cents := mathrand.IntN(100)
	return fmt.Sprintf("%d.%02d", dollars, cents)
}

// fakerSSN generates a random SSN in ###-##-#### format.
func fakerSSN() string {
	area := mathrand.IntN(899) + 100
	group := mathrand.IntN(99) + 1
	serial := mathrand.IntN(9999) + 1
	return fmt.Sprintf("%03d-%02d-%04d", area, group, serial)
}

// fakerPassport generates a random passport number (2 uppercase letters + 7 digits).
func fakerPassport() string {
	var sb strings.Builder
	sb.WriteByte(byte('A' + mathrand.IntN(26)))
	sb.WriteByte(byte('A' + mathrand.IntN(26)))
	for i := 0; i < 7; i++ {
		sb.WriteByte(byte('0' + mathrand.IntN(10)))
	}
	return sb.String()
}
