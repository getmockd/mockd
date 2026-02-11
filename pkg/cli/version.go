package cli

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
)

// BuildInfo contains build-time information
type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
}

// VersionOutput represents JSON output format
type VersionOutput struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

// RunVersion handles the version command
func RunVersion(info BuildInfo, args []string) error {
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
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	out := VersionOutput{
		Version: info.Version,
		Commit:  info.Commit,
		Date:    info.BuildDate,
		Go:      runtime.Version(),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}

	if *jsonOutput {
		return output.JSON(out)
	}

	// Format: mockd v0.1.0 (abc1234, 2025-01-06)
	//         go1.21.0 linux/amd64
	// Note: Version may already include 'v' prefix from git tags
	version := out.Version
	if len(version) > 0 && version[0] != 'v' {
		version = "v" + version
	}
	fmt.Printf("mockd %s (%s, %s)\n", version, out.Commit, out.Date)
	fmt.Printf("%s %s/%s\n", out.Go, out.OS, out.Arch)
	return nil
}
