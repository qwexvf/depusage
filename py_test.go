package depusage

import (
	"reflect"
	"testing"
)

func TestPy_ImportStatements(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []Import
	}{
		{
			name: "plain import",
			src:  `import requests`,
			want: []Import{
				{
					Module: "requests", DepKey: "requests", Kind: ImportStatic,
					Symbols: []string{"*"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "aliased import",
			src:  `import numpy as np`,
			want: []Import{
				{
					Module: "numpy", DepKey: "numpy", Kind: ImportStatic,
					Symbols: []string{"*"},
					Aliases: map[string]string{"np": "*"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "comma list",
			src:  `import os, sys`,
			want: []Import{
				{Module: "os", DepKey: "os", Kind: ImportStatic, Symbols: []string{"*"}, Line: 1, Column: 1},
				{Module: "sys", DepKey: "sys", Kind: ImportStatic, Symbols: []string{"*"}, Line: 1, Column: 1},
			},
		},
		{
			name: "dotted module",
			src:  `import urllib.parse`,
			want: []Import{
				{
					Module: "urllib.parse", DepKey: "urllib", Kind: ImportStatic,
					Symbols: []string{"*"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "from import",
			src:  `from requests import get, post`,
			want: []Import{
				{
					Module: "requests", DepKey: "requests", Kind: ImportStatic,
					Symbols: []string{"get", "post"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "from import with alias",
			src:  `from requests import get as g`,
			want: []Import{
				{
					Module: "requests", DepKey: "requests", Kind: ImportStatic,
					Symbols: []string{"get"},
					Aliases: map[string]string{"g": "get"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "from import dotted",
			src:  `from urllib.parse import urlparse`,
			want: []Import{
				{
					Module: "urllib.parse", DepKey: "urllib", Kind: ImportStatic,
					Symbols: []string{"urlparse"},
					Line:    1, Column: 1,
				},
			},
		},
		{
			name: "relative from",
			src:  `from .local import helper`,
			want: []Import{
				{
					Module: ".local", DepKey: "", Kind: ImportRelative,
					Symbols: []string{"helper"},
					Line:    1, Column: 1,
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Extract(Python, []byte(tc.src), Options{IncludeImports: true})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Imports, tc.want) {
				t.Errorf("Imports mismatch\n got: %#v\nwant: %#v", res.Imports, tc.want)
			}
		})
	}
}

func TestPy_DynamicImports(t *testing.T) {
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
			res, err := Extract(Python, []byte(tc.src), Options{IncludeImports: true})
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Imports) != 1 {
				t.Fatalf("want 1 import, got %d: %#v", len(res.Imports), res.Imports)
			}
			if res.Imports[0].Module != tc.want || res.Imports[0].Kind != ImportDynamic {
				t.Errorf("unexpected import: %#v", res.Imports[0])
			}
		})
	}
}

func TestPy_ConcurrentExtractIsSafe(t *testing.T) {
	src := `import requests`
	const N = 50
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		go func() {
			_, err := Extract(Python, []byte(src), Options{IncludeImports: true})
			errs <- err
		}()
	}
	for i := 0; i < N; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent extract %d: %v", i, err)
		}
	}
}
