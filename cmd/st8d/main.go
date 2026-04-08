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
	token := flag.String("token", "", "require this bearer token on all requests (disabled if empty)")
	flag.Parse()

	appLogger := logging.Logger()
	metricsRegistry := prometheus.DefaultRegisterer
	metricsCollector := st8metrics.New(metricsRegistry)

	svc := service.New(t4kv.New(*stateDir, t4kv.Config{
		Logger:            appLogger,
		MetricsAddr:       *metricsListen,
		MetricsRegisterer: metricsRegistry,
	}))

	var handler http.Handler = server.NewHTTP(svc, metricsCollector)
	if *token != "" {
		handler = bearerAuth(*token, handler)
		appLogger.Info("st8d bearer token authentication enabled")
	}

	srv := &http.Server{
		Addr:    *listen,
		Handler: handler,
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

// bearerAuth is middleware that requires "Authorization: Bearer <token>" on
// all requests except /healthz.
func bearerAuth(token string, next http.Handler) http.Handler {
	want := "Bearer " + token
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Authorization") != want {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
