;; depusage queries for PHP imports.
;;
;; PHP has two flavors of import:
;;   - namespace_use_declaration: `use Foo\Bar;` (compile-time)
;;   - require / require_once / include / include_once: runtime file
;;     loads. We treat them as ImportRequire when the argument is a
;;     literal path.

(namespace_use_declaration) @use_decl

;; Include-style runtime imports. These are expressions, not statements
;; — wrapped in expression_statement.
(include_expression) @include_expr
(include_once_expression) @include_expr
(require_expression) @include_expr
(require_once_expression) @include_expr
