package tsutil

import (
	"fmt"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// MustCompileQuery compiles a tree-sitter query at package init time
// and panics on failure. The query string is embedded at build time, so
// a compile error is a developer bug, not a runtime condition — panic
// is the right escalation.
func MustCompileQuery(lang *ts.Language, source, name string) *ts.Query {
	q, err := ts.NewQuery(lang, source)
	if err != nil {
		panic(fmt.Sprintf("depusage: compile %s query: %v", name, err))
	}
	return q
}

// CaptureIndex returns the index of a named capture in q, or -1 if
// the name is not present. Used at init time to pre-resolve capture
// names so the hot match-iteration loop can index by uint instead of
// string.
func CaptureIndex(q *ts.Query, name string) int {
	for i, n := range q.CaptureNames() {
		if n == name {
			return i
		}
	}
	return -1
}
