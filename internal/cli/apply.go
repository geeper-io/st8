package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/geeper-io/st8/internal/service"
)

func (a *App) applyCommand() *cobra.Command {
	var message string
	cmd := &cobra.Command{
		Use:   "apply <file> [file...]",
		Short: "Apply one or more files into state",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, cleanup, err := a.chooseClient(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			docs, err := loadDocuments(args)
			if err != nil {
				return err
			}
			res, err := backend.Apply(cmd.Context(), service.ApplyInput{
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
	return cmd
}
