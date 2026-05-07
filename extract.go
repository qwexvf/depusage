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
	case Python, Go, Rust, Ruby, Java, PHP, CSharp:
		return Result{}, fmt.Errorf("depusage: language %q not yet implemented", lang)
	}
	return Result{}, fmt.Errorf("depusage: unknown language %q", lang)
}
