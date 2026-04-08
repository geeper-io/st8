package cli

import (
	"context"
	"errors"
	"strings"

	st8client "github.com/geeper-io/st8/client"
	"github.com/geeper-io/st8/internal/server"
)

func (a *App) chooseClient(ctx context.Context) (st8client.Client, func(), error) {
	if a.opts.localServer && strings.TrimSpace(a.opts.serverURL) != "" {
		return nil, nil, errors.New("--local and --server cannot be used together")
	}
	if strings.TrimSpace(a.opts.serverURL) != "" {
		return st8client.NewHTTP(a.opts.serverURL, st8client.WithToken(a.opts.token)), func() {}, nil
	}
	if a.opts.localServer {
		instance, err := server.StartLocal(ctx, a.opts.stateDir)
		if err != nil {
			return nil, nil, err
		}
		return st8client.NewHTTP(instance.BaseURL, st8client.WithToken(a.opts.token)), func() {
			_ = instance.Close()
		}, nil
	}
	return nil, nil, errors.New("no st8d server configured; use --server <url>, set server.url in ~/.config/st8ctl/config.yaml, or use --local for a temporary local instance")
}
