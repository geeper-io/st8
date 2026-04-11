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
	s3Bucket := flag.String("s3-bucket", "", "S3 bucket for WAL archive and checkpoints (optional; enables durable storage) (env: ST8D_S3_BUCKET)")
	s3Prefix := flag.String("s3-prefix", "", "key prefix inside the S3 bucket (env: ST8D_S3_PREFIX)")
	s3Endpoint := flag.String("s3-endpoint", "", "custom S3 endpoint URL for MinIO or other S3-compatible stores, e.g. http://minio:9000 (env: ST8D_S3_ENDPOINT)")
	s3Region := flag.String("s3-region", "", "AWS region (env: ST8D_S3_REGION)")
	s3Profile := flag.String("s3-profile", "", "AWS shared config profile (env: ST8D_S3_PROFILE)")
	s3AccessKeyID := flag.String("s3-access-key-id", "", "AWS access key ID for static credentials (env: ST8D_S3_ACCESS_KEY_ID)")
	s3SecretAccessKey := flag.String("s3-secret-access-key", "", "AWS secret access key for static credentials (env: ST8D_S3_SECRET_ACCESS_KEY)")
	authConfig := flag.String("auth-config", "", "path to YAML auth config for per-token RBAC policies")
	readTimeout := flag.Duration("read-timeout", 30*time.Second, "HTTP read timeout")
	writeTimeout := flag.Duration("write-timeout", 60*time.Second, "HTTP write timeout")
	idleTimeout := flag.Duration("idle-timeout", 120*time.Second, "HTTP idle timeout")
	tlsCert := flag.String("tls-cert", "", "path to TLS certificate file (PEM); enables HTTPS when set together with --tls-key")
	tlsKey := flag.String("tls-key", "", "path to TLS private key file (PEM); enables HTTPS when set together with --tls-cert")
	flag.Parse()

	// Apply env var fallbacks for S3 flags (flags take precedence over env vars).
	applyEnvFlag(s3Bucket, "ST8D_S3_BUCKET")
	applyEnvFlag(s3Prefix, "ST8D_S3_PREFIX")
	applyEnvFlag(s3Endpoint, "ST8D_S3_ENDPOINT")
	applyEnvFlag(s3Region, "ST8D_S3_REGION")
	applyEnvFlag(s3Profile, "ST8D_S3_PROFILE")
	applyEnvFlag(s3AccessKeyID, "ST8D_S3_ACCESS_KEY_ID")
	applyEnvFlag(s3SecretAccessKey, "ST8D_S3_SECRET_ACCESS_KEY")

	// ST8D_TOKEN grants full admin access. When --auth-config is also set,
	// the token is prepended as an implicit admin entry in the loaded config.
	adminToken := os.Getenv("ST8D_TOKEN")

	appLogger := logging.Logger()
	metricsRegistry := prometheus.DefaultRegisterer
	metricsCollector := st8metrics.New(metricsRegistry)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	eng, err := t4kv.New(ctx, *stateDir, t4kv.Config{
		Logger:            appLogger,
		MetricsRegisterer: metricsRegistry,
		S3Bucket:          *s3Bucket,
		S3Prefix:          *s3Prefix,
		S3Endpoint:        *s3Endpoint,
		S3Region:          *s3Region,
		S3Profile:         *s3Profile,
		S3AccessKeyID:     *s3AccessKeyID,
		S3SecretAccessKey: *s3SecretAccessKey,
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

	var handler = server.NewHTTP(svc, metricsCollector, prometheus.DefaultGatherer, appLogger)

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

// applyEnvFlag sets *p from the environment variable env when *p is empty.
func applyEnvFlag(p *string, env string) {
	if *p == "" {
		*p = os.Getenv(env)
	}
}
