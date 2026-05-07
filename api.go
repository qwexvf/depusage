package depusage

import "github.com/qwexvf/depusage/internal/extract"

// Public types are aliases of the shared definitions in
// internal/extract. Aliases (rather than wrapper types) keep the
// underlying type identity, so values constructed by per-language
// sub-packages flow back through the public API without conversion.

type (
	Language   = extract.Language
	ImportKind = extract.ImportKind
	Import     = extract.Import
	UsedSymbol = extract.UsedSymbol
	Function   = extract.Function
	CallGraph  = extract.CallGraph
	Result     = extract.Result
	Options    = extract.Options
)

const (
	JavaScript = extract.JavaScript
	TypeScript = extract.TypeScript
	Python     = extract.Python
	Go         = extract.Go
	Rust       = extract.Rust
	Ruby       = extract.Ruby
	Java       = extract.Java
	PHP        = extract.PHP
	CSharp     = extract.CSharp
)

const (
	ImportStatic   = extract.ImportStatic
	ImportDynamic  = extract.ImportDynamic
	ImportRequire  = extract.ImportRequire
	ImportRelative = extract.ImportRelative
)
