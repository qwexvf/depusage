// Package rust implements the Rust import extractor.
package rust

import (
	_ "embed"
	"sync"

	"github.com/qwexvf/depusage/internal/extract"
	"github.com/qwexvf/depusage/internal/tsutil"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsrs "github.com/tree-sitter/tree-sitter-rust/bindings/go"
)

//go:embed queries.scm
var queriesSource string

type state struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	useDeclIdx     int
	externCrateIdx int
}

var s = &state{useDeclIdx: -1, externCrateIdx: -1}

func (st *state) init() {
	st.once.Do(func() {
		st.lang = ts.NewLanguage(tsrs.Language())
		st.query = tsutil.MustCompileQuery(st.lang, queriesSource, "rs")
		st.parsers = tsutil.NewParserPool(st.lang)
		st.cursors = tsutil.NewCursorPool()
		st.useDeclIdx = tsutil.CaptureIndex(st.query, "use_decl")
		st.externCrateIdx = tsutil.CaptureIndex(st.query, "extern_crate")
	})
}

// Extract runs the Rust import pass over body.
func Extract(body []byte, opts extract.Options) (extract.Result, error) {
	s.init()

	parser := s.parsers.Get()
	defer s.parsers.Put(parser)
	tree := parser.Parse(body, nil)
	if tree == nil {
		return extract.Result{}, nil
	}
	defer tree.Close()

	var res extract.Result
	if opts.IncludeImports {
		res.Imports = collectImports(tree, body)
	}
	return res, nil
}

func collectImports(tree *ts.Tree, body []byte) []extract.Import {
	cursor := s.cursors.Get()
	defer s.cursors.Put(cursor)

	matches := cursor.Matches(s.query, tree.RootNode(), body)
	imports := make([]extract.Import, 0, 8)
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		for _, cap := range m.Captures {
			node := cap.Node
			switch int(cap.Index) {
			case s.useDeclIdx:
				if imp, ok := parseUseDecl(&node, body); ok {
					imports = append(imports, imp)
				}
			case s.externCrateIdx:
				if imp, ok := parseExternCrate(&node, body); ok {
					imports = append(imports, imp)
				}
			}
		}
	}
	return imports
}

// parseUseDecl extracts the leading crate name from a
// `use foo::bar::Baz;` style declaration. We walk the first
// argument's leftmost identifier.
func parseUseDecl(n *ts.Node, body []byte) (extract.Import, bool) {
	arg := n.ChildByFieldName("argument")
	if arg == nil {
		// Fallback: first named child after `use`.
		for i := uint(0); i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if c != nil {
				arg = c
				break
			}
		}
	}
	if arg == nil {
		return extract.Import{}, false
	}
	crate := leadingCrate(arg, body)
	if crate == "" {
		return extract.Import{}, false
	}
	out := extract.Import{
		Module: crate,
		DepKey: DepKey(crate),
		Kind:   extract.ImportStatic,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	if IsRelative(crate) {
		out.Kind = extract.ImportRelative
	}
	return out, true
}

// parseExternCrate handles `extern crate foo;`.
func parseExternCrate(n *ts.Node, body []byte) (extract.Import, bool) {
	name := n.ChildByFieldName("name")
	if name == nil {
		return extract.Import{}, false
	}
	crate := string(name.Utf8Text(body))
	out := extract.Import{
		Module: crate,
		DepKey: DepKey(crate),
		Kind:   extract.ImportStatic,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	return out, true
}

// leadingCrate descends into a use-clause structure to find the
// leftmost identifier — that's the crate-or-pseudo-crate name.
//
// Handles: identifier, scoped_identifier, scoped_use_list, use_as_clause,
// use_wildcard, use_list (rare).
func leadingCrate(n *ts.Node, body []byte) string {
	switch n.Kind() {
	case "identifier", "self", "super", "crate":
		return string(n.Utf8Text(body))
	case "scoped_identifier", "scoped_use_list":
		if path := n.ChildByFieldName("path"); path != nil {
			return leadingCrate(path, body)
		}
		// No `path` field — happens for `foo::bar` where `foo` is the
		// path itself. Walk the leftmost named child.
		if c := n.NamedChild(0); c != nil {
			return leadingCrate(c, body)
		}
	case "use_as_clause":
		if path := n.ChildByFieldName("path"); path != nil {
			return leadingCrate(path, body)
		}
	case "use_wildcard":
		if c := n.NamedChild(0); c != nil {
			return leadingCrate(c, body)
		}
	}
	if c := n.NamedChild(0); c != nil {
		return leadingCrate(c, body)
	}
	return ""
}
