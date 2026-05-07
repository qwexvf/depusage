package rb

import "strings"

// DepKey returns the gem name as it appears in a Gemfile / lockfile.
//
//	"rails"            -> "rails"
//	"active_support"   -> "active_support"
//	"./local"          -> ""    (relative)
//	"./helpers"        -> ""    (relative)
//
// Ruby's `require 'foo/bar'` form is common — `bar` is a sub-path
// inside the `foo` gem. depusage takes the first segment as the
// gem name.
func DepKey(raw string) string {
	if raw == "" {
		return ""
	}
	if IsRelative(raw) {
		return ""
	}
	if i := strings.Index(raw, "/"); i > 0 {
		return raw[:i]
	}
	return raw
}

// IsRelative reports whether the path looks like a project-local
// reference rather than a gem.
func IsRelative(raw string) bool {
	return strings.HasPrefix(raw, "./") || strings.HasPrefix(raw, "../") ||
		strings.HasPrefix(raw, "/")
}
