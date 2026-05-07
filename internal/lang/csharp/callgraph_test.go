package csharp

import (
	"slices"
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestCSharpCallGraph_Methods(t *testing.T) {
	src := `
class C {
    public void Alpha() { Beta(); Gamma(); }
    public void Beta() { Gamma(); }
    public void Gamma() {}
}
`
	res, err := Extract([]byte(src), extract.Options{IncludeCallGraph: true})
	if err != nil {
		t.Fatal(err)
	}
	cg := res.CallGraph
	if cg == nil {
		t.Fatal("CallGraph nil")
	}
	if !slices.Equal(cg.Edges["Alpha"], []string{"Beta", "Gamma"}) {
		t.Errorf("Alpha = %v", cg.Edges["Alpha"])
	}
}
