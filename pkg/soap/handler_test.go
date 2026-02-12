package soap

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/getmockd/mockd/pkg/requestlog"
)

// mustNewHandler creates a new handler and fails the test if it errors.
func mustNewHandler(t *testing.T, config *SOAPConfig) *Handler {
	t.Helper()
	handler, err := NewHandler(config)
	if err != nil {
		t.Fatalf("NewHandler failed: %v", err)
	}
	return handler
}

func TestNewHandler(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-service",
		Path: "/service",
		Operations: map[string]OperationConfig{
			"GetUser": {
				Response: "<GetUserResponse><name>John</name></GetUserResponse>",
			},
		},
		Enabled: true,
	}

	handler, err := NewHandler(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler == nil {
		t.Fatal("expected handler to be created")
	}
	if handler.config != config {
		t.Error("expected config to be set")
	}
}

func TestNewHandler_WSDLFileNotFound(t *testing.T) {
	config := &SOAPConfig{
		ID:       "test-service",
		Path:     "/service",
		WSDLFile: "/non/existent/file.wsdl",
		Enabled:  true,
	}

	handler, err := NewHandler(config)
	if err == nil {
		t.Fatal("expected error for non-existent WSDL file")
	}
	if handler != nil {
		t.Error("expected nil handler when WSDL file not found")
	}
	if !strings.Contains(err.Error(), "failed to load WSDL file") {
		t.Errorf("expected error message about WSDL file, got: %v", err)
	}
}

func TestNewHandler_WSDLFileSuccess(t *testing.T) {
	// Create a temporary WSDL file
	tmpDir := t.TempDir()
	wsdlPath := filepath.Join(tmpDir, "test.wsdl")
	wsdlContent := `<?xml version="1.0"?><definitions name="TestService"></definitions>`
	if err := os.WriteFile(wsdlPath, []byte(wsdlContent), 0644); err != nil {
		t.Fatalf("failed to write temp WSDL: %v", err)
	}

	config := &SOAPConfig{
		ID:       "test-service",
		Path:     "/service",
		WSDLFile: wsdlPath,
		Enabled:  true,
	}

	handler, err := NewHandler(config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler == nil {
		t.Fatal("expected handler to be created")
	}
	if string(handler.wsdlData) != wsdlContent {
		t.Errorf("expected WSDL content %q, got %q", wsdlContent, string(handler.wsdlData))
	}
}

func TestHandler_ServeHTTP_WSDL(t *testing.T) {
	wsdlContent := `<?xml version="1.0"?><definitions name="TestService"></definitions>`
	config := &SOAPConfig{
		ID:      "test-service",
		Path:    "/service",
		WSDL:    wsdlContent,
		Enabled: true,
	}

	handler := mustNewHandler(t, config)

	req := httptest.NewRequest(http.MethodGet, "/service?wsdl", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != wsdlContent {
		t.Errorf("expected WSDL content, got %s", string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/xml") {
		t.Errorf("expected Content-Type text/xml, got %s", contentType)
	}
}

func TestHandler_ServeHTTP_MethodNotAllowed(t *testing.T) {
	config := &SOAPConfig{
		ID:      "test-service",
		Path:    "/service",
		Enabled: true,
	}

	handler := mustNewHandler(t, config)

	req := httptest.NewRequest(http.MethodGet, "/service", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", resp.StatusCode)
	}
}

func TestHandler_ServeHTTP_SOAP11Request(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-service",
		Path: "/service",
		Operations: map[string]OperationConfig{
			"GetUser": {
				SOAPAction: "http://example.com/GetUser",
				Response:   "<GetUserResponse><name>John Doe</name></GetUserResponse>",
			},
		},
		Enabled: true,
	}

	handler := mustNewHandler(t, config)

	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser xmlns="http://example.com/">
      <id>123</id>
    </GetUser>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/service", strings.NewReader(soapRequest))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "http://example.com/GetUser")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<name>John Doe</name>") {
		t.Errorf("expected response to contain name, got %s", string(body))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/xml") {
		t.Errorf("expected Content-Type text/xml, got %s", contentType)
	}
}

func TestHandler_ServeHTTP_SOAP12Request(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-service",
		Path: "/service",
		Operations: map[string]OperationConfig{
			"GetUser": {
				Response: "<GetUserResponse><name>Jane Doe</name></GetUserResponse>",
			},
		},
		Enabled: true,
	}

	handler := mustNewHandler(t, config)

	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <GetUser xmlns="http://example.com/">
      <id>456</id>
    </GetUser>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/service", strings.NewReader(soapRequest))
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "<name>Jane Doe</name>") {
		t.Errorf("expected response to contain name, got %s", string(body))
	}

	// SOAP 1.2 should return application/soap+xml
	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/soap+xml") {
		t.Errorf("expected Content-Type application/soap+xml, got %s", contentType)
	}
}

