// Package config provides configuration validation for mockd.
// This file contains validation for the v1 config schema.

package config

import (
	"fmt"
	"net/url"
	"strings"
)

// SchemaValidationError represents a single config validation error.
type SchemaValidationError struct {
	Path    string // Config path, e.g., "admins[0].port"
	Message string
}

func (e SchemaValidationError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("%s: %s", e.Path, e.Message)
	}
	return e.Message
}

// SchemaValidationResult contains all validation errors for a ProjectConfig.
type SchemaValidationResult struct {
	Errors []SchemaValidationError
}

// IsValid returns true if there are no validation errors.
func (r *SchemaValidationResult) IsValid() bool {
	return len(r.Errors) == 0
}

// Error returns a combined error message.
func (r *SchemaValidationResult) Error() string {
	if r.IsValid() {
		return ""
	}
	var msgs []string
	for _, e := range r.Errors {
		msgs = append(msgs, e.Error())
	}
	return strings.Join(msgs, "\n")
}

// AddError adds a validation error.
func (r *SchemaValidationResult) AddError(path, message string) {
	r.Errors = append(r.Errors, SchemaValidationError{Path: path, Message: message})
}

// ValidateProjectConfig validates a ProjectConfig structure and returns any errors found.
func ValidateProjectConfig(cfg *ProjectConfig) *SchemaValidationResult {
	result := &SchemaValidationResult{}

	// Version is required
	if cfg.Version == "" {
		result.AddError("version", "required")
	} else if cfg.Version != "1" {
		result.AddError("version", fmt.Sprintf("unsupported version %q, expected \"1\"", cfg.Version))
	}

	// Validate admins
	adminNames := make(map[string]bool)
	for i, admin := range cfg.Admins {
		path := fmt.Sprintf("admins[%d]", i)
		validateAdmin(&admin, path, adminNames, result)
	}

	// Validate engines
	engineNames := make(map[string]bool)
	for i, engine := range cfg.Engines {
		path := fmt.Sprintf("engines[%d]", i)
		validateEngine(&engine, path, engineNames, adminNames, result)
	}

	// Validate workspaces
	workspaceNames := make(map[string]bool)
	for i, workspace := range cfg.Workspaces {
		path := fmt.Sprintf("workspaces[%d]", i)
		validateWorkspace(&workspace, path, workspaceNames, engineNames, result)
	}

	// Validate mocks
	mockIDs := make(map[string]bool)
	for i, mock := range cfg.Mocks {
		path := fmt.Sprintf("mocks[%d]", i)
		validateMock(&mock, path, mockIDs, workspaceNames, result)
	}

	// Validate stateful resources
	resourceNames := make(map[string]bool)
	for i, resource := range cfg.StatefulResources {
		path := fmt.Sprintf("statefulResources[%d]", i)
		validateStatefulResource(&resource, path, resourceNames, workspaceNames, result)
	}

	return result
}

func validateAdmin(admin *AdminConfig, path string, names map[string]bool, result *SchemaValidationResult) {
	// Name is required
	if admin.Name == "" {
		result.AddError(path+".name", "required")
	} else {
		if names[admin.Name] {
			result.AddError(path+".name", fmt.Sprintf("duplicate admin name %q", admin.Name))
		}
		names[admin.Name] = true
	}

	if admin.IsLocal() {
		// Local admin requires port
		if admin.Port == 0 {
			result.AddError(path+".port", "required for local admin (no url specified)")
		} else if admin.Port < 1 || admin.Port > 65535 {
			result.AddError(path+".port", fmt.Sprintf("invalid port %d, must be 1-65535", admin.Port))
		}

		// Validate auth if specified
		if admin.Auth != nil {
			validateAdminAuth(admin.Auth, path+".auth", result)
		}

		// Validate persistence if specified
		if admin.Persistence != nil {
			validateAdminPersistence(admin.Persistence, path+".persistence", result)
		}

		// APIKey shouldn't be set for local admin
		if admin.APIKey != "" {
			result.AddError(path+".apiKey", "should not be set for local admin (use auth.keyFile instead)")
		}
	} else {
		// Remote admin requires valid URL
		if _, err := url.Parse(admin.URL); err != nil {
			result.AddError(path+".url", fmt.Sprintf("invalid URL: %v", err))
		}

		// Port shouldn't be set for remote admin
		if admin.Port != 0 {
			result.AddError(path+".port", "should not be set for remote admin (use url instead)")
		}

		// Auth shouldn't be set for remote admin
		if admin.Auth != nil {
			result.AddError(path+".auth", "should not be set for remote admin (use apiKey instead)")
		}

		// Persistence shouldn't be set for remote admin
		if admin.Persistence != nil {
			result.AddError(path+".persistence", "should not be set for remote admin")
		}
	}
}

