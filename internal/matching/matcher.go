// Package matching provides request matching algorithms.
package matching

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/mtls"
)

// MatchResult contains the result of matching a request against a mock.
type MatchResult struct {
	Mock                *mock.Mock
	Score               int
	Matched             bool
	PathPatternCaptures map[string]string      // Named capture groups from PathPattern regex
	JSONPathMatches     map[string]interface{} // Values extracted from JSONPath matching
}

// MatchScore calculates the match score for a request against a matcher.
// Returns 0 if there's no match, higher scores indicate better matches.
func MatchScore(matcher *mock.HTTPMatcher, r *http.Request, body []byte) int {
	score, _ := MatchScoreWithCaptures(matcher, r, body)
	return score
}

// MatchScoreWithCaptures calculates the match score and returns any regex captures.
// Returns 0 if there's no match, higher scores indicate better matches.
// Also returns a map of named capture groups from PathPattern regex matching.
func MatchScoreWithCaptures(matcher *mock.HTTPMatcher, r *http.Request, body []byte) (int, map[string]string) {
	score, pathCaptures, _ := MatchScoreWithAllCaptures(matcher, r, body)
	return score, pathCaptures
}

// MatchScoreWithAllCaptures calculates the match score and returns all captures.
// Returns 0 if there's no match, higher scores indicate better matches.
// Returns path pattern captures and JSONPath matched values.
func MatchScoreWithAllCaptures(matcher *mock.HTTPMatcher, r *http.Request, body []byte) (int, map[string]string, map[string]interface{}) {
	if matcher == nil {
		return 0, nil, nil
	}

	// Path and PathPattern are mutually exclusive
	if matcher.Path != "" && matcher.PathPattern != "" {
		return 0, nil, nil
	}

	score := 0
	var pathCaptures map[string]string
	var jsonPathMatches map[string]interface{}

	// Method matching (required if specified)
	if matcher.Method != "" {
		if !MatchMethod(matcher.Method, r.Method) {
			return 0, nil, nil // Method mismatch = no match
		}
		score += ScoreMethod
	}

	// Path matching (required if specified)
	if matcher.Path != "" {
		pathScore := MatchPath(matcher.Path, r.URL.Path)
		if pathScore == 0 {
			return 0, nil, nil // Path mismatch = no match
		}
		score += pathScore
	}

	// PathPattern regex matching (required if specified)
	if matcher.PathPattern != "" {
		pathScore, captures := MatchPathPattern(matcher.PathPattern, r.URL.Path)
		if pathScore == 0 {
			return 0, nil, nil // PathPattern mismatch = no match
		}
		score += pathScore
		pathCaptures = captures
	}

	// Header matching (supports wildcards via MatchHeaderPattern)
	for name, value := range matcher.Headers {
		if !MatchHeaderPattern(name, value, r.Header) {
			return 0, nil, nil // All headers must match
		}
		score += ScoreHeader
	}

	// Query param matching
	for name, value := range matcher.QueryParams {
		if !MatchQueryParam(name, value, r.URL.Query()) {
			return 0, nil, nil // All query params must match
		}
		score += ScoreQueryParam
	}

	// Body matching - BodyEquals, BodyContains, BodyPattern, and BodyJSONPath can be combined (AND logic)
	if matcher.BodyEquals != "" {
		if string(body) != matcher.BodyEquals {
			return 0, nil, nil // BodyEquals must match if specified
		}
		score += ScoreBodyEquals
	}

	if matcher.BodyContains != "" {
		if !strings.Contains(string(body), matcher.BodyContains) {
			return 0, nil, nil // BodyContains must match if specified
		}
		score += ScoreBodyContains
	}

	if matcher.BodyPattern != "" {
		bodyPatternScore := MatchBodyPattern(matcher.BodyPattern, body)
		if bodyPatternScore == 0 {
			return 0, nil, nil // BodyPattern must match if specified
		}
		score += bodyPatternScore
	}

	// JSONPath body matching
	if len(matcher.BodyJSONPath) > 0 {
		jpResult := MatchJSONPath(matcher.BodyJSONPath, body)
		if jpResult.Score == 0 {
			return 0, nil, nil // JSONPath must match if specified
		}
		score += jpResult.Score
		jsonPathMatches = jpResult.Matched
	}

	// MTLS matching
	if matcher.MTLS != nil {
		identity := mtls.FromContext(r.Context())
		if identity == nil {
			return 0, nil, nil // mTLS match required but no client cert
		}

		mtlsScore := matchMTLS(matcher.MTLS, identity)
		if mtlsScore == 0 {
			return 0, nil, nil // mTLS match failed
		}
		score += mtlsScore
	}

	return score, pathCaptures, jsonPathMatches
}

// MatchMethod checks if the request method matches.
func MatchMethod(expected, actual string) bool {
	return strings.EqualFold(expected, actual)
}

// MatchHTTPMatcher calculates the match score for a request against a mock.HTTPMatcher.
// Returns 0 if there's no match, higher scores indicate better matches.
// Deprecated: Use MatchScore instead.
func MatchHTTPMatcher(matcher *mock.HTTPMatcher, r *http.Request, body []byte) int {
	return MatchScore(matcher, r, body)
}

// MatchHTTPMatcherWithCaptures calculates the match score for mock.HTTPMatcher and returns captures.
// Returns 0 if there's no match, higher scores indicate better matches.
// Deprecated: Use MatchScoreWithCaptures instead.
func MatchHTTPMatcherWithCaptures(matcher *mock.HTTPMatcher, r *http.Request, body []byte) (int, map[string]string) {
	return MatchScoreWithCaptures(matcher, r, body)
}