func TestHandler_ServeHTTP_WithXPathTemplate(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-service",
		Path: "/service",
		Operations: map[string]OperationConfig{
			"GetUser": {
				Response: "<GetUserResponse><requestedId>{{xpath://GetUser/id}}</requestedId></GetUserResponse>",
			},
		},
		Enabled: true,
	}

	handler := mustNewHandler(t, config)

	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser>
      <id>user-789</id>
    </GetUser>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/service", strings.NewReader(soapRequest))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if !strings.Contains(string(body), "<requestedId>user-789</requestedId>") {
		t.Errorf("expected XPath substitution, got %s", string(body))
	}
}

func TestHandler_ServeHTTP_FaultResponse(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-service",
		Path: "/service",
		Operations: map[string]OperationConfig{
			"Divide": {
				Fault: &SOAPFault{
					Code:    "soap:Client",
					Message: "Division by zero",
					Detail:  "<errorCode>MATH_ERROR</errorCode>",
				},
			},
		},
		Enabled: true,
	}

	handler := mustNewHandler(t, config)

	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <Divide>
      <a>10</a>
      <b>0</b>
    </Divide>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/service", strings.NewReader(soapRequest))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500 for fault, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Division by zero") {
		t.Errorf("expected fault message, got %s", string(body))
	}
	if !strings.Contains(string(body), "<faultcode>soap:Client</faultcode>") {
		t.Errorf("expected fault code, got %s", string(body))
	}
}

func TestHandler_ServeHTTP_SOAP12Fault(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-service",
		Path: "/service",
		Operations: map[string]OperationConfig{
			"Divide": {
				Fault: &SOAPFault{
					Code:    "soap:Client",
					Message: "Division by zero",
				},
			},
		},
		Enabled: true,
	}

	handler := mustNewHandler(t, config)

	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <Divide>
      <a>10</a>
      <b>0</b>
    </Divide>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/service", strings.NewReader(soapRequest))
	req.Header.Set("Content-Type", "application/soap+xml; charset=utf-8")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	// SOAP 1.2 should use Sender instead of Client
	if !strings.Contains(string(body), "soap:Sender") {
		t.Errorf("expected SOAP 1.2 fault code (soap:Sender), got %s", string(body))
	}
	if !strings.Contains(string(body), "<soap:Text") {
		t.Errorf("expected SOAP 1.2 fault format, got %s", string(body))
	}
}

func TestHandler_ServeHTTP_UnknownOperation(t *testing.T) {
	config := &SOAPConfig{
		ID:         "test-service",
		Path:       "/service",
		Operations: map[string]OperationConfig{},
		Enabled:    true,
	}

	handler := mustNewHandler(t, config)

	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <UnknownOp></UnknownOp>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/service", strings.NewReader(soapRequest))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500 for unknown operation, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Unknown operation") {
		t.Errorf("expected unknown operation fault, got %s", string(body))
	}
}

func TestHandler_ServeHTTP_InvalidXML(t *testing.T) {
	config := &SOAPConfig{
		ID:      "test-service",
		Path:    "/service",
		Enabled: true,
	}

	handler := mustNewHandler(t, config)

	req := httptest.NewRequest(http.MethodPost, "/service", strings.NewReader("not xml"))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500 for invalid XML, got %d", resp.StatusCode)
	}
}

