package cli

import (
	"context"
	"errors"
	"io"

	"github.com/geeper-io/st8/internal/service"
)

type options struct {
	stateDir    string
	serverURL   string
	localServer bool
	scope       service.Scope
}

type App struct {
	opts   options
	stdout io.Writer
	stderr io.Writer
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	app := &App{
		opts: options{
			stateDir: ".st8",
			scope: service.Scope{
				Workspace:   "default",
				Environment: "dev",
				Branch:      "main",
			},
		},
		stdout: stdout,
		stderr: stderr,
	}

	root := app.rootCommand(ctx)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return 1, err
		}
		return 1, err
	}
	return 0, nil
}
