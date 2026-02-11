package portability

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/getmockd/mockd/pkg/config"
	"github.com/getmockd/mockd/pkg/mock"
)

// Template represents a built-in mock template.
type Template struct {
	// Name is the template identifier
	Name string

	// Description describes what the template creates
	Description string

	// PromptFields are fields the user should provide
	PromptFields []PromptField

	// Generator creates the MockCollection
	Generator func(params map[string]string) (*config.MockCollection, error)
}

// PromptField represents a field to prompt the user for.
type PromptField struct {
	// Name is the parameter name
	Name string

	// Description explains what the field is for
	Description string

	// Default is the default value if not provided
	Default string

	// Required indicates if the field must be provided
	Required bool
}

// templates is the registry of available templates.
var templates = map[string]*Template{}

// RegisterTemplate adds a template to the registry.
func RegisterTemplate(t *Template) {
	if t != nil {
		templates[t.Name] = t
	}
}

// GetTemplate returns a template by name.
func GetTemplate(name string) *Template {
	return templates[name]
}

// ListTemplates returns all available templates.
func ListTemplates() []*Template {
	result := make([]*Template, 0, len(templates))
	for _, t := range templates {
		result = append(result, t)
	}
	return result
}

// Generate creates a MockCollection from a template with the given parameters.
func (t *Template) Generate(params map[string]string) (*config.MockCollection, error) {
	if t.Generator == nil {
		return nil, fmt.Errorf("template %s has no generator", t.Name)
	}

	// Fill in defaults
	filled := make(map[string]string)
	for _, field := range t.PromptFields {
		if val, ok := params[field.Name]; ok && val != "" {
			filled[field.Name] = val
		} else if field.Default != "" {
			filled[field.Name] = field.Default
		} else if field.Required {
			return nil, fmt.Errorf("required field %s not provided", field.Name)
		}
	}

	return t.Generator(filled)
}

// helper to create a mock with standard fields filled in.
func newMock(id, name, method, path string, statusCode int, body string, now time.Time) *config.MockConfiguration {
	enabled := true
	return &config.MockConfiguration{
		ID:        id,
		Name:      name,
		Type:      mock.TypeHTTP,
		Enabled:   &enabled,
		CreatedAt: now,
		UpdatedAt: now,
		HTTP: &mock.HTTPSpec{
			Priority: 0,
			Matcher: &mock.HTTPMatcher{
				Method: method,
				Path:   path,
			},
			Response: &mock.HTTPResponse{
				StatusCode: statusCode,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: body,
			},
		},
	}
}

// init registers all built-in templates.
func init() {
	// Blank template
	RegisterTemplate(&Template{
		Name:        "blank",
		Description: "Empty mock collection",
		PromptFields: []PromptField{
			{Name: "name", Description: "Collection name", Default: "My Mocks", Required: false},
		},
		Generator: func(params map[string]string) (*config.MockCollection, error) {
			return &config.MockCollection{
				Version: "1.0",
				Name:    params["name"],
				Mocks:   []*config.MockConfiguration{},
			}, nil
		},
	})

	// CRUD template
	RegisterTemplate(&Template{
		Name:        "crud",
		Description: "REST CRUD endpoints (list, get, create, update, delete)",
		PromptFields: []PromptField{
			{Name: "name", Description: "Collection name", Default: "CRUD API", Required: false},
			{Name: "resource", Description: "Resource name (e.g., users, products)", Default: "items", Required: true},
		},
		Generator: generateCRUDTemplate,
	})

	// Auth template
	RegisterTemplate(&Template{
		Name:        "auth",
		Description: "Authentication flow (login, logout, refresh, me)",
		PromptFields: []PromptField{
			{Name: "name", Description: "Collection name", Default: "Auth API", Required: false},
			{Name: "basePath", Description: "Base path for auth endpoints", Default: "/auth", Required: false},
		},
		Generator: generateAuthTemplate,
	})

	// Pagination template
	RegisterTemplate(&Template{
		Name:        "pagination",
		Description: "List endpoint with pagination patterns",
		PromptFields: []PromptField{
			{Name: "name", Description: "Collection name", Default: "Paginated API", Required: false},
			{Name: "resource", Description: "Resource name", Default: "items", Required: true},
		},
		Generator: generatePaginationTemplate,
	})

	// Errors template
	RegisterTemplate(&Template{
		Name:        "errors",
		Description: "Common HTTP error responses (400, 401, 403, 404, 500)",
		PromptFields: []PromptField{
			{Name: "name", Description: "Collection name", Default: "Error Responses", Required: false},
		},
		Generator: generateErrorsTemplate,
	})
}

