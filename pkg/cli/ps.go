package cli

import (
	"flag"
	"fmt"
	"os"

	"github.com/getmockd/mockd/pkg/cli/internal/output"
	"github.com/getmockd/mockd/pkg/config"
)

// RunPs shows status of running mockd services.
func RunPs(args []string) error {
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)

	pidFile := fs.String("pid-file", defaultUpPIDPath(), "Path to PID file")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")

	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: mockd ps [flags]

Show status of running mockd services.

Flags:
      --pid-file <path>  Path to PID file (default: ~/.mockd/mockd.pid)
      --json             Output in JSON format

Examples:
  # Show running services
  mockd ps

  # JSON output
  mockd ps --json
`)
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}

	// Read PID file
	pidInfo, err := readUpPIDFile(*pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			if *jsonOutput {
				fmt.Println(`{"running":false,"services":[]}`)
			} else {
				fmt.Println("No running mockd services.")
			}
			return nil
		}
		return fmt.Errorf("reading PID file: %w", err)
	}

	// Check if main process is running
	running := processExists(pidInfo.PID)

	if *jsonOutput {
		return printPsJSON(pidInfo, running)
	}

	return printPsTable(pidInfo, running)
}

func printPsJSON(pidInfo *config.PIDFile, running bool) error {
	result := struct {
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

	return output.JSON(result)
}

func printPsTable(pidInfo *config.PIDFile, running bool) error {
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
		return nil
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

	return w.Flush()
}
