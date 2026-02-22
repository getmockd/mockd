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
		// Read PID file
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
