package php

import (
	"slices"
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestPHPUsedSymbols_NewClass(t *testing.T) {
	src := `<?php
use Symfony\Component\Console\Application;
$app = new Application();
`
	res, _ := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if len(res.UsedSymbols) != 1 {
		t.Fatalf("uses = %v", res.UsedSymbols)
	}
	if res.UsedSymbols[0].Symbol != "Application" {
		t.Errorf("Symbol = %q", res.UsedSymbols[0].Symbol)
	}
}

func TestPHPUsedSymbols_StaticCall(t *testing.T) {
	src := `<?php
use Foo\Bar;
Bar::doThing();
`
	res, _ := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if len(res.UsedSymbols) != 1 || res.UsedSymbols[0].Symbol != "doThing" {
		t.Errorf("uses = %v", res.UsedSymbols)
	}
}

func TestPHPCallGraph_Functions(t *testing.T) {
	src := `<?php
function alpha() { beta(); gamma(); }
function beta()  { gamma(); }
function gamma() {}
`
	res, _ := Extract([]byte(src), extract.Options{IncludeCallGraph: true})
	cg := res.CallGraph
	if cg == nil {
		t.Fatal("CallGraph nil")
	}
	if !slices.Equal(cg.Edges["alpha"], []string{"beta", "gamma"}) {
		t.Errorf("alpha = %v", cg.Edges["alpha"])
	}
}
