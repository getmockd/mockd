package soap

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/beevik/etree"
	"github.com/getmockd/mockd/pkg/metrics"
	"github.com/getmockd/mockd/pkg/protocol"
	"github.com/getmockd/mockd/pkg/requestlog"
	"github.com/getmockd/mockd/pkg/template"
	"github.com/getmockd/mockd/pkg/util"
)

// Interface compliance checks.
var (
	_ protocol.Handler         = (*Handler)(nil)
	_ protocol.HTTPHandler     = (*Handler)(nil)
	_ protocol.Recordable      = (*Handler)(nil)
	_ protocol.RequestLoggable = (*Handler)(nil)
)

// SOAPRecordingData contains all data needed to create a recording.
type SOAPRecordingData struct {
	Endpoint        string
	Operation       string
	SOAPAction      string
	SOAPVersion     string
	RequestBody     string
	ResponseBody    string
	ResponseStatus  int
	RequestHeaders  map[string]string
	ResponseHeaders map[string]string
	Duration        time.Duration
	HasFault        bool
	FaultCode       string
	FaultMessage    string
}

// SOAPRecordingStore is the interface for storing SOAP recordings.
type SOAPRecordingStore interface {
	Add(SOAPRecordingData) error
}

// Handler handles SOAP HTTP requests.
type Handler struct {
	config           *SOAPConfig
	wsdlData         []byte
	recordingEnabled bool
	recordingStore   SOAPRecordingStore
	recordingMu      sync.RWMutex
	requestLogger    requestlog.Logger
	loggerMu         sync.RWMutex
	templateEngine   *template.Engine
}

// NewHandler creates a new SOAP handler with the given configuration.
// Returns an error if a WSDL file is configured but cannot be loaded.
func NewHandler(config *SOAPConfig) (*Handler, error) {
	h := &Handler{
		config:         config,
		templateEngine: template.New(),
	}

	// Load WSDL data
	if err := h.loadWSDL(); err != nil {
		return nil, err
	}

	return h, nil
}

// loadWSDL loads WSDL content from file or inline config.
// Returns an error if a file is configured but cannot be read.
func (h *Handler) loadWSDL() error {
	if h.config.WSDLFile != "" {
		// Prevent path traversal and absolute path attacks
		cleanPath, safe := util.SafeFilePathAllowAbsolute(h.config.WSDLFile)
		if !safe {
			return fmt.Errorf("unsafe path in WSDLFile (traversal detected): %q", h.config.WSDLFile)
		}
		data, err := os.ReadFile(cleanPath)
		if err != nil {
			return fmt.Errorf("failed to load WSDL file %q: %w", cleanPath, err)
		}
		h.wsdlData = data
	} else if h.config.WSDL != "" {
		h.wsdlData = []byte(h.config.WSDL)
	}
	return nil
}

