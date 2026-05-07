package depusage

import (
	"fmt"

	"github.com/qwexvf/depusage/internal/lang/csharp"
	"github.com/qwexvf/depusage/internal/lang/golang"
	"github.com/qwexvf/depusage/internal/lang/java"
	"github.com/qwexvf/depusage/internal/lang/javascript"
	"github.com/qwexvf/depusage/internal/lang/php"
	"github.com/qwexvf/depusage/internal/lang/python"
	"github.com/qwexvf/depusage/internal/lang/ruby"
	"github.com/qwexvf/depusage/internal/lang/rust"
)

// Extract is the top-level dispatcher. It picks the per-language
// extractor implementation, runs the requested passes, and returns
// the aggregated result.
//
// Per-language extractors live under internal/lang/<name>. Each one
// owns its own tree-sitter parser pool, query, and dep-key normalizer.
//
// Concurrency: safe for concurrent callers — every per-language
// extractor maintains its own pool.
func Extract(lang Language, body []byte, opts Options) (Result, error) {
	switch lang {
	case JavaScript, TypeScript:
		// TS shares the JS extractor for now (P0 scope: imports only).
		// A TS-specific grammar will land in Phase 1 to cover
		// `import type` and `export = X` shapes.
		return javascript.Extract(body, opts)
	case Python:
		return python.Extract(body, opts)
	case Go:
		return golang.Extract(body, opts)
	case Rust:
		return rust.Extract(body, opts)
	case Ruby:
		return ruby.Extract(body, opts)
	case Java:
		return java.Extract(body, opts)
	case PHP:
		return php.Extract(body, opts)
	case CSharp:
		return csharp.Extract(body, opts)
	}
	return Result{}, fmt.Errorf("depusage: unknown language %q", lang)
}