func TestHandler_ServeHTTP_XPathMatching(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-service",
		Path: "/service",
		Operations: map[string]OperationConfig{
			"Divide": {
				Match: &SOAPMatch{
					XPath: map[string]string{
						"//Divide/b": "0",
					},
				},
				Fault: &SOAPFault{
					Code:    "soap:Client",
					Message: "Division by zero",
				},
			},
		},
		Enabled: true,
	}

	handler := mustNewHandler(t, config)

	// Request with b=0 should match and return fault
	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <Divide>
      <a>10</a>
      <b>0</b>
    </Divide>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/service", strings.NewReader(soapRequest))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	if !strings.Contains(string(body), "Division by zero") {
		t.Errorf("expected division by zero fault, got %s", string(body))
	}
}

func TestHandler_ServeHTTP_XPathMatchingNoMatch(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-service",
		Path: "/service",
		Operations: map[string]OperationConfig{
			"Divide": {
				Match: &SOAPMatch{
					XPath: map[string]string{
						"//Divide/b": "0",
					},
				},
				Fault: &SOAPFault{
					Code:    "soap:Client",
					Message: "Division by zero",
				},
			},
		},
		Enabled: true,
	}

	handler := mustNewHandler(t, config)

	// Request with b=5 should NOT match
	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <Divide>
      <a>10</a>
      <b>5</b>
    </Divide>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/service", strings.NewReader(soapRequest))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	body, _ := io.ReadAll(resp.Body)

	// Should return condition mismatch fault since operation name matched but XPath didn't
	if !strings.Contains(string(body), "No matching condition for operation") {
		t.Errorf("expected condition mismatch when XPath doesn't match, got %s", string(body))
	}
}

func TestHandler_ServeHTTP_Delay(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-service",
		Path: "/service",
		Operations: map[string]OperationConfig{
			"SlowOp": {
				Delay:    "50ms",
				Response: "<SlowOpResponse>done</SlowOpResponse>",
			},
		},
		Enabled: true,
	}

	handler := mustNewHandler(t, config)

	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <SlowOp></SlowOp>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/service", strings.NewReader(soapRequest))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestHandler_SOAP12ActionInContentType(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-service",
		Path: "/service",
		Operations: map[string]OperationConfig{
			"GetUser": {
				SOAPAction: "http://example.com/GetUser",
				Response:   "<GetUserResponse><name>Test</name></GetUserResponse>",
			},
		},
		Enabled: true,
	}

	handler := mustNewHandler(t, config)

	soapRequest := `<?xml version="1.0" encoding="UTF-8"?>
<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">
  <soap:Body>
    <GetUser><id>1</id></GetUser>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest(http.MethodPost, "/service", strings.NewReader(soapRequest))
	// SOAP 1.2 puts action in Content-Type header
	req.Header.Set("Content-Type", `application/soap+xml; charset=utf-8; action="http://example.com/GetUser"`)

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected status 200, got %d: %s", resp.StatusCode, string(body))
	}
}

// XPath utility tests

func TestMatchXPath(t *testing.T) {
	doc := etree.NewDocument()
	_ = doc.ReadFromString(`<?xml version="1.0"?>
<root>
  <user>
    <name>John</name>
    <age>30</age>
  </user>
</root>`)

	tests := []struct {
		name       string
		conditions map[string]string
		want       bool
	}{
		{
			name:       "empty conditions",
			conditions: map[string]string{},
			want:       true,
		},
		{
			name: "single match",
			conditions: map[string]string{
				"//name": "John",
			},
			want: true,
		},
		{
			name: "single no match",
			conditions: map[string]string{
				"//name": "Jane",
			},
			want: false,
		},
		{
			name: "multiple match",
			conditions: map[string]string{
				"//name": "John",
				"//age":  "30",
			},
			want: true,
		},
		{
			name: "partial match",
			conditions: map[string]string{
				"//name": "John",
				"//age":  "25",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchXPath(doc, tt.conditions)
			if got != tt.want {
				t.Errorf("MatchXPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractXPath(t *testing.T) {
	doc := etree.NewDocument()
	_ = doc.ReadFromString(`<?xml version="1.0"?>
