package python

import (
	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// binding maps a local Python identifier to the import that introduced
// it. Symbol = "*" for whole-module imports (`import requests` /
// `import numpy as np` — `requests`/`np` is the local binding); the
// actual member resolves at the use site via attribute access.
type binding struct {
	Module string
	DepKey string
	Symbol string
}

func buildBindings(imports []extract.Import) map[string]binding {
	out := map[string]binding{}
	for _, imp := range imports {
		if imp.Module == "" {
			continue
		}
		// Aliases first: explicit local-name renames.
		for local, canonical := range imp.Aliases {
			out[local] = binding{Module: imp.Module, DepKey: imp.DepKey, Symbol: canonical}
		}
		// Symbol bindings.
		for _, sym := range imp.Symbols {
			if sym == "*" {
				// `import foo` (Python's only "*" case here): the
				// local binding is the module's last dotted segment
				// (foo from foo.bar). Aliases already cover the
				// renamed form.
				if hasAliasForStar(imp.Aliases) {
					continue
				}
				local := lastDotSegment(imp.Module)
				if local == "" {
					continue
				}
				out[local] = binding{Module: imp.Module, DepKey: imp.DepKey, Symbol: "*"}
				continue
			}
			if isAliased(sym, imp.Aliases) {
				continue
			}
			out[sym] = binding{Module: imp.Module, DepKey: imp.DepKey, Symbol: sym}
		}
	}
	return out
}

func hasAliasForStar(aliases map[string]string) bool {
	for _, canonical := range aliases {
		if canonical == "*" {
			return true
		}
	}
	return false
}

func isAliased(sym string, aliases map[string]string) bool {
	for _, canonical := range aliases {
		if canonical == sym {
			return true
		}
	}
	return false
}

func lastDotSegment(s string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return s[i+1:]
		}
	}
	return s
}

// collectUsedSymbols walks the tree for two patterns:
//
//  1. Attribute access on a bound name: `np.array(...)`, `np.array`.
//     Tree-sitter node: `attribute` with `object` + `attribute` fields.
//
//  2. Direct call of a `from X import Y` binding: `Y()`. Tree-sitter
//     node: `call` with `function` = `identifier`.
func collectUsedSymbols(tree *ts.Tree, body []byte, imports []extract.Import) []extract.UsedSymbol {
	bindings := buildBindings(imports)
	if len(bindings) == 0 {
		return nil
	}
	var out []extract.UsedSymbol
	walk(tree.RootNode(), func(n *ts.Node) bool {
		switch n.Kind() {
		case "attribute":
			obj := n.ChildByFieldName("object")
			attr := n.ChildByFieldName("attribute")
			if obj == nil || attr == nil || obj.Kind() != "identifier" {
				return true
			}
			b, ok := bindings[string(obj.Utf8Text(body))]
			if !ok {
				return true
			}
			out = append(out, extract.UsedSymbol{
				Module: b.Module,
				DepKey: b.DepKey,
				Symbol: string(attr.Utf8Text(body)),
				Line:   int(attr.StartPosition().Row) + 1,
				Column: int(attr.StartPosition().Column) + 1,
			})
		case "call":
			fn := n.ChildByFieldName("function")
			if fn == nil || fn.Kind() != "identifier" {
				return true
			}
			b, ok := bindings[string(fn.Utf8Text(body))]
			if !ok {
				return true
			}
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

// walk does a DFS over the named children of n.
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
