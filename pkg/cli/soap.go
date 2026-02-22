package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/beevik/etree"
	"github.com/charmbracelet/huh"
	"github.com/getmockd/mockd/pkg/cli/internal/parse"
	"github.com/spf13/cobra"
)

var soapCmd = &cobra.Command{
	Use:   "soap",
	Short: "Manage and test SOAP endpoints",
}

var soapAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new SOAP mock endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use huh interactive forms if attributes are missing
		if !cmd.Flags().Changed("path") {
			var formPath, formAction, formResponse string

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("What is the SOAP endpoint path?").
						Placeholder("/soap/calculator").
						Value(&formPath).
						Validate(func(s string) error {
							if s == "" {
								return errors.New("path is required")
							}
							return nil
						}),
					huh.NewInput().
						Title("SOAP Action").
						Placeholder("http://example.com/Add").
						Value(&formAction).
						Validate(func(s string) error {
							if s == "" {
								return errors.New("action is required")
							}
							return nil
						}),
					huh.NewText().
						Title("Response XML").
						Placeholder("<AddResponse><Result>3</Result></AddResponse>").
						Value(&formResponse),
				),
			)
			if err := form.Run(); err != nil {
				return err
			}
			addPath = formPath
			addOperation = formAction
			addResponse = formResponse
		}
		addMockType = "soap"
		return runAdd(cmd, args)
	},
}

var (
	soapHeaders string
	soapPretty  bool
)

func init() {
	rootCmd.AddCommand(soapCmd)
	soapCmd.AddCommand(soapAddCmd)

	soapAddCmd.Flags().StringVar(&addPath, "path", "", "URL path to match")
	soapAddCmd.Flags().StringVar(&addOperation, "action", "", "SOAP action")
	soapAddCmd.Flags().StringVar(&addResponse, "response", "", "XML response body")

	// Add list/get/delete generic aliases
	soapCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List SOAP mocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			listMockType = "soap"
			return runList(cmd, args)
		},
	})
	soapCmd.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Get details of a SOAP mock",
		RunE:  runGet,
	})
	soapCmd.AddCommand(&cobra.Command{
		Use:   "delete",
		Short: "Delete a SOAP mock",
		RunE:  runDelete,
	})

	soapCmd.AddCommand(soapValidateCmd)

	soapCallCmd.Flags().StringVarP(&soapHeaders, "header", "H", "", "Additional headers (key:value,key2:value2)")
	soapCallCmd.Flags().BoolVar(&soapPretty, "pretty", true, "Pretty print output")
	soapCmd.AddCommand(soapCallCmd)
}

var soapValidateCmd = &cobra.Command{
	Use:   "validate <wsdl-file>",
	Short: "Validate a WSDL file",
	RunE:  runSOAPValidate,
}

var soapCallCmd = &cobra.Command{
	Use:   "call <endpoint> <action> <body>",
	Short: "Execute a SOAP call against an endpoint",
	RunE:  runSOAPCall,
}

// runSOAPValidate validates a WSDL file.
func runSOAPValidate(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return errors.New("wsdl file is required")
	}

	wsdlFile := args[0]

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
		return errors.New("wsdl validation failed: empty document")
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
		return errors.New("wsdl validation failed: missing WSDL namespace declaration")
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

	// Build structured result
	serviceNames := make([]string, 0, len(services))
	for _, svc := range services {
		serviceNames = append(serviceNames, svc.SelectAttrValue("name", "unnamed"))
	}
	opNames := make([]string, 0, len(operations))
	for _, pt := range portTypes {
		ptName := pt.SelectAttrValue("name", "unnamed")
		ops := pt.FindElements("operation")
		if len(ops) == 0 {
			ops = pt.FindElements("*[local-name()='operation']")
		}
		for _, op := range ops {
			opNames = append(opNames, ptName+"."+op.SelectAttrValue("name", "unnamed"))
		}
	}

	printResult(map[string]any{
		"valid":      true,
		"file":       wsdlFile,
		"services":   len(services),
		"portTypes":  len(portTypes),
		"bindings":   len(bindings),
		"operations": len(operations),
		"messages":   len(messages),
	}, func() {
		fmt.Printf("WSDL valid: %s\n", wsdlFile)
		fmt.Printf("  Services: %d\n", len(services))
		fmt.Printf("  Port Types: %d\n", len(portTypes))
		fmt.Printf("  Bindings: %d\n", len(bindings))
		fmt.Printf("  Operations: %d\n", len(operations))
		fmt.Printf("  Messages: %d\n", len(messages))

		if len(serviceNames) > 0 {
			fmt.Println("\nServices:")
			for _, svc := range services {
				svcName := svc.SelectAttrValue("name", "unnamed")
				fmt.Printf("  %s\n", svcName)

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

		if len(opNames) > 0 {
			fmt.Println("\nOperations:")
			for _, name := range opNames {
				fmt.Printf("  %s\n", name)
			}
		}
	})
	return nil
}

// runSOAPCall executes a SOAP call against an endpoint.
func runSOAPCall(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return errors.New("endpoint and operation are required")
	}

	endpoint := args[0]
	operation := args[1]
	body := ""
	if len(args) >= 3 {
		body = args[2]
	}

	soapAction := ""
	if cmd.Flags().Changed("action") {
		soapAction, _ = cmd.Flags().GetString("action")
	}
	soap12 := false
	if cmd.Flags().Changed("soap12") {
		soap12, _ = cmd.Flags().GetBool("soap12")
	}
	timeout := 30
	if cmd.Flags().Changed("timeout") {
		timeout, _ = cmd.Flags().GetInt("timeout")
	}

	// Build SOAP body
	var soapBody string
	if body != "" {
		// Load body from file if prefixed with @
		if len(body) > 0 && body[0] == '@' {
			bodyBytes, err := os.ReadFile(body[1:])
			if err != nil {
				return fmt.Errorf("failed to read body file: %w", err)
			}
			soapBody = string(bodyBytes)
		} else {
			soapBody = body
		}
	} else {
		// Generate minimal body for operation
		soapBody = fmt.Sprintf("<%s xmlns=\"http://tempuri.org/\"/>", operation)
	}

	// Build SOAP envelope
	var envelope string
	var contentType string
	if soap12 {
		envelope = buildSOAP12Envelope(soapBody)
		contentType = "application/soap+xml; charset=utf-8"
		if soapAction != "" {
			contentType = fmt.Sprintf("application/soap+xml; charset=utf-8; action=\"%s\"", soapAction)
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
	if !soap12 && soapAction != "" {
		req.Header.Set("SOAPAction", fmt.Sprintf("\"%s\"", soapAction))
	}

	// Add custom headers
	if soapHeaders != "" {
		for _, header := range parse.SplitHeaders(soapHeaders) {
			parts := parse.HeaderParts(header)
			if len(parts) == 2 {
				req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
			}
		}
	}

	// Execute request
	client := &http.Client{
		Timeout: secondsToDuration(timeout),
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Print response
	if soapPretty {
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
