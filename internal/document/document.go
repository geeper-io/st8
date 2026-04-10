package document

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Item struct {
	Key     string
	Content string
}

// Load reads files/dirs from paths and merges literal key=value pairs.
// Literal pairs take the form "key=value"; everything else is treated as a file path.
func Load(paths []string) ([]Item, error) {
	return LoadWithValues(paths, nil)
}

// LoadWithValues reads files from paths and appends literal key=value items.
func LoadWithValues(paths []string, values []string) ([]Item, error) {
	out := make([]Item, 0, len(paths)+len(values))
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
	for _, kv := range values {
		k, v, ok := strings.Cut(kv, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("--value %q: expected key=value", kv)
		}
		out = append(out, Item{Key: k, Content: v})
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
