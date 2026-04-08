package cli

import (
	"fmt"
	"strings"

	st8client "github.com/geeper-io/st8/client"
	"github.com/spf13/cobra"
)

func (a *App) restoreCommand() *cobra.Command {
	var revision int64
	var checkpoint string
	var fromBranch string
	var message string
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore state from a revision, checkpoint, or branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, cleanup, err := a.chooseClient(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			res, err := backend.Restore(cmd.Context(), st8client.RestoreInput{
				Scope:          a.opts.scope,
				FromRevision:   revision,
				FromCheckpoint: checkpoint,
				FromBranch:     fromBranch,
				Message:        message,
			})
			if err != nil {
				return err
			}
			if res.Noop {
				fmt.Fprintln(a.stdout, "Already at requested state.")
				return nil
			}
			fmt.Fprintf(a.stdout, "Restored into revision %d.\n", res.Revision)
			for _, change := range res.Changes {
				fmt.Fprintf(a.stdout, "%s %s\n", strings.ToUpper(change.Type), change.Key)
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&revision, "revision", 0, "restore from revision")
	cmd.Flags().StringVar(&checkpoint, "checkpoint", "", "restore from checkpoint")
	cmd.Flags().StringVar(&fromBranch, "from-branch", "", "restore from another branch")
	cmd.Flags().StringVar(&message, "message", "", "revision message")
	return cmd
}
