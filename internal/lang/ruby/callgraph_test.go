package ruby

import (
	"slices"
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestRubyCallGraph_Methods(t *testing.T) {
	src := `
def alpha
  beta
  gamma
end
def beta
  gamma
end
def gamma; end
`
	res, err := Extract([]byte(src), extract.Options{IncludeCallGraph: true})
	if err != nil {
		t.Fatal(err)
	}
	cg := res.CallGraph
	if cg == nil {
		t.Fatal("CallGraph nil")
	}
	// Order may vary; sort-then-compare via setEqual.
	if !setEqual(cg.Edges["alpha"], []string{"beta", "gamma"}) {
		t.Errorf("alpha = %v", cg.Edges["alpha"])
	}
	if !slices.Equal(cg.Edges["beta"], []string{"gamma"}) {
		t.Errorf("beta = %v", cg.Edges["beta"])
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
