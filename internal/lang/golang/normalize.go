package golang

import "strings"

// DepKey returns the import path as-is — Go modules are addressed by
// their full import path, not a prefix. Resolving "this path belongs
// to module M at version V" requires reading go.mod / go.sum, which
// is the consumer's job. depusage just preserves the raw string.
//
// Standard-library packages (no domain prefix) and the placeholder
// names "C" (cgo) and the empty string return "" — they're never a
// registry dep.
func DepKey(raw string) string {
	if raw == "" || raw == "C" {
		return ""
	}
	// Std-library heuristic: import paths with no "." in the first
	// segment are stdlib (e.g. "fmt", "encoding/json").
	if i := strings.Index(raw, "/"); i > 0 {
		if !strings.Contains(raw[:i], ".") {
			return ""
		}
	} else if !strings.Contains(raw, ".") {
		return ""
	}
	return raw
}