// MatchHTTPMatcherWithAllCaptures calculates the match score for mock.HTTPMatcher and returns all captures.
// Returns 0 if there's no match, higher scores indicate better matches.
// Deprecated: Use MatchScoreWithAllCaptures instead.
func MatchHTTPMatcherWithAllCaptures(matcher *mock.HTTPMatcher, r *http.Request, body []byte) (int, map[string]string, map[string]interface{}) {
	return MatchScoreWithAllCaptures(matcher, r, body)
}

// matchMTLS calculates the match score for mTLS certificate matching.
// Returns 0 if any required field doesn't match, otherwise returns
// the accumulated score based on matched fields.
func matchMTLS(m *mock.MTLSMatch, identity *mtls.ClientIdentity) int {
	score := 0

	// RequireAuth: check that the certificate is verified
	if m.RequireAuth {
		if !identity.Verified {
			return 0
		}
		score += ScoreMTLSRequireAuth
	}

	// Match CN (Common Name) - exact match
	if m.CN != "" {
		if identity.CommonName != m.CN {
			return 0
		}
		score += ScoreMTLSCommonName
	}

	// Match CNPattern (Common Name regex pattern)
	if m.CNPattern != "" {
		matched, err := regexp.MatchString(m.CNPattern, identity.CommonName)
		if err != nil || !matched {
			return 0
		}
		score += ScoreMTLSCNPattern
	}

	// Match OU (Organizational Unit)
	if m.OU != "" {
		found := false
		for _, ou := range identity.OrganizationalUnit {
			if ou == m.OU {
				found = true
				break
			}
		}
		if !found {
			return 0
		}
		score += ScoreMTLSOrgUnit
	}

	// Match O (Organization)
	if m.O != "" {
		found := false
		for _, o := range identity.Organization {
			if o == m.O {
				found = true
				break
			}
		}
		if !found {
			return 0
		}
		score += ScoreMTLSOrganization
	}

	// Match Fingerprint (SHA256 certificate fingerprint)
	if m.Fingerprint != "" {
		normalizedMatcher := normalizeFingerprint(m.Fingerprint)
		normalizedCert := normalizeFingerprint(identity.Fingerprint)
		if normalizedMatcher != normalizedCert {
			return 0
		}
		score += ScoreMTLSFingerprint
	}

	// Match Issuer (Issuer Common Name)
	if m.Issuer != "" {
		if identity.Issuer.CommonName != m.Issuer {
			return 0
		}
		score += ScoreMTLSIssuer
	}

	// Match SAN (Subject Alternative Names)
	if m.SAN != nil {
		sanScore := matchSAN(m.SAN, &identity.SANs)
		if sanScore == 0 {
			return 0
		}
		score += sanScore
	}

	return score
}

// normalizeFingerprint normalizes a certificate fingerprint for comparison.
// Handles various formats: raw hex, sha256: prefix, colons, and case differences.
func normalizeFingerprint(fp string) string {
	// Remove "sha256:" prefix if present
	fp = strings.TrimPrefix(fp, "sha256:")
	fp = strings.TrimPrefix(fp, "SHA256:")

	// Remove colons
	fp = strings.ReplaceAll(fp, ":", "")

	// Convert to lowercase
	return strings.ToLower(fp)
}

// matchSAN calculates the match score for Subject Alternative Names.
// Returns 0 if any specified field doesn't match.
func matchSAN(m *mock.SANMatch, sans *mtls.SubjectAltNames) int {
	score := 0

	if m.DNS != "" {
		if !containsWildcard(sans.DNSNames, m.DNS) {
			return 0
		}
		score += ScoreSANDNS
	}

	if m.Email != "" {
		if !containsExact(sans.EmailAddresses, m.Email) {
			return 0
		}
		score += ScoreSANEmail
	}

	if m.IP != "" {
		if !containsExact(sans.IPAddresses, m.IP) {
			return 0
		}
		score += ScoreSANIP
	}

	return score
}

// containsExact checks if the slice contains the exact value.
func containsExact(slice []string, value string) bool {
	for _, s := range slice {
		if s == value {
			return true
		}
	}
	return false
}

// containsWildcard checks if any value in the slice matches the pattern.
// Supports wildcard matching where pattern "*.example.com" matches "api.example.com".
func containsWildcard(slice []string, pattern string) bool {
	for _, s := range slice {
		if matchWildcardString(pattern, s) {
			return true
		}
	}
	return false
}

// matchWildcardString performs simple wildcard pattern matching.
// Supports * as a wildcard that matches any sequence of characters.
func matchWildcardString(pattern, value string) bool {
	// Exact match
	if pattern == value {
		return true
	}

	// No wildcards, must be exact
	if !strings.Contains(pattern, "*") {
		return false
	}

	// Split pattern by wildcards
	parts := strings.Split(pattern, "*")

	// Track position in value
	pos := 0

	for i, part := range parts {
		if part == "" {
			continue
		}

		// For first part, must be prefix
		if i == 0 {
			if !strings.HasPrefix(value, part) {
				return false
			}
			pos = len(part)
			continue
		}

		// For last part, must be suffix
		if i == len(parts)-1 {
			if !strings.HasSuffix(value[pos:], part) {
				return false
			}
			continue
		}

		// For middle parts, find the substring
		idx := strings.Index(value[pos:], part)
		if idx == -1 {
			return false
		}
		pos += idx + len(part)
	}

	return true
}
