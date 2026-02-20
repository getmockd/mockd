package template

import (
	"encoding/json"
	"net/http"

	"github.com/getmockd/mockd/pkg/mtls"
)

// Context holds all available data for template evaluation.
type Context struct {
	Request RequestContext
	MTLS    MTLSContext
	MQTT    MQTTContext
}

// MQTTContext holds MQTT-specific template data.
type MQTTContext struct {
	Topic        string
	ClientID     string
	Payload      map[string]any
	WildcardVals []string
	DeviceID     string
	MessageNum   int64
}

// NewMQTTContext creates a template context pre-populated with MQTT data.
// This allows MQTT payloads to use shared template variables like {{now}},
// {{uuid}}, {{timestamp}}, etc.
func NewMQTTContext(topic, clientID string, payload map[string]any, wildcardVals []string) *Context {
	return &Context{
		MQTT: MQTTContext{
			Topic:        topic,
			ClientID:     clientID,
			Payload:      payload,
			WildcardVals: wildcardVals,
		},
	}
}

// MTLSContext contains mTLS client certificate data available to templates.
type MTLSContext struct {
	// CN is the client certificate Common Name
	CN string
	// O is the first Organization from the certificate
	O string
	// OU is the first Organizational Unit from the certificate
	OU string
	// Serial is the certificate serial number
	Serial string
	// Fingerprint is the SHA256 fingerprint of the certificate
	Fingerprint string
	// IssuerCN is the issuer's Common Name
	IssuerCN string
	// NotBefore is the certificate validity start time (RFC3339)
	NotBefore string
	// NotAfter is the certificate validity end time (RFC3339)
	NotAfter string
	// SANDNS is the first DNS Subject Alternative Name
	SANDNS string
	// SANEmail is the first email Subject Alternative Name
	SANEmail string
	// SANIP is the first IP address Subject Alternative Name
	SANIP string
	// SANURI is the first URI Subject Alternative Name
	SANURI string
	// Verified indicates whether the certificate was verified
	Verified bool
	// Present indicates whether mTLS identity is available
	Present bool
}

// RequestContext contains HTTP request data available to templates.
type RequestContext struct {
	Method              string
	Path                string
	URL                 string
	Body                interface{}            // Parsed JSON or nil
	RawBody             string                 // Original body string
	Query               map[string][]string    // Query parameters
	Headers             map[string][]string    // HTTP headers
	PathParams          map[string]string      // Path parameters (from /users/{id} style paths)
	PathPatternCaptures map[string]string      // Named capture groups from PathPattern regex
	JSONPath            map[string]interface{} // Values extracted from JSONPath matching
}

// NewContext creates a template context from an HTTP request.
// It parses the request body and makes all request data available for templating.
func NewContext(r *http.Request, bodyBytes []byte) *Context {
	ctx := &Context{
		Request: RequestContext{
			Method:              r.Method,
			Path:                r.URL.Path,
			URL:                 r.URL.String(),
			RawBody:             string(bodyBytes),
			Query:               r.URL.Query(),
			Headers:             r.Header,
			PathParams:          make(map[string]string),
			PathPatternCaptures: make(map[string]string),
			JSONPath:            make(map[string]interface{}),
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

// SetJSONPathMatches populates the JSONPath context from matching results.
func (c *Context) SetJSONPathMatches(matches map[string]interface{}) {
	if matches == nil {
		return
	}
	for key, value := range matches {
		c.Request.JSONPath[key] = value
	}
}

// SetPathPatternCaptures populates the PathPatternCaptures from regex matching results.
func (c *Context) SetPathPatternCaptures(captures map[string]string) {
	if captures == nil {
		return
	}
	for key, value := range captures {
		c.Request.PathPatternCaptures[key] = value
	}
}

// NewContextFromMap creates a template context from parsed request data.
// This is used by non-HTTP protocols (gRPC, GraphQL, SOAP) that don't have
// a direct http.Request but have equivalent data.
func NewContextFromMap(body interface{}, headers map[string][]string) *Context {
	ctx := &Context{
		Request: RequestContext{
			Body:                body,
			Headers:             headers,
			Query:               make(map[string][]string),
			PathParams:          make(map[string]string),
			PathPatternCaptures: make(map[string]string),
			JSONPath:            make(map[string]interface{}),
		},
	}

	// Set RawBody from body if possible
	if body != nil {
		if jsonBytes, err := json.Marshal(body); err == nil {
			ctx.Request.RawBody = string(jsonBytes)
		}
	}

	// Initialize headers if nil
	if ctx.Request.Headers == nil {
		ctx.Request.Headers = make(map[string][]string)
	}

	return ctx
}

// SetMTLSFromIdentity populates the MTLS context from a ClientIdentity.
// If identity is nil, the MTLS context remains empty with Present=false.
func (c *Context) SetMTLSFromIdentity(identity *mtls.ClientIdentity) {
	if identity == nil {
		return
	}

	c.MTLS.Present = true
	c.MTLS.CN = identity.CommonName
	c.MTLS.Serial = identity.SerialNumber
	c.MTLS.Fingerprint = identity.Fingerprint
	c.MTLS.IssuerCN = identity.Issuer.CommonName
	c.MTLS.NotBefore = identity.NotBefore
	c.MTLS.NotAfter = identity.NotAfter
	c.MTLS.Verified = identity.Verified

	// Get first Organization if available
	if len(identity.Organization) > 0 {
		c.MTLS.O = identity.Organization[0]
	}

	// Get first Organizational Unit if available
	if len(identity.OrganizationalUnit) > 0 {
		c.MTLS.OU = identity.OrganizationalUnit[0]
	}

	// Get first DNS SAN if available
	if len(identity.SANs.DNSNames) > 0 {
		c.MTLS.SANDNS = identity.SANs.DNSNames[0]
	}

	// Get first email SAN if available
	if len(identity.SANs.EmailAddresses) > 0 {
		c.MTLS.SANEmail = identity.SANs.EmailAddresses[0]
	}

	// Get first IP SAN if available
	if len(identity.SANs.IPAddresses) > 0 {
		c.MTLS.SANIP = identity.SANs.IPAddresses[0]
	}

	// Get first URI SAN if available
	if len(identity.SANs.URIs) > 0 {
		c.MTLS.SANURI = identity.SANs.URIs[0]
	}
}
