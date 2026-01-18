// Package output provides common output formatting utilities.
package output

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
)

// JSON writes indented JSON to stdout.
func JSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Table creates an aligned table writer for stdout.
// Remember to call Flush() when done writing.
func Table() *tabwriter.Writer {
	return tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
}

// Warn prints a warning message to stderr.
func Warn(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "Warning: "+format+"\n", args...)
}
