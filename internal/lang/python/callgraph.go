package python

import (
	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// collectCallGraph builds a per-file callgraph for Python.
//
// Coverage:
//   - Top-level `def name(...)` and `async def name(...)`.
//   - Methods inside a `class_definition`'s body.
//
// Out of scope: nested closures (defs inside defs are picked up but
// scoping isn't modeled — calls in an outer fn that match an inner
// fn's name are still recorded as edges, which is fine for the
// noise-reduction use case).
//
// Python "exported" approximation: a function whose name doesn't
// start with `_` is treated as Exported.
func collectCallGraph(tree *ts.Tree, body []byte) *extract.CallGraph {
	root := tree.RootNode()

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

	walk(root, func(n *ts.Node) bool {
		if n.Kind() != "function_definition" {
			return true
		}
		nameN := n.ChildByFieldName("name")
		bodyN := n.ChildByFieldName("body")
		if nameN != nil {
			addFunc(string(nameN.Utf8Text(body)), n, bodyN)
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
			if n.Kind() != "call" {
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
