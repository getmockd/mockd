package matching

import (
	"testing"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/getmockd/mockd/pkg/mtls"
	"github.com/stretchr/testify/assert"
)

// createTestIdentity creates a test ClientIdentity with common values.
func createTestIdentity() *mtls.ClientIdentity {
	return &mtls.ClientIdentity{
		CommonName:         "test-service",
		Organization:       []string{"TestOrg"},
		OrganizationalUnit: []string{"Engineering"},
		Issuer: mtls.IssuerInfo{
			CommonName:   "Test CA",
			Organization: []string{"Test CA Org"},
		},
		Fingerprint: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		Verified:    true,
		SANs: mtls.SubjectAltNames{
			DNSNames:       []string{"test.example.com"},
			EmailAddresses: []string{"test@example.com"},
			IPAddresses:    []string{"192.168.1.1"},
		},
	}
}

func TestMatchMTLS_RequireAuth(t *testing.T) {
	tests := []struct {
		name      string
		mtls      *mock.MTLSMatch
		identity  *mtls.ClientIdentity
		wantScore int
	}{
		{
			name:      "requireAuth with verified cert",
			mtls:      &mock.MTLSMatch{RequireAuth: true},
			identity:  &mtls.ClientIdentity{Verified: true},
			wantScore: ScoreMTLSRequireAuth,
		},
		{
			name:      "requireAuth with unverified cert",
			mtls:      &mock.MTLSMatch{RequireAuth: true},
			identity:  &mtls.ClientIdentity{Verified: false},
			wantScore: 0,
		},
		{
			name:      "requireAuth false with unverified cert still matches",
			mtls:      &mock.MTLSMatch{RequireAuth: false},
			identity:  &mtls.ClientIdentity{Verified: false},
			wantScore: 0, // No criteria = score 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := matchMTLS(tt.mtls, tt.identity)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestMatchMTLS_CNPattern(t *testing.T) {
	tests := []struct {
		name      string
		mtls      *mock.MTLSMatch
		identity  *mtls.ClientIdentity
		wantScore int
	}{
		{
			name:      "cnPattern simple match",
			mtls:      &mock.MTLSMatch{CNPattern: "test-.*"},
			identity:  &mtls.ClientIdentity{CommonName: "test-service"},
			wantScore: ScoreMTLSCNPattern,
		},
		{
			name:      "cnPattern no match",
			mtls:      &mock.MTLSMatch{CNPattern: "prod-.*"},
			identity:  &mtls.ClientIdentity{CommonName: "test-service"},
			wantScore: 0,
		},
		{
			name:      "cnPattern anchored match",
			mtls:      &mock.MTLSMatch{CNPattern: "^test-service$"},
			identity:  &mtls.ClientIdentity{CommonName: "test-service"},
			wantScore: ScoreMTLSCNPattern,
		},
		{
			name:      "cnPattern anchored no match",
			mtls:      &mock.MTLSMatch{CNPattern: "^test$"},
			identity:  &mtls.ClientIdentity{CommonName: "test-service"},
			wantScore: 0,
		},
		{
			name:      "cnPattern case insensitive",
			mtls:      &mock.MTLSMatch{CNPattern: "(?i)TEST-SERVICE"},
			identity:  &mtls.ClientIdentity{CommonName: "test-service"},
			wantScore: ScoreMTLSCNPattern,
		},
		{
			name:      "cnPattern invalid regex returns 0",
			mtls:      &mock.MTLSMatch{CNPattern: "[invalid"},
			identity:  &mtls.ClientIdentity{CommonName: "test-service"},
			wantScore: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := matchMTLS(tt.mtls, tt.identity)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestMatchMTLS_Fingerprint(t *testing.T) {
	testFingerprint := "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

	tests := []struct {
		name      string
		mtls      *mock.MTLSMatch
		identity  *mtls.ClientIdentity
		wantScore int
	}{
		{
			name:      "fingerprint raw hex match",
			mtls:      &mock.MTLSMatch{Fingerprint: testFingerprint},
			identity:  &mtls.ClientIdentity{Fingerprint: testFingerprint},
			wantScore: ScoreMTLSFingerprint,
		},
		{
			name:      "fingerprint sha256: prefix match",
			mtls:      &mock.MTLSMatch{Fingerprint: "sha256:" + testFingerprint},
			identity:  &mtls.ClientIdentity{Fingerprint: testFingerprint},
			wantScore: ScoreMTLSFingerprint,
		},
		{
			name:      "fingerprint SHA256: prefix uppercase match",
			mtls:      &mock.MTLSMatch{Fingerprint: "SHA256:" + testFingerprint},
			identity:  &mtls.ClientIdentity{Fingerprint: testFingerprint},
			wantScore: ScoreMTLSFingerprint,
		},
		{
			name:      "fingerprint with colons match",
			mtls:      &mock.MTLSMatch{Fingerprint: "ab:cd:ef:01:23:45:67:89:ab:cd:ef:01:23:45:67:89:ab:cd:ef:01:23:45:67:89:ab:cd:ef:01:23:45:67:89"},
			identity:  &mtls.ClientIdentity{Fingerprint: testFingerprint},
			wantScore: ScoreMTLSFingerprint,
		},
		{
			name:      "fingerprint case insensitive match",
			mtls:      &mock.MTLSMatch{Fingerprint: "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789"},
			identity:  &mtls.ClientIdentity{Fingerprint: testFingerprint},
			wantScore: ScoreMTLSFingerprint,
		},
		{
			name:      "fingerprint no match",
			mtls:      &mock.MTLSMatch{Fingerprint: "0000000000000000000000000000000000000000000000000000000000000000"},
			identity:  &mtls.ClientIdentity{Fingerprint: testFingerprint},
			wantScore: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := matchMTLS(tt.mtls, tt.identity)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestMatchMTLS_Issuer(t *testing.T) {
	tests := []struct {
		name      string
		mtls      *mock.MTLSMatch
		identity  *mtls.ClientIdentity
		wantScore int
	}{
		{
			name: "issuer exact match",
			mtls: &mock.MTLSMatch{Issuer: "Test CA"},
			identity: &mtls.ClientIdentity{
				Issuer: mtls.IssuerInfo{CommonName: "Test CA"},
			},
			wantScore: ScoreMTLSIssuer,
		},
		{
			name: "issuer no match",
			mtls: &mock.MTLSMatch{Issuer: "Production CA"},
			identity: &mtls.ClientIdentity{
				Issuer: mtls.IssuerInfo{CommonName: "Test CA"},
			},
			wantScore: 0,
		},
		{
			name: "issuer case sensitive no match",
			mtls: &mock.MTLSMatch{Issuer: "test ca"},
			identity: &mtls.ClientIdentity{
				Issuer: mtls.IssuerInfo{CommonName: "Test CA"},
			},
			wantScore: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := matchMTLS(tt.mtls, tt.identity)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestMatchMTLS_Combined(t *testing.T) {
	identity := createTestIdentity()

	tests := []struct {
		name      string
		mtls      *mock.MTLSMatch
		wantScore int
	}{
		{
			name: "all criteria match",
			mtls: &mock.MTLSMatch{
				RequireAuth: true,
				CN:          "test-service",
				OU:          "Engineering",
				O:           "TestOrg",
				Fingerprint: identity.Fingerprint,
				Issuer:      "Test CA",
			},
			wantScore: ScoreMTLSRequireAuth + ScoreMTLSCommonName + ScoreMTLSOrgUnit +
				ScoreMTLSOrganization + ScoreMTLSFingerprint + ScoreMTLSIssuer,
		},
		{
			name: "some criteria match, one fails",
			mtls: &mock.MTLSMatch{
				RequireAuth: true,
				CN:          "test-service",
				Issuer:      "Wrong CA", // This should fail
			},
			wantScore: 0,
		},
		{
			name: "cnPattern and other criteria",
			mtls: &mock.MTLSMatch{
				CNPattern: "test-.*",
				OU:        "Engineering",
			},
			wantScore: ScoreMTLSCNPattern + ScoreMTLSOrgUnit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := matchMTLS(tt.mtls, identity)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestMatchMTLS_BackwardCompatibility(t *testing.T) {
	// Test that existing CN/OU/O/SAN configs still work
	identity := createTestIdentity()

	tests := []struct {
		name      string
		mtls      *mock.MTLSMatch
		wantScore int
	}{
		{
			name:      "legacy CN match",
			mtls:      &mock.MTLSMatch{CN: "test-service"},
			wantScore: ScoreMTLSCommonName,
		},
		{
			name:      "legacy OU match",
			mtls:      &mock.MTLSMatch{OU: "Engineering"},
			wantScore: ScoreMTLSOrgUnit,
		},
		{
			name:      "legacy O match",
			mtls:      &mock.MTLSMatch{O: "TestOrg"},
			wantScore: ScoreMTLSOrganization,
		},
		{
			name: "legacy SAN DNS match",
			mtls: &mock.MTLSMatch{
				SAN: &mock.SANMatch{DNS: "test.example.com"},
			},
			wantScore: ScoreSANDNS,
		},
		{
			name: "legacy SAN Email match",
			mtls: &mock.MTLSMatch{
				SAN: &mock.SANMatch{Email: "test@example.com"},
			},
			wantScore: ScoreSANEmail,
		},
		{
			name: "legacy SAN IP match",
			mtls: &mock.MTLSMatch{
				SAN: &mock.SANMatch{IP: "192.168.1.1"},
			},
			wantScore: ScoreSANIP,
		},
		{
			name: "legacy combined CN and OU",
			mtls: &mock.MTLSMatch{
				CN: "test-service",
				OU: "Engineering",
			},
			wantScore: ScoreMTLSCommonName + ScoreMTLSOrgUnit,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := matchMTLS(tt.mtls, identity)
			assert.Equal(t, tt.wantScore, score)
		})
	}
}

func TestNormalizeFingerprint(t *testing.T) {
	tests := []struct {
		name string
		fp   string
		want string
	}{
		{
			name: "raw lowercase",
			fp:   "abcdef0123456789",
			want: "abcdef0123456789",
		},
		{
			name: "raw uppercase",
			fp:   "ABCDEF0123456789",
			want: "abcdef0123456789",
		},
		{
			name: "sha256 prefix",
			fp:   "sha256:abcdef0123456789",
			want: "abcdef0123456789",
		},
		{
			name: "SHA256 prefix uppercase",
			fp:   "SHA256:ABCDEF0123456789",
			want: "abcdef0123456789",
		},
		{
			name: "with colons",
			fp:   "ab:cd:ef:01:23:45:67:89",
			want: "abcdef0123456789",
		},
		{
			name: "sha256 prefix with colons",
			fp:   "sha256:ab:cd:ef:01:23:45:67:89",
			want: "abcdef0123456789",
		},
		{
			name: "mixed case with colons",
			fp:   "AB:CD:EF:01:23:45:67:89",
			want: "abcdef0123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeFingerprint(tt.fp)
			assert.Equal(t, tt.want, got)
		})
	}
}
