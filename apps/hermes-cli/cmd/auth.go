package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth [username]",
	Short: "Authorization at system",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		username := args[0]
		// TODO: create http connection
		fmt.Fprintln(cmd.OutOrStdout(), "Successful authorization for user:", username)
	},
}

func init() {
	rootCmd.AddCommand(authCmd)
}
