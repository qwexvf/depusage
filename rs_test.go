package depusage

import (
	"testing"
)

func TestRust_UseDecls(t *testing.T) {
	cases := []struct {
		name        string
		src         string
		wantModule  string
		wantDepKey  string
		wantKind    ImportKind
	}{
		{"third-party", `use serde::Deserialize;`, "serde", "serde", ImportStatic},
		{"third-party scoped list", `use tokio::sync::{Mutex, RwLock};`, "tokio", "tokio", ImportStatic},
		{"third-party with alias", `use serde_json::Value as V;`, "serde_json", "serde_json", ImportStatic},
		{"std", `use std::collections::HashMap;`, "std", "", ImportStatic},
		{"core", `use core::mem;`, "core", "", ImportStatic},
		{"crate-local", `use crate::foo::bar;`, "crate", "", ImportRelative},
		{"super", `use super::parent;`, "super", "", ImportRelative},
		{"self", `use self::sibling;`, "self", "", ImportRelative},
		{"wildcard", `use serde::*;`, "serde", "serde", ImportStatic},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Extract(Rust, []byte(tc.src), Options{IncludeImports: true})
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

func TestRust_ExternCrate(t *testing.T) {
	res, err := Extract(Rust, []byte(`extern crate serde;`), Options{IncludeImports: true})
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
