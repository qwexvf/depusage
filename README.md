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

## Per-language examples

Each block shows the source you'd pass as `body` and the relevant
fields you get back. Position fields (`Line`, `Column`) are omitted
for brevity.

### JavaScript / TypeScript

```js
import { merge } from "lodash";
import _ from "lodash";
import * as L from "lodash";
const fs = require("fs");
const m  = await import("./local");
```

```go
res, _ := depusage.Extract(depusage.JavaScript, src, depusage.Options{
    IncludeImports: true, IncludeSymbols: true,
})
// Imports:
//   { Module: "lodash",  DepKey: "lodash", Kind: "static",
//     Symbols: ["merge"] }
//   { Module: "lodash",  DepKey: "lodash", Kind: "static",
//     Symbols: ["default"], Aliases: {"_": "default"} }
//   { Module: "lodash",  DepKey: "lodash", Kind: "static",
//     Symbols: ["*"],       Aliases: {"L": "*"} }
//   { Module: "fs",      DepKey: "",       Kind: "require"  } // node-builtin scheme
//   { Module: "./local", DepKey: "",       Kind: "relative" }
```

### Python

```python
import numpy as np
from requests import get, post
np.array([1, 2])
get("https://x")
```

```go
// Imports:
//   { Module: "numpy",    DepKey: "numpy",    Symbols: ["*"],    Aliases: {"np": "*"} }
//   { Module: "requests", DepKey: "requests", Symbols: ["get","post"] }
// UsedSymbols:
//   { DepKey: "numpy",    Symbol: "array" }
//   { DepKey: "requests", Symbol: "get"   }
```

### Go

```go
package main

import (
    "fmt"
    "github.com/spf13/cobra"
    f "fmt"
    _ "github.com/lib/pq"  // side-effect
)

func main() {
    fmt.Println("x")
    cobra.NewCommand()
}
```

```go
// Imports:
//   { Module: "fmt",                     DepKey: "",                      Kind: "static" }
//   { Module: "github.com/spf13/cobra",  DepKey: "github.com/spf13/cobra" }
//   { Module: "fmt",                     Aliases: {"f": "*"} }
//   { Module: "github.com/lib/pq",       Aliases: {"_": "*"} }
// UsedSymbols:
//   { Module: "fmt",                     Symbol: "Println"    }
//   { Module: "github.com/spf13/cobra",  Symbol: "NewCommand" }
```

Stdlib paths (`fmt`, `encoding/json`, …) get an empty `DepKey` —
`Module` is the canonical comparison key. Third-party paths use the
full module path; consumers doing module-root prefix matching should
strip the trailing sub-package segments themselves.

### Java

```java
package x;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.slf4j.LoggerFactory;

class C {
    void run() {
        ObjectMapper m = new ObjectMapper();
        var log = LoggerFactory.getLogger(C.class);
    }
}
```

```go
// Imports:
//   { Module: "com.fasterxml.jackson.databind.ObjectMapper", Symbols: ["ObjectMapper"] }
//   { Module: "org.slf4j.LoggerFactory",                     Symbols: ["LoggerFactory"] }
// UsedSymbols:
//   { Module: "com.fasterxml.jackson.databind.ObjectMapper", Symbol: "ObjectMapper" }
//   { Module: "org.slf4j.LoggerFactory",                     Symbol: "getLogger"    }
```

`DepKey` is intentionally empty for Java — the FQCN doesn't determine
the Maven `groupId:artifactId` without project-side metadata.
Consumers map `Module` against `pom.xml` / `build.gradle`.

### PHP

```php
<?php
use Symfony\Component\Console\Application;
use Foo\Bar;

$app = new Application();
Bar::doThing();
```

```go
// Imports:
//   { Module: "Symfony\\Component\\Console\\Application", Symbols: ["Application"] }
//   { Module: "Foo\\Bar",                                  Symbols: ["Bar"] }
// UsedSymbols:
//   { Module: "Symfony\\Component\\Console\\Application", Symbol: "Application" }
//   { Module: "Foo\\Bar",                                  Symbol: "doThing" }
```

### Rust

```rust
use serde::Deserialize;
use tokio::sync::{Mutex, RwLock};
use std::collections::HashMap;
use crate::foo::bar;
extern crate libc;
```

```go
// Imports:
//   { Module: "serde",  DepKey: "serde",  Kind: "static" }
//   { Module: "tokio",  DepKey: "tokio",  Kind: "static" }
//   { Module: "std",    DepKey: "",       Kind: "static" }     // stdlib
//   { Module: "crate",  DepKey: "",       Kind: "relative" }   // own crate
//   { Module: "libc",   DepKey: "libc",   Kind: "static" }     // extern crate
```

Used-symbol pass not implemented for Rust (see status note above).
Callgraph is supported.

### Ruby

```ruby
require "rest-client"
require "active_support/core_ext"
require_relative "./helpers"
gem "pg"
```

```go
// Imports:
//   { Module: "rest-client",            DepKey: "rest-client",   Kind: "require" }
//   { Module: "active_support/core_ext", DepKey: "active_support", Kind: "require" }
//   { Module: "./helpers",               DepKey: "",              Kind: "relative" }
//   { Module: "pg",                      DepKey: "pg",            Kind: "require" } // Gemfile shape
```

### C#

```csharp
using System.Collections.Generic;
using static System.Math;
using JS = System.Text.Json;
```

```go
// Imports:
//   { Module: "System.Collections.Generic", Symbols: ["Generic"] }
//   { Module: "System.Math",                Symbols: ["Math"] }
//   { Module: "System.Text.Json",           Symbols: ["Json"], Aliases: {"JS": "Json"} }
```

### Per-file callgraph (any language)

```js
function alpha() { beta(); gamma(); }
function beta()  { gamma(); }
function gamma() { return 42; }
```

```go
res, _ := depusage.Extract(depusage.JavaScript, src, depusage.Options{
    IncludeCallGraph: true,
})
// res.CallGraph.Funcs:
//   { Name: "alpha", Exported: false, StartLn: 1, EndLn: 1 }
//   { Name: "beta",  Exported: false, StartLn: 2, EndLn: 2 }
//   { Name: "gamma", Exported: false, StartLn: 3, EndLn: 3 }
// res.CallGraph.Edges:
//   "alpha" -> ["beta", "gamma"]
//   "beta"  -> ["gamma"]
```

Edges are bare-name calls within the same file. Method calls
(`obj.foo()`) and qualified calls (`pkg.fn()`) aren't edges — they
fall under `UsedSymbols` instead.

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
