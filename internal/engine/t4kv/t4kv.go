package t4kv

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/t4db/t4"

	"github.com/geeper-io/st8/internal/model"
)

const storeKey = "/__st8/store"

type Config struct {
	Logger            *logrus.Logger
	MetricsAddr       string
	MetricsRegisterer prometheus.Registerer
}

type Engine struct {
	node *t4.Node
}

func New(stateDir string, cfg Config) (*Engine, error) {
	dataDir := filepath.Join(stateDir, "engine")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	node, err := t4.Open(t4.Config{
		DataDir:           dataDir,
		Logger:            cfg.Logger,
		MetricsAddr:       cfg.MetricsAddr,
		MetricsRegisterer: cfg.MetricsRegisterer,
	})
	if err != nil {
		return nil, err
	}
	return &Engine{node: node}, nil
}

func (e *Engine) Close() error {
	return e.node.Close()
}

func (e *Engine) Load(_ context.Context) (*model.Database, error) {
	kv, err := e.node.Get(storeKey)
	if err != nil {
		return nil, err
	}
	if kv == nil || len(kv.Value) == 0 {
		return model.NewDatabase(), nil
	}

	var db model.Database
	if err := json.Unmarshal(kv.Value, &db); err != nil {
		return nil, err
	}
	if db.Revisions == nil {
		db.Revisions = map[int64]*model.Revision{}
	}
	if db.Namespaces == nil {
		db.Namespaces = map[string]*model.NamespaceState{}
	}
	if db.NextRevision == 0 {
		db.NextRevision = 1
	}
	return &db, nil
}

func (e *Engine) Save(ctx context.Context, db *model.Database) error {
	data, err := json.Marshal(db)
	if err != nil {
		return err
	}
	_, err = e.node.Put(ctx, storeKey, data, 0)
	return err
}
