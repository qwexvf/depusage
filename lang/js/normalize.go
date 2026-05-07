package js

import "strings"

// DepKey normalizes a raw npm module string to a lockfile-key form.
//
//	"lodash"           -> "lodash"
//	"lodash/merge"     -> "lodash"
//	"@scope/pkg"       -> "@scope/pkg"
//	"@scope/pkg/sub"   -> "@scope/pkg"
//	"./foo"            -> ""    (relative)
//	"../foo"           -> ""    (relative)
//	"/abs/path"        -> ""    (absolute)
//	"node:fs"          -> ""    (node builtin)
//	""                 -> ""
//
// Built-in modules (node:fs, node:path, etc.) and bare names that match
// the historic builtin list (fs, path, http, ...) are NOT stripped here
// — that's the consumer's job. depusage only refuses the patterns that
// can never be a registry dep: relative, absolute, and node: scheme.
func DepKey(raw string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") ||
		raw == "." || raw == ".." {
		return ""
	}
	if strings.HasPrefix(raw, "/") {
		return ""
	}
	if strings.HasPrefix(raw, "node:") {
		return ""
	}
	if strings.HasPrefix(raw, "@") {
		// Scoped: keep first two segments.
		parts := strings.SplitN(raw, "/", 3)
		if len(parts) < 2 {
			// "@something" with no slash — malformed but return as-is.
			return raw
		}
		return parts[0] + "/" + parts[1]
	}
	// Unscoped: keep first segment.
	if i := strings.Index(raw, "/"); i > 0 {
		return raw[:i]
	}
	return raw
}

// IsRelative reports whether the raw module string is a relative or
// absolute filesystem path. Used by the extractor to flag the
// [ImportRelative] kind.
func IsRelative(raw string) bool {
	return strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") ||
		raw == "." || raw == ".." || strings.HasPrefix(raw, "/")
}
