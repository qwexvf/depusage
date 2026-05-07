package golang

import (
	"slices"
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestGoUsedSymbols_PackageMember(t *testing.T) {
	src := `package main
import "fmt"
import "github.com/spf13/cobra"
func main() {
    fmt.Println("x")
    cobra.NewCommand()
}
`
	res, err := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if err != nil {
		t.Fatal(err)
	}
	// Group by Module (DepKey is empty for stdlib paths like "fmt").
	got := map[string][]string{}
	for _, u := range res.UsedSymbols {
		got[u.Module] = append(got[u.Module], u.Symbol)
	}
	if !setEqual(got["fmt"], []string{"Println"}) {
		t.Errorf("fmt = %v", got["fmt"])
	}
	if !setEqual(got["github.com/spf13/cobra"], []string{"NewCommand"}) {
		t.Errorf("cobra = %v", got["github.com/spf13/cobra"])
	}
}

func TestGoUsedSymbols_ExplicitAlias(t *testing.T) {
	src := `package main
import f "fmt"
func main() { f.Println("x") }
`
	res, _ := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if len(res.UsedSymbols) != 1 || res.UsedSymbols[0].Symbol != "Println" {
		t.Errorf("UsedSymbols = %v", res.UsedSymbols)
	}
}

func TestGoUsedSymbols_BlankAndDotNotBindings(t *testing.T) {
	src := `package main
import _ "embed"
import . "fmt"
func main() {}
`
	res, _ := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if len(res.UsedSymbols) != 0 {
		t.Errorf("blank/dot imports must not produce bindings: %v", res.UsedSymbols)
	}
}

func TestGoCallGraph_Functions(t *testing.T) {
	src := `package main
func alpha() { beta(); gamma() }
func beta()  { gamma() }
func gamma() {}
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
}

func TestGoCallGraph_ExportedHeuristic(t *testing.T) {
	src := `package main
func Public() {}
func internal() {}
`
	res, _ := Extract([]byte(src), extract.Options{IncludeCallGraph: true})
	for _, f := range res.CallGraph.Funcs {
		switch f.Name {
		case "Public":
			if !f.Exported {
				t.Error("Public should be Exported")
			}
		case "internal":
			if f.Exported {
				t.Error("internal should NOT be Exported")
			}
		}
	}
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
