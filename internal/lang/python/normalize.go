package python

import "strings"

// DepKey normalizes a Python module path to its top-level package
// name (the PyPI dep key, in most cases).
//
//	"requests"            -> "requests"
//	"requests.adapters"   -> "requests"
//	"flask.helpers"       -> "flask"
//	".local"              -> ""    (relative)
//	"..pkg.x"             -> ""    (relative)
//	""                    -> ""
//
// Limitation: namespace packages where the top-level differs from the
// PyPI distribution (e.g. `google.cloud.storage` from
// `google-cloud-storage`) need consumer-side mapping. depusage emits
// the dotted top-level only.
func DepKey(raw string) string {
	if raw == "" {
		return ""
	}
	if IsRelative(raw) {
		return ""
	}
	if i := strings.Index(raw, "."); i > 0 {
		return raw[:i]
	}
	return raw
}

// IsRelative reports whether the module path uses leading-dot relative
// notation (e.g. `.x`, `..y.z`, `.`).
func IsRelative(raw string) bool {
	return strings.HasPrefix(raw, ".")
}