func validateAdminAuth(auth *AdminAuthConfig, path string, result *SchemaValidationResult) {
	validTypes := map[string]bool{"api-key": true, "none": true}
	if auth.Type != "" && !validTypes[auth.Type] {
		result.AddError(path+".type", fmt.Sprintf("invalid auth type %q, must be \"api-key\" or \"none\"", auth.Type))
	}
}

func validateAdminPersistence(persistence *AdminPersistenceConfig, path string, result *SchemaValidationResult) {
	validTypes := map[string]bool{"sqlite": true, "memory": true}
	if persistence.Type != "" && !validTypes[persistence.Type] {
		result.AddError(path+".type", fmt.Sprintf("invalid persistence type %q, must be \"sqlite\" or \"memory\"", persistence.Type))
	}

	// Path is required for sqlite
	if persistence.Type == "sqlite" && persistence.Path == "" {
		result.AddError(path+".path", "required for sqlite persistence")
	}
}

func validateEngine(engine *EngineConfig, path string, names map[string]bool, adminNames map[string]bool, result *SchemaValidationResult) {
	// Name is required
	if engine.Name == "" {
		result.AddError(path+".name", "required")
	} else {
		if names[engine.Name] {
			result.AddError(path+".name", fmt.Sprintf("duplicate engine name %q", engine.Name))
		}
		names[engine.Name] = true
	}

	// Admin is required
	if engine.Admin == "" {
		result.AddError(path+".admin", "required")
	} else if !adminNames[engine.Admin] {
		result.AddError(path+".admin", fmt.Sprintf("references unknown admin %q", engine.Admin))
	}

	// At least one port must be specified
	if engine.HTTPPort == 0 && engine.HTTPSPort == 0 && engine.GRPCPort == 0 {
		result.AddError(path, "at least one of httpPort, httpsPort, or grpcPort must be specified")
	}

	// Validate ports
	if engine.HTTPPort != 0 && (engine.HTTPPort < 1 || engine.HTTPPort > 65535) {
		result.AddError(path+".httpPort", fmt.Sprintf("invalid port %d, must be 1-65535", engine.HTTPPort))
	}
	if engine.HTTPSPort != 0 && (engine.HTTPSPort < 1 || engine.HTTPSPort > 65535) {
		result.AddError(path+".httpsPort", fmt.Sprintf("invalid port %d, must be 1-65535", engine.HTTPSPort))
	}
	if engine.GRPCPort != 0 && (engine.GRPCPort < 1 || engine.GRPCPort > 65535) {
		result.AddError(path+".grpcPort", fmt.Sprintf("invalid port %d, must be 1-65535", engine.GRPCPort))
	}

	// Validate registration if specified
	if engine.Registration != nil {
		// Fingerprint validation
		if engine.Registration.Fingerprint != "" &&
			engine.Registration.Fingerprint != "auto" &&
			len(engine.Registration.Fingerprint) < 8 {
			result.AddError(path+".registration.fingerprint", "must be \"auto\" or at least 8 characters")
		}
	}
}

func validateWorkspace(workspace *WorkspaceConfig, path string, names map[string]bool, engineNames map[string]bool, result *SchemaValidationResult) {
	// Name is required
	if workspace.Name == "" {
		result.AddError(path+".name", "required")
	} else {
		if names[workspace.Name] {
			result.AddError(path+".name", fmt.Sprintf("duplicate workspace name %q", workspace.Name))
		}
		names[workspace.Name] = true
	}

	// Validate engine references
	for i, engineName := range workspace.Engines {
		if !engineNames[engineName] {
			result.AddError(path+fmt.Sprintf(".engines[%d]", i), fmt.Sprintf("references unknown engine %q", engineName))
		}
	}
}

