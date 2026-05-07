# depusage

Multi-language source-code reachability primitives for Go. Tree-sitter
under the hood; no IO, no project model — give it a `[]byte` of source
and it tells you what was imported, which symbols were used, and (per
file) who calls who.

Built for dependency analyzers, SBOM enrichers, and SAST tooling that
need to distinguish "lockfile entry imported and called" from "sitting
in `node_modules` and never touched."

## Status

Pre-1.0 — public types are stable, but new languages and rare
syntactic edge cases may still bend the API. Tracking the design in
[aegis-cli#25](https://github.com/qwexvf/aegis-cli/issues/25).

## Languages

| Language   | Imports | Used symbols | Callgraph |
|------------|:-------:|:------------:|:---------:|
| JavaScript |    ✓    |      ✓       |     ✓     |
| TypeScript |    ✓    |      ✓       |     ✓     |
| Python     |    ✓    |      ✓       |     ✓     |
| Go         |    ✓    |      ✓       |     ✓     |
| Java       |    ✓    |      ✓       |     ✓     |
| PHP        |    ✓    |      ✓       |     ✓     |
| Rust       |    ✓    |      —       |     ✓     |
| Ruby       |    ✓    |      —       |     ✓     |
| C#         |    ✓    |      —       |     ✓     |

**Used-symbol caveat.** The pass tracks bindings that an `import` /
`use` statement introduces by *name* (`import { foo } from 'bar'`,
`use Foo\Bar`, etc.). Rust's `use` brings names into scope but the
typical reachability hook is a `derive` macro on a struct rather than
a call site, so the result is sparse and we omit it for now. Ruby's
`require` doesn't bind a local name at all — gem entry-points become
runtime globals. C#'s `using NS;` opens a namespace without naming a
binding either. For all three, callers should treat
`Reachability=Used` as the strongest signal available.

## Usage

```go
import "github.com/qwexvf/depusage"

src := []byte(`
import { merge } from "lodash";
function transform(x) { return merge({}, x); }
function main()      { return transform({a: 1}); }
`)
res, err := depusage.Extract(depusage.JavaScript, src, depusage.Options{
    IncludeImports:   true,
    IncludeSymbols:   true,
    IncludeCallGraph: true,
})
if err != nil {
    log.Fatal(err)
}
for _, imp := range res.Imports {
    fmt.Println("import", imp.DepKey, imp.Symbols) // lodash [merge]
}
for _, u := range res.UsedSymbols {
    fmt.Println("used", u.DepKey, u.Symbol)        // lodash merge
}
for caller, callees := range res.CallGraph.Edges {
    fmt.Println("calls", caller, "->", callees)    // main -> [transform]
}
```

Each pass is opt-in via `Options`. Passing zero options returns an
empty `Result` — every cost is gated.

## Requirements

- Go 1.24+
- CGo enabled — tree-sitter ships a C runtime. Each language grammar
  adds ~3–4 MB to the final binary.

## Development

```sh
make test     # go test -race ./...
make lint     # golangci-lint run ./...
make fmt      # gofumpt + goimports
make check    # lint + test
```

Per-language extractors live under `internal/lang/<name>/`. Each
sub-package owns its own tree-sitter query, parser pool, and
`DepKey` normalizer; the public dispatcher in `extract.go` picks one
based on the `Language` argument.

Releases are tag-driven: pushing a `v*.*.*` tag runs the full quality
bar via `.github/workflows/release.yml` and creates a GitHub Release
with auto-generated notes. Per-version notes live in
[`CHANGELOG.md`](./CHANGELOG.md).

## License

MIT
