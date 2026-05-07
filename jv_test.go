package depusage

import "testing"

func TestJava_Imports(t *testing.T) {
	cases := []struct {
		name       string
		src        string
		wantModule string
		wantSyms   []string
	}{
		{
			name:       "FQCN import",
			src:        `package x; import com.fasterxml.jackson.databind.ObjectMapper;`,
			wantModule: "com.fasterxml.jackson.databind.ObjectMapper",
			wantSyms:   []string{"ObjectMapper"},
		},
		{
			name:       "static import",
			src:        `package x; import static com.foo.Bar.method;`,
			wantModule: "com.foo.Bar.method",
			wantSyms:   []string{"method"},
		},
		{
			name:       "wildcard",
			src:        `package x; import com.foo.*;`,
			wantModule: "com.foo.*",
			wantSyms:   []string{"*"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Extract(Java, []byte(tc.src), Options{IncludeImports: true})
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
			if len(got.Symbols) != len(tc.wantSyms) || (len(got.Symbols) > 0 && got.Symbols[0] != tc.wantSyms[0]) {
				t.Errorf("Symbols = %v, want %v", got.Symbols, tc.wantSyms)
			}
		})
	}
}
