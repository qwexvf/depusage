package depusage

import (
	_ "embed"
	"strings"
	"sync"

	"github.com/qwexvf/depusage/internal/tsutil"
	"github.com/qwexvf/depusage/lang/jv"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsjv "github.com/tree-sitter/tree-sitter-java/bindings/go"
)

//go:embed lang/jv/queries.scm
var jvQueriesSource string

type jvState struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	importDeclIdx int
}

var jvS = &jvState{importDeclIdx: -1}

func (s *jvState) init() {
	s.once.Do(func() {
		s.lang = ts.NewLanguage(tsjv.Language())
		s.query = tsutil.MustCompileQuery(s.lang, jvQueriesSource, "jv")
		s.parsers = tsutil.NewParserPool(s.lang)
		s.cursors = tsutil.NewCursorPool()
		s.importDeclIdx = tsutil.CaptureIndex(s.query, "import_decl")
	})
}

func jvExtract(body []byte, opts Options) (Result, error) {
	jvS.init()

	parser := jvS.parsers.Get()
	defer jvS.parsers.Put(parser)
	tree := parser.Parse(body, nil)
	if tree == nil {
		return Result{}, nil
	}
	defer tree.Close()

	var res Result
	if opts.IncludeImports {
		res.Imports = jvCollectImports(tree, body)
	}
	return res, nil
}

func jvCollectImports(tree *ts.Tree, body []byte) []Import {
	cursor := jvS.cursors.Get()
	defer jvS.cursors.Put(cursor)

	matches := cursor.Matches(jvS.query, tree.RootNode(), body)
	imports := make([]Import, 0, 8)
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		for _, cap := range m.Captures {
			if int(cap.Index) != jvS.importDeclIdx {
				continue
			}
			node := cap.Node
			if imp, ok := jvParseImportDecl(&node, body); ok {
				imports = append(imports, imp)
			}
		}
	}
	return imports
}

// jvParseImportDecl extracts the FQCN from an import_declaration.
// Strips trailing `.*` for wildcard imports.
func jvParseImportDecl(n *ts.Node, body []byte) (Import, bool) {
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
		return Import{}, false
	}
	module := fqcn
	if wildcard {
		module = fqcn + ".*"
	}
	out := Import{
		Module: module,
		DepKey: jv.DepKey(module),
		Kind:   ImportStatic,
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