// generateCRUDTemplate creates CRUD endpoints for a resource.
func generateCRUDTemplate(params map[string]string) (*config.MockCollection, error) {
	resource := params["resource"]
	basePath := "/" + strings.ToLower(resource)
	now := time.Now()

	// Sample item for responses
	titleCaser := cases.Title(language.English)
	sampleItem := fmt.Sprintf(`{"id": 1, "name": "Sample %s"}`, titleCaser.String(strings.TrimSuffix(resource, "s")))
	sampleList := fmt.Sprintf(`[%s, {"id": 2, "name": "Another %s"}]`, sampleItem, titleCaser.String(strings.TrimSuffix(resource, "s")))

	collection := &config.MockCollection{
		Version: "1.0",
		Name:    params["name"],
		Mocks: []*config.MockConfiguration{
			newMock("crud-list", "List "+resource, "GET", basePath, 200, sampleList, now),
			newMock("crud-get", "Get "+strings.TrimSuffix(resource, "s"), "GET", basePath+"/:id", 200, sampleItem, now),
			newMock("crud-create", "Create "+strings.TrimSuffix(resource, "s"), "POST", basePath, 201, sampleItem, now),
			newMock("crud-update", "Update "+strings.TrimSuffix(resource, "s"), "PUT", basePath+"/:id", 200, sampleItem, now),
			newMock("crud-delete", "Delete "+strings.TrimSuffix(resource, "s"), "DELETE", basePath+"/:id", 204, "", now),
		},
	}

	// Add Location header for create
	collection.Mocks[2].HTTP.Response.Headers["Location"] = basePath + "/1"

	// No content for delete
	collection.Mocks[4].HTTP.Response.Headers = map[string]string{}

	return collection, nil
}

// generateAuthTemplate creates authentication endpoints.
func generateAuthTemplate(params map[string]string) (*config.MockCollection, error) {
	basePath := params["basePath"]
	if basePath == "" {
		basePath = "/auth"
	}
	now := time.Now()

	collection := &config.MockCollection{
		Version: "1.0",
		Name:    params["name"],
		Mocks: []*config.MockConfiguration{
			newMock("auth-login", "Login", "POST", basePath+"/login", 200,
				`{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", "expiresIn": 3600, "tokenType": "Bearer"}`, now),
			newMock("auth-logout", "Logout", "POST", basePath+"/logout", 200,
				`{"message": "Logged out successfully"}`, now),
			newMock("auth-refresh", "Refresh Token", "POST", basePath+"/refresh", 200,
				`{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", "expiresIn": 3600, "tokenType": "Bearer"}`, now),
			newMock("auth-me", "Get Current User", "GET", basePath+"/me", 200,
				`{"id": 1, "email": "user@example.com", "name": "John Doe", "role": "user"}`, now),
		},
	}

	return collection, nil
}

// generatePaginationTemplate creates paginated list endpoints.
func generatePaginationTemplate(params map[string]string) (*config.MockCollection, error) {
	resource := params["resource"]
	basePath := "/" + strings.ToLower(resource)
	now := time.Now()

	// Cursor-based pagination response
	cursorResponse := `{
  "data": [
    {"id": 1, "name": "Item 1"},
    {"id": 2, "name": "Item 2"},
    {"id": 3, "name": "Item 3"}
  ],
  "cursor": {
    "next": "eyJpZCI6M30=",
    "hasMore": true
  }
}`

	// Offset-based pagination response
	offsetResponse := `{
  "data": [
    {"id": 1, "name": "Item 1"},
    {"id": 2, "name": "Item 2"},
    {"id": 3, "name": "Item 3"}
  ],
  "pagination": {
    "total": 100,
    "limit": 10,
    "offset": 0,
    "totalPages": 10,
    "currentPage": 1
  }
}`

	collection := &config.MockCollection{
		Version: "1.0",
		Name:    params["name"],
		Mocks: []*config.MockConfiguration{
			newMock("pagination-cursor", fmt.Sprintf("List %s (cursor)", resource), "GET", basePath, 200, cursorResponse, now),
			newMock("pagination-offset", fmt.Sprintf("List %s (offset)", resource), "GET", basePath+"/paginated", 200, offsetResponse, now),
		},
	}

	return collection, nil
}

// generateErrorsTemplate creates common error response endpoints.
func generateErrorsTemplate(params map[string]string) (*config.MockCollection, error) {
	now := time.Now()

	collection := &config.MockCollection{
		Version: "1.0",
		Name:    params["name"],
		Mocks: []*config.MockConfiguration{
			newMock("error-400", "Bad Request", "GET", "/error/400", 400,
				`{"error": "Bad Request", "message": "The request was malformed or contains invalid parameters", "code": "INVALID_REQUEST"}`, now),
			newMock("error-401", "Unauthorized", "GET", "/error/401", 401,
				`{"error": "Unauthorized", "message": "Authentication is required to access this resource", "code": "AUTH_REQUIRED"}`, now),
			newMock("error-403", "Forbidden", "GET", "/error/403", 403,
				`{"error": "Forbidden", "message": "You do not have permission to access this resource", "code": "ACCESS_DENIED"}`, now),
			newMock("error-404", "Not Found", "GET", "/error/404", 404,
				`{"error": "Not Found", "message": "The requested resource could not be found", "code": "NOT_FOUND"}`, now),
			newMock("error-500", "Internal Server Error", "GET", "/error/500", 500,
				`{"error": "Internal Server Error", "message": "An unexpected error occurred", "code": "INTERNAL_ERROR"}`, now),
		},
	}

	return collection, nil
}
