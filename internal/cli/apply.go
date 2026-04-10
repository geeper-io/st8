package cli

import (
	"fmt"
	"strings"

	st8client "github.com/geeper-io/st8/client"
	"github.com/spf13/cobra"
)

func (a *App) applyCommand() *cobra.Command {
	var message string
	var files, values []string
	cmd := &cobra.Command{
		Use:   "apply [-f file...] [--value key=value...] [file...]",
		Short: "Apply files and/or key=value pairs into state",
		RunE: func(cmd *cobra.Command, args []string) error {
			allFiles := append(files, args...)
			if len(allFiles) == 0 && len(values) == 0 {
				return fmt.Errorf("provide at least one -f <file> or --value key=value")
			}
			backend, cleanup, err := a.chooseClient(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			docs, err := loadDocumentsWithValues(allFiles, values)
			if err != nil {
				return err
			}
			res, err := backend.Apply(cmd.Context(), st8client.ApplyInput{
				Scope:     a.opts.scope,
				Documents: docs,
				Message:   message,
			})
			if err != nil {
				return err
			}
			if res.Noop {
				fmt.Fprintf(a.stdout, "No changes. Head stays at revision %d.\n", res.Revision)
				return nil
			}
			fmt.Fprintf(a.stdout, "Applied %d change(s) at revision %d.\n", len(res.Changes), res.Revision)
			for _, change := range res.Changes {
				fmt.Fprintf(a.stdout, "%s %s\n", strings.ToUpper(change.Type), change.Key)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&message, "message", "", "revision message")
	cmd.Flags().StringArrayVarP(&files, "file", "f", nil, "file or directory to apply (repeatable)")
	cmd.Flags().StringArrayVar(&values, "value", nil, "key=value pair to apply (repeatable)")
	return cmd
}
