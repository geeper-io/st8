package main

import (
	"flag"
	"net/http"

	"github.com/geeper-io/st8/internal/engine/t4kv"
	"github.com/geeper-io/st8/internal/logging"
	"github.com/geeper-io/st8/internal/server"
	"github.com/geeper-io/st8/internal/service"
)

func main() {
	listen := flag.String("listen", ":8748", "listen address")
	stateDir := flag.String("state-dir", ".st8d", "directory for server state")
	flag.Parse()

	appLogger := logging.Logger()
	svc := service.New(t4kv.New(*stateDir, t4kv.Config{
		Logger: appLogger,
	}))
	appLogger.Infof("st8d listening on %s", *listen)
	if err := http.ListenAndServe(*listen, server.NewHTTP(svc)); err != nil {
		appLogger.Fatal(err)
	}
}
