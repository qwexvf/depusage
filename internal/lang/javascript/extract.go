// Package javascript implements the JavaScript (and TypeScript)
// import extractor. The dispatcher in package depusage routes both
// Language values here.
package javascript

import (
	_ "embed"
	"sync"

	"github.com/qwexvf/depusage/internal/extract"
	"github.com/qwexvf/depusage/internal/tsutil"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsjs "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

//go:embed queries.scm
var queriesSource string

// state bundles the lazily-built tree-sitter machinery. We don't
// initialize at package init because the user may never call Extract
// for JS — and ts.NewLanguage allocates a few KB plus a CGo call.
type state struct {
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

var s = &state{
	importStmtIdx: -1,
	dynImportIdx:  -1,
	requireIdx:    -1,
}

func (st *state) init() {
	st.once.Do(func() {
		st.lang = ts.NewLanguage(tsjs.Language())
		st.query = tsutil.MustCompileQuery(st.lang, queriesSource, "js")
		st.parsers = tsutil.NewParserPool(st.lang)
		st.cursors = tsutil.NewCursorPool()
		st.importStmtIdx = tsutil.CaptureIndex(st.query, "import_stmt")
		st.dynImportIdx = tsutil.CaptureIndex(st.query, "dyn_import")
		st.requireIdx = tsutil.CaptureIndex(st.query, "require_call")
	})
}

// Extract runs the JS/TS import pass over body.
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
	// Symbols + callgraph land in P2/P3.

	return res, nil
}

// collectImports walks query matches and converts each captured
// container node into one or more Import records.
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
			idx := int(cap.Index)
			node := cap.Node
			switch idx {
			case s.importStmtIdx:
				if imp, ok := parseImportStatement(&node, body); ok {
					imports = append(imports, imp)
				}
			case s.dynImportIdx:
				if imp, ok := parseDynamicImport(&node, body); ok {
					imports = append(imports, imp)
				}
			case s.requireIdx:
				if imp, ok := parseRequireCall(&node, body); ok {
					imports = append(imports, imp)
				}
			}
		}
	}
	return imports
}

// parseImportStatement turns an `import_statement` AST node into one
// Import. Handles all five clause shapes:
//
//	import 'm'                        // side-effect
//	import x from 'm'                 // default
//	import { a, b as c } from 'm'     // named, with alias
//	import * as ns from 'm'           // namespace
//	import x, { a } from 'm'          // mixed default + named
func parseImportStatement(n *ts.Node, body []byte) (extract.Import, bool) {
	module, ok := importSource(n, body)
	if !ok {
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
		collectImportClause(clause, body, &out)
	}
	return out, true
}

// collectImportClause appends Symbols/Aliases for one import_clause
// node into out.
func collectImportClause(clause *ts.Node, body []byte, out *extract.Import) {
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

// parseDynamicImport handles `import('m')`. Returns false if the
// argument isn't a string literal (computed imports are out of scope).
func parseDynamicImport(n *ts.Node, body []byte) (extract.Import, bool) {
	args := n.ChildByFieldName("arguments")
	if args == nil {
		return extract.Import{}, false
	}
	module, ok := firstStringArg(args, body)
	if !ok {
		return extract.Import{}, false
	}
	out := extract.Import{
		Module: module,
		DepKey: DepKey(module),
		Kind:   extract.ImportDynamic,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	if IsRelative(module) {
		out.Kind = extract.ImportRelative
	}
	return out, true
}

// parseRequireCall handles `require('m')`.
func parseRequireCall(n *ts.Node, body []byte) (extract.Import, bool) {
	args := n.ChildByFieldName("arguments")
	if args == nil {
		return extract.Import{}, false
	}
	module, ok := firstStringArg(args, body)
	if !ok {
		return extract.Import{}, false
	}
	out := extract.Import{
		Module: module,
		DepKey: DepKey(module),
		Kind:   extract.ImportRequire,
		Line:   int(n.StartPosition().Row) + 1,
		Column: int(n.StartPosition().Column) + 1,
	}
	if IsRelative(module) {
		out.Kind = extract.ImportRelative
	}
	return out, true
}

// importSource pulls the source-string content out of an
// import_statement node.
func importSource(stmt *ts.Node, body []byte) (string, bool) {
	src := stmt.ChildByFieldName("source")
	if src == nil {
		return "", false
	}
	return stringLiteralValue(src, body)
}

// firstStringArg returns the verbatim content of the first argument
// to a call_expression's `arguments` node, if that argument is a
// string literal.
func firstStringArg(args *ts.Node, body []byte) (string, bool) {
	for i := uint(0); i < args.NamedChildCount(); i++ {
		c := args.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() == "string" || c.Kind() == "template_string" {
			return stringLiteralValue(c, body)
		}
		// First non-string arg ends the string scan.
		return "", false
	}
	return "", false
}

// stringLiteralValue extracts the inner content of a `string` or
// `template_string` node. For template strings with substitutions it
// returns false — those are computed.
func stringLiteralValue(s *ts.Node, body []byte) (string, bool) {
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
