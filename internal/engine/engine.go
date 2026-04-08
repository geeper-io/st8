package engine

import (
	"context"

	"github.com/geeper-io/st8/internal/model"
)

type Engine interface {
	Load(context.Context) (*model.Database, error)
	Save(context.Context, *model.Database) error
}
