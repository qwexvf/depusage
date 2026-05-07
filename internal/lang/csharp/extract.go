// Package csharp implements the C# import extractor.
package csharp

import (
	_ "embed"
	"strings"
	"sync"

	"github.com/qwexvf/depusage/internal/extract"
	"github.com/qwexvf/depusage/internal/tsutil"

	ts "github.com/tree-sitter/go-tree-sitter"
	tscs "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
)

//go:embed queries.scm
var queriesSource string

type state struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	usingDirIdx int
}

var s = &state{usingDirIdx: -1}

func (st *state) init() {
	st.once.Do(func() {
		st.lang = ts.NewLanguage(tscs.Language())
		st.query = tsutil.MustCompileQuery(st.lang, queriesSource, "cs")
		st.parsers = tsutil.NewParserPool(st.lang)
		st.cursors = tsutil.NewCursorPool()
		st.usingDirIdx = tsutil.CaptureIndex(st.query, "using_dir")
	})
}

// Extract runs the C# import pass over body.
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
	if opts.IncludeCallGraph {
		res.CallGraph = collectCallGraph(tree, body)
	}
	// UsedSymbols not implemented for C#: `using NS;` opens a namespace
	// without binding a specific name; types are referenced by short
	// name without an explicit per-import binding.
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
			if int(cap.Index) != s.usingDirIdx {
				continue
			}
			node := cap.Node
			if imp, ok := parseUsingDir(&node, body); ok {
				imports = append(imports, imp)
			}
		}
	}
	return imports
}

// parseUsingDir extracts the namespace from a using_directive.
// Handles plain `using X;`, `using static X.Y;`, and `using A = X.Y;`.
func parseUsingDir(n *ts.Node, body []byte) (extract.Import, bool) {
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
		return extract.Import{}, false
	}
	out := extract.Import{
		Module: name,
		DepKey: DepKey(name),
		Kind:   extract.ImportStatic,
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
