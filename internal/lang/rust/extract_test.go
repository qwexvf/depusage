package rust

import (
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestUseDecls(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		wantModule string
		wantDepKey string
		wantKind   extract.ImportKind
	}{
		{"third-party", `use serde::Deserialize;`, "serde", "serde", extract.ImportStatic},
		{"third-party scoped list", `use tokio::sync::{Mutex, RwLock};`, "tokio", "tokio", extract.ImportStatic},
		{"third-party with alias", `use serde_json::Value as V;`, "serde_json", "serde_json", extract.ImportStatic},
		{"std", `use std::collections::HashMap;`, "std", "", extract.ImportStatic},
		{"core", `use core::mem;`, "core", "", extract.ImportStatic},
		{"crate-local", `use crate::foo::bar;`, "crate", "", extract.ImportRelative},
		{"super", `use super::parent;`, "super", "", extract.ImportRelative},
		{"self", `use self::sibling;`, "self", "", extract.ImportRelative},
		{"wildcard", `use serde::*;`, "serde", "serde", extract.ImportStatic},
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
			got := res.Imports[0]
			if got.Module != tc.wantModule {
				t.Errorf("Module = %q, want %q", got.Module, tc.wantModule)
			}
			if got.DepKey != tc.wantDepKey {
				t.Errorf("DepKey = %q, want %q", got.DepKey, tc.wantDepKey)
			}
			if got.Kind != tc.wantKind {
				t.Errorf("Kind = %q, want %q", got.Kind, tc.wantKind)
			}
		})
	}
}

func TestExternCrate(t *testing.T) {
	res, err := Extract([]byte(`extern crate serde;`), extract.Options{IncludeImports: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imports) != 1 {
		t.Fatalf("want 1 import, got %d", len(res.Imports))
	}
	if res.Imports[0].Module != "serde" || res.Imports[0].DepKey != "serde" {
		t.Errorf("unexpected: %#v", res.Imports[0])
	}
}
