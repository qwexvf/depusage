package depusage

import (
	_ "embed"
	"sync"

	"github.com/qwexvf/depusage/internal/tsutil"
	"github.com/qwexvf/depusage/lang/js"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsjs "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

//go:embed lang/js/queries.scm
var jsQueriesSource string

// jsState bundles the lazily-built tree-sitter machinery for JS. We
// don't initialize at package init because the user may never call
// Extract for JS — and ts.NewLanguage allocates a few KB plus a CGo
// call.
type jsState struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	// Pre-resolved capture indices so the hot loop avoids string
	// lookups. -1 if a capture is absent.
	importStmtIdx int
	dynImportIdx  int
	requireIdx    int
}

var jsS = &jsState{
	importStmtIdx: -1,
	dynImportIdx:  -1,
	requireIdx:    -1,
}

func (s *jsState) init() {
	s.once.Do(func() {
		s.lang = ts.NewLanguage(tsjs.Language())
		s.query = tsutil.MustCompileQuery(s.lang, jsQueriesSource, "js")
		s.parsers = tsutil.NewParserPool(s.lang)
		s.cursors = tsutil.NewCursorPool()
		s.importStmtIdx = tsutil.CaptureIndex(s.query, "import_stmt")
		s.dynImportIdx = tsutil.CaptureIndex(s.query, "dyn_import")
		s.requireIdx = tsutil.CaptureIndex(s.query, "require_call")
	})
}

func jsExtract(body []byte, opts Options) (Result, error) {
	jsS.init()

	parser := jsS.parsers.Get()
	defer jsS.parsers.Put(parser)
	tree := parser.Parse(body, nil)
	if tree == nil {
		return Result{}, nil
	}
	defer tree.Close()

	var res Result

	if opts.IncludeImports {
		res.Imports = jsCollectImports(tree, body)
	}
	// Symbols + callgraph land in P2/P3.

	return res, nil
}

// jsCollectImports walks query matches and converts each captured
// container node into one or more Import records.
func jsCollectImports(tree *ts.Tree, body []byte) []Import {
	cursor := jsS.cursors.Get()
	defer jsS.cursors.Put(cursor)

	matches := cursor.Matches(jsS.query, tree.RootNode(), body)
	imports := make([]Import, 0, 8)
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		for _, cap := range m.Captures {
			idx := int(cap.Index)
			node := cap.Node
			switch idx {
			case jsS.importStmtIdx:
				if imp, ok := jsParseImportStatement(&node, body); ok {
					imports = append(imports, imp)
				}
			case jsS.dynImportIdx:
				if imp, ok := jsParseDynamicImport(&node, body); ok {
					imports = append(imports, imp)
				}
			case jsS.requireIdx:
				if imp, ok := jsParseRequireCall(&node, body); ok {
					imports = append(imports, imp)
				}
			}
		}
	}
	return imports
}

