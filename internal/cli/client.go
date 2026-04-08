package cli

import (
	"context"
	"errors"
	"strings"

	st8client "github.com/geeper-io/st8/client"
	"github.com/geeper-io/st8/internal/config"
	"github.com/geeper-io/st8/internal/server"
)

func (a *App) chooseClient(ctx context.Context) (st8client.Client, func(), error) {
	url := strings.TrimSpace(a.opts.serverURL)
	local := a.opts.localServer || url == config.LocalURL

	if local && url != "" && url != config.LocalURL {
		return nil, nil, errors.New("--local and --server cannot be used together")
	}

	if local {
		instance, err := server.StartLocal(ctx, a.opts.stateDir)
		if err != nil {
			return nil, nil, err
		}
		return st8client.NewHTTP(instance.BaseURL, st8client.WithToken(a.opts.token)), func() {
			_ = instance.Close()
		}, nil
	}

	if url == "" {
		return nil, nil, errors.New("no st8d server configured; use --server <url>, set server.url in config, or use --local")
	}

	return st8client.NewHTTP(url, st8client.WithToken(a.opts.token)), func() {}, nil
}
