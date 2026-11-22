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

func readPassword() ([]byte, error) {
	return term.ReadPassword(int(os.Stdin.Fd()))
}

func Execute() error {
	var connectionClient *client.HTTPClient
	var sessionStore client.SessionStore

	if serverURL := os.Getenv("HERMES_SERVER_URL"); serverURL != "" {
		store, err := newDefaultSessionStore(serverURL)
		if err != nil {
			return err
		}

		connectionClient = client.NewHTTPClient(serverURL)
		sessionStore = store
	}

	rootCmd.AddCommand(newAuthCmd(connectionClient, sessionStore, readPassword))
	rootCmd.AddCommand(newDocCmd(connectionClient, sessionStore))

	return rootCmd.Execute()
}
