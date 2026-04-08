package cli

import (
	"github.com/geeper-io/st8/internal/document"
	"github.com/geeper-io/st8/internal/service"
)

func loadDocuments(paths []string) ([]service.Document, error) {
	items, err := document.Load(paths)
	if err != nil {
		return nil, err
	}
	out := make([]service.Document, 0, len(items))
	for _, item := range items {
		out = append(out, service.Document{
			Key:     item.Key,
			Content: item.Content,
		})
	}
	return out, nil
}
