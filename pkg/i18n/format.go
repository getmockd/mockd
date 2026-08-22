package i18n

import "fmt"

// sprintf is a thin wrapper around fmt.Sprintf so that the i18n package
// has a single formatting dependency and can be swapped if needed.
func sprintf(format string, args ...interface{}) string {
	if len(args) == 0 {
		return format
	}
	return fmt.Sprintf(format, args...)
}
