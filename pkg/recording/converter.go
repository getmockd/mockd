// Package recording provides conversion from recordings to mock configurations.
package recording

import (
	"time"

	"github.com/getmockd/mockd/pkg/config"
)

// ConvertOptions configures how recordings are converted to mocks.
type ConvertOptions struct {
	IncludeHeaders bool // Include request headers in matcher
	Deduplicate    bool // Remove duplicate request patterns
}

// DefaultConvertOptions returns the default conversion options.
func DefaultConvertOptions() ConvertOptions {
	return ConvertOptions{
		IncludeHeaders: false,
		Deduplicate:    false,
	}
}

// ToMock converts a recording to a mock configuration.
func ToMock(r *Recording, opts ConvertOptions) *config.MockConfiguration {
	matcher := &config.RequestMatcher{
		Method: r.Request.Method,
		Path:   r.Request.Path,
	}

	if opts.IncludeHeaders && len(r.Request.Headers) > 0 {
		headers := make(map[string]string)
		for key, values := range r.Request.Headers {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
		matcher.Headers = headers
	}

	response := &config.ResponseDefinition{
		StatusCode: r.Response.StatusCode,
		Body:       string(r.Response.Body),
	}

	if len(r.Response.Headers) > 0 {
		headers := make(map[string]string)
		for key, values := range r.Response.Headers {
			if len(values) > 0 {
				headers[key] = values[0]
			}
		}
		response.Headers = headers
	}

	now := time.Now()
	return &config.MockConfiguration{
		ID:        generateID(),
		Matcher:   matcher,
		Response:  response,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// ToMocks converts multiple recordings to mock configurations.
func ToMocks(recordings []*Recording, opts ConvertOptions) []*config.MockConfiguration {
	mocks := make([]*config.MockConfiguration, 0, len(recordings))
	seen := make(map[string]bool)

	for _, r := range recordings {
		// Generate a key for deduplication
		key := r.Request.Method + ":" + r.Request.Path
		if opts.Deduplicate && seen[key] {
			continue
		}
		seen[key] = true

		mocks = append(mocks, ToMock(r, opts))
	}

	return mocks
}

// ConvertSession converts all recordings in a session to mocks.
func ConvertSession(session *Session, opts ConvertOptions) []*config.MockConfiguration {
	return ToMocks(session.Recordings(), opts)
}