// ServeHTTP implements the http.Handler interface.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle WSDL request - check if ?wsdl query param is present (case-insensitive)
	for key := range r.URL.Query() {
		if strings.EqualFold(key, "wsdl") {
			h.serveWSDL(w, r)
			return
		}
	}

	// Capture start time for recording
	startTime := time.Now()

	// Capture request headers for recording
	requestHeaders := make(map[string]string)
	for key, values := range r.Header {
		if len(values) > 0 {
			requestHeaders[key] = values[0]
		}
	}

	// Only accept POST for SOAP operations
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Read request body (bounded to prevent memory exhaustion)
	const maxSOAPBodySize = 10 << 20 // 10MB
	body, err := io.ReadAll(io.LimitReader(r.Body, maxSOAPBodySize))
	if err != nil {
		h.writeFault(w, &SOAPFault{
			Code:    "soap:Client",
			Message: "Failed to read request body",
		}, SOAP11)
		return
	}
	defer func() { _ = r.Body.Close() }()

	// Parse SOAP envelope
	doc, err := h.parseEnvelope(body)
	if err != nil {
		fault := &SOAPFault{
			Code:    "soap:Client",
			Message: "Failed to parse SOAP envelope: " + err.Error(),
		}
		h.writeFaultWithRecording(w, fault, SOAP11, startTime, r.URL.Path, "", "", string(body), requestHeaders, r)
		return
	}

	// Detect SOAP version
	version := h.detectSOAPVersion(doc)

	// Get SOAPAction header
	soapAction := h.getSOAPAction(r, version)

	// Extract operation name
	opName, err := h.extractOperation(doc, soapAction)
	if err != nil {
		fault := &SOAPFault{
			Code:    "soap:Client",
			Message: "Failed to determine operation: " + err.Error(),
		}
		h.writeFaultWithRecording(w, fault, version, startTime, r.URL.Path, "", soapAction, string(body), requestHeaders, r)
		return
	}

	// Find matching operation config
	opConfig := h.matchOperation(opName, doc)
	if opConfig == nil {
		fault := &SOAPFault{
			Code:    "soap:Client",
			Message: "Unknown operation: " + opName,
		}
		h.writeFaultWithRecording(w, fault, version, startTime, r.URL.Path, opName, soapAction, string(body), requestHeaders, r)
		return
	}

	// Apply delay if configured
	if opConfig.Delay != "" {
		if delay, err := parseDuration(opConfig.Delay); err == nil {
			time.Sleep(delay)
		}
	}

	// Return fault response if configured
	if opConfig.Fault != nil {
		h.writeFaultWithRecording(w, opConfig.Fault, version, startTime, r.URL.Path, opName, soapAction, string(body), requestHeaders, r)
		return
	}

	// Build and send response
	responseBody, err := h.buildResponse(opConfig.Response, doc, r, body)
	if err != nil {
		fault := &SOAPFault{
			Code:    "soap:Server",
			Message: "Failed to build response: " + err.Error(),
		}
		h.writeFaultWithRecording(w, fault, version, startTime, r.URL.Path, opName, soapAction, string(body), requestHeaders, r)
		return
	}

	h.writeResponseWithRecording(w, responseBody, version, startTime, r.URL.Path, opName, soapAction, string(body), requestHeaders, r)
}

// serveWSDL serves the WSDL document.
func (h *Handler) serveWSDL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	if h.wsdlData == nil {
		h.writeError(w, http.StatusNotFound, "WSDL not available")
		return
	}

	w.Header().Set("Content-Type", "text/xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.wsdlData)
}

// parseEnvelope parses a SOAP envelope from the request body.
func (h *Handler) parseEnvelope(body []byte) (*etree.Document, error) {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(body); err != nil {
		return nil, fmt.Errorf("invalid XML: %w", err)
	}

	// Validate it's a SOAP envelope
	root := doc.Root()
	if root == nil {
		return nil, errors.New("empty document")
	}

	if root.Tag != "Envelope" {
		return nil, fmt.Errorf("root element must be Envelope, got %s", root.Tag)
	}

	// Check namespace
	ns := root.Space
	if ns == "" {
		// Check for namespace attribute
		for _, attr := range root.Attr {
			if attr.Key == "xmlns" || strings.HasPrefix(attr.Key, "xmlns:") {
				if attr.Value == SOAP11Namespace || attr.Value == SOAP12Namespace {
					break
				}
			}
		}
	}

	return doc, nil
}

// detectSOAPVersion detects the SOAP version from the envelope namespace.
func (h *Handler) detectSOAPVersion(doc *etree.Document) SOAPVersion {
	root := doc.Root()
	if root == nil {
		return SOAP11
	}

	// Check namespace attributes
	for _, attr := range root.Attr {
		if strings.HasPrefix(attr.Key, "xmlns") {
			if attr.Value == SOAP12Namespace {
				return SOAP12
			}
		}
	}

	// Check element namespace
	if root.NamespaceURI() == SOAP12Namespace {
		return SOAP12
	}

	return SOAP11
}

