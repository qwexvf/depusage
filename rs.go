package depusage

import (
	_ "embed"
	"sync"

	"github.com/qwexvf/depusage/internal/tsutil"
	"github.com/qwexvf/depusage/lang/rs"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsrs "github.com/tree-sitter/tree-sitter-rust/bindings/go"
)

//go:embed lang/rs/queries.scm
var rsQueriesSource string

type rsState struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	useDeclIdx     int
	externCrateIdx int
}

var rsS = &rsState{useDeclIdx: -1, externCrateIdx: -1}

func (s *rsState) init() {
	s.once.Do(func() {
		s.lang = ts.NewLanguage(tsrs.Language())
		s.query = tsutil.MustCompileQuery(s.lang, rsQueriesSource, "rs")
		s.parsers = tsutil.NewParserPool(s.lang)
		s.cursors = tsutil.NewCursorPool()
		s.useDeclIdx = tsutil.CaptureIndex(s.query, "use_decl")
		s.externCrateIdx = tsutil.CaptureIndex(s.query, "extern_crate")
	})
}

func rsExtract(body []byte, opts Options) (Result, error) {
	rsS.init()

	parser := rsS.parsers.Get()
	defer rsS.parsers.Put(parser)
	tree := parser.Parse(body, nil)
	if tree == nil {
		return Result{}, nil
	}
	defer tree.Close()

	var res Result
	if opts.IncludeImports {
		res.Imports = rsCollectImports(tree, body)
	}
	return res, nil
}

func rsCollectImports(tree *ts.Tree, body []byte) []Import {
	cursor := rsS.cursors.Get()
	defer rsS.cursors.Put(cursor)

	matches := cursor.Matches(rsS.query, tree.RootNode(), body)
	imports := make([]Import, 0, 8)
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		for _, cap := range m.Captures {
			node := cap.Node
			switch int(cap.Index) {
			case rsS.useDeclIdx:
				if imp, ok := rsParseUseDecl(&node, body); ok {
					imports = append(imports, imp)
				}
			case rsS.externCrateIdx:
				if imp, ok := rsParseExternCrate(&node, body); ok {
					imports = append(imports, imp)
				}
			}
		}
	}
	return imports
}

// rsParseUseDecl extracts the leading crate name from a
// `use foo::bar::Baz;` style declaration. We walk the first
// argument's leftmost identifier.
func rsParseUseDecl(n *ts.Node, body []byte) (Import, bool) {
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
		return Import{}, false
	}
	crate := rsLeadingCrate(arg, body)
	if crate == "" {
		return Import{}, false
	}
	out := Import{
		Module: crate,
		DepKey: rs.DepKey(crate),
		Kind:   ImportStatic,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	if rs.IsRelative(crate) {
		out.Kind = ImportRelative
	}
	return out, true
}

// rsParseExternCrate handles `extern crate foo;`.
func rsParseExternCrate(n *ts.Node, body []byte) (Import, bool) {
	name := n.ChildByFieldName("name")
	if name == nil {
		return Import{}, false
	}
	crate := string(name.Utf8Text(body))
	out := Import{
		Module: crate,
		DepKey: rs.DepKey(crate),
		Kind:   ImportStatic,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	return out, true
}

// rsLeadingCrate descends into a use-clause structure to find the
// leftmost identifier — that's the crate-or-pseudo-crate name.
//
// Handles: identifier, scoped_identifier, scoped_use_list, use_as_clause,
// use_wildcard, use_list (rare).
func rsLeadingCrate(n *ts.Node, body []byte) string {
	switch n.Kind() {
	case "identifier", "self", "super", "crate":
		return string(n.Utf8Text(body))
	case "scoped_identifier", "scoped_use_list":
		if path := n.ChildByFieldName("path"); path != nil {
			return rsLeadingCrate(path, body)
		}
		// No `path` field — happens for `foo::bar` where `foo` is the
		// path itself. Walk the leftmost named child.
		if c := n.NamedChild(0); c != nil {
			return rsLeadingCrate(c, body)
		}
	case "use_as_clause":
		if path := n.ChildByFieldName("path"); path != nil {
			return rsLeadingCrate(path, body)
		}
	case "use_wildcard":
		if c := n.NamedChild(0); c != nil {
			return rsLeadingCrate(c, body)
		}
	}
	if c := n.NamedChild(0); c != nil {
		return rsLeadingCrate(c, body)
	}
	return ""
}
