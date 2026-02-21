package cli

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/spf13/cobra"
)

// VersionOutput represents JSON output format
type VersionOutput struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show mockd version information",
	RunE: func(cmd *cobra.Command, args []string) error {
		version := Version
		commit := Commit
		date := BuildDate

		if info, ok := debug.ReadBuildInfo(); ok {
			if version == "dev" {
				version = info.Main.Version
			}
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					if commit == "none" {
						commit = setting.Value
					}
				case "vcs.time":
					if date == "unknown" {
						date = setting.Value
					}
				case "vcs.modified":
					if setting.Value == "true" {
						commit += "-dirty"
					}
				}
			}
		}

		out := VersionOutput{
			Version: version,
			Commit:  commit,
			Date:    date,
			Go:      runtime.Version(),
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
		}

		if jsonOutput {
			return output.JSON(out)
		}

		v := out.Version
		if len(v) > 0 && v[0] != 'v' && v != "dev" && v != "(devel)" {
			v = "v" + v
		}
		fmt.Printf("mockd %s (%s, %s)\n", v, out.Commit, out.Date)
		fmt.Printf("%s %s/%s\n", out.Go, out.OS, out.Arch)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
