// Package golang implements the Go import extractor.
package golang

import (
	_ "embed"
	"strings"
	"sync"

	"github.com/qwexvf/depusage/internal/extract"
	"github.com/qwexvf/depusage/internal/tsutil"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsgo "github.com/tree-sitter/tree-sitter-go/bindings/go"
)

//go:embed queries.scm
var queriesSource string

type state struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	importSpecIdx int
}

var s = &state{importSpecIdx: -1}

func (st *state) init() {
	st.once.Do(func() {
		st.lang = ts.NewLanguage(tsgo.Language())
		st.query = tsutil.MustCompileQuery(st.lang, queriesSource, "go")
		st.parsers = tsutil.NewParserPool(st.lang)
		st.cursors = tsutil.NewCursorPool()
		st.importSpecIdx = tsutil.CaptureIndex(st.query, "import_spec")
	})
}

// Extract runs the Go import pass over body.
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
	if opts.IncludeImports || opts.IncludeSymbols {
		res.Imports = collectImports(tree, body)
	}
	if opts.IncludeSymbols {
		res.UsedSymbols = collectUsedSymbols(tree, body, res.Imports)
	}
	if opts.IncludeCallGraph {
		res.CallGraph = collectCallGraph(tree, body)
	}
	if !opts.IncludeImports {
		res.Imports = nil
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
			if int(cap.Index) != s.importSpecIdx {
				continue
			}
			if imp, ok := parseImportSpec(&node, body); ok {
				imports = append(imports, imp)
			}
		}
	}
	return imports
}

// parseImportSpec turns an `import_spec` node into one Import.
// Handles the four spec shapes: bare path, named alias, `.` import,
// `_` blank import.
func parseImportSpec(n *ts.Node, body []byte) (extract.Import, bool) {
	pathNode := n.ChildByFieldName("path")
	if pathNode == nil {
		return extract.Import{}, false
	}
	module := stringLiteralValue(pathNode, body)
	out := extract.Import{
		Module: module,
		DepKey: DepKey(module),
		Kind:   extract.ImportStatic,
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

// stringLiteralValue strips surrounding quotes from an
// interpreted_string_literal or raw_string_literal node text.
func stringLiteralValue(n *ts.Node, body []byte) string {
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
