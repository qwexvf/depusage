package depusage

import (
	_ "embed"
	"sync"

	"github.com/qwexvf/depusage/internal/tsutil"
	"github.com/qwexvf/depusage/lang/py"

	ts "github.com/tree-sitter/go-tree-sitter"
	tspy "github.com/tree-sitter/tree-sitter-python/bindings/go"
)

//go:embed lang/py/queries.scm
var pyQueriesSource string

type pyState struct {
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

var pyS = &pyState{
	importStmtIdx:   -1,
	importFromIdx:   -1,
	dynUnderscore:   -1,
	dynImportlibIdx: -1,
}

func (s *pyState) init() {
	s.once.Do(func() {
		s.lang = ts.NewLanguage(tspy.Language())
		s.query = tsutil.MustCompileQuery(s.lang, pyQueriesSource, "py")
		s.parsers = tsutil.NewParserPool(s.lang)
		s.cursors = tsutil.NewCursorPool()
		s.importStmtIdx = tsutil.CaptureIndex(s.query, "import_stmt")
		s.importFromIdx = tsutil.CaptureIndex(s.query, "import_from")
		s.dynUnderscore = tsutil.CaptureIndex(s.query, "dyn_underscore")
		s.dynImportlibIdx = tsutil.CaptureIndex(s.query, "dyn_importlib")
	})
}

func pyExtract(body []byte, opts Options) (Result, error) {
	pyS.init()

	parser := pyS.parsers.Get()
	defer pyS.parsers.Put(parser)
	tree := parser.Parse(body, nil)
	if tree == nil {
		return Result{}, nil
	}
	defer tree.Close()

	var res Result
	if opts.IncludeImports {
		res.Imports = pyCollectImports(tree, body)
	}
	return res, nil
}

func pyCollectImports(tree *ts.Tree, body []byte) []Import {
	cursor := pyS.cursors.Get()
	defer pyS.cursors.Put(cursor)

	matches := cursor.Matches(pyS.query, tree.RootNode(), body)
	imports := make([]Import, 0, 8)
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		for _, cap := range m.Captures {
			node := cap.Node
			switch int(cap.Index) {
			case pyS.importStmtIdx:
				imports = append(imports, pyParseImportStatement(&node, body)...)
			case pyS.importFromIdx:
				if imp, ok := pyParseImportFrom(&node, body); ok {
					imports = append(imports, imp)
				}
			case pyS.dynUnderscore, pyS.dynImportlibIdx:
				if imp, ok := pyParseDynamicCall(&node, body); ok {
					imports = append(imports, imp)
				}
			}
		}
	}
	return imports
}

// pyParseImportStatement handles `import a, b as c, d`. Each
// dotted_name in the comma-list becomes its own Import.
func pyParseImportStatement(n *ts.Node, body []byte) []Import {
	var out []Import
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
		imp := Import{
			Module:  module,
			DepKey:  py.DepKey(module),
			Symbols: []string{"*"},
			Kind:    ImportStatic,
			Line:    int(n.StartPosition().Row) + 1,
			Column:  int(n.StartPosition().Column) + 1,
		}
		if alias != "" {
			imp.Aliases = map[string]string{alias: "*"}
		}
		if py.IsRelative(module) {
			imp.Kind = ImportRelative
		}
		out = append(out, imp)
	}
	return out
}

// pyParseImportFrom handles `from foo.bar import a, b as c`.
//
// Module = "foo.bar"; Symbols = ["a", "b"]; Aliases["c"] = "b".
//
// Relative-from variants like `from . import x` produce Module = "."
// (or "..") with the imported names as Symbols. Module is relative.
// `from .pkg import x` becomes Module = ".pkg".
func pyParseImportFrom(n *ts.Node, body []byte) (Import, bool) {
	module := pyImportFromModule(n, body)
	if module == "" {
		return Import{}, false
	}
	out := Import{
		Module: module,
		DepKey: py.DepKey(module),
		Kind:   ImportStatic,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	if py.IsRelative(module) {
		out.Kind = ImportRelative
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

// pyImportFromModule reconstructs the module path from an
// import_from_statement. Tree-sitter-python represents it as a
// `module_name` field that's either a `dotted_name` or a leading
// sequence of relative-dot tokens followed by a `dotted_name`.
func pyImportFromModule(n *ts.Node, body []byte) string {
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

// pyParseDynamicCall handles `__import__('foo')` and
// `importlib.import_module('foo')`. Returns false if the first
// argument isn't a string literal.
func pyParseDynamicCall(n *ts.Node, body []byte) (Import, bool) {
	args := n.ChildByFieldName("arguments")
	if args == nil {
		return Import{}, false
	}
	for i := uint(0); i < args.NamedChildCount(); i++ {
		c := args.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() != "string" {
			return Import{}, false
		}
		raw, ok := pyStringLiteralValue(c, body)
		if !ok {
			return Import{}, false
		}
		out := Import{
			Module: raw,
			DepKey: py.DepKey(raw),
			Kind:   ImportDynamic,
			Line:   int(n.StartPosition().Row) + 1,
			Column: int(n.StartPosition().Column) + 1,
		}
		if py.IsRelative(raw) {
			out.Kind = ImportRelative
		}
		return out, true
	}
	return Import{}, false
}

// pyStringLiteralValue extracts the inner content of a python string
// node. Skips f-strings with substitutions.
func pyStringLiteralValue(s *ts.Node, body []byte) (string, bool) {
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
