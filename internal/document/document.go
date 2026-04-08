package document

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Item struct {
	Key     string
	Content string
}

func Load(paths []string) ([]Item, error) {
	out := make([]Item, 0, len(paths))
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out = append(out, Item{
			Key:     NormalizeKey(path),
			Content: NormalizeContent(path, raw),
		})
	}
	return out, nil
}

func NormalizeKey(path string) string {
	clean := filepath.ToSlash(filepath.Clean(path))
	return strings.TrimPrefix(clean, "./")
}

func NormalizeContent(path string, raw []byte) string {
	trimmed := bytes.TrimSpace(raw)
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" && len(trimmed) > 0 && json.Valid(trimmed) {
		var value any
		if err := json.Unmarshal(trimmed, &value); err == nil {
			if formatted, err := json.MarshalIndent(value, "", "  "); err == nil {
				return string(formatted) + "\n"
			}
		}
	}
	if len(trimmed) == 0 {
		return ""
	}
	return string(trimmed) + "\n"
}
