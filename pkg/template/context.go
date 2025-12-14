package template

import (
	"encoding/json"
	"io"
	"net/http"
)

// Context holds all available data for template evaluation.
type Context struct {
	Request RequestContext
}

// RequestContext contains HTTP request data available to templates.
type RequestContext struct {
	Method     string
	Path       string
	URL        string
	Body       interface{}         // Parsed JSON or nil
	RawBody    string              // Original body string
	Query      map[string][]string // Query parameters
	Headers    map[string][]string // HTTP headers
	PathParams map[string]string   // Path parameters
}

// NewContext creates a template context from an HTTP request.
// It parses the request body and makes all request data available for templating.
func NewContext(r *http.Request, bodyBytes []byte) *Context {
	ctx := &Context{
		Request: RequestContext{
			Method:     r.Method,
			Path:       r.URL.Path,
			URL:        r.URL.String(),
			RawBody:    string(bodyBytes),
			Query:      r.URL.Query(),
			Headers:    r.Header,
			PathParams: make(map[string]string),
		},
	}

	// Parse JSON body if Content-Type is application/json
	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" && len(bodyBytes) > 0 {
		var body interface{}
		if err := json.Unmarshal(bodyBytes, &body); err == nil {
			ctx.Request.Body = body
		}
	}

	return ctx
}

// NewContextFromRequest creates a template context by reading the request body.
// The body is read completely and can be read again if needed.
func NewContextFromRequest(r *http.Request) (*Context, error) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	r.Body.Close()

	return NewContext(r, bodyBytes), nil
}
