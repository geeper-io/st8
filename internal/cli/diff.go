package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) diffCommand() *cobra.Command {
	var revision int64
	var checkpoint string
	var files, values []string
	cmd := &cobra.Command{
		Use:   "diff [-f file...] [--value key=value...] [file...]",
		Short: "Compare current state to files, key=value pairs, or a prior revision",
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, cleanup, err := a.chooseClient(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			docs, err := loadDocumentsWithValues(append(files, args...), values)
			if err != nil {
				return err
			}
			res, err := backend.DiffDocuments(cmd.Context(), a.opts.scope, revision, checkpoint, docs)
			if err != nil {
				return err
			}
			if len(res.Changes) == 0 {
				fmt.Fprintln(a.stdout, "No differences.")
				return nil
			}
			fmt.Fprintf(a.stdout, "Diff from revision %d to %d\n", res.FromRevision, res.ToRevision)
			for _, change := range res.Changes {
				renderChange(a.stdout, change)
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&revision, "revision", 0, "diff against a specific revision")
	cmd.Flags().StringVar(&checkpoint, "checkpoint", "", "diff against a named checkpoint")
	cmd.Flags().StringArrayVarP(&files, "file", "f", nil, "file or directory to diff (repeatable)")
	cmd.Flags().StringArrayVarP(&values, "value", "v", nil, "key=value pair to diff (repeatable)")
	return cmd
}
