package cli

import (
	st8client "github.com/geeper-io/st8/client"
	"github.com/geeper-io/st8/internal/document"
)

func loadDocuments(paths []string) ([]st8client.Document, error) {
	return loadDocumentsWithValues(paths, nil)
}

func loadDocumentsWithValues(paths, values []string) ([]st8client.Document, error) {
	items, err := document.LoadWithValues(paths, values)
	if err != nil {
		return nil, err
	}
	out := make([]st8client.Document, len(items))
	for i, item := range items {
		out[i] = st8client.Document{Key: item.Key, Content: item.Content}
	}
	return out, nil
}
