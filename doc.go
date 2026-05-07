// Package depusage extracts dependency-usage facts from source code:
// which modules a file imports, which symbols of those imports are
// actually used, and (within a single file) who calls who.
//
// It is built for tools that need to answer "does the user's code
// reach this dependency?" without committing to a full whole-program
// callgraph: dependency analyzers cutting noise from unreachable
// CVEs, SBOM enrichers tagging used-vs-transitive, SAST tools
// gating findings on actual call paths.
//
// Tree-sitter does the parsing. Callers pass a [Language] enum and a
// []byte of source; the result is a typed [Result] with imports,
// optional [UsedSymbol]s, and an optional intra-file [CallGraph].
//
// No IO, no project model, no multi-file resolution — keep that in
// the consumer.
package depusage
