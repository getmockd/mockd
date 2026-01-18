package mock

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMTLSMatch_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mtls    *MTLSMatch
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty match is valid",
			mtls:    &MTLSMatch{},
			wantErr: false,
		},
		{
			name:    "valid CN",
			mtls:    &MTLSMatch{CN: "test-service"},
			wantErr: false,
		},
		{
			name:    "valid CNPattern",
			mtls:    &MTLSMatch{CNPattern: "test-.*"},
			wantErr: false,
		},
		{
			name:    "CN and CNPattern mutually exclusive",
			mtls:    &MTLSMatch{CN: "test-service", CNPattern: "test-.*"},
			wantErr: true,
			errMsg:  "cannot specify both cn and cnPattern",
		},
		{
			name:    "invalid CNPattern regex",
			mtls:    &MTLSMatch{CNPattern: "[invalid"},
			wantErr: true,
			errMsg:  "invalid regex pattern",
		},
		{
			name:    "valid fingerprint",
			mtls:    &MTLSMatch{Fingerprint: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"},
			wantErr: false,
		},
		{
			name:    "valid fingerprint with sha256 prefix",
			mtls:    &MTLSMatch{Fingerprint: "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"},
			wantErr: false,
		},
		{
			name:    "valid fingerprint with colons",
			mtls:    &MTLSMatch{Fingerprint: "ab:cd:ef:01:23:45:67:89:ab:cd:ef:01:23:45:67:89:ab:cd:ef:01:23:45:67:89:ab:cd:ef:01:23:45:67:89"},
			wantErr: false,
		},
		{
			name:    "fingerprint wrong length",
			mtls:    &MTLSMatch{Fingerprint: "abcdef"},
			wantErr: true,
			errMsg:  "fingerprint must be 64 hex characters",
		},
		{
			name:    "fingerprint invalid hex chars",
			mtls:    &MTLSMatch{Fingerprint: "ghijkl0123456789ghijkl0123456789ghijkl0123456789ghijkl0123456789"},
			wantErr: true,
			errMsg:  "fingerprint must contain only hex characters",
		},
		{
			name:    "valid requireAuth",
			mtls:    &MTLSMatch{RequireAuth: true},
			wantErr: false,
		},
		{
			name:    "valid issuer",
			mtls:    &MTLSMatch{Issuer: "Test CA"},
			wantErr: false,
		},
		{
			name: "valid combined fields",
			mtls: &MTLSMatch{
				RequireAuth: true,
				CNPattern:   "test-.*",
				OU:          "Engineering",
				Fingerprint: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
				Issuer:      "Test CA",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mtls.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeFingerprint(tt.fp)
			assert.Equal(t, tt.want, got)
		})
	}
}
