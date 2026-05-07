// Package php implements the PHP import extractor.
package php

import (
	_ "embed"
	"strings"
	"sync"

	"github.com/qwexvf/depusage/internal/extract"
	"github.com/qwexvf/depusage/internal/tsutil"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsphp "github.com/tree-sitter/tree-sitter-php/bindings/go"
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
	includeExprIdx int
}

var s = &state{useDeclIdx: -1, includeExprIdx: -1}

func (st *state) init() {
	st.once.Do(func() {
		st.lang = ts.NewLanguage(tsphp.LanguagePHP())
		st.query = tsutil.MustCompileQuery(st.lang, queriesSource, "php")
		st.parsers = tsutil.NewParserPool(st.lang)
		st.cursors = tsutil.NewCursorPool()
		st.useDeclIdx = tsutil.CaptureIndex(st.query, "use_decl")
		st.includeExprIdx = tsutil.CaptureIndex(st.query, "include_expr")
	})
}

// Extract runs the PHP import pass over body.
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
			switch int(cap.Index) {
			case s.useDeclIdx:
				imports = append(imports, parseUseDecl(&node, body)...)
			case s.includeExprIdx:
				if imp, ok := parseIncludeExpr(&node, body); ok {
					imports = append(imports, imp)
				}
			}
		}
	}
	return imports
}

// parseUseDecl handles `use Foo\Bar;`, `use Foo\Bar as B;`, and
// `use Foo\{Bar, Baz};` (group form). Each clause becomes an Import.
func parseUseDecl(n *ts.Node, body []byte) []extract.Import {
	var out []extract.Import
	line := int(n.StartPosition().Row) + 1
	col := int(n.StartPosition().Column) + 1

	// Walk children to find namespace_use_clause / namespace_use_group.
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case "namespace_use_clause":
			if imp, ok := useClauseToImport(c, "", body, line, col); ok {
				out = append(out, imp)
			}
		case "namespace_use_group":
			// `use Foo\{Bar, Baz};` — the prefix lives as the first
			// named child; group_clause children are the suffixes.
			prefix := ""
			for j := uint(0); j < c.NamedChildCount(); j++ {
				gc := c.NamedChild(j)
				if gc == nil {
					continue
				}
				if gc.Kind() == "namespace_name" {
					prefix = string(gc.Utf8Text(body))
					continue
				}
				if gc.Kind() == "namespace_use_clause" || gc.Kind() == "namespace_use_group_clause" {
					if imp, ok := useClauseToImport(gc, prefix, body, line, col); ok {
						out = append(out, imp)
					}
				}
			}
		}
	}
	return out
}

// useClauseToImport converts one `namespace_use_clause` (or group
// clause) into an Import. `prefix` is non-empty when the clause is
// inside a `Foo\{Bar, Baz}` group.
func useClauseToImport(c *ts.Node, prefix string, body []byte, line, col int) (extract.Import, bool) {
	var name, alias string
	if nameNode := c.ChildByFieldName("name"); nameNode != nil {
		name = string(nameNode.Utf8Text(body))
	}
	if aliasNode := c.ChildByFieldName("alias"); aliasNode != nil {
		alias = string(aliasNode.Utf8Text(body))
	}
	if name == "" {
		// No `name` field — fall back to the first qualified-name-like
		// named child (older grammar versions or group-clause shape).
		for i := uint(0); i < c.NamedChildCount(); i++ {
			ch := c.NamedChild(i)
			if ch == nil {
				continue
			}
			switch ch.Kind() {
			case "qualified_name", "namespace_name":
				if name == "" {
					name = string(ch.Utf8Text(body))
				}
			case "namespace_aliasing_clause":
				for j := uint(0); j < ch.NamedChildCount(); j++ {
					if id := ch.NamedChild(j); id != nil && id.Kind() == "name" {
						alias = string(id.Utf8Text(body))
						break
					}
				}
			}
		}
	}
	if name == "" {
		return extract.Import{}, false
	}
	full := name
	if prefix != "" {
		full = prefix + `\` + name
	}
	out := extract.Import{
		Module: full,
		DepKey: DepKey(full),
		Kind:   extract.ImportStatic,
		Line:   line,
		Column: col,
	}
	// Last segment is the imported symbol.
	if i := strings.LastIndex(full, `\`); i > 0 {
		out.Symbols = []string{full[i+1:]}
	} else {
		out.Symbols = []string{full}
	}
	if alias != "" {
		out.Aliases = map[string]string{alias: out.Symbols[0]}
	}
	return out, true
}

// parseIncludeExpr handles `require '...'` / `include '...'` and
// their `_once` variants. Returns false if the argument isn't a string.
func parseIncludeExpr(n *ts.Node, body []byte) (extract.Import, bool) {
	// The expression has one named child: the included expression.
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() != "string" && c.Kind() != "encapsed_string" {
			return extract.Import{}, false
		}
		raw, ok := stringValue(c, body)
		if !ok {
			return extract.Import{}, false
		}
		out := extract.Import{
			Module: raw,
			DepKey: "", // file paths never resolve to a Composer key
			Kind:   extract.ImportRelative,
			Line:   int(n.StartPosition().Row) + 1,
			Column: int(n.StartPosition().Column) + 1,
		}
		return out, true
	}
	return extract.Import{}, false
}

func stringValue(s *ts.Node, body []byte) (string, bool) {
	for i := uint(0); i < s.NamedChildCount(); i++ {
		c := s.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case "string_value", "string_content":
			return string(c.Utf8Text(body)), true
		case "interpolation":
			return "", false
		}
	}
	if s.NamedChildCount() == 0 {
		return "", true
	}
	return "", false
}
