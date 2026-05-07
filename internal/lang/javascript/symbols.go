package javascript

import (
	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// binding records what an in-scope identifier refers back to: the
// module it was imported from and the canonical symbol name within
// that module.
//
// Example: `import { merge as m } from 'lodash'` introduces
// binding{Local:"m", Module:"lodash", DepKey:"lodash", Symbol:"merge"}.
//
// Namespace imports (`import * as ns`) use Symbol = "*" — the actual
// member name is read off the property accessor at the call site.
//
// Default imports use Symbol = "default" — same pattern; the actual
// member is the property at the call site.
type binding struct {
	Module string
	DepKey string
	Symbol string // canonical symbol or "*" / "default"
}

// buildBindings flattens an Import slice into a local-name → binding
// map. Side-effect imports (`import 'm'`) and Aliases-less default
// imports (where the local name *is* the binding) are both handled.
func buildBindings(imports []extract.Import) map[string]binding {
	out := map[string]binding{}
	for _, imp := range imports {
		if imp.DepKey == "" && imp.Module == "" {
			continue
		}
		// Aliases map local-name → canonical-symbol. For each alias,
		// the local name is what appears in the source.
		for local, canonical := range imp.Aliases {
			out[local] = binding{
				Module: imp.Module,
				DepKey: imp.DepKey,
				Symbol: canonical,
			}
		}
		// Symbols without aliases bind under their canonical name. We
		// rely on the import collector having appended each canonical
		// to imp.Symbols and only the *renamed* ones to imp.Aliases.
		for _, sym := range imp.Symbols {
			if sym == "*" || sym == "default" {
				// Wildcard / default need an explicit local binding;
				// already handled via Aliases above.
				continue
			}
			// If this canonical symbol was renamed, an Aliases entry
			// already covers the local name; skip the canonical here.
			if isAliased(sym, imp.Aliases) {
				continue
			}
			out[sym] = binding{
				Module: imp.Module,
				DepKey: imp.DepKey,
				Symbol: sym,
			}
		}
	}
	return out
}

// isAliased reports whether sym was renamed in the aliases map.
// `import { foo as bar }` → aliases["bar"] = "foo"; foo *itself* is
// not in scope, so we shouldn't bind it.
func isAliased(sym string, aliases map[string]string) bool {
	for _, canonical := range aliases {
		if canonical == sym {
			return true
		}
	}
	return false
}

// collectUsedSymbols walks the tree for two patterns that indicate an
// imported binding is *used*:
//
//  1. `pkg.method(...)` — member_expression where the object is a
//     bound identifier. Emit the property name as the used symbol.
//     Covers default + namespace imports (`_`/`ns.method`).
//
//  2. `method(...)` — call_expression where the callee is a bound
//     identifier (the named-import case: `import { merge }; merge()`).
//     Emit the canonical symbol.
//
// Property reads without a call (e.g. `_.PI` as a constant) are also
// captured under (1) — they're still "used".
func collectUsedSymbols(tree *ts.Tree, body []byte, imports []extract.Import) []extract.UsedSymbol {
	bindings := buildBindings(imports)
	if len(bindings) == 0 {
		return nil
	}
	var out []extract.UsedSymbol
	walk(tree.RootNode(), func(n *ts.Node) bool {
		switch n.Kind() {
		case "member_expression":
			obj := n.ChildByFieldName("object")
			prop := n.ChildByFieldName("property")
			if obj == nil || prop == nil || obj.Kind() != "identifier" {
				return true // recurse into nested member_expressions
			}
			b, ok := bindings[string(obj.Utf8Text(body))]
			if !ok {
				return true
			}
			out = append(out, extract.UsedSymbol{
				Module: b.Module,
				DepKey: b.DepKey,
				Symbol: string(prop.Utf8Text(body)),
				Line:   int(prop.StartPosition().Row) + 1,
				Column: int(prop.StartPosition().Column) + 1,
			})
		case "call_expression":
			fn := n.ChildByFieldName("function")
			if fn == nil || fn.Kind() != "identifier" {
				return true
			}
			b, ok := bindings[string(fn.Utf8Text(body))]
			if !ok {
				return true
			}
			// Named-import call: emit the canonical symbol.
			// Skip namespace bindings (Symbol == "*") — those only
			// surface useful symbols via member_expression.
			if b.Symbol == "*" {
				return true
			}
			out = append(out, extract.UsedSymbol{
				Module: b.Module,
				DepKey: b.DepKey,
				Symbol: b.Symbol,
				Line:   int(fn.StartPosition().Row) + 1,
				Column: int(fn.StartPosition().Column) + 1,
			})
		}
		return true
	})
	return out
}

// walk does a DFS over the named children of n, calling fn on each
// node. Returning false from fn skips that subtree.
func walk(n *ts.Node, fn func(*ts.Node) bool) {
	if n == nil {
		return
	}
	if !fn(n) {
		return
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		walk(n.NamedChild(i), fn)
	}
}