// getSOAPAction extracts the SOAPAction from request headers.
func (h *Handler) getSOAPAction(r *http.Request, version SOAPVersion) string {
	if version == SOAP12 {
		// SOAP 1.2 uses action parameter in Content-Type
		contentType := r.Header.Get("Content-Type")
		if strings.Contains(contentType, "action=") {
			parts := strings.Split(contentType, ";")
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if strings.HasPrefix(part, "action=") {
					action := strings.TrimPrefix(part, "action=")
					action = strings.Trim(action, "\"")
					return action
				}
			}
		}
	}

	// SOAP 1.1 uses SOAPAction header
	action := r.Header.Get("SOAPAction")
	// Remove quotes if present
	action = strings.Trim(action, "\"")
	return action
}

// extractOperation determines which operation was called.
func (h *Handler) extractOperation(doc *etree.Document, soapAction string) (string, error) {
	// First try to match by SOAPAction
	if soapAction != "" {
		for name, op := range h.config.Operations {
			if op.SOAPAction == soapAction {
				return name, nil
			}
		}
	}

	// Fall back to extracting operation from Body
	body := doc.FindElement("//Body")
	if body == nil {
		// Try with namespace prefix
		body = doc.FindElement("//*[local-name()='Body']")
	}

	if body == nil {
		return "", errors.New("SOAP Body not found")
	}

	// Get first child of Body (the operation element)
	children := body.ChildElements()
	if len(children) == 0 {
		return "", errors.New("no operation element found in Body")
	}

	child := children[0]
	opName := child.Tag
	// Try to match operation name
	if _, ok := h.config.Operations[opName]; ok {
		return opName, nil
	}
	// Try without namespace prefix
	if idx := strings.LastIndex(opName, ":"); idx >= 0 {
		opName = opName[idx+1:]
		if _, ok := h.config.Operations[opName]; ok {
			return opName, nil
		}
	}
	// Return first child name even if not matched
	return child.Tag, nil
}

// matchOperation finds the matching operation config.
func (h *Handler) matchOperation(opName string, doc *etree.Document) *OperationConfig {
	// Direct match
	if op, ok := h.config.Operations[opName]; ok {
		// Check XPath match conditions
		if op.Match != nil && len(op.Match.XPath) > 0 {
			if !MatchXPath(doc, op.Match.XPath) {
				return nil
			}
		}
		return &op
	}

	// Try without namespace prefix
	if idx := strings.LastIndex(opName, ":"); idx >= 0 {
		shortName := opName[idx+1:]
		if op, ok := h.config.Operations[shortName]; ok {
			if op.Match != nil && len(op.Match.XPath) > 0 {
				if !MatchXPath(doc, op.Match.XPath) {
					return nil
				}
			}
			return &op
		}
	}

	return nil
}

// buildResponse builds the SOAP response XML from a template.
// It processes both XPath variables ({{xpath:/path}}) and general template
// variables ({{uuid}}, {{now}}, {{request.header.X-Custom}}, etc.).
//
//nolint:unparam // error is always nil but kept for future validation
func (h *Handler) buildResponse(responseTemplate string, requestDoc *etree.Document, r *http.Request, body []byte) ([]byte, error) {
	// First, process XPath variables from the SOAP request
	result := processTemplate(responseTemplate, requestDoc)

	// Then process general template variables using the template engine
	ctx := template.NewContext(r, body)
	result, _ = h.templateEngine.Process(result, ctx)

	return []byte(result), nil
}

// processTemplate replaces {{xpath:/path}} variables in the template.
var xpathVarRegex = regexp.MustCompile(`\{\{xpath:([^}]+)\}\}`)

func processTemplate(template string, doc *etree.Document) string {
	return xpathVarRegex.ReplaceAllStringFunc(template, func(match string) string {
		// Extract XPath from {{xpath:/path/to/element}}
		submatch := xpathVarRegex.FindStringSubmatch(match)
		if len(submatch) < 2 {
			return match
		}
		xpath := submatch[1]
		value := ExtractXPath(doc, xpath)
		return value
	})
}

// writeFault writes a SOAP fault response.
func (h *Handler) writeFault(w http.ResponseWriter, fault *SOAPFault, version SOAPVersion) {
	var faultXML []byte

	if version == SOAP12 {
		faultXML = h.buildFault12(fault)
		w.Header().Set("Content-Type", SOAP12ContentType)
	} else {
		faultXML = h.buildFault11(fault)
		w.Header().Set("Content-Type", SOAP11ContentType)
	}

	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(faultXML)
}

