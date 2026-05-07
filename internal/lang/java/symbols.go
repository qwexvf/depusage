package java

import (
	"strings"

	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// binding maps a Java short class-name to its FQCN-bearing import.
type binding struct {
	Module string
	DepKey string
}

// buildBindings indexes imports by the last dotted segment of the
// FQCN. Wildcard imports (`import com.foo.*`) and static-member
// imports are skipped — they don't introduce a single-name binding
// that resolves to a specific imported class.
func buildBindings(imports []extract.Import) map[string]binding {
	out := map[string]binding{}
	for _, imp := range imports {
		if imp.Module == "" || strings.HasSuffix(imp.Module, ".*") {
			continue
		}
		i := strings.LastIndex(imp.Module, ".")
		if i < 0 {
			continue
		}
		short := imp.Module[i+1:]
		out[short] = binding{Module: imp.Module, DepKey: imp.DepKey}
	}
	return out
}

// collectUsedSymbols walks for the three idiomatic Java use shapes:
//
//	new Bar()                            object_creation_expression
//	Bar.staticMethod()                   method_invocation w/ object: Bar
//	Bar.STATIC_FIELD                     field_access w/ object: Bar
//
// In every case we emit the imported class as the used symbol — it's
// the granularity the consumer actually cares about ("did the user
// reference type X from package P?").
func collectUsedSymbols(tree *ts.Tree, body []byte, imports []extract.Import) []extract.UsedSymbol {
	bindings := buildBindings(imports)
	if len(bindings) == 0 {
		return nil
	}
	var out []extract.UsedSymbol
	emit := func(b binding, n *ts.Node, name string) {
		out = append(out, extract.UsedSymbol{
			Module: b.Module,
			DepKey: b.DepKey,
			Symbol: name,
			Line:   int(n.StartPosition().Row) + 1,
			Column: int(n.StartPosition().Column) + 1,
		})
	}
	walk(tree.RootNode(), func(n *ts.Node) bool {
		switch n.Kind() {
		case "object_creation_expression":
			t := n.ChildByFieldName("type")
			if t == nil {
				return true
			}
			short := string(t.Utf8Text(body))
			if b, ok := bindings[short]; ok {
				emit(b, n, short)
			}
		case "method_invocation":
			obj := n.ChildByFieldName("object")
			name := n.ChildByFieldName("name")
			if obj == nil || name == nil || obj.Kind() != "identifier" {
				return true
			}
			short := string(obj.Utf8Text(body))
			if b, ok := bindings[short]; ok {
				emit(b, name, string(name.Utf8Text(body)))
			}
		case "field_access":
			obj := n.ChildByFieldName("object")
			fld := n.ChildByFieldName("field")
			if obj == nil || fld == nil || obj.Kind() != "identifier" {
				return true
			}
			short := string(obj.Utf8Text(body))
			if b, ok := bindings[short]; ok {
				emit(b, fld, string(fld.Utf8Text(body)))
			}
		}
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
