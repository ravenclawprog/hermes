package cmd

import (
	"hermes-cli/client"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var rootCmd = &cobra.Command{
	Use:   "hermes",
	Short: "Hermes — CLI-platform for coop work with documents",
	Long:  "Console client for managing, creating and editoring remote docs at real time",
}

func Execute() error {
	var authClient client.AuthClient
	var sessionStore client.SessionStore

	if serverURL := os.Getenv("HERMES_SERVER_URL"); serverURL != "" {
		store, err := newDefaultSessionStore(serverURL)
		if err != nil {
			return err
		}

		authClient = client.NewHTTPClient(serverURL)
		sessionStore = store
	}

	rootCmd.AddCommand(newAuthCmd(
		authClient,
		sessionStore,
		func() ([]byte, error) {
			return term.ReadPassword(int(os.Stdin.Fd()))
		},
	))

	return rootCmd.Execute()
}

func init() {
	// var verbose bool
}
