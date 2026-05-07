package rust

import (
	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// collectCallGraph builds a per-file callgraph for Rust.
//
// Coverage:
//   - function_item (`fn name() {}`) at any nesting level.
//
// Exported = the item carries a `pub` (or `pub(...)`) visibility
// modifier. Edges are bare-name calls — `name()` — only; method
// calls (`receiver.method()`) and qualified calls (`mod::fn()`) are
// out of scope.
func collectCallGraph(tree *ts.Tree, body []byte) *extract.CallGraph {
	type funcDef struct {
		fn       extract.Function
		bodyNode *ts.Node
	}
	defs := map[string]*funcDef{}
	var order []string

	addFunc := func(name string, exported bool, defNode, bodyNode *ts.Node) {
		if name == "" || defNode == nil || bodyNode == nil {
			return
		}
		if _, dup := defs[name]; dup {
			return
		}
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
		if n.Kind() != "function_item" {
			return true
		}
		nameN := n.ChildByFieldName("name")
		bodyN := n.ChildByFieldName("body")
		if nameN != nil {
			addFunc(string(nameN.Utf8Text(body)), hasPubVisibility(n), n, bodyN)
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

func hasPubVisibility(n *ts.Node) bool {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c != nil && c.Kind() == "visibility_modifier" {
			return true
		}
	}
	return false
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
