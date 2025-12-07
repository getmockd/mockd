package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// RunVersion handles the version command
func RunVersion(version string, args []string) error {
	fs := flag.NewFlagSet("version", flag.ContinueOnError)
	jsonOutput := fs.Bool("json", false, "Output version in JSON format")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd version [flags]

Show mockd version information.

Flags:
  --json    Output in JSON format

Examples:
  mockd version
  mockd version --json
`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *jsonOutput {
		out := struct {
			Version string `json:"version"`
		}{Version: version}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}

	fmt.Printf("mockd version %s\n", version)
	return nil
}