// writeFaultWithRecording writes a SOAP fault response and records if enabled.
func (h *Handler) writeFaultWithRecording(w http.ResponseWriter, fault *SOAPFault, version SOAPVersion, startTime time.Time, endpoint, operation, soapAction, requestBody string, requestHeaders map[string]string, r *http.Request) {
	var faultXML []byte
	var contentType string

	if version == SOAP12 {
		faultXML = h.buildFault12(fault)
		contentType = SOAP12ContentType
	} else {
		faultXML = h.buildFault11(fault)
		contentType = SOAP11ContentType
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write(faultXML)

	duration := time.Since(startTime)
	responseStr := string(faultXML)

	// Record metrics
	h.recordMetrics(endpoint, http.StatusInternalServerError, duration)

	// Record the fault response
	h.recordRequest(SOAPRecordingData{
		Endpoint:        endpoint,
		Operation:       operation,
		SOAPAction:      soapAction,
		SOAPVersion:     string(version),
		RequestBody:     requestBody,
		ResponseBody:    responseStr,
		ResponseStatus:  http.StatusInternalServerError,
		RequestHeaders:  requestHeaders,
		ResponseHeaders: map[string]string{"Content-Type": contentType},
		Duration:        duration,
		HasFault:        true,
		FaultCode:       fault.Code,
		FaultMessage:    fault.Message,
	})

	// Log the request
	h.logRequest(&requestlog.Entry{
		Timestamp:      startTime,
		Protocol:       requestlog.ProtocolSOAP,
		Method:         "POST",
		Path:           h.config.Path,
		QueryString:    r.URL.RawQuery,
		Headers:        r.Header,
		Body:           util.TruncateBody(requestBody, 0),
		BodySize:       len(requestBody),
		RemoteAddr:     r.RemoteAddr,
		ResponseStatus: http.StatusInternalServerError,
		ResponseBody:   util.TruncateBody(responseStr, 0),
		DurationMs:     int(duration.Milliseconds()),
		SOAP: &requestlog.SOAPMeta{
			Operation:   operation,
			SOAPAction:  soapAction,
			SOAPVersion: string(version),
			IsFault:     true,
			FaultCode:   fault.Code,
		},
	})
}

// buildFault11 builds a SOAP 1.1 fault response.
func (h *Handler) buildFault11(fault *SOAPFault) []byte {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString(`<soap:Envelope xmlns:soap="` + SOAP11Namespace + `">`)
	buf.WriteString(`<soap:Body>`)
	buf.WriteString(`<soap:Fault>`)
	buf.WriteString(`<faultcode>` + escapeXML(fault.Code) + `</faultcode>`)
	buf.WriteString(`<faultstring>` + escapeXML(fault.Message) + `</faultstring>`)
	if fault.Detail != "" {
		buf.WriteString(`<detail>` + fault.Detail + `</detail>`)
	}
	buf.WriteString(`</soap:Fault>`)
	buf.WriteString(`</soap:Body>`)
	buf.WriteString(`</soap:Envelope>`)
	return buf.Bytes()
}

// buildFault12 builds a SOAP 1.2 fault response.
func (h *Handler) buildFault12(fault *SOAPFault) []byte {
	// Map common fault codes to SOAP 1.2 codes
	code := fault.Code
	switch code {
	case "soap:Client", "Client":
		code = "soap:Sender"
	case "soap:Server", "Server":
		code = "soap:Receiver"
	}

	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString(`<soap:Envelope xmlns:soap="` + SOAP12Namespace + `">`)
	buf.WriteString(`<soap:Body>`)
	buf.WriteString(`<soap:Fault>`)
	buf.WriteString(`<soap:Code><soap:Value>` + escapeXML(code) + `</soap:Value></soap:Code>`)
	buf.WriteString(`<soap:Reason><soap:Text xml:lang="en">` + escapeXML(fault.Message) + `</soap:Text></soap:Reason>`)
	if fault.Detail != "" {
		buf.WriteString(`<soap:Detail>` + fault.Detail + `</soap:Detail>`)
	}
	buf.WriteString(`</soap:Fault>`)
	buf.WriteString(`</soap:Body>`)
	buf.WriteString(`</soap:Envelope>`)
	return buf.Bytes()
}

// writeResponseWithRecording writes a successful SOAP response and records if enabled.
func (h *Handler) writeResponseWithRecording(w http.ResponseWriter, body []byte, version SOAPVersion, startTime time.Time, endpoint, operation, soapAction, requestBody string, requestHeaders map[string]string, r *http.Request) {
	var response bytes.Buffer
	var ns string
	var contentType string

	if version == SOAP12 {
		ns = SOAP12Namespace
		contentType = SOAP12ContentType
	} else {
		ns = SOAP11Namespace
		contentType = SOAP11ContentType
	}

	w.Header().Set("Content-Type", contentType)

	response.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	response.WriteString(`<soap:Envelope xmlns:soap="` + ns + `">`)
	response.WriteString(`<soap:Body>`)
	response.Write(body)
	response.WriteString(`</soap:Body>`)
	response.WriteString(`</soap:Envelope>`)

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(response.Bytes())

	duration := time.Since(startTime)
	responseStr := response.String()

	// Record metrics
	h.recordMetrics(endpoint, http.StatusOK, duration)

	// Record the successful response
	h.recordRequest(SOAPRecordingData{
		Endpoint:        endpoint,
		Operation:       operation,
		SOAPAction:      soapAction,
		SOAPVersion:     string(version),
		RequestBody:     requestBody,
		ResponseBody:    responseStr,
		ResponseStatus:  http.StatusOK,
		RequestHeaders:  requestHeaders,
		ResponseHeaders: map[string]string{"Content-Type": contentType},
		Duration:        duration,
		HasFault:        false,
		FaultCode:       "",
		FaultMessage:    "",
	})

	// Log the request
	h.logRequest(&requestlog.Entry{
		Timestamp:      startTime,
		Protocol:       requestlog.ProtocolSOAP,
		Method:         "POST",
		Path:           h.config.Path,
		QueryString:    r.URL.RawQuery,
		Headers:        r.Header,
		Body:           util.TruncateBody(requestBody, 0),
		BodySize:       len(requestBody),
		RemoteAddr:     r.RemoteAddr,
		ResponseStatus: http.StatusOK,
		ResponseBody:   util.TruncateBody(responseStr, 0),
		DurationMs:     int(duration.Milliseconds()),
		SOAP: &requestlog.SOAPMeta{
			Operation:   operation,
			SOAPAction:  soapAction,
			SOAPVersion: string(version),
			IsFault:     false,
			FaultCode:   "",
		},
	})
}

// writeError writes an HTTP error response.
func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(message))
}

