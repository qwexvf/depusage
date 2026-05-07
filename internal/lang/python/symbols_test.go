package python

import (
	"slices"
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestPyUsedSymbols_AttributeAccess(t *testing.T) {
	src := `
import numpy as np
arr = np.array([1, 2])
m = np.matmul(a, b)
`
	res, err := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if err != nil {
		t.Fatal(err)
	}
	syms := names(res.UsedSymbols, "numpy")
	if !setEqual(syms, []string{"array", "matmul"}) {
		t.Errorf("UsedSymbols = %v, want [array matmul]", syms)
	}
}

func TestPyUsedSymbols_FromImportCall(t *testing.T) {
	src := `
from requests import get, post
r = get("https://x")
post("https://x", json={})
`
	res, err := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if err != nil {
		t.Fatal(err)
	}
	syms := names(res.UsedSymbols, "requests")
	if !setEqual(syms, []string{"get", "post"}) {
		t.Errorf("UsedSymbols = %v, want [get post]", syms)
	}
}

func TestPyUsedSymbols_Aliased(t *testing.T) {
	src := `
from requests import get as g
g("https://x")
`
	res, err := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if err != nil {
		t.Fatal(err)
	}
	syms := names(res.UsedSymbols, "requests")
	if !setEqual(syms, []string{"get"}) {
		t.Errorf("UsedSymbols = %v, want [get]", syms)
	}
}

func TestPyCallGraph_Functions(t *testing.T) {
	src := `
def alpha():
    beta()
    gamma()
def beta():
    gamma()
def gamma():
    return 42
`
	res, err := Extract([]byte(src), extract.Options{IncludeCallGraph: true})
	if err != nil {
		t.Fatal(err)
	}
	cg := res.CallGraph
	if cg == nil {
		t.Fatal("CallGraph nil")
	}
	if !slices.Equal(cg.Edges["alpha"], []string{"beta", "gamma"}) {
		t.Errorf("alpha edges = %v", cg.Edges["alpha"])
	}
	if !slices.Equal(cg.Edges["beta"], []string{"gamma"}) {
		t.Errorf("beta edges = %v", cg.Edges["beta"])
	}
}

func TestPyCallGraph_PrivateNamePrefix(t *testing.T) {
	src := `
def public_fn(): pass
def _private(): pass
`
	res, _ := Extract([]byte(src), extract.Options{IncludeCallGraph: true})
	for _, f := range res.CallGraph.Funcs {
		switch f.Name {
		case "public_fn":
			if !f.Exported {
				t.Error("public_fn should be Exported")
			}
		case "_private":
			if f.Exported {
				t.Error("_private should NOT be Exported")
			}
		}
	}
}

// --- helpers ---

func names(uses []extract.UsedSymbol, depKey string) []string {
	var out []string
	for _, u := range uses {
		if u.DepKey == depKey {
			out = append(out, u.Symbol)
		}
	}
	return out
}

func setEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	c := map[string]int{}
	for _, x := range a {
		c[x]++
	}
	for _, x := range b {
		c[x]--
	}
	for _, v := range c {
		if v != 0 {
			return false
		}
	}
	return true
}
