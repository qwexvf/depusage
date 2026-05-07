package javascript

import (
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestUsedSymbols_NamedImportCalls(t *testing.T) {
	src := `
import { merge, debounce } from 'lodash';
const a = merge({}, {});
const b = debounce(fn, 200);
`
	res, err := Extract([]byte(src), extract.Options{IncludeImports: true, IncludeSymbols: true})
	if err != nil {
		t.Fatal(err)
	}
	syms := symbolNames(res.UsedSymbols, "lodash")
	if !setEqual(syms, []string{"merge", "debounce"}) {
		t.Errorf("UsedSymbols(lodash) = %v, want [merge debounce]", syms)
	}
}

func TestUsedSymbols_DefaultImportMember(t *testing.T) {
	src := `
import _ from 'lodash';
_.merge({}, {});
const x = _.PI;
`
	res, err := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if err != nil {
		t.Fatal(err)
	}
	syms := symbolNames(res.UsedSymbols, "lodash")
	if !setEqual(syms, []string{"merge", "PI"}) {
		t.Errorf("UsedSymbols(lodash) = %v, want [merge PI]", syms)
	}
}

func TestUsedSymbols_NamespaceImport(t *testing.T) {
	src := `
import * as L from 'lodash';
L.merge({}, {});
L.debounce(fn);
`
	res, err := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if err != nil {
		t.Fatal(err)
	}
	syms := symbolNames(res.UsedSymbols, "lodash")
	if !setEqual(syms, []string{"merge", "debounce"}) {
		t.Errorf("UsedSymbols(lodash) = %v, want [merge debounce]", syms)
	}
}

func TestUsedSymbols_AliasedNamedImport(t *testing.T) {
	src := `
import { merge as m } from 'lodash';
m({}, {});
`
	res, err := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if err != nil {
		t.Fatal(err)
	}
	syms := symbolNames(res.UsedSymbols, "lodash")
	// Aliased name `m` maps back to canonical "merge".
	if !setEqual(syms, []string{"merge"}) {
		t.Errorf("UsedSymbols(lodash) = %v, want [merge]", syms)
	}
}

func TestUsedSymbols_OptOutWhenNotRequested(t *testing.T) {
	src := `import { merge } from 'lodash'; merge();`
	res, err := Extract([]byte(src), extract.Options{IncludeImports: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.UsedSymbols != nil {
		t.Errorf("UsedSymbols should be nil without IncludeSymbols, got %v", res.UsedSymbols)
	}
}

func TestUsedSymbols_OnlyClearsImportsWhenAlsoOptedOut(t *testing.T) {
	src := `import { merge } from 'lodash'; merge();`
	res, err := Extract([]byte(src), extract.Options{IncludeSymbols: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Imports != nil {
		t.Errorf("Imports should be cleared when only Symbols is requested, got %v", res.Imports)
	}
	if len(res.UsedSymbols) != 1 || res.UsedSymbols[0].Symbol != "merge" {
		t.Errorf("UsedSymbols = %v, want one entry for merge", res.UsedSymbols)
	}
}

// --- helpers ---

func symbolNames(uses []extract.UsedSymbol, depKey string) []string {
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
	seen := map[string]int{}
	for _, x := range a {
		seen[x]++
	}
	for _, x := range b {
		seen[x]--
	}
	for _, v := range seen {
		if v != 0 {
			return false
		}
	}
	return true
}
