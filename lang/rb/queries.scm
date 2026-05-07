;; depusage queries for Ruby imports.
;;
;; Ruby has no static import syntax — `require`, `require_relative`,
;; `load`, and `gem` are runtime calls. We capture the call shapes
;; we care about and let Go code extract the literal-string argument.

(call
  method: (identifier) @_m
  (#match? @_m "^(require|require_relative|load|gem|autoload)$")) @require_call
