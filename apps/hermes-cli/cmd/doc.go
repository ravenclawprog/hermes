package cmd

import (
	"fmt"
	"hermes-cli/editor"

	"github.com/spf13/cobra"
)

var docCmd = &cobra.Command{
	Use:   "doc",
	Short: "document managment",
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Show list of availaible docs",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: query for list of docs
		fmt.Fprintln(cmd.OutOrStdout(), "Doc list: [...]")
	},
}

var editCmd = &cobra.Command{
	Use:   "edit [document_id]",
	Short: "Edit documt at system editor",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		docID := args[0]
		fmt.Fprintln(cmd.OutOrStdout(), "Open docs ", docID, " for editoring")

		// stub
		stubRemoteContent := "Hello, Hermes! Edit this line."

		// calling editor
		updatedContent, err := editor.OpenInSystemEditor(stubRemoteContent)
		if err != nil {
			fmt.Printf("failed to open editor: %v", err)
			return
		}

		fmt.Fprintln(cmd.OutOrStdout(), "\n--- Document updated successfully! ---")
		fmt.Fprintln(cmd.OutOrStdout(), updatedContent)
	},
}

func init() {
	docCmd.AddCommand(listCmd)
	docCmd.AddCommand(editCmd)
	rootCmd.AddCommand(docCmd)
}
