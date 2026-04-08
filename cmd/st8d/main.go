package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os/signal"
	"syscall"
	"time"

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
	readTimeout := flag.Duration("read-timeout", 30*time.Second, "HTTP read timeout")
	writeTimeout := flag.Duration("write-timeout", 60*time.Second, "HTTP write timeout")
	idleTimeout := flag.Duration("idle-timeout", 120*time.Second, "HTTP idle timeout")
	flag.Parse()

	appLogger := logging.Logger()
	metricsRegistry := prometheus.DefaultRegisterer
	metricsCollector := st8metrics.New(metricsRegistry)

	eng, err := t4kv.New(*stateDir, t4kv.Config{
		Logger:            appLogger,
		MetricsAddr:       *metricsListen,
		MetricsRegisterer: metricsRegistry,
	})
	if err != nil {
		appLogger.Fatalf("failed to open storage engine: %v", err)
	}
	defer func() {
		if err := eng.Close(); err != nil {
			appLogger.Errorf("failed to close storage engine: %v", err)
		}
	}()

	svc := service.New(eng)

	var handler http.Handler = server.NewHTTP(svc, metricsCollector)
	if *token != "" {
		handler = bearerAuth(*token, handler)
		appLogger.Info("st8d bearer token authentication enabled")
	}

	srv := &http.Server{
		Addr:         *listen,
		Handler:      handler,
		ReadTimeout:  *readTimeout,
		WriteTimeout: *writeTimeout,
		IdleTimeout:  *idleTimeout,
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
