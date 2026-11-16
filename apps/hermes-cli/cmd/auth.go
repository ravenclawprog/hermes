package cmd

import (
	"fmt"
	"hermes-cli/client"

	"github.com/spf13/cobra"
)

func newAuthCmd(
	authClient client.AuthClient,
	sessionStore client.SessionStore,
	readPassword func() ([]byte, error),
) *cobra.Command {
	return &cobra.Command{
		Use:   "auth [username]",
		Short: "Authorization at system",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if authClient == nil {
				return fmt.Errorf("set HERMES_SERVER_URL to the document-service address")
			}
			if sessionStore == nil {
				return fmt.Errorf("session store is not configured")
			}
			username := args[0]
			fmt.Fprint(cmd.ErrOrStderr(), "Password: ")
			password, err := readPassword()
			fmt.Fprintln(cmd.ErrOrStderr())

			if err != nil {
				return fmt.Errorf("read password: %w", err)
			}
			if len(password) == 0 {
				return fmt.Errorf("password must not be empty")
			}

			session, err := authClient.Authenticate(cmd.Context(), username, password)
			if err != nil {
				return fmt.Errorf("authenticate: %w", err)
			}
			if err := sessionStore.Save(cmd.Context(), session); err != nil {
				return fmt.Errorf("save session: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Signed in as %s\n", session.UserID)
			return nil

		},
	}
}
