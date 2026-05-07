package golang

import (
	"strings"

	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// binding maps a Go local package name to the import that introduced
// it. Symbol is always "*" for Go — the actual member resolves at the
// use site (`pkg.Func`, `pkg.Type`).
type binding struct {
	Module string
	DepKey string
}

func buildBindings(imports []extract.Import) map[string]binding {
	out := map[string]binding{}
	for _, imp := range imports {
		if imp.Module == "" {
			continue
		}
		// Explicit alias overrides the path-derived name. Skip `_`
		// (blank) and `.` (dot) — neither introduces a usable binding.
		var local string
		for alias := range imp.Aliases {
			if alias == "_" || alias == "." {
				local = ""
				goto skipPathDerivation
			}
			local = alias
			break
		}
		if local == "" {
			local = lastPathSegment(imp.Module)
		}
	skipPathDerivation:
		if local == "" {
			continue
		}
		out[local] = binding{Module: imp.Module, DepKey: imp.DepKey}
	}
	return out
}

func lastPathSegment(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

// collectUsedSymbols walks for `selector_expression` nodes where the
// operand is a known imported package name. Each match emits one
// UsedSymbol with the field name.
func collectUsedSymbols(tree *ts.Tree, body []byte, imports []extract.Import) []extract.UsedSymbol {
	bindings := buildBindings(imports)
	if len(bindings) == 0 {
		return nil
	}
	var out []extract.UsedSymbol
	walk(tree.RootNode(), func(n *ts.Node) bool {
		if n.Kind() != "selector_expression" {
			return true
		}
		op := n.ChildByFieldName("operand")
		fld := n.ChildByFieldName("field")
		if op == nil || fld == nil || op.Kind() != "identifier" {
			return true
		}
		b, ok := bindings[string(op.Utf8Text(body))]
		if !ok {
			return true
		}
		out = append(out, extract.UsedSymbol{
			Module: b.Module,
			DepKey: b.DepKey,
			Symbol: string(fld.Utf8Text(body)),
			Line:   int(fld.StartPosition().Row) + 1,
			Column: int(fld.StartPosition().Column) + 1,
		})
		return true
	})
	return out
}

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
