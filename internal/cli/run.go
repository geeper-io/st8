package cli

import (
	"context"
	"errors"
	"io"
	"strings"

	st8client "github.com/geeper-io/st8/client"
	"github.com/geeper-io/st8/internal/config"
	"github.com/geeper-io/st8/internal/logging"
	"github.com/sirupsen/logrus"
)

type options struct {
	configPath  string
	stateDir    string
	serverURL   string
	token       string
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
	// Determine config path before full flag parsing so its values can seed
	// flag defaults. We pre-scan args for --config / --config=<path>.
	cfgPath, err := config.DefaultPath()
	if err != nil {
		cfgPath = ""
	}
	cfgPath = extractFlag(args, "config", cfgPath)

	cfg := &config.Config{}
	if cfgPath != "" {
		if loaded, err := config.Load(cfgPath); err == nil {
			cfg = loaded
		}
	}

	app := &App{
		opts: options{
			configPath: cfgPath,
			stateDir:   config.Or(cfg.Defaults.StateDir, config.DefaultStateDir()),
			serverURL:  cfg.Server.URL,
			token:      cfg.Server.Token,
			scope: st8client.Scope{
				Workspace:   config.Or(cfg.Defaults.Workspace, "default"),
				Environment: config.Or(cfg.Defaults.Environment, "dev"),
				Branch:      config.Or(cfg.Defaults.Branch, "main"),
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

// extractFlag scans args for --name <value> or --name=<value> and returns the
// value if found, otherwise defaultVal.
func extractFlag(args []string, name, defaultVal string) string {
	prefix := "--" + name + "="
	flag := "--" + name
	for i, arg := range args {
		if strings.HasPrefix(arg, prefix) {
			return strings.TrimPrefix(arg, prefix)
		}
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return defaultVal
}
