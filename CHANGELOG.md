# Changelog

All notable changes to depusage are documented here. Format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html)
once it reaches 1.0. Pre-1.0 minor bumps may include API changes.

## [Unreleased]

## [0.1.0] — 2026-05-08

First feature-complete release covering all three reachability passes
across the supported language set.

### Added

- **Used-symbol extraction** for JavaScript, TypeScript, Python, Go,
  Java, and PHP. `Options.IncludeSymbols` populates
  `Result.UsedSymbols` with every member access / call against a
  bound import.
- **Per-file callgraph** for all 9 languages.
  `Options.IncludeCallGraph` populates `Result.CallGraph` with the
  function/method definitions in the file plus the bare-name
  intra-file edges between them. Method calls and qualified calls
  (`obj.method()`, `pkg.fn()`) are out of scope by design.
- Documented limitations of used-symbol extraction for Rust, Ruby,
  and C# — their import forms don't bind specific local names so the
  pass returns nil for those languages.

### Changed

- Internal restructure: per-language extractors moved to
  `internal/lang/<name>/`. Public types moved to `internal/extract`
  and re-exported from the root via type aliases (preserving type
  identity). External API unchanged.
- CI now runs `golangci-lint` (v2.12) alongside `go test -race` and
  `go vet`.
- Release workflow added: pushing a `v*.*.*` tag runs the full
  quality bar and creates a GitHub Release with auto-generated notes.

## [0.0.2] — 2026-05-07

- CI lint-action bump for golangci-lint v2 support.

## [0.0.1] — 2026-05-07

Initial release.

### Added

- **Import extraction** for 9 languages: JavaScript, TypeScript,
  Python, Go, Rust, Ruby, Java, PHP, C#. Each language ships a
  per-ecosystem `DepKey` normalizer that maps the raw import string
  to the lockfile key (e.g. `@scope/pkg/sub` → `@scope/pkg`).
- Public API: `depusage.Extract(lang, body, opts) (Result, error)`
  with passes gated by `Options.IncludeImports / IncludeSymbols /
  IncludeCallGraph`.
- Concurrency-safe parser pool + cursor pool per language, lazily
  initialized.

[Unreleased]: https://github.com/qwexvf/depusage/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/qwexvf/depusage/compare/v0.0.2...v0.1.0
[0.0.2]: https://github.com/qwexvf/depusage/compare/v0.0.1...v0.0.2
[0.0.1]: https://github.com/qwexvf/depusage/releases/tag/v0.0.1
