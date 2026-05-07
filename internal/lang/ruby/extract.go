// Package ruby implements the Ruby import extractor.
package ruby

import (
	_ "embed"
	"sync"

	"github.com/qwexvf/depusage/internal/extract"
	"github.com/qwexvf/depusage/internal/tsutil"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsrb "github.com/tree-sitter/tree-sitter-ruby/bindings/go"
)

//go:embed queries.scm
var queriesSource string

type state struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	requireIdx int
}

var s = &state{requireIdx: -1}

func (st *state) init() {
	st.once.Do(func() {
		st.lang = ts.NewLanguage(tsrb.Language())
		st.query = tsutil.MustCompileQuery(st.lang, queriesSource, "rb")
		st.parsers = tsutil.NewParserPool(st.lang)
		st.cursors = tsutil.NewCursorPool()
		st.requireIdx = tsutil.CaptureIndex(st.query, "require_call")
	})
}

// Extract runs the Ruby import pass over body.
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
	// UsedSymbols not implemented for Ruby: `require` doesn't introduce
	// a local binding; gem entry-points become global constants/classes
	// resolved at runtime.
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
			if int(cap.Index) != s.requireIdx {
				continue
			}
			node := cap.Node
			if imp, ok := parseRequireCall(&node, body); ok {
				imports = append(imports, imp)
			}
		}
	}
	return imports
}

// parseRequireCall extracts the string literal argument from a
// `require/require_relative/load/gem/autoload` call. Returns false if
// the argument isn't a string literal.
func parseRequireCall(n *ts.Node, body []byte) (extract.Import, bool) {
	args := n.ChildByFieldName("arguments")
	if args == nil {
		return extract.Import{}, false
	}
	method := n.ChildByFieldName("method")
	methodName := ""
	if method != nil {
		methodName = string(method.Utf8Text(body))
	}

	// `gem 'name'` and `autoload :Sym, 'file'` — first string-arg is
	// the dependency name in both cases.
	for i := uint(0); i < args.NamedChildCount(); i++ {
		c := args.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() != "string" {
			// `autoload` takes a symbol first then a string — keep looking.
			if methodName == "autoload" {
				continue
			}
			return extract.Import{}, false
		}
		raw, ok := stringLiteralValue(c, body)
		if !ok {
			return extract.Import{}, false
		}
		out := extract.Import{
			Module: raw,
			DepKey: DepKey(raw),
			Kind:   extract.ImportRequire,
			Line:   int(n.StartPosition().Row) + 1,
			Column: int(n.StartPosition().Column) + 1,
		}
		if methodName == "require_relative" || IsRelative(raw) {
			out.Kind = extract.ImportRelative
			out.DepKey = ""
		}
		return out, true
	}
	return extract.Import{}, false
}

// stringLiteralValue extracts content from a Ruby string node.
// Returns false for interpolated strings (which contain escape_sequence
// or interpolation children).
func stringLiteralValue(s *ts.Node, body []byte) (string, bool) {
	for i := uint(0); i < s.NamedChildCount(); i++ {
		c := s.NamedChild(i)
		if c == nil {
			continue
		}
		if c.Kind() == "interpolation" {
			return "", false
		}
		if c.Kind() == "string_content" {
			return string(c.Utf8Text(body)), true
		}
	}
	if s.NamedChildCount() == 0 {
		return "", true
	}
	return "", false
}
