package config

import (
	"testing"

	"github.com/getmockd/mockd/pkg/mock"
	"github.com/stretchr/testify/assert"
)

func TestRequestMatcher_Validate_PathAndPathPattern(t *testing.T) {
	tests := []struct {
		name    string
		matcher *mock.HTTPMatcher
		wantErr bool
		errMsg  string
	}{
		{
			name: "path only is valid",
			matcher: &mock.HTTPMatcher{
				Path: "/api/users",
			},
			wantErr: false,
		},
		{
			name: "pathPattern only is valid",
			matcher: &mock.HTTPMatcher{
				PathPattern: `^/api/users/\d+$`,
			},
			wantErr: false,
		},
		{
			name: "both path and pathPattern is invalid",
			matcher: &mock.HTTPMatcher{
				Path:        "/api/users",
				PathPattern: `^/api/users/\d+$`,
			},
			wantErr: true,
			errMsg:  "cannot specify both path and pathPattern",
		},
		{
			name: "invalid pathPattern regex",
			matcher: &mock.HTTPMatcher{
				PathPattern: `[invalid`,
			},
			wantErr: true,
			errMsg:  "invalid regex pattern",
		},
		{
			name: "invalid bodyPattern regex",
			matcher: &mock.HTTPMatcher{
				Path:        "/api/users",
				BodyPattern: `(unclosed`,
			},
			wantErr: true,
			errMsg:  "invalid regex pattern",
		},
		{
			name: "valid pathPattern with named groups",
			matcher: &mock.HTTPMatcher{
				PathPattern: `^/api/(?P<resource>\w+)/(?P<id>\d+)$`,
			},
			wantErr: false,
		},
		{
			name: "valid bodyPattern",
			matcher: &mock.HTTPMatcher{
				Path:        "/api/users",
				BodyPattern: `"email":\s*"[^"]+"`,
			},
			wantErr: false,
		},
		{
			name: "bodyPattern as only criteria is valid",
			matcher: &mock.HTTPMatcher{
				BodyPattern: `"status":\s*"active"`,
			},
			wantErr: false,
		},
		{
			name: "pathPattern and bodyPattern combined is valid",
			matcher: &mock.HTTPMatcher{
				PathPattern: `^/api/users/\d+$`,
				BodyPattern: `"email":\s*"[^"]+"`,
			},
			wantErr: false,
		},
		{
			name: "bodyEquals, bodyContains, and bodyPattern can be combined",
			matcher: &mock.HTTPMatcher{
				Path:         "/api/users",
				BodyContains: "email",
				BodyPattern:  `"status":\s*"active"`,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.matcher.Validate()
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

func TestRequestMatcher_Validate_BodyJSONPath(t *testing.T) {
	tests := []struct {
		name    string
		matcher *mock.HTTPMatcher
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid bodyJsonPath only",
			matcher: &mock.HTTPMatcher{
				BodyJSONPath: map[string]interface{}{
					"$.status": "active",
				},
			},
			wantErr: false,
		},
		{
			name: "valid bodyJsonPath with nested path",
			matcher: &mock.HTTPMatcher{
				BodyJSONPath: map[string]interface{}{
					"$.user.name": "John",
				},
			},
			wantErr: false,
		},
		{
			name: "valid bodyJsonPath with array index",
			matcher: &mock.HTTPMatcher{
				BodyJSONPath: map[string]interface{}{
					"$.items[0].id": float64(123),
				},
			},
			wantErr: false,
		},
		{
			name: "valid bodyJsonPath with wildcard",
			matcher: &mock.HTTPMatcher{
				BodyJSONPath: map[string]interface{}{
					"$.items[*].type": "premium",
				},
			},
			wantErr: false,
		},
		{
			name: "valid bodyJsonPath with existence check",
			matcher: &mock.HTTPMatcher{
				BodyJSONPath: map[string]interface{}{
					"$.token": map[string]interface{}{"exists": true},
				},
			},
			wantErr: false,
		},
		{
			name: "valid bodyJsonPath combined with path",
			matcher: &mock.HTTPMatcher{
				Path: "/api/users",
				BodyJSONPath: map[string]interface{}{
					"$.status": "active",
				},
			},
			wantErr: false,
		},
		{
			name: "valid bodyJsonPath combined with bodyContains",
			matcher: &mock.HTTPMatcher{
				Path:         "/api/users",
				BodyContains: "email",
				BodyJSONPath: map[string]interface{}{
					"$.status": "active",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid JSONPath syntax - unclosed bracket",
			matcher: &mock.HTTPMatcher{
				BodyJSONPath: map[string]interface{}{
					"$[invalid": "value",
				},
			},
			wantErr: true,
			errMsg:  "invalid JSONPath expression",
		},
		{
			name: "invalid JSONPath syntax - bad filter",
			matcher: &mock.HTTPMatcher{
				BodyJSONPath: map[string]interface{}{
					"$[?(": "value",
				},
			},
			wantErr: true,
			errMsg:  "invalid JSONPath expression",
		},
		{
			name: "multiple valid JSONPath conditions",
			matcher: &mock.HTTPMatcher{
				BodyJSONPath: map[string]interface{}{
					"$.status":    "active",
					"$.user.name": "John",
					"$.count":     float64(5),
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.matcher.Validate()
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

func TestMockConfiguration_Validate_WithPatterns(t *testing.T) {
	tests := []struct {
		name    string
		mockCfg *MockConfiguration
		wantErr bool
	}{
		{
			name: "valid mock with pathPattern",
			mockCfg: &MockConfiguration{
				ID:      "test-1",
				Enabled: boolPtr(true),
				Type:    mock.TypeHTTP,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						PathPattern: `^/api/users/\d+$`,
					},
					Response: &mock.HTTPResponse{
						StatusCode: 200,
						Body:       "ok",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid mock with bodyPattern",
			mockCfg: &MockConfiguration{
				ID:      "test-2",
				Enabled: boolPtr(true),
				Type:    mock.TypeHTTP,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Path:        "/api/users",
						BodyPattern: `"email":\s*"[^"]+"`,
					},
					Response: &mock.HTTPResponse{
						StatusCode: 200,
						Body:       "ok",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid mock with both path and pathPattern",
			mockCfg: &MockConfiguration{
				ID:      "test-3",
				Enabled: boolPtr(true),
				Type:    mock.TypeHTTP,
				HTTP: &mock.HTTPSpec{
					Matcher: &mock.HTTPMatcher{
						Path:        "/api/users",
						PathPattern: `^/api/users$`,
					},
					Response: &mock.HTTPResponse{
						StatusCode: 200,
						Body:       "ok",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.mockCfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
