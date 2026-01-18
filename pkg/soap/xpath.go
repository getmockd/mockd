package soap

import (
	"strings"

	"github.com/beevik/etree"
)

// MatchXPath checks if a document matches all XPath conditions.
// Each condition maps an XPath expression to an expected value.
// Returns true if all conditions match.
func MatchXPath(doc *etree.Document, conditions map[string]string) bool {
	if doc == nil || len(conditions) == 0 {
		return true
	}

	for xpath, expected := range conditions {
		actual := ExtractXPath(doc, xpath)
		if actual != expected {
			return false
		}
	}

	return true
}

// ExtractXPath extracts the text value at the given XPath from a document.
// Returns an empty string if the path is not found.
//
// Supported XPath syntax:
//   - /path/to/element - absolute path
//   - //element - find anywhere in document
//   - /path/to/element/@attr - attribute value
//   - /path/to/element[1] - indexed access (1-based)
func ExtractXPath(doc *etree.Document, xpath string) string {
	if doc == nil || xpath == "" {
		return ""
	}

	// Use etree's built-in XPath support
	element := doc.FindElement(xpath)
	if element != nil {
		return strings.TrimSpace(element.Text())
	}

	// Try to find attribute
	if strings.Contains(xpath, "/@") {
		// Split xpath to get element path and attribute name
		parts := strings.Split(xpath, "/@")
		if len(parts) == 2 {
			elemPath := parts[0]
			attrName := parts[1]
			elem := doc.FindElement(elemPath)
			if elem != nil {
				attr := elem.SelectAttr(attrName)
				if attr != nil {
					return attr.Value
				}
			}
		}
	}

	return ""
}

// ExtractXPathFromElement extracts value at XPath relative to an element.
func ExtractXPathFromElement(elem *etree.Element, xpath string) string {
	if elem == nil || xpath == "" {
		return ""
	}

	// Handle relative paths
	if strings.HasPrefix(xpath, "./") {
		xpath = xpath[2:]
	}

	// Find child element
	child := elem.FindElement(xpath)
	if child != nil {
		return strings.TrimSpace(child.Text())
	}

	// Try attribute
	if strings.Contains(xpath, "/@") {
		parts := strings.Split(xpath, "/@")
		if len(parts) == 2 {
			elemPath := parts[0]
			attrName := parts[1]
			if elemPath == "" || elemPath == "." {
				attr := elem.SelectAttr(attrName)
				if attr != nil {
					return attr.Value
				}
			} else {
				child := elem.FindElement(elemPath)
				if child != nil {
					attr := child.SelectAttr(attrName)
					if attr != nil {
						return attr.Value
					}
				}
			}
		}
	}

	return ""
}

// FindAllByXPath finds all elements matching an XPath expression.
func FindAllByXPath(doc *etree.Document, xpath string) []*etree.Element {
	if doc == nil || xpath == "" {
		return nil
	}

	return doc.FindElements(xpath)
}

// BuildXPath constructs an XPath string from path segments.
func BuildXPath(segments ...string) string {
	if len(segments) == 0 {
		return ""
	}

	result := strings.Builder{}
	for i, seg := range segments {
		if i == 0 && !strings.HasPrefix(seg, "/") {
			result.WriteString("/")
		}
		if i > 0 && !strings.HasPrefix(seg, "/") && !strings.HasPrefix(seg, "[") {
			result.WriteString("/")
		}
		result.WriteString(seg)
	}

	return result.String()
}

// NormalizeXPath normalizes an XPath expression.
// It handles common variations and returns a canonical form.
func NormalizeXPath(xpath string) string {
	if xpath == "" {
		return ""
	}

	// Trim whitespace
	xpath = strings.TrimSpace(xpath)

	// Ensure leading slash for absolute paths
	if !strings.HasPrefix(xpath, "/") && !strings.HasPrefix(xpath, ".") {
		xpath = "/" + xpath
	}

	// Normalize double slashes (except at start)
	if strings.HasPrefix(xpath, "//") {
		xpath = "//" + strings.ReplaceAll(xpath[2:], "//", "/")
	} else if strings.HasPrefix(xpath, "/") {
		xpath = "/" + strings.ReplaceAll(xpath[1:], "//", "/")
	}

	return xpath
}
