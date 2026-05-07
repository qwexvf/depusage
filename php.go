package depusage

import (
	_ "embed"
	"strings"
	"sync"

	"github.com/qwexvf/depusage/internal/tsutil"
	"github.com/qwexvf/depusage/lang/php"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsphp "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

//go:embed lang/php/queries.scm
var phpQueriesSource string

type phpState struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	useDeclIdx    int
	includeExprIdx int
}

var phpS = &phpState{useDeclIdx: -1, includeExprIdx: -1}

func (s *phpState) init() {
	s.once.Do(func() {
		s.lang = ts.NewLanguage(tsphp.LanguagePHP())
		s.query = tsutil.MustCompileQuery(s.lang, phpQueriesSource, "php")
		s.parsers = tsutil.NewParserPool(s.lang)
		s.cursors = tsutil.NewCursorPool()
		s.useDeclIdx = tsutil.CaptureIndex(s.query, "use_decl")
		s.includeExprIdx = tsutil.CaptureIndex(s.query, "include_expr")
	})
}

func phpExtract(body []byte, opts Options) (Result, error) {
	phpS.init()

	parser := phpS.parsers.Get()
	defer phpS.parsers.Put(parser)
	tree := parser.Parse(body, nil)
	if tree == nil {
		return Result{}, nil
	}
	defer tree.Close()

	var res Result
	if opts.IncludeImports {
		res.Imports = phpCollectImports(tree, body)
	}
	return res, nil
}

func phpCollectImports(tree *ts.Tree, body []byte) []Import {
	cursor := phpS.cursors.Get()
	defer phpS.cursors.Put(cursor)

	matches := cursor.Matches(phpS.query, tree.RootNode(), body)
	imports := make([]Import, 0, 8)
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		for _, cap := range m.Captures {
			node := cap.Node
			switch int(cap.Index) {
			case phpS.useDeclIdx:
				imports = append(imports, phpParseUseDecl(&node, body)...)
			case phpS.includeExprIdx:
				if imp, ok := phpParseIncludeExpr(&node, body); ok {
					imports = append(imports, imp)
				}
			}
		}
	}
	return imports
}

// phpParseUseDecl handles `use Foo\Bar;`, `use Foo\Bar as B;`, and
// `use Foo\{Bar, Baz};` (group form). Each clause becomes an Import.
func phpParseUseDecl(n *ts.Node, body []byte) []Import {
	var out []Import
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
			if imp, ok := phpUseClauseToImport(c, "", body, line, col); ok {
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
					if imp, ok := phpUseClauseToImport(gc, prefix, body, line, col); ok {
						out = append(out, imp)
					}
				}
			}
		}
	}
	return out
}

// phpUseClauseToImport converts one `namespace_use_clause` (or group
// clause) into an Import. `prefix` is non-empty when the clause is
// inside a `Foo\{Bar, Baz}` group.
func phpUseClauseToImport(c *ts.Node, prefix string, body []byte, line, col int) (Import, bool) {
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
		return Import{}, false
	}
	full := name
	if prefix != "" {
		full = prefix + `\` + name
	}
	out := Import{
		Module: full,
		DepKey: php.DepKey(full),
		Kind:   ImportStatic,
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

// phpParseIncludeExpr handles `require '...'` / `include '...'` and
// their `_once` variants. Returns false if the argument isn't a string.
func phpParseIncludeExpr(n *ts.Node, body []byte) (Import, bool) {
	// The expression has one named child: the included expression.
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() != "string" && c.Kind() != "encapsed_string" {
			return Import{}, false
		}
		raw, ok := phpStringValue(c, body)
		if !ok {
			return Import{}, false
		}
		out := Import{
			Module: raw,
			DepKey: "", // file paths never resolve to a Composer key
			Kind:   ImportRelative,
			Line:   int(n.StartPosition().Row) + 1,
			Column: int(n.StartPosition().Column) + 1,
		}
		return out, true
	}
	return Import{}, false
}

func phpStringValue(s *ts.Node, body []byte) (string, bool) {
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
