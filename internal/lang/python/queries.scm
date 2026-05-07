;; depusage queries for Python imports.
;;
;; Captures container nodes; Go code walks their structure to extract
;; module names and bound symbols.

;; `import foo`, `import foo as bar`, `import foo, bar`
(import_statement) @import_stmt

;; `from foo import a, b`, `from .foo import x`, `from . import x`
(import_from_statement) @import_from

;; Future-future-import / star: covered by import_from_statement above.

;; Dynamic: __import__('foo'), importlib.import_module('foo')
(call
  function: (identifier) @_fn
  (#eq? @_fn "__import__")) @dyn_underscore

(call
  function: (attribute
    object: (identifier) @_obj
    attribute: (identifier) @_attr)
  (#eq? @_obj "importlib")
  (#eq? @_attr "import_module")) @dyn_importlib
