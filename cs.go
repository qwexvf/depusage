package depusage

import (
	_ "embed"
	"strings"
	"sync"

	"github.com/qwexvf/depusage/internal/tsutil"
	"github.com/qwexvf/depusage/lang/cs"

	ts "github.com/tree-sitter/go-tree-sitter"
	tscs "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
)

//go:embed lang/cs/queries.scm
var csQueriesSource string

type csState struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	usingDirIdx int
}

var csS = &csState{usingDirIdx: -1}

func (s *csState) init() {
	s.once.Do(func() {
		s.lang = ts.NewLanguage(tscs.Language())
		s.query = tsutil.MustCompileQuery(s.lang, csQueriesSource, "cs")
		s.parsers = tsutil.NewParserPool(s.lang)
		s.cursors = tsutil.NewCursorPool()
		s.usingDirIdx = tsutil.CaptureIndex(s.query, "using_dir")
	})
}

func csExtract(body []byte, opts Options) (Result, error) {
	csS.init()

	parser := csS.parsers.Get()
	defer csS.parsers.Put(parser)
	tree := parser.Parse(body, nil)
	if tree == nil {
		return Result{}, nil
	}
	defer tree.Close()

	var res Result
	if opts.IncludeImports {
		res.Imports = csCollectImports(tree, body)
	}
	return res, nil
}

func csCollectImports(tree *ts.Tree, body []byte) []Import {
	cursor := csS.cursors.Get()
	defer csS.cursors.Put(cursor)

	matches := cursor.Matches(csS.query, tree.RootNode(), body)
	imports := make([]Import, 0, 8)
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		for _, cap := range m.Captures {
			if int(cap.Index) != csS.usingDirIdx {
				continue
			}
			node := cap.Node
			if imp, ok := csParseUsingDir(&node, body); ok {
				imports = append(imports, imp)
			}
		}
	}
	return imports
}

// csParseUsingDir extracts the namespace from a using_directive.
// Handles plain `using X;`, `using static X.Y;`, and `using A = X.Y;`.
func csParseUsingDir(n *ts.Node, body []byte) (Import, bool) {
	var alias, name string
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case "name_equals":
			// `Alias =` form. The identifier is the alias.
			for j := uint(0); j < c.NamedChildCount(); j++ {
				if id := c.NamedChild(j); id != nil && id.Kind() == "identifier" {
					alias = string(id.Utf8Text(body))
				}
			}
		case "qualified_name", "identifier", "generic_name":
			name = string(c.Utf8Text(body))
		}
	}
	if name == "" {
		return Import{}, false
	}
	out := Import{
		Module: name,
		DepKey: cs.DepKey(name),
		Kind:   ImportStatic,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	// Last dotted segment is the symbol.
	if i := strings.LastIndex(name, "."); i > 0 {
		out.Symbols = []string{name[i+1:]}
	} else {
		out.Symbols = []string{name}
	}
	if alias != "" {
		out.Aliases = map[string]string{alias: out.Symbols[0]}
	}
	return out, true
}
