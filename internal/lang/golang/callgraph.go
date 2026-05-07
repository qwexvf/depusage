package golang

import (
	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// collectCallGraph builds a per-file callgraph for Go.
//
// Coverage:
//   - `function_declaration` (`func Foo() {}`)
//   - `method_declaration` (`func (r *R) Foo() {}`) — name is the
//     method identifier; receiver is informational only.
//
// Exported = identifier starts with uppercase.
//
// Edges include only same-file functions/methods called as bare
// identifiers (`Foo()`). `pkg.Foo()` calls go through UsedSymbols
// instead. Method calls (`r.Foo()`) are also dropped — resolving
// receivers is out of scope for the per-file graph.
func collectCallGraph(tree *ts.Tree, body []byte) *extract.CallGraph {
	type funcDef struct {
		fn       extract.Function
		bodyNode *ts.Node
	}
	defs := map[string]*funcDef{}
	var order []string

	addFunc := func(name string, defNode, bodyNode *ts.Node) {
		if name == "" || defNode == nil || bodyNode == nil {
			return
		}
		if _, dup := defs[name]; dup {
			return
		}
		exported := name != "" && name[0] >= 'A' && name[0] <= 'Z'
		defs[name] = &funcDef{
			fn: extract.Function{
				Name:     name,
				Exported: exported,
				StartLn:  int(defNode.StartPosition().Row) + 1,
				EndLn:    int(defNode.EndPosition().Row) + 1,
			},
			bodyNode: bodyNode,
		}
		order = append(order, name)
	}

	walk(tree.RootNode(), func(n *ts.Node) bool {
		switch n.Kind() {
		case "function_declaration", "method_declaration":
			nameN := n.ChildByFieldName("name")
			bodyN := n.ChildByFieldName("body")
			if nameN != nil {
				addFunc(string(nameN.Utf8Text(body)), n, bodyN)
			}
		}
		return true
	})

	if len(defs) == 0 {
		return nil
	}

	edges := map[string][]string{}
	for name, def := range defs {
		seen := map[string]struct{}{}
		walk(def.bodyNode, func(n *ts.Node) bool {
			if n.Kind() != "call_expression" {
				return true
			}
			fn := n.ChildByFieldName("function")
			if fn == nil || fn.Kind() != "identifier" {
				return true
			}
			callee := string(fn.Utf8Text(body))
			if callee == name {
				return true
			}
			if _, isLocal := defs[callee]; !isLocal {
				return true
			}
			if _, dup := seen[callee]; dup {
				return true
			}
			seen[callee] = struct{}{}
			edges[name] = append(edges[name], callee)
			return true
		})
	}

	cg := &extract.CallGraph{
		Funcs: make([]extract.Function, 0, len(order)),
		Edges: edges,
	}
	for _, name := range order {
		cg.Funcs = append(cg.Funcs, defs[name].fn)
	}
	return cg
}