<root>
  <user id="u1">
    <name>John</name>
    <age>30</age>
  </user>
</root>`)

	tests := []struct {
		name  string
		xpath string
		want  string
	}{
		{
			name:  "simple element",
			xpath: "//name",
			want:  "John",
		},
		{
			name:  "absolute path",
			xpath: "/root/user/name",
			want:  "John",
		},
		{
			name:  "attribute",
			xpath: "//user/@id",
			want:  "u1",
		},
		{
			name:  "non-existent",
			xpath: "//missing",
			want:  "",
		},
		{
			name:  "empty xpath",
			xpath: "",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractXPath(doc, tt.xpath)
			if got != tt.want {
				t.Errorf("ExtractXPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildXPath(t *testing.T) {
	tests := []struct {
		name     string
		segments []string
		want     string
	}{
		{
			name:     "empty",
			segments: []string{},
			want:     "",
		},
		{
			name:     "single",
			segments: []string{"root"},
			want:     "/root",
		},
		{
			name:     "multiple",
			segments: []string{"root", "user", "name"},
			want:     "/root/user/name",
		},
		{
			name:     "with attribute",
			segments: []string{"root", "user", "@id"},
			want:     "/root/user/@id",
		},
		{
			name:     "absolute start",
			segments: []string{"/root", "user"},
			want:     "/root/user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildXPath(tt.segments...)
			if got != tt.want {
				t.Errorf("BuildXPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeXPath(t *testing.T) {
	tests := []struct {
		name  string
		xpath string
		want  string
	}{
		{
			name:  "empty",
			xpath: "",
			want:  "",
		},
		{
			name:  "already normalized",
			xpath: "/root/user",
			want:  "/root/user",
		},
		{
			name:  "missing leading slash",
			xpath: "root/user",
			want:  "/root/user",
		},
		{
			name:  "double slash descendant",
			xpath: "//user",
			want:  "//user",
		},
		{
			name:  "whitespace",
			xpath: "  /root/user  ",
			want:  "/root/user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeXPath(tt.xpath)
			if got != tt.want {
				t.Errorf("NormalizeXPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input   string
		wantMs  int64
		wantErr bool
	}{
		{"100ms", 100, false},
		{"1s", 1000, false},
		{"100", 100, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseDuration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.Milliseconds() != tt.wantMs {
				t.Errorf("parseDuration() = %v, want %v ms", got, tt.wantMs)
			}
		})
	}
}

func TestProcessTemplate(t *testing.T) {
	doc := etree.NewDocument()
	doc.ReadFromString(`<?xml version="1.0"?>
<root>
  <Request>
    <id>123</id>
    <name>Test</name>
  </Request>
