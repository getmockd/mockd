package audit

var (
	registeredWriters  = make(map[string]WriterFactory)
	registeredRedactor RedactorFunc
)

type WriterFactory func(config map[string]interface{}) (AuditLogger, error)
type RedactorFunc func(entry *AuditEntry) *AuditEntry

func RegisterWriter(name string, factory WriterFactory) {
	registeredWriters[name] = factory
}

func RegisterRedactor(fn RedactorFunc) {
	registeredRedactor = fn
}

func GetRegisteredWriter(name string) (WriterFactory, bool) {
	f, ok := registeredWriters[name]
	return f, ok
}

func GetRegisteredRedactor() RedactorFunc {
	return registeredRedactor
}
