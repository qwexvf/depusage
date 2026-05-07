package java

import (
	"slices"
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestJavaUsedSymbols_NewObject(t *testing.T) {
	src := `package x;
import com.fasterxml.jackson.databind.ObjectMapper;
class C {
    void run() { new ObjectMapper(); }
}
`
	res, _ := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if len(res.UsedSymbols) != 1 {
		t.Fatalf("want 1 use, got %v", res.UsedSymbols)
	}
	if res.UsedSymbols[0].Symbol != "ObjectMapper" {
		t.Errorf("Symbol = %q", res.UsedSymbols[0].Symbol)
	}
}

func TestJavaUsedSymbols_StaticAccess(t *testing.T) {
	src := `package x;
import org.slf4j.LoggerFactory;
class C {
    void log() { LoggerFactory.getLogger(C.class); }
}
`
	res, _ := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if len(res.UsedSymbols) != 1 || res.UsedSymbols[0].Symbol != "getLogger" {
		t.Errorf("uses = %v", res.UsedSymbols)
	}
}

func TestJavaCallGraph_BareCallsOnly(t *testing.T) {
	src := `class C {
    public void alpha() { beta(); other.method(); }
    public void beta()  {}
}
`
	res, _ := Extract([]byte(src), extract.Options{IncludeCallGraph: true})
	if !slices.Equal(res.CallGraph.Edges["alpha"], []string{"beta"}) {
		t.Errorf("alpha edges = %v, want [beta]", res.CallGraph.Edges["alpha"])
	}
}