// escapeXML escapes special XML characters.
func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// parseDuration parses a duration string (supports "100ms", "1s", etc.)
func parseDuration(s string) (time.Duration, error) {
	// Try standard Go duration format first
	d, err := time.ParseDuration(s)
	if err == nil {
		return d, nil
	}

	// Try parsing as milliseconds number
	ms, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return time.Duration(ms) * time.Millisecond, nil
	}

	return 0, fmt.Errorf("invalid duration: %s", s)
}

// GetConfig returns the handler's configuration.
func (h *Handler) GetConfig() *SOAPConfig {
	return h.config
}

// BuildFault builds a SOAP fault response (public method for external use).
func (h *Handler) BuildFault(fault *SOAPFault) []byte {
	return h.buildFault11(fault)
}

// ID returns the handler ID.
func (h *Handler) ID() string {
	if h.config == nil {
		return ""
	}
	return h.config.ID
}

// EnableRecording enables recording for this handler.
func (h *Handler) EnableRecording() {
	h.recordingMu.Lock()
	defer h.recordingMu.Unlock()
	h.recordingEnabled = true
}

// DisableRecording disables recording for this handler.
func (h *Handler) DisableRecording() {
	h.recordingMu.Lock()
	defer h.recordingMu.Unlock()
	h.recordingEnabled = false
}

