package server

import (
	"context"
	"net"
	"net/http"

	"github.com/geeper-io/st8/internal/engine/t4kv"
	"github.com/geeper-io/st8/internal/logging"
	"github.com/geeper-io/st8/internal/service"
	"github.com/geeper-io/st8/internal/st8metrics"
	"github.com/prometheus/client_golang/prometheus"
)

type LocalInstance struct {
	BaseURL string
	closeFn func() error
}

func StartLocal(ctx context.Context, stateDir string) (*LocalInstance, error) {
	eng, err := t4kv.New(ctx, stateDir, t4kv.Config{Logger: logging.Logger()})
	if err != nil {
		return nil, err
	}
	svc := service.New(eng)
	handler := NewHTTP(svc, st8metrics.New(prometheus.NewRegistry()), nil, logging.Logger())

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		_ = eng.Close()
		return nil, err
	}

	srv := &http.Server{Handler: handler}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	go func() {
		_ = srv.Serve(listener)
	}()

	return &LocalInstance{
		BaseURL: "http://" + listener.Addr().String(),
		closeFn: func() error {
			err := srv.Shutdown(context.Background())
			_ = eng.Close()
			return err
		},
	}, nil
}

func (l *LocalInstance) Close() error {
	if l == nil || l.closeFn == nil {
		return nil
	}
	return l.closeFn()
}
