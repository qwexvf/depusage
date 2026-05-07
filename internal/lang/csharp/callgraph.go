package csharp

import (
	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// collectCallGraph builds a per-file callgraph for C#.
//
// Coverage:
//   - method_declaration (instance + static)
//   - constructor_declaration
//   - local_function_statement (rarely used but cheap to include)
//
// Exported = `public` modifier present. Edges are bare-name
// invocations within the same file; `obj.Method()` and `Type.Static()`
// don't appear in edges (the receiver / class isn't resolved here).
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
		case "method_declaration", "constructor_declaration", "local_function_statement":
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
			if n.Kind() != "invocation_expression" {
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

func hasPublicModifier(n *ts.Node, body []byte) bool {
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() == "modifier" && string(c.Utf8Text(body)) == "public" {
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