func validateMock(mock *MockEntry, path string, ids map[string]bool, workspaceNames map[string]bool, result *SchemaValidationResult) {
	// Determine mock type
	typeCount := 0
	if mock.IsInline() {
		typeCount++
	}
	if mock.IsFileRef() {
		typeCount++
	}
	if mock.IsGlob() {
		typeCount++
	}

	if typeCount == 0 {
		result.AddError(path, "must specify either inline mock (id, type, http), file, or files")
	} else if typeCount > 1 {
		result.AddError(path, "cannot mix inline mock fields with file/files references")
	}

	// Validate inline mock
	if mock.IsInline() {
		if mock.ID == "" {
			result.AddError(path+".id", "required for inline mock")
		} else {
			if ids[mock.ID] {
				result.AddError(path+".id", fmt.Sprintf("duplicate mock id %q", mock.ID))
			}
			ids[mock.ID] = true
		}

		// Validate workspace reference (allow empty for default workspace)
		if mock.Workspace != "" && len(workspaceNames) > 0 && !workspaceNames[mock.Workspace] {
			result.AddError(path+".workspace", fmt.Sprintf("references unknown workspace %q", mock.Workspace))
		}

		// Validate type
		validTypes := map[string]bool{"http": true, "grpc": true, "graphql": true, "websocket": true}
		if mock.Type == "" {
			result.AddError(path+".type", "required for inline mock")
		} else if !validTypes[mock.Type] {
			result.AddError(path+".type", fmt.Sprintf("invalid type %q, must be one of: http, grpc, graphql, websocket", mock.Type))
		}

		// Validate HTTP mock
		if mock.Type == "http" {
			if mock.HTTP == nil {
				result.AddError(path+".http", "required when type is \"http\"")
			} else {
				validateHTTPMock(mock.HTTP, path+".http", result)
			}
		}
	}

	// File ref validation is minimal - just check it's not empty
	if mock.IsFileRef() && mock.File == "" {
		result.AddError(path+".file", "cannot be empty")
	}

	if mock.IsGlob() && mock.Files == "" {
		result.AddError(path+".files", "cannot be empty")
	}
}

func validateHTTPMock(http *HTTPMockConfig, path string, result *SchemaValidationResult) {
	// Matcher validation
	if http.Matcher.Path == "" && http.Matcher.PathPattern == "" {
		result.AddError(path+".matcher.path", "either path or pathPattern is required")
	}

	// Response validation
	if http.Response.StatusCode != 0 && (http.Response.StatusCode < 100 || http.Response.StatusCode > 599) {
		result.AddError(path+".response.statusCode", fmt.Sprintf("invalid status code %d, must be 100-599", http.Response.StatusCode))
	}

	// Body and BodyFile are mutually exclusive
	if http.Response.Body != "" && http.Response.BodyFile != "" {
		result.AddError(path+".response", "cannot specify both body and bodyFile")
	}
}

func validateStatefulResource(resource *StatefulResourceEntry, path string, names map[string]bool, workspaceNames map[string]bool, result *SchemaValidationResult) {
	// Name is required
	if resource.Name == "" {
		result.AddError(path+".name", "required")
	} else {
		if names[resource.Name] {
			result.AddError(path+".name", fmt.Sprintf("duplicate resource name %q", resource.Name))
		}
		names[resource.Name] = true
	}

	// BasePath is required
	if resource.BasePath == "" {
		result.AddError(path+".basePath", "required")
	} else if !strings.HasPrefix(resource.BasePath, "/") {
		result.AddError(path+".basePath", "must start with /")
	}

	// Validate workspace reference
	if resource.Workspace != "" && len(workspaceNames) > 0 && !workspaceNames[resource.Workspace] {
		result.AddError(path+".workspace", fmt.Sprintf("references unknown workspace %q", resource.Workspace))
	}
}

// ValidatePortConflicts checks for port conflicts between services.
// This is a separate validation that can be run after merging configs.
func ValidatePortConflicts(cfg *ProjectConfig) *SchemaValidationResult {
	result := &SchemaValidationResult{}
	usedPorts := make(map[int]string) // port -> service name

	// Check admin ports
	for i, admin := range cfg.Admins {
		if admin.IsLocal() && admin.Port != 0 {
			key := admin.Port
			if existing, used := usedPorts[key]; used {
				result.AddError(fmt.Sprintf("admins[%d].port", i),
					fmt.Sprintf("port %d conflicts with %s", admin.Port, existing))
			} else {
				usedPorts[key] = fmt.Sprintf("admin/%s", admin.Name)
			}
		}
	}

	// Check engine ports
	for i, engine := range cfg.Engines {
		ports := map[string]int{
			"httpPort":  engine.HTTPPort,
			"httpsPort": engine.HTTPSPort,
			"grpcPort":  engine.GRPCPort,
		}

		for portName, port := range ports {
			if port == 0 {
				continue
			}

			if existing, used := usedPorts[port]; used {
				result.AddError(fmt.Sprintf("engines[%d].%s", i, portName),
					fmt.Sprintf("port %d conflicts with %s", port, existing))
			} else {
				usedPorts[port] = fmt.Sprintf("engine/%s:%s", engine.Name, portName)
			}
		}
	}

	return result
}
