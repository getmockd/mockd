package cli

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/getmockd/mockd/pkg/cli/internal/parse"
)

// RunSOAP handles the soap command and its subcommands.
func RunSOAP(args []string) error {
	if len(args) == 0 {
		printSOAPUsage()
		return nil
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "validate":
		return runSOAPValidate(subArgs)
	case "call":
		return runSOAPCall(subArgs)
	case "help", "--help", "-h":
		printSOAPUsage()
		return nil
	default:
		return fmt.Errorf("unknown soap subcommand: %s\n\nRun 'mockd soap --help' for usage", subcommand)
	}
}

func printSOAPUsage() {
	fmt.Print(`Usage: mockd soap <subcommand> [flags]

Manage and test SOAP web services.

Subcommands:
  validate  Validate a WSDL file
  call      Call a SOAP operation

Run 'mockd soap <subcommand> --help' for more information.
`)
}

// runSOAPValidate validates a WSDL file.
func runSOAPValidate(args []string) error {
	fs := flag.NewFlagSet("soap validate", flag.ContinueOnError)

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd soap validate <wsdl-file>

Validate a WSDL file.

Arguments:
  wsdl-file    Path to the WSDL file

Examples:
  # Validate a WSDL file
  mockd soap validate service.wsdl

  # Validate with full path
  mockd soap validate ./wsdl/calculator.wsdl
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("wsdl file is required")
	}

	wsdlFile := fs.Arg(0)

	// Read WSDL file
	wsdlBytes, err := os.ReadFile(wsdlFile)
	if err != nil {
		return fmt.Errorf("failed to read WSDL file: %w", err)
	}

	// Parse as XML
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(wsdlBytes); err != nil {
		return fmt.Errorf("wsdl validation failed: invalid XML: %w", err)
	}

	// Validate basic WSDL structure
	root := doc.Root()
	if root == nil {
		return fmt.Errorf("wsdl validation failed: empty document")
	}

	// Check for definitions element (WSDL 1.1) or description element (WSDL 2.0)
	if root.Tag != "definitions" && root.Tag != "description" {
		return fmt.Errorf("wsdl validation failed: root element must be 'definitions' (WSDL 1.1) or 'description' (WSDL 2.0), got '%s'", root.Tag)
	}

	// Validate namespace
	wsdlNS := false
	for _, attr := range root.Attr {
		if strings.HasPrefix(attr.Key, "xmlns") {
			if strings.Contains(attr.Value, "wsdl") || strings.Contains(attr.Value, "schemas.xmlsoap.org") {
				wsdlNS = true
				break
			}
		}
	}
	if !wsdlNS {
		return fmt.Errorf("wsdl validation failed: missing WSDL namespace declaration")
	}

	// Count elements
	services := doc.FindElements("//service")
	if len(services) == 0 {
		services = doc.FindElements("//*[local-name()='service']")
	}
	portTypes := doc.FindElements("//portType")
	if len(portTypes) == 0 {
		portTypes = doc.FindElements("//*[local-name()='portType']")
	}
	bindings := doc.FindElements("//binding")
	if len(bindings) == 0 {
		bindings = doc.FindElements("//*[local-name()='binding']")
	}
	operations := doc.FindElements("//operation")
	if len(operations) == 0 {
		operations = doc.FindElements("//*[local-name()='operation']")
	}
	messages := doc.FindElements("//message")
	if len(messages) == 0 {
		messages = doc.FindElements("//*[local-name()='message']")
	}

	// Print validation result
	fmt.Printf("WSDL valid: %s\n", wsdlFile)
	fmt.Printf("  Services: %d\n", len(services))
	fmt.Printf("  Port Types: %d\n", len(portTypes))
	fmt.Printf("  Bindings: %d\n", len(bindings))
	fmt.Printf("  Operations: %d\n", len(operations))
	fmt.Printf("  Messages: %d\n", len(messages))

	// List services and their operations
	if len(services) > 0 {
		fmt.Println("\nServices:")
		for _, svc := range services {
			svcName := svc.SelectAttrValue("name", "unnamed")
			fmt.Printf("  %s\n", svcName)

			// Find ports
			ports := svc.FindElements("port")
			if len(ports) == 0 {
				ports = svc.FindElements("*[local-name()='port']")
			}
			for _, port := range ports {
				portName := port.SelectAttrValue("name", "unnamed")
				binding := port.SelectAttrValue("binding", "")
				fmt.Printf("    Port: %s (binding: %s)\n", portName, binding)
			}
		}
	}

	// List operations from port types
	if len(portTypes) > 0 {
		fmt.Println("\nOperations:")
		for _, pt := range portTypes {
			ptName := pt.SelectAttrValue("name", "unnamed")
			ops := pt.FindElements("operation")
			if len(ops) == 0 {
				ops = pt.FindElements("*[local-name()='operation']")
			}
			for _, op := range ops {
				opName := op.SelectAttrValue("name", "unnamed")
				fmt.Printf("  %s.%s\n", ptName, opName)
			}
		}
	}

	return nil
}

