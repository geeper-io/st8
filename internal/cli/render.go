package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/geeper-io/st8/internal/model"
)

func renderChange(w io.Writer, change model.Change) {
	fmt.Fprintf(w, "\n%s %s\n", strings.ToUpper(change.Type), change.Key)
	if change.Type != "create" {
		fmt.Fprintln(w, "--- before")
		fmt.Fprint(w, change.Before)
	}
	if change.Type != "delete" {
		fmt.Fprintln(w, "+++ after")
		fmt.Fprint(w, change.After)
	}
}

func sortedObjectKeys(objects map[string]string) []string {
	keys := make([]string, 0, len(objects))
	for key := range objects {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
