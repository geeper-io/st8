package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) checkpointCommand() *cobra.Command {
	var description string
	cmd := &cobra.Command{
		Use:   "checkpoint <name>",
		Short: "Create a named checkpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, cleanup, err := a.chooseClient(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			res, err := backend.Checkpoint(cmd.Context(), a.opts.scope, args[0], description)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.stdout, "Checkpoint %s -> revision %d\n", res.Name, res.RevisionID)
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "checkpoint description")
	cmd.AddCommand(a.checkpointDeleteCommand())
	return cmd
}

func (a *App) checkpointDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a named checkpoint",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, cleanup, err := a.chooseClient(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			if err := backend.DeleteCheckpoint(cmd.Context(), a.opts.scope.Namespace, args[0]); err != nil {
				return err
			}
			fmt.Fprintf(a.stdout, "Deleted checkpoint %s\n", args[0])
			return nil
		},
	}
}
