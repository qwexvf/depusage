package javascript

import (
	"reflect"
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestStaticImports(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []extract.Import
	}{
		{
			name: "side-effect import",
			src:  `import 'lodash';`,
			want: []extract.Import{
				{Module: "lodash", DepKey: "lodash", Kind: extract.ImportStatic, Line: 1, Column: 1},
			},
		},
		{
			name: "default import",
			src:  `import _ from 'lodash';`,
			want: []extract.Import{
				{
					Module: "lodash", DepKey: "lodash", Kind: extract.ImportStatic,
					Symbols: []string{"default"},
					Aliases: map[string]string{"_": "default"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "named imports",
			src:  `import { merge, debounce } from 'lodash';`,
			want: []extract.Import{
				{
					Module: "lodash", DepKey: "lodash", Kind: extract.ImportStatic,
					Symbols: []string{"merge", "debounce"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "named with alias",
			src:  `import { merge as m, debounce } from 'lodash';`,
			want: []extract.Import{
				{
					Module: "lodash", DepKey: "lodash", Kind: extract.ImportStatic,
					Symbols: []string{"merge", "debounce"},
					Aliases: map[string]string{"m": "merge"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "namespace import",
			src:  `import * as L from 'lodash';`,
			want: []extract.Import{
				{
					Module: "lodash", DepKey: "lodash", Kind: extract.ImportStatic,
					Symbols: []string{"*"},
					Aliases: map[string]string{"L": "*"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "scoped package with subpath",
			src:  `import { x } from '@scope/pkg/sub/path';`,
			want: []extract.Import{
				{
					Module: "@scope/pkg/sub/path", DepKey: "@scope/pkg",
					Kind: extract.ImportStatic, Symbols: []string{"x"},
					Line: 1, Column: 1,
				},
			},
		},
		{
			name: "relative import",
			src:  `import { x } from './local';`,
			want: []extract.Import{
				{
					Module: "./local", DepKey: "", Kind: extract.ImportRelative,
					Symbols: []string{"x"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "node builtin scheme",
			src:  `import fs from 'node:fs';`,
			want: []extract.Import{
				{
					Module: "node:fs", DepKey: "", Kind: extract.ImportStatic,
					Symbols: []string{"default"},
					Aliases: map[string]string{"fs": "default"},
					Line:    1, Column: 1,
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Extract([]byte(tc.src), extract.Options{IncludeImports: true})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Imports, tc.want) {
				t.Errorf("Imports mismatch\n got: %#v\nwant: %#v", res.Imports, tc.want)
			}
		})
	}
}

func TestDynamicImport(t *testing.T) {
	src := `const m = await import('lodash');`
	res, err := Extract([]byte(src), extract.Options{IncludeImports: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imports) != 1 {
		t.Fatalf("want 1 import, got %d: %#v", len(res.Imports), res.Imports)
	}
	got := res.Imports[0]
	if got.Module != "lodash" || got.DepKey != "lodash" || got.Kind != extract.ImportDynamic {
		t.Errorf("unexpected import: %#v", got)
	}
}

func TestRequireCall(t *testing.T) {
	src := `const m = require('lodash');`
	res, err := Extract([]byte(src), extract.Options{IncludeImports: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imports) != 1 {
		t.Fatalf("want 1 import, got %d: %#v", len(res.Imports), res.Imports)
	}
	got := res.Imports[0]
	if got.Module != "lodash" || got.DepKey != "lodash" || got.Kind != extract.ImportRequire {
		t.Errorf("unexpected import: %#v", got)
	}
}

func TestDynamicImportComputedSkipped(t *testing.T) {
	src := `const name = 'lodash'; const m = await import(name);`
	res, err := Extract([]byte(src), extract.Options{IncludeImports: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imports) != 0 {
		t.Errorf("computed dynamic import should be skipped, got %#v", res.Imports)
	}
}

func TestRequireWithComputedArg(t *testing.T) {
	src := `const m = require(someVar);`
	res, err := Extract([]byte(src), extract.Options{IncludeImports: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imports) != 0 {
		t.Errorf("computed require should be skipped, got %#v", res.Imports)
	}
}

func TestMultipleImportsInOneFile(t *testing.T) {
	src := `
import _ from 'lodash';
import { z } from 'zod';
const fs = require('fs');
import('./dynamic').then(() => {});
`
	res, err := Extract([]byte(src), extract.Options{IncludeImports: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imports) != 4 {
		t.Fatalf("want 4 imports, got %d: %#v", len(res.Imports), res.Imports)
	}
	wantModules := []string{"lodash", "zod", "fs", "./dynamic"}
	for i, want := range wantModules {
		if res.Imports[i].Module != want {
			t.Errorf("import %d module = %q, want %q", i, res.Imports[i].Module, want)
		}
	}
}

func TestConcurrentExtractIsSafe(t *testing.T) {
	src := `import _ from 'lodash';`
	const N = 50
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		go func() {
			_, err := Extract([]byte(src), extract.Options{IncludeImports: true})
			errs <- err
		}()
	}
	for i := 0; i < N; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent extract %d: %v", i, err)
		}
	}
}
