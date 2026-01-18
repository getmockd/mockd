// Package mtls provides utilities for extracting and managing client identity
// information from mTLS (mutual TLS) connections.
package mtls

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"net"
	"net/url"
	"time"
)

// ClientIdentity represents the identity extracted from a client certificate.
// It contains all relevant subject information, issuer details, validity period,
// Subject Alternative Names (SANs), and a fingerprint for identification.
type ClientIdentity struct {
	CommonName         string          `json:"commonName"`
	Organization       []string        `json:"organization,omitempty"`
	OrganizationalUnit []string        `json:"organizationalUnit,omitempty"`
	Country            []string        `json:"country,omitempty"`
	SerialNumber       string          `json:"serialNumber"`
	Issuer             IssuerInfo      `json:"issuer"`
	NotBefore          string          `json:"notBefore"`
	NotAfter           string          `json:"notAfter"`
	SANs               SubjectAltNames `json:"sans,omitempty"`
	Fingerprint        string          `json:"fingerprint"` // SHA256 fingerprint
	Verified           bool            `json:"verified"`
}

// IssuerInfo contains information about the certificate issuer (CA).
type IssuerInfo struct {
	CommonName   string   `json:"commonName"`
	Organization []string `json:"organization,omitempty"`
}

// SubjectAltNames contains all Subject Alternative Names from the certificate.
type SubjectAltNames struct {
	DNSNames       []string `json:"dnsNames,omitempty"`
	EmailAddresses []string `json:"emailAddresses,omitempty"`
	IPAddresses    []string `json:"ipAddresses,omitempty"`
	URIs           []string `json:"uris,omitempty"`
}

// ExtractIdentity extracts identity information from an x509 certificate.
// The verified parameter indicates whether the certificate was successfully
// verified against the CA chain.
func ExtractIdentity(cert *x509.Certificate, verified bool) *ClientIdentity {
	if cert == nil {
		return nil
	}

	identity := &ClientIdentity{
		CommonName:         cert.Subject.CommonName,
		Organization:       copyStrings(cert.Subject.Organization),
		OrganizationalUnit: copyStrings(cert.Subject.OrganizationalUnit),
		Country:            copyStrings(cert.Subject.Country),
		SerialNumber:       cert.SerialNumber.String(),
		Issuer: IssuerInfo{
			CommonName:   cert.Issuer.CommonName,
			Organization: copyStrings(cert.Issuer.Organization),
		},
		NotBefore:   cert.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:    cert.NotAfter.UTC().Format(time.RFC3339),
		SANs:        extractSANs(cert),
		Fingerprint: Fingerprint(cert),
		Verified:    verified,
	}

	return identity
}

// extractSANs extracts all Subject Alternative Names from the certificate.
func extractSANs(cert *x509.Certificate) SubjectAltNames {
	sans := SubjectAltNames{
		DNSNames:       copyStrings(cert.DNSNames),
		EmailAddresses: copyStrings(cert.EmailAddresses),
		IPAddresses:    ipAddressesToStrings(cert.IPAddresses),
		URIs:           urisToStrings(cert.URIs),
	}
	return sans
}

// Fingerprint calculates the SHA256 fingerprint of a certificate.
// The fingerprint is returned as a lowercase hexadecimal string.
func Fingerprint(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	sum := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(sum[:])
}

// copyStrings creates a copy of a string slice to avoid sharing underlying arrays.
// Returns nil if the input is nil or empty.
func copyStrings(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

// ipAddressesToStrings converts a slice of net.IP to a slice of strings.
func ipAddressesToStrings(ips []net.IP) []string {
	if len(ips) == 0 {
		return nil
	}
	result := make([]string, len(ips))
	for i, ip := range ips {
		result[i] = ip.String()
	}
	return result
}

// urisToStrings converts a slice of *url.URL to a slice of strings.
func urisToStrings(uris []*url.URL) []string {
	if len(uris) == 0 {
		return nil
	}
	result := make([]string, len(uris))
	for i, uri := range uris {
		result[i] = uri.String()
	}
	return result
}
