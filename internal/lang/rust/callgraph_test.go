package rust

import (
	"slices"
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestRustCallGraph_Functions(t *testing.T) {
	src := `
pub fn alpha() { beta(); gamma(); }
fn beta() { gamma(); }
fn gamma() {}
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
		t.Errorf("alpha = %v", cg.Edges["alpha"])
	}
	if !slices.Equal(cg.Edges["beta"], []string{"gamma"}) {
		t.Errorf("beta = %v", cg.Edges["beta"])
	}
	for _, f := range cg.Funcs {
		switch f.Name {
		case "alpha":
			if !f.Exported {
				t.Error("alpha (pub) should be Exported")
			}
		case "beta":
			if f.Exported {
				t.Error("beta should NOT be Exported")
			}
		}
	}
}
