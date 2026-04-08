package cli

import (
	"context"
	"errors"
	"strings"

	"github.com/geeper-io/st8/internal/client"
	"github.com/geeper-io/st8/internal/engine/t4kv"
	"github.com/geeper-io/st8/internal/server"
	"github.com/geeper-io/st8/internal/service"
)

func (a *App) chooseClient(ctx context.Context) (client.Client, func(), error) {
	if a.opts.localServer && strings.TrimSpace(a.opts.serverURL) != "" {
		return nil, nil, errors.New("--local and --server cannot be used together")
	}
	if strings.TrimSpace(a.opts.serverURL) != "" {
		return client.NewHTTP(a.opts.serverURL), func() {}, nil
	}
	if a.opts.localServer {
		instance, err := server.StartLocal(ctx, a.opts.stateDir)
		if err != nil {
			return nil, nil, err
		}
		return client.NewHTTP(instance.BaseURL), func() {
			_ = instance.Close()
		}, nil
	}
	return client.NewLocal(service.New(t4kv.New(a.opts.stateDir))), func() {}, nil
}
