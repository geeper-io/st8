package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

const timeFormat = "2006-01-02T15:04:05Z07:00"

func (a *App) logCommand() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show revision history",
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, cleanup, err := a.chooseClient(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			entries, err := backend.Log(cmd.Context(), a.opts.scope, limit)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				fmt.Fprintf(a.stdout, "rev=%d parent=%d branch=%s at=%s message=%q changes=%d\n",
					entry.ID, entry.ParentID, entry.Branch, entry.CreatedAt.Format(timeFormat), entry.Message, len(entry.Changes))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "max revisions to show")
	return cmd
}
