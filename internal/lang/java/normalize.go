package java

// DepKey is intentionally empty for Java. Java imports are FQCNs
// (`com.foo.bar.Baz`); mapping them to Maven group:artifact requires
// a per-package table the library doesn't ship. Consumers (aegis,
// SBOM tools) match Module against lockfile-side metadata.
//
// We expose this stub so the API shape stays uniform — and so a
// future version can grow a heuristic without breaking callers.
func DepKey(_ string) string {
	return ""
}
