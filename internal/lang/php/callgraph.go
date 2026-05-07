package php

import (
	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// collectCallGraph builds a per-file callgraph for PHP.
//
// Coverage:
//   - function_definition (top-level `function foo() {}`)
//   - method_declaration (inside a class)
//
// Edges are bare-name calls (`foo()`); `Bar::method()` and
// `$obj->method()` are out of scope.
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
		case "function_definition":
			nameN := n.ChildByFieldName("name")
			bodyN := n.ChildByFieldName("body")
			if nameN != nil {
				addFunc(string(nameN.Utf8Text(body)), true, n, bodyN)
			}
		case "method_declaration":
			nameN := n.ChildByFieldName("name")
			bodyN := n.ChildByFieldName("body")
			if nameN != nil {
				addFunc(string(nameN.Utf8Text(body)), hasPublicVisibility(n), n, bodyN)
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
			if n.Kind() != "function_call_expression" {
				return true
			}
			fn := n.ChildByFieldName("function")
			if fn == nil {
				return true
			}
			if fn.Kind() != "name" {
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

func hasPublicVisibility(n *ts.Node) bool {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() == "visibility_modifier" {
			// Tree-sitter exposes the keyword as a token child; check text.
			if c.NamedChildCount() == 0 {
				return false
			}
			// More common: visibility token is a child with kind in {public, private, protected}
			for j := uint(0); j < c.NamedChildCount(); j++ {
				k := c.NamedChild(j).Kind()
				if k == "public" {
					return true
				}
			}
		}
	}
	return false
}
