package cli

import "github.com/getmockd/mockd/pkg/cli/internal/output"

// printResult outputs a single operation result.
//
// Contract: when --json is active, ONLY the JSON encoding of data is written
// to stdout. Human-readable prose (progress messages, hints) must go to stderr
// or be omitted entirely. textFn is called only in text mode.
func printResult(data any, textFn func()) {
	if jsonOutput {
		_ = output.JSON(data)
		return
	}
	textFn()
}

// printList outputs a collection of items.
//
// Same contract as printResult. textFn typically uses output.Table() for
// aligned columns.
func printList(data any, textFn func()) {
	if jsonOutput {
		_ = output.JSON(data)
		return
	}
	textFn()
}
