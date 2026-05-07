package rs

// DepKey returns the crate name for a Rust import path.
//
//	"serde"          -> "serde"
//	"serde::Deserialize" already gets parsed down to "serde" upstream
//	"crate::foo"     -> ""    (own crate)
//	"super::foo"     -> ""    (relative)
//	"self::foo"      -> ""    (relative)
//	"std::io"        -> ""    (stdlib)
//	"core::mem"      -> ""    (stdlib)
//	"alloc::vec"     -> ""    (stdlib)
//
// Cargo crate names use dashes; Rust import paths use underscores
// (cargo's --bin auto-renames). depusage stores the underscored form
// it sees in source. Consumers that want to compare against
// Cargo.toml dep names can s/_/-/g.
func DepKey(crate string) string {
	if crate == "" {
		return ""
	}
	switch crate {
	case "crate", "self", "super":
		return ""
	case "std", "core", "alloc", "test":
		return ""
	}
	return crate
}

// IsRelative reports whether the leading-path-segment refers to
// project-local code rather than an external crate.
func IsRelative(crate string) bool {
	switch crate {
	case "crate", "self", "super":
		return true
	}
	return false
}
