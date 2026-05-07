package java

import (
	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// collectCallGraph builds a per-file callgraph for Java.
//
// Coverage:
//   - method_declaration (instance + static methods)
//   - constructor_declaration (treated as a func named after the class)
//
// "Exported" mirrors the `public` modifier; package-private / private
// methods are not Exported. Edges are bare-name calls within the same
// file; `obj.method()` and `Pkg.method()` are out of scope (the
// receiver / class isn't resolved at the per-file level).
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
		switch n.Kind() {
		case "method_declaration", "constructor_declaration":
			nameN := n.ChildByFieldName("name")
			bodyN := n.ChildByFieldName("body")
			if nameN != nil {
				addFunc(string(nameN.Utf8Text(body)), hasPublicModifier(n, body), n, bodyN)
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
			if n.Kind() != "method_invocation" {
				return true
			}
			// We only count BARE calls — `foo()` — not `obj.foo()`.
			// Bare invocations have no `object` field set.
			if obj := n.ChildByFieldName("object"); obj != nil {
				return true
			}
			nm := n.ChildByFieldName("name")
			if nm == nil {
				return true
			}
			callee := string(nm.Utf8Text(body))
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

// hasPublicModifier reports whether a method/constructor declaration
// node carries a `public` access modifier.
func hasPublicModifier(n *ts.Node, body []byte) bool {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() != "modifiers" {
			continue
		}
		for j := uint(0); j < c.NamedChildCount(); j++ {
			mod := c.NamedChild(j)
			if mod != nil && mod.Kind() == "public" {
				return true
			}
		}
		break
	}
	return false
}
