;; depusage queries for Go imports.
;;
;; Tree-sitter-go represents imports as `import_declaration` nodes,
;; each containing one or more `import_spec` children (or a single
;; `import_spec_list` for the block form). We capture the spec
;; itself; Go code reads its `path` and optional `name` fields.
(import_spec) @import_spec
