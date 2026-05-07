package depusage

// Language identifies the source-language grammar to use. Each value
// corresponds to one tree-sitter grammar bundled by the library.
type Language string

const (
	JavaScript Language = "javascript"
	TypeScript Language = "typescript"
	Python     Language = "python"
	Go         Language = "go"
	Rust       Language = "rust"
	Ruby       Language = "ruby"
	Java       Language = "java"
	PHP        Language = "php"
	CSharp     Language = "csharp"
)

// ImportKind classifies how a module reference enters the file.
type ImportKind string

const (
	// ImportStatic is a top-level static import: ES `import`,
	// Python `from ... import`, Go `import "x"`, Java `import com.x.Y`.
	ImportStatic ImportKind = "static"

	// ImportDynamic is a runtime import expression: `import("x")`,
	// `__import__`, `Class.forName`. The argument may be a literal
	// (captured) or computed (skipped).
	ImportDynamic ImportKind = "dynamic"

	// ImportRequire is a function-style import: node `require("x")`,
	// Ruby `require "x"`, PHP `require_once`.
	ImportRequire ImportKind = "require"

	// ImportRelative covers same-project paths (`./x`, `../x`). The
	// raw Module is preserved; DepKey is empty.
	ImportRelative ImportKind = "relative"
)

// Import is a single reference to an external (or relative) module.
//
// Module is the verbatim string from the source. DepKey is the
// per-ecosystem normalization to a lockfile key (e.g. npm scoped
// `@scope/pkg/sub` → `@scope/pkg`); empty when normalization can't
// resolve to a single key (e.g. relative paths, computed imports).
type Import struct {
	Module  string
	DepKey  string
	Symbols []string          // imported binding names; ["*"] wildcard, ["default"] default
	Aliases map[string]string // local-name → canonical-symbol within Module
	Kind    ImportKind
	Line    int
	Column  int
}

// UsedSymbol records a call/access to an imported binding within the
// file. Module/DepKey come from the import that introduced the binding.
type UsedSymbol struct {
	Module string
	DepKey string
	Symbol string
	Line   int
	Column int
}

// Function is a function-or-method definition discovered in the file.
type Function struct {
	Name     string
	Exported bool // visible outside the file per language rules
	StartLn  int
	EndLn    int
}

// CallGraph is the per-file caller→callees graph. Edges keys are
// Function.Name values; entries point at other Function names within
// the same file. Calls to non-local symbols are excluded.
type CallGraph struct {
	Funcs []Function
	Edges map[string][]string
}

// Result aggregates one extraction pass.
//
// ParseError is non-nil when tree-sitter recovered from a syntax
// error; the surfaced facts may be partial but are not garbage.
type Result struct {
	Imports     []Import
	UsedSymbols []UsedSymbol
	CallGraph   *CallGraph
	ParseError  error
}

// Options selects which passes to run. Each pass gates downstream
// passes — UsedSymbols depends on Imports, CallGraph is independent.
type Options struct {
	IncludeImports   bool
	IncludeSymbols   bool
	IncludeCallGraph bool
}