// IsRecordingEnabled returns whether recording is enabled.
func (h *Handler) IsRecordingEnabled() bool {
	h.recordingMu.RLock()
	defer h.recordingMu.RUnlock()
	return h.recordingEnabled
}

// SetRecordingStore sets the recording store.
func (h *Handler) SetRecordingStore(store SOAPRecordingStore) {
	h.recordingMu.Lock()
	defer h.recordingMu.Unlock()
	h.recordingStore = store
}

// GetRecordingStore returns the recording store.
func (h *Handler) GetRecordingStore() SOAPRecordingStore {
	h.recordingMu.RLock()
	defer h.recordingMu.RUnlock()
	return h.recordingStore
}

// SetRequestLogger sets the request logger for lightweight request logging.
func (h *Handler) SetRequestLogger(logger requestlog.Logger) {
	h.loggerMu.Lock()
	defer h.loggerMu.Unlock()
	h.requestLogger = logger
}

// GetRequestLogger returns the request logger.
func (h *Handler) GetRequestLogger() requestlog.Logger {
	h.loggerMu.RLock()
	defer h.loggerMu.RUnlock()
	return h.requestLogger
}

// recordRequest records the request/response data asynchronously if recording is enabled.
func (h *Handler) recordRequest(data SOAPRecordingData) {
	h.recordingMu.RLock()
	enabled := h.recordingEnabled
	store := h.recordingStore
	h.recordingMu.RUnlock()

	if !enabled || store == nil {
		return
	}

	// Store recording asynchronously
	go func() {
		_ = store.Add(data)
	}()
}

// logRequest logs the request using the request logger if configured.
func (h *Handler) logRequest(entry *requestlog.Entry) {
	h.loggerMu.RLock()
	logger := h.requestLogger
	h.loggerMu.RUnlock()

	if logger == nil {
		return
	}

	logger.Log(entry)
}

// Protocol returns the protocol type for this handler.
func (h *Handler) Protocol() protocol.Protocol {
	return protocol.ProtocolSOAP
}

// Metadata returns descriptive information about the handler.
func (h *Handler) Metadata() protocol.Metadata {
	return protocol.Metadata{
		ID:                   h.ID(),
		Name:                 h.config.Name,
		Protocol:             protocol.ProtocolSOAP,
		Version:              "0.2.4",
		TransportType:        protocol.TransportHTTP1,
		ConnectionModel:      protocol.ConnectionModelStateless,
		CommunicationPattern: protocol.PatternRequestResponse,
		Capabilities: []protocol.Capability{
			protocol.CapabilityRecording,
			protocol.CapabilitySchemaValidation,
			protocol.CapabilityMocking,
		},
	}
}

// Start activates the handler. For HTTP handlers, this is typically a no-op
// as they are registered with an HTTP server that manages the lifecycle.
func (h *Handler) Start(ctx context.Context) error {
	return nil
}

// Stop gracefully shuts down the handler. For HTTP handlers, this is typically
// a no-op as the HTTP server manages connection draining.
func (h *Handler) Stop(ctx context.Context, timeout time.Duration) error {
	return nil
}

// Health returns the current health status of the handler.
func (h *Handler) Health(ctx context.Context) protocol.HealthStatus {
	return protocol.HealthStatus{
		Status:    protocol.HealthHealthy,
		CheckedAt: time.Now(),
	}
}

// Pattern returns the URL pattern this handler serves.
func (h *Handler) Pattern() string {
	if h.config == nil {
		return "/soap"
	}
	return h.config.Path
}

// recordMetrics records SOAP request metrics.
func (h *Handler) recordMetrics(path string, status int, duration time.Duration) {
	statusStr := strconv.Itoa(status)
	if metrics.RequestsTotal != nil {
		if vec, err := metrics.RequestsTotal.WithLabels("soap", path, statusStr); err == nil {
			_ = vec.Inc()
		}
	}
	if metrics.RequestDuration != nil {
		if vec, err := metrics.RequestDuration.WithLabels("soap", path); err == nil {
			vec.Observe(duration.Seconds())
		}
	}
}
