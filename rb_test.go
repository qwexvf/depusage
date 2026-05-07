package depusage

import "testing"

func TestRuby_RequireCalls(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		wantModule string
		wantDepKey string
		wantKind   ImportKind
	}{
		{"plain require", `require 'rails'`, "rails", "rails", ImportRequire},
		{"require subpath", `require 'active_support/core_ext'`, "active_support/core_ext", "active_support", ImportRequire},
		{"require_relative", `require_relative './helpers'`, "./helpers", "", ImportRelative},
		{"gem in Gemfile", `gem 'pg'`, "pg", "pg", ImportRequire},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Extract(Ruby, []byte(tc.src), Options{IncludeImports: true})
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

func TestRuby_NonStringArgSkipped(t *testing.T) {
	// `require some_var` — computed.
	res, err := Extract(Ruby, []byte(`require some_var`), Options{IncludeImports: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Imports) != 0 {
		t.Errorf("computed require should be skipped, got %#v", res.Imports)
	}
}
