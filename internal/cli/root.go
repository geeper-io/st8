package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

func (a *App) rootCommand(ctx context.Context) *cobra.Command {
	root := &cobra.Command{
		Use:           "st8ctl",
		Short:         "Safely apply, track, and roll back config state",
		Long:          "st8ctl safely applies, tracks, diffs, checkpoints, rolls back, and branches config state.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Config and auth flags — defaults come from the loaded config file.
	root.PersistentFlags().StringVar(&a.opts.configPath, "config", a.opts.configPath, "path to config file")
	root.PersistentFlags().StringVar(&a.opts.token, "token", a.opts.token, "bearer token for st8d authentication")

	// Connection flags.
	root.PersistentFlags().StringVar(&a.opts.serverURL, "server", a.opts.serverURL, "remote st8d base URL")
	root.PersistentFlags().BoolVar(&a.opts.localServer, "local", false, "start a temporary local st8d and use the HTTP client path")
	root.PersistentFlags().StringVar(&a.opts.stateDir, "state-dir", a.opts.stateDir, "directory for local st8 metadata")

	// Scope flags — defaults from config, or built-in fallbacks.
	root.PersistentFlags().StringVar(&a.opts.scope.Workspace, "workspace", a.opts.scope.Workspace, "workspace name")
	root.PersistentFlags().StringVar(&a.opts.scope.Environment, "env", a.opts.scope.Environment, "environment name")
	root.PersistentFlags().StringVar(&a.opts.scope.Branch, "branch", a.opts.scope.Branch, "branch name")

	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_ = ctx
		fmt.Fprintln(a.stdout, "st8ctl safely applies, tracks, diffs, checkpoints, rolls back, and branches config state.")
		fmt.Fprintln(a.stdout)
		fmt.Fprintln(a.stdout, "Use --server http://host:8748 to connect to st8d, or --local to start a temporary local instance for demos and testing.")
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
