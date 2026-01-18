package engine

import (
	"io"
	"net/http"
	"sort"

	"github.com/getmockd/mockd/internal/matching"
	"github.com/getmockd/mockd/pkg/mock"
)

// MatchResult contains the result of matching a request against a mock.
type MatchResult struct {
	Mock                *mock.Mock
	Score               int
	PathPatternCaptures map[string]string // Named capture groups from PathPattern regex
}

// SelectBestMatch finds the best matching mock for a request.
// Returns nil if no mock matches.
func SelectBestMatch(mocks []*mock.Mock, r *http.Request) *mock.Mock {
	result := SelectBestMatchWithCaptures(mocks, r)
	if result == nil {
		return nil
	}
	return result.Mock
}

// SelectBestMatchWithCaptures finds the best matching mock for a request.
// Returns nil if no mock matches. Also returns any regex captures from PathPattern.
func SelectBestMatchWithCaptures(mocks []*mock.Mock, r *http.Request) *MatchResult {
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		r.Body = io.NopCloser(NewBodyReader(body))
	}

	var matches []MatchResult

	for _, m := range mocks {
		if m == nil || !m.Enabled {
			continue
		}

		// Only HTTP mocks can be matched against HTTP requests
		if m.Type != mock.MockTypeHTTP || m.HTTP == nil || m.HTTP.Matcher == nil {
			continue
		}

		score, captures := matching.MatchScoreWithCaptures(m.HTTP.Matcher, r, body)
		if score > 0 {
			matches = append(matches, MatchResult{
				Mock:                m,
				Score:               score,
				PathPatternCaptures: captures,
			})
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort by score (descending), then by priority (descending)
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].Score != matches[j].Score {
			return matches[i].Score > matches[j].Score
		}
		// Get priority from HTTP spec
		pi, pj := 0, 0
		if matches[i].Mock.HTTP != nil {
			pi = matches[i].Mock.HTTP.Priority
		}
		if matches[j].Mock.HTTP != nil {
			pj = matches[j].Mock.HTTP.Priority
		}
		return pi > pj
	})

	return &matches[0]
}

// BodyReader is a simple bytes reader that implements io.ReadCloser.
type BodyReader struct {
	data   []byte
	offset int
}

// NewBodyReader creates a new BodyReader.
func NewBodyReader(data []byte) *BodyReader {
	return &BodyReader{data: data}
}

// Read implements io.Reader.
func (r *BodyReader) Read(p []byte) (n int, err error) {
	if r.offset >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.offset:])
	r.offset += n
	return n, nil
}

// Close implements io.Closer.
func (r *BodyReader) Close() error {
	return nil
}
