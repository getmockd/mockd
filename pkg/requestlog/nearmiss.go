package requestlog

// NearMissInfo is a log-friendly summary of a near-miss match.
// Stored on request log entries for unmatched requests.
type NearMissInfo struct {
	// MockID is the ID of the mock that partially matched.
	MockID string `json:"mockId"`

	// MockName is the display name of the mock (may be empty).
	MockName string `json:"mockName,omitempty"`

	// MatchPercentage is how close the match was (0-100).
	MatchPercentage int `json:"matchPercentage"`

	// Reason is a human-readable explanation of why it didn't fully match.
	Reason string `json:"reason"`
}
