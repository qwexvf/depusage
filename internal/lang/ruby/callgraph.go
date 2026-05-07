package ruby

import (
	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// collectCallGraph builds a per-file callgraph for Ruby.
//
// Coverage:
//   - `method` nodes (def name; ... end). Includes module + class
//     methods.
//
// Edges are bare receiver-less method calls in the body. Method
// calls on explicit receivers (`x.foo`) and module-resolution calls
// (`Mod.foo`) are dropped.
//
// Ruby has no `public`/`private` keyword visibility — they're toggled
// by mutating statements within a class. We approximate "exported"
// by name: lowercase letter or underscore prefix → not exported,
// otherwise yes. Conservative for Ruby's actual semantics but
// matches the conventional read.
func collectCallGraph(tree *ts.Tree, body []byte) *extract.CallGraph {
	type funcDef struct {
		fn       extract.Function
		bodyNode *ts.Node
	}
	defs := map[string]*funcDef{}
	var order []string

	addFunc := func(name string, defNode, bodyNode *ts.Node) {
		if name == "" || defNode == nil {
			return
		}
		if _, dup := defs[name]; dup {
			return
		}
		exported := name != "" && name[0] != '_'
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
		if n.Kind() != "method" {
			return true
		}
		nameN := n.ChildByFieldName("name")
		bodyN := n.ChildByFieldName("body")
		if nameN == nil {
			return true
		}
		// Use the method node itself as the body container if no body
		// field — DFS over named children covers it.
		if bodyN == nil {
			bodyN = n
		}
		addFunc(string(nameN.Utf8Text(body)), n, bodyN)
		return true
	})

	if len(defs) == 0 {
		return nil
	}

	edges := map[string][]string{}
	for name, def := range defs {
		seen := map[string]struct{}{}
		walk(def.bodyNode, func(n *ts.Node) bool {
			// Tree-sitter-ruby uses `call` for both receiver-less
			// invocations and receiver.method calls. Bare identifier
			// references with no receiver field are the bare-call shape.
			if n.Kind() != "call" && n.Kind() != "identifier" {
				return true
			}
			var callee string
			if n.Kind() == "identifier" {
				// Reject if this identifier IS the method's name node
				// or part of a larger call chain we already covered.
				if p := n.Parent(); p != nil && p.Kind() == "call" {
					return true
				}
				callee = string(n.Utf8Text(body))
			} else {
				if n.ChildByFieldName("receiver") != nil {
					return true
				}
				m := n.ChildByFieldName("method")
				if m == nil {
					return true
				}
				callee = string(m.Utf8Text(body))
			}
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
