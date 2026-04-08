package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) gcCommand() *cobra.Command {
	var keep int
	cmd := &cobra.Command{
		Use:   "gc",
		Short: "Prune old revisions",
		Long:  "Prune revisions beyond the keep limit. Checkpointed and branch-base revisions are always retained.",
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, cleanup, err := a.chooseClient(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			res, err := backend.GC(cmd.Context(), keep)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.stdout, "pruned=%d\n", res.Pruned)
			return nil
		},
	}
	cmd.Flags().IntVar(&keep, "keep", 10, "number of recent revisions to keep per branch")
	return cmd
}
