// Package matching provides request matching algorithms.
package matching

// Match score constants for body matching.
// Higher scores indicate more specific/precise matches.
const (
	// ScoreBodyEquals is the score for an exact body match.
	ScoreBodyEquals = 25

	// ScoreBodyPattern is the score for a body regex pattern match.
	// Between contains (20) and equals (25).
	ScoreBodyPattern = 22

	// ScoreBodyContains is the score for a body substring match.
	ScoreBodyContains = 20

	// ScoreBodyNoCriteria is the score when no body criteria is specified.
	ScoreBodyNoCriteria = 1
)

// Match score constants for path matching.
const (
	// ScorePathExact is the score for an exact path match.
	ScorePathExact = 15

	// ScorePathPattern is the score for a path regex pattern match.
	// Between exact (15) and named params (12).
	ScorePathPattern = 14

	// ScorePathNamedParams is the score for a path with named parameters match.
	ScorePathNamedParams = 12

	// ScorePathWildcard is the score for a wildcard path match.
	ScorePathWildcard = 10
)

// Match score constants for method, header, and query matching.
const (
	// ScoreMethod is the score for a method match.
	ScoreMethod = 10

	// ScoreHeader is the score for each header match.
	ScoreHeader = 10

	// ScoreQueryParam is the score for each query parameter match.
	ScoreQueryParam = 5
)

// Match score constants for JSONPath matching.
const (
	// ScoreJSONPathCondition is the score per matched JSONPath condition.
	ScoreJSONPathCondition = 15
)

// Match score constants for mTLS certificate matching.
const (
	// ScoreMTLSRequireAuth is the score for requiring mTLS authentication.
	// Low score as it's just a presence check.
	ScoreMTLSRequireAuth = 5

	// ScoreMTLSCommonName is the score for a Common Name (CN) match.
	ScoreMTLSCommonName = 15

	// ScoreMTLSCNPattern is the score for a Common Name pattern (regex) match.
	// Slightly less specific than exact CN match.
	ScoreMTLSCNPattern = 12

	// ScoreMTLSOrgUnit is the score for an Organizational Unit (OU) match.
	ScoreMTLSOrgUnit = 10

	// ScoreMTLSOrganization is the score for an Organization (O) match.
	ScoreMTLSOrganization = 10

	// ScoreMTLSFingerprint is the score for a certificate fingerprint match.
	// Highest score as it uniquely identifies a specific certificate.
	ScoreMTLSFingerprint = 50

	// ScoreMTLSIssuer is the score for an Issuer Common Name match.
	// Organization-level match.
	ScoreMTLSIssuer = 10

	// ScoreSANDNS is the score for a SAN DNS name match.
	ScoreSANDNS = 10

	// ScoreSANEmail is the score for a SAN email address match.
	ScoreSANEmail = 10

	// ScoreSANIP is the score for a SAN IP address match.
	ScoreSANIP = 10
)
