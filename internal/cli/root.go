package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) rootCommand(ctx context.Context) *cobra.Command {
	root := &cobra.Command{
		Use:           "st8",
		Short:         "Safely apply, track, and roll back config state",
		Long:          "st8 safely applies, tracks, diffs, checkpoints, rolls back, and branches config state.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&a.opts.stateDir, "state-dir", ".st8", "directory for local st8 metadata")
	root.PersistentFlags().StringVar(&a.opts.serverURL, "server", "", "remote st8d base URL")
	root.PersistentFlags().BoolVar(&a.opts.localServer, "local", false, "start a temporary local st8d and use the HTTP client path")
	root.PersistentFlags().StringVar(&a.opts.scope.Workspace, "workspace", "default", "workspace name")
	root.PersistentFlags().StringVar(&a.opts.scope.Environment, "env", "dev", "environment name")
	root.PersistentFlags().StringVar(&a.opts.scope.Branch, "branch", "main", "branch name")

	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_ = ctx
		fmt.Fprintln(a.stdout, "st8 safely applies, tracks, diffs, checkpoints, rolls back, and branches config state.")
		fmt.Fprintln(a.stdout)
		fmt.Fprintln(a.stdout, "Use --server http://host:8748 to talk to st8d, --local to boot a temporary local server, or omit both for direct embedded mode.")
		fmt.Fprintln(a.stdout)
		_ = cmd.Usage()
	})

	root.AddCommand(
		a.applyCommand(),
		a.getCommand(),
		a.diffCommand(),
		a.checkpointCommand(),
		a.rollbackCommand(),
		a.logCommand(),
		a.branchCommand(),
		a.restoreCommand(),
	)

	return root
}
