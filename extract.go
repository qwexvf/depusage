package depusage

import "fmt"

// Extract is the top-level dispatcher. It picks the per-language
// extractor implementation, runs the requested passes, and returns
// the aggregated result.
//
// All language extractors live in this package as siblings (js.go,
// py.go, ...) and share the parser pool / cursor pool helpers from
// internal/tsutil. Per-language tree-sitter queries live under
// lang/<x>/queries.scm and are //go:embed'd by the matching .go file.
//
// Concurrency: safe for concurrent callers. Each per-language
// extractor maintains its own parser/cursor pool.
func Extract(lang Language, body []byte, opts Options) (Result, error) {
	switch lang {
	case JavaScript:
		return jsExtract(body, opts)
	case TypeScript:
		// TS shares the JS extractor for now (P0 scope: imports only).
		// Phase 1 will swap in a TS-specific grammar to handle
		// `import type` and `export = X` shapes.
		return jsExtract(body, opts)
	case Python:
		return pyExtract(body, opts)
	case Go:
		return goExtract(body, opts)
	case Rust:
		return rsExtract(body, opts)
	case Ruby:
		return rbExtract(body, opts)
	case Java:
		return jvExtract(body, opts)
	case PHP:
		return phpExtract(body, opts)
	case CSharp:
		return csExtract(body, opts)
	}
	return Result{}, fmt.Errorf("depusage: unknown language %q", lang)
}
