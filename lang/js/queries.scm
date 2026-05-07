;; depusage queries for JavaScript imports.
;;
;; We capture the *containers* (import_statement, require/dynamic-import
;; call_expression) and let Go code walk their children to extract
;; module strings and bound symbols. Tree-sitter query language can't
;; cleanly express "all named import specifiers as a list" in one match,
;; so this hybrid keeps the query tiny and the Go side explicit.

;; Static ES imports: `import X from 'm'`, `import {a,b} from 'm'`,
;; `import * as ns from 'm'`, `import 'm'`.
(import_statement) @import_stmt

;; Dynamic import expression: `import('m')`. Tree-sitter exposes the
;; `import` keyword as a node when used as a callable.
(call_expression
  function: (import)) @dyn_import

;; CommonJS require: `require('m')`. Limited to the bare identifier
;; form; `mod.require()` etc. are deliberately ignored — they're
;; almost always something else.
(call_expression
  function: (identifier) @_fn
  (#eq? @_fn "require")) @require_call
