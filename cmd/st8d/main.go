package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/geeper-io/st8/internal/engine/t4kv"
	"github.com/geeper-io/st8/internal/logging"
	"github.com/geeper-io/st8/internal/server"
	"github.com/geeper-io/st8/internal/service"
	"github.com/geeper-io/st8/internal/st8metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	listen := flag.String("listen", ":8748", "listen address")
	metricsListen := flag.String("metrics-listen", "", "listen address for embedded t4 metrics (/metrics, /healthz, /readyz)")
	stateDir := flag.String("state-dir", ".st8d", "directory for server state")
	flag.Parse()

	appLogger := logging.Logger()
	metricsRegistry := prometheus.DefaultRegisterer
	metricsCollector := st8metrics.New(metricsRegistry)

	svc := service.New(t4kv.New(*stateDir, t4kv.Config{
		Logger:            appLogger,
		MetricsAddr:       *metricsListen,
		MetricsRegisterer: metricsRegistry,
	}))

	srv := &http.Server{
		Addr:    *listen,
		Handler: server.NewHTTP(svc, metricsCollector),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	appLogger.Infof("st8d listening on %s", *listen)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		appLogger.Fatal(err)
	}
}
