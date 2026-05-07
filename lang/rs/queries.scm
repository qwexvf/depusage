;; depusage queries for Rust imports.
;;
;; In Rust, dependency reachability hinges on `use` statements that
;; reference an external crate. We capture the use_declaration node
;; and let Go code walk to find the leading crate identifier.
(use_declaration) @use_decl

;; Old-style: `extern crate foo;`
(extern_crate_declaration) @extern_crate
