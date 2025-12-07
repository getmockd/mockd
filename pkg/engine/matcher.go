package engine

import (
	"io"
	"net/http"
	"sort"

	"github.com/getmockd/mockd/internal/matching"
	"github.com/getmockd/mockd/pkg/config"
)

// MatchResult contains the result of matching a request.
type MatchResult struct {
	Mock  *config.MockConfiguration
	Score int
}

// SelectBestMatch finds the best matching mock for a request.
// Returns nil if no mock matches.
func SelectBestMatch(mocks []*config.MockConfiguration, r *http.Request) *config.MockConfiguration {
	// Read body for matching
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
		// Reset body for potential further reads
		r.Body = io.NopCloser(NewBodyReader(body))
	}

	var matches []MatchResult

	for _, mock := range mocks {
		if mock == nil || !mock.Enabled {
			continue
		}

		score := matching.MatchScore(mock.Matcher, r, body)
		if score > 0 {
			matches = append(matches, MatchResult{
				Mock:  mock,
				Score: score,
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
		return matches[i].Mock.Priority > matches[j].Mock.Priority
	})

	return matches[0].Mock
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
