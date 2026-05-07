// Package python implements the Python import extractor.
package python

import (
	_ "embed"
	"sync"

	"github.com/qwexvf/depusage/internal/extract"
	"github.com/qwexvf/depusage/internal/tsutil"

	ts "github.com/tree-sitter/go-tree-sitter"
	tspy "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

//go:embed queries.scm
var queriesSource string

type state struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	importStmtIdx   int
	importFromIdx   int
	dynUnderscore   int
	dynImportlibIdx int
}

var s = &state{
	importStmtIdx:   -1,
	importFromIdx:   -1,
	dynUnderscore:   -1,
	dynImportlibIdx: -1,
}

func (st *state) init() {
	st.once.Do(func() {
		st.lang = ts.NewLanguage(tspy.Language())
		st.query = tsutil.MustCompileQuery(st.lang, queriesSource, "py")
		st.parsers = tsutil.NewParserPool(st.lang)
		st.cursors = tsutil.NewCursorPool()
		st.importStmtIdx = tsutil.CaptureIndex(st.query, "import_stmt")
		st.importFromIdx = tsutil.CaptureIndex(st.query, "import_from")
		st.dynUnderscore = tsutil.CaptureIndex(st.query, "dyn_underscore")
		st.dynImportlibIdx = tsutil.CaptureIndex(st.query, "dyn_importlib")
	})
}

// Extract runs the Python import pass over body.
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
			case s.importStmtIdx:
				imports = append(imports, parseImportStatement(&node, body)...)
			case s.importFromIdx:
				if imp, ok := parseImportFrom(&node, body); ok {
					imports = append(imports, imp)
				}
			case s.dynUnderscore, s.dynImportlibIdx:
				if imp, ok := parseDynamicCall(&node, body); ok {
					imports = append(imports, imp)
				}
			}
		}
	}
	return imports
}

// parseImportStatement handles `import a, b as c, d`. Each
// dotted_name in the comma-list becomes its own Import.
func parseImportStatement(n *ts.Node, body []byte) []extract.Import {
	var out []extract.Import
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		var module string
		var alias string
		switch c.Kind() {
		case "dotted_name":
			module = string(c.Utf8Text(body))
		case "aliased_import":
			name := c.ChildByFieldName("name")
			as := c.ChildByFieldName("alias")
			if name == nil {
				continue
			}
			module = string(name.Utf8Text(body))
			if as != nil {
				alias = string(as.Utf8Text(body))
			}
		default:
			continue
		}
		imp := extract.Import{
			Module:  module,
			DepKey:  DepKey(module),
			Symbols: []string{"*"},
			Kind:    extract.ImportStatic,
			Line:    int(n.StartPosition().Row) + 1,
			Column:  int(n.StartPosition().Column) + 1,
		}
		if alias != "" {
			imp.Aliases = map[string]string{alias: "*"}
		}
		if IsRelative(module) {
			imp.Kind = extract.ImportRelative
		}
		out = append(out, imp)
	}
	return out
}

// parseImportFrom handles `from foo.bar import a, b as c`.
//
// Module = "foo.bar"; Symbols = ["a", "b"]; Aliases["c"] = "b".
//
// Relative-from variants like `from . import x` produce Module = "."
// (or "..") with the imported names as Symbols. Module is relative.
// `from .pkg import x` becomes Module = ".pkg".
func parseImportFrom(n *ts.Node, body []byte) (extract.Import, bool) {
	module := importFromModule(n, body)
	if module == "" {
		return extract.Import{}, false
	}
	out := extract.Import{
		Module: module,
		DepKey: DepKey(module),
		Kind:   extract.ImportStatic,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	if IsRelative(module) {
		out.Kind = extract.ImportRelative
	}

	// Walk the rest of the children for imported names.
	moduleField := n.ChildByFieldName("module_name")
	for i := uint(0); i < n.NamedChildCount(); i++ {
		c := n.NamedChild(i)
		if c == nil {
			continue
		}
		if moduleField != nil && c.Id() == moduleField.Id() {
			continue
		}
		switch c.Kind() {
		case "dotted_name", "identifier":
			// `from foo import a` — `a` is a dotted_name child of the
			// import_from_statement (not the module).
			if c.Id() == moduleField.Id() {
				continue
			}
			out.Symbols = append(out.Symbols, string(c.Utf8Text(body)))
		case "aliased_import":
			name := c.ChildByFieldName("name")
			as := c.ChildByFieldName("alias")
			if name == nil {
				continue
			}
			canonical := string(name.Utf8Text(body))
			out.Symbols = append(out.Symbols, canonical)
			if as != nil {
				if out.Aliases == nil {
					out.Aliases = map[string]string{}
				}
				out.Aliases[string(as.Utf8Text(body))] = canonical
			}
		case "wildcard_import":
			out.Symbols = append(out.Symbols, "*")
		}
	}
	return out, true
}

// importFromModule reconstructs the module path from an
// import_from_statement. Tree-sitter-python represents it as a
// `module_name` field that's either a `dotted_name` or a leading
// sequence of relative-dot tokens followed by a `dotted_name`.
func importFromModule(n *ts.Node, body []byte) string {
	// Easy case: `module_name` field is set.
	if mod := n.ChildByFieldName("module_name"); mod != nil {
		// For `from .x import y`, mod text already includes the dots.
		return string(mod.Utf8Text(body))
	}
	// Fallback: walk children up to the `import` keyword and
	// concatenate.
	var prefix string
	for i := uint(0); i < n.ChildCount(); i++ {
		c := n.Child(i)
		if c == nil {
			continue
		}
		if c.Kind() == "import" {
			break
		}
		if c.Kind() == "from" {
			continue
		}
		prefix += string(c.Utf8Text(body))
	}
	return prefix
}

// parseDynamicCall handles `__import__('foo')` and
// `importlib.import_module('foo')`. Returns false if the first
// argument isn't a string literal.
func parseDynamicCall(n *ts.Node, body []byte) (extract.Import, bool) {
	args := n.ChildByFieldName("arguments")
	if args == nil {
		return extract.Import{}, false
	}
	for i := uint(0); i < args.NamedChildCount(); i++ {
		c := args.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() != "string" {
			return extract.Import{}, false
		}
		raw, ok := stringLiteralValue(c, body)
		if !ok {
			return extract.Import{}, false
		}
		out := extract.Import{
			Module: raw,
			DepKey: DepKey(raw),
			Kind:   extract.ImportDynamic,
			Line:   int(n.StartPosition().Row) + 1,
			Column: int(n.StartPosition().Column) + 1,
		}
		if IsRelative(raw) {
			out.Kind = extract.ImportRelative
		}
		return out, true
	}
	return extract.Import{}, false
}

// stringLiteralValue extracts the inner content of a python string
// node. Skips f-strings with substitutions.
func stringLiteralValue(s *ts.Node, body []byte) (string, bool) {
	for i := uint(0); i < s.NamedChildCount(); i++ {
		c := s.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case "string_content":
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
