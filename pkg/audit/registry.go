package audit

import "sync"

var (
	registryMu         sync.RWMutex
	registeredWriters  = make(map[string]WriterFactory)
	registeredRedactor RedactorFunc
)

type WriterFactory func(config map[string]interface{}) (AuditLogger, error)
type RedactorFunc func(entry *AuditEntry) *AuditEntry

func RegisterWriter(name string, factory WriterFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registeredWriters[name] = factory
}

func RegisterRedactor(fn RedactorFunc) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registeredRedactor = fn
}

func GetRegisteredWriter(name string) (WriterFactory, bool) {
	registryMu.RLock()
	defer registryMu.RUnlock()
	f, ok := registeredWriters[name]
	return f, ok
}

func GetRegisteredRedactor() RedactorFunc {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registeredRedactor
}
