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
	"time"
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

// fakerCurrencyNames contains full currency names (paired with fakerCurrencyCodes).
var fakerCurrencyNames = []string{
	"US Dollar", "Euro", "British Pound", "Japanese Yen", "Australian Dollar",
	"Canadian Dollar", "Swiss Franc", "Chinese Yuan", "Swedish Krona", "New Zealand Dollar",
	"Mexican Peso", "Singapore Dollar", "Hong Kong Dollar", "Norwegian Krone", "South Korean Won",
	"Turkish Lira", "Indian Rupee", "Russian Ruble", "Brazilian Real", "South African Rand",
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
// Faker Data — Text
// =============================================================================

// fakerWordList contains common English words for word/slug generation.
var fakerWordList = []string{
	"ocean", "river", "mountain", "forest", "desert", "valley", "island", "canyon",
	"cloud", "storm", "thunder", "breeze", "shadow", "light", "flame", "frost",
	"crystal", "silver", "golden", "iron", "stone", "marble", "pearl", "amber",
	"falcon", "eagle", "wolf", "tiger", "bear", "hawk", "raven", "lion",
	"horizon", "summit", "harbor", "bridge", "tower", "garden", "palace", "castle",
	"dream", "vision", "spirit", "echo", "pulse", "spark", "bloom", "drift",
	"brave", "swift", "calm", "bold", "keen", "vast", "deep", "pure",
	"design", "craft", "build", "forge", "shape", "blend", "carve", "weave",
}

// =============================================================================
// Faker Generation Functions
//
// All generation functions accept an rng parameter. When non-nil, the seeded
// PRNG is used for deterministic output. When nil, the global math/rand/v2
// source is used.
// =============================================================================

// fakerIPv4 generates a random IPv4 address.
func fakerIPv4(rng *mathrand.Rand) string {
	return fmt.Sprintf("%d.%d.%d.%d",
		rngIntN(rng, 256), rngIntN(rng, 256),
		rngIntN(rng, 256), rngIntN(rng, 256))
}

// fakerIPv6 generates a random IPv6 address in full expanded notation.
func fakerIPv6(rng *mathrand.Rand) string {
	groups := make([]string, 8)
	for i := range groups {
		groups[i] = fmt.Sprintf("%04x", rngIntN(rng, 65536))
	}
	return strings.Join(groups, ":")
}

// fakerMACAddress generates a random MAC address in uppercase hex notation.
func fakerMACAddress(rng *mathrand.Rand) string {
	return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X",
		rngIntN(rng, 256), rngIntN(rng, 256),
		rngIntN(rng, 256), rngIntN(rng, 256),
		rngIntN(rng, 256), rngIntN(rng, 256))
}

// fakerCreditCard generates a Luhn-valid 16-digit credit card number.
// Uses a Visa-like prefix (starts with 4).
func fakerCreditCard(rng *mathrand.Rand) string {
	digits := make([]int, 16)
	digits[0] = 4 // Visa-like prefix
	for i := 1; i < 15; i++ {
		digits[i] = rngIntN(rng, 10)
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
func fakerIBAN(rng *mathrand.Rand) string {
	prefix := fakerIBANPrefixes[rngIntN(rng, len(fakerIBANPrefixes))]
	checkDigits := fmt.Sprintf("%02d", rngIntN(rng, 90)+10)

	// Fill remaining length with random digits
	remaining := prefix.length - len(prefix.country) - 2 - len(prefix.bankPrefix)
	var sb strings.Builder
	sb.WriteString(prefix.country)
	sb.WriteString(checkDigits)
	sb.WriteString(prefix.bankPrefix)
	for i := 0; i < remaining; i++ {
		sb.WriteByte(byte('0' + rngIntN(rng, 10)))
	}
	return sb.String()
}

// fakerPrice generates a random price string with 2 decimal places.
func fakerPrice(rng *mathrand.Rand) string {
	dollars := rngIntN(rng, 999) + 1
	cents := rngIntN(rng, 100)
	return fmt.Sprintf("%d.%02d", dollars, cents)
}

// fakerSSN generates a random SSN in ###-##-#### format.
func fakerSSN(rng *mathrand.Rand) string {
	area := rngIntN(rng, 899) + 100
	group := rngIntN(rng, 99) + 1
	serial := rngIntN(rng, 9999) + 1
	return fmt.Sprintf("%03d-%02d-%04d", area, group, serial)
}

// fakerPassport generates a random passport number (2 uppercase letters + 7 digits).
func fakerPassport(rng *mathrand.Rand) string {
	var sb strings.Builder
	sb.WriteByte(byte('A' + rngIntN(rng, 26)))
	sb.WriteByte(byte('A' + rngIntN(rng, 26)))
	for i := 0; i < 7; i++ {
		sb.WriteByte(byte('0' + rngIntN(rng, 10)))
	}
	return sb.String()
}

// fakerCreditCardExp generates a future credit card expiration date in MM/YY format.
func fakerCreditCardExp(rng *mathrand.Rand) string {
	now := time.Now()
	// Random month 1-12, random year 1-5 years in the future
	month := rngIntN(rng, 12) + 1
	year := now.Year() + rngIntN(rng, 5) + 1
	return fmt.Sprintf("%02d/%02d", month, year%100)
}

// fakerCVV generates a random 3-digit CVV code.
func fakerCVV(rng *mathrand.Rand) string {
	return fmt.Sprintf("%03d", rngIntN(rng, 1000))
}

// fakerHexColor generates a random hex color string (e.g., "#FF5733").
func fakerHexColor(rng *mathrand.Rand) string {
	return fmt.Sprintf("#%02X%02X%02X",
		rngIntN(rng, 256), rngIntN(rng, 256), rngIntN(rng, 256))
}

// fakerLatitude generates a random latitude between -90.0 and 90.0 with 6 decimal places.
func fakerLatitude(rng *mathrand.Rand) string {
	lat := rngFloat64(rng)*180.0 - 90.0
	return fmt.Sprintf("%.6f", lat)
}

// fakerLongitude generates a random longitude between -180.0 and 180.0 with 6 decimal places.
func fakerLongitude(rng *mathrand.Rand) string {
	lng := rngFloat64(rng)*360.0 - 180.0
	return fmt.Sprintf("%.6f", lng)
}

// fakerWords generates n random words from the word list, space-separated.
func fakerWords(rng *mathrand.Rand, n int) string {
	if n <= 0 {
		n = 3
	}
	words := make([]string, n)
	for i := range words {
		words[i] = fakerWordList[rngIntN(rng, len(fakerWordList))]
	}
	return strings.Join(words, " ")
}

// fakerSlug generates a URL-friendly slug of 3 hyphen-separated words.
func fakerSlug(rng *mathrand.Rand) string {
	words := make([]string, 3)
	for i := range words {
		words[i] = fakerWordList[rngIntN(rng, len(fakerWordList))]
	}
	return strings.Join(words, "-")
}
