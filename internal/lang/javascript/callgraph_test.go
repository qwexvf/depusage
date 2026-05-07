package javascript

import (
	"slices"
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestCallGraph_FunctionDeclarations(t *testing.T) {
	src := `
function alpha() { beta(); gamma(); }
function beta()  { gamma(); }
function gamma() { return 42; }
`
	res, err := Extract([]byte(src), extract.Options{IncludeCallGraph: true})
	if err != nil {
		t.Fatal(err)
	}
	cg := res.CallGraph
	if cg == nil {
		t.Fatal("CallGraph should not be nil")
	}
	names := funcNames(cg.Funcs)
	if !slices.Equal(names, []string{"alpha", "beta", "gamma"}) {
		t.Errorf("funcs = %v, want [alpha beta gamma]", names)
	}
	if !setEqual(cg.Edges["alpha"], []string{"beta", "gamma"}) {
		t.Errorf("alpha edges = %v, want [beta gamma]", cg.Edges["alpha"])
	}
	if !setEqual(cg.Edges["beta"], []string{"gamma"}) {
		t.Errorf("beta edges = %v, want [gamma]", cg.Edges["beta"])
	}
	if len(cg.Edges["gamma"]) != 0 {
		t.Errorf("gamma should have no edges, got %v", cg.Edges["gamma"])
	}
}

func TestCallGraph_SkipsCallsToImports(t *testing.T) {
	src := `
import { merge } from 'lodash';
function helper() { merge({}, {}); }
function caller() { helper(); }
`
	res, err := Extract([]byte(src), extract.Options{IncludeCallGraph: true})
	if err != nil {
		t.Fatal(err)
	}
	cg := res.CallGraph
	if cg == nil {
		t.Fatal("CallGraph should not be nil")
	}
	// merge() is an import, not a same-file function — must be excluded.
	if slices.Contains(cg.Edges["helper"], "merge") {
		t.Errorf("imported call should not appear in edges: %v", cg.Edges["helper"])
	}
	if !slices.Equal(cg.Edges["caller"], []string{"helper"}) {
		t.Errorf("caller edges = %v, want [helper]", cg.Edges["caller"])
	}
}

func TestCallGraph_ArrowAndExpressionFunctions(t *testing.T) {
	src := `
const arrow = () => { decl(); };
const expr  = function() { decl(); };
function decl() {}
`
	res, err := Extract([]byte(src), extract.Options{IncludeCallGraph: true})
	if err != nil {
		t.Fatal(err)
	}
	cg := res.CallGraph
	names := funcNames(cg.Funcs)
	if !setEqual(names, []string{"arrow", "expr", "decl"}) {
		t.Errorf("funcs = %v, want {arrow, expr, decl}", names)
	}
	if !slices.Equal(cg.Edges["arrow"], []string{"decl"}) {
		t.Errorf("arrow edges = %v", cg.Edges["arrow"])
	}
	if !slices.Equal(cg.Edges["expr"], []string{"decl"}) {
		t.Errorf("expr edges = %v", cg.Edges["expr"])
	}
}

func TestCallGraph_ExportedFlag(t *testing.T) {
	src := `
export function publik() {}
function privit() {}
`
	res, err := Extract([]byte(src), extract.Options{IncludeCallGraph: true})
	if err != nil {
		t.Fatal(err)
	}
	cg := res.CallGraph
	for _, f := range cg.Funcs {
		switch f.Name {
		case "publik":
			if !f.Exported {
				t.Errorf("publik should be Exported")
			}
		case "privit":
			if f.Exported {
				t.Errorf("privit should NOT be Exported")
			}
		}
	}
}

func TestCallGraph_NilWhenNoFunctions(t *testing.T) {
	res, err := Extract([]byte(`const x = 1;`), extract.Options{IncludeCallGraph: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.CallGraph != nil {
		t.Errorf("CallGraph should be nil when no funcs found, got %v", res.CallGraph)
	}
}

func funcNames(fs []extract.Function) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Name
	}
	return out
}
