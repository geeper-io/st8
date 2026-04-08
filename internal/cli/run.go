package cli

import (
	"context"
	"errors"
	"io"

	st8client "github.com/geeper-io/st8/client"
	"github.com/geeper-io/st8/internal/logging"
	"github.com/sirupsen/logrus"
)

type options struct {
	stateDir    string
	serverURL   string
	localServer bool
	scope       st8client.Scope
}

type App struct {
	opts   options
	stdout io.Writer
	stderr io.Writer
	logger *logrus.Logger
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	app := &App{
		opts: options{
			stateDir: ".st8",
			scope: st8client.Scope{
				Workspace:   "default",
				Environment: "dev",
				Branch:      "main",
			},
		},
		stdout: stdout,
		stderr: stderr,
		logger: logging.Logger(),
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
