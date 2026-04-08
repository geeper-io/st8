package t4kv

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/t4db/t4"

	"github.com/geeper-io/st8/internal/model"
)

const storeKey = "/__st8/store"

type Engine struct {
	dataDir string
}

func New(stateDir string) *Engine {
	return &Engine{dataDir: filepath.Join(stateDir, "engine")}
}

func (e *Engine) Load(_ context.Context) (*model.Database, error) {
	node, err := t4.Open(t4.Config{DataDir: e.dataDir})
	if err != nil {
		return nil, err
	}
	defer node.Close()

	kv, err := node.Get(storeKey)
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
	if db.Workspaces == nil {
		db.Workspaces = map[string]*model.WorkspaceState{}
	}
	if db.NextRevision == 0 {
		db.NextRevision = 1
	}
	return &db, nil
}

func (e *Engine) Save(ctx context.Context, db *model.Database) error {
	if err := os.MkdirAll(e.dataDir, 0o755); err != nil {
		return err
	}
	node, err := t4.Open(t4.Config{DataDir: e.dataDir})
	if err != nil {
		return err
	}
	defer node.Close()

	data, err := json.Marshal(db)
	if err != nil {
		return err
	}
	_, err = node.Put(ctx, storeKey, data, 0)
	return err
}
