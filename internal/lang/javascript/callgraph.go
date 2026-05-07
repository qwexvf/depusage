package javascript

import (
	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// collectCallGraph builds a per-file callgraph: for each function
// definition discovered in the file, the set of other in-file
// functions it calls.
//
// Coverage:
//   - Top-level `function name() {}` declarations.
//   - Class methods (`method_definition` inside `class_declaration`).
//   - Variable-bound function/arrow expressions: `const f = () => {}`,
//     `const f = function() {}`.
//
// Out of scope: object-literal methods, prototype assignments, and
// anything where the name is computed at runtime.
//
// Edges only point at other discovered functions in the same file.
// Calls to imports / globals / unresolved names are silently dropped.
func collectCallGraph(tree *ts.Tree, body []byte) *extract.CallGraph {
	root := tree.RootNode()

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
			// Re-declaration: keep the first; later ones likely shadow
			// inside a block we don't model.
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

	walk(root, func(n *ts.Node) bool {
		switch n.Kind() {
		case "function_declaration":
			name := n.ChildByFieldName("name")
			b := n.ChildByFieldName("body")
			if name != nil {
				addFunc(string(name.Utf8Text(body)), isExportedAt(n), n, b)
			}
		case "method_definition":
			name := n.ChildByFieldName("name")
			b := n.ChildByFieldName("body")
			if name != nil {
				addFunc(string(name.Utf8Text(body)), false, n, b)
			}
		case "variable_declarator":
			// `const f = () => {}` / `const f = function() {}`.
			id := n.ChildByFieldName("name")
			val := n.ChildByFieldName("value")
			if id == nil || val == nil || id.Kind() != "identifier" {
				return true
			}
			var b *ts.Node
			switch val.Kind() {
			case "arrow_function":
				b = val.ChildByFieldName("body")
			case "function_expression", "function":
				b = val.ChildByFieldName("body")
			}
			if b == nil {
				return true
			}
			addFunc(string(id.Utf8Text(body)), isExportedAt(n), n, b)
		}
		return true
	})

	if len(defs) == 0 {
		return nil
	}

	// Pass 2: for each function, find local-name calls in its body.
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
				return true // self-recursion isn't a useful edge here
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

// isExportedAt walks up from n looking for an `export_statement`
// ancestor. JS / TS treat `export function foo() {}` and
// `export const f = () => {}` as exported.
func isExportedAt(n *ts.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		switch p.Kind() {
		case "export_statement":
			return true
		case "program":
			return false
		}
	}
	return false
}
