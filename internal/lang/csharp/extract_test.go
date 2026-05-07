package csharp

import (
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestUsingDirectives(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		wantModule string
	}{
		{"plain", `using System.Collections.Generic;`, "System.Collections.Generic"},
		{"static", `using static System.Math;`, "System.Math"},
		{"alias", `using JS = System.Text.Json;`, "System.Text.Json"},
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
			if res.Imports[0].Module != tc.wantModule {
				t.Errorf("Module = %q, want %q", res.Imports[0].Module, tc.wantModule)
			}
		})
	}
}
