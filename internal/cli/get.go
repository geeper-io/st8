package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
)

func (a *App) getCommand() *cobra.Command {
	var revision int64
	var checkpoint string
	cmd := &cobra.Command{
		Use:   "get [key]",
		Short: "Read current or historical state",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			backend, cleanup, err := a.chooseClient(cmd.Context())
			if err != nil {
				return err
			}
			defer cleanup()
			res, err := backend.Get(cmd.Context(), a.opts.scope, revision, checkpoint)
			if err != nil {
				return err
			}
			if len(args) == 1 {
				key := filepath.ToSlash(filepath.Clean(args[0]))
				value, ok := res.Objects[key]
				if !ok {
					return fmt.Errorf("key %q not found", key)
				}
				fmt.Fprint(a.stdout, value)
				return nil
			}
			fmt.Fprintf(a.stdout, "revision: %d\n", res.Revision)
			for _, key := range sortedObjectKeys(res.Objects) {
				fmt.Fprintf(a.stdout, "%s\n", key)
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&revision, "revision", 0, "read a specific revision")
	cmd.Flags().StringVar(&checkpoint, "checkpoint", "", "read a named checkpoint")
	return cmd
}