</root>`)

	tests := []struct {
		name     string
		template string
		want     string
	}{
		{
			name:     "no variables",
			template: "<Response>OK</Response>",
			want:     "<Response>OK</Response>",
		},
		{
			name:     "single variable",
			template: "<Response><id>{{xpath://Request/id}}</id></Response>",
			want:     "<Response><id>123</id></Response>",
		},
		{
			name:     "multiple variables",
			template: "<Response><id>{{xpath://Request/id}}</id><name>{{xpath://Request/name}}</name></Response>",
			want:     "<Response><id>123</id><name>Test</name></Response>",
		},
		{
			name:     "non-existent path",
			template: "<Response>{{xpath://missing}}</Response>",
			want:     "<Response></Response>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processTemplate(tt.template, doc)
			if got != tt.want {
				t.Errorf("processTemplate() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"<test>", "&lt;test&gt;"},
		{"a & b", "a &amp; b"},
		{`"quoted"`, "&quot;quoted&quot;"},
		{"it's", "it&apos;s"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeXML(tt.input)
			if got != tt.want {
				t.Errorf("escapeXML() = %q, want %q", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Session 13: SOAP Recording Integration Tests
// =============================================================================

// mockSOAPStore is a test double for SOAPRecordingStore.
type mockSOAPStore struct {
	mu         sync.Mutex
	recordings []SOAPRecordingData
	addErr     error // inject errors
}

func (s *mockSOAPStore) Add(data SOAPRecordingData) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.addErr != nil {
		return s.addErr
	}
	s.recordings = append(s.recordings, data)
	return nil
}

func (s *mockSOAPStore) getRecordings() []SOAPRecordingData {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]SOAPRecordingData, len(s.recordings))
	copy(result, s.recordings)
	return result
}

// mockRequestLogger is a test double for requestlog.Logger.
type mockRequestLogger struct {
	mu      sync.Mutex
	entries []*requestlog.Entry
}

func (l *mockRequestLogger) Log(entry *requestlog.Entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
}

func (l *mockRequestLogger) getEntries() []*requestlog.Entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]*requestlog.Entry, len(l.entries))
	copy(result, l.entries)
	return result
}

// --- Recording enable/disable tests ---

func TestHandler_RecordingToggle(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-rec",
		Path: "/soap",
		Operations: map[string]OperationConfig{
			"Ping": {Response: "<PingResponse>pong</PingResponse>"},
		},
		Enabled: true,
	}
	handler := mustNewHandler(t, config)

	// Initially disabled
	if handler.IsRecordingEnabled() {
		t.Error("Recording should be disabled by default")
	}

	// Enable
	handler.EnableRecording()
	if !handler.IsRecordingEnabled() {
		t.Error("Recording should be enabled after EnableRecording()")
	}

	// Disable
	handler.DisableRecording()
	if handler.IsRecordingEnabled() {
		t.Error("Recording should be disabled after DisableRecording()")
	}
}

func TestHandler_SetRecordingStore(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-store",
		Path: "/soap",
		Operations: map[string]OperationConfig{
			"Ping": {Response: "<PingResponse>pong</PingResponse>"},
		},
		Enabled: true,
	}
	handler := mustNewHandler(t, config)

	// Initially nil
	if handler.GetRecordingStore() != nil {
		t.Error("Recording store should be nil by default")
	}

	// Set store
	store := &mockSOAPStore{}
	handler.SetRecordingStore(store)
	if handler.GetRecordingStore() == nil {
		t.Error("Recording store should be set after SetRecordingStore()")
	}
}

func TestHandler_SetRequestLogger(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-logger",
		Path: "/soap",
		Operations: map[string]OperationConfig{
			"Ping": {Response: "<PingResponse>pong</PingResponse>"},
		},
		Enabled: true,
	}
	handler := mustNewHandler(t, config)

	// Initially nil
	if handler.GetRequestLogger() != nil {
		t.Error("Request logger should be nil by default")
	}

	// Set logger
	logger := &mockRequestLogger{}
	handler.SetRequestLogger(logger)
	if handler.GetRequestLogger() == nil {
		t.Error("Request logger should be set after SetRequestLogger()")
	}
}

// --- Recording with actual SOAP requests ---

func TestHandler_RecordingCapturesRequest(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-capture",
		Path: "/soap/service",
		Operations: map[string]OperationConfig{
			"GetUser": {
				Response: "<GetUserResponse><name>Alice</name></GetUserResponse>",
			},
		},
		Enabled: true,
	}
	handler := mustNewHandler(t, config)

	store := &mockSOAPStore{}
	handler.SetRecordingStore(store)
	handler.EnableRecording()

	// Send a valid SOAP 1.1 request
	soapBody := `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser><id>123</id></GetUser>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest("POST", "/soap/service", strings.NewReader(soapBody))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "GetUser")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Recording is async (goroutine) — wait briefly
	time.Sleep(50 * time.Millisecond)

	recordings := store.getRecordings()
	if len(recordings) != 1 {
		t.Fatalf("Expected 1 recording, got %d", len(recordings))
	}

	rec := recordings[0]
	if rec.Operation != "GetUser" {
		t.Errorf("Expected operation 'GetUser', got %q", rec.Operation)
	}
	if rec.SOAPVersion != "1.1" {
		t.Errorf("Expected SOAP version '1.1', got %q", rec.SOAPVersion)
	}
	if rec.ResponseStatus != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.ResponseStatus)
	}
	if rec.HasFault {
		t.Error("Expected HasFault=false for successful response")
	}
	if rec.RequestBody == "" {
		t.Error("Expected non-empty RequestBody")
	}
	if rec.ResponseBody == "" {
		t.Error("Expected non-empty ResponseBody")
	}
	if !strings.Contains(rec.ResponseBody, "Alice") {
		t.Errorf("Expected response body to contain 'Alice', got %q", rec.ResponseBody)
	}
}

