// Package recording provides conversion from SOAP recordings to mock configurations.
package recording

import (
	"encoding/xml"
	"regexp"
	"strings"

	"github.com/getmockd/mockd/pkg/soap"
)

// SOAPConvertOptions configures how SOAP recordings are converted to configs.
type SOAPConvertOptions struct {
	// Deduplicate uses first recording per operation
	Deduplicate bool `json:"deduplicate,omitempty"`

	// IncludeXPathMatch tries to extract XPath match conditions from request
	IncludeXPathMatch bool `json:"includeXPathMatch,omitempty"`

	// IncludeDelay includes recorded latency as delay
	IncludeDelay bool `json:"includeDelay,omitempty"`

	// PreserveFaults includes fault responses as Fault config
	PreserveFaults bool `json:"preserveFaults,omitempty"`
}

// DefaultSOAPConvertOptions returns default conversion options.
func DefaultSOAPConvertOptions() SOAPConvertOptions {
	return SOAPConvertOptions{
		Deduplicate:       true,
		IncludeXPathMatch: false,
		IncludeDelay:      false,
		PreserveFaults:    true,
	}
}

// ToOperationConfig converts a single SOAP recording to an OperationConfig.
func ToOperationConfig(rec *SOAPRecording, opts SOAPConvertOptions) *soap.OperationConfig {
	if rec == nil {
		return nil
	}

	cfg := &soap.OperationConfig{
		SOAPAction: rec.SOAPAction,
		Response:   rec.ResponseBody,
	}

	// Include delay if configured
	if opts.IncludeDelay && rec.Duration > 0 {
		cfg.Delay = rec.Duration.String()
	}

	// Preserve faults if configured
	if opts.PreserveFaults && rec.HasFault {
		cfg.Fault = &soap.SOAPFault{
			Code:    rec.FaultCode,
			Message: rec.FaultMessage,
		}
	}

	// Extract XPath match conditions if configured
	if opts.IncludeXPathMatch && rec.RequestBody != "" {
		match := extractXPathMatch(rec.RequestBody)
		if match != nil && len(match.XPath) > 0 {
			cfg.Match = match
		}
	}

	return cfg
}

// extractXPathMatch attempts to extract XPath match conditions from the request body.
func extractXPathMatch(requestBody string) *soap.SOAPMatch {
	// Parse the SOAP envelope to extract body content
	type envelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Content string `xml:",innerxml"`
		} `xml:"Body"`
	}

	var env envelope
	if err := xml.Unmarshal([]byte(requestBody), &env); err != nil {
		return nil
	}

	if env.Body.Content == "" {
		return nil
	}

	// Extract simple element values from the body
	xpathMap := make(map[string]string)

	// Use regex to find simple element patterns like <ElementName>value</ElementName>
	// This handles common cases without full XPath parsing
	// Note: Go's RE2 doesn't support backreferences, so we capture opening tag and verify closing tag manually
	re := regexp.MustCompile(`<([a-zA-Z][a-zA-Z0-9_]*?)(?:\s[^>]*)?>([^<]+)</([a-zA-Z][a-zA-Z0-9_]*?)>`)
	matches := re.FindAllStringSubmatch(env.Body.Content, -1)

	for _, match := range matches {
		if len(match) == 4 {
			openTag := match[1]
			value := strings.TrimSpace(match[2])
			closeTag := match[3]
			// Verify opening and closing tags match
			if openTag == closeTag && value != "" {
				// Create a simple XPath expression
				xpath := "//" + openTag
				xpathMap[xpath] = value
			}
		}
	}

	if len(xpathMap) == 0 {
		return nil
	}

	return &soap.SOAPMatch{
		XPath: xpathMap,
	}
}

// ToSOAPConfig converts recordings to a complete SOAPConfig.
func ToSOAPConfig(recordings []*SOAPRecording, opts SOAPConvertOptions) *soap.SOAPConfig {
	if len(recordings) == 0 {
		return nil
	}

	// Group recordings by endpoint and operation
	type key struct {
		endpoint  string
		operation string
	}
	groups := make(map[key][]*SOAPRecording)
	order := make([]key, 0)

	for _, rec := range recordings {
		k := key{endpoint: rec.Endpoint, operation: rec.Operation}
		if _, exists := groups[k]; !exists {
			order = append(order, k)
		}
		groups[k] = append(groups[k], rec)
	}

	// Use the first recording's endpoint as the path
	var path string
	if len(recordings) > 0 {
		path = recordings[0].Endpoint
	}

	// Build operations map
	operations := make(map[string]soap.OperationConfig)

	for _, k := range order {
		recs := groups[k]
		var selectedRec *SOAPRecording

		if opts.Deduplicate {
			// Use first recording for each operation
			selectedRec = recs[0]
		} else {
			// Use last recording
			selectedRec = recs[len(recs)-1]
		}

		// Convert to operation config
		opCfg := ToOperationConfig(selectedRec, opts)
		if opCfg != nil {
			operations[k.operation] = *opCfg
		}
	}

	return &soap.SOAPConfig{
		Path:       path,
		Operations: operations,
		Enabled:    true,
	}
}

// SOAPConvertResult contains the result of converting SOAP recordings.
type SOAPConvertResult struct {
	Config         *soap.SOAPConfig `json:"config"`
	OperationCount int              `json:"operationCount"`
	Total          int              `json:"total"`
	Warnings       []string         `json:"warnings,omitempty"`
}

// ConvertSOAPRecordings converts a set of recordings to a SOAPConfig with stats.
func ConvertSOAPRecordings(recordings []*SOAPRecording, opts SOAPConvertOptions) *SOAPConvertResult {
	result := &SOAPConvertResult{
		Total:    len(recordings),
		Warnings: make([]string, 0),
	}

	if len(recordings) == 0 {
		return result
	}

	result.Config = ToSOAPConfig(recordings, opts)

	// Count operations
	if result.Config != nil {
		result.OperationCount = len(result.Config.Operations)
	}

	// Add warnings for fault recordings
	faultCount := 0
	for _, rec := range recordings {
		if rec.HasFault {
			faultCount++
		}
	}
	if faultCount > 0 && !opts.PreserveFaults {
		result.Warnings = append(result.Warnings,
			"Some recordings contained SOAP faults which were not preserved")
	}

	// Add warning for multiple endpoints
	endpoints := make(map[string]bool)
	for _, rec := range recordings {
		endpoints[rec.Endpoint] = true
	}
	if len(endpoints) > 1 {
		result.Warnings = append(result.Warnings,
			"Multiple endpoints detected; only the first endpoint path was used")
	}

	return result
}

// MergeSOAPConfigs merges recordings into an existing SOAPConfig.
func MergeSOAPConfigs(base *soap.SOAPConfig, recordings []*SOAPRecording, opts SOAPConvertOptions) *soap.SOAPConfig {
	if base == nil {
		return ToSOAPConfig(recordings, opts)
	}

	newConfig := ToSOAPConfig(recordings, opts)
	if newConfig == nil {
		return base
	}

	// Initialize operations map if nil
	if base.Operations == nil {
		base.Operations = make(map[string]soap.OperationConfig)
	}

	// Merge new operations into base
	for opName, opCfg := range newConfig.Operations {
		base.Operations[opName] = opCfg
	}

	return base
}
