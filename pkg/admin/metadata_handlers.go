package admin

import (
	"encoding/json"
	"net/http"

	"github.com/getmockd/mockd/pkg/admin/engineclient"
	"github.com/getmockd/mockd/pkg/portability"
)

// SupportedFormat describes an import/export format.
type SupportedFormat struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Import      bool     `json:"import"`
	Export      bool     `json:"export"`
	Extensions  []string `json:"extensions"`
}

// FormatListResponse represents the response for listing formats.
type FormatListResponse struct {
	Formats []SupportedFormat `json:"formats"`
}

// MockTemplate describes a mock template.
type MockTemplate struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Category    string          `json:"category"`
	Parameters  []TemplateParam `json:"parameters,omitempty"`
}

// TemplateParam describes a template parameter.
type TemplateParam struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Required    bool   `json:"required"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description"`
}

// TemplateListResponse represents the response for listing templates.
type TemplateListResponse struct {
	Templates []MockTemplate `json:"templates"`
}

// GenerateFromTemplateRequest represents a request to generate mocks from a template.
type GenerateFromTemplateRequest struct {
	Parameters map[string]string `json:"parameters"`
}

// formatMetadata maps format IDs to their metadata.
var formatMetadata = map[portability.Format]SupportedFormat{
	portability.FormatMockd: {
		ID:          "mockd",
		Name:        "Mockd Native",
		Description: "Native mockd format",
		Import:      true,
		Export:      true,
		Extensions:  []string{".json", ".yaml"},
	},
	portability.FormatOpenAPI: {
		ID:          "openapi",
		Name:        "OpenAPI",
		Description: "OpenAPI/Swagger specification",
		Import:      true,
		Export:      true,
		Extensions:  []string{".json", ".yaml"},
	},
	portability.FormatPostman: {
		ID:          "postman",
		Name:        "Postman Collection",
		Description: "Postman v2.1 collection",
		Import:      true,
		Export:      false,
		Extensions:  []string{".json"},
	},
	portability.FormatHAR: {
		ID:          "har",
		Name:        "HAR",
		Description: "HTTP Archive format",
		Import:      true,
		Export:      false,
		Extensions:  []string{".har"},
	},
	portability.FormatWireMock: {
		ID:          "wiremock",
		Name:        "WireMock",
		Description: "WireMock stubs",
		Import:      true,
		Export:      false,
		Extensions:  []string{".json"},
	},
	portability.FormatCURL: {
		ID:          "curl",
		Name:        "cURL",
		Description: "cURL command",
		Import:      true,
		Export:      false,
		Extensions:  []string{".txt", ".sh"},
	},
}

// templateCategories maps template names to their categories.
var templateCategories = map[string]string{
	"blank":      "basic",
	"crud":       "rest",
	"auth":       "rest",
	"pagination": "rest",
	"errors":     "rest",
}

// handleListFormats handles GET /formats.
func (a *API) handleListFormats(w http.ResponseWriter, r *http.Request) {
	formats := make([]SupportedFormat, 0, len(portability.AllFormats()))

	for _, f := range portability.AllFormats() {
		if meta, ok := formatMetadata[f]; ok {
			// Ensure import/export flags are current from portability package
			meta.Import = f.CanImport()
			meta.Export = f.CanExport()
			formats = append(formats, meta)
		}
	}

	writeJSON(w, http.StatusOK, FormatListResponse{Formats: formats})
}

// handleListTemplates handles GET /templates.
func (a *API) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	portTemplates := portability.ListTemplates()
	templates := make([]MockTemplate, 0, len(portTemplates))

	for _, t := range portTemplates {
		template := MockTemplate{
			ID:          t.Name,
			Name:        t.Name,
			Description: t.Description,
			Category:    templateCategories[t.Name],
			Parameters:  make([]TemplateParam, 0, len(t.PromptFields)),
		}

		// Default to "other" category if not mapped
		if template.Category == "" {
			template.Category = "other"
		}

		for _, field := range t.PromptFields {
			template.Parameters = append(template.Parameters, TemplateParam{
				Name:        field.Name,
				Type:        "string",
				Required:    field.Required,
				Default:     field.Default,
				Description: field.Description,
			})
		}

		templates = append(templates, template)
	}

	writeJSON(w, http.StatusOK, TemplateListResponse{Templates: templates})
}

// handleGenerateFromTemplate handles POST /templates/{name}.
func (a *API) handleGenerateFromTemplate(w http.ResponseWriter, r *http.Request, engine *engineclient.Client) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "Template name is required")
		return
	}

	template := portability.GetTemplate(name)
	if template == nil {
		writeError(w, http.StatusNotFound, "not_found", "Template not found")
		return
	}

	var req GenerateFromTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// Allow empty body - use defaults
		req.Parameters = make(map[string]string)
	}

	collection, err := template.Generate(req.Parameters)
	if err != nil {
		writeError(w, http.StatusBadRequest, "generation_error", sanitizeError(err, a.logger(), "generate template"))
		return
	}

	// Import the generated mocks into the engine via HTTP client
	if err := engine.ImportConfig(r.Context(), collection, false); err != nil {
		writeError(w, http.StatusInternalServerError, "import_error", sanitizeError(err, a.logger(), "import template mocks"))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":  "Template generated successfully",
		"template": name,
		"mocks":    len(collection.Mocks),
		"config":   collection,
	})
}
