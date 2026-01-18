package soap

import (
	"encoding/xml"
)

// SOAPVersion represents the SOAP protocol version.
type SOAPVersion string

const (
	// SOAP11 represents SOAP 1.1 protocol.
	SOAP11 SOAPVersion = "1.1"
	// SOAP12 represents SOAP 1.2 protocol.
	SOAP12 SOAPVersion = "1.2"
)

// SOAP namespace URIs
const (
	SOAP11Namespace = "http://schemas.xmlsoap.org/soap/envelope/"
	SOAP12Namespace = "http://www.w3.org/2003/05/soap-envelope"
)

// SOAPConfig configures a SOAP endpoint.
type SOAPConfig struct {
	ID          string                     `json:"id" yaml:"id"`
	Name        string                     `json:"name,omitempty" yaml:"name,omitempty"`
	ParentID    string                     `json:"parentId,omitempty" yaml:"parentId,omitempty"`
	MetaSortKey float64                    `json:"metaSortKey,omitempty" yaml:"metaSortKey,omitempty"`
	Path        string                     `json:"path" yaml:"path"`
	WSDLFile    string                     `json:"wsdlFile,omitempty" yaml:"wsdlFile,omitempty"`
	WSDL        string                     `json:"wsdl,omitempty" yaml:"wsdl,omitempty"` // Inline WSDL
	Operations  map[string]OperationConfig `json:"operations,omitempty" yaml:"operations,omitempty"`
	Enabled     bool                       `json:"enabled" yaml:"enabled"`
}

// OperationConfig configures a single SOAP operation.
type OperationConfig struct {
	SOAPAction string     `json:"soapAction,omitempty" yaml:"soapAction,omitempty"`
	Response   string     `json:"response" yaml:"response"` // XML template
	Delay      string     `json:"delay,omitempty" yaml:"delay,omitempty"`
	Fault      *SOAPFault `json:"fault,omitempty" yaml:"fault,omitempty"`
	Match      *SOAPMatch `json:"match,omitempty" yaml:"match,omitempty"`
}

// SOAPMatch defines XPath-based request matching conditions.
type SOAPMatch struct {
	XPath map[string]string `json:"xpath,omitempty" yaml:"xpath,omitempty"` // XPath -> expected value
}

// SOAPFault defines a SOAP fault response.
type SOAPFault struct {
	Code    string `json:"code" yaml:"code"`       // soap:Client, soap:Server
	Message string `json:"message" yaml:"message"` // Human readable error
	Detail  string `json:"detail,omitempty" yaml:"detail,omitempty"`
}

// SOAPEnvelope represents a SOAP message envelope.
type SOAPEnvelope struct {
	XMLName xml.Name    `xml:"Envelope"`
	Header  *SOAPHeader `xml:"Header,omitempty"`
	Body    *SOAPBody   `xml:"Body"`
}

// SOAPHeader represents the SOAP header element.
type SOAPHeader struct {
	Content string `xml:",innerxml"`
}

// SOAPBody represents the SOAP body element.
type SOAPBody struct {
	Content string            `xml:",innerxml"`
	Fault   *SOAPFaultElement `xml:"Fault,omitempty"`
}

// SOAPFaultElement represents a SOAP Fault element in the body.
type SOAPFaultElement struct {
	XMLName  xml.Name `xml:"Fault"`
	Code     string   `xml:"faultcode,omitempty"`   // SOAP 1.1
	String   string   `xml:"faultstring,omitempty"` // SOAP 1.1
	Detail   string   `xml:"detail,omitempty"`      // SOAP 1.1
	Code12   *Code12  `xml:"Code,omitempty"`        // SOAP 1.2
	Reason12 *Reason  `xml:"Reason,omitempty"`      // SOAP 1.2
	Detail12 *Detail  `xml:"Detail,omitempty"`      // SOAP 1.2
}

// Code12 represents SOAP 1.2 fault code.
type Code12 struct {
	Value   string  `xml:"Value"`
	Subcode *Code12 `xml:"Subcode,omitempty"`
}

// Reason represents SOAP 1.2 fault reason.
type Reason struct {
	Text string `xml:"Text"`
}

// Detail represents SOAP 1.2 fault detail.
type Detail struct {
	Content string `xml:",innerxml"`
}

// ContentTypes for SOAP versions
const (
	SOAP11ContentType = "text/xml; charset=utf-8"
	SOAP12ContentType = "application/soap+xml; charset=utf-8"
)
