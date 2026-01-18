package portability

import (
	"sync"
)

// Registry manages importers and exporters for different formats.
type Registry struct {
	mu        sync.RWMutex
	importers map[Format]Importer
	exporters map[Format]Exporter
}

// defaultRegistry is the global registry instance.
var defaultRegistry = &Registry{
	importers: make(map[Format]Importer),
	exporters: make(map[Format]Exporter),
}

// RegisterImporter adds an importer to the default registry.
func RegisterImporter(importer Importer) {
	defaultRegistry.RegisterImporter(importer)
}

// RegisterExporter adds an exporter to the default registry.
func RegisterExporter(exporter Exporter) {
	defaultRegistry.RegisterExporter(exporter)
}

// GetImporter returns the importer for a format from the default registry.
func GetImporter(format Format) Importer {
	return defaultRegistry.GetImporter(format)
}

// GetExporter returns the exporter for a format from the default registry.
func GetExporter(format Format) Exporter {
	return defaultRegistry.GetExporter(format)
}

// ListImporters returns all registered importers from the default registry.
func ListImporters() []Importer {
	return defaultRegistry.ListImporters()
}

// ListExporters returns all registered exporters from the default registry.
func ListExporters() []Exporter {
	return defaultRegistry.ListExporters()
}

// RegisterImporter adds an importer to the registry.
func (r *Registry) RegisterImporter(importer Importer) {
	if importer == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.importers[importer.Format()] = importer
}

// RegisterExporter adds an exporter to the registry.
func (r *Registry) RegisterExporter(exporter Exporter) {
	if exporter == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.exporters[exporter.Format()] = exporter
}

// GetImporter returns the importer for a format.
func (r *Registry) GetImporter(format Format) Importer {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.importers[format]
}

// GetExporter returns the exporter for a format.
func (r *Registry) GetExporter(format Format) Exporter {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.exporters[format]
}

// ListImporters returns all registered importers.
func (r *Registry) ListImporters() []Importer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Importer, 0, len(r.importers))
	for _, imp := range r.importers {
		result = append(result, imp)
	}
	return result
}

// ListExporters returns all registered exporters.
func (r *Registry) ListExporters() []Exporter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Exporter, 0, len(r.exporters))
	for _, exp := range r.exporters {
		result = append(result, exp)
	}
	return result
}

// HasImporter checks if an importer is registered for the format.
func (r *Registry) HasImporter(format Format) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.importers[format]
	return ok
}

// HasExporter checks if an exporter is registered for the format.
func (r *Registry) HasExporter(format Format) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.exporters[format]
	return ok
}
