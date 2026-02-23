package cli

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var oauthCmd = &cobra.Command{
	Use:   "oauth",
	Short: "Manage and test OAuth/OIDC endpoints",
}

var oauthAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new OAuth/OIDC mock provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Use huh interactive forms if attributes are missing
		if !cmd.Flags().Changed("issuer") && !cmd.Flags().Changed("client-id") {
			var formIssuer, formClientID, formClientSecret, formUsername, formPassword string

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("OAuth issuer URL").
						Placeholder("http://localhost:4280").
						Value(&formIssuer),
					huh.NewInput().
						Title("Client ID").
						Placeholder("test-client").
						Value(&formClientID).
						Validate(func(s string) error {
							if s == "" {
								return errors.New("client ID is required")
							}
							return nil
						}),
					huh.NewInput().
						Title("Client Secret").
						Placeholder("test-secret").
						Value(&formClientSecret),
					huh.NewInput().
						Title("Test Username").
						Placeholder("testuser").
						Value(&formUsername),
					huh.NewInput().
						Title("Test Password").
						Placeholder("password").
						EchoMode(huh.EchoModePassword).
						Value(&formPassword),
				),
			)
			if err := form.Run(); err != nil {
				return err
			}
			if formIssuer != "" {
				addIssuer = formIssuer
			}
			if formClientID != "" {
				addClientID = formClientID
			}
			if formClientSecret != "" {
				addClientSecret = formClientSecret
			}
			if formUsername != "" {
				addOAuthUser = formUsername
			}
			if formPassword != "" {
				addOAuthPassword = formPassword
			}
		}
		addMockType = "oauth"
		return runAdd(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(oauthCmd)
	oauthCmd.AddCommand(oauthAddCmd)

	oauthAddCmd.Flags().StringVar(&addIssuer, "issuer", "", "OAuth issuer URL (default: http://localhost:4280)")
	oauthAddCmd.Flags().StringVar(&addClientID, "client-id", "test-client", "OAuth client ID")
	oauthAddCmd.Flags().StringVar(&addClientSecret, "client-secret", "test-secret", "OAuth client secret")
	oauthAddCmd.Flags().StringVar(&addOAuthUser, "oauth-user", "testuser", "OAuth test username")
	oauthAddCmd.Flags().StringVar(&addOAuthPassword, "oauth-password", "password", "OAuth test password")

	// Add list/get/delete generic aliases
	oauthCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List OAuth mocks",
		RunE: func(cmd *cobra.Command, args []string) error {
			listMockType = "oauth"
			return runList(cmd, args)
		},
	})
	oauthCmd.AddCommand(&cobra.Command{
		Use:   "get",
		Short: "Get details of an OAuth mock",
		RunE:  runGet,
	})
	oauthCmd.AddCommand(&cobra.Command{
		Use:   "delete",
		Short: "Delete an OAuth mock",
		RunE:  runDelete,
	})

	oauthCmd.AddCommand(oauthStatusCmd)
}

var oauthStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the OAuth provider status",
	RunE:  runOAuthStatus,
}

// runOAuthStatus shows the current OAuth provider status.
func runOAuthStatus(_ *cobra.Command, _ []string) error {
	// Get OAuth status from admin API
	client := NewAdminClientWithAuth(adminURL)
	mocks, err := client.ListMocksByType("oauth")
	if err != nil {
		return fmt.Errorf("failed to get OAuth status: %s", FormatConnectionError(err))
	}

	printResult(map[string]any{
		"count": len(mocks),
		"mocks": mocks,
	}, func() {
		if len(mocks) == 0 {
			fmt.Println("No OAuth providers configured")
			return
		}
		fmt.Printf("OAuth providers: %d\n", len(mocks))
		for _, m := range mocks {
			name := m.Name
			id := m.ID
			enabled := m.Enabled != nil && *m.Enabled
			status := "disabled"
			if enabled {
				status = "enabled"
			}
			fmt.Printf("  %s (%s) [%s]\n", name, id, status)
		}
	})
	return nil
}
