package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) branchCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch",
		Short: "List branches or create a new branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, cleanup, err := a.chooseClient(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			branches, err := backend.ListBranches(cmd.Context(), a.opts.scope)
			if err != nil {
				return err
			}
			for _, branch := range branches {
				current := ""
				if branch.Current {
					current = " *"
				}
				fmt.Fprintf(a.stdout, "%s head=%d base=%d%s\n", branch.Name, branch.HeadRevision, branch.BaseRevision, current)
			}
			return nil
		},
	}
	cmd.AddCommand(a.branchCreateCommand())
	return cmd
}

func (a *App) branchCreateCommand() *cobra.Command {
	var revision int64
	var checkpoint string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a new branch",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, cleanup, err := a.chooseClient(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			res, err := backend.CreateBranch(cmd.Context(), a.opts.scope, args[0], revision, checkpoint)
			if err != nil {
				return err
			}
			fmt.Fprintf(a.stdout, "Created branch %s at revision %d.\n", res.Name, res.HeadRevision)
			return nil
		},
	}
	cmd.Flags().Int64Var(&revision, "revision", 0, "base revision")
	cmd.Flags().StringVar(&checkpoint, "checkpoint", "", "base checkpoint")
	return cmd
}