// runSOAPCall executes a SOAP call against an endpoint.
func runSOAPCall(args []string) error {
	fs := flag.NewFlagSet("soap call", flag.ContinueOnError)

	soapAction := fs.String("action", "", "SOAPAction header value")
	fs.StringVar(soapAction, "a", "", "SOAPAction header (shorthand)")

	body := fs.String("body", "", "SOAP body content (XML) or @filename")
	fs.StringVar(body, "b", "", "SOAP body content (shorthand)")

	headers := fs.String("header", "", "Additional headers (key:value,key2:value2)")
	fs.StringVar(headers, "H", "", "Additional headers (shorthand)")

	soap12 := fs.Bool("soap12", false, "Use SOAP 1.2 (default: SOAP 1.1)")
	pretty := fs.Bool("pretty", true, "Pretty print output")
	timeout := fs.Int("timeout", 30, "Request timeout in seconds")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd soap call <endpoint> <operation> [flags]

Call a SOAP operation on an endpoint.

Arguments:
  endpoint     SOAP service endpoint URL
  operation    Operation name (used in SOAP body if no body provided)

Flags:
  -a, --action   SOAPAction header value
  -b, --body     SOAP body content (XML) or @filename
  -H, --header   Additional headers (key:value,key2:value2)
      --soap12   Use SOAP 1.2 (default: SOAP 1.1)
      --pretty   Pretty print output (default: true)
      --timeout  Request timeout in seconds (default: 30)

Examples:
  # Call an operation with auto-generated body
  mockd soap call http://localhost:4280/soap GetUser

  # Call with SOAPAction header
  mockd soap call http://localhost:4280/soap GetUser \
    -a "http://example.com/GetUser"

  # Call with custom body
  mockd soap call http://localhost:4280/soap GetUser \
    -b '<GetUser xmlns="http://example.com/"><id>123</id></GetUser>'

  # Call with body from file
  mockd soap call http://localhost:4280/soap GetUser -b @request.xml

  # Use SOAP 1.2
  mockd soap call http://localhost:4280/soap GetUser --soap12

Note: This command uses curl under the hood for HTTP requests.
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("endpoint and operation are required")
	}

	endpoint := fs.Arg(0)
	operation := fs.Arg(1)

	// Build SOAP body
	var soapBody string
	if *body != "" {
		// Load body from file if prefixed with @
		if len(*body) > 0 && (*body)[0] == '@' {
			bodyBytes, err := os.ReadFile((*body)[1:])
			if err != nil {
				return fmt.Errorf("failed to read body file: %w", err)
			}
			soapBody = string(bodyBytes)
		} else {
			soapBody = *body
		}
	} else {
		// Generate minimal body for operation
		soapBody = fmt.Sprintf("<%s xmlns=\"http://tempuri.org/\"/>", operation)
	}

	// Build SOAP envelope
	var envelope string
	var contentType string
	if *soap12 {
		envelope = buildSOAP12Envelope(soapBody)
		contentType = "application/soap+xml; charset=utf-8"
		if *soapAction != "" {
			contentType = fmt.Sprintf("application/soap+xml; charset=utf-8; action=\"%s\"", *soapAction)
		}
	} else {
		envelope = buildSOAP11Envelope(soapBody)
		contentType = "text/xml; charset=utf-8"
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(envelope))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	if !*soap12 && *soapAction != "" {
		req.Header.Set("SOAPAction", fmt.Sprintf("\"%s\"", *soapAction))
	}

	// Add custom headers
	if *headers != "" {
		for _, header := range parse.SplitHeaders(*headers) {
			parts := parse.HeaderParts(header)
			if len(parts) == 2 {
				req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}
	}

	// Execute request
	client := &http.Client{
		Timeout: secondsToDuration(*timeout),
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Print response
	if *pretty {
		prettyXML := prettyPrintXML(respBody)
		fmt.Println(prettyXML)
	} else {
		fmt.Println(string(respBody))
	}

	// Check for SOAP fault
	if bytes.Contains(respBody, []byte("Fault")) {
		return fmt.Errorf("SOAP fault received (HTTP %d)", resp.StatusCode)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return nil
}

// buildSOAP11Envelope builds a SOAP 1.1 envelope.
func buildSOAP11Envelope(body string) string {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString(`<soap:Envelope xmlns:soap="http://schemas.xmlsoap.org/soap/envelope/">`)
	buf.WriteString(`<soap:Body>`)
	buf.WriteString(body)
	buf.WriteString(`</soap:Body>`)
	buf.WriteString(`</soap:Envelope>`)
	return buf.String()
}

// buildSOAP12Envelope builds a SOAP 1.2 envelope.
func buildSOAP12Envelope(body string) string {
	var buf bytes.Buffer
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	buf.WriteString(`<soap:Envelope xmlns:soap="http://www.w3.org/2003/05/soap-envelope">`)
	buf.WriteString(`<soap:Body>`)
	buf.WriteString(body)
	buf.WriteString(`</soap:Body>`)
	buf.WriteString(`</soap:Envelope>`)
	return buf.String()
}

// prettyPrintXML formats XML with indentation.
func prettyPrintXML(xmlBytes []byte) string {
	doc := etree.NewDocument()
	if err := doc.ReadFromBytes(xmlBytes); err != nil {
		// If parsing fails, return as-is
		return string(xmlBytes)
	}
	doc.Indent(2)
	result, err := doc.WriteToString()
	if err != nil {
		return string(xmlBytes)
	}
	return result
}

// secondsToDuration converts seconds to time.Duration.
func secondsToDuration(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}
