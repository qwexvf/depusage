package php

import (
	"testing"

	"github.com/qwexvf/depusage/internal/extract"
)

func TestUseDeclarations(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		wantModule string
		wantAlias  string
	}{
		{
			name:       "plain use",
			src:        `<?php use Symfony\Component\Console\Application;`,
			wantModule: `Symfony\Component\Console\Application`,
		},
		{
			name:       "use with alias",
			src:        `<?php use Symfony\Component\Console\Application as App;`,
			wantModule: `Symfony\Component\Console\Application`,
			wantAlias:  "App",
		},
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
			if tc.wantAlias != "" {
				if got.Aliases[tc.wantAlias] == "" {
					t.Errorf("alias %q missing in %v", tc.wantAlias, got.Aliases)
				}
			}
		})
	}
}

func TestGroupUse(t *testing.T) {
	// Group form `use Foo\{Bar, Baz}` — known limitation; the grammar
	// node name varies across tree-sitter-php releases. Skipped until
	// we add a runtime probe of the actual tree shape.
	t.Skip("group use parsing is best-effort; tracked for follow-up")
}
