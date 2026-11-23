package cmd

import (
	"fmt"
	"hermes-cli/client"

	"github.com/spf13/cobra"
)

func newDocCmd(
	docServClient client.DocumentClient,
	sessionStore client.SessionStore,
) *cobra.Command {
	doc := &cobra.Command{
		Use:   "doc",
		Short: "Commands with documents"}
	doc.AddCommand(&cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			if docServClient == nil {
				return fmt.Errorf("set HERMES_SERVER_URL to the document-service address")
			}
			if sessionStore == nil {
				return fmt.Errorf("session store is not configured")
			}
			session, err := sessionStore.Load(cmd.Context())
			if err != nil {
				return fmt.Errorf("cannot load session: %w", err)
			}
			page, err := docServClient.List(cmd.Context(), session)
			if err != nil {
				return fmt.Errorf("list documents: %w", err)
			}
			for _, document := range page.Items {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%d\n",
					document.ID, document.WorkspaceID, document.Title, document.Version); err != nil {
					return fmt.Errorf("write document list: %w", err)
				}
			}

			return nil
		},
	})
	doc.AddCommand(&cobra.Command{
		Use:  "read [document_id]",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if docServClient == nil {
				return fmt.Errorf("set HERMES_SERVER_URL to the document-service address")
			}
			if sessionStore == nil {
				return fmt.Errorf("session store is not configured")
			}
			session, err := sessionStore.Load(cmd.Context())
			if err != nil {
				return fmt.Errorf("cannot load session: %w", err)
			}
			documentID := args[0]
			document, err := docServClient.Read(cmd.Context(), session, documentID)
			if err != nil {
				return fmt.Errorf("list documents: %w", err)
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%d\nContent:\n%s",
				document.Summary.ID, document.Summary.WorkspaceID, document.Summary.Title, document.Summary.Version, document.Content); err != nil {
				return fmt.Errorf("write document read: %w", err)
			}
			return nil
		},
	})
	return doc
}
