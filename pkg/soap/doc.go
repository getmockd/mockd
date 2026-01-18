// Package soap provides SOAP web service mocking capabilities for the mockd engine.
//
// This package enables mocking of SOAP 1.1 and SOAP 1.2 web services, including
// operation matching, XPath-based request matching, template-based responses,
// and WSDL serving.
//
// # Features
//
//   - SOAP 1.1 and 1.2 envelope parsing
//   - Operation matching by SOAPAction header or body element name
//   - XPath-based request matching for conditional responses
//   - XML template responses with {{xpath:/path}} variable substitution
//   - SOAP Fault response generation
//   - WSDL document serving at ?wsdl
//
// # Basic Usage
//
// Create a SOAP handler with configuration:
//
//	config := &soap.SOAPConfig{
//	    ID:   "calculator-service",
//	    Path: "/calculator",
//	    Operations: map[string]soap.OperationConfig{
//	        "Add": {
//	            SOAPAction: "http://example.com/Add",
//	            Response: `<AddResponse xmlns="http://example.com/">
//	                <result>{{xpath://Add/a}} + {{xpath://Add/b}}</result>
//	            </AddResponse>`,
//	        },
//	    },
//	    Enabled: true,
//	}
//
//	handler := soap.NewHandler(config)
//
// # XPath Matching
//
// Use XPath conditions to match specific request patterns:
//
//	"Divide": {
//	    Match: &soap.SOAPMatch{
//	        XPath: map[string]string{
//	            "//divisor": "0",
//	        },
//	    },
//	    Fault: &soap.SOAPFault{
//	        Code:    "soap:Client",
//	        Message: "Division by zero",
//	    },
//	},
//
// # Template Variables
//
// Response templates support XPath variable substitution:
//
//	Response: `<GetUserResponse>
//	    <userId>{{xpath://GetUser/id}}</userId>
//	    <requestedBy>{{xpath://Header/auth/user}}</requestedBy>
//	</GetUserResponse>`
//
// # WSDL Serving
//
// Provide WSDL documents via file path or inline:
//
//	config := &soap.SOAPConfig{
//	    WSDLFile: "/path/to/service.wsdl",
//	    // or inline:
//	    // WSDL: `<?xml version="1.0"?>...`,
//	}
//
// The WSDL is automatically served when ?wsdl is appended to the endpoint URL.
//
// # SOAP Versions
//
// The handler auto-detects SOAP version from the request envelope namespace:
//   - SOAP 1.1: http://schemas.xmlsoap.org/soap/envelope/
//   - SOAP 1.2: http://www.w3.org/2003/05/soap-envelope
//
// Responses are formatted according to the detected version.
//
// # Error Handling
//
// SOAP faults can be configured for error responses:
//
//	Fault: &soap.SOAPFault{
//	    Code:    "soap:Server",      // or soap:Client
//	    Message: "Service unavailable",
//	    Detail:  "<errorCode>503</errorCode>",
//	}
//
// Fault codes are automatically translated between SOAP versions:
//   - soap:Client -> soap:Sender (SOAP 1.2)
//   - soap:Server -> soap:Receiver (SOAP 1.2)
package soap
