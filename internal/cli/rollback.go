package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func (a *App) rollbackCommand() *cobra.Command {
	var revision int64
	var checkpoint string
	var message string
	cmd := &cobra.Command{
		Use:   "rollback",
		Short: "Roll back to a revision or checkpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, cleanup, err := a.chooseClient(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			res, err := backend.Rollback(cmd.Context(), a.opts.scope, revision, checkpoint, message)
			if err != nil {
				return err
			}
			if res.Noop {
				fmt.Fprintln(a.stdout, "Already at requested state.")
				return nil
			}
			fmt.Fprintf(a.stdout, "Rolled back at revision %d.\n", res.Revision)
			for _, change := range res.Changes {
				fmt.Fprintf(a.stdout, "%s %s\n", strings.ToUpper(change.Type), change.Key)
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&revision, "revision", 0, "rollback target revision")
	cmd.Flags().StringVar(&checkpoint, "checkpoint", "", "rollback target checkpoint")
	cmd.Flags().StringVar(&message, "message", "", "revision message")
	return cmd
}
