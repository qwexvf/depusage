package python

import (
	"reflect"
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestImportStatements(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []extract.Import
	}{
		{
			name: "plain import",
			src:  `import requests`,
			want: []extract.Import{
				{
					Module: "requests", DepKey: "requests", Kind: extract.ImportStatic,
					Symbols: []string{"*"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "aliased import",
			src:  `import numpy as np`,
			want: []extract.Import{
				{
					Module: "numpy", DepKey: "numpy", Kind: extract.ImportStatic,
					Symbols: []string{"*"},
					Aliases: map[string]string{"np": "*"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "comma list",
			src:  `import os, sys`,
			want: []extract.Import{
				{Module: "os", DepKey: "os", Kind: extract.ImportStatic, Symbols: []string{"*"}, Line: 1, Column: 1},
				{Module: "sys", DepKey: "sys", Kind: extract.ImportStatic, Symbols: []string{"*"}, Line: 1, Column: 1},
			},
		},
		{
			name: "dotted module",
			src:  `import urllib.parse`,
			want: []extract.Import{
				{
					Module: "urllib.parse", DepKey: "urllib", Kind: extract.ImportStatic,
					Symbols: []string{"*"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "from import",
			src:  `from requests import get, post`,
			want: []extract.Import{
				{
					Module: "requests", DepKey: "requests", Kind: extract.ImportStatic,
					Symbols: []string{"get", "post"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "from import with alias",
			src:  `from requests import get as g`,
			want: []extract.Import{
				{
					Module: "requests", DepKey: "requests", Kind: extract.ImportStatic,
					Symbols: []string{"get"},
					Aliases: map[string]string{"g": "get"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "from import dotted",
			src:  `from urllib.parse import urlparse`,
			want: []extract.Import{
				{
					Module: "urllib.parse", DepKey: "urllib", Kind: extract.ImportStatic,
					Symbols: []string{"urlparse"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "relative from",
			src:  `from .local import helper`,
			want: []extract.Import{
				{
					Module: ".local", DepKey: "", Kind: extract.ImportRelative,
					Symbols: []string{"helper"},
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

func TestDynamicImports(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string // expected module
	}{
		{"__import__", `m = __import__('requests')`, "requests"},
		{"importlib", `m = importlib.import_module('requests')`, "requests"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Extract([]byte(tc.src), extract.Options{IncludeImports: true})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Imports) != 1 {
				t.Fatalf("want 1 import, got %d: %#v", len(res.Imports), res.Imports)
			}
			if res.Imports[0].Module != tc.want || res.Imports[0].Kind != extract.ImportDynamic {
				t.Errorf("unexpected import: %#v", res.Imports[0])
			}
		})
	}
}

func TestConcurrentExtractIsSafe(t *testing.T) {
	src := `import requests`
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