func TestHandler_RecordingDisabledNoCapture(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-disabled",
		Path: "/soap/service",
		Operations: map[string]OperationConfig{
			"GetUser": {
				Response: "<GetUserResponse><name>Bob</name></GetUserResponse>",
			},
		},
		Enabled: true,
	}
	handler := mustNewHandler(t, config)

	store := &mockSOAPStore{}
	handler.SetRecordingStore(store)
	// Recording NOT enabled

	soapBody := `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser><id>1</id></GetUser>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest("POST", "/soap/service", strings.NewReader(soapBody))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "GetUser")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}

	time.Sleep(50 * time.Millisecond)
	recordings := store.getRecordings()
	if len(recordings) != 0 {
		t.Errorf("Expected 0 recordings when disabled, got %d", len(recordings))
	}
}

func TestHandler_RecordingNoStoreNoCapture(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-nostore",
		Path: "/soap/service",
		Operations: map[string]OperationConfig{
			"GetUser": {
				Response: "<GetUserResponse><name>Carol</name></GetUserResponse>",
			},
		},
		Enabled: true,
	}
	handler := mustNewHandler(t, config)

	// Recording enabled but NO store set
	handler.EnableRecording()

	soapBody := `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser><id>1</id></GetUser>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest("POST", "/soap/service", strings.NewReader(soapBody))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "GetUser")
	w := httptest.NewRecorder()

	// Should not panic even without a store
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestHandler_RecordingCapturesFault(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-fault-rec",
		Path: "/soap/service",
		Operations: map[string]OperationConfig{
			"GetUser": {
				Response: "<GetUserResponse/>",
				Fault: &SOAPFault{
					Code:    "Server",
					Message: "Internal failure",
				},
			},
		},
		Enabled: true,
	}
	handler := mustNewHandler(t, config)

	store := &mockSOAPStore{}
	handler.SetRecordingStore(store)
	handler.EnableRecording()

	soapBody := `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser><id>1</id></GetUser>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest("POST", "/soap/service", strings.NewReader(soapBody))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "GetUser")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	time.Sleep(50 * time.Millisecond)
	recordings := store.getRecordings()
	if len(recordings) != 1 {
		t.Fatalf("Expected 1 recording, got %d", len(recordings))
	}

	rec := recordings[0]
	if !rec.HasFault {
		t.Error("Expected HasFault=true for fault response")
	}
	if rec.FaultCode == "" {
		t.Error("Expected non-empty FaultCode")
	}
	if rec.ResponseStatus != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for fault, got %d", rec.ResponseStatus)
	}
}

func TestHandler_RequestLoggerCapturesRequest(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-logger-cap",
		Path: "/soap/service",
		Operations: map[string]OperationConfig{
			"GetUser": {
				Response: "<GetUserResponse><name>Dave</name></GetUserResponse>",
			},
		},
		Enabled: true,
	}
	handler := mustNewHandler(t, config)

	logger := &mockRequestLogger{}
	handler.SetRequestLogger(logger)

	soapBody := `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser><id>1</id></GetUser>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest("POST", "/soap/service", strings.NewReader(soapBody))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "GetUser")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Request logging is synchronous — should be available immediately
	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	entry := entries[0]
	if entry.Protocol != "soap" {
		t.Errorf("Expected protocol 'soap', got %q", entry.Protocol)
	}
	if entry.Method != "POST" {
		t.Errorf("Expected method 'POST', got %q", entry.Method)
	}
	if entry.SOAP == nil {
		t.Fatal("Expected SOAP metadata to be populated")
	}
	if entry.SOAP.Operation != "GetUser" {
		t.Errorf("Expected SOAP operation 'GetUser', got %q", entry.SOAP.Operation)
	}
	if entry.SOAP.SOAPVersion != "1.1" {
		t.Errorf("Expected SOAP version '1.1', got %q", entry.SOAP.SOAPVersion)
	}
	if entry.SOAP.IsFault {
		t.Error("Expected IsFault=false for successful response")
	}
	if entry.ResponseStatus != http.StatusOK {
		t.Errorf("Expected response status 200, got %d", entry.ResponseStatus)
	}
}

