// Package java implements the Java import extractor.
package java

import (
	_ "embed"
	"strings"
	"sync"

	"github.com/qwexvf/depusage/internal/extract"
	"github.com/qwexvf/depusage/internal/tsutil"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsjv "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

//go:embed queries.scm
var queriesSource string

type state struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	importDeclIdx int
}

var s = &state{importDeclIdx: -1}

func (st *state) init() {
	st.once.Do(func() {
		st.lang = ts.NewLanguage(tsjv.Language())
		st.query = tsutil.MustCompileQuery(st.lang, queriesSource, "java")
		st.parsers = tsutil.NewParserPool(st.lang)
		st.cursors = tsutil.NewCursorPool()
		st.importDeclIdx = tsutil.CaptureIndex(st.query, "import_decl")
	})
}

// Extract runs the Java import pass over body.
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
			if int(cap.Index) != s.importDeclIdx {
				continue
			}
			node := cap.Node
			if imp, ok := parseImportDecl(&node, body); ok {
				imports = append(imports, imp)
			}
		}
	}
	return imports
}

// parseImportDecl extracts the FQCN from an import_declaration.
// Strips trailing `.*` for wildcard imports.
func parseImportDecl(n *ts.Node, body []byte) (extract.Import, bool) {
	// Walk children for the scoped_identifier or scoped_type_identifier.
	// The `static` keyword and `*` asterisk live as siblings.
	var fqcn string
	wildcard := false
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case "scoped_identifier", "scoped_type_identifier", "identifier":
			fqcn = string(c.Utf8Text(body))
		case "asterisk":
			wildcard = true
		}
	}
	if fqcn == "" {
		return extract.Import{}, false
	}
	module := fqcn
	if wildcard {
		module = fqcn + ".*"
	}
	out := extract.Import{
		Module: module,
		DepKey: DepKey(module),
		Kind:   extract.ImportStatic,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	if wildcard {
		out.Symbols = []string{"*"}
	} else {
		// Last dotted segment is the imported symbol.
		if i := strings.LastIndex(fqcn, "."); i > 0 {
			out.Symbols = []string{fqcn[i+1:]}
		}
	}
	return out, true
}