// jsParseImportStatement turns an `import_statement` AST node into
// one Import. Handles all five clause shapes:
//
//	import 'm'                        // side-effect
//	import x from 'm'                 // default
//	import { a, b as c } from 'm'     // named, with alias
//	import * as ns from 'm'           // namespace
//	import x, { a } from 'm'          // mixed default + named
func jsParseImportStatement(n *ts.Node, body []byte) (Import, bool) {
	module, ok := jsImportSource(n, body)
	if !ok {
		return Import{}, false
	}
	out := Import{
		Module: module,
		DepKey: js.DepKey(module),
		Kind:   ImportStatic,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	if js.IsRelative(module) {
		out.Kind = ImportRelative
	}

	// Walk the import_clause (if present) for symbols + aliases.
	clause := n.ChildByFieldName("import_clause")
	if clause == nil {
		// Try positional lookup — older grammars don't expose the field.
		for i := uint(0); i < n.NamedChildCount(); i++ {
			c := n.NamedChild(i)
			if c != nil && c.Kind() == "import_clause" {
				clause = c
				break
			}
		}
	}
	if clause != nil {
		jsCollectImportClause(clause, body, &out)
	}
	return out, true
}

// jsCollectImportClause appends Symbols/Aliases for one import_clause
// node into out.
func jsCollectImportClause(clause *ts.Node, body []byte, out *Import) {
	for i := uint(0); i < clause.NamedChildCount(); i++ {
		c := clause.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case "identifier":
			// Default import: `import X from 'm'`
			out.Symbols = append(out.Symbols, "default")
			local := string(c.Utf8Text(body))
			if local != "default" {
				if out.Aliases == nil {
					out.Aliases = map[string]string{}
				}
				out.Aliases[local] = "default"
			}
		case "namespace_import":
			// `import * as ns from 'm'`
			out.Symbols = append(out.Symbols, "*")
			for j := uint(0); j < c.NamedChildCount(); j++ {
				if id := c.NamedChild(j); id != nil && id.Kind() == "identifier" {
					if out.Aliases == nil {
						out.Aliases = map[string]string{}
					}
					out.Aliases[string(id.Utf8Text(body))] = "*"
					break
				}
			}
		case "named_imports":
			// `import { a, b as c } from 'm'`
			for j := uint(0); j < c.NamedChildCount(); j++ {
				spec := c.NamedChild(j)
				if spec == nil || spec.Kind() != "import_specifier" {
					continue
				}
				name := spec.ChildByFieldName("name")
				alias := spec.ChildByFieldName("alias")
				if name == nil {
					continue
				}
				canonical := string(name.Utf8Text(body))
				out.Symbols = append(out.Symbols, canonical)
				if alias != nil {
					if out.Aliases == nil {
						out.Aliases = map[string]string{}
					}
					out.Aliases[string(alias.Utf8Text(body))] = canonical
				}
			}
		}
	}
}

// jsParseDynamicImport handles `import('m')`. Returns false if the
// argument isn't a string literal (computed imports are out of scope).
func jsParseDynamicImport(n *ts.Node, body []byte) (Import, bool) {
	args := n.ChildByFieldName("arguments")
	if args == nil {
		return Import{}, false
	}
	module, ok := jsFirstStringArg(args, body)
	if !ok {
		return Import{}, false
	}
	out := Import{
		Module: module,
		DepKey: js.DepKey(module),
		Kind:   ImportDynamic,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	if js.IsRelative(module) {
		out.Kind = ImportRelative
	}
	return out, true
}

// jsParseRequireCall handles `require('m')`.
func jsParseRequireCall(n *ts.Node, body []byte) (Import, bool) {
	args := n.ChildByFieldName("arguments")
	if args == nil {
		return Import{}, false
	}
	module, ok := jsFirstStringArg(args, body)
	if !ok {
		return Import{}, false
	}
	out := Import{
		Module: module,
		DepKey: js.DepKey(module),
		Kind:   ImportRequire,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	if js.IsRelative(module) {
		out.Kind = ImportRelative
	}
	return out, true
}

// jsImportSource pulls the source-string content out of an
// import_statement node.
func jsImportSource(stmt *ts.Node, body []byte) (string, bool) {
	src := stmt.ChildByFieldName("source")
	if src == nil {
		return "", false
	}
	return jsStringLiteralValue(src, body)
}

// jsFirstStringArg returns the verbatim content of the first argument
// to a call_expression's `arguments` node, if that argument is a
// string literal.
func jsFirstStringArg(args *ts.Node, body []byte) (string, bool) {
	for i := uint(0); i < args.NamedChildCount(); i++ {
		c := args.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() == "string" || c.Kind() == "template_string" {
			return jsStringLiteralValue(c, body)
		}
		// First non-string arg ends the string scan.
		return "", false
	}
	return "", false
}

// jsStringLiteralValue extracts the inner content of a `string` or
// `template_string` node. For template strings with substitutions it
// returns false — those are computed.
func jsStringLiteralValue(s *ts.Node, body []byte) (string, bool) {
	for i := uint(0); i < s.NamedChildCount(); i++ {
		c := s.NamedChild(i)
		if c == nil {
			continue
		}
		switch c.Kind() {
		case "string_fragment":
			return string(c.Utf8Text(body)), true
		case "template_substitution":
			// Template with ${} — computed; bail.
			return "", false
		}
	}
	// Empty string ('' or ""): NamedChildCount is 0 but the node text
	// is the quoted empty string. Treat as empty module.
	if s.NamedChildCount() == 0 {
		return "", true
	}
	return "", false
}
