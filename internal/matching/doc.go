// Package matching provides request matching algorithms for the mock server.
//
// It implements scoring-based matching for HTTP requests against mock definitions,
// supporting multiple matching criteria including:
//
//   - Path matching: exact paths, wildcard patterns, named parameters, and regex patterns
//   - Method matching: HTTP method verification
//   - Header matching: exact values and wildcard patterns
//   - Query parameter matching: key-value verification
//   - Body matching: exact, contains, regex patterns, and JSONPath expressions
//   - mTLS identity matching: client certificate verification
//
// The matching system uses a weighted scoring algorithm where more specific matches
// receive higher scores. When multiple mocks could match a request, the one with
// the highest score is selected. Score constants are defined in scores.go.
//
// Key types:
//
//   - MatchResult: Contains the matching outcome including score and captured values
//   - MatchScore: Calculates match scores for requests against matchers
//   - MatchScoreWithCaptures: Returns match scores along with regex capture groups
package matching
