# depusage

Multi-language source-code reachability primitives for Go. Tree-sitter
under the hood; no IO, no project model — give it a `[]byte` of source
and it tells you what was imported, which symbols were used, and (per
file) who calls who.

Built for dependency analyzers, SBOM enrichers, and SAST tooling that
need to distinguish "lockfile entry imported and called" from "sitting
in `node_modules` and never touched."

## Status

Pre-1.0. API may shift. Tracking the design in
[aegis-cli#25](https://github.com/qwexvf/aegis-cli/issues/25).

## Languages

| Language   | Imports | Used symbols | Callgraph |
|------------|:-------:|:------------:|:---------:|
| JavaScript |    ✓    |       —      |     —     |
| TypeScript |    —    |       —      |     —     |
| Python     |    —    |       —      |     —     |
| Go         |    —    |       —      |     —     |
| Rust       |    —    |       —      |     —     |
| Ruby       |    —    |       —      |     —     |
| Java       |    —    |       —      |     —     |
| PHP        |    —    |       —      |     —     |
| C#         |    —    |       —      |     —     |

## Usage

```go
import "github.com/qwexvf/depusage"

src := []byte(`import { merge } from "lodash"; merge({}, {});`)
res, err := depusage.Extract(depusage.JavaScript, src, depusage.Options{
    IncludeImports: true,
})
if err != nil {
    log.Fatal(err)
}
for _, imp := range res.Imports {
    fmt.Println(imp.DepKey, imp.Symbols) // lodash [merge]
}
```

## Requirements

- Go 1.24+
- CGo enabled — tree-sitter ships a C runtime. Each language grammar
  adds ~3–4 MB to the final binary.

## License

MIT
