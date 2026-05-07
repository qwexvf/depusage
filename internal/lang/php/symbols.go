package php

import (
	"strings"

	"github.com/qwexvf/depusage/internal/extract"

	ts "github.com/tree-sitter/go-tree-sitter"
)

type binding struct {
	Module string
	DepKey string
}

// buildBindings indexes use-imports by their short name (last
// `\`-separated segment) or by the explicit alias.
func buildBindings(imports []extract.Import) map[string]binding {
	out := map[string]binding{}
	for _, imp := range imports {
		if imp.Module == "" {
			continue
		}
		// Determine the in-scope name.
		var local string
		for alias := range imp.Aliases {
			local = alias
			break
		}
		if local == "" {
			i := strings.LastIndex(imp.Module, `\`)
			if i >= 0 {
				local = imp.Module[i+1:]
			} else {
				local = imp.Module
			}
		}
		if local == "" {
			continue
		}
		out[local] = binding{Module: imp.Module, DepKey: imp.DepKey}
	}
	return out
}

// collectUsedSymbols handles the two PHP idioms that imply a use'd
// class is actually referenced:
//
//	new Bar()                 — object_creation_expression
//	Bar::method() / Bar::CONST — scoped_call_expression / scoped_property_access_expression
//
// Instance method calls (`->method()`) aren't tied to the import
// directly (the receiver is a runtime value), so they're skipped.
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
			// First named child after `new` is the class name node.
			for i := uint(0); i < n.NamedChildCount(); i++ {
				c := n.NamedChild(i)
				if c == nil {
					continue
				}
				if c.Kind() == "name" || c.Kind() == "qualified_name" {
					short := lastNameSegment(string(c.Utf8Text(body)))
					if b, ok := bindings[short]; ok {
						emit(b, n, short)
					}
					break
				}
			}
		case "scoped_call_expression", "scoped_property_access_expression", "class_constant_access_expression":
			scope := n.ChildByFieldName("scope")
			nameN := n.ChildByFieldName("name")
			if scope == nil {
				return true
			}
			short := lastNameSegment(string(scope.Utf8Text(body)))
			b, ok := bindings[short]
			if !ok {
				return true
			}
			memberName := short
			if nameN != nil {
				memberName = string(nameN.Utf8Text(body))
			}
			anchor := scope
			if nameN != nil {
				anchor = nameN
			}
			emit(b, anchor, memberName)
		}
		return true
	})
	return out
}

// lastNameSegment returns the last \-separated segment of a PHP name.
// `Foo\Bar\Baz` -> `Baz`. Plain `Bar` -> `Bar`.
func lastNameSegment(s string) string {
	if i := strings.LastIndex(s, `\`); i >= 0 {
		return s[i+1:]
	}
	return s
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
