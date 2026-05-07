// Package tsutil holds tree-sitter helpers shared across the
// per-language extractors: a per-language parser pool and a tiny
// query-compile-once wrapper. Internal — not part of the public API.
package tsutil

import (
	"sync"

	ts "github.com/tree-sitter/go-tree-sitter"
)

// ParserPool hands out *ts.Parser instances pre-bound to a single
// language. Parsers are not goroutine-safe individually; the pool lets
// concurrent callers pick one, parse, return.
type ParserPool struct {
	lang *ts.Language
	pool sync.Pool
}

// NewParserPool returns a pool that produces parsers bound to lang.
func NewParserPool(lang *ts.Language) *ParserPool {
	pp := &ParserPool{lang: lang}
	pp.pool.New = func() any {
		p := ts.NewParser()
		_ = p.SetLanguage(lang)
		return p
	}
	return pp
}

// Get returns a parser ready to Parse. Callers must Put the parser
// back when done.
func (pp *ParserPool) Get() *ts.Parser {
	return pp.pool.Get().(*ts.Parser)
}

// Put returns a parser to the pool. The parser is not Closed — pool
// users own its lifetime.
func (pp *ParserPool) Put(p *ts.Parser) {
	pp.pool.Put(p)
}

// CursorPool hands out *ts.QueryCursor instances. Cursors are also
// not goroutine-safe individually but are cheap to allocate; pooling
// avoids the allocation in tight loops.
type CursorPool struct {
	pool sync.Pool
}

// NewCursorPool returns an empty pool. Cursors are created lazily.
func NewCursorPool() *CursorPool {
	cp := &CursorPool{}
	cp.pool.New = func() any { return ts.NewQueryCursor() }
	return cp
}

// Get returns a fresh-or-recycled cursor.
func (cp *CursorPool) Get() *ts.QueryCursor {
	return cp.pool.Get().(*ts.QueryCursor)
}

// Put returns a cursor to the pool.
func (cp *CursorPool) Put(c *ts.QueryCursor) {
	cp.pool.Put(c)
}
