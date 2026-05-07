package depusage

import (
	_ "embed"
	"sync"

	"github.com/qwexvf/depusage/internal/tsutil"
	"github.com/qwexvf/depusage/lang/rb"

	ts "github.com/tree-sitter/go-tree-sitter"
	tsrb "github.com/tree-sitter/tree-sitter-ruby/bindings/go"
)

//go:embed lang/rb/queries.scm
var rbQueriesSource string

type rbState struct {
	once    sync.Once
	lang    *ts.Language
	query   *ts.Query
	parsers *tsutil.ParserPool
	cursors *tsutil.CursorPool

	requireIdx int
}

var rbS = &rbState{requireIdx: -1}

func (s *rbState) init() {
	s.once.Do(func() {
		s.lang = ts.NewLanguage(tsrb.Language())
		s.query = tsutil.MustCompileQuery(s.lang, rbQueriesSource, "rb")
		s.parsers = tsutil.NewParserPool(s.lang)
		s.cursors = tsutil.NewCursorPool()
		s.requireIdx = tsutil.CaptureIndex(s.query, "require_call")
	})
}

func rbExtract(body []byte, opts Options) (Result, error) {
	rbS.init()

	parser := rbS.parsers.Get()
	defer rbS.parsers.Put(parser)
	tree := parser.Parse(body, nil)
	if tree == nil {
		return Result{}, nil
	}
	defer tree.Close()

	var res Result
	if opts.IncludeImports {
		res.Imports = rbCollectImports(tree, body)
	}
	return res, nil
}

func rbCollectImports(tree *ts.Tree, body []byte) []Import {
	cursor := rbS.cursors.Get()
	defer rbS.cursors.Put(cursor)

	matches := cursor.Matches(rbS.query, tree.RootNode(), body)
	imports := make([]Import, 0, 8)
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		for _, cap := range m.Captures {
			if int(cap.Index) != rbS.requireIdx {
				continue
			}
			node := cap.Node
			if imp, ok := rbParseRequireCall(&node, body); ok {
				imports = append(imports, imp)
			}
		}
	}
	return imports
}

// rbParseRequireCall extracts the string literal argument from a
// `require/require_relative/load/gem/autoload` call. Returns false if
// the argument isn't a string literal.
func rbParseRequireCall(n *ts.Node, body []byte) (Import, bool) {
	args := n.ChildByFieldName("arguments")
	if args == nil {
		return Import{}, false
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
			return Import{}, false
		}
		raw, ok := rbStringLiteralValue(c, body)
		if !ok {
			return Import{}, false
		}
		out := Import{
			Module: raw,
			DepKey: rb.DepKey(raw),
			Kind:   ImportRequire,
			Line:   int(n.StartPosition().Row) + 1,
			Column: int(n.StartPosition().Column) + 1,
		}
		if methodName == "require_relative" || rb.IsRelative(raw) {
			out.Kind = ImportRelative
			out.DepKey = ""
		}
		return out, true
	}
	return Import{}, false
}

// rbStringLiteralValue extracts content from a Ruby string node.
// Returns false for interpolated strings (which contain escape_sequence
// or interpolation children).
func rbStringLiteralValue(s *ts.Node, body []byte) (string, bool) {
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
