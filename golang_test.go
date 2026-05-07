package depusage

import (
	"reflect"
	"testing"
)

func TestGo_ImportSpecs(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []Import
	}{
		{
			name: "single import",
			src: `package main
import "fmt"`,
			want: []Import{
				{
					Module: "fmt", DepKey: "", Kind: ImportStatic,
					Line: 2, Column: 8,
				},
			},
		},
		{
			name: "third-party import",
			src: `package main
import "github.com/spf13/cobra"`,
			want: []Import{
				{
					Module: "github.com/spf13/cobra", DepKey: "github.com/spf13/cobra",
					Kind: ImportStatic, Line: 2, Column: 8,
				},
			},
		},
		{
			name: "block import",
			src: `package main
import (
	"fmt"
	"github.com/spf13/cobra"
)`,
			want: []Import{
				{Module: "fmt", DepKey: "", Kind: ImportStatic, Line: 3, Column: 2},
				{
					Module: "github.com/spf13/cobra", DepKey: "github.com/spf13/cobra",
					Kind: ImportStatic, Line: 4, Column: 2,
				},
			},
		},
		{
			name: "named alias",
			src: `package main
import f "fmt"`,
			want: []Import{
				{
					Module: "fmt", DepKey: "", Kind: ImportStatic,
					Aliases: map[string]string{"f": "*"},
					Line:    2, Column: 8,
				},
			},
		},
		{
			name: "blank import",
			src: `package main
import _ "github.com/lib/pq"`,
			want: []Import{
				{
					Module: "github.com/lib/pq", DepKey: "github.com/lib/pq",
					Kind:    ImportStatic,
					Aliases: map[string]string{"_": "*"},
					Line:    2, Column: 8,
				},
			},
		},
		{
			name: "dot import",
			src: `package main
import . "fmt"`,
			want: []Import{
				{
					Module: "fmt", DepKey: "", Kind: ImportStatic,
					Aliases: map[string]string{".": "*"},
					Line:    2, Column: 8,
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Extract(Go, []byte(tc.src), Options{IncludeImports: true})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(res.Imports, tc.want) {
				t.Errorf("Imports mismatch\n got: %#v\nwant: %#v", res.Imports, tc.want)
			}
		})
	}
}
