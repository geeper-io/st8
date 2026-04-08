package local

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/geeper-io/st8/internal/model"
)

type Engine struct {
	path string
}

func New(stateDir string) *Engine {
	return &Engine{path: filepath.Join(stateDir, "store.json")}
}

func (e *Engine) Load(_ context.Context) (*model.Database, error) {
	data, err := os.ReadFile(e.path)
	if errors.Is(err, os.ErrNotExist) {
		return model.NewDatabase(), nil
	}
	if err != nil {
		return nil, err
	}
	var db model.Database
	if err := json.Unmarshal(data, &db); err != nil {
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

func (e *Engine) Close() error { return nil }

func (e *Engine) Ping(_ context.Context) error { return nil }

func (e *Engine) Save(_ context.Context, db *model.Database) error {
	if err := os.MkdirAll(filepath.Dir(e.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(e.path, data, 0o644)
}
