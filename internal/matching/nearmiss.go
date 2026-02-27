package matching

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/mtls"
)

// FieldResult describes whether a single matcher field matched the request.
type FieldResult struct {
	Field    string      `json:"field"`
	Matched  bool        `json:"matched"`
	Score    int         `json:"score"`
	MaxScore int         `json:"maxScore"`
	Expected interface{} `json:"expected,omitempty"`
	Actual   interface{} `json:"actual,omitempty"`
	Details  interface{} `json:"details,omitempty"`
}

// HeaderDetail describes the match result for a single header.
type HeaderDetail struct {
	Key      string `json:"key"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
	Matched  bool   `json:"matched"`
}

// NearMiss is a mock that partially matched an incoming request.
type NearMiss struct {
	MockID           string        `json:"mockId"`
	MockName         string        `json:"mockName,omitempty"`
	Score            int           `json:"score"`
	MaxPossibleScore int           `json:"maxPossibleScore"`
	MatchPercentage  int           `json:"matchPercentage"`
	Fields           []FieldResult `json:"fields"`
	Reason           string        `json:"reason"`
}

// MatchBreakdown evaluates every field in the matcher against the request
// without short-circuiting, returning per-field match/mismatch results.
// Only fields that the matcher specifies are included in the breakdown.
func MatchBreakdown(matcher *mock.HTTPMatcher, r *http.Request, body []byte) *NearMiss {
	if matcher == nil {
		return &NearMiss{}
	}

	// Path and PathPattern are mutually exclusive — invalid config scores zero
	if matcher.Path != "" && matcher.PathPattern != "" {
		return &NearMiss{}
	}

	result := &NearMiss{}

	// Method
	if matcher.Method != "" {
		matched := MatchMethod(matcher.Method, r.Method)
		score := 0
		if matched {
			score = ScoreMethod
		}
		result.Fields = append(result.Fields, FieldResult{
			Field:    "method",
			Matched:  matched,
			Score:    score,
			MaxScore: ScoreMethod,
			Expected: matcher.Method,
			Actual:   r.Method,
		})
		result.Score += score
		result.MaxPossibleScore += ScoreMethod
	}

	// Path (exact / named params / wildcard)
	if matcher.Path != "" {
		pathScore := MatchPath(matcher.Path, r.URL.Path)
		matched := pathScore > 0
		maxScore := maxPathScore(matcher.Path)
		if matched {
			result.Score += pathScore
		}
		result.Fields = append(result.Fields, FieldResult{
			Field:    "path",
			Matched:  matched,
			Score:    pathScore,
			MaxScore: maxScore,
			Expected: matcher.Path,
			Actual:   r.URL.Path,
		})
		result.MaxPossibleScore += maxScore
	}

	// PathPattern (regex)
	if matcher.PathPattern != "" {
		pathScore, _ := MatchPathPattern(matcher.PathPattern, r.URL.Path)
		matched := pathScore > 0
		if matched {
			result.Score += pathScore
		}
		result.Fields = append(result.Fields, FieldResult{
			Field:    "pathPattern",
			Matched:  matched,
			Score:    pathScore,
			MaxScore: ScorePathPattern,
			Expected: matcher.PathPattern,
			Actual:   r.URL.Path,
		})
		result.MaxPossibleScore += ScorePathPattern
	}

	// Headers
	if len(matcher.Headers) > 0 {
		allMatched := true
		headerScore := 0
		var details []HeaderDetail
		for name, expected := range matcher.Headers {
			matched := MatchHeaderPattern(name, expected, r.Header)
			actual := r.Header.Get(name)
			if actual == "" {
				actual = "(missing)"
			}
			if matched {
				headerScore += ScoreHeader
			} else {
				allMatched = false
			}
			details = append(details, HeaderDetail{
				Key:      name,
				Expected: expected,
				Actual:   actual,
				Matched:  matched,
			})
		}
		maxScore := len(matcher.Headers) * ScoreHeader
		result.Fields = append(result.Fields, FieldResult{
			Field:    "headers",
			Matched:  allMatched,
			Score:    headerScore,
			MaxScore: maxScore,
			Details:  details,
		})
		result.Score += headerScore
		result.MaxPossibleScore += maxScore
	}

	// Query params
	if len(matcher.QueryParams) > 0 {
		allMatched := true
		qpScore := 0
		var details []HeaderDetail // reuse struct — same shape
		queryValues := r.URL.Query()
		for name, expected := range matcher.QueryParams {
			matched := MatchQueryParam(name, expected, queryValues)
			actual := queryValues.Get(name)
			if actual == "" {
				actual = "(missing)"
			}
			if matched {
				qpScore += ScoreQueryParam
			} else {
				allMatched = false
			}
			details = append(details, HeaderDetail{
				Key:      name,
				Expected: expected,
				Actual:   actual,
				Matched:  matched,
			})
		}
		maxScore := len(matcher.QueryParams) * ScoreQueryParam
		result.Fields = append(result.Fields, FieldResult{
			Field:    "queryParams",
			Matched:  allMatched,
			Score:    qpScore,
			MaxScore: maxScore,
			Details:  details,
		})
		result.Score += qpScore
		result.MaxPossibleScore += maxScore
	}

	// Body equals
	if matcher.BodyEquals != "" {
		matched := string(body) == matcher.BodyEquals
		score := 0
		if matched {
			score = ScoreBodyEquals
		}
		// Truncate actual body for display
		actual := truncate(string(body), 200)
		result.Fields = append(result.Fields, FieldResult{
			Field:    "bodyEquals",
			Matched:  matched,
			Score:    score,
			MaxScore: ScoreBodyEquals,
			Expected: truncate(matcher.BodyEquals, 200),
			Actual:   actual,
		})
		result.Score += score
		result.MaxPossibleScore += ScoreBodyEquals
	}

	// Body contains
	if matcher.BodyContains != "" {
		matched := strings.Contains(string(body), matcher.BodyContains)
		score := 0
		if matched {
			score = ScoreBodyContains
		}
		actual := "(body does not contain substring)"
		if matched {
			actual = "(body contains substring)"
		}
		result.Fields = append(result.Fields, FieldResult{
			Field:    "bodyContains",
			Matched:  matched,
			Score:    score,
			MaxScore: ScoreBodyContains,
			Expected: fmt.Sprintf("contains %q", matcher.BodyContains),
			Actual:   actual,
		})
		result.Score += score
		result.MaxPossibleScore += ScoreBodyContains
	}

	// Body pattern (regex)
	if matcher.BodyPattern != "" {
		bpScore := MatchBodyPattern(matcher.BodyPattern, body)
		matched := bpScore > 0
		score := 0
		if matched {
			score = bpScore
		}
		actual := "(body does not match pattern)"
		if matched {
			actual = "(body matches pattern)"
		}
		result.Fields = append(result.Fields, FieldResult{
			Field:    "bodyPattern",
			Matched:  matched,
			Score:    score,
			MaxScore: ScoreBodyPattern,
			Expected: matcher.BodyPattern,
			Actual:   actual,
		})
		result.Score += score
		result.MaxPossibleScore += ScoreBodyPattern
	}

	// JSONPath
	if len(matcher.BodyJSONPath) > 0 {
		jpResult := MatchJSONPath(matcher.BodyJSONPath, body)
		matched := jpResult.Score > 0
		score := 0
		if matched {
			score = jpResult.Score
		}
		maxScore := len(matcher.BodyJSONPath) * ScoreJSONPathCondition
		result.Fields = append(result.Fields, FieldResult{
			Field:    "bodyJSONPath",
			Matched:  matched,
			Score:    score,
			MaxScore: maxScore,
			Expected: matcher.BodyJSONPath,
		})
		result.Score += score
		result.MaxPossibleScore += maxScore
	}

	// mTLS
	if matcher.MTLS != nil {
		identity := mtls.FromContext(r.Context())
		if identity == nil {
			// mTLS required but no client cert — everything fails
			mtlsMaxScore := estimateMTLSMaxScore(matcher.MTLS)
			result.Fields = append(result.Fields, FieldResult{
				Field:    "mtls",
				Matched:  false,
				Score:    0,
				MaxScore: mtlsMaxScore,
				Expected: "client certificate required",
				Actual:   "(no client certificate)",
			})
			result.MaxPossibleScore += mtlsMaxScore
		} else {
			mtlsScore := matchMTLS(matcher.MTLS, identity)
			mtlsMaxScore := estimateMTLSMaxScore(matcher.MTLS)
			matched := mtlsScore > 0
			if matched {
				result.Score += mtlsScore
			}
			result.Fields = append(result.Fields, FieldResult{
				Field:    "mtls",
				Matched:  matched,
				Score:    mtlsScore,
				MaxScore: mtlsMaxScore,
			})
			result.MaxPossibleScore += mtlsMaxScore
		}
	}

	// Calculate percentage
	if result.MaxPossibleScore > 0 {
		result.MatchPercentage = (result.Score * 100) / result.MaxPossibleScore
	}

	// Generate reason
	result.Reason = GenerateReason(result.Fields)

	return result
}

// CollectNearMisses evaluates all mocks against the request and returns the
// top N by partial match score. Only includes mocks with at least one field
// matched (score > 0). This function is only called on 404s — zero overhead
// on matched requests.
func CollectNearMisses(mocks []*mock.Mock, r *http.Request, body []byte, topN int) []NearMiss {
	if topN <= 0 {
		topN = 3
	}

	var candidates []NearMiss

	for _, m := range mocks {
		if m == nil || (m.Enabled != nil && !*m.Enabled) {
			continue
		}
		if m.Type != mock.TypeHTTP || m.HTTP == nil || m.HTTP.Matcher == nil {
			continue
		}

		nm := MatchBreakdown(m.HTTP.Matcher, r, body)
		if nm.Score == 0 {
			continue // Nothing matched at all — not interesting
		}

		nm.MockID = m.ID
		nm.MockName = m.Name

		candidates = append(candidates, *nm)
	}

	// Sort by score descending, then percentage descending
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score != candidates[j].Score {
			return candidates[i].Score > candidates[j].Score
		}
		return candidates[i].MatchPercentage > candidates[j].MatchPercentage
	})

	if len(candidates) > topN {
		candidates = candidates[:topN]
	}

	return candidates
}

// GenerateReason creates a human-readable explanation of why a mock
// partially matched but ultimately failed.
func GenerateReason(fields []FieldResult) string {
	if len(fields) == 0 {
		return "no fields to compare"
	}

	var matched []string
	var firstMismatch *FieldResult

	for i := range fields {
		if fields[i].Matched {
			matched = append(matched, fields[i].Field)
		} else if firstMismatch == nil {
			firstMismatch = &fields[i]
		}
	}

	if firstMismatch == nil {
		return "all specified fields matched"
	}

	if len(matched) == 0 {
		return formatMismatch(firstMismatch)
	}

	matchedStr := joinFields(matched)
	return matchedStr + " matched, but " + formatMismatch(firstMismatch)
}

// formatMismatch formats a single field mismatch into a human-readable string.
func formatMismatch(f *FieldResult) string {
	switch f.Field {
	case "method":
		return fmt.Sprintf("method expected %q, got %q", f.Expected, f.Actual)
	case "path", "pathPattern":
		return fmt.Sprintf("path expected %q, got %q", f.Expected, f.Actual)
	case "headers":
		if details, ok := f.Details.([]HeaderDetail); ok {
			for _, d := range details {
				if !d.Matched {
					return fmt.Sprintf("header %s expected %q, got %q", d.Key, d.Expected, d.Actual)
				}
			}
		}
		return "header mismatch"
	case "queryParams":
		if details, ok := f.Details.([]HeaderDetail); ok {
			for _, d := range details {
				if !d.Matched {
					return fmt.Sprintf("query param %s expected %q, got %q", d.Key, d.Expected, d.Actual)
				}
			}
		}
		return "query parameter mismatch"
	case "bodyEquals":
		return fmt.Sprintf("body expected exact match %q", f.Expected)
	case "bodyContains":
		return fmt.Sprintf("body expected to contain %q", f.Expected)
	case "bodyPattern":
		return fmt.Sprintf("body expected to match pattern %q", f.Expected)
	case "bodyJSONPath":
		return "body JSONPath condition not satisfied"
	case "mtls":
		return fmt.Sprintf("mTLS %v", f.Actual)
	default:
		return f.Field + " did not match"
	}
}

// joinFields joins field names with commas and "and".
func joinFields(fields []string) string {
	switch len(fields) {
	case 0:
		return ""
	case 1:
		return fields[0]
	case 2:
		return fields[0] + " and " + fields[1]
	default:
		return strings.Join(fields[:len(fields)-1], ", ") + ", and " + fields[len(fields)-1]
	}
}

// maxPathScore returns the maximum possible score for a path pattern.
func maxPathScore(path string) int {
	if strings.Contains(path, "{") {
		return ScorePathNamedParams
	}
	if strings.Contains(path, "*") {
		return ScorePathWildcard
	}
	return ScorePathExact
}

// estimateMTLSMaxScore estimates the max possible score for mTLS matching.
func estimateMTLSMaxScore(m *mock.MTLSMatch) int {
	score := 0
	if m.RequireAuth {
		score += ScoreMTLSRequireAuth
	}
	if m.CN != "" {
		score += ScoreMTLSCommonName
	}
	if m.CNPattern != "" {
		score += ScoreMTLSCNPattern
	}
	if m.OU != "" {
		score += ScoreMTLSOrgUnit
	}
	if m.O != "" {
		score += ScoreMTLSOrganization
	}
	if m.Fingerprint != "" {
		score += ScoreMTLSFingerprint
	}
	if m.Issuer != "" {
		score += ScoreMTLSIssuer
	}
	if m.SAN != nil {
		if m.SAN.DNS != "" {
			score += ScoreSANDNS
		}
		if m.SAN.Email != "" {
			score += ScoreSANEmail
		}
		if m.SAN.IP != "" {
			score += ScoreSANIP
		}
	}
	return score
}

// truncate shortens a string to maxLen, appending "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