func TestHandler_RequestLoggerNilNoCapture(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-logger-nil",
		Path: "/soap/service",
		Operations: map[string]OperationConfig{
			"GetUser": {
				Response: "<GetUserResponse><name>Eve</name></GetUserResponse>",
			},
		},
		Enabled: true,
	}
	handler := mustNewHandler(t, config)
	// No logger set — should not panic

	soapBody := `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser><id>1</id></GetUser>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest("POST", "/soap/service", strings.NewReader(soapBody))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "GetUser")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
}

func TestHandler_RecordingAndLoggingTogether(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-both",
		Path: "/soap/service",
		Operations: map[string]OperationConfig{
			"GetUser": {
				Response: "<GetUserResponse><name>Frank</name></GetUserResponse>",
			},
		},
		Enabled: true,
	}
	handler := mustNewHandler(t, config)

	store := &mockSOAPStore{}
	logger := &mockRequestLogger{}
	handler.SetRecordingStore(store)
	handler.SetRequestLogger(logger)
	handler.EnableRecording()

	soapBody := `<?xml version="1.0"?>
<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">
  <soap:Body>
    <GetUser><id>42</id></GetUser>
  </soap:Body>
</soap:Envelope>`

	req := httptest.NewRequest("POST", "/soap/service", strings.NewReader(soapBody))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "GetUser")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d: %s", w.Code, w.Body.String())
	}

	time.Sleep(50 * time.Millisecond)

	// Both recording and logging should have captured
	recordings := store.getRecordings()
	if len(recordings) != 1 {
		t.Fatalf("Expected 1 recording, got %d", len(recordings))
	}
	entries := logger.getEntries()
	if len(entries) != 1 {
		t.Fatalf("Expected 1 log entry, got %d", len(entries))
	}

	// Both should agree on the operation
	if recordings[0].Operation != entries[0].SOAP.Operation {
		t.Errorf("Recording operation %q != log entry operation %q",
			recordings[0].Operation, entries[0].SOAP.Operation)
	}
}

func TestHandler_RecordingThreadSafety(t *testing.T) {
	config := &SOAPConfig{
		ID:   "test-threadsafe",
		Path: "/soap/service",
		Operations: map[string]OperationConfig{
			"Ping": {Response: "<PingResponse>pong</PingResponse>"},
		},
		Enabled: true,
	}
	handler := mustNewHandler(t, config)

	store := &mockSOAPStore{}
	handler.SetRecordingStore(store)

	// Toggle recording on/off from multiple goroutines while serving requests
	var wg sync.WaitGroup
	const numGoroutines = 20

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if idx%2 == 0 {
				handler.EnableRecording()
			} else {
				handler.DisableRecording()
			}
			_ = handler.IsRecordingEnabled()
		}(i)
	}

	wg.Wait()
	// No data races or panics = pass
}
