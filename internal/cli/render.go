package cli

import (
	"fmt"
	"io"
	"slices"
	"strings"

	st8client "github.com/geeper-io/st8/client"
	"github.com/pmezard/go-difflib/difflib"
)

func renderChange(w io.Writer, change st8client.Change) {
	fmt.Fprintf(w, "\n")

	before, after := ensureNewline(change.Before), ensureNewline(change.After)

	var fromFile, toFile string
	switch change.Type {
	case "create":
		fromFile = "/dev/null"
		toFile = change.Key
	case "delete":
		fromFile = change.Key
		toFile = "/dev/null"
	default:
		fromFile = change.Key
		toFile = change.Key
	}

	ud := difflib.UnifiedDiff{
		A:        difflib.SplitLines(before),
		B:        difflib.SplitLines(after),
		FromFile: fromFile,
		ToFile:   toFile,
		Context:  3,
	}
	text, err := difflib.GetUnifiedDiffString(ud)
	if err != nil || text == "" {
		// Fallback for binary or unparseable content
		fmt.Fprintf(w, "--- %s\n+++ %s\n", fromFile, toFile)
		if change.Type != "create" {
			for _, line := range strings.SplitAfter(before, "\n") {
				fmt.Fprintf(w, "-%s", line)
			}
		}
		if change.Type != "delete" {
			for _, line := range strings.SplitAfter(after, "\n") {
				fmt.Fprintf(w, "+%s", line)
			}
		}
		return
	}
	fmt.Fprint(w, text)
}

// ensureNewline guarantees the string ends with a newline so the diff
// algorithm produces clean output for content that lacks a trailing newline.
func ensureNewline(s string) string {
	if s != "" && !strings.HasSuffix(s, "\n") {
		return s + "\n"
	}
	return s
}

func sortedObjectKeys(objects map[string]string) []string {
	keys := make([]string, 0, len(objects))
	for key := range objects {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
