package cli

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var httpCmd = &cobra.Command{
	Use:   "http",
	Short: "Manage and test HTTP/REST endpoints",
	Long:  `Manage and test HTTP/REST endpoints.`,
}

var httpAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new HTTP mock endpoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		// If flags were intentionally omitted (e.g., just ran "mockd http add"), run interactive prompt
		if !cmd.Flags().Changed("path") {
			var formPath, formMethod, formBody string
			var formStatus int
			formStatusStr := "200"

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("What is the URL path to match?").
						Placeholder("/api/v1/users").
						Value(&formPath).
						Validate(func(s string) error {
							if s == "" {
								return errors.New("path is required")
							}
							return nil
						}),
					huh.NewSelect[string]().
						Title("What HTTP method should it respond to?").
						Options(
							huh.NewOption("GET", "GET"),
							huh.NewOption("POST", "POST"),
							huh.NewOption("PUT", "PUT"),
							huh.NewOption("DELETE", "DELETE"),
							huh.NewOption("PATCH", "PATCH"),
						).
						Value(&formMethod),
					huh.NewInput().
						Title("What status code should it return?").
						Value(&formStatusStr),
					huh.NewText().
						Title("Response Body (JSON)").
						Placeholder(`{"status": "ok"}`).
						Value(&formBody),
				),
			)

			err := form.Run()
			if err != nil {
				return err
			}

			// Apply form results if they didn't abort
			_, _ = fmt.Sscanf(formStatusStr, "%d", &formStatus)
			_ = cmd.Flags().Set("path", formPath)
			_ = cmd.Flags().Set("method", formMethod)
			_ = cmd.Flags().Set("status", formStatusStr)
			_ = cmd.Flags().Set("body", formBody)
		}

		// Delegate to the shared generic RunAdd (which is now cobra runAdd)
		// We have to set the "type" flag explicitly for the generic add router
		addMockType = "http"
		return runAdd(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(httpCmd)
	httpCmd.AddCommand(httpAddCmd)

	// Since we delegate to runAdd, we need to bind the exact same flags `mockd add` has
	// so parsing context carries over cleanly. We'll reuse the global targets for HTTP specifically.
	httpAddCmd.Flags().StringVar(&addPath, "path", "", "URL path to match")
	httpAddCmd.Flags().StringVar(&addMethod, "method", "GET", "HTTP method to match")
	httpAddCmd.Flags().IntVar(&addStatus, "status", 200, "Response status code")
	httpAddCmd.Flags().StringVar(&addBody, "body", "", "Response body")
	httpAddCmd.Flags().StringVar(&addName, "name", "", "Mock display name")
	httpAddCmd.Flags().StringVar(&addStatefulOperation, "stateful-operation", "", "Wire to a custom stateful operation (e.g., TransferFunds)")

	// Missing commands like list/get/delete fall back to root aliases exactly like before
	httpCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List HTTP mocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			listMockType = "http"
			return runList(cmd, args)
		},
	})
}
