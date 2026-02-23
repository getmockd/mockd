package cli

import (
	"fmt"
	"os"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/config"
	"github.com/spf13/cobra"
)

var psPidFile string

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "Show status of running mockd services",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Try serve/start PID file format first (cli.PIDFile), then
		// fall back to up PID file format (config.PIDFile). Both write
		// to ~/.mockd/mockd.pid but with different JSON schemas.
		if serveInfo, err := ReadPIDFile(psPidFile); err == nil {
			running := serveInfo.IsRunning()
			printResult(buildPsResultFromServe(serveInfo, running), func() {
				printPsTableFromServe(serveInfo, running)
			})
			return nil
		}

		// Fall back to up/down format
		pidInfo, err := readUpPIDFile(psPidFile)
		if err != nil {
			if os.IsNotExist(err) {
				printResult(map[string]any{"running": false, "services": []any{}}, func() {
					fmt.Println("No running mockd services.")
				})
				return nil
			}
			return fmt.Errorf("reading PID file: %w", err)
		}

		// Check if main process is running
		running := processExists(pidInfo.PID)

		printResult(buildPsResult(pidInfo, running), func() {
			printPsTable(pidInfo, running)
		})
		return nil
	},
}

func init() {
	psCmd.Flags().StringVar(&psPidFile, "pid-file", defaultUpPIDPath(), "Path to PID file")
	rootCmd.AddCommand(psCmd)
}

// buildPsResultFromServe builds JSON output from a serve/start PID file.
func buildPsResultFromServe(info *PIDFile, running bool) any {
	type svc struct {
		Name   string `json:"name"`
		Type   string `json:"type"`
		Port   int    `json:"port"`
		Status string `json:"status"`
	}
	var services []svc
	if info.Components.Admin.Enabled {
		st := "running"
		if !running {
			st = "stopped"
		}
		services = append(services, svc{Name: "admin", Type: "admin", Port: info.Components.Admin.Port, Status: st})
	}
	if info.Components.Engine.Enabled {
		st := "running"
		if !running {
			st = "stopped"
		}
		services = append(services, svc{Name: "engine", Type: "engine", Port: info.Components.Engine.Port, Status: st})
	}
	return struct {
		Running   bool   `json:"running"`
		PID       int    `json:"pid"`
		StartedAt string `json:"startedAt"`
		Version   string `json:"version,omitempty"`
		Services  []svc  `json:"services"`
	}{
		Running:   running,
		PID:       info.PID,
		StartedAt: info.StartTime.Format("2006-01-02T15:04:05Z07:00"),
		Version:   info.Version,
		Services:  services,
	}
}

// printPsTableFromServe prints human-readable output from a serve/start PID file.
func printPsTableFromServe(info *PIDFile, running bool) {
	status := "running"
	if !running {
		status = "stopped (stale PID file)"
	}

	fmt.Printf("mockd (PID %d) - %s\n", info.PID, status)
	fmt.Printf("Started: %s\n", info.StartTime.Format("2006-01-02T15:04:05Z07:00"))
	if info.Version != "" {
		fmt.Printf("Version: %s\n", info.Version)
	}
	fmt.Println()

	w := output.Table()
	_, _ = fmt.Fprintln(w, "NAME\tTYPE\tPORT\tSTATUS")

	if info.Components.Admin.Enabled {
		st := "running"
		if !running {
			st = "stopped"
		}
		_, _ = fmt.Fprintf(w, "admin\tadmin\t%d\t%s\n", info.Components.Admin.Port, st)
	}
	if info.Components.Engine.Enabled {
		st := "running"
		if !running {
			st = "stopped"
		}
		_, _ = fmt.Fprintf(w, "engine\tengine\t%d\t%s\n", info.Components.Engine.Port, st)
	}

	_ = w.Flush()
}

func buildPsResult(pidInfo *config.PIDFile, running bool) any {
	return struct {
		Running   bool                    `json:"running"`
		PID       int                     `json:"pid"`
		StartedAt string                  `json:"startedAt"`
		Config    string                  `json:"config"`
		Services  []config.PIDFileService `json:"services"`
	}{
		Running:   running,
		PID:       pidInfo.PID,
		StartedAt: pidInfo.StartedAt,
		Config:    pidInfo.Config,
		Services:  pidInfo.Services,
	}
}

func printPsTable(pidInfo *config.PIDFile, running bool) {
	status := "running"
	if !running {
		status = "stopped (stale PID file)"
	}

	fmt.Printf("mockd (PID %d) - %s\n", pidInfo.PID, status)
	fmt.Printf("Started: %s\n", pidInfo.StartedAt)
	fmt.Printf("Config: %s\n", pidInfo.Config)
	fmt.Println()

	if len(pidInfo.Services) == 0 {
		fmt.Println("No services recorded.")
		return
	}

	w := output.Table()
	_, _ = fmt.Fprintln(w, "NAME\tTYPE\tPORT\tSTATUS")

	for _, svc := range pidInfo.Services {
		svcStatus := "running"
		if !running {
			svcStatus = "stopped"
		} else if svc.PID > 0 && !processExists(svc.PID) {
			svcStatus = "stopped"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", svc.Name, svc.Type, svc.Port, svcStatus)
	}

	_ = w.Flush()
}
