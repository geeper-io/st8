package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/geeper-io/st8/internal/auth"
	"github.com/geeper-io/st8/internal/engine/t4kv"
	"github.com/geeper-io/st8/internal/logging"
	"github.com/geeper-io/st8/internal/server"
	"github.com/geeper-io/st8/internal/service"
	"github.com/geeper-io/st8/internal/st8metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func main() {
	listen := flag.String("listen", ":8748", "listen address")
	stateDir := flag.String("state-dir", ".st8d", "directory for server state")
	authConfig := flag.String("auth-config", "", "path to YAML auth config for per-token RBAC policies")
	readTimeout := flag.Duration("read-timeout", 30*time.Second, "HTTP read timeout")
	writeTimeout := flag.Duration("write-timeout", 60*time.Second, "HTTP write timeout")
	idleTimeout := flag.Duration("idle-timeout", 120*time.Second, "HTTP idle timeout")
	tlsCert := flag.String("tls-cert", "", "path to TLS certificate file (PEM); enables HTTPS when set together with --tls-key")
	tlsKey := flag.String("tls-key", "", "path to TLS private key file (PEM); enables HTTPS when set together with --tls-cert")
	flag.Parse()

	// ST8D_TOKEN grants full admin access. When --auth-config is also set,
	// the token is prepended as an implicit admin entry in the loaded config.
	adminToken := os.Getenv("ST8D_TOKEN")

	appLogger := logging.Logger()
	metricsRegistry := prometheus.DefaultRegisterer
	metricsCollector := st8metrics.New(metricsRegistry)

	eng, err := t4kv.New(*stateDir, t4kv.Config{
		Logger:            appLogger,
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

	var handler http.Handler = server.NewHTTP(svc, metricsCollector, prometheus.DefaultGatherer, appLogger)

	switch {
	case *authConfig != "":
		cfg, err := auth.Load(*authConfig)
		if err != nil {
			appLogger.Fatalf("failed to load auth config: %v", err)
		}
		// Prepend ST8D_TOKEN as an implicit admin entry when both are set.
		if adminToken != "" {
			cfg.Tokens = append([]auth.TokenEntry{{
				Token: adminToken,
				Name:  "admin",
				Allow: auth.Policy{
					Namespaces: []string{"*"},
					Branches:   []string{"*"},
					Verbs:      []string{auth.VerbRead, auth.VerbWrite, auth.VerbAdmin},
				},
			}}, cfg.Tokens...)
		}
		handler = auth.Middleware(cfg, handler)
		appLogger.Infof("st8d RBAC auth enabled (%d token(s) loaded)", len(cfg.Tokens))

	case adminToken != "":
		handler = auth.Middleware(auth.AdminConfig(adminToken), handler)
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
	if *tlsCert != "" && *tlsKey != "" {
		appLogger.Info("TLS enabled")
		if err := srv.ListenAndServeTLS(*tlsCert, *tlsKey); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Fatal(err)
		}
	} else {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			appLogger.Fatal(err)
		}
	}
}


