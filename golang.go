package depusage

import (
	_ "embed"
	"strings"
	"sync"

	"github.com/qwexvf/depusage/internal/tsutil"
	"github.com/qwexvf/depusage/lang/golang"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsgo "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

//go:embed lang/golang/queries.scm
var goQueriesSource string

type goState struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	importSpecIdx int
}

var goS = &goState{importSpecIdx: -1}

func (s *goState) init() {
	s.once.Do(func() {
		s.lang = ts.NewLanguage(tsgo.Language())
		s.query = tsutil.MustCompileQuery(s.lang, goQueriesSource, "go")
		s.parsers = tsutil.NewParserPool(s.lang)
		s.cursors = tsutil.NewCursorPool()
		s.importSpecIdx = tsutil.CaptureIndex(s.query, "import_spec")
	})
}

func goExtract(body []byte, opts Options) (Result, error) {
	goS.init()

	parser := goS.parsers.Get()
	defer goS.parsers.Put(parser)
	tree := parser.Parse(body, nil)
	if tree == nil {
		return Result{}, nil
	}
	defer tree.Close()

	var res Result
	if opts.IncludeImports {
		res.Imports = goCollectImports(tree, body)
	}
	return res, nil
}

func goCollectImports(tree *ts.Tree, body []byte) []Import {
	cursor := goS.cursors.Get()
	defer goS.cursors.Put(cursor)

	matches := cursor.Matches(goS.query, tree.RootNode(), body)
	imports := make([]Import, 0, 8)
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		for _, cap := range m.Captures {
			node := cap.Node
			if int(cap.Index) != goS.importSpecIdx {
				continue
			}
			if imp, ok := goParseImportSpec(&node, body); ok {
				imports = append(imports, imp)
			}
		}
	}
	return imports
}

// goParseImportSpec turns an `import_spec` node into one Import.
// Handles the four spec shapes: bare path, named alias, `.` import,
// `_` blank import.
func goParseImportSpec(n *ts.Node, body []byte) (Import, bool) {
	pathNode := n.ChildByFieldName("path")
	if pathNode == nil {
		return Import{}, false
	}
	module := goStringLiteralValue(pathNode, body)
	out := Import{
		Module: module,
		DepKey: golang.DepKey(module),
		Kind:   ImportStatic,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	if name := n.ChildByFieldName("name"); name != nil {
		alias := string(name.Utf8Text(body))
		switch alias {
		case "_":
			// blank import: kept as a side-effect import; alias to "_"
			// keeps the information available to consumers.
			out.Aliases = map[string]string{"_": "*"}
		case ".":
			// dot import: brings names into the current namespace.
			out.Aliases = map[string]string{".": "*"}
		default:
			out.Aliases = map[string]string{alias: "*"}
		}
	}
	return out, true
}

// goStringLiteralValue strips surrounding quotes from an
// interpreted_string_literal or raw_string_literal node text.
func goStringLiteralValue(n *ts.Node, body []byte) string {
	raw := string(n.Utf8Text(body))
	raw = strings.TrimSpace(raw)
	if len(raw) >= 2 {
		first, last := raw[0], raw[len(raw)-1]
		if (first == '"' && last == '"') || (first == '`' && last == '`') {
			return raw[1 : len(raw)-1]
		}
	}
	return raw
}
